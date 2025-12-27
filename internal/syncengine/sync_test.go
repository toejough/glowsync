package syncengine_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/pkg/fileops"
	"github.com/joe/copy-files/pkg/filesystem"
)

//go:generate impgen syncengine.TimeProvider
//go:generate impgen filesystem.FileSystem
//go:generate impgen filesystem.FileScanner

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

	fsImp := NewFileSystemImp(t)
	scannerImp := NewFileScannerImp(t)

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
		destScannerImp := NewFileScannerImp(t)
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

func TestEngineDeviousContentMode(t *testing.T) {
	t.Parallel()

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a test file in source
	testFile := filepath.Join(sourceDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a file with same size/modtime but different content in dest
	destFile := filepath.Join(destDir, "test.txt")
	err = os.WriteFile(destFile, []byte("diff content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create dest file: %v", err)
	}

	// Get source file info to copy modtime
	srcInfo, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat source file: %v", err)
	}

	// Set dest file to have same modtime as source
	err = os.Chtimes(destFile, srcInfo.ModTime(), srcInfo.ModTime())
	if err != nil {
		t.Fatalf("Failed to set dest file modtime: %v", err)
	}

	// Create engine with DeviousContent mode
	engine := syncengine.NewEngine(sourceDir, destDir)
	engine.ChangeType = config.DeviousContent
	engine.FileOps = fileops.NewRealFileOps()

	// Run Analyze - should detect hash mismatch
	err = engine.Analyze()

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(int(engine.Status.TotalFiles)).Should(Equal(1), "Should detect 1 file needs sync due to hash mismatch")

	// Run Sync
	err = engine.Sync()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify file was synced with correct content
	content, err := os.ReadFile(destFile)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(string(content)).Should(Equal("test content"))
}

func TestEngineParanoidMode(t *testing.T) {
	t.Parallel()

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a test file in source
	testFile := filepath.Join(sourceDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a file with same size/modtime but different content in dest
	destFile := filepath.Join(destDir, "test.txt")
	err = os.WriteFile(destFile, []byte("diff content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create dest file: %v", err)
	}

	// Get source file info to copy modtime
	srcInfo, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat source file: %v", err)
	}

	// Set dest file to have same modtime as source
	err = os.Chtimes(destFile, srcInfo.ModTime(), srcInfo.ModTime())
	if err != nil {
		t.Fatalf("Failed to set dest file modtime: %v", err)
	}

	// Create engine with Paranoid mode
	engine := syncengine.NewEngine(sourceDir, destDir)
	engine.ChangeType = config.Paranoid
	engine.FileOps = fileops.NewRealFileOps()

	// Run Analyze - should detect byte-by-byte mismatch
	err = engine.Analyze()

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(int(engine.Status.TotalFiles)).Should(Equal(1), "Should detect 1 file needs sync due to byte mismatch")

	// Run Sync
	err = engine.Sync()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify file was synced with correct content
	content, err := os.ReadFile(destFile)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(string(content)).Should(Equal("test content"))
}

