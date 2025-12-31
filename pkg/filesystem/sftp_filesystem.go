package filesystem

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/sftp"
)

// SFTPFileSystem implements FileSystem for SFTP connections.
type SFTPFileSystem struct {
	conn   *SFTPConnection
	client *sftp.Client
}

// NewSFTPFileSystem creates a new SFTP filesystem using an established connection.
func NewSFTPFileSystem(conn *SFTPConnection) *SFTPFileSystem {
	return &SFTPFileSystem{
		conn:   conn,
		client: conn.Client(),
	}
}

// Scan returns an iterator over all files in a remote directory tree.
func (fs *SFTPFileSystem) Scan(path string) FileScanner {
	return newSFTPScanner(fs.client, path)
}

// Open opens a remote file for reading.
func (fs *SFTPFileSystem) Open(path string) (File, error) {
	file, err := fs.client.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote file %s: %w", path, err)
	}

	return newSFTPFile(file, path), nil
}

// Create creates a remote file for writing.
func (fs *SFTPFileSystem) Create(path string) (File, error) {
	file, err := fs.client.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote file %s: %w", path, err)
	}

	return newSFTPFile(file, path), nil
}

// MkdirAll creates a remote directory and all necessary parents.
func (fs *SFTPFileSystem) MkdirAll(path string, perm os.FileMode) error {
	err := fs.client.MkdirAll(path)
	if err != nil {
		return fmt.Errorf("failed to create remote directory %s: %w", path, err)
	}

	return nil
}

// Chtimes changes the access and modification times of a remote file.
func (fs *SFTPFileSystem) Chtimes(path string, atime, mtime time.Time) error {
	err := fs.client.Chtimes(path, atime, mtime)
	if err != nil {
		return fmt.Errorf("failed to change times for remote file %s: %w", path, err)
	}

	return nil
}

// Remove removes a remote file or empty directory.
func (fs *SFTPFileSystem) Remove(path string) error {
	err := fs.client.Remove(path)
	if err != nil {
		return fmt.Errorf("failed to remove remote file %s: %w", path, err)
	}

	return nil
}

// Stat returns file information for a remote file.
func (fs *SFTPFileSystem) Stat(path string) (os.FileInfo, error) {
	info, err := fs.client.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat remote file %s: %w", path, err)
	}

	return info, nil
}
