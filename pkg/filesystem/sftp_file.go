package filesystem

import (
	"os"

	"github.com/pkg/sftp"
)

// SFTPFile wraps sftp.File to implement the filesystem.File interface.
type SFTPFile struct {
	file *sftp.File
	path string
}

// newSFTPFile creates a new SFTPFile wrapper.
func newSFTPFile(file *sftp.File, path string) *SFTPFile {
	return &SFTPFile{
		file: file,
		path: path,
	}
}

// Read reads from the SFTP file.
func (f *SFTPFile) Read(p []byte) (n int, err error) {
	return f.file.Read(p)
}

// Write writes to the SFTP file.
func (f *SFTPFile) Write(p []byte) (n int, err error) {
	return f.file.Write(p)
}

// Close closes the SFTP file.
func (f *SFTPFile) Close() error {
	return f.file.Close()
}

// Stat returns file information for the SFTP file.
func (f *SFTPFile) Stat() (os.FileInfo, error) {
	return f.file.Stat()
}
