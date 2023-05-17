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

	"github.com/gorilla/mux"
)

//go:embed pages/*.html
var content embed.FS

var mediaDir = "./media"

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

type Gallery struct {
	Year   string
	Images []Image
}

type Image struct {
	Name string
}

type LinkedImage struct {
	Cur  Image
	Prev Image
	Next Image
}

type Post struct {
	Year  string
	Image LinkedImage
}

func returnError(w http.ResponseWriter, header int, msg string) {
	w.WriteHeader(header)
	w.Write([]byte(msg))
}

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/gallery/", indexHandler)
	r.HandleFunc("/gallery/{year}", yearHandler)
	r.HandleFunc("/gallery/{year}/{id}", postHandler)

	fs := http.FileServer(http.Dir(mediaDir))
	r.PathPrefix("/gallery/assets/").Handler(http.StripPrefix("/gallery/assets/", fs))

	address := "localhost:8080"
	fmt.Printf("Listening on %s\n", address)
	http.ListenAndServe(address, r)
}

func findImage(year string, fileName string) (LinkedImage, error) {
	li := LinkedImage{}

	files, err := os.ReadDir(path.Join(mediaDir, year))
	if err != nil {
		return li, nil
	}

	for i := 0; i < len(files); i++ {
		f := files[i]

		if getFileName(f.Name()) != fileName {
			continue
		}

		li.Cur = Image{Name: f.Name()}

		// Image found

		if len(files) == 1 { // Array length of 1
			return li, nil
		}

		if i == 0 {
			// First item
			li.Next = Image{Name: files[i+1].Name()}
		} else if i == len(files)-1 {
			// Last item
			li.Prev = Image{Name: files[i-1].Name()}
		} else {
			// Middle item
			li.Next = Image{Name: files[i+1].Name()}
			li.Prev = Image{Name: files[i-1].Name()}
		}

		return li, nil
	}

	return li, fmt.Errorf("image with id %s not found", fileName)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	years, _ := filepath.Glob("media/*")
	for i, year := range years {
		years[i] = strings.TrimPrefix(year, "media/")
	}

	err := tmpl.ExecuteTemplate(w, "index.html", years)
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	year := vars["year"]

	li, err := findImage(year, id)
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}

	post := Post{
		Year:  year,
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
	var images []Image

	files, err := os.ReadDir(path.Join(mediaDir, year))
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, f := range files {
		if strings.Contains(f.Name(), "100x100") {
			continue
		}

		images = append(images, Image{Name: f.Name()})
	}

	gallery := Gallery{
		Year:   year,
		Images: images,
	}

	err = tmpl.ExecuteTemplate(w, "year.html", gallery)
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}
}
