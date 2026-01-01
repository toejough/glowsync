//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package fileops

import (
	"os"
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers
)

// TestBufferSize_UsedInCopyLoop verifies that the BufferSize constant matches expected value.
// This test ensures that when BufferSize is increased to 64KB, the copy loop uses larger buffers.
// This test will FAIL until Phase 1.1 increases BufferSize to 64KB.
func TestBufferSize_UsedInCopyLoop(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Verify BufferSize is 64KB
	expectedBufferSize := 64 * 1024
	g.Expect(BufferSize).Should(Equal(expectedBufferSize),
		"BufferSize should be 64KB for improved copy performance")

	// The actual buffer allocation in osCopyLoopWithStats (line 448) and
	// osSimpleCopyLoop (line 505) uses: buf := make([]byte, BufferSize)
	// We can't directly test the buffer size without modifying the function,
	// but we verify the constant is correct
}

// TestCompareOSFileContents_UsesCorrectBufferSize verifies buffer allocation in comparison.
// This test will FAIL until Phase 1.1 increases BufferSize to 64KB.
func TestCompareOSFileContents_UsesCorrectBufferSize(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Verify BufferSize is 64KB
	expectedBufferSize := 64 * 1024
	g.Expect(BufferSize).Should(Equal(expectedBufferSize),
		"BufferSize should be 64KB for compareOSFileContents (line 408-409)")

	// The compareOSFileContents function (lines 407-440) allocates:
	// buf1 := make([]byte, BufferSize)
	// buf2 := make([]byte, BufferSize)
	// This test verifies the constant used for these allocations is 64KB
}

// TestOsCopyLoopWithStats tests the internal osCopyLoopWithStats helper function.
func TestOsCopyLoopWithStats(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp files
	tmpDir := t.TempDir()
	srcPath := tmpDir + "/source.txt"
	dstPath := tmpDir + "/dest.txt"

	content := []byte("test content for copy loop")

	err := os.WriteFile(srcPath, content, 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Open source file
	sourceFile, err := os.Open(srcPath)
	g.Expect(err).ShouldNot(HaveOccurred())

	defer func() { _ = sourceFile.Close() }()

	// Create destination file
	destFile, err := os.Create(dstPath)
	g.Expect(err).ShouldNot(HaveOccurred())

	defer func() { _ = destFile.Close() }()

	// Create stats
	stats := &CopyStats{}

	// Call the helper function
	written, err := osCopyLoopWithStats(sourceFile, destFile, stats, int64(len(content)), srcPath, nil, nil)

	// Verify results
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(written).Should(Equal(int64(len(content))))
	g.Expect(stats.ReadTime).Should(Not(BeZero()))
	g.Expect(stats.WriteTime).Should(Not(BeZero()))

	// Verify file was copied correctly
	err = destFile.Close()
	g.Expect(err).ShouldNot(HaveOccurred())

	copiedContent, err := os.ReadFile(dstPath)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(copiedContent).Should(Equal(content))
}

// TestOsCopyLoopWithStatsCancel tests cancellation of the copy loop.
func TestOsCopyLoopWithStatsCancel(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp files
	tmpDir := t.TempDir()
	srcPath := tmpDir + "/source.txt"
	dstPath := tmpDir + "/dest.txt"

	// Create a larger file to ensure we can cancel mid-copy
	content := make([]byte, 1024*1024) // 1MB

	err := os.WriteFile(srcPath, content, 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Open source file
	sourceFile, err := os.Open(srcPath)
	g.Expect(err).ShouldNot(HaveOccurred())

	defer func() { _ = sourceFile.Close() }()

	// Create destination file
	destFile, err := os.Create(dstPath)
	g.Expect(err).ShouldNot(HaveOccurred())

	defer func() { _ = destFile.Close() }()

	// Create stats
	stats := &CopyStats{}

	// Create cancel channel and close it immediately
	cancelChan := make(chan struct{})
	close(cancelChan)

	// Call the helper function
	_, err = osCopyLoopWithStats(sourceFile, destFile, stats, int64(len(content)), srcPath, nil, cancelChan)

	// Verify cancellation error
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err.Error()).Should(ContainSubstring("copy cancelled"))
}
