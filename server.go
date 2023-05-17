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

var funcs = template.FuncMap{
	"hasSuffix": strings.HasSuffix,
}

var tmpl *template.Template = template.Must(template.New("").Funcs(funcs).ParseFS(content, "pages/*.html"))

type Gallery struct {
	Year   string
	Images []Image
}

type Image struct {
	Name string
	Ext  string
	Year string
}

func returnError(w http.ResponseWriter, header int, msg string) {
	w.WriteHeader(header)
	w.Write([]byte(msg))
}

func getFileName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/gallery/", indexHandler)
	r.HandleFunc("/gallery/{year}", yearHandler)
	r.HandleFunc("/gallery/{year}/{id}", postHandler)

	fs := http.FileServer(http.Dir("./media/"))
	r.PathPrefix("/gallery/assets/").Handler(http.StripPrefix("/gallery/assets/", fs))

	address := "localhost:8080"
	fmt.Printf("Listening on %s\n", address)
	http.ListenAndServe(address, r)
}

func findFile(dir string, fileName string) (string, error) {
	var foundPath string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) (string error) {
		if err != nil {
			return err
		}

		if !info.IsDir() && getFileName(info.Name()) == fileName {
			foundPath = info.Name()
			return nil
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return foundPath, nil
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
	dir := fmt.Sprintf("./media/%s", year)

	fileName, err := findFile(dir, id)
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}

	image := Image{Ext: path.Ext(fileName), Name: id, Year: year}

	err = tmpl.ExecuteTemplate(w, "post.html", image)
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func yearHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	year := vars["year"]
	var images []Image

	files, err := os.ReadDir(path.Join("./media", year))
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, f := range files {
		if strings.Contains(f.Name(), "100x100") {
			continue
		}

		images = append(images, Image{Ext: path.Ext(f.Name()), Name: getFileName(f.Name()), Year: year})
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
