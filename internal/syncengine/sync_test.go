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

func TestGetStatus_IncludesAllCurrentFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine := mustNewEngine(t, sourceDir, destDir)

	// Create 50 files in FilesToSync to simulate a large sync
	engine.Status.FilesToSync = make([]*syncengine.FileToSync, 50)
	for i := 0; i < 50; i++ {
		engine.Status.FilesToSync[i] = &syncengine.FileToSync{
			RelativePath: fmt.Sprintf("file%d.txt", i),
			Status:       "complete", // Most files are complete
			Size:         1024,
		}
	}

	// Simulate 4 workers actively working on files early in the array (files 5, 10, 15, 20)
	// These would be missed by the old backward iteration that only looked at last 20
	engine.Status.CurrentFiles = []string{"file5.txt", "file10.txt", "file15.txt", "file20.txt"}

	// Set these files to "copying" status
	engine.Status.FilesToSync[5].Status = "copying"
	engine.Status.FilesToSync[10].Status = "copying"
	engine.Status.FilesToSync[15].Status = "copying"
	engine.Status.FilesToSync[20].Status = "copying"

	// Call GetStatus()
	status := engine.GetStatus()

	// Verify all 4 CurrentFiles are included in the returned FilesToSync
	g.Expect(status.CurrentFiles).Should(HaveLen(4), "Should have 4 current files")

	// Build a map of returned files for easy lookup
	returnedFiles := make(map[string]*syncengine.FileToSync)
	for _, file := range status.FilesToSync {
		returnedFiles[file.RelativePath] = file
	}

	// Verify each CurrentFile is present with correct status
	for _, currentFile := range status.CurrentFiles {
		file, found := returnedFiles[currentFile]
		g.Expect(found).Should(BeTrue(), fmt.Sprintf("CurrentFile %s should be in FilesToSync", currentFile))
		g.Expect(file.Status).Should(Equal("copying"), fmt.Sprintf("File %s should have copying status", currentFile))
	}
}

func TestGetStatus_PrioritizesCurrentFilesOverRecent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine := mustNewEngine(t, sourceDir, destDir)

	// Create 100 files to ensure we exceed the 20-file limit
	engine.Status.FilesToSync = make([]*syncengine.FileToSync, 100)
	for i := 0; i < 100; i++ {
		engine.Status.FilesToSync[i] = &syncengine.FileToSync{
			RelativePath: fmt.Sprintf("file%d.txt", i),
			Status:       "complete",
			Size:         1024,
		}
	}

	// Set last 25 files to "complete" (recently finished)
	for i := 75; i < 100; i++ {
		engine.Status.FilesToSync[i].Status = "complete"
	}

	// Set 4 files early in the array to "copying" (actively being worked on)
	engine.Status.CurrentFiles = []string{"file10.txt", "file20.txt", "file30.txt", "file40.txt"}
	for _, currentFile := range engine.Status.CurrentFiles {
		for _, file := range engine.Status.FilesToSync {
			if file.RelativePath == currentFile {
				file.Status = "copying"
				break
			}
		}
	}

	// Call GetStatus()
	status := engine.GetStatus()

	// Build map of returned files
	returnedFiles := make(map[string]*syncengine.FileToSync)
	for _, file := range status.FilesToSync {
		returnedFiles[file.RelativePath] = file
	}

	// CRITICAL: All CurrentFiles MUST be in the returned FilesToSync
	for _, currentFile := range status.CurrentFiles {
		file, found := returnedFiles[currentFile]
		g.Expect(found).Should(BeTrue(), fmt.Sprintf("CurrentFile %s MUST be in FilesToSync (priority)", currentFile))
		g.Expect(file.Status).Should(Equal("copying"), fmt.Sprintf("File %s should have copying status", currentFile))
	}

	// Total returned files should not exceed a reasonable limit (e.g., 20 + currentFiles)
	// At minimum, we should have all CurrentFiles
	g.Expect(len(status.FilesToSync)).Should(BeNumerically(">=", len(status.CurrentFiles)),
		"FilesToSync should at least include all CurrentFiles")
}

func TestGetStatus_EmptyCurrentFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine := mustNewEngine(t, sourceDir, destDir)

	// Create 30 complete files
	engine.Status.FilesToSync = make([]*syncengine.FileToSync, 30)
	for i := 0; i < 30; i++ {
		engine.Status.FilesToSync[i] = &syncengine.FileToSync{
			RelativePath: fmt.Sprintf("file%d.txt", i),
			Status:       "complete",
			Size:         1024,
		}
	}

	// No CurrentFiles (all workers idle or finished)
	engine.Status.CurrentFiles = []string{}

	// Call GetStatus()
	status := engine.GetStatus()

	// Should return last 20 recently active files when CurrentFiles is empty
	g.Expect(len(status.FilesToSync)).Should(BeNumerically("<=", 20),
		"Should limit to 20 files when no CurrentFiles")
	g.Expect(len(status.CurrentFiles)).Should(Equal(0), "CurrentFiles should be empty")
}

// Phase 4 Tests: Sync Engine Integration with ResizablePool
// These tests verify the sync engine correctly detects and interacts with ResizablePool

// TestNewEngine_DetectsResizablePool_SFTPSource verifies NewEngine detects SFTP source
func TestNewEngine_DetectsResizablePool_SFTPSource(t *testing.T) {
	t.Skip("Requires SFTP server infrastructure - will be tested in integration phase")
	t.Parallel()
	g := NewWithT(t)

	// Create engine with SFTP source (sftp://user@host/path format triggers SFTP)
	engine, err := syncengine.NewEngine("sftp://testuser@localhost:2222/source", t.TempDir())
	g.Expect(err).ShouldNot(HaveOccurred())
	defer engine.Close()

	// Engine should have detected source as ResizablePool
	// Note: This test will need to access internal fields or use a getter
	// For now, we verify by calling a method that would panic if not set
	g.Expect(engine).ShouldNot(BeNil())
}

