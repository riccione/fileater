package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Category definitions for file extensions
var (
	videoExts = map[string]struct{}{
		".mp4": {}, ".mkv": {}, ".avi": {}, ".mov": {}, ".wmv": {}, ".flv": {}, ".webm": {},
	}
	audioExts = map[string]struct{}{
		".mp3": {}, ".wav": {}, ".ogg": {}, ".flac": {}, ".aac": {}, ".m4a": {}, ".wma": {},
	}
	docsExts = map[string]struct{}{
		".pdf": {}, ".doc": {}, ".docx": {}, ".txt": {}, ".md": {}, ".rtf": {}, ".odt": {}, ".xlsx": {}, ".pptx": {},
	}
	// Target directory names
	targetDirs = []string{"video", "docs", "audio", "mix"}
)

type Organizer struct {
	RootPath    string
	DryRun      bool
	TargetPaths map[string]struct{}
}

func NewOrganizer(root string, dryRun bool) *Organizer {
	return &Organizer{
		RootPath:    root,
		DryRun:      dryRun,
		TargetPaths: make(map[string]struct{}),
	}
}

// processFile determines the destination and moves the file
func (o *Organizer) processFile(path string, d fs.DirEntry) error {
	category := o.categorizeFile(path)
	destDir := filepath.Join(o.RootPath, category)
	destPath := filepath.Join(destDir, d.Name())

	// Safety check: Don't move if source is already the destination
	if path == destPath {
		return nil
	}

	// Handle name collisions only if not in dry-run
	finalDest := destPath
	if !o.DryRun {
		finalDest = o.resolveCollision(destPath)
	}

	if o.DryRun {
		log.Printf("[DRYRUN] Would move: %s -> %s (%s)", d.Name(), finalDest, category)
		return nil
	}

	// Perform the move
	if err := os.Rename(path, finalDest); err != nil {
		return err
	}

	log.Printf("Moved: %s -> %s (%s)", d.Name(), filepath.Base(finalDest), category)
	return nil
}

// categorizeFile determines the folder category based on extension
func (o *Organizer) categorizeFile(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	if _, ok := videoExts[ext]; ok {
		return "video"
	}
	if _, ok := audioExts[ext]; ok {
		return "audio"
	}
	if _, ok := docsExts[ext]; ok {
		return "docs"
	}
	return "mix"
}

// resolveCollision appends a counter to the filename if a file already exists
// Example: file.txt -> file_1.txt
func (o *Organizer) resolveCollision(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	counter := 1
	for {
		newBase := fmt.Sprintf("%s_%d%s", name, counter, ext)
		newPath := filepath.Join(dir, newBase)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
		counter++
	}
}

// Run executes the organization process
func (o *Organizer) Run(ctx context.Context) error {
	// Path validation and resolution
	absPath, err := filepath.Abs(o.RootPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	o.RootPath = absPath

	// Prepare target directories
	for _, dirName := range targetDirs {
		dirPath := filepath.Join(o.RootPath, dirName)
		o.TargetPaths[dirPath] = struct{}{}

		if !o.DryRun {
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
			}
		} else {
			log.Printf("[DRYRUN] Would create directory: %s", dirPath)
		}
	}

	// Walk the directory tree
	var processedCount, errorCount int
	err = filepath.WalkDir(o.RootPath, func(path string, d fs.DirEntry, err error) error {
		// Check if context was cancelled (Ctrl+C)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			log.Printf("Error accessing path %s: %v", path, err)
			errorCount++
			return nil
		}

		// Skip directories and existing target folders
		if d.IsDir() {
			if _, isTarget := o.TargetPaths[path]; isTarget && path != o.RootPath {
				return filepath.SkipDir
			}
			return nil
		}

		// Process individual file
		if err := o.processFile(path, d); err != nil {
			log.Printf("Error moving %s: %v", path, err)
			errorCount++
		} else {
			processedCount++
		}

		return nil
	})

	log.Printf("Finished. Processed: %d files, Errors: %d", processedCount, errorCount)
	return err
}
