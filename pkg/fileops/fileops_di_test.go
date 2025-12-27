package fileops_test

//go:generate impgen filesystem.FileSystem
//go:generate impgen filesystem.FileScanner

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/joe/copy-files/pkg/fileops"
	"github.com/joe/copy-files/pkg/filesystem"
)

func TestCountFiles(t *testing.T) {
	t.Parallel()

	fsImp := NewFileSystemImp(t)
	scannerImp := NewFileScannerImp(t)
	ops := fileops.NewFileOps(fsImp.Mock)

	// Set up expectations in a goroutine
	go func() {
		// Expect Scan call
		fsImp.ExpectCallIs.Scan().ExpectArgsAre("/test").InjectResult(scannerImp.Mock)

		// Expect Next calls - return 3 files then done
		scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{RelativePath: "file1.txt", IsDir: false}, true)
		scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{RelativePath: "file2.txt", IsDir: false}, true)
		scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{RelativePath: "dir1", IsDir: true}, true)
		scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{}, false)

		// Expect Err call
		scannerImp.ExpectCallIs.Err().InjectResult(nil)
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

	fsImp := NewFileSystemImp(t)
	scannerImp := NewFileScannerImp(t)
	ops := fileops.NewFileOps(fsImp.Mock)

	progressCalls := 0
	progressCallback := func(path string, count int) {
		progressCalls++
	}

	// Set up expectations in a goroutine
	go func() {
		// Expect Scan call
		fsImp.ExpectCallIs.Scan().ExpectArgsAre("/test").InjectResult(scannerImp.Mock)

		// Expect Next calls - return 10 files to trigger progress callback
		for range 10 {
			scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{RelativePath: "file.txt", IsDir: false}, true)
		}
		scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{}, false)

		// Expect Err call
		scannerImp.ExpectCallIs.Err().InjectResult(nil)
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

func TestFileOpsScanDirectory(t *testing.T) {
	t.Parallel()

	fsImp := NewFileSystemImp(t)
	scannerImp := NewFileScannerImp(t)
	ops := fileops.NewFileOps(fsImp.Mock)

	// Set up expectations in a goroutine
	go func() {
		// Expect Scan call
		fsImp.ExpectCallIs.Scan().ExpectArgsAre("/test").InjectResult(scannerImp.Mock)

		// Expect Next calls - return 2 files then done
		scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{
			RelativePath: "file1.txt",
			Size:         100,
			ModTime:      time.Now(),
			IsDir:        false,
		}, true)
		scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{
			RelativePath: "file2.txt",
			Size:         200,
			ModTime:      time.Now(),
			IsDir:        false,
		}, true)
		scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{}, false)

		// Expect Err call
		scannerImp.ExpectCallIs.Err().InjectResult(nil)
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

	fsImp := NewFileSystemImp(t)
	scannerImp := NewFileScannerImp(t)
	ops := fileops.NewFileOps(fsImp.Mock)

	progressCalls := 0
	progressCallback := func(path string, current, total int) {
		progressCalls++
	}

	// Set up expectations in a goroutine
	go func() {
		// Expect Scan call
		fsImp.ExpectCallIs.Scan().ExpectArgsAre("/test").InjectResult(scannerImp.Mock)

		// Expect Next calls - return 2 files then done
		scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{
			RelativePath: "file1.txt",
			Size:         100,
			ModTime:      time.Now(),
			IsDir:        false,
		}, true)
		scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{
			RelativePath: "file2.txt",
			Size:         200,
			ModTime:      time.Now(),
			IsDir:        false,
		}, true)
		scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{}, false)

		// Expect Err call
		scannerImp.ExpectCallIs.Err().InjectResult(nil)
	}()

	// Call ScanDirectoryWithProgress
	files, err := ops.ScanDirectoryWithProgress("/test", progressCallback)

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(files).Should(HaveLen(2))
	g.Expect(progressCalls).Should(Equal(2))
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

func TestFileOpsCompareFilesBytes(t *testing.T) {
	t.Parallel()

	// Use real filesystem for this test since we need to actually read file content
	tmpDir := t.TempDir()
	file1 := tmpDir + "/file1.txt"
	file2 := tmpDir + "/file2.txt"

	// Create two identical files
	content := []byte("test content")
	if err := os.WriteFile(file1, content, 0o600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(file2, content, 0o600); err != nil {
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

func TestFileOpsCopyFile(t *testing.T) {
	t.Parallel()

	// Use real filesystem for this test
	tmpDir := t.TempDir()
	srcFile := tmpDir + "/source.txt"
	dstFile := tmpDir + "/dest.txt"

	// Create source file
	content := []byte("test content to copy")
	if err := os.WriteFile(srcFile, content, 0o600); err != nil {
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

func TestFileOpsRemove(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.txt"
	if err := os.WriteFile(testFile, []byte("test"), 0o600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ops := fileops.NewRealFileOps()
	err := ops.Remove(testFile)

	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestFileOpsStat(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.txt"
	if err := os.WriteFile(testFile, []byte("test"), 0o600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ops := fileops.NewRealFileOps()
	info, err := ops.Stat(testFile)

	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(info).Should(Not(BeNil()))
}

func TestFileOpsChtimes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.txt"
	if err := os.WriteFile(testFile, []byte("test"), 0o600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ops := fileops.NewRealFileOps()
	now := time.Now()
	err := ops.Chtimes(testFile, now, now)

	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestFileOpsCopyFileWithStats(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := tmpDir + "/source.txt"
	dstFile := tmpDir + "/dest.txt"

	content := []byte("test content")
	if err := os.WriteFile(srcFile, content, 0o600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ops := fileops.NewRealFileOps()
	stats, err := ops.CopyFileWithStats(srcFile, dstFile, nil, nil)

	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(stats).Should(Not(BeNil()))
	g.Expect(stats.BytesCopied).Should(Equal(int64(len(content))))
}
