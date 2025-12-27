package syncengine_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/pkg/fileops"
	"github.com/joe/copy-files/pkg/filesystem"
)

func TestNewEngine(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")

	if engine == nil {
		t.Error("NewEngine should return non-nil engine")
	}
}

func TestEngineCancel(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")

	// Use imptest wrapper for Cancel method
	cancelWrapper := syncengine.NewEngineCancel(t, engine.Cancel)
	cancelWrapper.Start()
	cancelWrapper.ExpectReturnedValuesAre() // Cancel returns nothing

	// Cancel should be idempotent
	cancelWrapper2 := syncengine.NewEngineCancel(t, engine.Cancel)
	cancelWrapper2.Start()
	cancelWrapper2.ExpectReturnedValuesAre()
}

func TestEngineEnableFileLogging(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")

	// Use imptest wrapper for EnableFileLogging method
	wrapper := syncengine.NewEngineEnableFileLogging(t, engine.EnableFileLogging)

	// Test with invalid path (should return error)
	wrapper.Start("/invalid/path/that/cannot/be/created/log.txt")
	wrapper.ExpectReturnedValuesShould(Not(BeNil()))
}

func TestEngineCloseLog(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")

	// Use imptest wrapper for CloseLog method
	wrapper := syncengine.NewEngineCloseLog(t, engine.CloseLog)

	// CloseLog should be safe to call even if no log is open
	wrapper.Start()
	wrapper.ExpectReturnedValuesAre() // CloseLog returns nothing

	// Should be idempotent
	wrapper2 := syncengine.NewEngineCloseLog(t, engine.CloseLog)
	wrapper2.Start()
	wrapper2.ExpectReturnedValuesAre()
}

func TestEngineRegisterStatusCallback(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")

	callbackCalled := false
	callback := func(status *syncengine.Status) {
		callbackCalled = true
	}

	engine.RegisterStatusCallback(callback)

	// We can't easily test that the callback is called without running Analyze/Sync,
	// but we can verify that RegisterStatusCallback doesn't panic
	if callbackCalled {
		t.Error("Callback should not be called immediately")
	}
}

func TestEngineGetStatus(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")

	status := engine.GetStatus()

	if status == nil {
		t.Error("GetStatus should return non-nil status")
	}
}

func TestFormatBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero bytes", 0, "0 B"},
		{"small bytes", 512, "512 B"},
		{"1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"1 MB", 1024 * 1024, "1.0 MB"},
		{"1 GB", 1024 * 1024 * 1024, "1.0 GB"},
		{"1.5 GB", 1536 * 1024 * 1024, "1.5 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wrapper := syncengine.NewFormatBytesImp(t, syncengine.FormatBytes)
			wrapper.Start(tt.bytes)
			wrapper.ExpectReturnedValuesAre(tt.expected)
		})
	}
}

func TestEngineAnalyze(t *testing.T) {
	t.Parallel()

	fsImp := filesystem.NewFileSystemImp(t)
	scannerImp := filesystem.NewFileScannerImp(t)

	engine := syncengine.NewEngine("/source", "/dest")
	engine.FileOps = fileops.NewFileOps(fsImp.Mock)

	// Set up expectations in a goroutine
	go func() {
		// Expect Scan call for source directory
		fsImp.ExpectCallIs.Scan().ExpectArgsAre("/source").InjectResult(scannerImp.Mock)

		// Return empty directory (no files)
		scannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{}, false)
		scannerImp.ExpectCallIs.Err().InjectResult(nil)

		// Expect Scan call for dest directory
		destScannerImp := filesystem.NewFileScannerImp(t)
		fsImp.ExpectCallIs.Scan().ExpectArgsAre("/dest").InjectResult(destScannerImp.Mock)

		// Return empty directory (no files)
		destScannerImp.ExpectCallIs.Next().InjectResults(filesystem.FileInfo{}, false)
		destScannerImp.ExpectCallIs.Err().InjectResult(nil)
	}()

	// Call Analyze
	err := engine.Analyze()

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestEngineSync(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")

	// Call Sync with no files to sync (should return immediately)
	err := engine.Sync()

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestEngineSyncAdaptive(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")
	engine.AdaptiveMode = true

	// Call Sync with no files to sync (should return immediately)
	err := engine.Sync()

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestEngineSyncWithFile(t *testing.T) {
	t.Parallel()

	// This test is more complex - it needs to mock the entire file copy operation
	// For now, let's just verify that Sync can handle a file in the queue
	// We'll use a real temporary directory to avoid complex mocking

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a test file in source
	testFile := filepath.Join(sourceDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create engine
	engine := syncengine.NewEngine(sourceDir, destDir)

	// Run Analyze to populate FilesToSync
	err = engine.Analyze()
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Run Sync
	err = engine.Sync()

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify file was copied
	destFile := filepath.Join(destDir, "test.txt")
	_, err = os.Stat(destFile)
	g.Expect(err).ShouldNot(HaveOccurred())
}
