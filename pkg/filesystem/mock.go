// Package filesystem provides an abstraction layer for filesystem operations.
package filesystem

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MockFileSystem is an in-memory filesystem implementation for testing.
type MockFileSystem struct {
	mu    sync.RWMutex
	files map[string]*mockFile
}

// mockFile represents a file in the mock filesystem.
type mockFile struct {
	path    string
	data    []byte
	modTime time.Time
	isDir   bool
	perm    os.FileMode
}

// mockFileInfo implements os.FileInfo for mock files.
type mockFileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
	perm    os.FileMode
}

func (fi *mockFileInfo) Name() string       { return fi.name }
func (fi *mockFileInfo) Size() int64        { return fi.size }
func (fi *mockFileInfo) Mode() os.FileMode  { return fi.perm }
func (fi *mockFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *mockFileInfo) IsDir() bool        { return fi.isDir }
func (fi *mockFileInfo) Sys() interface{}   { return nil }

// mockFileHandle implements the File interface for reading/writing.
type mockFileHandle struct {
	fs     *MockFileSystem
	path   string
	reader *bytes.Reader
	writer *bytes.Buffer
	closed bool
}

func (f *mockFileHandle) Read(p []byte) (int, error) {
	if f.closed {
		return 0, os.ErrClosed
	}
	if f.reader == nil {
		return 0, io.EOF
	}
	return f.reader.Read(p)
}

func (f *mockFileHandle) Write(p []byte) (int, error) {
	if f.closed {
		return 0, os.ErrClosed
	}
	if f.writer == nil {
		f.writer = &bytes.Buffer{}
	}
	return f.writer.Write(p)
}

func (f *mockFileHandle) Close() error {
	if f.closed {
		return os.ErrClosed
	}
	f.closed = true
	
	// If we were writing, save the data
	if f.writer != nil {
		f.fs.mu.Lock()
		defer f.fs.mu.Unlock()
		
		if file, exists := f.fs.files[f.path]; exists {
			file.data = f.writer.Bytes()
		} else {
			f.fs.files[f.path] = &mockFile{
				path:    f.path,
				data:    f.writer.Bytes(),
				modTime: time.Now(),
				isDir:   false,
				perm:    0644,
			}
		}
	}
	
	return nil
}

func (f *mockFileHandle) Stat() (os.FileInfo, error) {
	if f.closed {
		return nil, os.ErrClosed
	}
	
	f.fs.mu.RLock()
	defer f.fs.mu.RUnlock()
	
	file, exists := f.fs.files[f.path]
	if !exists {
		return nil, os.ErrNotExist
	}
	
	return &mockFileInfo{
		name:    filepath.Base(f.path),
		size:    int64(len(file.data)),
		modTime: file.modTime,
		isDir:   file.isDir,
		perm:    file.perm,
	}, nil
}

// NewMockFileSystem creates a new in-memory filesystem.
func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files: make(map[string]*mockFile),
	}
}

// Scan returns an iterator over all files in a directory tree.
func (fs *MockFileSystem) Scan(path string) FileScanner {
	return newMockFileScanner(fs, path)
}

// Stat returns file information.
func (fs *MockFileSystem) Stat(path string) (os.FileInfo, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	file, exists := fs.files[path]
	if !exists {
		return nil, os.ErrNotExist
	}

	return &mockFileInfo{
		name:    filepath.Base(path),
		size:    int64(len(file.data)),
		modTime: file.modTime,
		isDir:   file.isDir,
		perm:    file.perm,
	}, nil
}

// Remove removes a file or empty directory.
func (fs *MockFileSystem) Remove(path string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	file, exists := fs.files[path]
	if !exists {
		return os.ErrNotExist
	}

	// If it's a directory, check if it's empty
	if file.isDir {
		for p := range fs.files {
			if strings.HasPrefix(p, path+"/") {
				return fmt.Errorf("directory not empty")
			}
		}
	}

	delete(fs.files, path)
	return nil
}


// Chtimes changes the access and modification times of a file.
func (fs *MockFileSystem) Chtimes(path string, atime, mtime time.Time) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	file, exists := fs.files[path]
	if !exists {
		return os.ErrNotExist
	}

	file.modTime = mtime
	return nil
}

// MkdirAll creates a directory and all necessary parents.
func (fs *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.mkdirAllLocked(path, perm)
}

// mkdirAllLocked is the internal implementation that assumes the lock is held.
func (fs *MockFileSystem) mkdirAllLocked(path string, perm os.FileMode) error {
	if path == "." || path == "/" {
		return nil
	}

	// Create parent directories first
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		if err := fs.mkdirAllLocked(dir, perm); err != nil {
			return err
		}
	}

	// Create this directory if it doesn't exist
	if _, exists := fs.files[path]; !exists {
		fs.files[path] = &mockFile{
			path:    path,
			data:    nil,
			modTime: time.Now(),
			isDir:   true,
			perm:    perm,
		}
	}

	return nil
}

// Open opens a file for reading.
func (fs *MockFileSystem) Open(path string) (File, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	file, exists := fs.files[path]
	if !exists {
		return nil, os.ErrNotExist
	}

	if file.isDir {
		return nil, fmt.Errorf("is a directory")
	}

	return &mockFileHandle{
		fs:     fs,
		path:   path,
		reader: bytes.NewReader(file.data),
		closed: false,
	}, nil
}

// Create creates a file for writing.
func (fs *MockFileSystem) Create(path string) (File, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Create parent directories if needed
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		_ = fs.mkdirAllLocked(dir, 0755)
	}

	// Create or truncate the file
	fs.files[path] = &mockFile{
		path:    path,
		data:    []byte{},
		modTime: time.Now(),
		isDir:   false,
		perm:    0644,
	}

	return &mockFileHandle{
		fs:     fs,
		path:   path,
		writer: &bytes.Buffer{},
		closed: false,
	}, nil
}


// Helper methods for testing

// AddFile adds a file to the mock filesystem with the given content and modtime.
func (fs *MockFileSystem) AddFile(path string, content []byte, modTime time.Time) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Create parent directories if needed
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		_ = fs.mkdirAllLocked(dir, 0755)
	}

	fs.files[path] = &mockFile{
		path:    path,
		data:    append([]byte(nil), content...),
		modTime: modTime,
		isDir:   false,
		perm:    0644,
	}
}

// AddDir adds a directory to the mock filesystem.
func (fs *MockFileSystem) AddDir(path string, modTime time.Time) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.files[path] = &mockFile{
		path:    path,
		data:    nil,
		modTime: modTime,
		isDir:   true,
		perm:    0755,
	}
}

// GetFile retrieves a file's content from the mock filesystem.
func (fs *MockFileSystem) GetFile(path string) ([]byte, time.Time, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	file, exists := fs.files[path]
	if !exists {
		return nil, time.Time{}, os.ErrNotExist
	}

	if file.isDir {
		return nil, time.Time{}, fmt.Errorf("is a directory")
	}

	return append([]byte(nil), file.data...), file.modTime, nil
}

// Exists checks if a path exists in the mock filesystem.
func (fs *MockFileSystem) Exists(path string) bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	_, exists := fs.files[path]
	return exists
}

// ListFiles returns all file paths in the mock filesystem.
func (fs *MockFileSystem) ListFiles() []string {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	paths := make([]string, 0, len(fs.files))
	for p := range fs.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

