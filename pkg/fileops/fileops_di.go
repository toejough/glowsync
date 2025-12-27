package fileops

//go:generate impgen fileops.CopyFileWithStats

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/joe/copy-files/pkg/filesystem"
)

// FileOps provides file operations with dependency injection for filesystem access.
// This allows for testing without actual filesystem I/O.
type FileOps struct {
	FS filesystem.FileSystem
}

// NewFileOps creates a new FileOps instance with the given filesystem.
func NewFileOps(fs filesystem.FileSystem) *FileOps {
	return &FileOps{FS: fs}
}

// NewRealFileOps creates a new FileOps instance using the real filesystem.
func NewRealFileOps() *FileOps {
	return &FileOps{FS: filesystem.NewRealFileSystem()}
}

// CountFiles quickly counts the total number of files/directories in a path.
func (fo *FileOps) CountFiles(rootPath string) (int, error) {
	return fo.CountFilesWithProgress(rootPath, nil)
}

// CountFilesWithProgress counts files with progress reporting.
func (fo *FileOps) CountFilesWithProgress(rootPath string, progressCallback CountProgressCallback) (int, error) {
	scanner := fo.FS.Scan(rootPath)
	count := 0

	for info, ok := scanner.Next(); ok; info, ok = scanner.Next() {
		count++

		// Report progress every 10 files to avoid spam
		if progressCallback != nil && count%10 == 0 {
			path := filepath.Join(rootPath, info.RelativePath)
			progressCallback(path, count)
		}
	}

	err := scanner.Err()
	if err != nil {
		return count, err
	}

	return count, nil
}

// ScanDirectory recursively scans a directory and returns file information.
func (fo *FileOps) ScanDirectory(rootPath string) (map[string]*FileInfo, error) {
	return fo.ScanDirectoryWithProgress(rootPath, nil)
}

// ScanDirectoryWithProgress recursively scans a directory with progress reporting.
// This now scans only once, collecting files and reporting progress as we go.
func (fo *FileOps) ScanDirectoryWithProgress(rootPath string, progressCallback ScanProgressCallback) (map[string]*FileInfo, error) {
	files := make(map[string]*FileInfo)
	fileCount := 0

	scanner := fo.FS.Scan(rootPath)
	for info, ok := scanner.Next(); ok; info, ok = scanner.Next() {
		path := filepath.Join(rootPath, info.RelativePath)

		fileInfo := &FileInfo{
			Path:         path,
			RelativePath: info.RelativePath,
			Size:         info.Size,
			ModTime:      info.ModTime,
			IsDir:        info.IsDir,
		}

		files[info.RelativePath] = fileInfo
		fileCount++

		// Report progress if callback provided
		// Note: totalCount is 0 because we don't know the total until we finish scanning
		if progressCallback != nil {
			progressCallback(path, fileCount, 0)
		}
	}

	err := scanner.Err()
	if err != nil {
		return files, err
	}

	return files, nil
}

// ComputeFileHash computes SHA256 hash of a file.
func (fo *FileOps) ComputeFileHash(filePath string) (string, error) {
	file, err := fo.FS.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	hash := sha256.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CompareFilesBytes performs byte-by-byte comparison of two files.
// Returns true if files are identical, false if they differ.
func (fo *FileOps) CompareFilesBytes(path1, path2 string) (bool, error) {
	// Open both files
	file1, err := fo.FS.Open(path1)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = file1.Close()
	}()

	file2, err := fo.FS.Open(path2)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = file2.Close()
	}()

	// Get file sizes
	info1, err := file1.Stat()
	if err != nil {
		return false, err
	}
	info2, err := file2.Stat()
	if err != nil {
		return false, err
	}

	// Quick size check
	if info1.Size() != info2.Size() {
		return false, nil
	}

	// Compare byte-by-byte
	buf1 := make([]byte, 32*1024)
	buf2 := make([]byte, 32*1024)

	for {
		n1, err1 := file1.Read(buf1)
		n2, err2 := file2.Read(buf2)

		if n1 != n2 {
			return false, nil
		}

		if n1 > 0 {
			for i := range n1 {
				if buf1[i] != buf2[i] {
					return false, nil
				}
			}
		}

		if errors.Is(err1, io.EOF) && errors.Is(err2, io.EOF) {
			return true, nil
		}

		if err1 != nil {
			return false, err1
		}
		if err2 != nil {
			return false, err2
		}
	}
}

