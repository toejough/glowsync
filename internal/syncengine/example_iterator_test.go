package syncengine_test

import (
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/pkg/fileops"
	"github.com/joe/copy-files/pkg/filesystem"
)

// This is a DEMONSTRATION of how the new iterator-based FileSystem interface works.
// The engine now uses Scan() instead of Walk(), which makes it easier to test with imptest.
//
// This test uses MockFileSystem (a fake) for simplicity, but the key point is that
// the engine now uses the iterator pattern, which would make FileSystemImp testing
// much easier with interleaved expectations.

func TestEngineAnalyze_WithIteratorPattern_Example(t *testing.T) {
	t.Parallel()

	// For now, use MockFileSystem to demonstrate that the iterator pattern works
	mockFS := filesystem.NewMockFileSystem()

	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	// Set up test data
	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), baseTime.Add(-1*time.Hour))
	mockFS.AddFile(filepath.Join(srcDir, "file2.txt"), []byte("content2"), baseTime.Add(-2*time.Hour))
	mockFS.AddDir(dstDir, baseTime)

	// Create engine with mock filesystem
	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify engine state
	NewWithT(t).Expect(engine.Status).Should(And(
		HaveField("TotalFiles", Equal(2)),
		HaveField("FilesToSync", HaveLen(2)),
		HaveField("TotalBytes", Equal(int64(16))),
	))
}

// TestEngineAnalyze_WithFileSystemImp demonstrates interaction-based testing with FileSystemImp.
// This is more verbose but reveals implementation details like scanning twice.
func TestEngineAnalyze_WithFileSystemImp(t *testing.T) {
	t.Parallel()

	// Create imptest mock filesystem
	fsImp := NewFileSystemImp(t)

	// Set up test data
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	// Create engine with mock filesystem
	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount
	engine.FileOps = fileops.NewFileOps(fsImp.Mock)

	// Start analyze in a goroutine using the wrapper
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start()

	// ===== SOURCE SCAN (scan #1) =====
	// After optimization, ScanDirectoryWithProgress only scans once
	srcScanner := NewFileScannerImp(t)
	fsImp.ExpectCallIs.Scan().ExpectArgsAre(srcDir).InjectResult(srcScanner.Mock)

	// Iterate through files - FileInfo is returned directly from Next()
	srcScanner.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{
		RelativePath: "file1.txt",
		Size:         8,
		ModTime:      baseTime.Add(-1 * time.Hour),
	}, true)
	srcScanner.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{
		RelativePath: "file2.txt",
		Size:         8,
		ModTime:      baseTime.Add(-2 * time.Hour),
	}, true)
	srcScanner.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{}, false)
	srcScanner.ExpectCallIs.Err().InjectResult(nil)

	// ===== DEST SCAN (scan #2) =====
	dstScanner := NewFileScannerImp(t)
	fsImp.ExpectCallIs.Scan().ExpectArgsAre(dstDir).InjectResult(dstScanner.Mock)

	// Empty directory
	dstScanner.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{}, false)
	dstScanner.ExpectCallIs.Err().InjectResult(nil)

	// Verify Analyze completed successfully
	analyzeWrapper.ExpectReturnedValuesAre(nil)

	// Verify engine status
	NewWithT(t).Expect(engine.Status).Should(And(
		HaveField("TotalFiles", Equal(2)),
		HaveField("FilesToSync", HaveLen(2)),
	))
}

// This demonstrates the key advantages of the iterator pattern with imptest:
//
// 1. **Interleaved expectations** - No goroutines needed! We can call Start(),
//    then set up expectations, then verify results. The test reads like a conversation.
//
// 2. **Clear control flow** - It's obvious what the engine is doing and when:
//    - Scan source → provide scanner
//    - Iterate files → provide file info
//    - Scan dest → provide scanner
//    - Iterate files → provide file info
//
// 3. **Easy to test edge cases** - Want to test error handling? Just inject an error:
//    srcScannerImp.ExpectCallIs.Err().InjectResults(fmt.Errorf("disk error"))
//
// 4. **No implementation coupling** - We don't care HOW the engine scans files,
//    we just verify WHAT it asks for and WHAT it does with the responses.
//
// 5. **Type-safe** - imptest generates type-safe mocks, so we can't accidentally
//    inject the wrong type or forget to handle a call.
//
// Compare this to the MockFileSystem approach in TestEngineAnalyze:
// - MockFileSystem: Set up state, run code, verify state (state-based testing)
// - FileSystemImp: Interleave expectations and execution (interaction-based testing)
//
// Both approaches have their place:
// - Use MockFileSystem for simple integration tests where you care about final state
// - Use FileSystemImp for detailed unit tests where you care about interactions
//
// The iterator pattern makes FileSystemImp much easier to use than the callback-based
// Walk pattern would have been.

