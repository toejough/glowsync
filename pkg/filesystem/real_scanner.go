package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
)

// realFileScanner implements FileScanner using filepath.Walk with progressive yielding.
type realFileScanner struct {
	root    string
	fileCh  chan FileInfo
	errCh   chan error
	err     error
	started bool
	done    bool
}

// Err returns any error that occurred during scanning.
func (s *realFileScanner) Err() error {
	return s.err
}

// Next advances to the next file and returns its info.
func (s *realFileScanner) Next() (FileInfo, bool) {
	// Start walking on first call
	if !s.started {
		s.startWalking()
		s.started = true
	}

	// If already done, return false
	if s.done {
		return FileInfo{}, false
	}

	// Try to get next file from channel
	select {
	case file, ok := <-s.fileCh:
		if !ok {
			// Channel closed, check for error
			select {
			case err := <-s.errCh:
				s.err = err
			default:
				// No error, just finished
			}
			s.done = true

			return FileInfo{}, false
		}

		return file, true
	case err := <-s.errCh:
		// Error occurred during walk
		s.err = err
		s.done = true

		return FileInfo{}, false
	}
}

// startWalking begins the directory walk in a background goroutine.
func (s *realFileScanner) startWalking() {
	s.fileCh = make(chan FileInfo)
	s.errCh = make(chan error, 1)

	go func() {
		defer close(s.fileCh)

		walkErr := filepath.Walk(s.root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Get relative path
			relPath, err := filepath.Rel(s.root, path)
			if err != nil {
				return fmt.Errorf("failed to get relative path for %s: %w", path, err)
			}

			// Skip the root directory itself
			if relPath == "." {
				return nil
			}

			// Send file info to channel (yields immediately)
			s.fileCh <- FileInfo{
				RelativePath: relPath,
				Size:         info.Size(),
				ModTime:      info.ModTime(),
				IsDir:        info.IsDir(),
			}

			return nil
		})

		if walkErr != nil {
			s.errCh <- walkErr
		}
	}()
}

// newRealFileScanner creates a new scanner for the given directory.
func newRealFileScanner(root string) *realFileScanner {
	return &realFileScanner{
		root: root,
	}
}
