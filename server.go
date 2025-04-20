package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
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

// Prefix of your web server URL under which this gallery is hosted
// e.g. if you have you main site on mysite.org and gallery under mysite.org/gallery
// you should configure nginx (or other web server) reverse proxy to /gallery and set prefix to /gallery
var urlPrefix = getEnv("CCG_URL_PREFIX", "/gallery")

var webRoot = getEnv("CCG_WEB_ROOT", "")

var funcs = template.FuncMap{
	"myFunc": func() string { return "hi" }, // This is just a placeholder in case I need to call a function inside any template
}

// Load template pages files
var tmpl *template.Template = template.Must(template.New("").Funcs(funcs).ParseFS(galleryDir, "web/gallery/*.html"))

type MediaFileType string

const (
	Other     = "Other"
	Image     = "Image"
	Video     = "Video"
	Directory = "Directory"
)

type Media struct {
	Type MediaFileType
	// FileName of the file with extension e.g. 2024/1234_post_0.jpg
	FileName string
	// Name of the file directory
	DirName string
	// Full path to public CDN location of media asset
	PublicPath string
	// Relative URL path as accessed by the client when browsing e.g. kif/2024, snay/2022/myAlbum
	RelativePageURL string
	// Full path to the page where asset is rendered e.g. example.com/gallery/kif/2024
	AbsolutePageURL string
}

type LinkedMedia struct {
	Cur  Media
	Prev Media
	Next Media
}

type GalleryPage struct {
	Title    string
	Images   []Media
	URLParam string
	BackLink string
	Styles   template.CSS
	JS       template.JS
	GridSize string
}

type PlayerPage struct {
	Title    string
	Image    LinkedMedia
	URLParam string
	BackLink string
	Styles   template.CSS
	JS       template.JS
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

func makeMedia(url string) Media {

	relativePath := strings.TrimSuffix(strings.TrimPrefix(url, "/"), "/")
	fName := ""

	if !isDir(relativePath) {
		fName = path.Base(relativePath)
	}

	return Media{
		Type:            getMediaType(fName),
		FileName:        fName,
		DirName:         path.Base(relativePath),
		PublicPath:      fmt.Sprintf("%s/%s", webRoot, relativePath),
		RelativePageURL: relativePath,
		AbsolutePageURL: path.Join(urlPrefix, relativePath),
	}
}

// Return new LinkedMedia that has pointers to next and previous media file
func makeLinkMedia(m Media, images []fs.DirEntry) (LinkedMedia, error) {
	li := LinkedMedia{}

	for i := 0; i < len(images); i++ {
		f := images[i]

		if f.Name() != m.FileName {
			continue
		}

		li.Cur = m

		// Image found

		if len(images) == 1 { // Array length of 1
			return li, nil
		}

		dir := path.Dir(m.RelativePageURL)

		if i == 0 {
			// First item
			li.Next = makeMedia(path.Join(dir, images[i+1].Name()))
		} else if i == len(images)-1 {
			// Last item
			li.Prev = makeMedia(path.Join(dir, images[i-1].Name()))
		} else {
			// Middle item
			li.Next = makeMedia(path.Join(dir, images[i+1].Name()))
			li.Prev = makeMedia(path.Join(dir, images[i-1].Name()))
		}

		return li, nil
	}

	return li, fmt.Errorf("image with id %s not found", m.FileName)
}

func s3List() ([]string, error) {
	endpoint := getEnv("CCG_S3_ENDPOINT", "nyc3.digitaloceanspaces.com")
	region := getEnv("CCG_S3_REGION", "nyc3")

	bucket := getEnv("CCG_S3_BUCKET", "cc-storage")
	key := getEnv("CCG_S3_KEY", "")
	secret := getEnv("CCG_S3_SECRET", "")
	galleryFolder := getEnv("CCG_S3_ROOT_DIR", "gallery")

	if key == "" || secret == "" {
		fmt.Println("[!] Can not connect to S3. S3_KEY or S3_SECRET enviromental variables are not set!")
		os.Exit(1)
	}

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
			names = append(names, strings.TrimPrefix(*item.Key, galleryFolder+"/"))
		}

		return true
	})

	if err != nil {
		fmt.Println("failed to list objects", err)
		return []string{}, err
	}

	return names, nil
}

