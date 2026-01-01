package filesystem

import (
	"fmt"
	"os"
	"time"
)

// PoolConfig configures the SFTP client pool size limits.
type PoolConfig struct {
	InitialSize int
	MinSize     int
	MaxSize     int
}

// SFTPFileSystem implements FileSystem for SFTP connections.
type SFTPFileSystem struct {
	pool *SFTPClientPool
}

// NewSFTPFileSystem creates a new SFTP filesystem using an established connection.
// If config is nil, DefaultPoolConfig() is used.
func NewSFTPFileSystem(conn *SFTPConnection, config *PoolConfig) (*SFTPFileSystem, error) {
	// Use default config if none provided
	if config == nil {
		config = DefaultPoolConfig()
	}

	// Validate config
	if config.MinSize <= 0 {
		return nil, fmt.Errorf("minSize must be greater than 0, got %d", config.MinSize) //nolint:err113,lll // Validation error with actual values
	}
	if config.InitialSize < config.MinSize {
		return nil, fmt.Errorf("initialSize (%d) must be >= minSize (%d)", config.InitialSize, config.MinSize) //nolint:err113,lll // Validation error with actual values
	}
	if config.InitialSize > config.MaxSize {
		return nil, fmt.Errorf("initialSize (%d) must be <= maxSize (%d)", config.InitialSize, config.MaxSize) //nolint:err113,lll // Validation error with actual values
	}

	// Create SFTP client pool with configured limits
	pool, err := NewSFTPClientPoolWithLimits(
		conn.SSHClient(),
		config.InitialSize,
		config.MinSize,
		config.MaxSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SFTP client pool: %w", err)
	}

	return &SFTPFileSystem{
		pool: pool,
	}, nil
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

// Close closes the SFTP client pool and releases all resources.
func (fs *SFTPFileSystem) Close() error {
	if fs.pool != nil {
		return fs.pool.Close()
	}

	return nil
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
func (fs *SFTPFileSystem) MkdirAll(path string, perm os.FileMode) error { //nolint:revive,lll // perm unused - SFTP uses server defaults, parameter required by FileSystem interface
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

// PoolMaxSize returns the maximum allowed pool size.
func (fs *SFTPFileSystem) PoolMaxSize() int {
	return fs.pool.MaxSize()
}

// PoolMinSize returns the minimum allowed pool size.
func (fs *SFTPFileSystem) PoolMinSize() int {
	return fs.pool.MinSize()
}

// PoolSize returns the current actual number of connections in the pool.
func (fs *SFTPFileSystem) PoolSize() int {
	return fs.pool.Size()
}

// PoolTargetSize returns the current target pool size.
func (fs *SFTPFileSystem) PoolTargetSize() int {
	return fs.pool.TargetSize()
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

// ResizePool sets the target pool size.
// Delegates to the underlying pool's Resize method.
func (fs *SFTPFileSystem) ResizePool(targetSize int) {
	fs.pool.Resize(targetSize)
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

// DefaultPoolConfig returns the default pool configuration.
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		InitialSize: 4, //nolint:mnd // Default pool size
		MinSize:     1,
		MaxSize:     16, //nolint:mnd // Maximum pool connections
	}
}