func TestEngineDeleteOrphanedFiles(t *testing.T) {
	t.Parallel()

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a test file in source
	testFile := filepath.Join(sourceDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Copy test file to dest first
	destTestFile := filepath.Join(destDir, "test.txt")
	err = os.WriteFile(destTestFile, []byte("test content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create dest test file: %v", err)
	}

	// Create an orphaned file in dest (not in source)
	orphanFile := filepath.Join(destDir, "orphan.txt")
	err = os.WriteFile(orphanFile, []byte("orphan content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create orphan file: %v", err)
	}

	// Create engine
	engine := syncengine.NewEngine(sourceDir, destDir)
	engine.FileOps = fileops.NewRealFileOps()

	// Run Analyze - this will delete orphaned files
	err = engine.Analyze()

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify orphan file was deleted
	_, err = os.Stat(orphanFile)
	g.Expect(os.IsNotExist(err)).Should(BeTrue(), "Orphaned file should be deleted during Analyze")
}

func TestEngineDeleteOrphanedDirectories(t *testing.T) {
	t.Parallel()

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a test file in source
	testFile := filepath.Join(sourceDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create an orphaned directory in dest (not in source)
	orphanDir := filepath.Join(destDir, "orphan_dir")
	err = os.MkdirAll(orphanDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create orphan directory: %v", err)
	}

	// Create a file inside the orphaned directory
	orphanFile := filepath.Join(orphanDir, "file.txt")
	err = os.WriteFile(orphanFile, []byte("orphan content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create file in orphan directory: %v", err)
	}

	// Create engine
	engine := syncengine.NewEngine(sourceDir, destDir)
	engine.FileOps = fileops.NewRealFileOps()

	// Run Analyze - this will delete orphaned directories
	err = engine.Analyze()

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify orphan directory was deleted
	_, err = os.Stat(orphanDir)
	g.Expect(os.IsNotExist(err)).Should(BeTrue(), "Orphaned directory should be deleted during Analyze")
}

func TestEngineAdaptiveScaling(t *testing.T) {
	// Don't run in parallel - we need to control timing
	// t.Parallel()

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create many test files with very large content to slow down copying
	// This gives the adaptive scaling ticker time to evaluate
	largeContent := make([]byte, 1024*1024*10) // 10 MB per file
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	numFiles := 50
	for i := 0; i < numFiles; i++ {
		testFile := filepath.Join(sourceDir, filepath.FromSlash(fmt.Sprintf("file%03d.txt", i)))
		err := os.WriteFile(testFile, largeContent, 0o600)
		if err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}
	}

	// Create engine with adaptive mode enabled
	engine := syncengine.NewEngine(sourceDir, destDir)
	engine.AdaptiveMode = true
	engine.Workers = 0 // 0 means adaptive (starts with 1 worker)
	engine.FileOps = fileops.NewRealFileOps()

	// Enable file logging to see adaptive scaling decisions
	logFile := filepath.Join(t.TempDir(), "adaptive.log")
	err := engine.EnableFileLogging(logFile)
	if err != nil {
		t.Logf("Warning: Failed to enable file logging: %v", err)
	}
	defer engine.CloseLog()

	// Run Analyze
	err = engine.Analyze()

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(int(engine.Status.TotalFiles)).Should(Equal(numFiles), fmt.Sprintf("Should detect %d files need sync", numFiles))

	// Run Sync with adaptive mode
	// With 50 files of 10MB each and starting with 1 worker,
	// the adaptive scaling should trigger after processing 5 files (targetFilesPerWorker)
	err = engine.Sync()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify all files were copied
	for i := 0; i < numFiles; i++ {
		destFile := filepath.Join(destDir, filepath.FromSlash(fmt.Sprintf("file%03d.txt", i)))
		_, err := os.Stat(destFile)
		g.Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("File %d should exist", i))
	}

	// Note: Adaptive scaling may or may not trigger depending on timing and system performance,
	// but the test verifies that adaptive mode works correctly
}

func TestHandleCopyError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a test file in source
	testFile := filepath.Join(sourceDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create engine with real FileOps for Analyze
	engine := syncengine.NewEngine(sourceDir, destDir)
	engine.FileOps = fileops.NewRealFileOps()

	// Run Analyze to populate FilesToSync
	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(int(engine.Status.TotalFiles)).Should(Equal(1))

	// Now replace FileOps with a mock that returns an error
	fsImp := NewFileSystemImp(t)
	mockFileOps := fileops.NewFileOps(fsImp.Mock)
	engine.FileOps = mockFileOps

	// Set up the mock to return an error when opening the source file
	// Run this in a goroutine so it doesn't block
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Expect Open call for the source file with a timeout
		fsImp.Within(5 * time.Second).ExpectCallIs.Open().ExpectArgsAre(testFile).InjectResults(nil, fmt.Errorf("mock error: permission denied"))
	}()

	// Give the goroutine a moment to set up the expectation
	time.Sleep(100 * time.Millisecond)

	// Run Sync - this should fail
	err = engine.Sync()

	// Wait for the mock expectation to complete
	<-done

	g.Expect(err).Should(HaveOccurred(), "Sync should fail due to mock error")
	g.Expect(int(engine.Status.FailedFiles)).Should(Equal(1), "Should have 1 failed file")
	g.Expect(engine.Status.Errors).Should(HaveLen(1), "Should have 1 error")
	g.Expect(engine.Status.Errors[0].Error.Error()).Should(ContainSubstring("mock error"))
}

