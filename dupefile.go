package main

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
)

// Args holds command line arguments.
type Args struct {
	Dir    string
	Config string
	Live   bool
}

// File holds information about one file.
type File struct {
	Basename string
	Path     string
	Size     int64
	Hash     []byte
}

// Rule defines what to do with a duplicate file found in two directories.
type Rule struct {
	KeepDir   string `json:"keep"`
	RemoveDir string `json:"remove"`
}

func main() {
	log.SetFlags(0)

	args, err := getArgs()
	if err != nil {
		log.Fatalf("Error: %s", err)
	}

	rules, err := readRules(args.Config)
	if err != nil {
		log.Fatalf("Unable to read rules from config: %s: %s", args.Config, err)
	}

	log.Print("Looking for files...")
	files, err := findFiles(args.Dir)
	if err != nil {
		log.Fatalf("Unable to find files: %s", err)
	}

	if len(files) == 0 {
		log.Printf("No files found.")
	}

	log.Print("Calculating checksums...")
	if err := calculateChecksums(files); err != nil {
		log.Fatalf("Unable to calculate checksums: %s", err)
	}

	log.Print("Reporting/resolving duplicate files...")
	if err := reportAndResolveDuplicates(rules, files, args.Live); err != nil {
		log.Fatalf("Unable to report/resolve duplicates: %s", err)
	}
}

func getArgs() (*Args, error) {
	dir := flag.String("dir", "", "Directory to examine.")
	config := flag.String("conf", "", "Path to a configuration file.")
	live := flag.Bool("live", false, "Enable file deletion.")

	flag.Parse()

	if len(*dir) == 0 {
		flag.PrintDefaults()
		return nil, fmt.Errorf("you must provide a directory")
	}

	if len(*config) == 0 {
		flag.PrintDefaults()
		return nil, fmt.Errorf("you must provide a configuration file")
	}

	return &Args{
		Dir:    *dir,
		Config: *config,
		Live:   *live,
	}, nil
}

func readRules(configFile string) ([]Rule, error) {
	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read config: %s", err)
	}

	type Config struct {
		Rules []Rule
	}

	config := Config{}
	if err := json.Unmarshal(buf, &config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %s", err)
	}

	if len(config.Rules) == 0 {
		return nil, fmt.Errorf("no rules found")
	}

	for i, rule := range config.Rules {
		if len(rule.KeepDir) == 0 || len(rule.RemoveDir) == 0 {
			return nil, fmt.Errorf("rule %d is missing keep/remove directory", i+i)
		}
		if rule.KeepDir[0] != '/' || rule.RemoveDir[0] != '/' {
			return nil,
				fmt.Errorf("rule %d is has non-absolute keep/remove directory", i+i)
		}
		if rule.KeepDir == rule.RemoveDir {
			return nil,
				fmt.Errorf("rule %d is has identical keep/remove directory", i+i)
		}
	}

	return config.Rules, nil
}

func findFiles(dir string) ([]*File, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("stat: %s: %s", dir, err)
	}

	if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	dh, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("open: %s: %s", dir, err)
	}

	fis, err := dh.Readdir(0)
	if err != nil {
		_ = dh.Close()
		return nil, fmt.Errorf("readdir: %s: %s", dir, err)
	}

	if err := dh.Close(); err != nil {
		return nil, fmt.Errorf("close: %s: %s", dir, err)
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
	fileCount := len(files)

	for i, file := range files {
		fh, err := os.Open(file.Path)
		if err != nil {
			return fmt.Errorf("open: %s: %s", file.Path, err)
		}

		reader := bufio.NewReader(fh)

		hasher := md5.New()

		n, err := reader.WriteTo(hasher)
		if err != nil {
			_ = fh.Close()
			return fmt.Errorf("writing to hash failed: %s: %s", file.Path, err)
		}

		if n != file.Size {
			_ = fh.Close()
			return fmt.Errorf("short read/write: %s", file.Path)
		}

		file.Hash = hasher.Sum(nil)

		if err := fh.Close(); err != nil {
			return fmt.Errorf("close: %s: %s", file.Path, err)
		}

		fmt.Printf("\r%d/%d", i+1, fileCount)
	}

	// Complete the status/count line.
	fmt.Printf("\n")

	return nil
}

func reportAndResolveDuplicates(rules []Rule, files []*File, live bool) error {
	checksumToFile := make(map[[md5.Size]byte]*File)

	for _, file := range files {
		// Make a []byte array with a defined size for a key lookup.
		var checksum [md5.Size]byte
		for i, b := range file.Hash {
			checksum[i] = b
		}

		// Is this a possible duplicate? We can tell by whether we've seen a file
		// with the same checksum yet.
		foundFile, ok := checksumToFile[checksum]
		if !ok {
			checksumToFile[checksum] = file
			continue
		}

		// Hash collision. Deep compare to determine whether the files are really
		// the same.
		identical, err := isIdentical(foundFile, file)
		if err != nil {
			return fmt.Errorf("unable to compare files: %s %s: %s", foundFile.Path,
				file.Path, err)
		}
		if !identical {
			return fmt.Errorf(
				"hash collision but the files are not identical! %s and %s",
				file.Path, foundFile.Path)
		}

		log.Printf("Duplicate files found: %s and %s", file.Path, foundFile.Path)
		foundRule, err := resolveDuplicate(rules, foundFile, file, live)
		if err != nil {
			return err
		}

		if !foundRule {
			log.Printf("No rule found for duplicate files: %s and %s", file.Path,
				foundFile.Path)
		}
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
		return nil, fmt.Errorf("open: %s: %s", file, err)
	}

	contents, err := ioutil.ReadAll(fh)
	if err != nil {
		_ = fh.Close()
		return nil, fmt.Errorf("failed ReadAll: %s: %s", file.Path, err)
	}

	if err := fh.Close(); err != nil {
		return nil, fmt.Errorf("close: %s: %s", file.Path, err)
	}

	if int64(len(contents)) != file.Size {
		return nil, fmt.Errorf("short read: %s", file.Path)
	}

	return contents, nil
}

// Return whether there was a rule. If there was a rule (true), then return
// whether there was an error. Not having a rule is not an error (because we may
// want to just report).
func resolveDuplicate(
	rules []Rule,
	file1,
	file2 *File,
	live bool,
) (bool, error) {
	dir1, _ := path.Split(file1.Path)
	dir2, _ := path.Split(file2.Path)

	for _, rule := range rules {
		if dir1 == rule.KeepDir && dir2 == rule.RemoveDir {
			if live {
				log.Printf("Deleting %s", file2.Path)
				if err := os.Remove(file2.Path); err != nil {
					return true, fmt.Errorf("unable to remove: %s: %s", file2.Path, err)
				}
			} else {
				log.Printf("Non-live mode. Would delete %s", file2.Path)
			}
			return true, nil
		}

		if dir1 == rule.RemoveDir && dir2 == rule.KeepDir {
			if live {
				log.Printf("Deleting %s", file1.Path)
				if err := os.Remove(file1.Path); err != nil {
					return true, fmt.Errorf("unable to remove: %s: %s", file1.Path, err)
				}
			} else {
				log.Printf("Non-live mode. Would delete %s", file2.Path)
			}

			return true, nil
		}
	}

	return false, nil
}

func (f *File) String() string {
	return fmt.Sprintf("%s %x", f.Path, f.Hash)
}
