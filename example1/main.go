package main

import (
	"fmt"
	"io/fs"
	"os"
	"testing/fstest"
)

func readDirRecursive(fsys fs.FS, dir string) error {
	files, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		// Construct the full file path
		path := dir + "/" + file.Name()

		if file.IsDir() {
			// If it is a directory, read it recursively
			err := readDirRecursive(fsys, path)
			if err != nil {
				return err
			}
		} else {
			// This is a file, handle it
			fmt.Println("File:", path)
		}
	}

	return nil
}

func main() {
	fsys := fstest.MapFS{
		"dir/file":         &fstest.MapFile{},
		"dir/subdir/file2": &fstest.MapFile{},
		// Add more files/directories here...
	}

	items, err := fs.ReadDir(fsys, ".")
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	for _, file := range items {
		fmt.Println(file)
	}
}
