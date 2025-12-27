package syncengine_test

import (
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

//go:generate impgen syncengine.NewEngine
//go:generate impgen fileops.FilesNeedSync
//go:generate impgen fileops.ComputeFileHash
//go:generate impgen fileops.CompareFilesBytes
//go:generate impgen fileops.CountFiles
//go:generate impgen fileops.ScanDirectory

// Try generating wrappers for Engine methods (now that package name conflict is resolved)
//go:generate impgen syncengine.Engine.Analyze
//go:generate impgen syncengine.Engine.Sync
//go:generate impgen syncengine.Engine.GetStatus

// Generate mocks for FileSystem and FileScanner interfaces
//go:generate impgen filesystem.FileSystem
//go:generate impgen filesystem.FileScanner

func TestNewEngine(t *testing.T) {
	t.Parallel()

	// Use imptest wrapper for NewEngine
	wrapper := NewNewEngineImp(t, syncengine.NewEngine)
	wrapper.Start("/source", "/dest").ExpectReturnedValuesShould(
		And(
			Not(BeNil()),
			HaveField("SourcePath", Equal("/source")),
			HaveField("DestPath", Equal("/dest")),
			HaveField("Status", Not(BeNil())),
		),
	)
}

func TestEngineAnalyze(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()

	// Set up test data
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	// Create source directory with test files
	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), baseTime.Add(-1*time.Hour))
	mockFS.AddFile(filepath.Join(srcDir, "file2.txt"), []byte("content2"), baseTime.Add(-2*time.Hour))
	mockFS.AddDir(filepath.Join(srcDir, "subdir"), baseTime)
	mockFS.AddFile(filepath.Join(srcDir, "subdir/file3.txt"), []byte("content3"), baseTime.Add(-3*time.Hour))

	// Create empty destination directory
	mockFS.AddDir(dstDir, baseTime)

	// Create engine with mock filesystem
	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil) // Should succeed with no error

	// Verify engine status using gomega matchers
	expectedBytes := int64(len("content1") + len("content2") + len("content3"))
	NewWithT(t).Expect(engine.Status).Should(And(
		HaveField("TotalFiles", Equal(3)),
		HaveField("FilesToSync", HaveLen(3)),
		HaveField("TotalBytes", Equal(expectedBytes)),
	))
}

func TestEngineAnalyzeWithExistingFiles(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()

	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	// Create identical file in both directories with same modtime
	identicalFile := "identical.txt"
	identicalContent := []byte("same content")
	modTime := baseTime.Add(-1 * time.Hour)

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)
	mockFS.AddFile(filepath.Join(srcDir, identicalFile), identicalContent, modTime)
	mockFS.AddFile(filepath.Join(dstDir, identicalFile), identicalContent, modTime)

	// Create a different file in source only
	differentFile := "different.txt"
	mockFS.AddFile(filepath.Join(srcDir, differentFile), []byte("new content"), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should only need to sync the different file
	NewWithT(t).Expect(engine.Status).Should(HaveField("TotalFiles", Equal(1)))
}

func TestEngineSync(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()

	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	// Create test files
	testFiles := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	}

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)
	for path, content := range testFiles {
		mockFS.AddFile(filepath.Join(srcDir, path), []byte(content), baseTime)
	}

	// Create engine
	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify all files were synced
	g := NewWithT(t)
	g.Expect(engine.Status).Should(HaveField("ProcessedFiles", Equal(2)))

	// Verify files exist in destination
	for path, expectedContent := range testFiles {
		dstPath := filepath.Join(dstDir, path)
		content, _, err := mockFS.GetFile(dstPath)
		g.Expect(err).Should(BeNil(), "Failed to read destination file %s", path)
		g.Expect(string(content)).Should(Equal(expectedContent), "Content mismatch for %s", path)
	}
}

func TestEngineDeleteExtraFiles(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()

	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	// Create file in source
	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)
	mockFS.AddFile(filepath.Join(srcDir, "keep.txt"), []byte("keep this"), baseTime)

	// Create extra file in destination
	extraFile := filepath.Join(dstDir, "delete.txt")
	mockFS.AddFile(extraFile, []byte("delete this"), baseTime)

	// Analyze should delete the extra file
	// Use FluctuatingCount mode to ensure full scan (not monotonic optimization)
	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount
	engine.FileOps = fileops.NewFileOps(mockFS)
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Extra file should be deleted
	NewWithT(t).Expect(mockFS.Exists(extraFile)).Should(BeFalse(), "Expected extra file to be deleted")
}

