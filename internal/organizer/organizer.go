package organizer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/riccione/fileater/internal/rollback"
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
	Recursive   bool
	Logger      *slog.Logger
	TargetPaths map[string]struct{}
	// map of Category name => set of ext
	Categories map[string]map[string]struct{}

	startTime  time.Time
	totalBytes int64

	minSize int64
	maxSize int64

	deleteDupes bool

	movedFiles  map[string]string
	deletedDirs []string
}

type Metrics struct {
	ExecutionTime  time.Duration
	FilesProcessed int
	TotalBytes     int64
	FilesPerSecond float64
}

func (m Metrics) String() string {
	var dataStr string
	if m.TotalBytes >= 1<<30 {
		dataStr = fmt.Sprintf("%.2f GB", float64(m.TotalBytes)/(1<<30))
	} else {
		dataStr = fmt.Sprintf("%.2f MB", float64(m.TotalBytes)/(1<<20))
	}

	return fmt.Sprintf(
		"Summary\n"+
			"+----------------------+-------------+\n"+
			"| Metric               | Value       |\n"+
			"+----------------------+-------------+\n"+
			"| Total Exec Time      | %-11s |\n"+
			"| Total Files          | %-11d |\n"+
			"| Total Data Moved     | %-11s |\n"+
			"| Avg Files/sec        | %-11.2f |\n"+
			"+----------------------+-------------+\n",
		m.ExecutionTime.Round(time.Second),
		m.FilesProcessed,
		dataStr,
		m.FilesPerSecond,
	)
}

func NewOrganizer(root string, dryRun bool, recursive bool, logger *slog.Logger, minSizeStr, maxSizeStr string, deleteDupes bool) (*Organizer, error) {
	o := &Organizer{
		RootPath:    root,
		DryRun:      dryRun,
		Recursive:   recursive,
		Logger:      logger,
		TargetPaths: make(map[string]struct{}),
		Categories:  make(map[string]map[string]struct{}),
		deleteDupes: deleteDupes,
		movedFiles:  make(map[string]string),
		deletedDirs: []string{},
	}

	minSize, err := ParseSize(minSizeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid min-size: %w", err)
	}
	o.minSize = minSize

	maxSize, err := ParseSize(maxSizeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid max-size: %w", err)
	}
	o.maxSize = maxSize

	if minSize > 0 && maxSize > 0 && minSize > maxSize {
		return nil, fmt.Errorf("min-size cannot be greater than max-size")
	}

	return o, nil
}

