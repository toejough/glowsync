//nolint:varnamelen // Test files use idiomatic short variable names (t, tt, g, c, etc.)
package syncengine_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/pkg/fileops"
	"github.com/joe/copy-files/pkg/filesystem"
)

// mustNewEngine creates a new engine and fails the test if there's an error
func mustNewEngine(t *testing.T, source, dest string) *syncengine.Engine {
	t.Helper()
	engine, err := syncengine.NewEngine(source, dest)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	return engine
}

func TestAdaptiveScalingWithMockedTime(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create multiple large test files to trigger adaptive scaling
	createLargeTestFiles(t, sourceDir, 20)

	// Create mocked TimeProvider
	timeImp := NewTimeProviderImp(t)

	// Create engine with adaptive mode
	engine := setupAdaptiveEngine(sourceDir, destDir, timeImp)

	// Set up mock expectations for Analyze phase
	// The Analyze phase calls Now() during scanSourceDirectory and scanDestDirectory
	analyzeStartTime := time.Now()
	go func() {
		// Expect first Now() call (AnalysisStartTime initialization)
		nowCall1 := timeImp.Within(5 * time.Second).ExpectCallIs.Now()
		nowCall1.InjectResult(analyzeStartTime)

		// Expect subsequent Now() calls during progress callbacks (up to 40 total - 20 files x 2 scans)
		for range 40 {
			nowCall := timeImp.Within(5 * time.Second).ExpectCallIs.Now()
			nowCall.InjectResult(analyzeStartTime.Add(100 * time.Millisecond)) // Simulate some elapsed time
		}
	}()

	// Run Analyze
	err := engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(engine.Status.TotalFiles).Should(Equal(20))

	// Set up the mock time provider
	tickerChan := make(chan time.Time, 100)
	done := make(chan struct{})

	// Start a goroutine to handle mock expectations
	runMockTimeProvider(timeImp, tickerChan, done)

	// Run Sync
	err = engine.Sync()

	// Signal the goroutine to stop
	close(done)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(engine.Status.ProcessedFiles).Should(Equal(20))
}

//nolint:lll // Function signature with inline nolint comment
func TestEngineAdaptiveScaling(t *testing.T) { //nolint:paralleltest // Don't run in parallel - we need to control timing
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
	for i := range numFiles {
		testFile := filepath.Join(sourceDir, filepath.FromSlash(fmt.Sprintf("file%03d.txt", i)))

		err := os.WriteFile(testFile, largeContent, 0o600)
		if err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}
	}

	// Create engine with adaptive mode enabled
	engine := mustNewEngine(t, sourceDir, destDir)
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
	g.Expect(engine.Status.TotalFiles).Should(Equal(numFiles), fmt.Sprintf("Should detect %d files need sync", numFiles))

	// Run Sync with adaptive mode
	// With 50 files of 10MB each and starting with 1 worker,
	// the adaptive scaling should trigger after processing 5 files (targetFilesPerWorker)
	err = engine.Sync()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify all files were copied
	for i := range numFiles {
		destFile := filepath.Join(destDir, filepath.FromSlash(fmt.Sprintf("file%03d.txt", i)))
		_, err := os.Stat(destFile)
		g.Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("File %d should exist", i))
	}

	// Note: Adaptive scaling may or may not trigger depending on timing and system performance,
	// but the test verifies that adaptive mode works correctly
}

