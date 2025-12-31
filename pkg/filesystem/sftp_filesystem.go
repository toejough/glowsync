package filesystem

import (
	"fmt"
	"os"
	"time"
)

// SFTPFileSystem implements FileSystem for SFTP connections.
type SFTPFileSystem struct {
	conn *SFTPConnection
	pool *SFTPClientPool
}

// NewSFTPFileSystem creates a new SFTP filesystem using an established connection.
func NewSFTPFileSystem(conn *SFTPConnection) (*SFTPFileSystem, error) {
	// Create SFTP client pool with 8 concurrent clients
	const poolSize = 8

	pool, err := NewSFTPClientPool(conn.SSHClient(), poolSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create SFTP client pool: %w", err)
	}

	return &SFTPFileSystem{
		conn: conn,
		pool: pool,
	}, nil
}

// Scan returns an iterator over all files in a remote directory tree.
func (fs *SFTPFileSystem) Scan(path string) FileScanner {
	// Acquire a client from the pool for scanning
	client, err := fs.pool.Acquire()
	if err != nil {
		// Return a scanner with error state
		return newSFTPScannerWithError(err)
	}

	// Create scanner that will release client when done
	return newPooledSFTPScanner(client, path, fs.pool)
}

// Open opens a remote file for reading.
func (fs *SFTPFileSystem) Open(path string) (File, error) {
	client, err := fs.pool.Acquire()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire SFTP client: %w", err)
	}

	file, err := client.Open(path)
	if err != nil {
		fs.pool.Release(client)
		return nil, fmt.Errorf("failed to open remote file %s: %w", path, err)
	}

	// Wrap with pooled file - auto-releases client on close
	pooledFile, err := NewPooledSFTPFile(file, client, fs.pool)
	if err != nil {
		_ = file.Close()
		fs.pool.Release(client)
		return nil, fmt.Errorf("failed to create pooled file: %w", err)
	}

	return pooledFile, nil
}

// Create creates a remote file for writing.
func (fs *SFTPFileSystem) Create(path string) (File, error) {
	client, err := fs.pool.Acquire()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire SFTP client: %w", err)
	}

	file, err := client.Create(path)
	if err != nil {
		fs.pool.Release(client)
		return nil, fmt.Errorf("failed to create remote file %s: %w", path, err)
	}

	// Wrap with pooled file - auto-releases client on close
	pooledFile, err := NewPooledSFTPFile(file, client, fs.pool)
	if err != nil {
		_ = file.Close()
		fs.pool.Release(client)
		return nil, fmt.Errorf("failed to create pooled file: %w", err)
	}

	return pooledFile, nil
}

// MkdirAll creates a remote directory and all necessary parents.
func (fs *SFTPFileSystem) MkdirAll(path string, perm os.FileMode) error {
	client, err := fs.pool.Acquire()
	if err != nil {
		return fmt.Errorf("failed to acquire SFTP client: %w", err)
	}
	defer fs.pool.Release(client)

	err = client.MkdirAll(path)
	if err != nil {
		return fmt.Errorf("failed to create remote directory %s: %w", path, err)
	}

	return nil
}

// Chtimes changes the access and modification times of a remote file.
func (fs *SFTPFileSystem) Chtimes(path string, atime, mtime time.Time) error {
	client, err := fs.pool.Acquire()
	if err != nil {
		return fmt.Errorf("failed to acquire SFTP client: %w", err)
	}
	defer fs.pool.Release(client)

	err = client.Chtimes(path, atime, mtime)
	if err != nil {
		return fmt.Errorf("failed to change times for remote file %s: %w", path, err)
	}

	return nil
}

// Remove removes a remote file or empty directory.
func (fs *SFTPFileSystem) Remove(path string) error {
	client, err := fs.pool.Acquire()
	if err != nil {
		return fmt.Errorf("failed to acquire SFTP client: %w", err)
	}
	defer fs.pool.Release(client)

	err = client.Remove(path)
	if err != nil {
		return fmt.Errorf("failed to remove remote file %s: %w", path, err)
	}

	return nil
}

// Stat returns file information for a remote file.
func (fs *SFTPFileSystem) Stat(path string) (os.FileInfo, error) {
	client, err := fs.pool.Acquire()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire SFTP client: %w", err)
	}
	defer fs.pool.Release(client)

	info, err := client.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat remote file %s: %w", path, err)
	}

	return info, nil
}

// Close closes the SFTP client pool and releases all resources.
func (fs *SFTPFileSystem) Close() error {
	if fs.pool != nil {
		return fs.pool.Close()
	}

	return nil
}