func TestEngineStatusCallback(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()

	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	// Create a test file
	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)
	mockFS.AddFile(filepath.Join(srcDir, "test.txt"), []byte("test content"), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.FileOps = fileops.NewFileOps(mockFS)

	var callbackCount int
	engine.RegisterStatusCallback(func(status *syncengine.Status) {
		callbackCount++
		if status == nil {
			t.Error("Expected non-nil status")
		}
	})

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Callback should have been called multiple times
	if callbackCount == 0 {
		t.Error("Expected callback to be called")
	}
}

func TestGetStatus(t *testing.T) {
	t.Parallel()

	// Use imptest wrapper for NewEngine
	wrapper := NewNewEngineImp(t, syncengine.NewEngine)
	wrapper.Start("/source", "/dest").ExpectReturnedValuesShould(
		And(
			Not(BeNil()),
			WithTransform(func(e *syncengine.Engine) *syncengine.Status {
				return e.GetStatus()
			}, And(
				HaveField("TotalFiles", Equal(0)),
				HaveField("TotalBytes", Equal(int64(0))),
			)),
		),
	)
}

// TestMonotonicCount_SameCountSameFiles tests that when file counts match,
// monotonic-count mode assumes everything is fine and skips detailed scanning
func TestMonotonicCount_SameCountSameFiles(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()

	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	// Create identical files in both directories
	testFiles := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
		"file3.txt": "content3",
	}

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	for path, content := range testFiles {
		srcPath := filepath.Join(srcDir, path)
		dstPath := filepath.Join(dstDir, path)

		mockFS.AddFile(srcPath, []byte(content), baseTime)
		mockFS.AddFile(dstPath, []byte(content), baseTime)
	}

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.MonotonicCount
	engine.FileOps = fileops.NewFileOps(mockFS)
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// With monotonic-count mode and same file count, should assume everything is fine
	// and skip sync (this is the optimization - we trust that same count = same files)
	if engine.Status.TotalFiles != 0 {
		t.Errorf("Expected 0 files to sync (monotonic-count optimization), got %d", engine.Status.TotalFiles)
	}

	// Verify that TotalFilesInSource was still counted (we need to count to know if counts match)
	if engine.Status.TotalFilesInSource != 3 {
		t.Errorf("Expected 3 total files in source, got %d", engine.Status.TotalFilesInSource)
	}
}

// TestMonotonicCount_SameCountDifferentContent tests that monotonic-count mode
// will INCORRECTLY skip sync when counts match but content differs
// This demonstrates the trade-off: speed for correctness
func TestMonotonicCount_SameCountDifferentContent(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()

	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	// Create files with same names but different content
	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)
	mockFS.AddFile(filepath.Join(srcDir, "file1.txt"), []byte("new content"), baseTime)
	mockFS.AddFile(filepath.Join(dstDir, "file1.txt"), []byte("old content"), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.MonotonicCount
	engine.FileOps = fileops.NewFileOps(mockFS)
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Monotonic-count mode should skip sync because counts match (1 file in each)
	// This is the trade-off: we assume same count = same files
	if engine.Status.TotalFiles != 0 {
		t.Errorf("Expected 0 files to sync (monotonic-count assumes same count = same files), got %d", engine.Status.TotalFiles)
	}
}

