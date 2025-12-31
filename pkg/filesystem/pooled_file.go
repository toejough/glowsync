package filesystem

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"sync"

	"github.com/pkg/sftp"
)

// sftpFile is an interface that abstracts sftp.File operations for testing.
type sftpFile interface {
	io.Reader
	io.Writer
	io.Closer
	Stat() (os.FileInfo, error)
}

// clientPool is an interface that abstracts SFTP client pool operations for testing.
type clientPool interface {
	Acquire() (*sftp.Client, error)
	Release(*sftp.Client)
	Close() error
}

// PooledSFTPFile wraps an sftpFile and automatically releases the associated
// SFTP client back to the pool when Close() is called.
//
// This ensures that pool clients are properly returned even if the caller
// forgets to manually release them, preventing pool exhaustion.
//
// Example usage:
//
//	client, err := pool.Acquire()
//	if err != nil {
//	    return err
//	}
//	file, err := client.Create("/remote/path")
//	if err != nil {
//	    pool.Release(client)
//	    return err
//	}
//	pooledFile := NewPooledSFTPFile(file, client, pool)
//	defer pooledFile.Close() // Automatically releases client
type PooledSFTPFile struct {
	file   sftpFile
	client *sftp.Client
	pool   clientPool
	mu     sync.Mutex
	closed bool
}

// NewPooledSFTPFile creates a new pooled SFTP file wrapper.
// The wrapper will automatically release the client back to the pool when Close() is called.
//
// Parameters:
//   - file: The SFTP file to wrap (must not be nil)
//   - client: The SFTP client associated with the file (must not be nil)
//   - pool: The pool to release the client to on Close() (must not be nil)
//
// Returns an error if any parameter is nil.
func NewPooledSFTPFile(file sftpFile, client *sftp.Client, pool clientPool) (*PooledSFTPFile, error) {
	if file == nil {
		return nil, errors.New("file cannot be nil")
	}

	if client == nil {
		return nil, errors.New("client cannot be nil")
	}

	if pool == nil {
		return nil, errors.New("pool cannot be nil")
	}

	return &PooledSFTPFile{
		file:   file,
		client: client,
		pool:   pool,
		closed: false,
	}, nil
}

// Read reads up to len(p) bytes into p from the underlying file.
// Returns fs.ErrClosed if the file has been closed.
func (f *PooledSFTPFile) Read(p []byte) (int, error) {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return 0, fs.ErrClosed
	}
	f.mu.Unlock()

	return f.file.Read(p)
}

// Write writes len(p) bytes from p to the underlying file.
// Returns fs.ErrClosed if the file has been closed.
func (f *PooledSFTPFile) Write(p []byte) (int, error) {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return 0, fs.ErrClosed
	}
	f.mu.Unlock()

	return f.file.Write(p)
}

// Close closes the underlying file and releases the SFTP client back to the pool.
//
// CRITICAL: The client is ALWAYS released to the pool, even if closing the file fails.
// This prevents pool exhaustion in error scenarios.
//
// Close is idempotent - calling it multiple times is safe and will only close
// the file and release the client once.
//
// Returns any error from closing the underlying file.
func (f *PooledSFTPFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil // Idempotent - already closed
	}

	f.closed = true

	// Close file first, capture any error
	fileErr := f.file.Close()

	// ALWAYS release client to pool, even if file.Close() failed
	// This is critical to prevent pool exhaustion
	f.pool.Release(f.client)

	// Return file close error (if any)
	return fileErr
}

// Stat returns file information for the underlying file.
// Returns fs.ErrClosed if the file has been closed.
func (f *PooledSFTPFile) Stat() (os.FileInfo, error) {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil, fs.ErrClosed
	}
	f.mu.Unlock()

	return f.file.Stat()
}
