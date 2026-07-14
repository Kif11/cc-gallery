package main

import (
	"archive/zip"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing/fstest"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type readFileFunc func(path string) ([]byte, error)

type s3Object struct {
	Name string
	Size int64
}

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

// URL path under which assets are served
var assetsRoute = getEnv("CCG_ASSETS_ROUTE", "/assets")

// Load template pages files
var tmpl *template.Template = template.Must(template.New("").ParseFS(galleryDir, "web/gallery/*.html"))

type MediaFileType string

const (
	Other = "Other"
	Image = "Image"
	Video = "Video"
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
	Title       string
	Images      []Media
	URLParam    string
	BackLink    string
	Styles      template.CSS
	JS          template.JS
	GridSize    string
	CurrentPath string
	URLPrefix   string
	AlbumSize   string
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
	value, ok := os.LookupEnv(name)
	if !ok {
		fmt.Printf("[-] Environment value for %s is not set, using default '%s'\n", name, fallback)
		return fallback
	}
	return value
}

func getMediaType(ext string) MediaFileType {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return Image
	case ".mp4", ".mov", ".webm":
		return Video
	default:
		return Other
	}
}

func makeMedia(relativeURL string, assetsRoute string, urlPrefix string) Media {
	// Clean up the path by removing leading and trailing slashes
	relativeURL = strings.Trim(relativeURL, "/")

	// For files, get the filename
	fName := ""
	if !isDir(relativeURL) {
		fName = path.Base(relativeURL)
	}

	// Handle special case for root
	publicPath := assetsRoute + "/" + relativeURL
	if relativeURL == "/" {
		publicPath = assetsRoute + "/"
	}

	ext := filepath.Ext(fName)

	return Media{
		Type:            getMediaType(ext),
		FileName:        fName,
		DirName:         path.Base(relativeURL),
		PublicPath:      publicPath,
		RelativePageURL: relativeURL,
		AbsolutePageURL: path.Join(urlPrefix, relativeURL),
	}
}

// Return new LinkedMedia that has pointers to next and previous media file
func makeLinkMedia(m Media, images []fs.DirEntry) (LinkedMedia, error) {
	li := LinkedMedia{Cur: m}

	// Find the index of current media in images array
	index := -1
	for i, f := range images {
		if f.Name() == m.FileName {
			index = i
			break
		}
	}

	// Return error if not found
	if index == -1 {
		return li, fmt.Errorf("image with id %s not found", m.FileName)
	}

	dir := path.Dir(m.RelativePageURL)

	// Set previous media if not first item
	if index > 0 {
		li.Prev = makeMedia(path.Join(dir, images[index-1].Name()), assetsRoute, urlPrefix)
	}

	// Set next media if not last item
	if index < len(images)-1 {
		li.Next = makeMedia(path.Join(dir, images[index+1].Name()), assetsRoute, urlPrefix)
	}

	return li, nil
}

func s3List() ([]s3Object, error) {
	endpoint := getEnv("CCG_S3_ENDPOINT", "nyc3.digitaloceanspaces.com")
	region := getEnv("CCG_S3_REGION", "nyc3")

	bucket := getEnv("CCG_S3_BUCKET", "cc-storage")
	key := getEnv("CCG_S3_KEY", "")
	secret := getEnv("CCG_S3_SECRET", "")
	galleryFolder := getEnv("CCG_S3_ROOT_DIR", "")

	if key == "" || secret == "" {
		fmt.Println("[!] Can not connect to S3. S3_KEY or S3_SECRET environmental variables are not set!")
		os.Exit(1)
	}

	s3Config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(key, secret, ""),
		Endpoint:    aws.String(endpoint),
		Region:      aws.String(region),
	}

	newSession, err := session.NewSession(s3Config)
	if err != nil {
		return []s3Object{}, err
	}
	svc := s3.New(newSession)

	objects := []s3Object{}

	// List all objects in the bucket with the specified prefix
	err = svc.ListObjectsPages(&s3.ListObjectsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(galleryFolder),
	}, func(p *s3.ListObjectsOutput, last bool) (shouldContinue bool) {
		for _, item := range p.Contents {
			objects = append(objects, s3Object{
				Name: strings.TrimPrefix(*item.Key, galleryFolder+"/"),
				Size: *item.Size,
			})
		}

		return true
	})

	if err != nil {
		fmt.Println("failed to list objects", err)
		return []s3Object{}, err
	}

	return objects, nil
}