func TestEngineAnalyze(t *testing.T) {
	t.Parallel()

	fsImp := NewFileSystemImp(t)
	scannerImp := NewFileScannerImp(t)

	engine := mustNewEngine(t, "/source", "/dest")
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

func TestEngineCancel(t *testing.T) {
	t.Parallel()

	engine := mustNewEngine(t, "/source", "/dest")

	// Use imptest wrapper for Cancel method
	cancelWrapper := syncengine.NewEngineCancel(t, engine.Cancel)
	cancelWrapper.Start()
	cancelWrapper.ExpectReturnedValuesAre() // Cancel returns nothing

	// Cancel should be idempotent
	cancelWrapper2 := syncengine.NewEngineCancel(t, engine.Cancel)
	cancelWrapper2.Start()
	cancelWrapper2.ExpectReturnedValuesAre()
}

func TestEngineCloseLog(t *testing.T) {
	t.Parallel()

	engine := mustNewEngine(t, "/source", "/dest")

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
	engine := mustNewEngine(t, sourceDir, destDir)
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
	engine := mustNewEngine(t, sourceDir, destDir)
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

func TestEngineDeviousContentMode(t *testing.T) {
	t.Parallel()

	sourceDir, destDir, destFile := setupSameSizeModtimeTest(t)

	// Create engine with DeviousContent mode
	engine := mustNewEngine(t, sourceDir, destDir)
	engine.ChangeType = config.DeviousContent
	engine.FileOps = fileops.NewRealFileOps()

	// Run Analyze - should detect hash mismatch
	err := engine.Analyze()

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(engine.Status.TotalFiles).Should(Equal(1), "Should detect 1 file needs sync due to hash mismatch")

	// Run Sync
	err = engine.Sync()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify file was synced with correct content
	content, err := os.ReadFile(destFile)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(string(content)).Should(Equal("test content"))
}

func TestEngineEnableFileLogging(t *testing.T) {
	t.Parallel()

	engine := mustNewEngine(t, "/source", "/dest")

	// Use imptest wrapper for EnableFileLogging method
	wrapper := syncengine.NewEngineEnableFileLogging(t, engine.EnableFileLogging)

	// Test with invalid path (should return error)
	wrapper.Start("/invalid/path/that/cannot/be/created/log.txt")
	wrapper.ExpectReturnedValuesShould(Not(BeNil()))
}

//nolint:gocognit,funlen,cyclop,noinlineerr // Integration test with comprehensive scenarios
func TestEngineFilePatternFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		pattern         string
		createFiles     []string
		expectedMatches []string
	}{
		{
			name:    "empty pattern matches all",
			pattern: "",
			createFiles: []string{
				"video.mov",
				"photo.jpg",
				"document.pdf",
			},
			expectedMatches: []string{
				"video.mov",
				"photo.jpg",
				"document.pdf",
			},
		},
		{
			name:    "simple extension filter",
			pattern: "*.mov",
			createFiles: []string{
				"video1.mov",
				"video2.mov",
				"photo.jpg",
				"document.pdf",
			},
			expectedMatches: []string{
				"video1.mov",
				"video2.mov",
			},
		},
		{
			name:    "double star nested files",
			pattern: "**/*.mov",
			createFiles: []string{
				"video.mov",
				"videos/clip.mov",
				"videos/2023/vacation.mov",
				"photos/pic.jpg",
			},
			expectedMatches: []string{
				"video.mov",
				"videos/clip.mov",
				"videos/2023/vacation.mov",
			},
		},
		{
			name:    "brace expansion multiple extensions",
			pattern: "*.{mov,mp4}",
			createFiles: []string{
				"video1.mov",
				"video2.mp4",
				"video3.avi",
				"photo.jpg",
			},
			expectedMatches: []string{
				"video1.mov",
				"video2.mp4",
			},
		},
		{
			name:    "specific directory pattern",
			pattern: "videos/*.mov",
			createFiles: []string{
				"root.mov",
				"videos/clip1.mov",
				"videos/clip2.mov",
				"photos/vid.mov",
			},
			expectedMatches: []string{
				"videos/clip1.mov",
				"videos/clip2.mov",
			},
		},
		{
			name:    "case insensitive matching",
			pattern: "*.MOV",
			createFiles: []string{
				"video1.mov",
				"video2.MOV",
				"video3.MoV",
				"photo.jpg",
			},
			expectedMatches: []string{
				"video1.mov",
				"video2.MOV",
				"video3.MoV",
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Create temporary directories
			sourceDir := t.TempDir()
			destDir := t.TempDir()

			// Create test files in source
			for _, filePath := range testCase.createFiles {
				fullPath := filepath.Join(sourceDir, filePath)

				// Create parent directory if needed
				parentDir := filepath.Dir(fullPath)

				err := os.MkdirAll(parentDir, 0o755)
				if err != nil {
					t.Fatalf("Failed to create directory %s: %v", parentDir, err)
				}

				// Create file with some content
				err = os.WriteFile(fullPath, []byte("test content"), 0o600)
				if err != nil {
					t.Fatalf("Failed to create file %s: %v", fullPath, err)
				}
			}

			// Create engine with file pattern
			engine := mustNewEngine(t, sourceDir, destDir)
			engine.FilePattern = testCase.pattern

			// Run analysis
			err := engine.Analyze()
			if err != nil {
				t.Fatalf("Analysis failed: %v", err)
			}

			// Get the status to see what files were found
			status := engine.GetStatus()

			// Check that the correct number of files were matched
			if status.TotalFiles != len(testCase.expectedMatches) {
				t.Errorf("Expected %d files to match pattern '%s', got %d",
					len(testCase.expectedMatches),
					testCase.pattern,
					status.TotalFiles,
				)
			}

			// Sync the files
			err = engine.Sync()
			if err != nil {
				t.Fatalf("Sync failed: %v", err)
			}

			// Verify only the expected files were synced to destination
			for _, expectedFile := range testCase.expectedMatches {
				destPath := filepath.Join(destDir, expectedFile)
				if _, err := os.Stat(destPath); os.IsNotExist(err) {
					t.Errorf("Expected file %s to be synced but it doesn't exist in destination", expectedFile)
				}
			}

			// Verify files that shouldn't match were NOT synced
			for _, createdFile := range testCase.createFiles {
				isExpected := slices.Contains(testCase.expectedMatches, createdFile)

				if !isExpected {
					destPath := filepath.Join(destDir, createdFile)
					if _, err := os.Stat(destPath); !os.IsNotExist(err) {
						t.Errorf("File %s should NOT have been synced but it exists in destination", createdFile)
					}
				}
			}
		})
	}
}