func (o *Organizer) hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	buf := make([]byte, 32*1024) // 32KB buffer for memory efficiency

	for {
		n, err := file.Read(buf)
		if n > 0 {
			if _, writeErr := hasher.Write(buf[:n]); writeErr != nil {
				return "", writeErr
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (o *Organizer) findDuplicate(srcSize int64, srcHash string, destDir string) (string, error) {
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		destPath := filepath.Join(destDir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.Size() != srcSize {
			continue
		}

		destHash, err := o.hashFile(destPath)
		if err != nil {
			continue
		}

		if srcHash == destHash {
			return destPath, nil
		}
	}

	return "", nil
}

// UseDefaultCategories sets up the initial categories if no JSON is provided
func (o *Organizer) UseDefaultCategories() {
	o.Categories = map[string]map[string]struct{}{
		"video": {".mp4": {}, ".mkv": {}, ".avi": {}},
		"audio": {".mp3": {}, ".wav": {}, ".flac": {}},
		"docs":  {".pdf": {}, ".docx": {}, ".txt": {}, ".md": {}},
	}
}

// LoadConfig reads JSON file and populates Categories
func (o *Organizer) LoadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Temporary map to decode JSON
	var rawConfig map[string][]string
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		return err
	}

	// Convert to our internal map[string]map[string]struct{} for O(1) lookup
	for cat, exts := range rawConfig {
		o.Categories[cat] = make(map[string]struct{})
		for _, ext := range exts {
			o.Categories[cat][strings.ToLower(ext)] = struct{}{}
		}
	}
	return nil
}

// processFile determines the destination and moves the file
func (o *Organizer) processFile(path string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	srcSize := info.Size()

	category := o.categorizeFile(path)
	destDir := filepath.Join(o.RootPath, category)

	// Duplicate detection - check if destDir exists before hashing
	if _, err := os.Stat(destDir); err == nil {
		srcHash, err := o.hashFile(path)
		if err == nil {
			dupPath, err := o.findDuplicate(srcSize, srcHash, destDir)
			if err == nil && dupPath != "" {
				log.Printf("Duplicate found: %s matches existing file %s", d.Name(), dupPath)
				o.Logger.Info("Duplicate detected",
					"action", "DUPLICATE",
					"source", path,
					"duplicate", dupPath,
				)

				if o.deleteDupes {
					if err := os.Remove(path); err != nil {
						o.Logger.Error("Failed to delete duplicate source",
							"action", "DELETE",
							"source", path,
							"error", err.Error(),
						)
						return fmt.Errorf("failed to delete duplicate: %w", err)
					}
					log.Printf("Deleted duplicate: %s", path)
					o.Logger.Info("Duplicate deleted",
						"action", "DELETE",
						"source", path,
						"duplicate_of", dupPath,
					)
				}
				return nil
			}
		}
	}

	// Resolve collisions
	destPath := filepath.Join(destDir, d.Name())
	finalDest := o.resolveCollision(destPath)

	// Safety check: Don't move if source is already the destination
	if path == destPath {
		return nil
	}

	// Get file size for metrics (before move)
	var size int64
	if o.DryRun {
		fi, err := os.Stat(path)
		if err == nil {
			size = fi.Size()
		}
	} else {
		var err error
		size, err = o.moveFile(path, finalDest)
		if err != nil {
			o.Logger.Error("Move failed",
				"action", "MOVE",
				"source", path,
				"destination", finalDest,
				"error", err.Error(),
			)
			return fmt.Errorf("Move failed: %w", err)
		}
		o.movedFiles[finalDest] = path
	}

	o.totalBytes += size

	// Log only success outcome
	if !o.DryRun {
		log.Printf("Moved: %s => %s (%s)", d.Name(), filepath.Base(finalDest), category)
		o.Logger.Info("File moved",
			"action", "MOVE",
			"source", path,
			"destination", finalDest,
		)
	}

	return nil
}

func (o *Organizer) SaveHistory() error {
	state := rollback.HistoryState{
		MovedFiles:  o.movedFiles,
		DeletedDirs: o.deletedDirs,
		RootPath:    o.RootPath,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history state: %w", err)
	}

	statePath := filepath.Join(o.RootPath, ".fileater-history.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	log.Printf("History saved to: %s", statePath)
	return nil
}

// categorizeFile determines the folder category based on extension
func (o *Organizer) categorizeFile(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	for category, extensions := range o.Categories {
		if _, ok := extensions[ext]; ok {
			return category
		}
	}

	return "mix"
}

func ParseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, nil
	}

	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	type unitInfo struct {
		unit string
		mult int64
	}
	units := []unitInfo{
		{"TB", 1024 * 1024 * 1024 * 1024},
		{"T", 1024 * 1024 * 1024 * 1024},
		{"GB", 1024 * 1024 * 1024},
		{"G", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"M", 1024 * 1024},
		{"KB", 1024},
		{"K", 1024},
		{"B", 1},
	}

	for _, ui := range units {
		if strings.HasSuffix(sizeStr, ui.unit) {
			numStr := strings.TrimSuffix(sizeStr, ui.unit)
			if numStr == "" {
				return 0, fmt.Errorf("invalid size format: %s", sizeStr)
			}
			value, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid size value: %s", numStr)
			}
			return value * ui.mult, nil
		}
	}

	return 0, fmt.Errorf("unknown size unit in: %s", sizeStr)
}

// resolveCollision appends a counter to the filename if a file already exists
// Example: file.txt -> file_1.txt
func (o *Organizer) resolveCollision(path string) string {
	// Check if the original path is already available
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	// If it exists, start looking for _1, _2, etc.
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

// moveFile handles the physical relocation of files with safety fallbacks.
func (o *Organizer) moveFile(src, dst string) (int64, error) {
	if o.DryRun {
		log.Printf("[DRYRUN] Would move %s to %s", src, dst)
		return 0, nil
	}

	// Try atomic rename first
	err := os.Rename(src, dst)
	if err == nil {
		fi, err := os.Stat(dst)
		if err != nil {
			return 0, err
		}
		return fi.Size(), nil
	}

	// Fallback for cross-device or other rename failures
	sFile, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("failed to open source: %w", err)
	}
	defer sFile.Close()

	dFile, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("failed to create destination: %w", err)
	}
	defer dFile.Close()

	// Efficient streaming copy
	written, err := io.Copy(dFile, sFile)
	if err != nil {
		return 0, fmt.Errorf("copy failed: %w", err)
	}

	// Ensure data is flushed to disk before removing source
	if err := dFile.Sync(); err != nil {
		return 0, fmt.Errorf("sync failed: %w", err)
	}

	// Close handles before removal (crucial for Windows)
	sFile.Close()
	dFile.Close()

	if err := os.Remove(src); err != nil {
		return 0, err
	}

	return written, nil
}

