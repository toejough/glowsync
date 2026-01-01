//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package fileops_test

//go:generate impgen --dependency filesystem.FileSystem
//go:generate impgen --dependency filesystem.FileScanner

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/pkg/fileops"
	"github.com/joe/copy-files/pkg/filesystem"
)

// TestFileOps_BufferSize_Is64KB verifies that FileOps uses 64KB buffer size.
// This test will FAIL until Phase 1.1 increases BufferSize to 64KB.
func TestFileOps_BufferSize_Is64KB(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Verify BufferSize constant is 64KB
	expectedBufferSize := 64 * 1024
	g.Expect(fileops.BufferSize).Should(Equal(expectedBufferSize),
		"BufferSize should be 64KB for improved FileOps performance")

	// The copyLoop (line 449) and simpleCopyLoop (line 503) in fileops_di.go
	// both allocate: buf := make([]byte, BufferSize)
	// This ensures the buffer allocation uses 64KB
}

// TestFileOps_CompareFilesBytes_Uses64KBBuffer verifies buffer allocation in comparison.
// This test will FAIL until Phase 1.1 increases BufferSize to 64KB.
func TestFileOps_CompareFilesBytes_Uses64KBBuffer(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Verify BufferSize constant is 64KB
	expectedBufferSize := 64 * 1024
	g.Expect(fileops.BufferSize).Should(Equal(expectedBufferSize),
		"BufferSize should be 64KB for compareFileContents (line 413-414 in fileops_di.go)")

	// The compareFileContents method allocates two buffers:
	// buf1 := make([]byte, BufferSize)
	// buf2 := make([]byte, BufferSize)
	// This test verifies they use 64KB
}

func TestCountFiles(t *testing.T) {
	t.Parallel()

	fsMock := MockFileSystem(t)
	scannerMock := MockFileScanner(t)
	ops := fileops.NewFileOps(fsMock.Interface())

	// Set up expectations in a goroutine
	go func() {
		// Expect Scan call
		fsMock.Scan.ExpectCalledWithExactly("/test").InjectReturnValues(scannerMock.Interface())

		// Expect Next calls - return 3 files then done
		scannerMock.Next.ExpectCalledWithExactly().InjectReturnValues(filesystem.FileInfo{RelativePath: "file1.txt", IsDir: false}, true)
		scannerMock.Next.ExpectCalledWithExactly().InjectReturnValues(filesystem.FileInfo{RelativePath: "file2.txt", IsDir: false}, true)
		scannerMock.Next.ExpectCalledWithExactly().InjectReturnValues(filesystem.FileInfo{RelativePath: "dir1", IsDir: true}, true)
		scannerMock.Next.ExpectCalledWithExactly().InjectReturnValues(filesystem.FileInfo{}, false)

		// Expect Err call
		scannerMock.Err.ExpectCalledWithExactly().InjectReturnValues(nil)
	}()

	// Call CountFiles
	count, err := ops.CountFiles("/test")

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(count).Should(Equal(3))
}

func TestCountFilesWithProgress(t *testing.T) {
	t.Parallel()

	fsMock := MockFileSystem(t)
	scannerMock := MockFileScanner(t)
	ops := fileops.NewFileOps(fsMock.Interface())

	progressCalls := 0
	progressCallback := func(_ string, _ int) {
		progressCalls++
	}

	// Set up expectations in a goroutine
	go func() {
		// Expect Scan call
		fsMock.Scan.ExpectCalledWithExactly("/test").InjectReturnValues(scannerMock.Interface())

		// Expect Next calls - return 10 files to trigger progress callback
		for range 10 {
			scannerMock.Next.ExpectCalledWithExactly().InjectReturnValues(filesystem.FileInfo{RelativePath: "file.txt", IsDir: false}, true)
		}

		scannerMock.Next.ExpectCalledWithExactly().InjectReturnValues(filesystem.FileInfo{}, false)

		// Expect Err call
		scannerMock.Err.ExpectCalledWithExactly().InjectReturnValues(nil)
	}()

	// Call CountFilesWithProgress
	count, err := ops.CountFilesWithProgress("/test", progressCallback)

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(count).Should(Equal(10))
	// Progress callback is called every 10 files, so should be called once
	g.Expect(progressCalls).Should(Equal(1))
}

