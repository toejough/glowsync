package filesystem

import (
	"os"
	"path/filepath"
)

// realFileScanner implements FileScanner using filepath.Walk.
type realFileScanner struct {
	root    string
	files   []FileInfo
	index   int
	err     error
	scanned bool
}

// newRealFileScanner creates a new scanner for the given directory.
func newRealFileScanner(root string) *realFileScanner {
	return &realFileScanner{
		root:  root,
		files: make([]FileInfo, 0),
		index: -1,
	}
}

// Next advances to the next file and returns its info.
func (s *realFileScanner) Next() (FileInfo, bool) {
	// Scan on first call
	if !s.scanned {
		s.scan()
		s.scanned = true
	}

	// Check if we have an error
	if s.err != nil {
		return FileInfo{}, false
	}

	// Advance to next file
	s.index++
	if s.index >= len(s.files) {
		return FileInfo{}, false
	}

	return s.files[s.index], true
}

// Err returns any error that occurred during scanning.
func (s *realFileScanner) Err() error {
	return s.err
}

// scan walks the directory tree and collects all files.
func (s *realFileScanner) scan() {
	s.err = filepath.Walk(s.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Add file info
		s.files = append(s.files, FileInfo{
			RelativePath: relPath,
			Size:         info.Size(),
			ModTime:      info.ModTime(),
			IsDir:        info.IsDir(),
		})

		return nil
	})
}