// Run executes the organization process
func (o *Organizer) Run(ctx context.Context) error {
	o.startTime = time.Now()

	// Path validation and resolution
	absPath, err := filepath.Abs(o.RootPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	o.RootPath = absPath

	// Prepare target directories
	requiredDirs := []string{"mix"}
	for catName := range o.Categories {
		requiredDirs = append(requiredDirs, catName)
	}

	for _, dirName := range requiredDirs {
		dirPath := filepath.Join(o.RootPath, dirName)
		o.TargetPaths[dirPath] = struct{}{}

		if !o.DryRun {
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				o.Logger.Error("Failed to create directory",
					"action", "CREATE_DIR",
					"path", dirPath,
					"error", err.Error(),
				)
				return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
			}
			o.Logger.Info("Directory created",
				"action", "CREATE_DIR",
				"path", dirPath,
			)
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
			o.Logger.Error("Error accessing path",
				"path", path,
				"error", err.Error(),
			)
			errorCount++
			return nil
		}

		// Skip directories and existing target folders
		if d.IsDir() {
			// Never skip the root dir
			if path == o.RootPath {
				return nil
			}

			// Always skip target subdirs
			if _, isTarget := o.TargetPaths[path]; isTarget && path != o.RootPath {
				return filepath.SkipDir
			}

			if !o.Recursive {
				return filepath.SkipDir
			}

			return nil
		}

		// Size filter check
		if o.minSize > 0 || o.maxSize > 0 {
			info, err := d.Info()
			if err != nil {
				log.Printf("Error getting file info for %s: %v", path, err)
				return nil
			}
			size := info.Size()
			if o.minSize > 0 && size < o.minSize {
				log.Printf("Skipped (too small): %s (%d bytes)", path, size)
				o.Logger.Info("File skipped - too small",
					"action", "SKIP_SIZE",
					"path", path,
					"size", size,
				)
				return nil
			}
			if o.maxSize > 0 && size > o.maxSize {
				log.Printf("Skipped (too large): %s (%d bytes)", path, size)
				o.Logger.Info("File skipped - too large",
					"action", "SKIP_SIZE",
					"path", path,
					"size", size,
				)
				return nil
			}
		}

		// Process individual file
		if err := o.processFile(path, d); err != nil {
			log.Printf("Error moving %s: %v", path, err)
			o.Logger.Error("Error moving file",
				"source", path,
				"error", err.Error(),
			)
			errorCount++
		} else {
			processedCount++
		}

		return nil
	})

	// Cleanup logic for empty directories
	if o.Recursive && !o.DryRun {
		log.Println("Cleaning up empty subdirectories...")
		if err := o.cleanupEmptyDirs(); err != nil {
			log.Printf("Cleanup error: %v", err)
		}
	}

	executionTime := time.Since(o.startTime)
	filesPerSecond := float64(processedCount) / executionTime.Seconds()
	if executionTime.Seconds() == 0 {
		filesPerSecond = float64(processedCount)
	}

	metrics := Metrics{
		ExecutionTime:  executionTime,
		FilesProcessed: processedCount,
		TotalBytes:     o.totalBytes,
		FilesPerSecond: filesPerSecond,
	}

	fmt.Print(metrics.String())

	if !o.DryRun && len(o.movedFiles) > 0 {
		if err := o.SaveHistory(); err != nil {
			log.Printf("Warning: failed to save history file: %v", err)
		}
	}

	return err
}

// cleanupEmptyDirs walks the path and removes empty folders.
func (o *Organizer) cleanupEmptyDirs() error {
	// We use a slice to collect paths so we can sort them or process them
	// without interfering with the active walk.
	var dirs []string
	err := filepath.WalkDir(o.RootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != o.RootPath {
			// Skip target folders (video, audio, etc.)
			if _, isTarget := o.TargetPaths[path]; isTarget {
				return filepath.SkipDir
			}
			dirs = append(dirs, path)
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Process directories in reverse order (deepest first)
	for i := len(dirs) - 1; i >= 0; i-- {
		path := dirs[i]
		entries, err := os.ReadDir(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Already removed by a deeper recursive call
			}
			return err
		}

		if len(entries) == 0 {
			log.Printf("Removing empty directory: %s", path)
			o.deletedDirs = append(o.deletedDirs, path)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				o.Logger.Error("Failed to remove directory",
					"action", "DELETE_DIR",
					"path", path,
					"error", err.Error(),
				)
				return err
			}
			o.Logger.Info("Directory removed",
				"action", "DELETE_DIR",
				"path", path,
			)
		}
	}
	return nil
}