func TestAdaptiveScalingWithMockedTime(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create multiple large test files to trigger adaptive scaling
	// Use larger files so copying takes longer
	largeContent := make([]byte, 1024*1024) // 1MB per file
	for i := range 20 {
		testFile := filepath.Join(sourceDir, fmt.Sprintf("file%d.txt", i))
		err := os.WriteFile(testFile, largeContent, 0o600)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Create engine with adaptive mode
	engine := syncengine.NewEngine(sourceDir, destDir)
	engine.Workers = 0 // 0 means adaptive mode
	engine.AdaptiveMode = true
	engine.FileOps = fileops.NewRealFileOps()

	// Create mocked TimeProvider
	timeImp := NewTimeProviderImp(t)
	engine.TimeProvider = timeImp.Mock

	// Run Analyze
	err := engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(int(engine.Status.TotalFiles)).Should(Equal(20))

	// Set up the mock time provider
	currentTime := time.Now()
	tickerChan := make(chan time.Time, 100)

	// Start a goroutine to handle mock expectations
	done := make(chan struct{})
	go func() {
		defer close(done)

		// Expect NewTicker call
		call := timeImp.Within(5 * time.Second).ExpectCallIs.NewTicker()
		call.ExpectArgsAre(1 * time.Second)

		// Create a mock ticker
		mockTicker := &syncengine.MockTicker{
			TickChan: tickerChan,
		}
		call.InjectResult(mockTicker)

		// Send ticks every 100ms to trigger evaluateAndScale
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for range 50 {
			select {
			case <-ticker.C:
				// Expect Now() call
				nowCall := timeImp.Within(1 * time.Second).ExpectCallIs.Now()
				nowCall.InjectResult(currentTime)
				currentTime = currentTime.Add(1 * time.Second)

				// Send a tick to the mock ticker
				select {
				case tickerChan <- currentTime:
				default:
				}
			case <-done:
				return
			}
		}
	}()

	// Run Sync
	err = engine.Sync()

	// Signal the goroutine to stop
	close(done)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(int(engine.Status.ProcessedFiles)).Should(Equal(20))
}

func TestEvaluateAndScaleDirectly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create engine
	engine := syncengine.NewEngine(sourceDir, destDir)

	// Create mocked TimeProvider
	timeImp := NewTimeProviderImp(t)
	engine.TimeProvider = timeImp.Mock

	// Create a worker control channel
	workerControl := make(chan bool, 10)

	// Test 1: First call (baseline establishment)
	state := &syncengine.AdaptiveScalingState{}
	currentTime := time.Now()

	// Set up goroutine to handle Now() calls
	done := make(chan struct{})
	go func() {
		defer close(done)

		// Expect Now() call for baseline
		nowCall1 := timeImp.Within(2 * time.Second).ExpectCallIs.Now()
		nowCall1.InjectResult(currentTime)

		// Expect Now() call for second evaluation
		currentTime2 := currentTime.Add(2 * time.Second)
		nowCall2 := timeImp.Within(2 * time.Second).ExpectCallIs.Now()
		nowCall2.InjectResult(currentTime2)
	}()

	// Call evaluateAndScale for the first time (baseline)
	engine.EvaluateAndScale(state, 5, 1, 1024*1024, 10, workerControl)

	// Verify that a worker was added (baseline adds a worker)
	select {
	case addWorker := <-workerControl:
		g.Expect(addWorker).Should(BeTrue())
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected worker to be added for baseline")
	}

	// Test 2: Second call (with speed calculation)
	// Call evaluateAndScale again with more files processed
	engine.EvaluateAndScale(state, 10, 2, 2*1024*1024, 10, workerControl)

	// Verify that makeScalingDecision was called (it should add a worker since lastPerWorkerSpeed == 0)
	select {
	case addWorker := <-workerControl:
		g.Expect(addWorker).Should(BeTrue())
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected worker to be added after first measurement")
	}

	// Wait for the mock expectations to complete
	<-done
}