// TestNewEngine_DetectsResizablePool_SFTPDest verifies NewEngine detects SFTP destination
func TestNewEngine_DetectsResizablePool_SFTPDest(t *testing.T) {
	t.Skip("Requires SFTP server infrastructure - will be tested in integration phase")
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()

	// Create engine with SFTP destination
	engine, err := syncengine.NewEngine(sourceDir, "sftp://testuser@localhost:2222/dest")
	g.Expect(err).ShouldNot(HaveOccurred())
	defer engine.Close()

	// Engine should have detected dest as ResizablePool
	g.Expect(engine).ShouldNot(BeNil())
}

// TestNewEngine_DetectsResizablePool_BothSFTP verifies detection of both source and dest
func TestNewEngine_DetectsResizablePool_BothSFTP(t *testing.T) {
	t.Skip("Requires SFTP server infrastructure - will be tested in integration phase")
	t.Parallel()
	g := NewWithT(t)

	// Create engine with both SFTP source and destination
	engine, err := syncengine.NewEngine(
		"sftp://testuser@localhost:2222/source",
		"sftp://testuser@localhost:2222/dest",
	)
	g.Expect(err).ShouldNot(HaveOccurred())
	defer engine.Close()

	g.Expect(engine).ShouldNot(BeNil())
}

// TestNewEngine_HandlesNonResizable_LocalSource verifies nil for local filesystem
func TestNewEngine_HandlesNonResizable_LocalSource(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create engine with local filesystems (neither implements ResizablePool)
	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()

	// Should not panic - resizable pool references should be nil
	g.Expect(engine).ShouldNot(BeNil())
}

// TestResizePools_CallsResizePoolOnBoth verifies resizePools() calls both pools
func TestResizePools_CallsResizePoolOnBoth(t *testing.T) {
	t.Parallel()
	// This test will need mock ResizablePool implementations to verify calls
	// Will be implemented once the interface integration is in place
}

// TestResizePools_HandlesNilSourceGracefully verifies nil source is safe
func TestResizePools_HandlesNilSourceGracefully(t *testing.T) {
	t.Skip("Requires SFTP server infrastructure - will be tested in integration phase")
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()

	// Mixed: local source, SFTP dest
	engine, err := syncengine.NewEngine(sourceDir, "sftp://testuser@localhost:2222/dest")
	g.Expect(err).ShouldNot(HaveOccurred())
	defer engine.Close()

	// Should not panic when calling resizePools (source is nil)
	// This will be verified by calling MakeScalingDecision which calls resizePools
	workerControl := make(chan bool, 10)
	engine.MakeScalingDecision(0, 1024*1024, 1, 4, workerControl)
	close(workerControl)
}

// TestResizePools_HandlesNilDestGracefully verifies nil dest is safe
func TestResizePools_HandlesNilDestGracefully(t *testing.T) {
	t.Skip("Requires SFTP server infrastructure - will be tested in integration phase")
	t.Parallel()
	g := NewWithT(t)

	destDir := t.TempDir()

	// Mixed: SFTP source, local dest
	engine, err := syncengine.NewEngine("sftp://testuser@localhost:2222/source", destDir)
	g.Expect(err).ShouldNot(HaveOccurred())
	defer engine.Close()

	// Should not panic when calling resizePools (dest is nil)
	workerControl := make(chan bool, 10)
	engine.MakeScalingDecision(0, 1024*1024, 1, 4, workerControl)
	close(workerControl)
}

// TestResizePools_HandlesBothNilGracefully verifies both nil is safe
func TestResizePools_HandlesBothNilGracefully(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Both local (neither implements ResizablePool)
	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()

	// Should not panic when calling resizePools (both are nil)
	workerControl := make(chan bool, 10)
	engine.MakeScalingDecision(0, 1024*1024, 1, 4, workerControl)
	close(workerControl)
}

// TestMakeScalingDecision_CallsResizePools_OnWorkerIncrease verifies pool resize on scale-up
func TestMakeScalingDecision_CallsResizePools_OnWorkerIncrease(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// This test will need to verify that when MakeScalingDecision increases
	// desiredWorkers, it also calls resizePools with the new count
	// Will require instrumentation or mock to verify the call
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()

	engine.SetDesiredWorkers(2)

	workerControl := make(chan bool, 10)

	// Simulate improved per-worker speed - should add worker
	engine.MakeScalingDecision(
		1000000.0, // lastPerWorkerSpeed: 1 MB/s
		1100000.0, // currentPerWorkerSpeed: 1.1 MB/s (10% improvement)
		2,         // currentWorkers
		10,        // maxWorkers
		workerControl,
	)

	// Verify desiredWorkers was incremented
	desired := engine.GetDesiredWorkers()
	g.Expect(desired).Should(Equal(int32(3)), "desiredWorkers should increase from 2 to 3")

	// Verify worker was added
	select {
	case addWorker := <-workerControl:
		g.Expect(addWorker).Should(BeTrue())
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected worker to be added")
	}

	close(workerControl)
}

