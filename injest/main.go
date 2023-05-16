package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/nfnt/resize"
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

var rootDir = "instagram"

func main() {
	// Read the JSON file
	file, _ := os.Open("instagram_data/content/posts_1.json")
	defer file.Close()

	decoder := json.NewDecoder(file)
	mediaList := []MediaList{}

	err := decoder.Decode(&mediaList)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, list := range mediaList {
		for _, media := range list.Media {
			// Convert the creation timestamp to a date
			date := time.Unix(media.CreationTimestamp, 0)
			unixTimestamp := strconv.Itoa(int(date.Unix()))

			// Create the directory if it does not exist
			dir := fmt.Sprintf("gallery/%d", date.Year())
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				os.MkdirAll(dir, 0755)
			}

			// Copy the file
			srcPath := filepath.Join(rootDir, media.URI)
			srcFile, err := os.Open(srcPath)
			if err != nil {
				fmt.Println(err)
				continue
			}
			defer srcFile.Close()

			dstPath := filepath.Join(dir, fmt.Sprintf("%s%s", unixTimestamp, path.Ext(media.URI)))
			dstFile, err := os.Create(dstPath)
			if err != nil {
				fmt.Println(err)
				continue
			}
			defer dstFile.Close()

			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				fmt.Println(err)
				continue
			}

			// Generate a thumbnail for images
			if filepath.Ext(media.URI) == ".jpg" {
				srcFile.Seek(0, 0) // reset the read pointer
				img, _, err := image.Decode(srcFile)
				if err != nil {
					fmt.Println(err)
					continue
				}

				thumb := resize.Thumbnail(100, 100, img, resize.Lanczos3)
				thumbPath := filepath.Join(dir, fmt.Sprintf("%s_100x100.jpg", unixTimestamp))
				thumbFile, err := os.Create(thumbPath)
				if err != nil {
					fmt.Println(err)
					continue
				}
				defer thumbFile.Close()

				jpeg.Encode(thumbFile, thumb, &jpeg.Options{Quality: 80})
			}
		}
	}
}
