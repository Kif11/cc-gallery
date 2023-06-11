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
//go:embed public/*.css
var content embed.FS

var mediaDir = "public/media"

var funcs = template.FuncMap{
	"isImage": isImage,
	"isVideo": isVideo,
}

var tmpl *template.Template = template.Must(template.New("").Funcs(funcs).ParseFS(content, "pages/*.html"))

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

func getCss(path string) (template.CSS, error) {
	css, err := content.ReadFile(path)
	if err != nil {
		return "", err
	}

	globalCss, err := content.ReadFile("public/global.css")
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
		PublicPath: path.Join("/", "gallery", "assets", "media", user, year, fileName),
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

func findImage(user string, year string, fileName string) (LinkedMedia, error) {
	li := LinkedMedia{}

	files, err := os.ReadDir(path.Join(mediaDir, user, year))
	if err != nil {
		return li, nil
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

	styles, err := getCss("public/index.css")
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

func userHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	user := vars["user"]

	yearsDir := fmt.Sprintf("%s/%s/", mediaDir, user)
	globPath := fmt.Sprintf("%s*", yearsDir)
	yearsFolder, _ := filepath.Glob(globPath)
	var years []string
	for _, year := range yearsFolder {
		years = append([]string{strings.TrimPrefix(year, yearsDir)}, years...)
	}

	styles, err := getCss("public/user.css")
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

	styles, err := getCss("public/post.css")
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

		images = append(images, makeMedia(f.Name(), user, year))
	}

	styles, err := getCss("public/year.css")
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

func main() {
	r := mux.NewRouter()

	fs := http.FileServer(http.Dir("public"))
	r.PathPrefix("/gallery/assets/").Handler(http.StripPrefix("/gallery/assets/", fs))

	r.HandleFunc("/gallery", indexHandler)
	r.HandleFunc("/gallery/{user}", userHandler)
	r.HandleFunc("/gallery/{user}/{year}", yearHandler)
	r.HandleFunc("/gallery/{user}/{year}/{id}", postHandler)

	address := "localhost:8080"
	fmt.Printf("Listening on %s\n", address)
	http.ListenAndServe(address, trimTrailingSlash(r))
}
