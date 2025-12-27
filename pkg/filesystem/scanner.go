package filesystem

//go:generate impgen filesystem.FileScanner

import (
	"time"
)

// FileScanner is an iterator over files in a directory.
// It provides a simple Next pattern for traversing directory contents.
type FileScanner interface {
	// Next advances to the next file and returns its info.
	// Returns (FileInfo{}, false) when done or on error.
	// Check Err() after Next() returns false to distinguish between end-of-scan and error.
	Next() (FileInfo, bool)

	// Err returns any error that occurred during scanning.
	// Should be checked after Next() returns false.
	Err() error
}

// FileInfo contains metadata about a file.
// This is our own type (not os.FileInfo) to make it easier to work with.
type FileInfo struct {
	// RelativePath is the path relative to the scan root
	RelativePath string

	// Size is the file size in bytes
	Size int64

	// ModTime is the modification time
	ModTime time.Time

	// IsDir indicates if this is a directory
	IsDir bool
}
