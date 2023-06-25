package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

var mediaDir = "public/media"

var funcs = template.FuncMap{
	"isImage": isImage,
	"isVideo": isVideo,
}

var tmpl *template.Template = template.Must(template.New("").Funcs(funcs).ParseGlob("pages/gallery/*.html"))

type MediaType int64

const (
	Other           = -1
	Image MediaType = iota
	Video
)

type Media struct {
	Type       MediaType
	Name       string
	PublicPath string
	PageLink   string
}

type LinkedMedia struct {
	Cur  Media
	Prev Media
	Next Media
}

type IndexPage struct {
	Users  []string
	Styles template.CSS
}

type UserPage struct {
	User   string
	Years  []string
	Styles template.CSS
}

type YearPage struct {
	Year   string
	User   string
	Images []Media
	Styles template.CSS
}

type PostPage struct {
	Year   string
	User   string
	Image  LinkedMedia
	Styles template.CSS
}

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

func stripExtension(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
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

func readFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return []byte{}, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return bytes, err
	}

	return bytes, nil
}

func getCss(path string) (template.CSS, error) {
	css, err := readFile(path)
	if err != nil {
		return "", err
	}

	globalCss, err := readFile("public/global.css")
	if err != nil {
		return "", err
	}

	return template.CSS(append(css, globalCss...)), nil
}

func getMediaType(filePath string) MediaType {
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

func makeMedia(fileName string, user string, year string) Media {
	return Media{
		Type:       getMediaType(fileName),
		Name:       fileName,
		PublicPath: path.Join("/", "assets", "media", user, year, fileName),
		PageLink:   path.Join("/", "gallery", user, year, stripExtension(fileName)),
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
		return dirNames, err
	}

	// Pick directories only
	for _, entry := range entries {
		if entry.IsDir() {
			dirNames = append(dirNames, entry.Name())
		}
	}

	sort.Slice(dirNames, func(i, j int) bool {
		return dirNames[i] < dirNames[j]
	})

	return dirNames, nil
}

func findImage(user string, year string, fileName string) (LinkedMedia, error) {
	li := LinkedMedia{}

	files, err := os.ReadDir(path.Join(mediaDir, user, year))
	if err != nil {
		return li, err
	}

	for i := 0; i < len(files); i++ {
		f := files[i]

		if stripExtension(f.Name()) != fileName {
			continue
		}

		li.Cur = makeMedia(f.Name(), user, year)

		// Image found

		if len(files) == 1 { // Array length of 1
			return li, nil
		}

		if i == 0 {
			// First item
			li.Next = makeMedia(files[i+1].Name(), user, year)
		} else if i == len(files)-1 {
			// Last item
			li.Prev = makeMedia(files[i-1].Name(), user, year)
		} else {
			// Middle item
			li.Next = makeMedia(files[i+1].Name(), user, year)
			li.Prev = makeMedia(files[i-1].Name(), user, year)
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

	styles, err := getCss("public/gallery/index.css")
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pageData := IndexPage{
		Users:  users,
		Styles: styles,
	}

	err = tmpl.ExecuteTemplate(w, "index.html", pageData)
	if err != nil {
		returnError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func makeUserHandler(user string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		yearsDir := fmt.Sprintf("%s/%s/", mediaDir, user)

		yearsFolder, _ := listDirs(yearsDir)

		if len(yearsFolder) == 0 {
			returnError(w, http.StatusNotFound, "Not Found")
			return
		}

		var years []string
		for _, year := range yearsFolder {
			years = append([]string{strings.TrimPrefix(year, yearsDir)}, years...)
		}

		styles, err := getCss("public/gallery/user.css")
		if err != nil {
			returnError(w, http.StatusInternalServerError, err.Error())
			return
		}

		pageData := UserPage{
			User:   user,
			Years:  years,
			Styles: styles,
		}

		err = tmpl.ExecuteTemplate(w, "user.html", pageData)
		if err != nil {
			returnError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

func makePostHandler(user string, year string, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		li, err := findImage(user, year, id)
		if err != nil {
			returnError(w, http.StatusNotFound, "Not Found")
			return
		}

		styles, err := getCss("public/gallery/post.css")
		if err != nil {
			returnError(w, http.StatusInternalServerError, err.Error())
			return
		}

		post := PostPage{
			Year:   year,
			User:   user,
			Image:  li,
			Styles: styles,
		}

		err = tmpl.ExecuteTemplate(w, "post.html", post)
		if err != nil {
			returnError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

func makeYearHandler(user string, year string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var images []Media

		mediaPath := path.Join(mediaDir, user, year)

		files, err := os.ReadDir(mediaPath)
		if err != nil {
			returnError(w, http.StatusNotFound, "Not Found")
			return
		}

		for _, f := range files {
			if strings.Contains(f.Name(), "100x100") {
				continue
			}

			images = append(images, makeMedia(f.Name(), user, year))
		}

		styles, err := getCss("public/gallery/year.css")
		if err != nil {
			returnError(w, http.StatusInternalServerError, err.Error())
			return
		}

		gallery := YearPage{
			Year:   year,
			User:   user,
			Images: images,
			Styles: styles,
		}

		err = tmpl.ExecuteTemplate(w, "year.html", gallery)
		if err != nil {
			returnError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

func galleryRootHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.FieldsFunc(r.URL.Path, func(c rune) bool {
		return c == '/'
	})

	/*
		 Handle the following routes:
			/
			/{user}
			/{user}/{year}
			/{user}/{year}/{media_id}
	*/

	switch len(parts) {
	case 0:
		indexHandler(w, r)
	case 1:
		makeUserHandler(parts[0])(w, r)
	case 2:
		makeYearHandler(parts[0], parts[1])(w, r)
	case 3:
		makePostHandler(parts[0], parts[1], parts[2])(w, r)
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "hello\n")
}

func main() {
	mux := http.NewServeMux()
	galleryMux := http.NewServeMux()

	// Handle public assets from public directory under example.com/assets URL
	fs := http.FileServer(http.Dir("public"))
	mux.Handle("/assets/", http.StripPrefix("/assets", fs))

	// Configure gallery mux
	galleryMux.HandleFunc("/", galleryRootHandler)

	// Configure main mux
	mux.Handle("/gallery/", http.StripPrefix("/gallery", galleryMux))
	mux.HandleFunc("/", rootHandler)

	address := "localhost:8080"
	fmt.Printf("Listening on %s\n", address)
	log.Fatal(http.ListenAndServe(address, mux))
}
