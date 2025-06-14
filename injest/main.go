package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type PhotoMetadata struct {
	ExifData []struct {
		ISO               int    `json:"iso"`
		FocalLength       string `json:"focal_length"`
		LensModel         string `json:"lens_model"`
		SceneCaptureType  string `json:"scene_capture_type"`
		Software          string `json:"software"`
		DeviceID          string `json:"device_id"`
		SceneType         int    `json:"scene_type"`
		CameraPosition    string `json:"camera_position"`
		LensMake          string `json:"lens_make"`
		DateTimeDigitized string `json:"date_time_digitized"`
		DateTimeOriginal  string `json:"date_time_original"`
		SourceType        string `json:"source_type"`
		Aperture          string `json:"aperture"`
		ShutterSpeed      string `json:"shutter_speed"`
		MeteringMode      string `json:"metering_mode"`
	} `json:"exif_data"`
}

type VideoMetadata struct {
	ExifData []struct {
		DeviceID         string `json:"device_id"`
		DateTimeOriginal string `json:"date_time_original"`
		SourceType       string `json:"source_type"`
	} `json:"exif_data"`
}

type MediaMetadata struct {
	PhotoMetadata struct {
		ExifData []struct {
			Latitude      float64 `json:"latitude"`
			Longitude     float64 `json:"longitude"`
			PhotoMetadata `json:"photo_metadata,omitempty"`
			VideoMetadata `json:"video_metadata,omitempty"`
		} `json:"exif_data"`
	} `json:"photo_metadata"`
}

type InstType string

const (
	Post  InstType = "post"
	IgTv  InstType = "igtv"
	Story InstType = "story"
	Reel  InstType = "reel"
)

type Media struct {
	URI               string        `json:"uri"`
	CreationTimestamp int64         `json:"creation_timestamp"`
	MediaMetadata     MediaMetadata `json:"media_metadata"`
	Title             string        `json:"title"`
	Index             int           // In case of multiple media per post represent the index of the media
	Type              InstType
	User              string
}

type MediaList struct {
	Media []Media `json:"media"`
}

type IgTvMedia struct {
	IgTvMedia []MediaList `json:"ig_igtv_media"`
}

type Stories struct {
	IGStories []Media `json:"ig_stories"`
}

type Reels struct {
	IGReels []MediaList `json:"ig_reels_media"`
}

func listDirs(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		return []string{}, err
	}

	var dirNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirNames = append(dirNames, entry.Name())
		}
	}

	return dirNames, nil
}

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

// Get file name without extension
func fileName(path string) string {
	fileName := filepath.Base(path)
	extension := filepath.Ext(fileName)

	if strings.HasPrefix(fileName, ".") {
		extension = filepath.Ext(strings.TrimPrefix(fileName, "."))
	}

	return fileName[:len(fileName)-len(extension)]
}

func readJson(jsonFile string, target any) error {
	file, err := os.Open(jsonFile)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	err = decoder.Decode(target)
	if err != nil {
		return err
	}

	return nil
}

func cleanPath(path string) string {
	switch filepath.Ext(path) {
	case "":
		// Instagram archive contains h264 encoded mp4 files with no extensions
		return fmt.Sprintf("%s%s", path, ".mp4")
	case ".heic":
		// Instagram archive contains images with .heic extension that just JPGs
		return fmt.Sprintf("%s/%s%s", filepath.Dir(path), fileName(path), ".jpg")
	}

	return path
}

func makeDstPath(media Media, destinationDir string) string {
	date := time.Unix(media.CreationTimestamp, 0)
	unixTimestamp := strconv.Itoa(int(date.Unix()))
	fileName := fmt.Sprintf("%s_%s_%d", media.Type, unixTimestamp, media.Index)

	// Create the directory if it does not exist
	dir := fmt.Sprintf("%s/%s/%d", destinationDir, media.User, date.Year())
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}

	return cleanPath(
		filepath.Join(
			dir,
			fmt.Sprintf("%s%s", fileName, path.Ext(media.URI))),
	)
}

func hydrateMedia(media []Media, mediaType InstType, user string) []Media {
	newMedia := []Media{}
	for idx, v := range media {
		v.Type = mediaType
		v.Index = idx
		v.User = user
		newMedia = append(newMedia, v)
	}
	return newMedia
}

func processUserMedia(user string, srcDir string, dstDir string) error {
	// Read posts metadata
	postsFile := fmt.Sprintf("%s/%s/content/posts_1.json", srcDir, user)
	mediaList := []MediaList{}
	allMedia := []Media{}

	err := readJson(postsFile, &mediaList)
	if err != nil {
		fmt.Printf("error reading posts metadata: %s. Skipping\n", err)
	}

	// Read IgTv metadata
	igTvFile := fmt.Sprintf("%s/%s/content/igtv_videos.json", srcDir, user)
	igTvList := IgTvMedia{}

	err = readJson(igTvFile, &igTvList)
	if err != nil {
		fmt.Printf("error reading igtv metadata: %s. Skipping\n", err)
	}

	// Process stories
	storiesFile := fmt.Sprintf("%s/%s/content/stories.json", srcDir, user)
	stories := Stories{}

	err = readJson(storiesFile, &stories)
	if err != nil {
		fmt.Printf("error reading stories metadata: %s. Skipping\n", err)
	}

	// Process reels
	reelsFile := fmt.Sprintf("%s/%s/content/reels.json", srcDir, user)
	reels := Reels{}

	err = readJson(reelsFile, &reels)
	if err != nil {
		fmt.Printf("error reading reels metadata: %s. Skipping\n", err)
	}

	// Append post
	for _, m := range mediaList {
		allMedia = append(allMedia, hydrateMedia(m.Media, Post, user)...)
	}

	// Append igTv
	for _, m := range igTvList.IgTvMedia {
		allMedia = append(allMedia, hydrateMedia(m.Media, IgTv, user)...)
	}

	// Append stories
	allMedia = append(allMedia, hydrateMedia(stories.IGStories, Story, user)...)

	// Append reels
	for _, m := range reels.IGReels {
		allMedia = append(allMedia, hydrateMedia(m.Media, Reel, user)...)
	}

	// // Debug
	// for i, v := range allMedia {
	// 	if i < 100 {
	// 		fmt.Printf("Idx: %d, %+v\n", i, v)
	// 	}
	// }
	// os.Exit(0)

	newMedia := make(map[string][]Media)

	// Each post can have multiple images and videos
	for _, media := range allMedia {
		srcPath := filepath.Join(srcDir, user, media.URI)
		dstPath := makeDstPath(media, dstDir)

		exists, err := fileExists(dstPath)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if exists {
			continue
		}

		err = copyFile(srcPath, dstPath)
		if err != nil {
			fmt.Println("Error copying file:", err)
		}

		newMedia[dstPath] = append(newMedia[dstPath], media)
	}

	return nil
}

func main() {

	if len(os.Args) < 3 {
		fmt.Printf("Usage: ingest <instagram_data_dir> <destination_dir>\n")
		os.Exit(0)
	}

	srcMetadataDir := os.Args[1]
	destinationDir := os.Args[2]

	users, err := listDirs(srcMetadataDir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Users found:", users)

	for _, user := range users {
		err := processUserMedia(user, srcMetadataDir, destinationDir)
		if err != nil {
			fmt.Printf("Error processing user %s: %v\n", user, err)
			continue
		}
	}
}
