package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
)

type Gallery struct {
	Year   string
	Images []Image
}

type Image struct {
	Name string
	Path string
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
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))

	address := "localhost:8080"
	fmt.Printf("Listening on %s\n", address)
	http.ListenAndServe(address, r)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	years, _ := filepath.Glob("media/*")
	for i, year := range years {
		years[i] = strings.TrimPrefix(year, "media/")
	}

	t, _ := template.ParseFiles("pages/index.html")
	t.Execute(w, years)
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

	image := Image{Path: path.Join("assets", year, fileName), Name: id}

	t, err := template.ParseFiles("pages/post.html")

	if err != nil {
		fmt.Println(err)
	}

	err = t.Execute(w, image)

	if err != nil {
		fmt.Printf("Error executing template: %s\n", err)
	}
}

func yearHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	year := vars["year"]
	files, _ := filepath.Glob("media/" + year + "/*.jpg")

	var images []Image

	for _, f := range files {
		if strings.Contains(f, "100x100") {
			continue
		}

		images = append(images, Image{Path: filepath.Base(f), Name: getFileName(f)})
	}

	gallery := Gallery{
		Year:   year,
		Images: images,
	}

	t, err := template.ParseFiles("pages/year.html")

	if err != nil {
		fmt.Println(err)
	}

	err = t.Execute(w, gallery)

	if err != nil {
		fmt.Printf("Error executing template: %s\n", err)
	}
}