func TestEngineGetStatus(t *testing.T) {
	t.Parallel()

	engine := mustNewEngine(t, "/source", "/dest")

	status := engine.GetStatus()

	if status == nil {
		t.Error("GetStatus should return non-nil status")
	}
}

func TestEngineMarkFileCompleteWithoutCopy(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test markFileCompleteWithoutCopy indirectly through hash optimization
	// which is the code path that calls this function
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create identical files in source and dest
	testContent := []byte("test content for hash optimization")
	srcFile := filepath.Join(sourceDir, "test.txt")
	dstFile := filepath.Join(destDir, "test.txt")

	err := os.WriteFile(srcFile, testContent, 0o600)
	g.Expect(err).ShouldNot(HaveOccurred())

	err = os.WriteFile(dstFile, testContent, 0o600)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create engine with Content mode (enables hash optimization)
	engine := mustNewEngine(t, sourceDir, destDir)
	engine.ChangeType = config.Content
	engine.FileOps = fileops.NewRealFileOps()

	// Register callback to verify markFileCompleteWithoutCopy calls it
	callbackCalled := false

	engine.RegisterStatusCallback(func(_ *syncengine.Status) {
		callbackCalled = true
	})

	// Set older modtime on dest to trigger sync check
	srcInfo, err := os.Stat(srcFile)
	g.Expect(err).ShouldNot(HaveOccurred())

	oldTime := srcInfo.ModTime().Add(-1 * time.Hour)

	err = os.Chtimes(dstFile, oldTime, oldTime)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Run Analyze - should detect file needs sync due to modtime
	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(engine.Status.TotalFiles).Should(Equal(1))

	// Capture initial state
	initialProcessedFiles := engine.Status.ProcessedFiles
	initialTransferredBytes := engine.Status.TransferredBytes

	// Run Sync - hash optimization should kick in
	err = engine.Sync()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify markFileCompleteWithoutCopy was called (indirectly)
	// by checking that:
	// 1. ProcessedFiles was incremented
	g.Expect(engine.Status.ProcessedFiles).Should(Equal(initialProcessedFiles + 1))

	// 2. TransferredBytes was updated (file size added)
	g.Expect(engine.Status.TransferredBytes).Should(BeNumerically(">", initialTransferredBytes))

	// 3. Status callback was called
	g.Expect(callbackCalled).Should(BeTrue())

	// 4. File modtime was updated to match source
	dstInfo, err := os.Stat(dstFile)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(dstInfo.ModTime().Unix()).Should(Equal(srcInfo.ModTime().Unix()))
}

func TestEngineParanoidMode(t *testing.T) {
	t.Parallel()

	sourceDir, destDir, destFile := setupSameSizeModtimeTest(t)

	// Create engine with Paranoid mode
	engine := mustNewEngine(t, sourceDir, destDir)
	engine.ChangeType = config.Paranoid
	engine.FileOps = fileops.NewRealFileOps()

	// Run Analyze - should detect byte-by-byte mismatch
	err := engine.Analyze()

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(engine.Status.TotalFiles).Should(Equal(1), "Should detect 1 file needs sync due to byte mismatch")

	// Run Sync
	err = engine.Sync()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify file was synced with correct content
	content, err := os.ReadFile(destFile)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(string(content)).Should(Equal("test content"))
}

