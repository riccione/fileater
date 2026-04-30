package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type HistoryState struct {
	MovedFiles  map[string]string `json:"moved_files"`
	DeletedDirs []string         `json:"deleted_dirs"`
	RootPath    string           `json:"root_path"`
}

func Undo(rootPath string, dryRun bool) error {
	statePath := filepath.Join(rootPath, ".fileater-history.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return fmt.Errorf("history file not found: %s", statePath)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("failed to read history file: %w", err)
	}

	var state HistoryState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse history file: %w", err)
	}

	var failures []string

	if dryRun {
		log.Println("[DRY RUN] Showing what would be restored:")
	}

	for _, dir := range state.DeletedDirs {
		if !isSubPath(rootPath, dir) {
			log.Printf("skipping directory outside root: %s", dir)
			continue
		}
		if dryRun {
			log.Printf("[DRY RUN] Would recreate directory: %s", dir)
		} else {
			if err := os.MkdirAll(dir, 0755); err != nil {
				msg := fmt.Sprintf("failed to recreate directory %s: %v", dir, err)
				log.Println(msg)
				failures = append(failures, msg)
			} else {
				log.Printf("recreated directory: %s", dir)
			}
		}
	}

	for currentPath, originalPath := range state.MovedFiles {
		if _, err := os.Stat(currentPath); os.IsNotExist(err) {
			msg := fmt.Sprintf("current path not found, skipping: %s", currentPath)
			log.Println(msg)
			if !dryRun {
				failures = append(failures, msg)
			}
			continue
		}

		originalDir := filepath.Dir(originalPath)
		if dryRun {
			log.Printf("[DRY RUN] Would move %s back to %s", currentPath, originalPath)
			log.Printf("[DRY RUN] Would create parent directory if needed: %s", originalDir)
		} else {
			if err := os.MkdirAll(originalDir, 0755); err != nil {
				msg := fmt.Sprintf("failed to create parent directory %s: %v", originalDir, err)
				log.Println(msg)
				failures = append(failures, msg)
				continue
			}

			if err := os.Rename(currentPath, originalPath); err != nil {
				msg := fmt.Sprintf("failed to move %s back to %s: %v", currentPath, originalPath, err)
				log.Println(msg)
				failures = append(failures, msg)
			} else {
				log.Printf("moved back: %s -> %s", currentPath, originalPath)
			}
		}
	}

	if !dryRun {
		if err := os.Remove(statePath); err != nil {
			log.Printf("warning: failed to delete history file: %v", err)
		} else {
			log.Printf("deleted history file: %s", statePath)
		}

		if len(failures) > 0 {
			return fmt.Errorf("undo completed with %d failure(s): %v", len(failures), failures)
		}
	} else {
		log.Printf("[DRY RUN] Would delete history file: %s", statePath)
	}

	return nil
}

// isSubPath checks if target path is within root path, ensuring proper path boundary
func isSubPath(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)

	if root == target {
		return true
	}

	rootWithSep := root + string(filepath.Separator)
	return strings.HasPrefix(target, rootWithSep)
}
