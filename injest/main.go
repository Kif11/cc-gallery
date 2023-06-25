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

type IgTvMedia struct {
	IgTvMedia []MediaList `json:"ig_igtv_media"`
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

var srcMetadataDir = "/Users/kif/pr/instagram_data"
var destinationDir = "/Users/kif/pr/gallery2/public/media"

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

func readJson(jsonFile string, target any) error {
	file, _ := os.Open(jsonFile)
	defer file.Close()

	decoder := json.NewDecoder(file)

	err := decoder.Decode(target)
	if err != nil {
		return err
	}

	return nil
}

func processUserMedia(user string) {
	// Read posts metadata
	postsFile := fmt.Sprintf("%s/%s/content/posts_1.json", srcMetadataDir, user)
	mediaList := []MediaList{}

	err := readJson(postsFile, &mediaList)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Read IgTv metadata
	igTvFile := fmt.Sprintf("%s/%s/content/igtv_videos.json", srcMetadataDir, user)
	igTvList := IgTvMedia{}

	err = readJson(igTvFile, &igTvList)
	if err != nil {
		fmt.Println(err)
		return
	}

	allMediaList := append(mediaList, igTvList.IgTvMedia...)
	newMedia := make(map[string][]Media)

	for _, list := range allMediaList {
		// Each post can have multiple images and videos
		for idx, media := range list.Media {
			// Convert the creation timestamp to a date
			date := time.Unix(media.CreationTimestamp, 0)
			unixTimestamp := strconv.Itoa(int(date.Unix()))

			// Create the directory if it does not exist
			dir := fmt.Sprintf("%s/%s/%d", destinationDir, user, date.Year())
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				os.MkdirAll(dir, 0755)
			}

			srcPath := filepath.Join(srcMetadataDir, user, media.URI)
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
	users, err := listDirs(srcMetadataDir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, user := range users {
		processUserMedia(user)
	}
}
