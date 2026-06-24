package history

// HistoryState holds the state of a file organization run for undo/rollback.
type HistoryState struct {
	MovedFiles  map[string]string `json:"moved_files"`
	DeletedDirs []string          `json:"deleted_dirs"`
	RootPath    string            `json:"root_path"`
}
