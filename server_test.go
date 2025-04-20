package main

import (
	"io/fs"
	"testing"
)

// Test cases
func TestMakePathFromUrl(t *testing.T) {
	tests := []struct {
		url           string
		expectedMedia Media
	}{
		{"directory/", Media{
			Type:            Directory,
			PublicPath:      "/public/media/directory",
			FileName:        "",
			LocalPath:       "/home/bob/media/directory",
			RelativePageURL: "directory",
			AbsolutePageURL: "/gallery/directory",
		}},
		{"directory/file.jpg", Media{
			Type:            Image,
			PublicPath:      "/public/media/directory/file.jpg",
			FileName:        "file.jpg",
			LocalPath:       "/home/bob/media/directory/file.jpg",
			RelativePageURL: "directory/file.jpg",
			AbsolutePageURL: "/gallery/directory/file.jpg",
		}},
	}

	conf := Config{
		WebRoot:    "/public/media",
		ServerRoot: "/gallery",
		LocalRoot:  "/home/bob/media",
	}

	for _, test := range tests {
		t.Run(test.url, func(t *testing.T) {
			media := makeMedia(test.url, conf)

			if media.Type != test.expectedMedia.Type {
				t.Errorf("Expected Type %v, got %v", test.expectedMedia.Type, media.Type)
			}
			if media.PublicPath != test.expectedMedia.PublicPath {
				t.Errorf("Expected PublicPath %v, got %v", test.expectedMedia.PublicPath, media.PublicPath)
			}
			if media.LocalPath != test.expectedMedia.LocalPath {
				t.Errorf("Expected LocalPath %v, got %v", test.expectedMedia.LocalPath, media.LocalPath)
			}
			if media.RelativePageURL != test.expectedMedia.RelativePageURL {
				t.Errorf("Expected RelativeURL %v, got %v", test.expectedMedia.RelativePageURL, media.RelativePageURL)
			}
			if media.AbsolutePageURL != test.expectedMedia.AbsolutePageURL {
				t.Errorf("Expected AbsolutePageURL %v, got %v", test.expectedMedia.AbsolutePageURL, media.AbsolutePageURL)
			}
			if media.FileName != test.expectedMedia.FileName {
				t.Errorf("Expected FileName %v, got %v", test.expectedMedia.FileName, media.FileName)
			}
		})
	}
}

func TestExtractNumbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int64
	}{
		{
			name:     "post with timestamp and index",
			input:    "post_1577580588_0.jpg",
			expected: []int64{1577580588, 0},
		},
		{
			name:     "year only",
			input:    "2024",
			expected: []int64{2024},
		},
		{
			name:     "multiple numbers in name",
			input:    "album_2024_01_15_001.jpg",
			expected: []int64{2024, 1, 15, 1},
		},
		{
			name:     "no numbers",
			input:    "album.jpg",
			expected: []int64{},
		},
		{
			name:     "numbers at start and end",
			input:    "2024_album_001",
			expected: []int64{2024, 1},
		},
		{
			name:     "consecutive numbers",
			input:    "1234567890",
			expected: []int64{1234567890},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractNumbers(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("extractNumbers(%q) length mismatch: got %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("extractNumbers(%q) at index %d: got %v, want %v", tt.input, i, result[i], tt.expected[i])
				}
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
			name: "simple numeric comparison",
			files: []fs.DirEntry{
				&mockDirEntry{name: "post_1.jpg"},
				&mockDirEntry{name: "post_3.jpg"},
				&mockDirEntry{name: "post_2.jpg"},
			},
			expected: []string{"post_3.jpg", "post_2.jpg", "post_1.jpg"},
		},
		{
			name: "multiple numbers in filename",
			files: []fs.DirEntry{
				&mockDirEntry{name: "post_2024_01_15.jpg"},
				&mockDirEntry{name: "post_2024_01_14.jpg"},
				&mockDirEntry{name: "post_2024_01_16.jpg"},
			},
			expected: []string{"post_2024_01_16.jpg", "post_2024_01_15.jpg", "post_2024_01_14.jpg"},
		},
		{
			name: "different number lengths",
			files: []fs.DirEntry{
				&mockDirEntry{name: "post_1.jpg"},
				&mockDirEntry{name: "post_10.jpg"},
				&mockDirEntry{name: "post_2.jpg"},
			},
			expected: []string{"post_10.jpg", "post_2.jpg", "post_1.jpg"},
		},
		{
			name: "fallback to string comparison",
			files: []fs.DirEntry{
				&mockDirEntry{name: "post_b.jpg"},
				&mockDirEntry{name: "post_a.jpg"},
				&mockDirEntry{name: "post_c.jpg"},
			},
			expected: []string{"post_a.jpg", "post_b.jpg", "post_c.jpg"},
		},
		{
			name: "mixed numeric and non-numeric",
			files: []fs.DirEntry{
				&mockDirEntry{name: "post_2.jpg"},
				&mockDirEntry{name: "post_a.jpg"},
				&mockDirEntry{name: "post_1.jpg"},
			},
			expected: []string{"post_2.jpg", "post_1.jpg", "post_a.jpg"},
		},
		{
			name: "timestamp based filenames",
			files: []fs.DirEntry{
				&mockDirEntry{name: "post_1577580588_0.jpg"},
				&mockDirEntry{name: "post_1577580588_1.jpg"},
				&mockDirEntry{name: "post_1577580587_0.jpg"},
				&mockDirEntry{name: "post_1577580589_0.jpg"},
			},
			expected: []string{"post_1577580589_0.jpg", "post_1577580588_1.jpg", "post_1577580588_0.jpg", "post_1577580587_0.jpg"},
		},
		{
			name: "year directories",
			files: []fs.DirEntry{
				&mockDirEntry{name: "2023"},
				&mockDirEntry{name: "2025"},
				&mockDirEntry{name: "2024"},
			},
			expected: []string{"2025", "2024", "2023"},
		},
		{
			name: "directories before files",
			files: []fs.DirEntry{
				&mockDirEntry{name: "file1.jpg"},
				&mockDirEntry{name: "2024", isDir: true},
				&mockDirEntry{name: "file2.jpg"},
				&mockDirEntry{name: "2023", isDir: true},
			},
			expected: []string{"2024", "2023", "file2.jpg", "file1.jpg"},
		},
		{
			name: "directories sorted among themselves",
			files: []fs.DirEntry{
				&mockDirEntry{name: "2023", isDir: true},
				&mockDirEntry{name: "2025", isDir: true},
				&mockDirEntry{name: "2024", isDir: true},
				&mockDirEntry{name: "file.jpg"},
			},
			expected: []string{"2025", "2024", "2023", "file.jpg"},
		},
		{
			name: "files sorted among themselves after directories",
			files: []fs.DirEntry{
				&mockDirEntry{name: "file2.jpg"},
				&mockDirEntry{name: "2024", isDir: true},
				&mockDirEntry{name: "file1.jpg"},
				&mockDirEntry{name: "file3.jpg"},
			},
			expected: []string{"2024", "file3.jpg", "file2.jpg", "file1.jpg"},
		},
		{
			name: "directories before files with numeric sorting",
			files: []fs.DirEntry{
				&mockDirEntry{name: "post_2.jpg"},
				&mockDirEntry{name: "2024", isDir: true},
				&mockDirEntry{name: "post_1.jpg"},
				&mockDirEntry{name: "2023", isDir: true},
			},
			expected: []string{"2024", "2023", "post_2.jpg", "post_1.jpg"},
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

// mockDirEntry implements fs.DirEntry for testing
type mockDirEntry struct {
	name  string
	isDir bool
}

func (m *mockDirEntry) Name() string               { return m.name }
func (m *mockDirEntry) IsDir() bool                { return m.isDir }
func (m *mockDirEntry) Type() fs.FileMode          { return 0 }
func (m *mockDirEntry) Info() (fs.FileInfo, error) { return nil, nil }