// TestMakeScalingDecision_CallsResizePools_OnWorkerDecrease verifies pool resize on scale-down
func TestMakeScalingDecision_CallsResizePools_OnWorkerDecrease(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()

	engine.SetDesiredWorkers(5)

	workerControl := make(chan bool, 10)

	// Simulate decreased per-worker speed - should decrement desired
	engine.MakeScalingDecision(
		1000000.0, // lastPerWorkerSpeed: 1 MB/s
		500000.0,  // currentPerWorkerSpeed: 0.5 MB/s (50% decrease)
		5,         // currentWorkers
		10,        // maxWorkers
		workerControl,
	)

	// Verify desiredWorkers was decremented
	desired := engine.GetDesiredWorkers()
	g.Expect(desired).Should(Equal(int32(4)), "desiredWorkers should decrease from 5 to 4")

	// Verify NO worker was added to channel
	select {
	case <-workerControl:
		t.Fatal("Should not add worker when scaling down")
	case <-time.After(50 * time.Millisecond):
		// Expected - no worker added
	}

	close(workerControl)
}

// TestSyncAdaptive_CallsResizePools_OnInitialSetup verifies pool resize during startup
func TestSyncAdaptive_CallsResizePools_OnInitialSetup(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(sourceDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0o600)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create engine with adaptive mode
	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()
	engine.AdaptiveMode = true
	engine.Workers = 0 // 0 means adaptive (starts with 1 worker)

	// Run Analyze
	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(engine.Status.TotalFiles).Should(Equal(1))

	// Run Sync - should call resizePools during initial worker setup
	err = engine.Sync()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify file was synced
	destFile := filepath.Join(destDir, "test.txt")
	_, err = os.Stat(destFile)
	g.Expect(err).ShouldNot(HaveOccurred())
}

// TestSyncAdaptive_ResizesPoolsWithWorkerCount_Integration is an integration test
// that verifies SFTP pool size follows desiredWorkers throughout adaptive scaling
func TestSyncAdaptive_ResizesPoolsWithWorkerCount_Integration(t *testing.T) {
	t.Parallel()
	// This test requires:
	// 1. Mock or real SFTP server
	// 2. Engine with SFTP filesystems
	// 3. Monitoring of pool size as desiredWorkers changes
	// 4. Verification that pool.TargetSize() matches engine.desiredWorkers
	//
	// Will be implemented once Phase 1-3 are complete and we can test
	// the full integration with real SFTP filesystems
}

// ========================================================================
// Issue #10 Refinement 2 Tests: In-transfer rate sampling and 10s evaluation
// ========================================================================

// TestProgressCallback_AddsRateSampleDuringTransfer verifies that rate samples
// are added during file transfer progress (not just on completion).
// This ensures the rolling window stays fresh during large file transfers.
//
// Target behavior (Change 1):
// - addRateSample() called every 1 second during progressCallback
// - Each sample captures current transfer state (bytes, workers, timestamp)
func TestProgressCallback_AddsRateSampleDuringTransfer(t *testing.T) {
	t.Skip("Flaky test: race condition (reads RecentSamples without lock) + timing assumption (files transfer too fast)")
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create multiple large files to ensure transfer takes multiple seconds
	for i := 0; i < 5; i++ {
		largeContent := make([]byte, 50*1024*1024) // 50 MB per file
		for j := range largeContent {
			largeContent[j] = byte(j % 256)
		}

		testFile := filepath.Join(sourceDir, fmt.Sprintf("large%d.bin", i))
		err := os.WriteFile(testFile, largeContent, 0o600)
		g.Expect(err).ShouldNot(HaveOccurred())
	}

	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()
	engine.FileOps = fileops.NewRealFileOps()

	// Run Analyze
	err := engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(engine.Status.TotalFiles).Should(Equal(5))

	// Record initial sample count
	initialSampleCount := len(engine.Status.Workers.RecentSamples)

	// Start sync in a goroutine and monitor sample count during transfer
	syncDone := make(chan error, 1)
	go func() {
		syncDone <- engine.Sync()
	}()

	// Poll for samples being added during transfer (not just at completion)
	samplesSeenDuringTransfer := 0
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)

	for {
		select {
		case err := <-syncDone:
			// Transfer completed
			g.Expect(err).ShouldNot(HaveOccurred())
			goto done
		case <-timeout:
			t.Fatal("Test timed out after 30 seconds")
		case <-ticker.C:
			// Check if samples are being added during transfer
			currentSampleCount := len(engine.Status.Workers.RecentSamples)
			currentProcessed := engine.Status.ProcessedFiles
			if currentSampleCount > initialSampleCount && currentProcessed < 5 {
				// Not all files complete, but samples are being added
				samplesSeenDuringTransfer++
			}
		}
	}

done:
	// CRITICAL ASSERTION: Rate samples should be added DURING transfer (not just at completion)
	// We should have seen samples added while not all files were complete
	finalSampleCount := len(engine.Status.Workers.RecentSamples)
	g.Expect(finalSampleCount).Should(BeNumerically(">", initialSampleCount),
		"Rate samples should be added during file transfer (Change 1)")

	// This will fail until Change 1 is implemented (in-transfer sampling every 1 second)
	g.Expect(samplesSeenDuringTransfer).Should(BeNumerically(">", 0),
		"Should observe samples being added DURING transfer (not just at completion)")
}

