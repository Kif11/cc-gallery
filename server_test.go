package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"testing/fstest"
)

// Test cases
func TestMakeMedia(t *testing.T) {
	tests := []struct {
		name               string
		url                string
		assetsRoute        string
		urlPrefix          string
		expectedType       MediaFileType
		expectedFileName   string
		expectedDirName    string
		expectedPublicPath string
		expectedRelURL     string
		expectedAbsURL     string
	}{
		{
			name:               "directory",
			url:                "directory/",
			assetsRoute:        "/public/media",
			urlPrefix:          "/gallery",
			expectedType:       Other,
			expectedFileName:   "",
			expectedDirName:    "directory",
			expectedPublicPath: "/public/media/directory",
			expectedRelURL:     "directory",
			expectedAbsURL:     "/gallery/directory",
		},
		{
			name:               "file",
			url:                "directory/file.jpg",
			assetsRoute:        "/public/media",
			urlPrefix:          "/gallery",
			expectedType:       Image,
			expectedFileName:   "file.jpg",
			expectedDirName:    "file.jpg",
			expectedPublicPath: "/public/media/directory/file.jpg",
			expectedRelURL:     "directory/file.jpg",
			expectedAbsURL:     "/gallery/directory/file.jpg",
		},
		{
			name:               "root",
			url:                "/",
			assetsRoute:        "/public/media",
			urlPrefix:          "/gallery",
			expectedType:       Other,
			expectedFileName:   "",
			expectedDirName:    ".",
			expectedPublicPath: "/public/media/",
			expectedRelURL:     "",
			expectedAbsURL:     "/gallery",
		},
		{
			name:               "assetsRoute using CDN URL",
			url:                "/",
			assetsRoute:        "https://cdn.example.com/media",
			urlPrefix:          "/gallery",
			expectedType:       Other,
			expectedFileName:   "",
			expectedDirName:    ".",
			expectedPublicPath: "https://cdn.example.com/media/",
			expectedRelURL:     "",
			expectedAbsURL:     "/gallery",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			media := makeMedia(tt.url, tt.assetsRoute, tt.urlPrefix)

			if media.Type != tt.expectedType {
				t.Errorf("Type = %v, want %v", media.Type, tt.expectedType)
			}
			if media.FileName != tt.expectedFileName {
				t.Errorf("FileName = %v, want %v", media.FileName, tt.expectedFileName)
			}
			if media.DirName != tt.expectedDirName {
				t.Errorf("DirName = %v, want %v", media.DirName, tt.expectedDirName)
			}
			if media.PublicPath != tt.expectedPublicPath {
				t.Errorf("PublicPath = %v, want %v", media.PublicPath, tt.expectedPublicPath)
			}
			if media.RelativePageURL != tt.expectedRelURL {
				t.Errorf("RelativePageURL = %v, want %v", media.RelativePageURL, tt.expectedRelURL)
			}
			if media.AbsolutePageURL != tt.expectedAbsURL {
				t.Errorf("AbsolutePageURL = %v, want %v", media.AbsolutePageURL, tt.expectedAbsURL)
			}
		})
	}
}