func TestEngineRegisterStatusCallback(t *testing.T) {
	t.Parallel()

	engine := mustNewEngine(t, "/source", "/dest")

	callbackCalled := false
	callback := func(_ *syncengine.Status) {
		callbackCalled = true
	}

	engine.RegisterStatusCallback(callback)

	// We can't easily test that the callback is called without running Analyze/Sync,
	// but we can verify that RegisterStatusCallback doesn't panic
	if callbackCalled {
		t.Error("Callback should not be called immediately")
	}
}

func TestEngineSync(t *testing.T) {
	t.Parallel()

	engine := mustNewEngine(t, "/source", "/dest")

	// Call Sync with no files to sync (should return immediately)
	err := engine.Sync()

	// Verify results
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestEngineSyncAdaptive(t *testing.T) {
	t.Parallel()

	engine := mustNewEngine(t, "/source", "/dest")
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
	engine := mustNewEngine(t, sourceDir, destDir)

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

func TestEvaluateAndScaleDirectly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create engine
	engine := mustNewEngine(t, sourceDir, destDir)

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
	engine := mustNewEngine(t, sourceDir, destDir)
	engine.FileOps = fileops.NewRealFileOps()

	// Run Analyze to populate FilesToSync
	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(engine.Status.TotalFiles).Should(Equal(1))

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
		//nolint:lll // Test mock method chain with inline error injection
		fsImp.Within(5*time.Second).ExpectCallIs.Open().ExpectArgsAre(testFile).InjectResults(nil, errors.New("mock error: permission denied"))
	}()

	// Give the goroutine a moment to set up the expectation
	time.Sleep(100 * time.Millisecond)

	// Run Sync - this should fail
	err = engine.Sync()

	// Wait for the mock expectation to complete
	<-done

	g.Expect(err).Should(HaveOccurred(), "Sync should fail due to mock error")
	g.Expect(engine.Status.FailedFiles).Should(Equal(1), "Should have 1 failed file")
	g.Expect(engine.Status.Errors).Should(HaveLen(1), "Should have 1 error")
	g.Expect(engine.Status.Errors[0].Error.Error()).Should(ContainSubstring("mock error"))
}

func TestMakeScalingDecisionDirectly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create engine
	engine := mustNewEngine(t, sourceDir, destDir)

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

//go:generate impgen syncengine.TimeProvider
//go:generate impgen filesystem.FileSystem
//go:generate impgen filesystem.FileScanner