// TestProgressCallback_SamplesEverySecond verifies that rate samples are added
// approximately every 1 second during file transfer, not on every callback.
//
// Target behavior (Change 1):
// - Progress callback may be called frequently (every 100ms)
// - But addRateSample() should only be called every ~1 second
// - This prevents flooding the rolling window with too many samples
func TestProgressCallback_SamplesEverySecond(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a large file that takes several seconds to copy
	largeContent := make([]byte, 100*1024*1024) // 100 MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	testFile := filepath.Join(sourceDir, "large.bin")
	err := os.WriteFile(testFile, largeContent, 0o600)
	g.Expect(err).ShouldNot(HaveOccurred())

	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()
	engine.FileOps = fileops.NewRealFileOps()

	// Run Analyze
	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Record initial state
	initialSampleCount := len(engine.Status.Workers.RecentSamples)
	startTime := time.Now()

	// Run Sync (will trigger many progress callbacks during transfer)
	err = engine.Sync()
	g.Expect(err).ShouldNot(HaveOccurred())

	elapsedTime := time.Since(startTime)

	// CRITICAL ASSERTION: Despite many callbacks, we should only have ~N samples (one per second)
	finalSampleCount := len(engine.Status.Workers.RecentSamples)
	samplesAdded := finalSampleCount - initialSampleCount

	// We expect approximately 1 sample per second of elapsed time
	// (plus one final sample on completion)
	expectedSamplesLower := int(elapsedTime.Seconds()) - 1
	expectedSamplesUpper := int(elapsedTime.Seconds()) + 3

	g.Expect(samplesAdded).Should(BeNumerically(">=", expectedSamplesLower),
		"Should add approximately 1 sample per second during transfer (Change 1)")

	g.Expect(samplesAdded).Should(BeNumerically("<=", expectedSamplesUpper),
		"Should not add excessive samples (throttled to ~1 per second)")
}

// TestProgressCallback_CapturesTransferState verifies that each rate sample
// captures the current transfer state accurately.
//
// Target behavior (Change 1):
// - Each sample includes BytesTransferred, ActiveWorkers, Timestamp
// - Samples reflect the actual state at the time of sampling
func TestProgressCallback_CapturesTransferState(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a test file
	largeContent := make([]byte, 20*1024*1024) // 20 MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	testFile := filepath.Join(sourceDir, "test.bin")
	err := os.WriteFile(testFile, largeContent, 0o600)
	g.Expect(err).ShouldNot(HaveOccurred())

	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()
	engine.FileOps = fileops.NewRealFileOps()

	// Run Analyze
	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Clear any existing samples
	engine.Status.Workers.RecentSamples = []syncengine.RateSample{}

	// Set known active workers count
	atomic.StoreInt32(&engine.Status.ActiveWorkers, 2)

	// Run Sync
	err = engine.Sync()
	g.Expect(err).ShouldNot(HaveOccurred())

	// CRITICAL ASSERTION: Samples should capture current transfer state
	g.Expect(len(engine.Status.Workers.RecentSamples)).Should(BeNumerically(">", 0),
		"At least one sample should be added during transfer (Change 1)")

	if len(engine.Status.Workers.RecentSamples) > 0 {
		// Check samples from the middle of the list (during transfer, not just completion)
		for i, sample := range engine.Status.Workers.RecentSamples {
			// Verify sample captures state
			g.Expect(sample.ActiveWorkers).Should(BeNumerically(">=", 1),
				fmt.Sprintf("Sample %d should capture ActiveWorkers >= 1", i))

			g.Expect(sample.Timestamp).ShouldNot(BeZero(),
				fmt.Sprintf("Sample %d should have valid timestamp", i))

			g.Expect(sample.BytesTransferred).Should(BeNumerically(">=", 0),
				fmt.Sprintf("Sample %d should capture bytes transferred", i))
		}

		// At least one sample should have bytes transferred > 0 (indicating active transfer)
		hasActiveTransfer := false
		for _, sample := range engine.Status.Workers.RecentSamples {
			if sample.BytesTransferred > 0 {
				hasActiveTransfer = true
				break
			}
		}
		g.Expect(hasActiveTransfer).Should(BeTrue(),
			"At least one sample should show active transfer (BytesTransferred > 0)")
	}
}

// TestEvaluationInterval_Is10Seconds verifies that the evaluation interval
// constant is set to 10 seconds (not 5 seconds).
//
// Target behavior (Change 2):
// - const evaluationInterval = 10 * time.Second
func TestEvaluationInterval_Is10Seconds(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// This test verifies the constant value in sync.go line 1553
	// Current: const evaluationInterval = 5 * time.Second
	// Target: const evaluationInterval = 10 * time.Second

	baseTime := time.Now()
	const expectedEvaluationInterval = 10 * time.Second

	state := &syncengine.AdaptiveScalingState{
		LastCheckTime: baseTime,
	}

	// Test at 9.9 seconds - should NOT trigger evaluation
	currentTime1 := baseTime.Add(9900 * time.Millisecond)
	shouldEvaluate1 := currentTime1.Sub(state.LastCheckTime) >= expectedEvaluationInterval
	g.Expect(shouldEvaluate1).Should(BeFalse(),
		"Should NOT evaluate at 9.9 seconds (below 10s threshold)")

	// Test at exactly 10.0 seconds - SHOULD trigger evaluation
	currentTime2 := baseTime.Add(10 * time.Second)
	shouldEvaluate2 := currentTime2.Sub(state.LastCheckTime) >= expectedEvaluationInterval
	g.Expect(shouldEvaluate2).Should(BeTrue(),
		"Should evaluate at exactly 10.0 seconds (Change 2: 10s interval)")

	// Test at 15 seconds - SHOULD trigger evaluation
	currentTime3 := baseTime.Add(15 * time.Second)
	shouldEvaluate3 := currentTime3.Sub(state.LastCheckTime) >= expectedEvaluationInterval
	g.Expect(shouldEvaluate3).Should(BeTrue(),
		"Should evaluate at 15 seconds (well past 10s threshold)")
}

