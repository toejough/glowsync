package filesystem

import (
	"fmt"
)

// CreateFileSystem creates a FileSystem for the given path.
// Returns (filesystem, basePath, closer, error).
// - filesystem: The FileSystem to use for operations
// - basePath: The actual path to use with the filesystem (stripped of URL prefix)
// - closer: A function to call when done (closes SFTP connections), or nil for local
func CreateFileSystem(pathStr string) (FileSystem, string, func(), error) {
	parsed, err := ParsePath(pathStr)
	if err != nil {
		return nil, "", nil, err
	}

	if !parsed.IsRemote {
		// Local filesystem
		return NewRealFileSystem(), parsed.LocalPath, nil, nil
	}

	// SFTP filesystem
	conn, err := Connect(parsed.Host, parsed.Port, parsed.User)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to connect to %s@%s:%d: %w",
			parsed.User, parsed.Host, parsed.Port, err)
	}

	fs := NewSFTPFileSystem(conn)
	closer := func() {
		_ = conn.Close()
	}

	return fs, parsed.Path, closer, nil
}

// CreateFileSystemPair creates filesystems for source and destination paths.
// Returns (sourceFS, destFS, sourcePath, destPath, closer, error).
// The closer function should be called when done to clean up any connections.
func CreateFileSystemPair(sourcePath, destPath string) (
	sourceFS FileSystem,
	destFS FileSystem,
	srcPath string,
	dstPath string,
	closer func(),
	err error,
) {
	var srcCloser, dstCloser func()

	// Create source filesystem
	sourceFS, srcPath, srcCloser, err = CreateFileSystem(sourcePath)
	if err != nil {
		return nil, nil, "", "", nil, fmt.Errorf("failed to create source filesystem: %w", err)
	}

	// Create destination filesystem
	destFS, dstPath, dstCloser, err = CreateFileSystem(destPath)
	if err != nil {
		// Clean up source if destination fails
		if srcCloser != nil {
			srcCloser()
		}
		return nil, nil, "", "", nil, fmt.Errorf("failed to create destination filesystem: %w", err)
	}

	// Create combined closer
	closer = func() {
		if srcCloser != nil {
			srcCloser()
		}
		if dstCloser != nil {
			dstCloser()
		}
	}

	return sourceFS, destFS, srcPath, dstPath, closer, nil
}