// TestMonotonicCount_SameCountDifferentFiles tests that when file counts match but files differ,
// monotonic-count mode INCORRECTLY skips sync (this is the trade-off for speed)
func TestMonotonicCount_SameCountDifferentFiles(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()

	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	// Create different files with same count
	srcFiles := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
		"file3.txt": "content3",
	}

	dstFiles := map[string]string{
		"fileA.txt": "contentA",
		"fileB.txt": "contentB",
		"fileC.txt": "contentC",
	}

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	for path, content := range srcFiles {
		mockFS.AddFile(filepath.Join(srcDir, path), []byte(content), baseTime)
	}

	for path, content := range dstFiles {
		mockFS.AddFile(filepath.Join(dstDir, path), []byte(content), baseTime)
	}

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.MonotonicCount
	engine.FileOps = fileops.NewFileOps(mockFS)
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Monotonic-count mode assumes same count = same files, so it skips sync
	// This is the trade-off: speed for correctness
	if engine.Status.TotalFiles != 0 {
		t.Errorf("Expected 0 files to sync (monotonic-count assumes same count = same files), got %d", engine.Status.TotalFiles)
	}

	// Verify old destination files still exist (not deleted because we skipped sync)
	for path := range dstFiles {
		dstPath := filepath.Join(dstDir, path)
		if !mockFS.Exists(dstPath) {
			t.Errorf("Expected file %s to still exist in destination (sync was skipped)", path)
		}
	}
}

// TestMonotonicCount_DifferentCount tests that when file counts differ, sync is performed
func TestMonotonicCount_DifferentCount(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create more files in source than destination
	srcFiles := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
		"file3.txt": "content3",
	}

	dstFiles := map[string]string{
		"file1.txt": "content1",
	}

	for path, content := range srcFiles {
		srcPath := filepath.Join(srcDir, path)
		mockFS.AddFile(srcPath, []byte(content), baseTime)
	}

	for path, content := range dstFiles {
		dstPath := filepath.Join(dstDir, path)
		mockFS.AddFile(dstPath, []byte(content), baseTime)

	}

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.MonotonicCount
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should sync the 2 missing files
	if engine.Status.TotalFiles != 2 {
		t.Errorf("Expected 2 files to sync, got %d", engine.Status.TotalFiles)
	}
}

// TestFluctuatingCount_AddedFiles tests that files added to source are copied to destination
func TestFluctuatingCount_AddedFiles(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create files in source
	srcFiles := map[string]string{
		"existing.txt": "existing content",
		"new1.txt":     "new content 1",
		"new2.txt":     "new content 2",
	}

	// Only one file exists in destination
	dstFiles := map[string]string{
		"existing.txt": "existing content",
	}

	for path, content := range srcFiles {
		mockFS.AddFile(filepath.Join(srcDir, path), []byte(content), baseTime)
	}

	for path, content := range dstFiles {
		mockFS.AddFile(filepath.Join(dstDir, path), []byte(content), baseTime)
	}

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 2 new files to sync
	if engine.Status.TotalFiles != 2 {
		t.Errorf("Expected 2 files to sync, got %d", engine.Status.TotalFiles)
	}

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify new files were copied
	for path := range srcFiles {
		dstPath := filepath.Join(dstDir, path)
		if !mockFS.Exists(dstPath) {
			t.Errorf("Expected file %s to exist in destination", path)
		}
	}
}