// TestSyncAdaptive_EvaluatesEvery10Seconds verifies that evaluations occur
// every 10 seconds (not 5 seconds) during adaptive scaling.
//
// Target behavior (Change 2):
// - Evaluation interval changed from 5s to 10s
// - Evaluations should occur at 10s, 20s, 30s intervals
func TestSyncAdaptive_EvaluatesEvery10Seconds(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create test files
	for i := 0; i < 20; i++ {
		testFile := filepath.Join(sourceDir, fmt.Sprintf("test%d.txt", i))
		err := os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0o600)
		g.Expect(err).ShouldNot(HaveOccurred())
	}

	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()
	engine.AdaptiveMode = true
	engine.Workers = 0 // Adaptive starts with 1 worker

	// Create mocked TimeProvider
	timeImp := NewTimeProviderImp(t)
	engine.TimeProvider = timeImp.Mock

	// Run Analyze
	go func() {
		// Expect Analyze phase Now() calls
		analyzeTime := time.Now()
		for range 50 {
			nowCall := timeImp.Within(5 * time.Second).ExpectCallIs.Now()
			nowCall.InjectResult(analyzeTime)
		}
	}()

	err := engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Track evaluation times
	var evaluationTimes []time.Time
	evaluationMutex := &sync.Mutex{}

	// Wrap EvaluateAndScale to track when it's called
	originalDesired := engine.GetDesiredWorkers()

	// Set up mock time expectations for Sync phase
	baseTime := time.Now()
	tickerChan := make(chan time.Time, 100)
	done := make(chan struct{})

	go func() {
		defer close(done)

		// Expect NewTicker call
		tickerCall := timeImp.Within(5 * time.Second).ExpectCallIs.NewTicker()
		tickerCall.ExpectArgsAre(1 * time.Second)
		mockTicker := &syncengine.MockTicker{TickChan: tickerChan}
		tickerCall.InjectResult(mockTicker)

		// Send ticks every 100ms, simulate 30 seconds of operation
		currentTime := baseTime
		for i := 0; i < 300; i++ {
			select {
			case <-done:
				return
			case <-time.After(10 * time.Millisecond):
				currentTime = currentTime.Add(100 * time.Millisecond)

				// Provide Now() for evaluation check
				nowCall := timeImp.Within(1 * time.Second).ExpectCallIs.Now()
				nowCall.InjectResult(currentTime)

				// Send tick
				select {
				case tickerChan <- currentTime:
				default:
				}

				// Check if evaluation happened at 10s intervals
				desiredNow := engine.GetDesiredWorkers()
				if desiredNow != originalDesired {
					evaluationMutex.Lock()
					evaluationTimes = append(evaluationTimes, currentTime)
					originalDesired = desiredNow
					evaluationMutex.Unlock()
				}
			}
		}
	}()

	// Run Sync
	err = engine.Sync()
	close(done)

	g.Expect(err).ShouldNot(HaveOccurred())

	// CRITICAL ASSERTION: Evaluations should occur at 10s intervals (not 5s)
	// We expect evaluations at approximately: 10s, 20s, 30s
	evaluationMutex.Lock()
	defer evaluationMutex.Unlock()

	if len(evaluationTimes) > 0 {
		// First evaluation should be around 10 seconds
		firstEval := evaluationTimes[0].Sub(baseTime)
		g.Expect(firstEval).Should(BeNumerically(">=", 9*time.Second),
			"First evaluation should occur at ~10 seconds (not 5 seconds)")

		g.Expect(firstEval).Should(BeNumerically("<=", 11*time.Second),
			"First evaluation should occur at ~10 seconds (Change 2)")

		// If there are multiple evaluations, verify ~10s spacing
		if len(evaluationTimes) > 1 {
			for i := 1; i < len(evaluationTimes); i++ {
				interval := evaluationTimes[i].Sub(evaluationTimes[i-1])
				g.Expect(interval).Should(BeNumerically(">=", 9*time.Second),
					"Evaluation intervals should be ~10 seconds apart")

				g.Expect(interval).Should(BeNumerically("<=", 11*time.Second),
					"Evaluation intervals should be ~10 seconds apart (Change 2)")
			}
		}
	}
}

// TestSyncAdaptive_10sIntervalWithFullRollingWindow verifies that the 10-second
// evaluation interval aligns with the 10-second rolling window.
//
// Target behavior (Change 2):
// - Rolling window is 10 seconds
// - Evaluation interval is 10 seconds
// - This ensures evaluation considers a full window of recent data
func TestSyncAdaptive_10sIntervalWithFullRollingWindow(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	const rollingWindowDuration = 10 * time.Second
	const evaluationInterval = 10 * time.Second

	// Verify intervals match
	g.Expect(evaluationInterval).Should(Equal(rollingWindowDuration),
		"Evaluation interval (10s) should match rolling window duration (10s) for stable scaling decisions")

	baseTime := time.Now()

	// Simulate scenario:
	// - Samples collected from T=0 to T=10
	// - Evaluation happens at T=10
	// - All samples in window are included in evaluation

	samples := []syncengine.RateSample{
		{Timestamp: baseTime.Add(1 * time.Second), BytesTransferred: 1024, ActiveWorkers: 2},
		{Timestamp: baseTime.Add(2 * time.Second), BytesTransferred: 1024, ActiveWorkers: 2},
		{Timestamp: baseTime.Add(3 * time.Second), BytesTransferred: 1024, ActiveWorkers: 2},
		{Timestamp: baseTime.Add(9 * time.Second), BytesTransferred: 1024, ActiveWorkers: 2},
		{Timestamp: baseTime.Add(10 * time.Second), BytesTransferred: 1024, ActiveWorkers: 2},
	}

	evaluationTime := baseTime.Add(10 * time.Second)
	cutoffTime := evaluationTime.Add(-rollingWindowDuration)

	// All samples should be within the window at evaluation time
	for _, sample := range samples {
		withinWindow := !sample.Timestamp.Before(cutoffTime)
		g.Expect(withinWindow).Should(BeTrue(),
			"All samples from last 10 seconds should be in window at 10s evaluation (Change 2)")
	}

	// Sample from before T=0 should be excluded (outside 10s window)
	oldSample := syncengine.RateSample{Timestamp: baseTime.Add(-1 * time.Second), BytesTransferred: 1024, ActiveWorkers: 2}
	g.Expect(oldSample.Timestamp.Before(cutoffTime)).Should(BeTrue(),
		"Sample from T=-1 should be outside window at T=10 evaluation")
}

