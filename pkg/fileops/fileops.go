// Package fileops provides file operation utilities for copying and comparing files.
package fileops

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// FileInfo represents information about a file
type FileInfo struct {
	Path         string
	RelativePath string
	Size         int64
	ModTime      time.Time
	Hash         string
	IsDir        bool
}

// ProgressCallback is called during file operations to report progress
type ProgressCallback func(bytesTransferred int64, totalBytes int64, currentFile string)

// CopyStats contains timing information about a copy operation
type CopyStats struct {
	BytesCopied int64
	ReadTime    time.Duration
	WriteTime   time.Duration
}

// ScanProgressCallback is called during directory scanning to report progress
// Parameters: currentPath, scannedCount, totalCount (0 if unknown)
type ScanProgressCallback func(path string, scannedCount int, totalCount int)

// CountProgressCallback is called during file counting to report progress
// Parameters: currentPath, countSoFar
type CountProgressCallback func(path string, count int)

// CountFiles quickly counts the total number of files/directories in a path
func CountFiles(rootPath string) (int, error) {
	return CountFilesWithProgress(rootPath, nil)
}

// CountFilesWithProgress counts files with progress reporting
func CountFilesWithProgress(rootPath string, progressCallback CountProgressCallback) (int, error) {
	count := 0
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		count++

		// Report progress every 10 files to avoid spam
		if progressCallback != nil && count%10 == 0 {
			progressCallback(path, count)
		}

		return nil
	})

	return count, err
}

// ScanDirectory recursively scans a directory and returns file information
func ScanDirectory(rootPath string) (map[string]*FileInfo, error) {
	return ScanDirectoryWithProgress(rootPath, nil)
}

// ScanDirectoryWithProgress recursively scans a directory with progress reporting
func ScanDirectoryWithProgress(rootPath string, progressCallback ScanProgressCallback) (map[string]*FileInfo, error) {
	files := make(map[string]*FileInfo)
	fileCount := 0

	// First, count total files if we have a progress callback
	totalCount := 0
	if progressCallback != nil {
		var err error
		// Use the progress callback during counting too
		totalCount, err = CountFilesWithProgress(rootPath, func(path string, count int) {
			// Report counting progress (with totalCount = 0 to indicate counting phase)
			progressCallback(path, count, 0)
		})
		if err != nil {
			// If counting fails, continue without total count
			totalCount = 0
		}
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		fileInfo := &FileInfo{
			Path:         path,
			RelativePath: relPath,
			Size:         info.Size(),
			ModTime:      info.ModTime(),
			IsDir:        info.IsDir(),
		}

		files[relPath] = fileInfo
		fileCount++

		// Report progress if callback provided
		if progressCallback != nil {
			progressCallback(path, fileCount, totalCount)
		}

		return nil
	})

	return files, err
}

// ComputeFileHash computes SHA256 hash of a file
func ComputeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath) // #nosec G304 - file path is controlled by caller
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// CopyFile copies a file from src to dst with progress reporting
func CopyFile(src, dst string, progress ProgressCallback) (int64, error) {
	sourceFile, err := os.Open(src) // #nosec G304 - file path is controlled by caller
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
	if err := os.MkdirAll(dstDir, 0750); err != nil { // #nosec G301 - directory permissions
		return 0, err
	}

	// Create destination file
	destFile, err := os.Create(dst) // #nosec G304 - file path is controlled by caller
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
		if err == io.EOF {
			break
		}
		if err != nil {
			return written, err
		}
	}

	// Preserve modification time
	if err := os.Chtimes(dst, sourceInfo.ModTime(), sourceInfo.ModTime()); err != nil {
		return written, err
	}

	return written, nil
}

// CopyFileWithStats copies a file and returns detailed timing statistics
// If cancelChan is provided and closed, the copy will be aborted
func CopyFileWithStats(src, dst string, progress ProgressCallback, cancelChan <-chan struct{}) (*CopyStats, error) {
	stats := &CopyStats{}

	sourceFile, err := os.Open(src) // #nosec G304 - file path is controlled by caller
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
	if err := os.MkdirAll(dstDir, 0750); err != nil { // #nosec G301 - directory permissions
		return stats, err
	}

	// Create destination file
	destFile, err := os.Create(dst) // #nosec G304 - file path is controlled by caller
	if err != nil {
		return stats, err
	}

	// Track whether copy completed successfully
	copyCompleted := false
	defer func() {
		_ = destFile.Close()
		// If copy was cancelled or failed, delete the partial file
		if !copyCompleted {
			_ = os.Remove(dst)
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
		if err == io.EOF {
			break
		}
		if err != nil {
			return stats, err
		}
	}

	stats.BytesCopied = written

	// Close the file before setting modification time
	// This is important for network filesystems like SMB
	if err := destFile.Close(); err != nil {
		return stats, err
	}

	// Preserve modification time
	if err := os.Chtimes(dst, sourceInfo.ModTime(), sourceInfo.ModTime()); err != nil {
		return stats, err
	}

	// Mark copy as completed successfully
	copyCompleted = true
	return stats, nil
}

// FilesNeedSync determines if two files need synchronization
func FilesNeedSync(src, dst *FileInfo) bool {
	if dst == nil {
		return true // Destination doesn't exist
	}
	
	// Compare sizes first (quick check)
	if src.Size != dst.Size {
		return true
	}
	
	// Compare modification times
	if !src.ModTime.Equal(dst.ModTime) {
		return true
	}
	
	return false
}

