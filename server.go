package main

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
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

type MediaFileType int64

const (
	Other               = -1
	Image MediaFileType = iota
	Video
)

type Media struct {
	Type       MediaFileType
	Name       string
	PublicPath string
	PageLink   string
}

type LinkedMedia struct {
	Cur  Media
	Prev Media
	Next Media
}

type AlbumsPage struct {
	Path     string
	Albums   []string
	BackLink string
	Styles   template.CSS
}

type GalleryPage struct {
	Path         string
	Images       []Media
	BackLink     string
	Styles       template.CSS
	PageSettings PageSettings
}

type PlayerPage struct {
	Path     string
	Image    LinkedMedia
	BackLink string
	Styles   template.CSS
}

const (
	NewFirst MediaOrder = "new_first"
	OldFirst MediaOrder = "old_first"
)

type MediaOrder string

type PageSettings struct {
	Path       string
	GridSize   string
	MediaOrder MediaOrder
}

var pageSettings = []PageSettings{
	{
		Path:       "kif",
		GridSize:   "300px",
		MediaOrder: NewFirst,
	},
	{
		Path:       "snay",
		GridSize:   "200px",
		MediaOrder: NewFirst,
	},
}

func pageSettingsForPath(p string, settings []PageSettings) PageSettings {
	match := PageSettings{
		GridSize:   "300px",
		MediaOrder: NewFirst,
	}

	for _, s := range settings {
		if strings.HasPrefix(p, s.Path) {
			match = s
		}
	}

	return match
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

func writeError(w http.ResponseWriter, header int, msg string) {
	w.WriteHeader(header)
	w.Write([]byte(msg))
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

func getMediaType(filePath string) MediaFileType {
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

func sortMedia(files []fs.DirEntry, order MediaOrder) []fs.DirEntry {
	sort.Slice(files, func(i, j int) bool {
		// Extract time stamp from file names
		parts1 := strings.Split(files[i].Name(), "_")
		parts2 := strings.Split(files[j].Name(), "_")

		if len(parts1) < 2 || len(parts2) < 2 {
			return files[i].Name() < files[j].Name()
		}

		switch order {
		case NewFirst:
			return parts1[1] > parts2[1]
		case OldFirst:
			return parts1[1] < parts2[1]
		default:
			return parts1[1] > parts2[1]
		}
	})

	return files
}

func makeMedia(fileName string, subPath string) Media {
	return Media{
		Type:       getMediaType(fileName),
		Name:       fileName,
		PublicPath: path.Join("/", "public", "media", subPath, fileName),
		PageLink:   path.Join("/", "gallery", subPath, fileName),
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

func findImage(subPath string) (LinkedMedia, error) {
	li := LinkedMedia{}
	fileName := filepath.Base(subPath)
	dir := path.Dir(subPath)

	files, err := os.ReadDir(path.Join(mediaDir, dir))
	if err != nil {
		return li, err
	}

	settings := pageSettingsForPath(subPath, pageSettings)
	sortedMedia := sortMedia(files, settings.MediaOrder)

	for i := 0; i < len(sortedMedia); i++ {
		f := sortedMedia[i]

		if f.Name() != fileName {
			continue
		}

		li.Cur = makeMedia(f.Name(), dir)

		// Image found

		if len(files) == 1 { // Array length of 1
			return li, nil
		}

		if i == 0 {
			// First item
			li.Next = makeMedia(files[i+1].Name(), dir)
		} else if i == len(files)-1 {
			// Last item
			li.Prev = makeMedia(files[i-1].Name(), dir)
		} else {
			// Middle item
			li.Next = makeMedia(files[i+1].Name(), dir)
			li.Prev = makeMedia(files[i-1].Name(), dir)
		}

		return li, nil
	}

	return li, fmt.Errorf("image with id %s not found", fileName)
}

// albumsHandler renders folders in given directory as albums
func albumsHandler(subPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dir := fmt.Sprintf("%s/%s/", mediaDir, subPath)

		subFolders, _ := listDirs(dir)

		if len(subFolders) == 0 {
			writeError(w, http.StatusNotFound, "Not Found")
			return
		}

		var albums []string
		for _, year := range subFolders {
			albums = append([]string{strings.TrimPrefix(year, dir)}, albums...)
		}

		styles, err := getCss("public/gallery/album.css")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		pageData := AlbumsPage{
			Path:     subPath,
			Albums:   albums,
			BackLink: path.Dir(subPath),
			Styles:   styles,
		}

		err = tmpl.ExecuteTemplate(w, "album.html", pageData)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

// playerHandler render individual media on it's own page
func playerHandler(uri string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		li, err := findImage(uri)
		if err != nil {
			writeError(w, http.StatusNotFound, "Not Found")
			return
		}

		styles, err := getCss("public/gallery/player.css")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		post := PlayerPage{
			Path:     uri,
			Image:    li,
			BackLink: path.Dir(uri),
			Styles:   styles,
		}

		err = tmpl.ExecuteTemplate(w, "player.html", post)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

// galleryHandler renders folder with images as a gallery
func galleryHandler(subPath string, filter string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var images []Media
		settings := pageSettingsForPath(subPath, pageSettings)

		mediaPath := path.Join(mediaDir, subPath)

		files, err := os.ReadDir(mediaPath)
		if err != nil {
			writeError(w, http.StatusNotFound, "Not Found")
			return
		}

		sortedMedia := sortMedia(files, settings.MediaOrder)

		for _, f := range sortedMedia {
			if filter == "" {
				images = append(images, makeMedia(f.Name(), subPath))
				continue
			}

			if strings.Contains(f.Name(), filter) {
				images = append(images, makeMedia(f.Name(), subPath))
			}
		}

		styles, err := getCss("public/gallery/gallery.css")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		gallery := GalleryPage{
			Path:         subPath,
			Images:       images,
			BackLink:     path.Dir(subPath),
			Styles:       styles,
			PageSettings: pageSettingsForPath(subPath, pageSettings),
		}

		err = tmpl.ExecuteTemplate(w, "gallery.html", gallery)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

func galleryRootHandler(w http.ResponseWriter, r *http.Request) {

	p := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/"), "/")

	fullPath := path.Join(mediaDir, p)

	stat, err := os.Stat(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}

	if stat.IsDir() {
		content, err := os.ReadDir(fullPath)
		if err != nil {
			panic(err)
		}

		dirs := []fs.DirEntry{}
		files := []fs.DirEntry{}
		for _, c := range content {
			if c.IsDir() {
				dirs = append(dirs, c)
				continue
			}
			files = append(files, c)
		}

		if len(dirs) > 0 {
			albumsHandler(p)(w, r)
			return
		}

		if len(files) > 0 {
			mediaType := r.URL.Query().Get("filter")
			galleryHandler(p, mediaType)(w, r)
			return
		}

		writeError(w, http.StatusNotFound, "Not Found")
		return
	}

	playerHandler(p)(w, r)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "🐈\n")
}

func main() {
	mux := http.NewServeMux()
	galleryMux := http.NewServeMux()

	// Handle public assets from public directory under example.com/assets URL
	fs := http.FileServer(http.Dir("public"))
	mux.Handle("/public/", http.StripPrefix("/public", fs))

	// Configure gallery mux
	galleryMux.HandleFunc("/", galleryRootHandler)

	// Configure main mux
	mux.Handle("/gallery/", http.StripPrefix("/gallery", galleryMux))
	mux.HandleFunc("/", rootHandler)

	address := "localhost:8080"
	fmt.Printf("Listening on %s\n", address)
	log.Fatal(http.ListenAndServe(address, mux))
}