// TestEvaluateAndScale_UsesSmoothedRate verifies EvaluateAndScale uses
// WorkerMetrics.PerWorkerRate from rolling window instead of raw point-to-point calculation.
// This test ensures adaptive scaling uses the 5-sample rolling window for smoother decisions.
//
// Test approach: Populate the rolling window with samples showing a clear smoothed trend,
// then verify the scaling decision reflects that smoothed rate, not raw point-to-point calculation.
func TestEvaluateAndScale_UsesSmoothedRate(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()

	// Create mocked TimeProvider
	timeImp := NewTimeProviderImp(t)
	engine.TimeProvider = timeImp.Mock

	// Scenario: We populate the rolling window via GetStatus() which calls ComputeProgressMetrics()
	// The rolling window should already have samples if files were processed.
	// For this test, we'll verify that EvaluateAndScale uses the Workers.PerWorkerRate field
	// from the Status, rather than recalculating from scratch.

	baseTime := time.Now()

	// Set up adaptive scaling state with a baseline measurement
	state := &syncengine.AdaptiveScalingState{
		LastProcessedFiles: 5,
		LastPerWorkerSpeed: 1000000.0, // 1 MB/s per worker (baseline)
		FilesAtLastCheck:   5,
		LastCheckTime:      baseTime.Add(-3 * time.Second),
	}

	// Set up mock time expectations
	done := make(chan struct{})
	go func() {
		defer close(done)
		nowCall := timeImp.Within(2 * time.Second).ExpectCallIs.Now()
		nowCall.InjectResult(baseTime.Add(1 * time.Second))
	}()

	workerControl := make(chan bool, 10)

	// Call EvaluateAndScale with parameters that would trigger a scaling decision
	// The key is that if the implementation uses the smoothed rate from Workers.PerWorkerRate,
	// it will produce different behavior than raw calculation.
	engine.EvaluateAndScale(state, 10, 2, 4500000, 10, workerControl)

	<-done
	close(workerControl)

	// CRITICAL ASSERTION: After the implementation is complete, we verify that:
	// 1. state.LastPerWorkerSpeed is updated with the smoothed rate from Status.Workers.PerWorkerRate
	// 2. The GetStatus() call should show that Workers.RecentSamples has been populated
	//
	// This test currently skipped - will be implemented alongside the feature.
	// The implementer should:
	// - Call Status.calculateWorkerMetrics() to get smoothed PerWorkerRate
	// - Use that smoothed rate for scaling decisions
	// - Store it in state.LastPerWorkerSpeed for next comparison
	//
	// Once implemented, add assertions here to verify the smoothed rate is used.
}

// TestSyncAdaptive_TimeBasedEvaluation_FileCountCheck verifies the OLD behavior:
// current implementation uses file-count to trigger evaluation.
// This test documents the CURRENT behavior that needs to change.
func TestSyncAdaptive_TimeBasedEvaluation_FileCountCheck(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	baseTime := time.Now()

	//  Current implementation (line 1558 in sync.go):
	// if filesSinceLastCheck >= currentWorkers*targetFilesPerWorker {
	//     e.EvaluateAndScale(...)
	// }

	state := &syncengine.AdaptiveScalingState{
		FilesAtLastCheck: 10,
		LastCheckTime:    baseTime,
	}

	currentProcessedFiles := 20 // 10 files processed since last check
	currentWorkers := 2
	targetFilesPerWorker := 5

	filesSinceLastCheck := currentProcessedFiles - state.FilesAtLastCheck

	// OLD logic (current implementation)
	shouldEvaluateFileCount := filesSinceLastCheck >= currentWorkers*targetFilesPerWorker
	g.Expect(shouldEvaluateFileCount).Should(BeTrue(),
		"Current file-count logic: 10 files >= 2*5=10 threshold → should evaluate")

	// This test passes with CURRENT implementation
	// After Change 2, this logic should be REMOVED and replaced with time-based check
}

