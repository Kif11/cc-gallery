package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"
)

type MediaMetadata struct {
	PhotoMetadata struct {
		ExifData []struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"exif_data"`
	} `json:"photo_metadata"`
}

type Media struct {
	URI               string        `json:"uri"`
	CreationTimestamp int64         `json:"creation_timestamp"`
	MediaMetadata     MediaMetadata `json:"media_metadata"`
	Title             string        `json:"title"`
}

type MediaList struct {
	Media []Media `json:"media"`
}

func listDirs(path string) ([]string, error) {
	var dirNames []string

	// Open the directory
	f, err := os.Open(path)
	if err != nil {
		return dirNames, err
	}
	defer f.Close()

	// Read the directory entries
	entries, err := f.Readdir(-1)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		return dirNames, err
	}

	// Iterate over the entries and print the directories
	for _, entry := range entries {
		if entry.IsDir() {
			dirNames = append(dirNames, entry.Name())
		}
	}

	return dirNames, nil
}

var baseDir = "../instagram_data"

func copyFile(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("could not open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("could not create destination file: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("error while copying file: %w", err)
	}

	return nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil { // File exists
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) { // File does not exist
		return false, nil
	}
	// Unknown error
	return false, err
}

func processUserMedia(user string) {
	postsFile := fmt.Sprintf("../instagram_data/%s/content/posts_1.json", user)
	file, _ := os.Open(postsFile)
	defer file.Close()

	decoder := json.NewDecoder(file)
	mediaList := []MediaList{}

	err := decoder.Decode(&mediaList)
	if err != nil {
		fmt.Println(err)
		return
	}

	newMedia := make(map[string][]Media)

	for _, list := range mediaList {
		// Each post can have multiple images and videos
		for idx, media := range list.Media {
			// Convert the creation timestamp to a date
			date := time.Unix(media.CreationTimestamp, 0)
			unixTimestamp := strconv.Itoa(int(date.Unix()))

			// Create the directory if it does not exist
			dir := fmt.Sprintf("media/%s/%d", user, date.Year())
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				os.MkdirAll(dir, 0755)
			}

			srcPath := filepath.Join(baseDir, user, media.URI)
			newFileName := fmt.Sprintf("%s_%d", unixTimestamp, idx)
			dstPath := filepath.Join(dir, fmt.Sprintf("%s%s", newFileName, path.Ext(media.URI)))

			if filepath.Ext(dstPath) == "" {
				dstPath = fmt.Sprintf("%s%s", dstPath, ".mp4")
			}

			exists, err := fileExists(dstPath)
			if err != nil {
				fmt.Println(err)
				return
			}
			if !exists {
				err := copyFile(srcPath, dstPath)
				if err != nil {
					fmt.Println(err)
				}
			}

			key := fmt.Sprintf("%s/%s/%d", "media", user, date.Year())
			newMedia[key] = append(newMedia[key], media)

			// // Generate a thumbnail for images
			// if filepath.Ext(media.URI) == ".jpg" {
			// 	srcFile.Seek(0, 0) // reset the read pointer
			// 	img, _, err := image.Decode(srcFile)
			// 	if err != nil {
			// 		fmt.Println(err)
			// 		continue
			// 	}

			// 	thumb := resize.Thumbnail(100, 100, img, resize.Lanczos3)
			// 	thumbPath := filepath.Join(dir, fmt.Sprintf("%s_100x100.jpg", unixTimestamp))
			// 	thumbFile, err := os.Create(thumbPath)
			// 	if err != nil {
			// 		fmt.Println(err)
			// 		continue
			// 	}
			// 	defer thumbFile.Close()

			// 	jpeg.Encode(thumbFile, thumb, &jpeg.Options{Quality: 80})
			// }
		}

		for path, media := range newMedia {
			file, err := json.MarshalIndent(media, "", " ")
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			err = ioutil.WriteFile(fmt.Sprintf("%s.json", path), file, 0644)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}
}

func main() {
	users, err := listDirs(baseDir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, user := range users {
		processUserMedia(user)
	}
}
