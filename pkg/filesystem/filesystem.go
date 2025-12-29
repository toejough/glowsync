// Package filesystem provides an abstraction layer for filesystem operations
// to enable dependency injection and testing without actual filesystem I/O.
package filesystem

import (
	"fmt"
	"io"
	"os"
	"time"
)

// File is an interface that abstracts file operations.
// This allows us to work with both real files and mock files.
type File interface {
	io.Reader
	io.Writer
	io.Closer
	Stat() (os.FileInfo, error)
}

// FileSystem is an interface that abstracts filesystem operations.
// This allows for dependency injection and testing with mock implementations.
type FileSystem interface {
	// NEW: Iterator-based scanning (easier to test with imptest)
	// Scan returns an iterator over all files in a directory tree.
	// The iterator will traverse the directory recursively.
	Scan(path string) FileScanner

	// Low-level file operations (needed for CopyFile with progress/cancellation)
	Open(path string) (File, error)
	Create(path string) (File, error)
	MkdirAll(path string, perm os.FileMode) error
	Chtimes(path string, atime, mtime time.Time) error
	Remove(path string) error
	Stat(path string) (os.FileInfo, error)
}

// RealFileSystem implements FileSystem using actual os/filepath functions.
type RealFileSystem struct{}

// NewRealFileSystem creates a new RealFileSystem instance.
func NewRealFileSystem() *RealFileSystem {
	return &RealFileSystem{}
}

// Chtimes changes the access and modification times of a file.
func (fs *RealFileSystem) Chtimes(path string, atime, mtime time.Time) error {
	err := os.Chtimes(path, atime, mtime)
	if err != nil {
		return fmt.Errorf("failed to change times for %s: %w", path, err)
	}

	return nil
}

// Create creates a file for writing.
func (fs *RealFileSystem) Create(path string) (File, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", path, err)
	}

	return file, nil
}

// MkdirAll creates a directory and all necessary parents.
func (fs *RealFileSystem) MkdirAll(path string, perm os.FileMode) error {
	err := os.MkdirAll(path, perm)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	return nil
}

// Open opens a file for reading.
func (fs *RealFileSystem) Open(path string) (File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", path, err)
	}

	return file, nil
}

// Remove removes a file or empty directory.
func (fs *RealFileSystem) Remove(path string) error {
	err := os.Remove(path)
	if err != nil {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}

	return nil
}

// Scan returns an iterator over all files in a directory tree.
func (fs *RealFileSystem) Scan(path string) FileScanner {
	return newRealFileScanner(path)
}

// Stat returns file information.
func (fs *RealFileSystem) Stat(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat %s: %w", path, err)
	}

	return info, nil
}