// Returns `update` function which can be used to refresh s3 entried
// that cached in memory map
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

		for _, path := range files {
			if path == "" {
				continue
			}
			s3Fs[path] = &fstest.MapFile{}
		}

		return nil
	}
}

func listFsItems(fSys fs.FS, path string) ([]fs.DirEntry, error) {
	fsItems := []fs.DirEntry{}

	var err error
	var dirs []fs.DirEntry

	dirs, err = fs.ReadDir(fSys, path)
	if err != nil {
		return fsItems, fmt.Errorf("error reading directory %s: %w", path, err)
	}

	for _, c := range dirs {
		if c.Name() == "." {
			continue
		}

		fsItems = append(fsItems, c)
	}

	return fsItems, nil
}

func stripFirsToken(name, sep string) string {
	if strings.Contains(name, sep) {
		return strings.Join(strings.Split(name, sep)[1:], sep)
	}
	return name
}

// This is the main sorting for the gallery.
// It assume file names are "name", "2024" or "post_12345_0.jpg" which follows the template "<media_type>_<unix_time>_<index>.<ext>"".
// It will attemnt to strip first token before "_" delimiter.
// It will sort it base on go string comparison which use ASCII order for each character
// e.g. "0" comes before "a", "A" comes before "a" and "+" comes before "0" and "a".
// It compare each char in order, if char is equal the next char is compared.
// See https://www.asciitable.com/
func sortDirEntries(files []fs.DirEntry) []fs.DirEntry {
	sort.Slice(files, func(i, j int) bool {
		n1 := stripFirsToken(files[i].Name(), "_")
		n2 := stripFirsToken(files[j].Name(), "_")

		return n1 > n2
	})

	return files
}

