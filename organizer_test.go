package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCategorizeFile(t *testing.T) {
	o := NewOrganizer(".", true)

	// Manually populate categories to simulate a loaded config
	o.Categories = map[string]map[string]struct{}{
		"video":  {".mp4": {}, ".mkv": {}, ".avi": {}},
		"audio":  {".mp3": {}, ".wav": {}},
		"docs":   {".pdf": {}, ".txt": {}},
		"images": {".jpg": {}, ".png": {}},
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"Video MP4", "movie.mp4", "video"},
		{"Video MKV", "video.mkv", "video"},
		{"Audio MP3", "song.mp3", "audio"},
		{"Document PDF", "report.pdf", "docs"},
		{"Image PNG", "photo.png", "images"},
		{"Unknown Extension", "archive.zip", "mix"},
		{"No Extension", "README", "mix"},
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

func TestRun_CreatesDirectories(t *testing.T) {
	// Setup a clean environment
	tmpDir := t.TempDir()
	ctx := context.Background()
	o := NewOrganizer(tmpDir, false) // false = Not a dry run, actually create them

	// Define custom categories
	o.Categories = map[string]map[string]struct{}{
		"documents": {".pdf": {}},
		"media":     {".mp4": {}},
	}

	// Execute the Run method
	// This will trigger the directory creation logic
	if err := o.Run(ctx); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify the directories exist on disk
	expectedDirs := []string{"documents", "media", "mix"}

	for _, name := range expectedDirs {
		path := filepath.Join(tmpDir, name)
		info, err := os.Stat(path)

		if os.IsNotExist(err) {
			t.Errorf("Expected directory %s was not created", name)
			continue
		}
		if !info.IsDir() {
			t.Errorf("Path %s exists but is not a directory", name)
		}
	}
}

func TestMoveFile(t *testing.T) {
	tmpDir := t.TempDir()
	o := NewOrganizer(tmpDir, false)

	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "destination.txt")

	// Create a dummy source file
	content := []byte("hello world")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatal(err)
	}

	// This will fail to compile because o.moveFile isn't defined yet
	err := o.moveFile(src, dst)
	if err != nil {
		t.Errorf("moveFile failed: %v", err)
	}

	// Verify destination exists and source is gone
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("Destination file was not created")
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("Source file still exists after move")
	}
}

func TestRun_NonRecursiveByDefault(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file in root
	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root"), 0644)

	// Create a file in a subdir
	subDir := filepath.Join(tmpDir, "my_subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0644)

	// o.Recursive is false by default
	o := NewOrganizer(tmpDir, false)
	o.UseDefaultCategories()

	ctx := context.Background()
	o.Run(ctx)

	// Check: nested.txt should still be in my_subdir, not moved to docs/
	nestedPath := filepath.Join(subDir, "nested.txt")
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Errorf("Subdirectory file was moved, but should have been ignored by default")
	}
}