func TestNewEngine(t *testing.T) {
	t.Parallel()

	engine := mustNewEngine(t, "/source", "/dest")

	if engine == nil {
		t.Error("NewEngine should return non-nil engine")
	}
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

// createLargeTestFiles creates multiple large test files in the source directory
func createLargeTestFiles(t *testing.T, sourceDir string, numFiles int) {
	t.Helper()

	largeContent := make([]byte, 1024*1024) // 1MB per file

	for i := range numFiles {
		testFile := filepath.Join(sourceDir, fmt.Sprintf("file%d.txt", i))

		err := os.WriteFile(testFile, largeContent, 0o600)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
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

// runMockTimeProvider starts a goroutine to handle mock time expectations
func runMockTimeProvider(timeImp *TimeProviderImp, tickerChan chan time.Time, done chan struct{}) {
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

		currentTime := time.Now()

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
}

// setupAdaptiveEngine creates and configures an engine with adaptive mode
func setupAdaptiveEngine(sourceDir, destDir string, timeImp *TimeProviderImp) *syncengine.Engine {
	engine, err := syncengine.NewEngine(sourceDir, destDir)
	if err != nil {
		panic(fmt.Sprintf("NewEngine failed: %v", err))
	}
	engine.Workers = 0 // 0 means adaptive mode
	engine.AdaptiveMode = true
	engine.FileOps = fileops.NewRealFileOps()
	engine.TimeProvider = timeImp.Mock

	return engine
}

// setupSameSizeModtimeTest creates test files with same size/modtime but different content
func setupSameSizeModtimeTest(t *testing.T) (sourceDir, destDir, destFile string) {
	t.Helper()

	// Create temp directories
	sourceDir = t.TempDir()
	destDir = t.TempDir()

	// Create a test file in source
	testFile := filepath.Join(sourceDir, "test.txt")

	err := os.WriteFile(testFile, []byte("test content"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a file with same size/modtime but different content in dest
	destFile = filepath.Join(destDir, "test.txt")

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

	return sourceDir, destDir, destFile
}

// TestStatus_AnalysisFields_Initialization verifies new analysis tracking fields initialize to zero
func TestStatus_AnalysisFields_Initialization(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		StartTime: time.Now(),
	}

	// Verify analysis tracking fields are zero-initialized
	g.Expect(status.ScannedBytes).Should(Equal(int64(0)))
	g.Expect(status.TotalBytesToScan).Should(Equal(int64(0)))
	g.Expect(status.AnalysisStartTime).Should(Equal(time.Time{}))
	g.Expect(status.AnalysisRate).Should(Equal(float64(0)))
}

// TestStatus_AnalysisFields_SetAndGet verifies fields can be set and retrieved
func TestStatus_AnalysisFields_SetAndGet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		StartTime: time.Now(),
	}

	// Set analysis tracking fields
	now := time.Now()
	status.ScannedBytes = 1024
	status.TotalBytesToScan = 4096
	status.AnalysisStartTime = now
	status.AnalysisRate = 42.5

	// Verify fields can be retrieved
	g.Expect(status.ScannedBytes).Should(Equal(int64(1024)))
	g.Expect(status.TotalBytesToScan).Should(Equal(int64(4096)))
	g.Expect(status.AnalysisStartTime).Should(Equal(now))
	g.Expect(status.AnalysisRate).Should(Equal(42.5))
}

// TestCalculateAnalysisProgress_CountingPhase verifies IsCounting=true when totals unknown
func TestCalculateAnalysisProgress_CountingPhase(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		StartTime:         time.Now(),
		ScannedFiles:      50,
		TotalFilesToScan:  0, // Unknown during counting
		ScannedBytes:      1024 * 50,
		TotalBytesToScan:  0, // Unknown during counting
		AnalysisStartTime: time.Now().Add(-5 * time.Second),
	}

	progress := status.CalculateAnalysisProgress()

	// During counting phase, IsCounting should be true
	g.Expect(progress.IsCounting).Should(BeTrue())
	g.Expect(progress.FilesPercent).Should(Equal(float64(0)))
	g.Expect(progress.BytesPercent).Should(Equal(float64(0)))
	g.Expect(progress.OverallPercent).Should(Equal(float64(0)))
}

// TestCalculateAnalysisProgress_ProcessingPhase verifies correct percentages when totals known
func TestCalculateAnalysisProgress_ProcessingPhase(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		StartTime:         time.Now(),
		ScannedFiles:      50,
		TotalFilesToScan:  100,
		ScannedBytes:      1024 * 50,
		TotalBytesToScan:  1024 * 100,
		AnalysisStartTime: time.Now().Add(-10 * time.Second),
	}

	progress := status.CalculateAnalysisProgress()

	// During processing phase, IsCounting should be false
	g.Expect(progress.IsCounting).Should(BeFalse())
	g.Expect(progress.FilesPercent).Should(Equal(50.0))
	g.Expect(progress.BytesPercent).Should(Equal(50.0))
	// Overall is average of files, bytes, and time percent
	g.Expect(progress.OverallPercent).Should(BeNumerically(">=", 0))
	g.Expect(progress.OverallPercent).Should(BeNumerically("<=", 100))
}

// TestCalculateAnalysisProgress_EdgeCases verifies handling of zero values
func TestCalculateAnalysisProgress_EdgeCases(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test zero files case
	status := &syncengine.Status{
		StartTime:         time.Now(),
		ScannedFiles:      0,
		TotalFilesToScan:  0,
		ScannedBytes:      0,
		TotalBytesToScan:  0,
		AnalysisStartTime: time.Time{}, // Not started
	}

	progress := status.CalculateAnalysisProgress()
	g.Expect(progress.IsCounting).Should(BeTrue())
	g.Expect(progress.OverallPercent).Should(Equal(float64(0)))
}

// TestCalculateAnalysisProgress_ProgressionAccuracy verifies 50/100 = 50%
func TestCalculateAnalysisProgress_ProgressionAccuracy(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		StartTime:         time.Now(),
		ScannedFiles:      50,
		TotalFilesToScan:  100,
		ScannedBytes:      2048,
		TotalBytesToScan:  4096,
		AnalysisStartTime: time.Now().Add(-10 * time.Second),
	}

	progress := status.CalculateAnalysisProgress()

	g.Expect(progress.FilesPercent).Should(Equal(50.0))
	g.Expect(progress.BytesPercent).Should(Equal(50.0))
}