// TestFluctuatingCount_RemovedFiles tests that files removed from source are deleted from destination
func TestFluctuatingCount_RemovedFiles(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Only one file in source
	srcFiles := map[string]string{
		"keep.txt": "keep this",
	}

	// Multiple files in destination
	dstFiles := map[string]string{
		"keep.txt":    "keep this",
		"delete1.txt": "delete this",
		"delete2.txt": "delete this too",
	}

	for path, content := range srcFiles {
		mockFS.AddFile(filepath.Join(srcDir, path), []byte(content), baseTime)
	}

	for path, content := range dstFiles {
		mockFS.AddFile(filepath.Join(dstDir, path), []byte(content), baseTime)
	}

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Perform sync (Analyze doesn't delete, Sync does)
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify deleted files are gone
	if mockFS.Exists(filepath.Join(dstDir, "delete1.txt")) {
		t.Error("Expected delete1.txt to be deleted from destination")
	}
	if mockFS.Exists(filepath.Join(dstDir, "delete2.txt")) {
		t.Error("Expected delete2.txt to be deleted from destination")
	}

	// Verify kept file still exists
	if !mockFS.Exists(filepath.Join(dstDir, "keep.txt")) {
		t.Error("Expected keep.txt to still exist in destination")
	}
}

// TestFluctuatingCount_BothAddedAndRemoved tests handling both adds and removes in one sync
func TestFluctuatingCount_BothAddedAndRemoved(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Source has some files
	srcFiles := map[string]string{
		"keep.txt": "keep this",
		"new.txt":  "new file",
	}

	// Destination has different files
	dstFiles := map[string]string{
		"keep.txt":   "keep this",
		"remove.txt": "remove this",
	}

	for path, content := range srcFiles {
		mockFS.AddFile(filepath.Join(srcDir, path), []byte(content), baseTime)
	}

	for path, content := range dstFiles {
		mockFS.AddFile(filepath.Join(dstDir, path), []byte(content), baseTime)
	}

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 1 file to sync (new.txt)
	if engine.Status.TotalFiles != 1 {
		t.Errorf("Expected 1 file to sync, got %d", engine.Status.TotalFiles)
	}

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify new file was added
	if !mockFS.Exists(filepath.Join(dstDir, "new.txt")) {
		t.Error("Expected new.txt to be copied to destination")
	}

	// Verify removed file is gone
	if mockFS.Exists(filepath.Join(dstDir, "remove.txt")) {
		t.Error("Expected remove.txt to be deleted from destination")
	}

	// Verify kept file still exists
	if !mockFS.Exists(filepath.Join(dstDir, "keep.txt")) {
		t.Error("Expected keep.txt to still exist in destination")
	}
}

// TestFluctuatingCount_SameFiles tests that when files match, nothing is synced
func TestFluctuatingCount_SameFiles(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Same files in both
	files := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	}

	for path, content := range files {
		mockFS.AddFile(filepath.Join(srcDir, path), []byte(content), baseTime)
		mockFS.AddFile(filepath.Join(dstDir, path), []byte(content), baseTime)
	}

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 0 files to sync
	if engine.Status.TotalFiles != 0 {
		t.Errorf("Expected 0 files to sync, got %d", engine.Status.TotalFiles)
	}
}

// TestContent_ModifiedFile tests that files with different content are copied
func TestContent_ModifiedFile(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file in both locations with different content
	// Source has newer modtime to trigger Content mode check
	srcContent := "new content"
	dstContent := "old content"

	srcTime := baseTime
	dstTime := baseTime.Add(-1 * time.Hour)  // Older modtime

	mockFS.AddFile(filepath.Join(srcDir, "file.txt"), []byte(srcContent), srcTime)
	mockFS.AddFile(filepath.Join(dstDir, "file.txt"), []byte(dstContent), dstTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.Content
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 1 file to sync
	if engine.Status.TotalFiles != 1 {
		t.Errorf("Expected 1 file to sync, got %d", engine.Status.TotalFiles)
	}

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify content was updated
	content, _, err := mockFS.GetFile(filepath.Join(dstDir, "file.txt"))
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}
	if string(content) != srcContent {
		t.Errorf("Expected content %q, got %q", srcContent, string(content))
	}
}

// TestContent_TouchedFile tests that files with same content but different modtime only update modtime
func TestContent_TouchedFile(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file with same content in both locations
	content := "same content"

	srcPath := filepath.Join(srcDir, "file.txt")
	dstPath := filepath.Join(dstDir, "file.txt")

	// Add source file with baseTime
	mockFS.AddFile(srcPath, []byte(content), baseTime)

	// Add dest file with older modtime
	oldTime := baseTime.Add(-1 * time.Hour)
	mockFS.AddFile(dstPath, []byte(content), oldTime)
	initialModTime := oldTime

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.Content
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 1 file to sync (modtime differs)
	if engine.Status.TotalFiles != 1 {
		t.Errorf("Expected 1 file to sync, got %d", engine.Status.TotalFiles)
	}

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify content is still the same
	newContent, _, err := mockFS.GetFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}
	if string(newContent) != content {
		t.Errorf("Expected content %q, got %q", content, string(newContent))
	}

	// Verify modtime was updated
	_, newModTime, err := mockFS.GetFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to get dest file: %v", err)
	}

	if newModTime.Equal(initialModTime) {
		t.Error("Expected modtime to be updated, but it wasn't")
	}

	// Modtime should now match source (baseTime)
	timeDiff := newModTime.Sub(baseTime).Abs()
	if timeDiff > time.Second {
		t.Errorf("Expected modtime to match source (diff < 1s), got diff of %v", timeDiff)
	}
}