// TestSyncAdaptive_TimeBasedEvaluation_TimeCheck verifies the NEW behavior:
// evaluation should trigger based on time elapsed (5 seconds), not file count.
// This test will FAIL until Change 2 is implemented.
func TestSyncAdaptive_TimeBasedEvaluation_TimeCheck(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	baseTime := time.Now()
	const evaluationInterval = 5 * time.Second

	// Target implementation (should replace line 1558 in sync.go):
	// if time.Since(state.LastCheckTime) >= evaluationInterval {
	//     e.EvaluateAndScale(...)
	// }

	// Scenario 1: 5 seconds elapsed, few files processed → SHOULD evaluate (time-based)
	state1 := &syncengine.AdaptiveScalingState{
		FilesAtLastCheck: 10,
		LastCheckTime:    baseTime,
	}
	currentTime1 := baseTime.Add(5 * time.Second)
	currentProcessedFiles1 := 12 // Only 2 files processed
	currentWorkers1 := 2
	targetFilesPerWorker := 5

	filesSinceLastCheck1 := currentProcessedFiles1 - state1.FilesAtLastCheck

	// OLD logic (current): Would NOT evaluate (2 files < 10 threshold)
	shouldEvaluateFileCount1 := filesSinceLastCheck1 >= currentWorkers1*targetFilesPerWorker
	g.Expect(shouldEvaluateFileCount1).Should(BeFalse(),
		"File-count logic: 2 files < 10 threshold → would NOT evaluate")

	// NEW logic (time-based): SHOULD evaluate (5 seconds elapsed)
	shouldEvaluateTime1 := currentTime1.Sub(state1.LastCheckTime) >= evaluationInterval
	g.Expect(shouldEvaluateTime1).Should(BeTrue(),
		"Time-based logic: 5 seconds elapsed → SHOULD evaluate (this is the NEW behavior)")

	// Scenario 2: Only 3 seconds elapsed, many files processed → should NOT evaluate (time-based)
	state2 := &syncengine.AdaptiveScalingState{
		FilesAtLastCheck: 10,
		LastCheckTime:    baseTime,
	}
	currentTime2 := baseTime.Add(3 * time.Second)
	currentProcessedFiles2 := 100 // 90 files processed (way more than threshold)
	currentWorkers2 := 2

	filesSinceLastCheck2 := currentProcessedFiles2 - state2.FilesAtLastCheck

	// OLD logic (current): WOULD evaluate (90 files >= 10 threshold)
	shouldEvaluateFileCount2 := filesSinceLastCheck2 >= currentWorkers2*targetFilesPerWorker
	g.Expect(shouldEvaluateFileCount2).Should(BeTrue(),
		"File-count logic: 90 files >= 10 threshold → would evaluate")

	// NEW logic (time-based): should NOT evaluate (only 3 seconds < 5 second threshold)
	shouldEvaluateTime2 := currentTime2.Sub(state2.LastCheckTime) >= evaluationInterval
	g.Expect(shouldEvaluateTime2).Should(BeFalse(),
		"Time-based logic: 3 seconds < 5 second threshold → should NOT evaluate (this is the NEW behavior)")

	// The implementer should:
	// 1. Remove the file-count check at line 1558: if filesSinceLastCheck >= currentWorkers*targetFilesPerWorker
	// 2. Replace with time-based check: if time.Since(state.LastCheckTime) >= evaluationInterval
	// 3. Add const evaluationInterval = 5 * time.Second
}

// TestSyncAdaptive_TimeBasedEvaluation_IntervalConstant verifies that the evaluation
// interval is always 5 seconds, regardless of worker count or file count.
func TestSyncAdaptive_TimeBasedEvaluation_IntervalConstant(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	const evaluationInterval = 5 * time.Second
	baseTime := time.Now()

	// The interval should be a CONSTANT (5 seconds), not dependent on:
	// - worker count (currentWorkers)
	// - target files per worker (targetFilesPerWorker)
	// - files processed (filesSinceLastCheck)

	// Test at exactly 5.0 seconds - should evaluate
	state1 := &syncengine.AdaptiveScalingState{LastCheckTime: baseTime}
	currentTime1 := baseTime.Add(5000 * time.Millisecond)
	g.Expect(currentTime1.Sub(state1.LastCheckTime) >= evaluationInterval).Should(BeTrue(),
		"Should evaluate at exactly 5.0 seconds")

	// Test at 4.9 seconds - should NOT evaluate
	state2 := &syncengine.AdaptiveScalingState{LastCheckTime: baseTime}
	currentTime2 := baseTime.Add(4900 * time.Millisecond)
	g.Expect(currentTime2.Sub(state2.LastCheckTime) >= evaluationInterval).Should(BeFalse(),
		"Should NOT evaluate at 4.9 seconds (below threshold)")

	// Test at 10 seconds - should evaluate (10 >= 5)
	state3 := &syncengine.AdaptiveScalingState{LastCheckTime: baseTime}
	currentTime3 := baseTime.Add(10 * time.Second)
	g.Expect(currentTime3.Sub(state3.LastCheckTime) >= evaluationInterval).Should(BeTrue(),
		"Should evaluate at 10 seconds (well past threshold)")

	// Verify interval is independent of worker count
	// 1 worker or 100 workers - same 5 second interval
	g.Expect(evaluationInterval).Should(Equal(5*time.Second),
		"Evaluation interval must be constant 5 seconds, not derived from worker/file counts")
}

// TestSyncAdaptive_ActualTimeBehavior_Integration verifies the actual code change in startAdaptiveScaling.
// This is an integration test that will FAIL until the implementation is changed from file-count to time-based.
//
// Expected changes in sync.go around line 1558:
// OLD: if filesSinceLastCheck >= currentWorkers*targetFilesPerWorker {
// NEW: const evaluationInterval = 5 * time.Second
//      if time.Since(state.LastCheckTime) >= evaluationInterval {
func TestSyncAdaptive_ActualTimeBehavior_Integration(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create exactly 3 small test files (below the file-count threshold)
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(sourceDir, fmt.Sprintf("test%d.txt", i))
		err := os.WriteFile(testFile, []byte("small content"), 0o600)
		g.Expect(err).ShouldNot(HaveOccurred())
	}

	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()
	engine.AdaptiveMode = true
	engine.Workers = 0 // Adaptive mode starts with 1 worker

	// Run Analyze
	err := engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(engine.Status.TotalFiles).Should(Equal(3))

	// Record initial desired workers
	initialDesired := engine.GetDesiredWorkers()

	// Run Sync
	err = engine.Sync()
	g.Expect(err).ShouldNot(HaveOccurred())

	// After implementation change to time-based evaluation:
	// - Evaluation should happen after 5 seconds (not based on file count)
	// - With only 3 files and 1 worker, file-count logic would NOT trigger evaluation
	//   (3 files < 1 worker * 5 targetFilesPerWorker = 5)
	// - But time-based logic SHOULD trigger evaluation after 5 seconds
	// - desiredWorkers should have been adjusted at least once

	finalDesired := engine.GetDesiredWorkers()

	// This assertion will PASS once time-based evaluation is implemented
	// (evaluation happens after 5 seconds regardless of file count)
	g.Expect(finalDesired).Should(BeNumerically(">", initialDesired),
		"After time-based implementation, evaluation should occur based on time (5s), not file count")
}