func TestAdaptiveScalingDecrementsDesiredWorkers(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create engine
	engine := mustNewEngine(t, sourceDir, destDir)

	// Initialize desiredWorkers to 5
	engine.SetDesiredWorkers(5)

	// Create a worker control channel
	workerControl := make(chan bool, 10)

	// Call MakeScalingDecision with decreased per-worker speed
	engine.MakeScalingDecision(
		1000000.0, // lastPerWorkerSpeed: 1 MB/s per worker
		50000.0,   // currentPerWorkerSpeed: 0.05 MB/s per worker (much lower)
		5,         // currentWorkers
		10,        // maxWorkers
		workerControl,
	)

	// Verify that desiredWorkers was decremented
	desired := engine.GetDesiredWorkers()
	g.Expect(desired).Should(Equal(int32(4)), "desiredWorkers should decrement from 5 to 4 when speed drops")

	// Verify that NO worker was added (channel should be empty)
	select {
	case <-workerControl:
		t.Fatal("Should not add worker when per-worker speed decreased")
	case <-time.After(50 * time.Millisecond):
		// Expected - no worker added
	}
}

func TestWorkerExitsWhenOverDesiredCount(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create some test files to sync
	for i := 1; i <= 3; i++ {
		testFile := filepath.Join(sourceDir, fmt.Sprintf("test%d.txt", i))
		err := os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0o600)
		g.Expect(err).ShouldNot(HaveOccurred())
	}

	// Create engine
	engine := mustNewEngine(t, sourceDir, destDir)
	engine.AdaptiveMode = true

	// Run analysis to populate FilesToSync
	err := engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(engine.Status.TotalFiles).Should(Equal(3))

	// Set up initial state: 5 active workers, but desired is only 2
	atomic.StoreInt32(&engine.Status.ActiveWorkers, 5)
	engine.SetDesiredWorkers(2)

	// Create channels
	jobs := make(chan *syncengine.FileToSync, 10)
	errors := make(chan error, 10)
	var wg sync.WaitGroup

	// Start 5 workers
	for i := 0; i < 5; i++ {
		wg.Add(1)

		go engine.TestWorker(&wg, jobs, errors)
	}

	// Send enough jobs for all workers to process and check CAS
	for _, file := range engine.Status.FilesToSync {
		jobs <- file
	}

	// Close jobs channel
	close(jobs)

	// Wait for workers to finish
	wg.Wait()

	// Verify that activeWorkers decreased to desiredWorkers (2)
	// Three workers should have exited via CAS
	finalActive := atomic.LoadInt32(&engine.Status.ActiveWorkers)
	g.Expect(finalActive).Should(Equal(int32(2)), "activeWorkers should decrease from 5 to 2")
}

func TestWorkerCASPreventsStampede(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create test files
	for i := 1; i <= 10; i++ {
		testFile := filepath.Join(sourceDir, fmt.Sprintf("test%d.txt", i))
		err := os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0o600)
		g.Expect(err).ShouldNot(HaveOccurred())
	}

	// Create engine
	engine := mustNewEngine(t, sourceDir, destDir)
	engine.AdaptiveMode = true

	// Run analysis
	err := engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(engine.Status.TotalFiles).Should(Equal(10))

	// Simulate scenario: 10 workers active, desired is 7 (need to remove 3)
	atomic.StoreInt32(&engine.Status.ActiveWorkers, 10)
	engine.SetDesiredWorkers(7)

	// Create channels
	jobs := make(chan *syncengine.FileToSync, 10)
	errors := make(chan error, 10)
	var wg sync.WaitGroup

	// Add all jobs to channel
	for _, file := range engine.Status.FilesToSync {
		jobs <- file
	}
	close(jobs)

	// Start 10 workers - they'll all try to exit at similar times
	for i := 0; i < 10; i++ {
		wg.Add(1)

		go engine.TestWorker(&wg, jobs, errors)
	}

	// Wait for all workers to finish
	wg.Wait()

	// Verify exactly 3 workers exited (activeWorkers should be 7)
	finalActive := atomic.LoadInt32(&engine.Status.ActiveWorkers)
	g.Expect(finalActive).Should(Equal(int32(7)), "CAS should prevent stampede - exactly 3 workers should exit")
}
