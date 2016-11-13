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
	"crypto/sha256"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
)

// Args holds command line arguments.
type Args struct {
	Dir string
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
	err = reportDupes(files)
	if err != nil {
		log.Fatalf("Unable to report duplicates: %s", err)
	}
}

func getArgs() (*Args, error) {
	dir := flag.String("dir", "", "Directory to examine.")

	flag.Parse()

	if len(*dir) == 0 {
		flag.PrintDefaults()
		return nil, fmt.Errorf("You must provide a directory.")
	}

	return &Args{
		Dir: *dir,
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

		hasher := sha256.New()

		n, err := reader.WriteTo(hasher)
		if err != nil {
			return fmt.Errorf("Writing to hash: %s: %s", file.Path, err)
		}

		if n != file.Size {
			return fmt.Errorf("Short read/write: %s", file.Path)
		}

		file.Hash = hasher.Sum(nil)
	}

	return nil
}

func reportDupes(files []*File) error {
	checksumToFile := make(map[[sha256.Size]byte]*File)

	for _, file := range files {
		var checksum [sha256.Size]byte
		for i, b := range file.Hash {
			checksum[i] = b
		}

		log.Printf("%s", file)

		foundFile, ok := checksumToFile[checksum]
		if ok {
			log.Printf("Duplicate found: %s and %s", file.Path, foundFile.Path)
			continue
		}

		checksumToFile[checksum] = file
	}

	return nil
}

func (f *File) String() string {
	return fmt.Sprintf("%s %x", f.Path, f.Hash)
}
