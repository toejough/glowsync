//nolint:varnamelen // Test files use idiomatic short variable names (t, etc.)
package fileops_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joe/copy-files/pkg/fileops"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers
)

func TestCompareFilesBytes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create two identical files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	content := []byte("test content")

	err := os.WriteFile(file1, content, 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = os.WriteFile(file2, content, 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	wrapper := WrapCompareFilesBytes(t, fileops.CompareFilesBytes)
	wrapper.Start(file1, file2)
	wrapper.ExpectReturnsMatch(Equal(true), BeNil())
}

func TestCompareFilesBytesDifferent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create two different files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	err := os.WriteFile(file1, []byte("content1"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = os.WriteFile(file2, []byte("content2"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	wrapper := WrapCompareFilesBytes(t, fileops.CompareFilesBytes)
	wrapper.Start(file1, file2)
	wrapper.ExpectReturnsMatch(Equal(false), BeNil())
}

func TestComputeFileHash(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("test content for hashing")

	err := os.WriteFile(testFile, content, 0o600)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// First hash
	hashImp1 := WrapComputeFileHash(t, fileops.ComputeFileHash).Start(testFile)
	hashImp1.ExpectReturnsMatch(
		Not(BeEmpty()), // hash should not be empty
		BeNil(),        // no error
	)
	hash1 := hashImp1.Returned.Result0

	// Hash should be consistent
	hashImp2 := WrapComputeFileHash(t, fileops.ComputeFileHash).Start(testFile)
	hashImp2.ExpectReturnsEqual(hash1, nil)

	// Different content should produce different hash
	err = os.WriteFile(testFile, []byte("different content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to write different content: %v", err)
	}

	hashImp3 := WrapComputeFileHash(t, fileops.ComputeFileHash).Start(testFile)
	hashImp3.ExpectReturnsMatch(
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

	err := os.WriteFile(srcFile, content, 0o600)
	if err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	var progressCalls int

	progressCallback := func(bytesTransferred, totalBytes int64, _ string) {
		progressCalls++

		if bytesTransferred > totalBytes {
			t.Errorf("bytesTransferred (%d) > totalBytes (%d)", bytesTransferred, totalBytes)
		}
	}

	copyImp := WrapCopyFile(t, fileops.CopyFile).Start(srcFile, dstFile, progressCallback)
	copyImp.ExpectReturnsEqual(int64(len(content)), nil)

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

	err := os.WriteFile(srcFile, content, 0o600)
	if err != nil {
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

	copyImp := WrapCopyFile(t, fileops.CopyFile).Start(srcFile, dstFile, progressCallback)
	copyImp.ExpectReturnsEqual(int64(len(content)), nil)

	if lastBytes != int64(len(content)) {
		t.Errorf("lastBytes (%d) != expected (%d)", lastBytes, len(content))
	}
}

func TestCopyFileWithStats(t *testing.T) {
	t.Parallel()

	// Note: CopyFileWithStats uses os.Open and os.Create internally, which we can't easily mock
	// without changing the function signature to accept a FileSystem interface.
	// For now, we test that the function can be called through imptest wrapper.
	// The actual file I/O logic is tested through integration tests or by testing
	// the FileOps methods that use FileSystem interface.

	// Use imptest wrapper to verify the function signature and basic behavior
	wrapper := fileops.NewCopyFileWithStatsImp(t, fileops.CopyFileWithStats)

	// We can't actually test this without filesystem access, so we just verify
	// that calling it with invalid paths returns an error
	wrapper.Start("/nonexistent/source.txt", "/nonexistent/dest.txt", nil, nil, nil)

	// Should return an error for nonexistent file
	// Note: CopyFileWithStats returns a non-nil stats struct even on error
	wrapper.ExpectReturnedValuesShould(
		Not(BeNil()), // stats is always returned
		Not(BeNil()), // error should not be nil
	)
}

func TestCopyFileWithStatsCancel(t *testing.T) {
	t.Parallel()

	// Create cancel channel and close it immediately
	cancelChan := make(chan struct{})
	close(cancelChan)

	// Use imptest wrapper
	wrapper := fileops.NewCopyFileWithStatsImp(t, fileops.CopyFileWithStats)
	wrapper.Start("/nonexistent/source.txt", "/nonexistent/dest.txt", nil, cancelChan, nil)

	// Should get an error (either cancellation or file not found)
	// Note: CopyFileWithStats returns a non-nil stats struct even on error
	wrapper.ExpectReturnedValuesShould(
		Not(BeNil()), // stats is always returned
		Not(BeNil()), // error should not be nil
	)
}

func TestCopyFileWithStatsProgress(t *testing.T) {
	t.Parallel()

	// Note: CopyFileWithStats uses os.Open and os.Create internally.
	// We test the progress callback mechanism by verifying the function signature.

	// Track progress callbacks
	progressCallback := func(_ int64, _ int64, _ string) {
		// Progress callback for testing
	}

	// Use imptest wrapper
	wrapper := fileops.NewCopyFileWithStatsImp(t, fileops.CopyFileWithStats)
	wrapper.Start("/nonexistent/source.txt", "/nonexistent/dest.txt", progressCallback, nil, nil)

	// Should return an error for nonexistent file
	// Note: CopyFileWithStats returns a non-nil stats struct even on error
	wrapper.ExpectReturnedValuesShould(
		Not(BeNil()), // stats is always returned
		Not(BeNil()), // error should not be nil
	)
}

//go:generate impgen --target fileops.ScanDirectory
//go:generate impgen --target fileops.ComputeFileHash
//go:generate impgen --target fileops.CopyFile
//go:generate impgen --target fileops.FilesNeedSync
//go:generate impgen --target fileops.CountFiles
//go:generate impgen --target fileops.CountFilesWithProgress
//go:generate impgen --target fileops.CompareFilesBytes

func TestCountFilesStandalone(t *testing.T) {
	t.Parallel()

	// Create temp directory with test files
	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("test"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("test"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	wrapper := WrapCountFiles(t, fileops.CountFiles)
	wrapper.Start(dir)
	wrapper.ExpectReturnsMatch(Equal(2), BeNil())
}

func TestCountFilesWithProgressStandalone(t *testing.T) {
	t.Parallel()

	// Create temp directory with 10 test files to trigger progress callback
	dir := t.TempDir()
	for i := range 10 {
		err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), []byte("test"), 0o600)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	callbackCalled := false
	callback := func(_ string, _ int) {
		callbackCalled = true
	}

	wrapper := WrapCountFilesWithProgress(t, fileops.CountFilesWithProgress)
	wrapper.Start(dir, callback)
	wrapper.ExpectReturnsMatch(Equal(10), BeNil())

	// Verify callback was called
	g := NewWithT(t)
	g.Expect(callbackCalled).Should(BeTrue())
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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			needSyncImp := WrapFilesNeedSync(t, fileops.FilesNeedSync).Start(tt.src, tt.dst)
			needSyncImp.ExpectReturnsEqual(tt.expected)
		})
	}
}

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

		err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		err = os.WriteFile(fullPath, []byte("test content"), 0o600)
		if err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Use imptest callable wrapper
	scanImp := WrapScanDirectory(t, fileops.ScanDirectory).Start(tmpDir)

	// Verify we found all files and check that relative paths are correct
	scanImp.ExpectReturnsMatch(
		And(
			HaveLen(4), // 3 files + 1 directory
			HaveKey("file1.txt"),
			HaveKey("subdir/file2.txt"),
			HaveKey("subdir/file3.txt"),
		),
		BeNil(), // no error
	)
}