func TestSortDirEntries(t *testing.T) {
	tests := []struct {
		name     string
		files    []fs.DirEntry
		expected []string
	}{
		{
			name: "simple sorting by stripFirsToken",
			files: []fs.DirEntry{
				&mockDirEntry{name: "post_1.jpg"},
				&mockDirEntry{name: "post_3.jpg"},
				&mockDirEntry{name: "post_2.jpg"},
			},
			expected: []string{"post_3.jpg", "post_2.jpg", "post_1.jpg"},
		},
		{
			name: "directories sort correctly",
			files: []fs.DirEntry{
				&mockDirEntry{name: "2023", isDir: true},
				&mockDirEntry{name: "2025", isDir: true},
				&mockDirEntry{name: "2024", isDir: true},
			},
			expected: []string{"2025", "2024", "2023"},
		},
		{
			name: "special filenames with timestamps and indices",
			files: []fs.DirEntry{
				&mockDirEntry{name: "story_1000_1.jpg"},
				&mockDirEntry{name: "story_1000_0.jpg"},
				&mockDirEntry{name: "story_2000_0.jpg"},
				&mockDirEntry{name: "story_500_0.jpg"},
			},
			expected: []string{
				"story_2000_0.jpg",
				"story_1000_0.jpg",
				"story_1000_1.jpg",
				"story_500_0.jpg",
			},
		},
		{
			name: "mixed regular and special filenames",
			files: []fs.DirEntry{
				&mockDirEntry{name: "regular2.jpg"},
				&mockDirEntry{name: "story_1000_0.jpg"},
				&mockDirEntry{name: "regular1.jpg"},
			},
			expected: []string{
				"story_1000_0.jpg",
				"regular2.jpg",
				"regular1.jpg",
			},
		},
		{
			name: "directories and files mixed",
			files: []fs.DirEntry{
				&mockDirEntry{name: "story_1000_0.jpg", isDir: false},
				&mockDirEntry{name: "dir1", isDir: true},
				&mockDirEntry{name: "dir2", isDir: true},
				&mockDirEntry{name: "regular.jpg", isDir: false},
			},
			expected: []string{
				"dir2",
				"dir1",
				"story_1000_0.jpg",
				"regular.jpg",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sortDirEntries(tt.files)
			if len(result) != len(tt.expected) {
				t.Errorf("sortDirEntries() length mismatch: got %v, want %v", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i].Name() != tt.expected[i] {
					t.Errorf("sortDirEntries() at index %d: got %v, want %v", i, result[i].Name(), tt.expected[i])
				}
			}
		})
	}
}

func TestMakeLinkMedia(t *testing.T) {
	// Create test media
	m := Media{
		Type:            Image,
		FileName:        "file2.jpg",
		RelativePageURL: "test/file2.jpg",
	}

	// Create dir entries
	files := []fs.DirEntry{
		&mockDirEntry{name: "file1.jpg"},
		&mockDirEntry{name: "file2.jpg"},
		&mockDirEntry{name: "file3.jpg"},
	}

	linkedMedia, err := makeLinkMedia(m, files)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check current media
	if linkedMedia.Cur.FileName != "file2.jpg" {
		t.Errorf("Expected Cur.FileName to be file2.jpg, got %v", linkedMedia.Cur.FileName)
	}

	// Check prev/next media
	// Adjust assertions based on the implementation details of makeLinkMedia
	if linkedMedia.Prev.FileName == "" && linkedMedia.Next.FileName == "" {
		t.Errorf("Expected at least one of Prev or Next to be populated")
	}
}