// TestContent_NewFile tests that new files are copied
func TestContent_NewFile(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file only in source
	mockFS.AddFile(filepath.Join(srcDir, "new.txt"), []byte("new content"), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.Content
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 1 file to sync
	if engine.Status.TotalFiles != 1 {
		t.Errorf("Expected 1 file to sync, got %d", engine.Status.TotalFiles)
	}

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify file was copied
	if !mockFS.Exists(filepath.Join(dstDir, "new.txt")) {
		t.Error("Expected new.txt to be copied to destination")
	}
}

// TestContent_SameFile tests that files with same modtime are not synced
func TestContent_SameFile(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file in both locations
	content := "same content"
	srcPath := filepath.Join(srcDir, "file.txt")
	dstPath := filepath.Join(dstDir, "file.txt")

	// Add files with same content and modtime
	mockFS.AddFile(srcPath, []byte(content), baseTime)
	mockFS.AddFile(dstPath, []byte(content), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.Content
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 0 files to sync
	if engine.Status.TotalFiles != 0 {
		t.Errorf("Expected 0 files to sync, got %d", engine.Status.TotalFiles)
	}
}

// TestContent_DeletedFile tests that deleted files are removed from destination
func TestContent_DeletedFile(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file only in destination
	mockFS.AddFile(filepath.Join(dstDir, "deleted.txt"), []byte("old content"), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.Content
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify file was deleted
	if mockFS.Exists(filepath.Join(dstDir, "deleted.txt")) {
		t.Error("Expected deleted.txt to be removed from destination")
	}
}

// TestDeviousContent_ModifiedWithSameModtime tests that files modified with matching modtime are detected
func TestDeviousContent_ModifiedWithSameModtime(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file with different content in both locations
	srcContent := "malicious new content"
	dstContent := "original content"

	srcPath := filepath.Join(srcDir, "file.txt")
	dstPath := filepath.Join(dstDir, "file.txt")

	mockFS.AddFile(srcPath, []byte(srcContent), baseTime)
	mockFS.AddFile(dstPath, []byte(dstContent), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.DeviousContent
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should detect the content difference despite matching modtime
	if engine.Status.TotalFiles != 1 {
		t.Errorf("Expected 1 file to sync, got %d", engine.Status.TotalFiles)
	}

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify content was updated
	content, _, err := mockFS.GetFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}
	if string(content) != srcContent {
		t.Errorf("Expected content %q, got %q", srcContent, string(content))
	}
}

// TestDeviousContent_SameContent tests that files with same content are not synced
func TestDeviousContent_SameContent(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file with same content in both locations
	content := "same content"

	srcPath := filepath.Join(srcDir, "file.txt")
	dstPath := filepath.Join(dstDir, "file.txt")

	// Add files with same content but different modtimes (shouldn't matter for DeviousContent mode)
	mockFS.AddFile(srcPath, []byte(content), baseTime)
	oldTime := baseTime.Add(-1 * time.Hour)
	mockFS.AddFile(dstPath, []byte(content), oldTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.DeviousContent
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 0 files to sync (content is the same)
	if engine.Status.TotalFiles != 0 {
		t.Errorf("Expected 0 files to sync, got %d", engine.Status.TotalFiles)
	}
}

// TestDeviousContent_DifferentContent tests that files with different content are synced
func TestDeviousContent_DifferentContent(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file with different content
	srcContent := "new content"
	dstContent := "old content"

	mockFS.AddFile(filepath.Join(srcDir, "file.txt"), []byte(srcContent), baseTime)
	mockFS.AddFile(filepath.Join(dstDir, "file.txt"), []byte(dstContent), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.DeviousContent
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 1 file to sync
	if engine.Status.TotalFiles != 1 {
		t.Errorf("Expected 1 file to sync, got %d", engine.Status.TotalFiles)
	}

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify content was updated
	content, _, err := mockFS.GetFile(filepath.Join(dstDir, "file.txt"))
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}
	if string(content) != srcContent {
		t.Errorf("Expected content %q, got %q", srcContent, string(content))
	}
}

// TestDeviousContent_NewFile tests that new files are copied
func TestDeviousContent_NewFile(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file only in source
	mockFS.AddFile(filepath.Join(srcDir, "new.txt"), []byte("new content"), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.DeviousContent
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 1 file to sync
	if engine.Status.TotalFiles != 1 {
		t.Errorf("Expected 1 file to sync, got %d", engine.Status.TotalFiles)
	}

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify file was copied
	if !mockFS.Exists(filepath.Join(dstDir, "new.txt")) {
		t.Error("Expected new.txt to be copied to destination")
	}
}

// TestDeviousContent_DeletedFile tests that deleted files are removed from destination
func TestDeviousContent_DeletedFile(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file only in destination
	mockFS.AddFile(filepath.Join(dstDir, "deleted.txt"), []byte("old content"), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.DeviousContent
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify file was deleted
	if mockFS.Exists(filepath.Join(dstDir, "deleted.txt")) {
		t.Error("Expected deleted.txt to be removed from destination")
	}
}

// TestParanoid_SameContent tests that files with identical bytes are not synced
func TestParanoid_SameContent(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file with same content in both locations
	content := "identical byte-by-byte content"

	srcPath := filepath.Join(srcDir, "file.txt")
	dstPath := filepath.Join(dstDir, "file.txt")

	// Add files with same content but different modtimes (shouldn't matter for Paranoid mode)
	mockFS.AddFile(srcPath, []byte(content), baseTime)
	oldTime := baseTime.Add(-2 * time.Hour)
	mockFS.AddFile(dstPath, []byte(content), oldTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.Paranoid
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 0 files to sync (bytes are identical)
	if engine.Status.TotalFiles != 0 {
		t.Errorf("Expected 0 files to sync, got %d", engine.Status.TotalFiles)
	}
}

// TestParanoid_DifferentContent tests that files with different bytes are synced
func TestParanoid_DifferentContent(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file with different content
	srcContent := "source content here"
	dstContent := "destination content"

	mockFS.AddFile(filepath.Join(srcDir, "file.txt"), []byte(srcContent), baseTime)
	mockFS.AddFile(filepath.Join(dstDir, "file.txt"), []byte(dstContent), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.Paranoid
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 1 file to sync
	if engine.Status.TotalFiles != 1 {
		t.Errorf("Expected 1 file to sync, got %d", engine.Status.TotalFiles)
	}

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify content was updated
	content, _, err := mockFS.GetFile(filepath.Join(dstDir, "file.txt"))
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}
	if string(content) != srcContent {
		t.Errorf("Expected content %q, got %q", srcContent, string(content))
	}
}

// TestParanoid_NewFile tests that new files are copied
func TestParanoid_NewFile(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file only in source
	mockFS.AddFile(filepath.Join(srcDir, "new.txt"), []byte("brand new file"), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.Paranoid
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 1 file to sync
	if engine.Status.TotalFiles != 1 {
		t.Errorf("Expected 1 file to sync, got %d", engine.Status.TotalFiles)
	}

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify file was copied
	if !mockFS.Exists(filepath.Join(dstDir, "new.txt")) {
		t.Error("Expected new.txt to be copied to destination")
	}
}

// TestParanoid_DeletedFile tests that deleted files are removed from destination
func TestParanoid_DeletedFile(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file only in destination
	mockFS.AddFile(filepath.Join(dstDir, "deleted.txt"), []byte("to be deleted"), baseTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.Paranoid
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrappers for Analyze and Sync methods
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Perform sync
	syncWrapper := NewEngineSync(t, engine.Sync)
	syncWrapper.Start().ExpectReturnedValuesAre(nil)

	// Verify file was deleted
	if mockFS.Exists(filepath.Join(dstDir, "deleted.txt")) {
		t.Error("Expected deleted.txt to be removed from destination")
	}
}

// TestParanoid_DifferentModtime tests that files with same bytes but different modtime are not synced
func TestParanoid_DifferentModtime(t *testing.T) {
	t.Parallel()

	// Create mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	baseTime := time.Now()
	srcDir := "/source"
	dstDir := "/dest"

	mockFS.AddDir(srcDir, baseTime)
	mockFS.AddDir(dstDir, baseTime)

	// Create file with same content
	content := "exact same bytes"

	srcPath := filepath.Join(srcDir, "file.txt")
	dstPath := filepath.Join(dstDir, "file.txt")

	// Add files with same content but very different modtimes
	mockFS.AddFile(srcPath, []byte(content), baseTime)
	veryOldTime := baseTime.Add(-24 * time.Hour)
	mockFS.AddFile(dstPath, []byte(content), veryOldTime)

	engine := syncengine.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.Paranoid
	engine.FileOps = fileops.NewFileOps(mockFS)

	// Use imptest wrapper for Analyze method
	analyzeWrapper := NewEngineAnalyze(t, engine.Analyze)
	analyzeWrapper.Start().ExpectReturnedValuesAre(nil)

	// Should find 0 files to sync (bytes are identical, modtime doesn't matter)
	if engine.Status.TotalFiles != 0 {
		t.Errorf("Expected 0 files to sync, got %d", engine.Status.TotalFiles)
	}
}

// Unit tests using imptest for fileops functions

// TestFileopsComputeFileHash tests the ComputeFileHash function using imptest
func TestFileopsComputeFileHash(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content for hashing")

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Use imptest wrapper for ComputeFileHash
	wrapper := NewComputeFileHashImp(t, fileops.ComputeFileHash)
	wrapper.Start(testFile).ExpectReturnedValuesShould(
		And(
			Not(BeEmpty()),
			HaveLen(64), // SHA256 hash should be 64 hex characters
		),
		BeNil(), // no error expected
	)

	// Verify hash is consistent
	wrapper2 := NewComputeFileHashImp(t, fileops.ComputeFileHash)
	wrapper2.Start(testFile).ExpectReturnedValuesShould(
		Equal(wrapper.Returned.Result0), // Should match first hash
		BeNil(),
	)
}

// TestFileopsCompareFilesBytes tests the CompareFilesBytes function using imptest
func TestFileopsCompareFilesBytes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	file3 := filepath.Join(tmpDir, "file3.txt")

	content1 := []byte("identical content")
	content2 := []byte("different content")

	if err := os.WriteFile(file1, content1, 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, content1, 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}
	if err := os.WriteFile(file3, content2, 0644); err != nil {
		t.Fatalf("Failed to write file3: %v", err)
	}

	// Test identical files
	wrapper1 := NewCompareFilesBytesImp(t, fileops.CompareFilesBytes)
	wrapper1.Start(file1, file2).ExpectReturnedValuesShould(
		BeTrue(), // files should be identical
		BeNil(),  // no error expected
	)

	// Test different files
	wrapper2 := NewCompareFilesBytesImp(t, fileops.CompareFilesBytes)
	wrapper2.Start(file1, file3).ExpectReturnedValuesShould(
		BeFalse(), // files should be different
		BeNil(),
	)
}

// TestFileopsCountFiles tests the CountFiles function using imptest
func TestFileopsCountFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create test files
	testFiles := []string{
		"file1.txt",
		"file2.txt",
		"subdir/file3.txt",
		"subdir/file4.txt",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Use imptest wrapper for CountFiles
	wrapper := NewCountFilesImp(t, fileops.CountFiles)
	wrapper.Start(tmpDir).ExpectReturnedValuesShould(
		Equal(5), // Should count 4 files + 1 subdir = 5 total
		BeNil(),  // no error expected
	)
}

// TestFileopsScanDirectory tests the ScanDirectory function using imptest
func TestFileopsScanDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"file1.txt":        "content1",
		"file2.txt":        "content2",
		"subdir/file3.txt": "content3",
	}

	for file, content := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Use imptest wrapper for ScanDirectory
	wrapper := NewScanDirectoryImp(t, fileops.ScanDirectory)
	wrapper.Start(tmpDir).ExpectReturnedValuesShould(
		And(
			HaveLen(4), // Should find 4 entries (3 files + 1 subdir)
			HaveKey("file1.txt"),
			HaveKey("file2.txt"),
			HaveKey("subdir"),
			HaveKey("subdir/file3.txt"),
		),
		BeNil(), // no error expected
	)
}

