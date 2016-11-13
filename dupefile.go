//
// This program is to help me deal with duplicate image files I have.
//
// I am trying to organize my collection of images/galleries. I have found that
// there are copies of images in several cases. I think I will be able to delete
// plenty if I can cut down on this duplication. Then I will have less to
// organize.
//
// What this program will do:
// - Recursively find all files.
// - Calculate checksum of each file.
// - Report any two files with identical checksums.
// - Report any two files with the identical base name.
//
// Then I hope to be able to add functionality to delete dupes.
package main

import (
	"bufio"
	"crypto/md5"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
)

// Args holds command line arguments.
type Args struct {
	Dir  string
	Live bool
}

// File holds information about one file.
type File struct {
	Basename string
	Path     string
	Size     int64
	Hash     []byte
}

func main() {
	log.SetFlags(0)

	args, err := getArgs()
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Looking for files...")
	files, err := findFiles(args.Dir)
	if err != nil {
		log.Fatalf("Unable to find files: %s", err)
	}

	log.Print("Calculating checksums...")
	err = calculateChecksums(files)
	if err != nil {
		log.Fatalf("Unable to calculate checksums: %s", err)
	}

	log.Print("Reporting duplicates...")
	err = reportDupes(files, args.Live)
	if err != nil {
		log.Fatalf("Unable to report duplicates: %s", err)
	}
}

func getArgs() (*Args, error) {
	dir := flag.String("dir", "", "Directory to examine.")
	live := flag.Bool("live", false, "Enable file deletion.")

	flag.Parse()

	if len(*dir) == 0 {
		flag.PrintDefaults()
		return nil, fmt.Errorf("You must provide a directory.")
	}

	return &Args{
		Dir:  *dir,
		Live: *live,
	}, nil
}

func findFiles(dir string) ([]*File, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("Stat: %s: %s", dir, err)
	}

	if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	dh, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("Open: %s: %s", dir, err)
	}

	fis, err := dh.Readdir(0)
	if err != nil {
		_ = dh.Close()
		return nil, fmt.Errorf("Readdir: %s: %s", dir, err)
	}

	err = dh.Close()
	if err != nil {
		return nil, fmt.Errorf("Close: %s: %s", dir, err)
	}

	foundFiles := []*File{}

	for _, fi := range fis {
		if fi.Name() == "." || fi.Name() == ".." {
			continue
		}

		filePath := path.Join(dir, fi.Name())

		if fi.IsDir() {
			dirFiles, err := findFiles(filePath)
			if err != nil {
				return nil, err
			}

			foundFiles = append(foundFiles, dirFiles...)
			continue
		}

		foundFiles = append(foundFiles, &File{
			Basename: fi.Name(),
			Path:     filePath,
			Size:     fi.Size(),
		})
	}

	return foundFiles, nil
}

func calculateChecksums(files []*File) error {
	for _, file := range files {
		fh, err := os.Open(file.Path)
		if err != nil {
			return fmt.Errorf("Open: %s: %s", file.Path, err)
		}

		reader := bufio.NewReader(fh)

		//hasher := sha256.New()
		hasher := md5.New()

		n, err := reader.WriteTo(hasher)
		if err != nil {
			_ = fh.Close()
			return fmt.Errorf("Writing to hash: %s: %s", file.Path, err)
		}

		if n != file.Size {
			_ = fh.Close()
			return fmt.Errorf("Short read/write: %s", file.Path)
		}

		file.Hash = hasher.Sum(nil)

		err = fh.Close()
		if err != nil {
			return fmt.Errorf("Close: %s: %s", file.Path, err)
		}
	}

	return nil
}

func reportDupes(files []*File, live bool) error {
	//checksumToFile := make(map[[sha256.Size]byte]*File)
	checksumToFile := make(map[[md5.Size]byte]*File)

	for _, file := range files {
		//var checksum [sha256.Size]byte
		var checksum [md5.Size]byte
		for i, b := range file.Hash {
			checksum[i] = b
		}

		//log.Printf("%s", file)

		foundFile, ok := checksumToFile[checksum]
		if ok {
			// Hash collision. Deep compare.
			identical, err := isIdentical(foundFile, file)
			if err != nil {
				return fmt.Errorf("Unable to compare files: %s %s: %s", foundFile.Path,
					file.Path, err)
			}

			if identical {
				log.Printf("Duplicate found: %s and %s", file.Path, foundFile.Path)
				err := resolveDuplicate(foundFile, file, live)
				if err != nil {
					return err
				}
				continue
			}

			log.Printf("Hash collision but not identical: %s and %s", file.Path,
				foundFile.Path)
			continue
		}

		checksumToFile[checksum] = file
	}

	return nil
}