// Test isDir function
func TestIsDir(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "directory",
			path:     "folder",
			expected: true,
		},
		{
			name:     "file with extension",
			path:     "file.jpg",
			expected: false,
		},
		{
			name:     "path with slash",
			path:     "folder/subfolder",
			expected: true,
		},
		{
			name:     "file with path",
			path:     "folder/file.jpg",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDir(tt.path)
			if result != tt.expected {
				t.Errorf("isDir(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// Test getEnv function
func TestGetEnv(t *testing.T) {
	// Save any existing env var
	oldValue := os.Getenv("TEST_ENV_VAR")
	defer os.Setenv("TEST_ENV_VAR", oldValue)

	tests := []struct {
		name     string
		envVar   string
		envValue string
		fallback string
		expected string
	}{
		{
			name:     "env var set",
			envVar:   "TEST_ENV_VAR",
			envValue: "set-value",
			fallback: "fallback-value",
			expected: "set-value",
		},
		{
			name:     "env var is empty",
			envVar:   "TEST_ENV_VAR",
			envValue: "",
			fallback: "fallback-value",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable for the test
			os.Setenv(tt.envVar, tt.envValue)

			result := getEnv(tt.envVar, tt.fallback)
			if result != tt.expected {
				t.Errorf("getEnv(%q, %q) = %v, want %v", tt.envVar, tt.fallback, result, tt.expected)
			}
		})
	}
}

// Test getMediaType function
func TestGetMediaType(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected MediaFileType
	}{
		{
			name:     "jpg image",
			filePath: "image.jpg",
			expected: Image,
		},
		{
			name:     "jpeg image",
			filePath: "image.jpeg",
			expected: Image,
		},
		{
			name:     "png image",
			filePath: "image.png",
			expected: Image,
		},
		{
			name:     "webp image",
			filePath: "image.webp",
			expected: Image,
		},
		{
			name:     "mp4 video",
			filePath: "video.mp4",
			expected: Video,
		},
		{
			name:     "mov video",
			filePath: "video.mov",
			expected: Video,
		},
		{
			name:     "webm video",
			filePath: "video.webm",
			expected: Video,
		},
		{
			name:     "directory",
			filePath: "folder",
			expected: Other,
		},
		{
			name:     "other file type",
			filePath: "document.pdf",
			expected: Other,
		},
		{
			name:     "uppercase extension",
			filePath: "image.JPG",
			expected: Image,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMediaType(path.Ext(tt.filePath))
			if result != tt.expected {
				t.Errorf("getMediaType(%q) = %v, want %v", tt.filePath, result, tt.expected)
			}
		})
	}
}
func TestFilterDirEntries(t *testing.T) {
	entries := []fs.DirEntry{
		&mockDirEntry{name: "post_1.jpg", isDir: false},
		&mockDirEntry{name: "reel_2.mp4", isDir: false},
		&mockDirEntry{name: "story_3.jpg", isDir: false},
		&mockDirEntry{name: "2023", isDir: true},
		&mockDirEntry{name: "album", isDir: true},
	}

	tests := []struct {
		name             string
		filter           string
		expectedLen      int
		expectedContains []string
	}{
		{
			name:             "empty filter",
			filter:           "",
			expectedLen:      5, // All entries
			expectedContains: []string{"post_1.jpg", "reel_2.mp4", "story_3.jpg", "2023", "album"},
		},
		{
			name:             "single filter word",
			filter:           "post",
			expectedLen:      1, // Only post_1.jpg
			expectedContains: []string{"post_1.jpg"},
		},
		{
			name:             "multiple filter words",
			filter:           "post reel",
			expectedLen:      2, // post_1.jpg + reel_2.mp4
			expectedContains: []string{"post_1.jpg", "reel_2.mp4"},
		},
		{
			name:             "non-matching filter",
			filter:           "nonexistent",
			expectedLen:      0, // No matches
			expectedContains: []string{},
		},
		{
			name:             "filter with empty word",
			filter:           "story  reel",
			expectedLen:      5, // All entries due to empty word
			expectedContains: []string{"post_1.jpg", "reel_2.mp4", "story_3.jpg", "2023", "album"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterDirEntries(entries, tt.filter)

			if len(result) != tt.expectedLen {
				t.Errorf("filterDirEntries(%v, %q) length = %v, want %v", "entries", tt.filter, len(result), tt.expectedLen)
			}

			// Check all expected entries are present
			for _, expected := range tt.expectedContains {
				found := false
				for _, entry := range result {
					if entry.Name() == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("filterDirEntries(%v, %q) should contain %q but it doesn't", "entries", tt.filter, expected)
				}
			}
		})
	}
}

