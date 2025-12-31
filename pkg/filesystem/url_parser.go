package filesystem

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParsedPath represents either a local path or an SFTP URL.
type ParsedPath struct {
	IsRemote bool

	// For local paths
	LocalPath string

	// For SFTP paths
	Host string
	Port int
	User string
	Path string // Remote path
}

// ParsePath parses a path string, detecting whether it's a local path or SFTP URL.
// SFTP URLs have the format: sftp://user@host:port/path/to/dir
// Port is optional (defaults to 22)
// Examples:
//   - sftp://joe@myserver.com/home/joe/data
//   - sftp://joe@myserver.com:2222/backups
//   - /local/path/to/files (local path)
func ParsePath(path string) (*ParsedPath, error) {
	// Check if it's an SFTP URL
	if strings.HasPrefix(path, "sftp://") {
		return parseSFTPURL(path)
	}

	// Otherwise it's a local path
	return &ParsedPath{
		IsRemote:  false,
		LocalPath: path,
	}, nil
}

// parseSFTPURL parses an SFTP URL into its components.
func parseSFTPURL(sftpURL string) (*ParsedPath, error) {
	u, err := url.Parse(sftpURL)
	if err != nil {
		return nil, fmt.Errorf("invalid SFTP URL: %w", err)
	}

	if u.Scheme != "sftp" {
		return nil, fmt.Errorf("expected sftp:// scheme, got %s://", u.Scheme)
	}

	// Extract user
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("SFTP URL must include username (sftp://user@host/path)")
	}
	user := u.User.Username()

	// Extract host
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("SFTP URL must include host")
	}

	// Extract port (default to 22)
	port := 22
	if portStr := u.Port(); portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port number: %w", err)
		}
		port = p
	}

	// Extract path
	// SFTP path convention:
	//   sftp://user@host/path  → relative to home directory (strip leading /)
	//   sftp://user@host//path → absolute path /path (strip one /)
	//   sftp://user@host       → home directory (.)
	remotePath := u.Path
	if remotePath == "" || remotePath == "/" {
		remotePath = "."
	} else if strings.HasPrefix(remotePath, "//") {
		// Absolute path: strip one /
		remotePath = remotePath[1:]
	} else {
		// Relative to home: strip leading /
		remotePath = strings.TrimPrefix(remotePath, "/")
	}

	return &ParsedPath{
		IsRemote: true,
		Host:     host,
		Port:     port,
		User:     user,
		Path:     remotePath,
	}, nil
}
