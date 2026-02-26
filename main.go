package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
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

func main() {
	// Flag parsing
	dryRun := flag.Bool("dryrun", false, "Simulate the operation without moving files")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <path>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Organizes files recursively into categorized folders.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Validate Arguments
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Error: exactly one path argument is required\n")
		flag.Usage()
		os.Exit(1)
	}

	rootPath := flag.Arg(0)

	// Setup context with Signal Handling (Ctrl+C)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\nInterrupt received, shutting down...")
		cancel()
	}()

	// Validate Root Path
	info, err := os.Stat(rootPath)
	if err != nil {
		log.Fatalf("Error accessing path '%s': %v", rootPath, err)
	}
	if !info.IsDir() {
		log.Fatalf("Path '%s' is not a directory", rootPath)
	}

	// Resolve to absolute path to avoid relative path confusion
	rootPath, err = filepath.Abs(rootPath)
	if err != nil {
		log.Fatalf("Error resolving absolute path: %v", err)
	}

	log.Printf("Starting organization of: %s", rootPath)
	if *dryRun {
		log.Println("Mode: DRY RUN (No files will be moved)")
	}

	// Pre-create target directories to ensure they exist and to identify them during walk
	targetPaths := make(map[string]struct{})
	for _, dirName := range targetDirs {
		dirPath := filepath.Join(rootPath, dirName)
		targetPaths[dirPath] = struct{}{}

		if !*dryRun {
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				log.Fatalf("Failed to create directory '%s': %v", dirPath, err)
			}
		} else {
			// In dryrun, we just log that we would create them
			log.Printf("[DRYRUN] Would create directory: %s", dirPath)
		}
	}

	// Walk the Directory Tree
	// We use a counter for stats
	var processedCount int
	var errorCount int

	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		// Check context for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			log.Printf("Error accessing path %s: %v", path, err)
			errorCount++
			return nil // Continue walking despite error
		}

		// Skip directories (we only process files)
		if d.IsDir() {
			// Optimization: If we are inside a target directory, skip it entirely
			// to avoid processing files we just moved or existing organized files.
			if _, isTarget := targetPaths[path]; isTarget {
				return filepath.SkipDir
			}
			return nil
		}

		// Process File
		category := categorizeFile(path)
		destDir := filepath.Join(rootPath, category)
		destPath := filepath.Join(destDir, d.Name())

		// Safety check: Don't move if source is already destination
		if path == destPath {
			return nil
		}

		// Handle Name Collisions
		if !*dryRun {
			if _, err := os.Stat(destPath); err == nil {
				destPath = resolveCollision(destPath)
			}
		}

		if *dryRun {
			log.Printf("[DRYRUN] Would move: %s -> %s (%s)", path, destPath, category)
		} else {
			if err := os.Rename(path, destPath); err != nil {
				log.Printf("Error moving %s: %v", path, err)
				errorCount++
				return nil
			}
			log.Printf("Moved: %s -> %s (%s)", path, destPath, category)
		}

		processedCount++
		return nil
	})

	if err != nil {
		log.Fatalf("Walk failed: %v", err)
	}

	log.Printf("Finished. Processed: %d files, Errors: %d", processedCount, errorCount)
}

// categorizeFile determines the folder category based on extension
func categorizeFile(path string) string {
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
func resolveCollision(path string) string {
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
