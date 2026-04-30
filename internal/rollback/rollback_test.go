package rollback

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUndo_NoHistoryFile(t *testing.T) {
	tmpDir := t.TempDir()

	err := Undo(tmpDir, false)
	if err == nil {
		t.Fatal("expected error when no history file exists")
	}
}

func TestUndo_RestoreFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create organized structure
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	originalFile := filepath.Join(tmpDir, "test.txt")
	movedFile := filepath.Join(docsDir, "test.txt")

	os.WriteFile(originalFile, []byte("content"), 0644)
	os.Rename(originalFile, movedFile)

	// Create history file
	state := HistoryState{
		MovedFiles: map[string]string{
			movedFile: originalFile,
		},
		DeletedDirs: []string{},
		RootPath:    tmpDir,
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, ".fileater-history.json"), data, 0644)

	// Run undo
	if err := Undo(tmpDir, false); err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	// Verify file is back
	if _, err := os.Stat(originalFile); os.IsNotExist(err) {
		t.Error("original file was not restored")
	}
	if _, err := os.Stat(movedFile); !os.IsNotExist(err) {
		t.Error("moved file should be gone")
	}

	// Verify history file is deleted
	if _, err := os.Stat(filepath.Join(tmpDir, ".fileater-history.json")); !os.IsNotExist(err) {
		t.Error("history file should be deleted after undo")
	}
}

func TestUndo_RestoreDeletedDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create and delete a directory to simulate cleanup
	deletedDir := filepath.Join(tmpDir, "old_subdir")
	os.MkdirAll(deletedDir, 0755)
	os.Remove(deletedDir)

	// Create history file with deleted dir
	state := HistoryState{
		MovedFiles:  map[string]string{},
		DeletedDirs: []string{deletedDir},
		RootPath:    tmpDir,
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, ".fileater-history.json"), data, 0644)

	// Run undo
	if err := Undo(tmpDir, false); err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	// Verify directory is recreated
	if _, err := os.Stat(deletedDir); os.IsNotExist(err) {
		t.Error("deleted directory was not recreated")
	}
}

func TestUndo_MissingSourceFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create history file pointing to non-existent file
	state := HistoryState{
		MovedFiles: map[string]string{
			filepath.Join(tmpDir, "docs", "nonexistent.txt"): filepath.Join(tmpDir, "nonexistent.txt"),
		},
		DeletedDirs: []string{},
		RootPath:    tmpDir,
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, ".fileater-history.json"), data, 0644)

	// Should return error since source file is missing
	err := Undo(tmpDir, false)
	if err == nil {
		t.Fatal("expected error when source file is missing")
	}
}

func TestUndo_InvalidHistoryFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid JSON
	os.WriteFile(filepath.Join(tmpDir, ".fileater-history.json"), []byte("invalid json"), 0644)

	err := Undo(tmpDir, false)
	if err == nil {
		t.Fatal("expected error for invalid history file")
	}
}

func TestUndo_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create organized structure
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	originalFile := filepath.Join(tmpDir, "test.txt")
	movedFile := filepath.Join(docsDir, "test.txt")

	os.WriteFile(originalFile, []byte("content"), 0644)
	os.Rename(originalFile, movedFile)

	// Create history file
	state := HistoryState{
		MovedFiles: map[string]string{
			movedFile: originalFile,
		},
		DeletedDirs: []string{filepath.Join(tmpDir, "olddir")},
		RootPath:    tmpDir,
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, ".fileater-history.json"), data, 0644)

	// Run undo with dryRun=true
	if err := Undo(tmpDir, true); err != nil {
		t.Fatalf("Dry-run Undo failed: %v", err)
	}

	// Verify file is NOT back (dry run should not move)
	if _, err := os.Stat(originalFile); !os.IsNotExist(err) {
		t.Error("original file should NOT be restored in dry-run mode")
	}
	if _, err := os.Stat(movedFile); os.IsNotExist(err) {
		t.Error("moved file should still exist in dry-run mode")
	}

	// Verify history file is NOT deleted
	if _, err := os.Stat(filepath.Join(tmpDir, ".fileater-history.json")); os.IsNotExist(err) {
		t.Error("history file should NOT be deleted in dry-run mode")
	}
}
