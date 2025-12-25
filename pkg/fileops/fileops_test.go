package fileops_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joe/copy-files/pkg/fileops"
	. "github.com/onsi/gomega"
)

//go:generate impgen fileops.ScanDirectory
//go:generate impgen fileops.ComputeFileHash
//go:generate impgen fileops.CopyFile
//go:generate impgen fileops.FilesNeedSync

func TestScanDirectory(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create some test files
	testFiles := []string{
		"file1.txt",
		"subdir/file2.txt",
		"subdir/file3.txt",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Use imptest callable wrapper
	scanImp := NewScanDirectoryImp(t, fileops.ScanDirectory).Start(tmpDir)

	// Verify we found all files and check that relative paths are correct
	scanImp.ExpectReturnedValuesShould(
		And(
			HaveLen(4), // 3 files + 1 directory
			HaveKey("file1.txt"),
			HaveKey("subdir/file2.txt"),
			HaveKey("subdir/file3.txt"),
		),
		BeNil(), // no error
	)
}

func TestComputeFileHash(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("test content for hashing")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// First hash
	hashImp1 := NewComputeFileHashImp(t, fileops.ComputeFileHash).Start(testFile)
	hashImp1.ExpectReturnedValuesShould(
		Not(BeEmpty()), // hash should not be empty
		BeNil(),        // no error
	)
	hash1 := hashImp1.Returned.Result0

	// Hash should be consistent
	hashImp2 := NewComputeFileHashImp(t, fileops.ComputeFileHash).Start(testFile)
	hashImp2.ExpectReturnedValuesAre(hash1, nil)

	// Different content should produce different hash
	if err := os.WriteFile(testFile, []byte("different content"), 0644); err != nil {
		t.Fatalf("Failed to write different content: %v", err)
	}
	hashImp3 := NewComputeFileHashImp(t, fileops.ComputeFileHash).Start(testFile)
	hashImp3.ExpectReturnedValuesShould(
		Not(Equal(hash1)), // different hash
		BeNil(),           // no error
	)
}

func TestCopyFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest", "destination.txt")

	content := []byte("test content to copy")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	var progressCalls int
	progressCallback := func(bytesTransferred, totalBytes int64, currentFile string) {
		progressCalls++
		if bytesTransferred > totalBytes {
			t.Errorf("bytesTransferred (%d) > totalBytes (%d)", bytesTransferred, totalBytes)
		}
	}

	copyImp := NewCopyFileImp(t, fileops.CopyFile).Start(srcFile, dstFile, progressCallback)
	copyImp.ExpectReturnedValuesAre(int64(len(content)), nil)

	if progressCalls == 0 {
		t.Error("Expected progress callback to be called")
	}

	// Verify destination file exists and has correct content
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	if string(dstContent) != string(content) {
		t.Errorf("Content mismatch: expected %q, got %q", content, dstContent)
	}

	// Verify modification time is preserved
	srcInfo, err := os.Stat(srcFile)
	if err != nil {
		t.Fatalf("Failed to stat source file: %v", err)
	}
	dstInfo, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("Failed to stat destination file: %v", err)
	}
	if !srcInfo.ModTime().Equal(dstInfo.ModTime()) {
		t.Error("Modification times don't match")
	}
}

func TestFilesNeedSync(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name     string
		src      *fileops.FileInfo
		dst      *fileops.FileInfo
		expected bool
	}{
		{
			name:     "destination doesn't exist",
			src:      &fileops.FileInfo{Size: 100, ModTime: now},
			dst:      nil,
			expected: true,
		},
		{
			name:     "different sizes",
			src:      &fileops.FileInfo{Size: 100, ModTime: now},
			dst:      &fileops.FileInfo{Size: 200, ModTime: now},
			expected: true,
		},
		{
			name:     "different mod times",
			src:      &fileops.FileInfo{Size: 100, ModTime: now},
			dst:      &fileops.FileInfo{Size: 100, ModTime: now.Add(-time.Hour)},
			expected: true,
		},
		{
			name:     "identical files",
			src:      &fileops.FileInfo{Size: 100, ModTime: now},
			dst:      &fileops.FileInfo{Size: 100, ModTime: now},
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			needSyncImp := NewFilesNeedSyncImp(t, fileops.FilesNeedSync).Start(tt.src, tt.dst)
			needSyncImp.ExpectReturnedValuesAre(tt.expected)
		})
	}
}

func TestCopyFileWithProgress(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "large.txt")
	dstFile := filepath.Join(tmpDir, "large_copy.txt")

	// Create a larger file to test progress reporting
	content := make([]byte, 100*1024) // 100KB
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	var lastBytes int64
	progressCallback := func(bytesTransferred, totalBytes int64, currentFile string) {
		if bytesTransferred < lastBytes {
			t.Errorf("bytesTransferred (%d) < lastBytes (%d)", bytesTransferred, lastBytes)
		}
		if totalBytes != int64(len(content)) {
			t.Errorf("totalBytes (%d) != expected (%d)", totalBytes, len(content))
		}
		if currentFile != srcFile {
			t.Errorf("currentFile (%s) != expected (%s)", currentFile, srcFile)
		}
		lastBytes = bytesTransferred
	}

	copyImp := NewCopyFileImp(t, fileops.CopyFile).Start(srcFile, dstFile, progressCallback)
	copyImp.ExpectReturnedValuesAre(int64(len(content)), nil)

	if lastBytes != int64(len(content)) {
		t.Errorf("lastBytes (%d) != expected (%d)", lastBytes, len(content))
	}
}