// Returns `update` function which can be used to refresh s3 entries
// that cached in memory map, and a readFile function for fetching file content from S3
func s3FS(fileListFn func() ([]s3Object, error)) (fs.FS, readFileFunc, func(string) int64, func() error) {
	var s3Fs fstest.MapFS = make(map[string]*fstest.MapFile)
	sizes := make(map[string]int64)

	endpoint := getEnv("CCG_S3_ENDPOINT", "nyc3.digitaloceanspaces.com")
	region := getEnv("CCG_S3_REGION", "nyc3")
	bucket := getEnv("CCG_S3_BUCKET", "cc-storage")
	key := getEnv("CCG_S3_KEY", "")
	secret := getEnv("CCG_S3_SECRET", "")
	galleryFolder := getEnv("CCG_S3_ROOT_DIR", "")

	s3Config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(key, secret, ""),
		Endpoint:    aws.String(endpoint),
		Region:      aws.String(region),
	}
	sess, _ := session.NewSession(s3Config)
	svc := s3.New(sess)

	readFile := func(p string) ([]byte, error) {
		key := galleryFolder + "/" + p
		if galleryFolder == "" {
			key = p
		}
		result, err := svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return nil, err
		}
		defer result.Body.Close()
		return io.ReadAll(result.Body)
	}

	sizeFn := func(p string) int64 {
		return sizes[p]
	}

	return s3Fs, readFile, sizeFn, func() error {
		objects, err := fileListFn()
		if err != nil {
			return err
		}

		// Clear the maps
		for k := range s3Fs {
			delete(s3Fs, k)
		}
		for k := range sizes {
			delete(sizes, k)
		}

		for _, obj := range objects {
			if obj.Name == "" {
				continue
			}
			s3Fs[obj.Name] = &fstest.MapFile{}
			sizes[obj.Name] = obj.Size
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

// This function assumes the following file names:
// - story_12345_0.jpg which follow the template {type}_{unix_timestamp}_{index}.{ext}
// - file.jpg (arbitrary name)
// if file name match the template it will be sorted by timestamp (descending) and then by index (ascending)
// if file name does not match the template it will be sorted by name (descending)
func sortDirEntries(files []fs.DirEntry) []fs.DirEntry {
	// Define regex pattern for special filenames
	pattern := regexp.MustCompile(`^([^_]+)_(\d+)_(\d+)`)

	// Make a copy to avoid modifying original slice
	sorted := make([]fs.DirEntry, len(files))
	copy(sorted, files)

	// Sort the slice using custom comparison function
	sort.Slice(sorted, func(i, j int) bool {
		// Always put directories first
		if sorted[i].IsDir() && !sorted[j].IsDir() {
			return true
		}
		if !sorted[i].IsDir() && sorted[j].IsDir() {
			return false
		}

		nameI := sorted[i].Name()
		nameJ := sorted[j].Name()

		// Try to match both filenames against the pattern
		matchI := pattern.FindStringSubmatch(nameI)
		matchJ := pattern.FindStringSubmatch(nameJ)

		// If both files match the pattern
		if matchI != nil && matchJ != nil {
			// Compare timestamps
			timestampI, _ := strconv.ParseInt(matchI[2], 10, 64)
			timestampJ, _ := strconv.ParseInt(matchJ[2], 10, 64)

			if timestampI != timestampJ {
				return timestampI > timestampJ // ascending order
			}

			// If timestamps are equal, compare indices
			indexI, _ := strconv.Atoi(matchI[3])
			indexJ, _ := strconv.Atoi(matchJ[3])

			return indexI < indexJ // ascending order
		}

		// Fall back to alphabetical sorting for non-matching files
		return nameI > nameJ
	})

	return sorted
}

// filterDirEntries takes filter string like "post reel" and keep all
// DirEntries that include "post" or "reel" in their filename
func filterDirEntries(entries []fs.DirEntry, filter string) (filtered []fs.DirEntry) {
	// Early return if filter is empty
	if filter == "" {
		return entries
	}

	parts := strings.Split(filter, " ")
	hasEmptyWord := false

	// Check if there's an empty word in the filter parts
	for _, word := range parts {
		if word == "" {
			hasEmptyWord = true
			break
		}
	}

	for _, f := range entries {
		// Include all files if filter has empty word
		if hasEmptyWord {
			filtered = append(filtered, f)
			continue
		}

		// Look for any filter word in the filename
		for _, word := range parts {
			if strings.Contains(f.Name(), word) {
				filtered = append(filtered, f)
				break
			}
		}
	}

	return filtered
}

func writeError(w http.ResponseWriter, header int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(header)
	w.Write([]byte(msg))
}

// playerHandler render individual media on it's own page
func playerHandler(li LinkedMedia, title string, backLink string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Construct back link that lead to the gallery page.
		// The link include "p" parameter that hold current media.
		// Gallery page will scroll that media into view.
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
func galleryHandler(media []Media, title string, backLink string, currentPath string, albumSize string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get grid size from URL parameter, default to 300px if not specified
		gridSize := r.URL.Query().Get("grid")
		if gridSize == "" {
			gridSize = "300px"
		}

		gallery := GalleryPage{
			Title:       title,
			Images:      media,
			URLParam:    "?" + r.URL.RawQuery,
			BackLink:    backLink,
			Styles:      template.CSS(append(galleryCss, globalCss...)),
			GridSize:    gridSize,
			JS:          template.JS(append(globalJs, galleryJs...)),
			CurrentPath: currentPath,
			URLPrefix:   urlPrefix,
			AlbumSize:   albumSize,
		}

		err := tmpl.ExecuteTemplate(w, "gallery.html", gallery)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

func filterNonSupported(entries []fs.DirEntry) []fs.DirEntry {
	filtered := []fs.DirEntry{}
	for _, en := range entries {
		ext := filepath.Ext(en.Name())
		if en.IsDir() {
			filtered = append(filtered, en)
			continue
		}
		if getMediaType(ext) == Other {
			continue
		}
		filtered = append(filtered, en)
	}

	return filtered
}

func getMediaSearchPath(url string) string {
	ext := path.Ext(url)
	sp := strings.Trim(url, "/")

	if sp == "" {
		sp = "."
	}

	if ext != "" {
		// URL is a file, remove the file name
		sp = path.Dir(sp)
	}

	return sp
}

// Root handler that select appropriate HTTP handler depending on the route requested
func makeGalleryRootHandler(fSys fs.FS, sizeFn func(string) int64) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		p := getMediaSearchPath(r.URL.Path)

		fsItems, err := listFsItems(fSys, p)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		filter := r.URL.Query().Get("filter")
		filtered := filterNonSupported(fsItems)
		filtered = filterDirEntries(filtered, filter)

		sortedFsEntries := sortDirEntries(filtered)

		// If media is a file and one of the supported media extensions when render it in the player
		if getMediaType(path.Ext(r.URL.Path)) != Other {
			/*
			 * PLAYER
			 */

			m := makeMedia(r.URL.Path, assetsRoute, urlPrefix)
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
				m := makeMedia(path.Join(r.URL.Path, f.Name()), assetsRoute, urlPrefix)
				media = append(media, m)
			}

			galleryHandler(media, r.URL.Path, path.Dir(urlPrefix+"/"+r.URL.Path), r.URL.Path, getAlbumSize(p, sortedFsEntries, sizeFn))(w, r)
		}
	}
}

// Handler that will update s3 file list.
// Because fetching media from s3 is slow we prefetch the entire collection into RAM.
// When user update media in the s3 bucket changed won't be reflected until
// this server is restarted OR user call GET /update endpoint
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

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func getAlbumSize(dirPath string, entries []fs.DirEntry, sizeFn func(string) int64) string {
	var total int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		total += sizeFn(path.Join(dirPath, entry.Name()))
	}
	return formatSize(total)
}

