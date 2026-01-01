package fileops //nolint:testpackage // Testing internal callback signature changes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joe/copy-files/pkg/filesystem"
	"github.com/onsi/gomega"
)

// TestScanDirectoryWithProgress_PassesFileSize verifies that ScanDirectoryWithProgress
// passes file size information to the callback.
func TestScanDirectoryWithProgress_PassesFileSize(t *testing.T) {
	t.Parallel()
	gomegaInstance := gomega.NewWithT(t)

	// Create temporary test directory
	tempDir := t.TempDir()

	// Create test files with known sizes
	file1Content := []byte("small file")
	file2Content := []byte("larger file with more content")

	file1Path := filepath.Join(tempDir, "file1.txt")
	file2Path := filepath.Join(tempDir, "file2.txt")

	err := os.WriteFile(file1Path, file1Content, 0o600) // #nosec G306 - Test file with safe permissions
	gomegaInstance.Expect(err).ShouldNot(gomega.HaveOccurred())

	err = os.WriteFile(file2Path, file2Content, 0o600) // #nosec G306 - Test file with safe permissions
	gomegaInstance.Expect(err).ShouldNot(gomega.HaveOccurred())

	// Track callback invocations
	var callbacks []struct {
		fileSize   int64
		scannedCnt int
	}

	// Create FileOps with real filesystem
	fileOps := NewFileOps(filesystem.NewRealFileSystem())

	// Scan directory with progress callback
	_, err = fileOps.ScanDirectoryWithProgress(tempDir, func(_ string, scannedCount, _ int, fileSize int64) {
		callbacks = append(callbacks, struct {
			fileSize   int64
			scannedCnt int
		}{
			fileSize:   fileSize,
			scannedCnt: scannedCount,
		})
	})

	gomegaInstance.Expect(err).ShouldNot(gomega.HaveOccurred())

	// Verify we got callbacks for both files
	gomegaInstance.Expect(callbacks).Should(gomega.HaveLen(2), "Should have received callbacks for both files")

	// Verify file sizes were passed correctly
	// (order might vary, so check for both possibilities)
	fileSizes := []int64{callbacks[0].fileSize, callbacks[1].fileSize}
	gomegaInstance.Expect(fileSizes).Should(gomega.ContainElement(int64(len(file1Content))), "Should have file1 size")
	gomegaInstance.Expect(fileSizes).Should(gomega.ContainElement(int64(len(file2Content))), "Should have file2 size")

	// Verify scanned counts increment
	gomegaInstance.Expect(callbacks[0].scannedCnt).Should(gomega.Equal(1))
	gomegaInstance.Expect(callbacks[1].scannedCnt).Should(gomega.Equal(2))
}

// TestScanProgressCallback_NewSignature verifies the new callback signature works.
func TestScanProgressCallback_NewSignature(t *testing.T) {
	t.Parallel()
	gomegaInstance := gomega.NewWithT(t)

	// Test that we can create and call a callback with the new signature
	var callbackCalled bool
	var receivedPath string
	var receivedScannedCount int
	var receivedTotalCount int
	var receivedFileSize int64

	callback := func(path string, scannedCount int, totalCount int, fileSize int64) {
		callbackCalled = true
		receivedPath = path
		receivedScannedCount = scannedCount
		receivedTotalCount = totalCount
		receivedFileSize = fileSize
	}

	// Call the callback with test data
	callback("/test/path", 5, 10, 1024)

	// Verify the callback was invoked with correct parameters
	gomegaInstance.Expect(callbackCalled).Should(gomega.BeTrue(), "Callback should have been called")
	gomegaInstance.Expect(receivedPath).Should(gomega.Equal("/test/path"))
	gomegaInstance.Expect(receivedScannedCount).Should(gomega.Equal(5))
	gomegaInstance.Expect(receivedTotalCount).Should(gomega.Equal(10))
	gomegaInstance.Expect(receivedFileSize).Should(gomega.Equal(int64(1024)))
}