func TestFileOpsChtimes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	testFile := tmpDir + "/test.txt"

	err := os.WriteFile(testFile, []byte("test"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ops := fileops.NewRealFileOps()
	now := time.Now()

	err = ops.Chtimes(testFile, now, now)

	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestFileOpsCompareFilesBytes(t *testing.T) {
	t.Parallel()

	// Use real filesystem for this test since we need to actually read file content
	tmpDir := t.TempDir()
	file1 := tmpDir + "/file1.txt"
	file2 := tmpDir + "/file2.txt"

	// Create two identical files
	content := []byte("test content")

	err := os.WriteFile(file1, content, 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = os.WriteFile(file2, content, 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create FileOps with real filesystem
	ops := fileops.NewRealFileOps()

	// Compare files
	same, err := ops.CompareFilesBytes(file1, file2)

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(same).Should(BeTrue())
}

func TestFileOpsComputeFileHash(t *testing.T) {
	t.Parallel()

	// Use real filesystem for this test since we need to actually read file content
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.txt"

	// Create a test file
	content := []byte("test content for hashing")

	err := os.WriteFile(testFile, content, 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create FileOps with real filesystem
	ops := fileops.NewRealFileOps()

	// Compute hash
	hash, err := ops.ComputeFileHash(testFile)

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(hash).Should(Not(BeEmpty()))
}

func TestFileOpsCopyFile(t *testing.T) {
	t.Parallel()

	// Use real filesystem for this test
	tmpDir := t.TempDir()
	srcFile := tmpDir + "/source.txt"
	dstFile := tmpDir + "/dest.txt"

	// Create source file
	content := []byte("test content to copy")

	err := os.WriteFile(srcFile, content, 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create FileOps with real filesystem
	ops := fileops.NewRealFileOps()

	// Copy file
	bytesWritten, err := ops.CopyFile(srcFile, dstFile, nil)

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(bytesWritten).Should(Equal(int64(len(content))))

	// Verify destination file exists and has same content
	dstContent, err := os.ReadFile(dstFile)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(dstContent).Should(Equal(content))
}

func TestFileOpsCopyFileWithStats(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := tmpDir + "/source.txt"
	dstFile := tmpDir + "/dest.txt"

	content := []byte("test content")

	err := os.WriteFile(srcFile, content, 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ops := fileops.NewRealFileOps()
	stats, err := ops.CopyFileWithStats(srcFile, dstFile, nil, nil, nil)

	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(stats).Should(Not(BeNil()))
	g.Expect(stats.BytesCopied).Should(Equal(int64(len(content))))
}

func TestFileOpsRemove(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	testFile := tmpDir + "/test.txt"

	err := os.WriteFile(testFile, []byte("test"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ops := fileops.NewRealFileOps()
	err = ops.Remove(testFile)

	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestFileOpsScanDirectory(t *testing.T) {
	t.Parallel()

	fsMock := MockFileSystem(t)
	scannerMock := MockFileScanner(t)
	ops := fileops.NewFileOps(fsMock.Interface())

	// Set up expectations in a goroutine
	go func() {
		// Expect Scan call
		fsMock.Scan.ExpectCalledWithExactly("/test").InjectReturnValues(scannerMock.Interface())

		// Expect Next calls - return 2 files then done
		scannerMock.Next.ExpectCalledWithExactly().InjectReturnValues(filesystem.FileInfo{
			RelativePath: "file1.txt",
			Size:         100,
			ModTime:      time.Now(),
			IsDir:        false,
		}, true)
		scannerMock.Next.ExpectCalledWithExactly().InjectReturnValues(filesystem.FileInfo{
			RelativePath: "file2.txt",
			Size:         200,
			ModTime:      time.Now(),
			IsDir:        false,
		}, true)
		scannerMock.Next.ExpectCalledWithExactly().InjectReturnValues(filesystem.FileInfo{}, false)

		// Expect Err call
		scannerMock.Err.ExpectCalledWithExactly().InjectReturnValues(nil)
	}()

	// Call ScanDirectory
	files, err := ops.ScanDirectory("/test")

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(files).Should(HaveLen(2))
	g.Expect(files).Should(HaveKey("file1.txt"))
	g.Expect(files).Should(HaveKey("file2.txt"))
	g.Expect(files["file1.txt"].Size).Should(Equal(int64(100)))
	g.Expect(files["file2.txt"].Size).Should(Equal(int64(200)))
}

func TestFileOpsScanDirectoryWithProgress(t *testing.T) {
	t.Parallel()

	fsMock := MockFileSystem(t)
	scannerMock := MockFileScanner(t)
	ops := fileops.NewFileOps(fsMock.Interface())

	progressCalls := 0
	progressCallback := func(_ string, _, _ int, _ int64) {
		progressCalls++
	}

	// Set up expectations in a goroutine
	go func() {
		// Expect Scan call
		fsMock.Scan.ExpectCalledWithExactly("/test").InjectReturnValues(scannerMock.Interface())

		// Expect Next calls - return 2 files then done
		scannerMock.Next.ExpectCalledWithExactly().InjectReturnValues(filesystem.FileInfo{
			RelativePath: "file1.txt",
			Size:         100,
			ModTime:      time.Now(),
			IsDir:        false,
		}, true)
		scannerMock.Next.ExpectCalledWithExactly().InjectReturnValues(filesystem.FileInfo{
			RelativePath: "file2.txt",
			Size:         200,
			ModTime:      time.Now(),
			IsDir:        false,
		}, true)
		scannerMock.Next.ExpectCalledWithExactly().InjectReturnValues(filesystem.FileInfo{}, false)

		// Expect Err call
		scannerMock.Err.ExpectCalledWithExactly().InjectReturnValues(nil)
	}()

	// Call ScanDirectoryWithProgress
	files, err := ops.ScanDirectoryWithProgress("/test", progressCallback)

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(files).Should(HaveLen(2))
	g.Expect(progressCalls).Should(Equal(2))
}

func TestFileOpsStat(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	testFile := tmpDir + "/test.txt"

	err := os.WriteFile(testFile, []byte("test"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ops := fileops.NewRealFileOps()
	info, err := ops.Stat(testFile)

	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(info).Should(Not(BeNil()))
}