// calculateZipSize computes the exact byte size of a ZIP archive in Store mode
// without downloading any file contents. It only needs filenames and sizes.
// ZIP entry overhead: local header (30 + name) + data + data descriptor (16) + central dir entry (46 + name) + EOCD (22)
func calculateZipSize(entries []fs.DirEntry, dirPath string, sizeFn func(string) int64) int64 {
	var total int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		total += 30 + int64(len(name)) // local file header
		total += sizeFn(path.Join(dirPath, name)) // file data
		total += 16                                // data descriptor
		total += 46 + int64(len(name)) // central directory entry
	}
	total += 22 // end of central directory record
	return total
}

func makeDownloadHandler(fSys fs.FS, readFile readFileFunc, sizeFn func(string) int64) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Strip the urlPrefix + "/download" prefix from the path
		p := strings.TrimPrefix(r.URL.Path, urlPrefix+"/download")
		p = strings.Trim(p, "/")
		if p == "" {
			p = "."
		}

		fsItems, err := listFsItems(fSys, p)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		filtered := filterNonSupported(fsItems)
		sorted := sortDirEntries(filtered)

		w.Header().Set("Content-Type", "application/zip")
		folderName := path.Base(p)
		if folderName == "." {
			folderName = "gallery"
		}
		w.Header().Set("Content-Disposition", "attachment; filename=\""+folderName+".zip\"")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", calculateZipSize(sorted, p, sizeFn)))

		zipWriter := zip.NewWriter(w)
		defer zipWriter.Close()

		for _, entry := range sorted {
			if entry.IsDir() {
				continue
			}
			data, err := readFile(path.Join(p, entry.Name()))
			if err != nil {
				continue
			}
			header := &zip.FileHeader{
				Name:     entry.Name(),
				Method:   zip.Store,
			}
			f, err := zipWriter.CreateHeader(header)
			if err != nil {
				continue
			}
			f.Write(data)
		}
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, urlPrefix, http.StatusFound)
}

