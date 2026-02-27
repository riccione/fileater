package main

import (
	"testing"
)

func TestCategorizeFile(t *testing.T) {
	o := NewOrganizer(".", true)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"Video MP4", "movie.mp4", "video"},
		{"Video MKV", "video.mkv", "video"},
		{"Audio MP3", "song.mp3", "audio"},
		{"Document PDF", "report.pdf", "docs"},
		{"Document TXT", "notes.txt", "docs"},
		{"Unknown Extension", "archive.zip", "mix"},
		{"No Extension", "README", "mix"},
		{"Uppercase Extension", "IMAGE.PNG", "mix"}, // PNG isn't in our maps yet
		{"Mixed Case Video", "CLIP.mKv", "video"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := o.categorizeFile(tt.path)
			if result != tt.expected {
				t.Errorf("categorizeFile(%s) = %s; want %s", tt.path, result, tt.expected)
			}
		})
	}
}

func TestResolveCollisionString(t *testing.T) {
	// Note: To test the actual file system, we would need os.Create
	// This is a logic-only test for the naming pattern
	o := NewOrganizer(".", true)

	// Test base case: if file doesn't exist, return original
	// (In a real test, we'd use a temp directory)
	path := "test_file.txt"
	result := o.resolveCollision(path)

	if result != path {
		t.Errorf("Expected %s, got %s", path, result)
	}
}