// filterDirEntries takes filter string like "post reel" and keep all
// DirEntries that include "post" or "reel" in their filename
func filterDirEntries(entries []fs.DirEntry, filter string) (filtered []fs.DirEntry) {

	parts := strings.Split(filter, " ")

	for _, f := range entries {
		if f.IsDir() {
			filtered = append(filtered, f)
			continue
		}

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

func writeError(w http.ResponseWriter, header int, msg string) {
	w.WriteHeader(header)
	w.Write([]byte(msg))
}

func valueFromCookies(cookies []*http.Cookie, name string) string {
	for _, c := range cookies {
		if c.Name == name {
			return c.Value
		}
	}

	return ""
}

// playerHandler render individual media on it's own page
func playerHandler(li LinkedMedia, title string, backLink string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Construct back link that lead to the gallery page.
		// The link include "p" parameter that hold current media.
		// Gallary page will sroll that media into view.
		qq, _ := url.QueryUnescape(r.URL.RawQuery)

		values, err := url.ParseQuery(qq)
		if err != nil {
			panic(err)
		}

		params := make(url.Values)
		for k, v := range values {
			params[k] = v
		}
		params.Set("p", path.Base(li.Cur.FileName))

		post := PlayerPage{
			Title:    title,
			Image:    li,
			BackLink: backLink,
			URLParam: "?" + params.Encode(),
			Styles:   template.CSS(append(playerCss, globalCss...)),
			JS:       template.JS(append(globalJs, playerJs...)),
		}

		err = tmpl.ExecuteTemplate(w, "player.html", post)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

// galleryHandler renders folder with images as a gallery
func galleryHandler(media []Media, title string, backLink string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get grid size from URL parameter, default to 300px if not specified
		gridSize := r.URL.Query().Get("grid")
		if gridSize == "" {
			gridSize = "300px"
		}

		gallery := GalleryPage{
			Title:    title,
			Images:   media,
			URLParam: "?" + r.URL.RawQuery,
			BackLink: backLink,
			Styles:   template.CSS(append(galleryCss, globalCss...)),
			GridSize: gridSize,
			JS:       template.JS(append(globalJs, galleryJs...)),
		}

		err := tmpl.ExecuteTemplate(w, "gallery.html", gallery)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

// Root handler that select apropriate HTTP handler depending on the route requested
func makeGalleryRootHandler(fSys fs.FS) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Build Media struct for the current requested route
		// that hold all necessery paths
		m := makeMedia(r.URL.Path)

		searchPath := m.RelativePageURL
		if searchPath == "" {
			searchPath = "."
		}
		if m.Type != Directory {
			searchPath = path.Dir(m.RelativePageURL)
		}

		fsItems, err := listFsItems(fSys, searchPath)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		// filter := valueFromCookies(r.Cookies(), "filter")
		// if filter == "" {
		// 	filter = r.URL.Query().Get("filter")
		// }
		filter := r.URL.Query().Get("filter")
		filtered := filterDirEntries(fsItems, filter)

		sortedFsEntries := sortDirEntries(filtered)

		if m.Type == Image || m.Type == Video {
			/*
			 * PLAYER
			 */

			li, err := makeLinkMedia(m, sortedFsEntries)
			if err != nil {
				writeError(w, http.StatusNotFound, "Not Found")
				return
			}

			playerHandler(li, m.FileName, path.Dir(m.AbsolutePageURL))(w, r)
		} else {
			/*
			 * GALLERY
			 */

			var media []Media
			for _, f := range sortedFsEntries {
				mi := makeMedia(path.Join(m.RelativePageURL, f.Name()))
				media = append(media, mi)
			}

			galleryHandler(media, m.RelativePageURL, path.Dir(m.AbsolutePageURL))(w, r)
		}
	}
}

// Handler that will update s3 file list.
// Because fetching media from s3 is slow we prefetch the entire collection into RAM.
// When user update media in the s3 bucket changed won't be reflected until
//
//	a) This server is restarted
//	b) User call GET /updage endpoint
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
	http.Redirect(w, r, urlPrefix, http.StatusFound)
}

// Returns a local filesystem handler that implements fs.FS interface
func localFS(path string) (fs.FS, func() error) {
	return os.DirFS(path), func() error {
		// No-op update function since local filesystem is always up to date
		return nil
	}
}

func main() {
	mux := http.NewServeMux()
	galleryMux := http.NewServeMux()

	localAssetFolder := getEnv("CCG_LOCAL_ASSET_FOLDER", "")

	var rootFS fs.FS
	var update func() error

	if localAssetFolder != "" {
		// Use local folder as a media backend
		rootFS, update = localFS(localAssetFolder)

		assetsFolder := getEnv("CCG_ASSETS_FOLDER", "assets")
		assetsURLPrefix := getEnv("CCG_ASSETS_URL_PREFIX", "/assets")

		// Handle public assets from public directory under example.com/assets URL
		fs := http.FileServer(http.Dir(assetsFolder))
		mux.Handle(assetsURLPrefix+"/", http.StripPrefix(assetsURLPrefix, fs))
	} else {
		// Use s3 as media backend
		rootFS, update = digitalOceanSpacesFS()
	}

	err := update()
	if err != nil {
		panic(err)
	}

	galleryRootHandler := makeGalleryRootHandler(rootFS)

	// Configure gallery mux
	galleryMux.HandleFunc("/", galleryRootHandler)

	updateHandler := makeUpdateHandler(update)

	// Configure main mux
	mux.HandleFunc(urlPrefix+"/update", updateHandler)
	mux.Handle(urlPrefix+"/", http.StripPrefix(urlPrefix, galleryMux))
	mux.HandleFunc("/", rootHandler)

	address := getEnv("CCG_SERVER_ADDRESS", "localhost:8080")
	fmt.Printf("[+] Listening on %s\n", address)
	log.Fatal(http.ListenAndServe(address, mux))
}