// expectWorkerAdded verifies that a worker was added to the control channel.
func expectWorkerAdded(t *testing.T, g *WithT, workerControl <-chan bool, testName string) {
	t.Helper()
	select {
	case addWorker := <-workerControl:
		g.Expect(addWorker).Should(BeTrue())
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Expected worker to be added for %s", testName)
	}
}

// expectNoWorkerAdded verifies that no worker was added to the control channel.
func expectNoWorkerAdded(t *testing.T, workerControl <-chan bool, testName string) {
	t.Helper()
	select {
	case <-workerControl:
		t.Fatalf("Expected no worker to be added for %s", testName)
	case <-time.After(100 * time.Millisecond):
		// Good - no worker added
	}
}

func TestMakeScalingDecisionDirectly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create engine
	engine := syncengine.NewEngine(sourceDir, destDir)

	// Create a worker control channel
	workerControl := make(chan bool, 10)

	// Test 1: First measurement (lastPerWorkerSpeed == 0)
	engine.MakeScalingDecision(0, 1024*1024, 1, 10, workerControl)
	expectWorkerAdded(t, g, workerControl, "first measurement")

	// Test 2: Speed decreased (speedRatio < 0.98)
	engine.MakeScalingDecision(1024*1024, 900*1024, 2, 10, workerControl)
	expectNoWorkerAdded(t, workerControl, "speed decreased")

	// Test 3: Speed improved (speedRatio >= 1.02)
	engine.MakeScalingDecision(1024*1024, 1100*1024, 2, 10, workerControl)
	expectWorkerAdded(t, g, workerControl, "speed improved")

	// Test 4: Speed stable (0.98 <= speedRatio < 1.02)
	engine.MakeScalingDecision(1024*1024, 1010*1024, 3, 10, workerControl)
	expectWorkerAdded(t, g, workerControl, "speed stable")

	// Test 5: At max workers
	engine.MakeScalingDecision(1024*1024, 1100*1024, 10, 10, workerControl)
	expectNoWorkerAdded(t, workerControl, "at max workers")
}

func TestRealTimeProvider(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	provider := &syncengine.RealTimeProvider{}

	// Test Now()
	before := time.Now()
	now := provider.Now()
	after := time.Now()

	g.Expect(now).Should(BeTemporally(">=", before))
	g.Expect(now).Should(BeTemporally("<=", after))

	// Test NewTicker()
	ticker := provider.NewTicker(100 * time.Millisecond)
	g.Expect(ticker).ShouldNot(BeNil())

	// Verify ticker works
	select {
	case <-ticker.C():
		// Good - ticker fired
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Ticker did not fire within expected time")
	}

	ticker.Stop()
}

func TestMockTicker(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tickChan := make(chan time.Time, 1)
	mockTicker := &syncengine.MockTicker{
		TickChan: tickChan,
	}

	// Test C()
	c := mockTicker.C()
	g.Expect(c).ShouldNot(BeNil())

	// Test that we can send and receive on the channel
	testTime := time.Now()
	tickChan <- testTime

	select {
	case receivedTime := <-c:
		g.Expect(receivedTime).Should(Equal(testTime))
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Did not receive time from ticker channel")
	}

	// Test Stop()
	mockTicker.Stop()

	// Verify channel is closed
	_, ok := <-c
	g.Expect(ok).Should(BeFalse(), "Channel should be closed after Stop()")
}
