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
// Supports dual filesystems for cross-filesystem operations (e.g., local to SFTP).
type FileOps struct {
	FS filesystem.FileSystem // Legacy single filesystem field

	// Dual filesystem support (optional)
	SourceFS filesystem.FileSystem // Source filesystem for copy operations
	DestFS   filesystem.FileSystem // Destination filesystem for copy operations
}

// NewFileOps creates a new FileOps instance with the given filesystem.
func NewFileOps(fs filesystem.FileSystem) *FileOps {
	return &FileOps{FS: fs}
}

// NewDualFileOps creates a new FileOps instance with separate source and destination filesystems.
func NewDualFileOps(sourceFS, destFS filesystem.FileSystem) *FileOps {
	return &FileOps{
		FS:       sourceFS, // Default to source for backward compatibility
		SourceFS: sourceFS,
		DestFS:   destFS,
	}
}

// NewRealFileOps creates a new FileOps instance using the real filesystem.
func NewRealFileOps() *FileOps {
	return &FileOps{FS: filesystem.NewRealFileSystem()}
}

// Chtimes changes the access and modification times of a file
func (fo *FileOps) Chtimes(path string, atime, mtime time.Time) error {
	err := fo.FS.Chtimes(path, atime, mtime)
	if err != nil {
		return fmt.Errorf("failed to change times for %s: %w", path, err)
	}

	return nil
}

func (fo *FileOps) CompareFilesBytes(path1, path2 string) (bool, error) {
	// Open both files
	file1, err := fo.FS.Open(path1)
	if err != nil {
		return false, fmt.Errorf("failed to open file %s: %w", path1, err)
	}

	defer func() {
		_ = file1.Close()
	}()

	file2, err := fo.FS.Open(path2)
	if err != nil {
		return false, fmt.Errorf("failed to open file %s: %w", path2, err)
	}

	defer func() {
		_ = file2.Close()
	}()

	// Get file sizes
	info1, err := file1.Stat()
	if err != nil {
		return false, fmt.Errorf("failed to stat file %s: %w", path1, err)
	}

	info2, err := file2.Stat()
	if err != nil {
		return false, fmt.Errorf("failed to stat file %s: %w", path2, err)
	}

	// Quick size check
	if info1.Size() != info2.Size() {
		return false, nil
	}

	// Compare byte-by-byte
	identical, err := fo.compareFileContents(file1, file2)
	if err != nil {
		return false, fmt.Errorf("failed to compare %s and %s: %w", path1, path2, err)
	}

	return identical, nil
}

// ComputeFileHash computes SHA256 hash of a file.
func (fo *FileOps) ComputeFileHash(filePath string) (string, error) {
	file, err := fo.FS.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	defer func() {
		_ = file.Close()
	}()

	hash := sha256.New()

	_, err = io.Copy(hash, file)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s for hashing: %w", filePath, err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (fo *FileOps) CopyFile(src, dst string, progress ProgressCallback) (int64, error) {
	// Get source and destination filesystems
	srcFS := fo.getSourceFS()
	dstFS := fo.getDestFS()

	sourceFile, err := srcFS.Open(src)
	if err != nil {
		return 0, fmt.Errorf("failed to open source file %s: %w", src, err)
	}

	defer func() {
		_ = sourceFile.Close()
	}()

	// Get source file info
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat source file %s: %w", src, err)
	}

	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dst)

	err = dstFS.MkdirAll(dstDir, DefaultDirPermissions)
	if err != nil {
		return 0, fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	// Create destination file
	destFile, err := dstFS.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}

	defer func() {
		_ = destFile.Close()
	}()

	// Copy with progress tracking
	written, err := fo.simpleCopyLoop(sourceFile, destFile, sourceInfo.Size(), src, progress)
	if err != nil {
		return written, fmt.Errorf("failed to copy %s to %s: %w", src, dst, err)
	}

	// Preserve modification time
	err = dstFS.Chtimes(dst, sourceInfo.ModTime(), sourceInfo.ModTime())
	if err != nil {
		return written, fmt.Errorf("failed to preserve modification time for %s: %w", dst, err)
	}

	return written, nil
}

// getSourceFS returns the source filesystem, falling back to FS if SourceFS is not set.
func (fo *FileOps) getSourceFS() filesystem.FileSystem {
	if fo.SourceFS != nil {
		return fo.SourceFS
	}
	return fo.FS
}

// getDestFS returns the destination filesystem, falling back to FS if DestFS is not set.
func (fo *FileOps) getDestFS() filesystem.FileSystem {
	if fo.DestFS != nil {
		return fo.DestFS
	}
	return fo.FS
}

