package filesystem

import (
	"path/filepath"
	"sort"
	"strings"
)

// mockFileScanner implements FileScanner for MockFileSystem.
type mockFileScanner struct {
	fs      *MockFileSystem
	root    string
	files   []FileInfo
	index   int
	scanned bool
}

// newMockFileScanner creates a new scanner for the given directory.
func newMockFileScanner(fs *MockFileSystem, root string) *mockFileScanner {
	return &mockFileScanner{
		fs:    fs,
		root:  root,
		files: make([]FileInfo, 0),
		index: -1,
	}
}

// Next advances to the next file and returns its info.
func (s *mockFileScanner) Next() (FileInfo, bool) {
	// Scan on first call
	if !s.scanned {
		s.scan()
		s.scanned = true
	}

	// Advance to next file
	s.index++
	if s.index >= len(s.files) {
		return FileInfo{}, false
	}

	return s.files[s.index], true
}

// Err returns any error that occurred during scanning.
func (s *mockFileScanner) Err() error {
	return nil // MockFileSystem doesn't produce errors during scanning
}

// scan collects all files under the root directory.
func (s *mockFileScanner) scan() {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	// Collect all files under the root
	for path, file := range s.fs.files {
		// Skip the root itself
		if path == s.root {
			continue
		}

		// Check if this file is under the root
		if !strings.HasPrefix(path, s.root+"/") && path != s.root {
			continue
		}

		// Get relative path
		relPath, err := filepath.Rel(s.root, path)
		if err != nil {
			continue
		}

		// Skip if it's the current directory
		if relPath == "." {
			continue
		}

		// Add file info
		s.files = append(s.files, FileInfo{
			RelativePath: relPath,
			Size:         int64(len(file.data)),
			ModTime:      file.modTime,
			IsDir:        file.isDir,
		})
	}

	// Sort files by path for consistent ordering
	sort.Slice(s.files, func(i, j int) bool {
		return s.files[i].RelativePath < s.files[j].RelativePath
	})
}