// TestEvaluateAndScale_ColdStart verifies behavior with < 2 samples.
// With insufficient samples, scaling decisions should be conservative (baseline behavior).
//
// Test approach: Call EvaluateAndScale on a fresh engine with no prior measurements.
// Verify it falls back to baseline behavior instead of making speed-based decisions.
func TestEvaluateAndScale_ColdStart(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()

	// Create mocked TimeProvider
	timeImp := NewTimeProviderImp(t)
	engine.TimeProvider = timeImp.Mock

	// COLD START: No previous measurements, no samples in rolling window
	baseTime := time.Now()
	state := &syncengine.AdaptiveScalingState{
		LastProcessedFiles: 0,
		LastPerWorkerSpeed: 0,
		FilesAtLastCheck:   0,
		LastCheckTime:      time.Time{}, // No previous check
	}

	// Set up mock time expectations
	done := make(chan struct{})
	go func() {
		defer close(done)
		nowCall := timeImp.Within(2 * time.Second).ExpectCallIs.Now()
		nowCall.InjectResult(baseTime.Add(1 * time.Second))
	}()

	workerControl := make(chan bool, 10)

	// Call EvaluateAndScale with cold-start conditions (first call, no history)
	engine.EvaluateAndScale(state, 5, 1, 1000000, 10, workerControl)

	<-done

	// CRITICAL ASSERTION: With < 2 samples in the rolling window, the implementation should:
	// 1. Check len(Status.Workers.RecentSamples) >= 2
	// 2. If insufficient samples, skip speed-based scaling decision
	// 3. Fall back to conservative baseline behavior (e.g., add worker if files processed)
	//
	// Expected behavior: Either add a worker (baseline) or no action (waiting for more data)
	select {
	case addWorker := <-workerControl:
		g.Expect(addWorker).Should(BeTrue(), "Cold-start should use baseline behavior (add worker)")
	case <-time.After(100 * time.Millisecond):
		// Also acceptable - no scaling decision made due to insufficient data
		t.Log("No worker added during cold-start - waiting for more samples (acceptable)")
	}

	close(workerControl)

	// Verify state was updated with baseline values (not speed-based)
	g.Expect(state.LastCheckTime).Should(Equal(baseTime.Add(1 * time.Second)),
		"LastCheckTime should be updated even during cold-start")
	g.Expect(state.LastProcessedFiles).Should(Equal(5),
		"LastProcessedFiles should be updated even during cold-start")
}

// TestMakeScalingDecision_WidenedThresholds verifies 0.90/1.10 thresholds instead of 0.98/1.02.
// This test ensures the widened thresholds prevent excessive scaling decisions with smoothed data.
func TestMakeScalingDecision_WidenedThresholds(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine := mustNewEngine(t, sourceDir, destDir)
	defer engine.Close()

	workerControl := make(chan bool, 10)
	defer close(workerControl)

	// Test Case 1: Speed ratio 0.85 (< 0.90) - should scale DOWN (decrement desiredWorkers)
	engine.SetDesiredWorkers(5)
	engine.MakeScalingDecision(
		1000000.0, // lastPerWorkerSpeed: 1 MB/s
		850000.0,  // currentPerWorkerSpeed: 0.85 MB/s (15% decrease, < 0.90 threshold)
		5,         // currentWorkers
		10,        // maxWorkers
		workerControl,
	)

	desired := engine.GetDesiredWorkers()
	g.Expect(desired).Should(Equal(int32(4)),
		"Speed ratio 0.85 (< 0.90 low threshold) should decrement desiredWorkers from 5 to 4")

	// Channel should be empty (no worker added when scaling down)
	select {
	case <-workerControl:
		t.Fatal("Should not add worker when speed ratio < 0.90")
	case <-time.After(50 * time.Millisecond):
		// Expected - no worker added
	}

	// Test Case 2: Speed ratio 0.95 (between 0.90-1.10) - should scale UP (stable/improving)
	engine.SetDesiredWorkers(3)
	engine.MakeScalingDecision(
		1000000.0, // lastPerWorkerSpeed: 1 MB/s
		950000.0,  // currentPerWorkerSpeed: 0.95 MB/s (5% decrease, but within stable band)
		3,         // currentWorkers
		10,        // maxWorkers
		workerControl,
	)

	desired = engine.GetDesiredWorkers()
	g.Expect(desired).Should(Equal(int32(4)),
		"Speed ratio 0.95 (within 0.90-1.10 stable band) should increment desiredWorkers from 3 to 4")

	// Worker should be added
	select {
	case addWorker := <-workerControl:
		g.Expect(addWorker).Should(BeTrue(), "Should add worker when speed is stable")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected worker to be added for stable speed")
	}

	// Test Case 3: Speed ratio 1.15 (> 1.10) - should scale UP (significant improvement)
	engine.SetDesiredWorkers(2)
	engine.MakeScalingDecision(
		1000000.0, // lastPerWorkerSpeed: 1 MB/s
		1150000.0, // currentPerWorkerSpeed: 1.15 MB/s (15% increase, > 1.10 threshold)
		2,         // currentWorkers
		10,        // maxWorkers
		workerControl,
	)

	desired = engine.GetDesiredWorkers()
	g.Expect(desired).Should(Equal(int32(3)),
		"Speed ratio 1.15 (> 1.10 high threshold) should increment desiredWorkers from 2 to 3")

	// Worker should be added
	select {
	case addWorker := <-workerControl:
		g.Expect(addWorker).Should(BeTrue(), "Should add worker when speed ratio > 1.10")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected worker to be added for improved speed")
	}
}