// CopyFileWithStats copies a file and returns detailed timing statistics.
// If cancelChan is provided and closed, the copy will be aborted.
// If onDataComplete is provided, it will be called after data transfer but before file close/chtimes.
//
//nolint:lll,funlen // Long function signature with channel parameter; function handles file copy with finalization callback
func (fo *FileOps) CopyFileWithStats(src, dst string, progress ProgressCallback, cancelChan <-chan struct{}, onDataComplete func()) (*CopyStats, error) {
	stats := &CopyStats{}

	// Get source and destination filesystems
	srcFS := fo.getSourceFS()
	dstFS := fo.getDestFS()

	sourceFile, err := srcFS.Open(src)
	if err != nil {
		return stats, fmt.Errorf("failed to open source file %s: %w", src, err)
	}

	defer func() {
		_ = sourceFile.Close()
	}()

	// Get source file info
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return stats, fmt.Errorf("failed to stat source file %s: %w", src, err)
	}

	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dst)

	err = dstFS.MkdirAll(dstDir, DefaultDirPermissions)
	if err != nil {
		return stats, fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	// Create destination file
	destFile, err := dstFS.Create(dst)
	if err != nil {
		return stats, fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}

	// Track whether copy completed successfully
	copyCompleted := false

	defer func() {
		_ = destFile.Close()
		// If copy was cancelled or failed, delete the partial file
		if !copyCompleted {
			_ = dstFS.Remove(dst)
		}
	}()

	// Copy with progress tracking and timing
	written, err := fo.copyLoop(sourceFile, destFile, stats, sourceInfo.Size(), src, progress, cancelChan)
	if err != nil {
		return stats, fmt.Errorf("failed to copy %s to %s: %w", src, dst, err)
	}

	stats.BytesCopied = written

	// Call onDataComplete callback after data transfer completes, before file finalization
	if onDataComplete != nil {
		onDataComplete()
	}

	// Close the file before setting modification time
	// This is important for network filesystems like SMB
	err = destFile.Close()
	if err != nil {
		return stats, fmt.Errorf("failed to close destination file %s: %w", dst, err)
	}

	// Preserve modification time
	err = dstFS.Chtimes(dst, sourceInfo.ModTime(), sourceInfo.ModTime())
	if err != nil {
		return stats, fmt.Errorf("failed to preserve modification time for %s: %w", dst, err)
	}

	// Mark copy as completed successfully
	copyCompleted = true

	return stats, nil
}

// CountFiles quickly counts the total number of files/directories in a path.
func (fo *FileOps) CountFiles(rootPath string) (int, error) {
	return fo.CountFilesWithProgress(rootPath, nil)
}

// CountFilesWithProgress counts files with progress reporting.
// Uses fo.FS for single-filesystem operations, or fo.getSourceFS() for dual-filesystem.
func (fo *FileOps) CountFilesWithProgress(rootPath string, progressCallback CountProgressCallback) (int, error) {
	fs := fo.getSourceFS()
	return fo.countFilesWithProgressFS(fs, rootPath, progressCallback)
}

// CountDestFilesWithProgress counts destination files with progress reporting.
// Used for dual-filesystem operations where source and dest are different.
func (fo *FileOps) CountDestFilesWithProgress(rootPath string, progressCallback CountProgressCallback) (int, error) {
	fs := fo.getDestFS()
	return fo.countFilesWithProgressFS(fs, rootPath, progressCallback)
}

// countFilesWithProgressFS counts files using the specified filesystem.
func (fo *FileOps) countFilesWithProgressFS(fs filesystem.FileSystem, rootPath string, progressCallback CountProgressCallback) (int, error) {
	scanner := fs.Scan(rootPath)
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
		return count, fmt.Errorf("failed to count files in %s: %w", rootPath, err)
	}

	return count, nil
}

// Remove removes a file or empty directory.
// Uses fo.FS for single-filesystem operations, or fo.getSourceFS() for dual-filesystem.
func (fo *FileOps) Remove(path string) error {
	fs := fo.getSourceFS()
	err := fs.Remove(path)
	if err != nil {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}

	return nil
}

// RemoveFromDest removes a file or empty directory from the destination filesystem.
// Used for dual-filesystem operations where source and dest are different.
func (fo *FileOps) RemoveFromDest(path string) error {
	fs := fo.getDestFS()
	err := fs.Remove(path)
	if err != nil {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}

	return nil
}

// ScanDirectory recursively scans a directory and returns file information.
func (fo *FileOps) ScanDirectory(rootPath string) (map[string]*FileInfo, error) {
	return fo.ScanDirectoryWithProgress(rootPath, nil)
}

// ScanDirectoryWithProgress recursively scans a directory with progress reporting.
// Uses fo.FS for single-filesystem operations, or fo.getSourceFS() for dual-filesystem.
//
//nolint:lll // Long function signature with callback parameter
func (fo *FileOps) ScanDirectoryWithProgress(rootPath string, progressCallback ScanProgressCallback) (map[string]*FileInfo, error) {
	fs := fo.getSourceFS()
	return fo.scanDirectoryWithProgressFS(fs, rootPath, progressCallback)
}

// ScanDestDirectoryWithProgress recursively scans destination directory with progress reporting.
// Used for dual-filesystem operations where source and dest are different.
//
//nolint:lll // Long function signature with callback parameter
func (fo *FileOps) ScanDestDirectoryWithProgress(rootPath string, progressCallback ScanProgressCallback) (map[string]*FileInfo, error) {
	fs := fo.getDestFS()
	return fo.scanDirectoryWithProgressFS(fs, rootPath, progressCallback)
}