// CopyFile copies a file from src to dst with progress reporting.
func (fo *FileOps) CopyFile(src, dst string, progress ProgressCallback) (int64, error) {
	sourceFile, err := fo.FS.Open(src)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	// Get source file info
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return 0, err
	}

	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dst)
	if err := fo.FS.MkdirAll(dstDir, 0o750); err != nil {
		return 0, err
	}

	// Create destination file
	destFile, err := fo.FS.Create(dst)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = destFile.Close()
	}()

	// Copy with progress tracking
	var written int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		nr, err := sourceFile.Read(buf)
		if nr > 0 {
			nw, err := destFile.Write(buf[0:nr])
			if err != nil {
				return written, err
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
			written += int64(nw)

			if progress != nil {
				progress(written, sourceInfo.Size(), src)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return written, err
		}
	}

	// Preserve modification time
	err = fo.FS.Chtimes(dst, sourceInfo.ModTime(), sourceInfo.ModTime())
	if err != nil {
		return written, err
	}

	return written, nil
}

// Remove removes a file or empty directory
func (fo *FileOps) Remove(path string) error {
	return fo.FS.Remove(path)
}

// Stat returns file information
func (fo *FileOps) Stat(path string) (os.FileInfo, error) {
	return fo.FS.Stat(path)
}

// Chtimes changes the access and modification times of a file
func (fo *FileOps) Chtimes(path string, atime, mtime time.Time) error {
	return fo.FS.Chtimes(path, atime, mtime)
}

// CopyFileWithStats copies a file and returns detailed timing statistics.
// If cancelChan is provided and closed, the copy will be aborted.
func (fo *FileOps) CopyFileWithStats(src, dst string, progress ProgressCallback, cancelChan <-chan struct{}) (*CopyStats, error) {
	stats := &CopyStats{}

	sourceFile, err := fo.FS.Open(src)
	if err != nil {
		return stats, err
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	// Get source file info
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return stats, err
	}

	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dst)
	if err := fo.FS.MkdirAll(dstDir, 0o750); err != nil {
		return stats, err
	}

	// Create destination file
	destFile, err := fo.FS.Create(dst)
	if err != nil {
		return stats, err
	}

	// Track whether copy completed successfully
	copyCompleted := false
	defer func() {
		_ = destFile.Close()
		// If copy was cancelled or failed, delete the partial file
		if !copyCompleted {
			_ = fo.FS.Remove(dst)
		}
	}()

	// Copy with progress tracking and timing
	var written int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		// Check for cancellation before each read
		if cancelChan != nil {
			select {
			case <-cancelChan:
				return stats, fmt.Errorf("copy cancelled")
			default:
			}
		}

		// Time the read operation
		readStart := time.Now()
		nr, err := sourceFile.Read(buf)
		stats.ReadTime += time.Since(readStart)

		if nr > 0 {
			// Time the write operation
			writeStart := time.Now()
			nw, err := destFile.Write(buf[0:nr])
			stats.WriteTime += time.Since(writeStart)

			if err != nil {
				return stats, err
			}
			if nr != nw {
				return stats, io.ErrShortWrite
			}
			written += int64(nw)

			if progress != nil {
				progress(written, sourceInfo.Size(), src)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return stats, err
		}
	}

	stats.BytesCopied = written

	// Close the file before setting modification time
	// This is important for network filesystems like SMB
	err = destFile.Close()
	if err != nil {
		return stats, err
	}

	// Preserve modification time
	err = fo.FS.Chtimes(dst, sourceInfo.ModTime(), sourceInfo.ModTime())
	if err != nil {
		return stats, err
	}

	// Mark copy as completed successfully
	copyCompleted = true

	return stats, nil
}
