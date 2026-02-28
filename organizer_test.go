package main

import (
	"os"
	"path/filepath"
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

func TestResolveCollision(t *testing.T) {
	// Create a temporary directory unique to this test run
	tmpDir := t.TempDir()
	o := NewOrganizer(tmpDir, false)

	// Scenario 1: File does not exist
	// We create a path inside our empty temp directory
	path := filepath.Join(tmpDir, "test_file.txt")
	result := o.resolveCollision(path)

	if result != path {
		t.Errorf("Expected original path %s, got %s", path, result)
	}

	// Scenario 2: File exists (Collision)
	// We create the file manually to force the logic to trigger
	if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(tmpDir, "test_file_1.txt")
	result = o.resolveCollision(path)

	if result != expected {
		t.Errorf("Expected collision path %s, got %s", expected, result)
	}
}