// scanDirectoryWithProgressFS scans a directory using the specified filesystem.
func (fo *FileOps) scanDirectoryWithProgressFS(fs filesystem.FileSystem, rootPath string, progressCallback ScanProgressCallback) (map[string]*FileInfo, error) {
	files := make(map[string]*FileInfo)
	fileCount := 0

	scanner := fs.Scan(rootPath)
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
			progressCallback(path, fileCount, 0, info.Size)
		}
	}

	err := scanner.Err()
	if err != nil {
		return files, fmt.Errorf("failed to scan directory %s: %w", rootPath, err)
	}

	return files, nil
}

// Stat returns file information
func (fo *FileOps) Stat(path string) (os.FileInfo, error) {
	info, err := fo.FS.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat %s: %w", path, err)
	}

	return info, nil
}

// compareFileContents performs byte-by-byte comparison of two open files.
func (fo *FileOps) compareFileContents(file1, file2 filesystem.File) (bool, error) {
	buf1 := make([]byte, BufferSize)
	buf2 := make([]byte, BufferSize)

	for {
		//nolint:varnamelen // n1/n2 are idiomatic for bytes read
		n1, err1 := file1.Read(buf1)
		n2, err2 := file2.Read(buf2)

		if n1 != n2 {
			return false, nil
		}

		if n1 > 0 && !compareByteBuffers(buf1, buf2, n1) {
			return false, nil
		}

		if errors.Is(err1, io.EOF) && errors.Is(err2, io.EOF) {
			return true, nil
		}

		if err1 != nil {
			return false, fmt.Errorf("failed to read from first file: %w", err1)
		}

		if err2 != nil {
			return false, fmt.Errorf("failed to read from second file: %w", err2)
		}
	}
}

// copyLoop performs the actual file copy with progress tracking and timing.
//
//nolint:lll // Long function signature with many parameters including channel
func (fo *FileOps) copyLoop(sourceFile filesystem.File, destFile filesystem.File, stats *CopyStats, sourceSize int64, srcPath string, progress ProgressCallback, cancelChan <-chan struct{}) (int64, error) {
	var written int64

	buf := make([]byte, BufferSize) // 32KB buffer

	var (
		nr, nw int //nolint:varnamelen // nr/nw are idiomatic for bytes read/written
		err    error
	)

	for {
		err = checkCancellation(cancelChan)
		if err != nil {
			return written, err
		}

		// Time the read operation
		readStart := time.Now()
		nr, err = sourceFile.Read(buf)
		stats.ReadTime += time.Since(readStart)

		if nr > 0 {
			nw, err = writeBufferWithTiming(destFile, buf, nr, stats)
			if err != nil {
				return written, fmt.Errorf("failed to write to destination: %w", err)
			}

			if nr != nw {
				return written, fmt.Errorf("short write: %w", io.ErrShortWrite)
			}

			written += int64(nw)

			if progress != nil {
				progress(written, sourceSize, srcPath)
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return written, fmt.Errorf("failed to read from source: %w", err)
		}
	}

	return written, nil
}

// CopyFile copies a file from src to dst with progress reporting.
// simpleCopyLoop performs a basic file copy with progress tracking.
//
//nolint:lll // Long function signature with many parameters
func (fo *FileOps) simpleCopyLoop(sourceFile filesystem.File, destFile filesystem.File, sourceSize int64, srcPath string, progress ProgressCallback) (int64, error) {
	var written int64

	buf := make([]byte, BufferSize) // 32KB buffer

	for {
		nr, err := sourceFile.Read(buf) //nolint:varnamelen // nr is idiomatic for bytes read
		if nr > 0 {
			nw, err := destFile.Write(buf[0:nr]) //nolint:varnamelen // nw is idiomatic for bytes written
			if err != nil {
				return written, fmt.Errorf("failed to write to destination: %w", err)
			}

			if nr != nw {
				return written, fmt.Errorf("short write: %w", io.ErrShortWrite)
			}

			written += int64(nw)

			if progress != nil {
				progress(written, sourceSize, srcPath)
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return written, fmt.Errorf("failed to read from source: %w", err)
		}
	}

	return written, nil
}

// compareByteBuffers compares two byte buffers up to n bytes.
func compareByteBuffers(buf1, buf2 []byte, n int) bool {
	for i := range n {
		if buf1[i] != buf2[i] {
			return false
		}
	}

	return true
}

// writeBufferWithTiming writes a buffer to a file and tracks the write time.
func writeBufferWithTiming(destFile filesystem.File, buf []byte, nr int, stats *CopyStats) (int, error) {
	writeStart := time.Now()
	nw, err := destFile.Write(buf[0:nr])
	stats.WriteTime += time.Since(writeStart)

	return nw, err //nolint:wrapcheck // Error is from io.Writer interface, context is clear
}
