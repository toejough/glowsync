// Package filesystem provides an abstraction layer for filesystem operations
// to enable dependency injection and testing without actual filesystem I/O.
package filesystem

import (
	"io"
	"os"
	"path/filepath"
	"time"
)

// FileSystem is an interface that abstracts filesystem operations.
// This allows for dependency injection and testing with mock implementations.
type FileSystem interface {
	// File operations
	Stat(path string) (os.FileInfo, error)
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	Remove(path string) error
	RemoveAll(path string) error
	Chtimes(path string, atime, mtime time.Time) error
	MkdirAll(path string, perm os.FileMode) error
	
	// File I/O
	Open(path string) (File, error)
	Create(path string) (File, error)
	
	// Directory operations
	Walk(root string, fn filepath.WalkFunc) error
}

// File is an interface that abstracts file operations.
// This allows us to work with both real files and mock files.
type File interface {
	io.Reader
	io.Writer
	io.Closer
	Stat() (os.FileInfo, error)
}

// RealFileSystem implements FileSystem using actual os/filepath functions.
type RealFileSystem struct{}

// NewRealFileSystem creates a new RealFileSystem instance.
func NewRealFileSystem() *RealFileSystem {
	return &RealFileSystem{}
}

// Stat returns file information.
func (fs *RealFileSystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// ReadFile reads the entire file.
func (fs *RealFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes data to a file.
func (fs *RealFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// Remove removes a file or empty directory.
func (fs *RealFileSystem) Remove(path string) error {
	return os.Remove(path)
}

// RemoveAll removes a path and any children it contains.
func (fs *RealFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Chtimes changes the access and modification times of a file.
func (fs *RealFileSystem) Chtimes(path string, atime, mtime time.Time) error {
	return os.Chtimes(path, atime, mtime)
}

// MkdirAll creates a directory and all necessary parents.
func (fs *RealFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Open opens a file for reading.
func (fs *RealFileSystem) Open(path string) (File, error) {
	return os.Open(path)
}

// Create creates a file for writing.
func (fs *RealFileSystem) Create(path string) (File, error) {
	return os.Create(path)
}

// Walk walks the file tree rooted at root.
func (fs *RealFileSystem) Walk(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(root, fn)
}

