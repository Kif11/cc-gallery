package main

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"bytes"
	"encoding/base64"
	"image/jpeg"

	"github.com/disintegration/imaging"
	"github.com/gorilla/mux"
)

//go:embed pages/*.html
var content embed.FS

var mediaDir = "media"

func isImage(file string) bool {
	ext := strings.ToLower(path.Ext(file))
	if ext == ".jpg" || ext == ".webp" || ext == ".png" || ext == ".gif" {
		return true
	}

	return false
}

func isVideo(file string) bool {
	ext := strings.ToLower(path.Ext(file))
	if ext == ".mp4" || ext == ".mov" {
		return true
	}

	return false
}

func getFileName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

var funcs = template.FuncMap{
	"isImage":     isImage,
	"isVideo":     isVideo,
	"getFileName": getFileName,
}

var tmpl *template.Template = template.Must(template.New("").Funcs(funcs).ParseFS(content, "pages/*.html"))

func GenerateThumbnail(imagePath string) (string, error) {
	// Open the image file
	srcImage, err := imaging.Open(imagePath)
	if err != nil {
		return "", err
	}

	// Resize the image to 100x100px while preserving aspect ratio
	resizedImage := imaging.Resize(srcImage, 100, 100, imaging.Lanczos)

	// Blur the image
	blurredImage := imaging.Blur(resizedImage, 0.5)

	// Create a buffer to save the image
	var buf bytes.Buffer

	// Write the image to the buffer in JPEG format
	err = jpeg.Encode(&buf, blurredImage, nil)
	if err != nil {
		return "", err
	}

	// Get the byte slice from the buffer
	bytes := buf.Bytes()

	// Base64 encode the byte slice
	base64String := base64.StdEncoding.EncodeToString(bytes)

	// Return the base64 encoded image
	return base64String, nil
}

type Gallery struct {
	Year   string
	User   string
	Images []Media
}

type MediaType int64

const (
	Other           = -1
	Image MediaType = iota
	Video
)

type Media struct {
	Type      MediaType
	Name      string
	Thumbnail string // base64
}

type LinkedImage struct {
	Cur  Media
	Prev Media
	Next Media
}

type Post struct {
	Year  string
	User  string
	Image LinkedImage
}

type IndexPage struct {
	Users []string
}

type UserPage struct {
	User  string
	Years []string
}

func returnError(w http.ResponseWriter, header int, msg string) {
	w.WriteHeader(header)
	w.Write([]byte(msg))
}

func trimTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
		next.ServeHTTP(w, r)
	})
}

func GetMediaType(filePath string) MediaType {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
		return Image
	case ".mp4", ".mov", ".webm":
		return Video
	default:
		return Other
	}
}

func makeMedia(name string) Media {
	return Media{
		Type: GetMediaType(name),
		Name: name,
	}
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

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/gallery", indexHandler)
	r.HandleFunc("/gallery/{user}", userHandler)
	r.HandleFunc("/gallery/{user}/{year}", yearHandler)
	r.HandleFunc("/gallery/{user}/{year}/{id}", postHandler)

	fs := http.FileServer(http.Dir(mediaDir))
	r.PathPrefix("/gallery/assets/").Handler(http.StripPrefix("/gallery/assets/", fs))

	address := "localhost:8080"
	fmt.Printf("Listening on %s\n", address)
	http.ListenAndServe(address, trimTrailingSlash(r))
}

func findImage(user string, year string, fileName string) (LinkedImage, error) {
	li := LinkedImage{}

	files, err := os.ReadDir(path.Join(mediaDir, user, year))
	if err != nil {
		return li, nil
	}

	for i := 0; i < len(files); i++ {
		f := files[i]

		if getFileName(f.Name()) != fileName {
			continue
		}

		li.Cur = Media{Name: f.Name()}

		// Image found

		if len(files) == 1 { // Array length of 1
			return li, nil
		}

		if i == 0 {
			// First item
			li.Next = makeMedia(files[i+1].Name())
		} else if i == len(files)-1 {
			// Last item
			li.Prev = makeMedia(files[i-1].Name())
		} else {
			// Middle item
			li.Next = makeMedia(files[i+1].Name())
			li.Prev = makeMedia(files[i-1].Name())
		}

		return li, nil
	}

	return li, fmt.Errorf("image with id %s not found", fileName)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	users, err := listDirs(mediaDir)
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i, user := range users {
		users[i] = strings.TrimPrefix(user, mediaDir)
	}

	pageData := IndexPage{
		Users: users,
	}

	err = tmpl.ExecuteTemplate(w, "index.html", pageData)
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	user := vars["user"]

	yearsDir := fmt.Sprintf("%s/%s/", mediaDir, user)
	globPath := fmt.Sprintf("%s*", yearsDir)
	years, _ := filepath.Glob(globPath)
	for i, year := range years {
		years[i] = strings.TrimPrefix(year, yearsDir)
	}

	pageData := UserPage{
		User:  user,
		Years: years,
	}

	err := tmpl.ExecuteTemplate(w, "user.html", pageData)
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	year := vars["year"]
	user := vars["user"]

	li, err := findImage(user, year, id)
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// base64, err = GenerateThumbnail(li.Cur)
	// if err != nil {
	// 	returnError(w, http.StatusInternalServerError, err.Error())
	// 	return
	// }

	post := Post{
		Year:  year,
		User:  user,
		Image: li,
	}

	err = tmpl.ExecuteTemplate(w, "post.html", post)
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func yearHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	year := vars["year"]
	user := vars["user"]
	var images []Media

	files, err := os.ReadDir(path.Join(mediaDir, user, year))
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, f := range files {
		if strings.Contains(f.Name(), "100x100") {
			continue
		}

		images = append(images, Media{Name: f.Name()})
	}

	gallery := Gallery{
		Year:   year,
		User:   user,
		Images: images,
	}

	err = tmpl.ExecuteTemplate(w, "year.html", gallery)
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}
}