func isIdentical(file1, file2 *File) (bool, error) {
	contents1, err := readFile(file1)
	if err != nil {
		return false, err
	}

	contents2, err := readFile(file2)
	if err != nil {
		return false, err
	}

	if len(contents1) != len(contents2) {
		return false, nil
	}

	for i := range contents1 {
		if contents1[i] != contents2[i] {
			return false, nil
		}
	}

	return true, nil
}

func readFile(file *File) ([]byte, error) {
	fh, err := os.Open(file.Path)
	if err != nil {
		return nil, fmt.Errorf("Open: %s: %s", file, err)
	}

	contents, err := ioutil.ReadAll(fh)
	if err != nil {
		_ = fh.Close()
		return nil, fmt.Errorf("ReadAll: %s: %s", file.Path, err)
	}

	err = fh.Close()
	if err != nil {
		return nil, fmt.Errorf("Close: %s: %s", file.Path, err)
	}

	if int64(len(contents)) != file.Size {
		return nil, fmt.Errorf("Short read: %s", file.Path)
	}

	return contents, nil
}

func resolveDuplicate(file1, file2 *File, live bool) error {
	dir1, _ := path.Split(file1.Path)
	dir2, _ := path.Split(file2.Path)

	type Rule struct {
		KeepDir string
		RmDir   string
	}

	rules := []Rule{
		{
			KeepDir: "/home/will/t/testing/",
			RmDir:   "/home/will/t/testing/2/",
		},
		{
			KeepDir: "/home/will/images/game screenshots/WoW_screenshots/",
			RmDir:   "/home/will/images/game screenshots/WoW_screenshots/",
		},
		{
			KeepDir: "/home/will/images/storey_albums/Albums/2016 Italy/",
			RmDir:   "/home/will/images/tablet-2016-05-25/",
		},
		{
			KeepDir: "/home/will/images/storey_albums/Albums/2016 Mexico/",
			RmDir:   "/home/will/images/nexus-5x/",
		},
		{
			KeepDir: "/home/will/images/storey_albums/Baltimore 2010 before alter/",
			RmDir:   "/home/will/images/dad camera backup 2012-10-13/",
		},
		{
			KeepDir: "/home/will/images/storey_albums/2011 Harrison hotsprings before alter/",
			RmDir:   "/home/will/images/dad camera backup 2012-10-13/",
		},
		{
			KeepDir: "/home/will/images/storey_albums/2010 Tofino before alter/",
			RmDir:   "/home/will/images/dad camera backup 2012-10-13/",
		},
		{
			KeepDir: "/home/will/images/storey_albums/2012 Father office photos/",
			RmDir:   "/home/will/images/dad camera backup 2012-10-13/",
		},
		{
			KeepDir: "/home/will/images/storey_albums/Florida 2011 before alter/",
			RmDir:   "/home/will/images/dad camera backup 2012-10-13/",
		},
		{
			KeepDir: "/home/will/images/storey_albums/2011 Block party before alter/",
			RmDir:   "/home/will/images/dad camera backup 2012-10-13/",
		},
		{
			KeepDir: "/home/will/images/storey_albums/2011 Granny birthday before alter/",
			RmDir:   "/home/will/images/dad camera backup 2012-10-13/",
		},
	}

	for _, rule := range rules {
		if dir1 == rule.KeepDir && dir2 == rule.RmDir {
			log.Printf("Deleting %s", file2.Path)
			if live {
				err := os.Remove(file2.Path)
				if err != nil {
					return fmt.Errorf("Unable to remove: %s: %s", file2.Path, err)
				}
			}
			continue
		}
		if dir1 == rule.RmDir && dir2 == rule.KeepDir {
			log.Printf("Deleting %s", file1.Path)
			if live {
				err := os.Remove(file1.Path)
				if err != nil {
					return fmt.Errorf("Unable to remove: %s: %s", file1.Path, err)
				}
			}
			continue
		}
	}

	return nil
}

func (f *File) String() string {
	return fmt.Sprintf("%s %x", f.Path, f.Hash)
}
