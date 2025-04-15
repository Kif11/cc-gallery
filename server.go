package main

import (
	"embed"
	_ "embed"
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
	"strconv"
	"strings"
	"testing/fstest"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

//go:embed web/gallery/*.html
var galleryDir embed.FS

//go:embed web/global.css
var globalCss []byte

//go:embed web/gallery/album.css
var albumCss []byte

//go:embed web/gallery/player.css
var playerCss []byte

//go:embed web/gallery/gallery.css
var galleryCss []byte

//go:embed web/global.js
var globalJs []byte

//go:embed web/gallery/gallery.js
var galleryJs []byte

//go:embed web/gallery/player.js
var playerJs []byte

type Config struct {
	// Root path of remote public server where media is stored
	CDNRoot string
	// Root of this server
	ServerRoot string
	// Local to the server director with media
	LocalRoot string
	// Switch backend from local directory to Digital Ocean Spaces (AWS S3 compatible storage)
	UseDigitalOceanSpaces string
}

// For local directory
// var config = Config{
// 	ServerRoot:            "/gallery",
// 	CDNRoot:               "/public/media",
// 	LocalRoot:             "/Users/kif/pr/ccgallery/public/media",
// 	UseDigitalOceanSpaces: getEnv("CCGALLERY_USE_SPACES_STORAGE", "0"),
// }l

// For Digital Ocean Spaces (S3)
var config = Config{
	ServerRoot:            "/gallery",
	CDNRoot:               "https://cdn.codercat.xyz/gallery",
	LocalRoot:             ".",
	UseDigitalOceanSpaces: getEnv("CCGALLERY_USE_SPACES_STORAGE", "1"),
}

var funcs = template.FuncMap{
	"isImage": isImage,
	"isVideo": isVideo,
}

// Load template pages files
var tmpl *template.Template = template.Must(template.New("").Funcs(funcs).ParseFS(galleryDir, "web/gallery/*.html"))

type MediaFileType int64

const (
	Other               = -1
	Image MediaFileType = iota
	Video
	Directory
)

type Media struct {
	Type MediaFileType
	// FileName of the file with extension e.g. 2024/1234_post_0.jpg
	FileName string
	// Full path to public CDN location of media asset
	PublicPath string
	// Full disc path to the asset on the server /homes/bob/public/media/kif/2024/1234_post_0.jpg
	LocalPath string
	// Relative URL path as accessed by the client when browsing e.g. kif/2024, snay/2022/myAlbum
	RelativeURL string
	// Full path to the page where asset is rendered e.g. example.com/gallery/kif/2024
	UrlPath string
}

type LinkedMedia struct {
	Cur  Media
	Prev Media
	Next Media
}

type Album struct {
	Name string
	Link string
}

type AlbumsPage struct {
	Title    string
	Albums   []Album
	BackLink string
	Styles   template.CSS
	JS       template.JS
}

type GalleryPage struct {
	Title        string
	Images       []Media
	BackLink     string
	Styles       template.CSS
	JS           template.JS
	PageSettings PageSettings
}

type PlayerPage struct {
	Title    string
	Image    LinkedMedia
	BackLink string
	Styles   template.CSS
	JS       template.JS
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

func getMediaType(filePath string) MediaFileType {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
		return Image
	case ".mp4", ".mov", ".webm":
		return Video
	case "":
		return Directory
	default:
		return Other
	}
}

func makeMedia(url string, config Config) Media {

	relativePath := strings.TrimSuffix(strings.TrimPrefix(url, "/"), "/")
	fName := ""

	if !isDir(relativePath) {
		fName = path.Base(relativePath)
	}

	return Media{
		Type:        getMediaType(fName),
		FileName:    fName,
		PublicPath:  fmt.Sprintf("%s/%s", config.CDNRoot, relativePath),
		RelativeURL: relativePath,
		LocalPath:   path.Join(config.LocalRoot, relativePath),
		UrlPath:     path.Join(config.ServerRoot, relativePath),
	}
}

// Return new LinkedMedia that has pointers to next and previous media file
func makeLinkMedia(m Media, images []fs.DirEntry) (LinkedMedia, error) {
	li := LinkedMedia{}

	settings := pageSettingsForPath(m.RelativeURL, pageSettings)
	sortedMedia := sortDirEntries(images, settings.MediaOrder)

	for i := 0; i < len(sortedMedia); i++ {
		f := sortedMedia[i]

		if f.Name() != m.FileName {
			continue
		}

		li.Cur = m

		// Image found

		if len(images) == 1 { // Array length of 1
			return li, nil
		}

		if i == 0 {
			// First item
			li.Next = makeMedia(path.Join(path.Dir(m.RelativeURL), images[i+1].Name()), config)
		} else if i == len(images)-1 {
			// Last item
			li.Prev = makeMedia(path.Join(path.Dir(m.RelativeURL), images[i-1].Name()), config)
		} else {
			// Middle item
			li.Next = makeMedia(path.Join(path.Dir(m.RelativeURL), images[i+1].Name()), config)
			li.Prev = makeMedia(path.Join(path.Dir(m.RelativeURL), images[i-1].Name()), config)
		}

		return li, nil
	}

	return li, fmt.Errorf("image with id %s not found", m.FileName)
}

// albumsHandler renders folders in given directory as albums
func albumsHandler(albums []Album, backLink string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageData := AlbumsPage{
			Albums:   albums,
			BackLink: backLink,
			Styles:   template.CSS(append(albumCss, globalCss...)),
			JS:       template.JS(globalJs),
		}

		err := tmpl.ExecuteTemplate(w, "album.html", pageData)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

// playerHandler render individual media on it's own page
func playerHandler(li LinkedMedia, backLink string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		post := PlayerPage{
			Image:    li,
			BackLink: backLink,
			Styles:   template.CSS(append(playerCss, globalCss...)),
			JS:       template.JS(append(globalJs, playerJs...)),
		}

		err := tmpl.ExecuteTemplate(w, "player.html", post)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

// galleryHandler renders folder with images as a gallery
func galleryHandler(media []Media, title string, backLink string, settings PageSettings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gallery := GalleryPage{
			Title:        title,
			Images:       media,
			BackLink:     backLink,
			Styles:       template.CSS(append(galleryCss, globalCss...)),
			PageSettings: settings,
			JS:           template.JS(append(globalJs, galleryJs...)),
		}

		err := tmpl.ExecuteTemplate(w, "gallery.html", gallery)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

func isDir(path string) bool {
	return filepath.Ext(path) == ""
}

func getEnv(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		fmt.Printf("[-] Environment value for %s is not set, using default '%s'\n", name, fallback)
		return fallback
	}
	return value
}

func s3List() ([]string, error) {
	endpoint := "nyc3.digitaloceanspaces.com"
	region := "nyc3"

	bucket := getEnv("SPACES_BUCKET", "cc-storage")
	key := getEnv("SPACES_KEY", "")
	secret := getEnv("SPACES_SECRET", "")
	galleryFolder := "gallery"

	// TODO Check of env var defined

	s3Config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(key, secret, ""),
		Endpoint:    aws.String(endpoint),
		Region:      aws.String(region),
	}

	newSession, err := session.NewSession(s3Config)
	if err != nil {
		return []string{}, err
	}
	svc := s3.New(newSession)

	names := []string{}

	i := 0
	err = svc.ListObjectsPages(&s3.ListObjectsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(galleryFolder),
	}, func(p *s3.ListObjectsOutput, last bool) (shouldContinue bool) {
		i++

		for _, item := range p.Contents {
			names = append(names, *item.Key)
		}

		return true
	})

	if err != nil {
		fmt.Println("failed to list objects", err)
		return []string{}, err
	}

	return names, nil
}

func digitalOceanSpacesFS() (fs.FS, func() error) {
	var s3Fs fstest.MapFS = make(map[string]*fstest.MapFile)

	return s3Fs, func() error {
		files, err := s3List()
		if err != nil {
			return err
		}

		// Clear the map
		for k := range s3Fs {
			delete(s3Fs, k)
		}

		for _, name := range files {
			path := strings.TrimPrefix(name, "gallery/")
			if path == "" {
				continue
			}
			s3Fs[path] = &fstest.MapFile{}
		}

		return nil
	}
}

type FsItems struct {
	Folders []fs.DirEntry
	Files   []fs.DirEntry
}

func listFsItems(fSys fs.FS, path string) (FsItems, error) {
	fsItems := FsItems{}

	var err error
	var dirs []fs.DirEntry

	dirs, err = fs.ReadDir(fSys, path)
	if err != nil {
		return fsItems, err
	}

	for _, c := range dirs {
		if c.Name() == "" {
			continue
		}

		if c.IsDir() {
			fsItems.Folders = append(fsItems.Folders, c)
			continue
		}

		fsItems.Files = append(fsItems.Files, c)
	}

	return fsItems, nil
}

func sortDirEntries(files []fs.DirEntry, order MediaOrder) []fs.DirEntry {
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

// filterDirEntries takes filter string like "post reel" and filter all
// DirEntries that include "post" or "reel" in their filename
func filterDirEntries(entries []fs.DirEntry, filter string) (filtered []fs.DirEntry) {

	parts := strings.Split(filter, " ")

	for _, f := range entries {
		for _, word := range parts {
			if word == "" {
				filtered = append(filtered, f)
				continue
			}

			if strings.Contains(f.Name(), word) {
				filtered = append(filtered, f)
			}
		}

	}

	return filtered
}

func valueFromCookies(cookies []*http.Cookie, name string) string {
	for _, c := range cookies {
		if c.Name == name {
			return c.Value
		}
	}

	return ""
}

func makeGalleryRootHandler(fSys fs.FS) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		m := makeMedia(r.URL.Path, config)

		searchPath := m.LocalPath
		if m.Type != Directory {
			searchPath = path.Dir(m.LocalPath)
		}

		fsItems, err := listFsItems(fSys, searchPath)
		if err != nil {
			writeError(w, http.StatusNotFound, "Not Found")
			return
		}

		settings := pageSettingsForPath(m.RelativeURL, pageSettings)
		filter := valueFromCookies(r.Cookies(), "filter")
		if filter == "" {
			filter = r.URL.Query().Get("filter")
		}
		filtered := filterDirEntries(fsItems.Files, filter)
		sortedFsEntries := sortDirEntries(filtered, settings.MediaOrder)

		// 1. PATH IS A FILE. RENDER VIEWER
		if m.Type != Directory {

			li, err := makeLinkMedia(m, sortedFsEntries)
			if err != nil {
				writeError(w, http.StatusNotFound, "Not Found")
				return
			}

			backLink := path.Dir(m.UrlPath) + "?p=" + path.Base(m.UrlPath)

			playerHandler(li, backLink)(w, r)

			return
		}

		// 2. PATH CONTAINS FOLDERS. RENDER ALBUM VIEW
		if len(fsItems.Folders) > 0 {

			var albums []Album
			for _, i := range fsItems.Folders {
				albums = append(albums, Album{
					Name: i.Name(),
					Link: path.Join(config.ServerRoot, r.URL.Path, i.Name()),
				})
			}

			sort.Slice(albums, func(i, j int) bool {
				year1, err := strconv.ParseInt(albums[i].Name, 10, 32)
				if err != nil {
					fmt.Printf("sort failed, can not parse album name '%s' to int.\n", albums[i].Name)
					return false
				}
				year2, err := strconv.ParseInt(albums[j].Name, 10, 32)
				if err != nil {
					fmt.Printf("sort failed, can not parse album name '%s' to int.\n", albums[j].Name)
					return false
				}
				return year1 > year2
			})

			albumsHandler(albums, filepath.Dir(config.ServerRoot))(w, r)

			return
		}

		// 3. PATH CONTAINS MEDIA FILES. RENDER GALLERY VIEW
		if len(fsItems.Files) > 0 {

			var media []Media
			for _, f := range sortedFsEntries {
				mi := makeMedia(path.Join(m.RelativeURL, f.Name()), config)
				media = append(media, mi)
			}

			galleryHandler(media, m.RelativeURL, path.Dir(m.UrlPath), settings)(w, r)

			return
		}

		writeError(w, http.StatusNotFound, "Not Found")
	}

}

func makeUpdateHandler(update func() error) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := update()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
	}
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

	dirs, update := digitalOceanSpacesFS()

	err := update()
	if err != nil {
		panic(err)
	}

	galleryRootHandler := makeGalleryRootHandler(dirs)

	// Configure gallery mux
	galleryMux.HandleFunc("/", galleryRootHandler)

	updateHandler := makeUpdateHandler(update)

	// Configure main mux
	mux.HandleFunc(config.ServerRoot+"/update", updateHandler)
	mux.Handle(config.ServerRoot+"/", http.StripPrefix(config.ServerRoot, galleryMux))
	mux.HandleFunc("/", rootHandler)

	address := "localhost:8080"
	fmt.Printf("Listening on %s\n", address)
	log.Fatal(http.ListenAndServe(address, mux))
}