// Returns a local filesystem handler that implements fs.FS interface
func localFS(folder string) (fs.FS, readFileFunc, func(string) int64, func() error) {
	readFile := func(p string) ([]byte, error) {
		return os.ReadFile(filepath.Join(folder, p))
	}
	sizeFn := func(p string) int64 {
		info, err := os.Stat(filepath.Join(folder, p))
		if err != nil {
			return 0
		}
		return info.Size()
	}
	return os.DirFS(folder), readFile, sizeFn, func() error {
		// No-op update function since local filesystem is always up to date
		return nil
	}
}

func main() {
	mux := http.NewServeMux()
	galleryMux := http.NewServeMux()

	assetsFolder := getEnv("CCG_LOCAL_ASSETS_FOLDER", "")

	var rootFS fs.FS
	var readFile readFileFunc
	var sizeFn func(string) int64
	var update func() error

	if assetsFolder != "" {
		// Use local folder as a media backend
		rootFS, readFile, sizeFn, update = localFS(assetsFolder)

		// Handle public assets from public directory under example.com/assets URL
		fs := http.FileServer(http.Dir(assetsFolder))
		mux.Handle(assetsRoute+"/", http.StripPrefix(assetsRoute, fs))
	} else {
		// Use s3 as media backend
		rootFS, readFile, sizeFn, update = s3FS(s3List)
	}

	err := update()
	if err != nil {
		panic(err)
	}

	galleryRootHandler := makeGalleryRootHandler(rootFS, sizeFn)
	downloadHandler := makeDownloadHandler(rootFS, readFile, sizeFn)

	// Configure gallery mux
	galleryMux.HandleFunc("/", galleryRootHandler)

	updateHandler := makeUpdateHandler(update)

	// Configure main mux
	mux.HandleFunc(urlPrefix+"/update", updateHandler)
	mux.HandleFunc(urlPrefix+"/download/", downloadHandler)
	mux.Handle(urlPrefix+"/", http.StripPrefix(urlPrefix, galleryMux))
	mux.HandleFunc("/", rootHandler)

	address := getEnv("CCG_SERVER_ADDRESS", "localhost:8080")
	fmt.Printf("[+] Listening on %s\n", address)
	log.Fatal(http.ListenAndServe(address, mux))
}
