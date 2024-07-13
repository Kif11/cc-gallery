package main

import (
	"testing"
)

// Test cases
func TestMakePathFromUrl(t *testing.T) {
	tests := []struct {
		url           string
		expectedMedia Media
	}{
		{"directory/", Media{
			Type:        Directory,
			PublicPath:  "/public/media/directory",
			LocalPath:   "/home/bob/media/directory",
			RelativeURL: "directory/",
			UrlPath:     "/gallery/directory",
		}},
	}

	conf := Config{
		CDNRoot:    "/public/media",
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
			if media.RelativeURL != test.expectedMedia.RelativeURL {
				t.Errorf("Expected RelativeURL %v, got %v", test.expectedMedia.RelativeURL, media.RelativeURL)
			}
			if media.UrlPath != test.expectedMedia.UrlPath {
				t.Errorf("Expected UrlPath %v, got %v", test.expectedMedia.UrlPath, media.UrlPath)
			}
			if media.FileName != test.expectedMedia.FileName {
				t.Errorf("Expected FileName %v, got %v", test.expectedMedia.FileName, media.FileName)
			}
		})
	}
}
