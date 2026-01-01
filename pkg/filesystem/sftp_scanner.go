package filesystem

import (
	"fmt"
	"path"

	"github.com/pkg/sftp"
)

// pooledSFTPScanner wraps an sftpScanner and automatically releases
// the SFTP client back to the pool after scanning completes.
type pooledSFTPScanner struct {
	scanner *sftpScanner
	client  *sftp.Client
	pool    ClientPool
	done    bool
}

// Err returns any error that occurred during scanning.
func (s *pooledSFTPScanner) Err() error {
	return s.scanner.Err()
}

// Next advances to the next file and returns its info.
// Automatically releases the client back to the pool when scanning is complete.
func (s *pooledSFTPScanner) Next() (FileInfo, bool) {
	info, hasNext := s.scanner.Next()

	// If we're done scanning and haven't released yet, release the client
	if !hasNext && !s.done {
		s.done = true
		s.pool.Release(s.client)
	}

	return info, hasNext
}

// sftpScanner implements FileScanner for SFTP directories.
type sftpScanner struct {
	client  *sftp.Client
	root    string
	files   []FileInfo
	index   int
	err     error
	scanned bool
}

// Err returns any error that occurred during scanning.
func (s *sftpScanner) Err() error {
	return s.err
}

// Next advances to the next file and returns its info.
func (s *sftpScanner) Next() (FileInfo, bool) {
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

// scan walks the remote directory tree and collects all files.
func (s *sftpScanner) scan() {
	walker := s.client.Walk(s.root)

	for walker.Step() {
		if err := walker.Err(); err != nil { //nolint:noinlineerr // Inline error check is idiomatic for walker error handling
			s.err = fmt.Errorf("error scanning SFTP directory: %w", err)
			return
		}

		stat := walker.Stat()
		fullPath := walker.Path()

		// Skip the root directory itself
		if fullPath == s.root {
			continue
		}

		// Calculate relative path
		relPath, err := relativePath(s.root, fullPath)
		if err != nil {
			s.err = fmt.Errorf("failed to get relative path for %s: %w", fullPath, err)
			return
		}

		// Add file info
		s.files = append(s.files, FileInfo{
			RelativePath: relPath,
			Size:         stat.Size(),
			ModTime:      stat.ModTime(),
			IsDir:        stat.IsDir(),
		})
	}
}

// newPooledSFTPScanner creates a new pooled scanner.
func newPooledSFTPScanner(client *sftp.Client, root string, pool ClientPool) *pooledSFTPScanner {
	return &pooledSFTPScanner{
		scanner: newSFTPScanner(client, root),
		client:  client,
		pool:    pool,
		done:    false,
	}
}

// newSFTPScanner creates a new scanner for the given SFTP directory.
func newSFTPScanner(client *sftp.Client, root string) *sftpScanner {
	return &sftpScanner{
		client: client,
		root:   root,
		files:  make([]FileInfo, 0),
		index:  -1,
	}
}

// newSFTPScannerWithError creates a scanner in an error state.
func newSFTPScannerWithError(err error) *sftpScanner {
	return &sftpScanner{
		err:     err,
		scanned: true,
		files:   make([]FileInfo, 0),
		index:   -1,
	}
}

// relativePath computes the relative path from root to target.
// Uses path package (not filepath) since SFTP always uses forward slashes.
func relativePath(root, target string) (string, error) {
	// Clean both paths
	root = path.Clean(root)
	target = path.Clean(target)

	// Ensure root ends with /
	if root != "/" && root[len(root)-1] != '/' {
		root += "/"
	}

	// Check if target starts with root
	if len(target) < len(root) {
		return "", fmt.Errorf("target %s is not under root %s", target, root) //nolint:err113,lll // Path validation error with actual paths
	}

	if target[:len(root)] != root {
		return "", fmt.Errorf("target %s is not under root %s", target, root) //nolint:err113,lll // Path validation error with actual paths
	}

	// Return the relative portion
	relPath := target[len(root):]
	if relPath == "" {
		return ".", nil
	}

	return relPath, nil
}