// Test listFsItems function
func TestListFsItems(t *testing.T) {
	// Create a mock filesystem
	mockFs := fstest.MapFS{
		"file1.txt":           &fstest.MapFile{},
		"file2.jpg":           &fstest.MapFile{},
		"subfolder/file3.mp4": &fstest.MapFile{},
		".hidden":             &fstest.MapFile{},
	}

	tests := []struct {
		name        string
		path        string
		expectedLen int
		expectedErr bool
	}{
		{
			name:        "root directory",
			path:        ".",
			expectedLen: 4, // file1.txt, file2.jpg, .hidden, subfolder
			expectedErr: false,
		},
		{
			name:        "subfolder",
			path:        "subfolder",
			expectedLen: 1, // file3.mp4
			expectedErr: false,
		},
		{
			name:        "nonexistent directory",
			path:        "nonexistent",
			expectedLen: 0,
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := listFsItems(mockFs, tt.path)

			if tt.expectedErr && err == nil {
				t.Errorf("listFsItems(%v, %q) expected error but got none", "mockFs", tt.path)
				return
			}

			if !tt.expectedErr && err != nil {
				t.Errorf("listFsItems(%v, %q) unexpected error: %v", "mockFs", tt.path, err)
				return
			}

			if !tt.expectedErr && len(result) != tt.expectedLen {
				t.Errorf("listFsItems(%v, %q) returned %d items, want %d", "mockFs", tt.path, len(result), tt.expectedLen)
			}
		})
	}
}

// Test writeError function
func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
	}{
		{
			name:       "not found error",
			statusCode: http.StatusNotFound,
			message:    "Resource not found",
		},
		{
			name:       "internal server error",
			statusCode: http.StatusInternalServerError,
			message:    "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a response recorder
			w := httptest.NewRecorder()

			// Call the function
			writeError(w, tt.statusCode, tt.message)

			// Check the status code
			if w.Code != tt.statusCode {
				t.Errorf("writeError() status code = %v, want %v", w.Code, tt.statusCode)
			}

			// Check the response body
			if w.Body.String() != tt.message {
				t.Errorf("writeError() body = %q, want %q", w.Body.String(), tt.message)
			}
		})
	}
}

// Test digitalOceanSpacesFS function
func TestDigitalOceanSpacesFS(t *testing.T) {
	// Mock file list function
	mockFiles := []string{
		"folder1/image1.jpg",
		"folder1/image2.jpg",
		"folder2/video1.mp4",
		"", // Empty path should be ignored
	}
	mockFileListFn := func() ([]string, error) {
		return mockFiles, nil
	}

	// Create the filesystem
	fs, update := digitalOceanSpacesFS(mockFileListFn)

	// Test initial state before update
	if _, err := fs.Open("folder1/image1.jpg"); err == nil {
		t.Error("Expected error before update, got nil")
	}

	// Test update function
	if err := update(); err != nil {
		t.Errorf("update() returned unexpected error: %v", err)
	}

	// Test that files exist after update
	for _, path := range mockFiles {
		if path == "" {
			continue
		}
		if _, err := fs.Open(path); err != nil {
			t.Errorf("fs.Open(%q) returned unexpected error: %v", path, err)
		}
	}

	// Test non-existent file
	if _, err := fs.Open("nonexistent.jpg"); err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	// Test error case
	errorFileListFn := func() ([]string, error) {
		return nil, fmt.Errorf("mock error")
	}
	_, errorUpdate := digitalOceanSpacesFS(errorFileListFn)
	if err := errorUpdate(); err == nil {
		t.Error("Expected error from update with failing fileListFn, got nil")
	}

	// Test that update clears previous files
	mockFiles = []string{"newfile.jpg"}
	if err := update(); err != nil {
		t.Errorf("second update() returned unexpected error: %v", err)
	}

	// Previous files should no longer exist
	if _, err := fs.Open("folder1/image1.jpg"); err == nil {
		t.Error("Expected error for old file after update, got nil")
	}

	// New file should exist
	if _, err := fs.Open("newfile.jpg"); err != nil {
		t.Errorf("fs.Open(%q) returned unexpected error: %v", "newfile.jpg", err)
	}
}

// mockDirEntry implements fs.DirEntry for testing
type mockDirEntry struct {
	name  string
	isDir bool
}

func (m *mockDirEntry) Name() string               { return m.name }
func (m *mockDirEntry) IsDir() bool                { return m.isDir }
func (m *mockDirEntry) Type() fs.FileMode          { return 0 }
func (m *mockDirEntry) Info() (fs.FileInfo, error) { return nil, nil }
