package sync_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/sync"
)

func TestNewEngine(t *testing.T) {
	t.Parallel()

	engine := sync.NewEngine("/source", "/dest")

	if engine == nil {
		t.Fatal("Expected engine to not be nil")
	}
	if engine.SourcePath != "/source" {
		t.Errorf("Expected SourcePath to be '/source', got '%s'", engine.SourcePath)
	}
	if engine.DestPath != "/dest" {
		t.Errorf("Expected DestPath to be '/dest', got '%s'", engine.DestPath)
	}
	if engine.Status == nil {
		t.Fatal("Expected Status to not be nil")
	}
}

func TestEngineAnalyze(t *testing.T) {
	t.Parallel()

	// Create temporary directories
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create test files in source
	testFiles := map[string]string{
		"file1.txt":        "content1",
		"file2.txt":        "content2",
		"subdir/file3.txt": "content3",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(srcDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Create engine and analyze
	engine := sync.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount // Use full scan mode

	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should find all files need syncing
	expectedBytes := int64(len("content1") + len("content2") + len("content3"))
	if engine.Status.TotalFiles != 3 {
		t.Errorf("Expected 3 total files, got %d", engine.Status.TotalFiles)
	}
	if len(engine.Status.FilesToSync) != 3 {
		t.Errorf("Expected 3 files to sync, got %d", len(engine.Status.FilesToSync))
	}
	if engine.Status.TotalBytes != expectedBytes {
		t.Errorf("Expected %d total bytes, got %d", expectedBytes, engine.Status.TotalBytes)
	}
}

func TestEngineAnalyzeWithExistingFiles(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create identical file in both directories
	identicalFile := "identical.txt"
	identicalContent := []byte("same content")

	srcPath := filepath.Join(srcDir, identicalFile)
	dstPath := filepath.Join(dstDir, identicalFile)

	if err := os.WriteFile(srcPath, identicalContent, 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}
	if err := os.WriteFile(dstPath, identicalContent, 0644); err != nil {
		t.Fatalf("Failed to write dest file: %v", err)
	}

	// Make sure mod times are the same
	info, err := os.Stat(srcPath)
	if err != nil {
		t.Fatalf("Failed to stat source file: %v", err)
	}
	if err := os.Chtimes(dstPath, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("Failed to set mod time: %v", err)
	}

	// Create a different file in source
	differentFile := "different.txt"
	if err := os.WriteFile(filepath.Join(srcDir, differentFile), []byte("new content"), 0644); err != nil {
		t.Fatalf("Failed to write different file: %v", err)
	}

	engine := sync.NewEngine(srcDir, dstDir)
	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should only need to sync the different file
	if engine.Status.TotalFiles != 1 {
		t.Errorf("Expected 1 file to sync, got %d", engine.Status.TotalFiles)
	}
}

func TestEngineSync(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(srcDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Create engine, analyze, and sync
	engine := sync.NewEngine(srcDir, dstDir)
	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if err := engine.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify all files were synced
	if engine.Status.ProcessedFiles != 2 {
		t.Errorf("Expected 2 processed files, got %d", engine.Status.ProcessedFiles)
	}

	// Verify files exist in destination
	for path, expectedContent := range testFiles {
		dstPath := filepath.Join(dstDir, path)
		content, err := os.ReadFile(dstPath)
		if err != nil {
			t.Fatalf("Failed to read destination file %s: %v", path, err)
		}
		if string(content) != expectedContent {
			t.Errorf("Content mismatch for %s: expected %q, got %q", path, expectedContent, string(content))
		}
	}
}

func TestEngineDeleteExtraFiles(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create file in source
	srcFile := filepath.Join(srcDir, "keep.txt")
	if err := os.WriteFile(srcFile, []byte("keep this"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Create extra file in destination
	extraFile := filepath.Join(dstDir, "delete.txt")
	if err := os.WriteFile(extraFile, []byte("delete this"), 0644); err != nil {
		t.Fatalf("Failed to write extra file: %v", err)
	}

	// Analyze should delete the extra file
	// Use FluctuatingCount mode to ensure full scan (not monotonic optimization)
	engine := sync.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount
	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Extra file should be deleted
	if _, err := os.Stat(extraFile); !os.IsNotExist(err) {
		t.Error("Expected extra file to be deleted")
	}
}

func TestEngineStatusCallback(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create a test file
	srcFile := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(srcFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	engine := sync.NewEngine(srcDir, dstDir)

	var callbackCount int
	engine.RegisterStatusCallback(func(status *sync.Status) {
		callbackCount++
		if status == nil {
			t.Error("Expected non-nil status")
		}
	})

	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if err := engine.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Callback should have been called multiple times
	if callbackCount == 0 {
		t.Error("Expected callback to be called")
	}
}

func TestGetStatus(t *testing.T) {
	t.Parallel()

	engine := sync.NewEngine("/source", "/dest")

	status := engine.GetStatus()
	if status.TotalFiles != 0 {
		t.Errorf("Expected 0 total files, got %d", status.TotalFiles)
	}
	if status.TotalBytes != 0 {
		t.Errorf("Expected 0 total bytes, got %d", status.TotalBytes)
	}
}

// TestMonotonicCount_SameCountSameFiles tests that when file counts match,
// monotonic-count mode assumes everything is fine and skips detailed scanning
func TestMonotonicCount_SameCountSameFiles(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create identical files in both directories
	testFiles := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
		"file3.txt": "content3",
	}

	for path, content := range testFiles {
		srcPath := filepath.Join(srcDir, path)
		dstPath := filepath.Join(dstDir, path)

		if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write source file: %v", err)
		}
		if err := os.WriteFile(dstPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write dest file: %v", err)
		}

		// Make sure mod times are the same
		info, err := os.Stat(srcPath)
		if err != nil {
			t.Fatalf("Failed to stat source file: %v", err)
		}
		if err := os.Chtimes(dstPath, info.ModTime(), info.ModTime()); err != nil {
			t.Fatalf("Failed to set mod time: %v", err)
		}
	}

	engine := sync.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.MonotonicCount

	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

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

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create files with same names but different content
	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("new content"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "file1.txt"), []byte("old content"), 0644); err != nil {
		t.Fatalf("Failed to write dest file: %v", err)
	}

	engine := sync.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.MonotonicCount

	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

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

	srcDir := t.TempDir()
	dstDir := t.TempDir()

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

	for path, content := range srcFiles {
		if err := os.WriteFile(filepath.Join(srcDir, path), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write source file: %v", err)
		}
	}

	for path, content := range dstFiles {
		if err := os.WriteFile(filepath.Join(dstDir, path), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write dest file: %v", err)
		}
	}

	engine := sync.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.MonotonicCount

	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Monotonic-count mode assumes same count = same files, so it skips sync
	// This is the trade-off: speed for correctness
	if engine.Status.TotalFiles != 0 {
		t.Errorf("Expected 0 files to sync (monotonic-count assumes same count = same files), got %d", engine.Status.TotalFiles)
	}

	// Verify old destination files still exist (not deleted because we skipped sync)
	for path := range dstFiles {
		dstPath := filepath.Join(dstDir, path)
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			t.Errorf("Expected file %s to still exist in destination (sync was skipped)", path)
		}
	}
}

// TestMonotonicCount_DifferentCount tests that when file counts differ, sync is performed
func TestMonotonicCount_DifferentCount(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

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
		if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write source file: %v", err)
		}
	}

	for path, content := range dstFiles {
		dstPath := filepath.Join(dstDir, path)
		if err := os.WriteFile(dstPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write dest file: %v", err)
		}

		// Make sure mod time matches source for file1.txt
		srcPath := filepath.Join(srcDir, path)
		info, err := os.Stat(srcPath)
		if err != nil {
			t.Fatalf("Failed to stat source file: %v", err)
		}
		if err := os.Chtimes(dstPath, info.ModTime(), info.ModTime()); err != nil {
			t.Fatalf("Failed to set mod time: %v", err)
		}
	}

	engine := sync.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.MonotonicCount

	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should sync the 2 missing files
	if engine.Status.TotalFiles != 2 {
		t.Errorf("Expected 2 files to sync, got %d", engine.Status.TotalFiles)
	}
}

// TestFluctuatingCount_AddedFiles tests that files added to source are copied to destination
func TestFluctuatingCount_AddedFiles(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

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
		if err := os.WriteFile(filepath.Join(srcDir, path), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write source file: %v", err)
		}
	}

	for path, content := range dstFiles {
		if err := os.WriteFile(filepath.Join(dstDir, path), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write dest file: %v", err)
		}
	}

	engine := sync.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount

	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should find 2 new files to sync
	if engine.Status.TotalFiles != 2 {
		t.Errorf("Expected 2 files to sync, got %d", engine.Status.TotalFiles)
	}

	// Perform sync
	if err := engine.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify new files were copied
	for path := range srcFiles {
		dstPath := filepath.Join(dstDir, path)
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist in destination", path)
		}
	}
}

// TestFluctuatingCount_RemovedFiles tests that files removed from source are deleted from destination
func TestFluctuatingCount_RemovedFiles(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

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
		if err := os.WriteFile(filepath.Join(srcDir, path), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write source file: %v", err)
		}
	}

	for path, content := range dstFiles {
		if err := os.WriteFile(filepath.Join(dstDir, path), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write dest file: %v", err)
		}
	}

	engine := sync.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount

	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Perform sync (Analyze doesn't delete, Sync does)
	if err := engine.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify deleted files are gone
	if _, err := os.Stat(filepath.Join(dstDir, "delete1.txt")); !os.IsNotExist(err) {
		t.Error("Expected delete1.txt to be deleted from destination")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "delete2.txt")); !os.IsNotExist(err) {
		t.Error("Expected delete2.txt to be deleted from destination")
	}

	// Verify kept file still exists
	if _, err := os.Stat(filepath.Join(dstDir, "keep.txt")); os.IsNotExist(err) {
		t.Error("Expected keep.txt to still exist in destination")
	}
}

// TestFluctuatingCount_BothAddedAndRemoved tests handling both adds and removes in one sync
func TestFluctuatingCount_BothAddedAndRemoved(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

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
		if err := os.WriteFile(filepath.Join(srcDir, path), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write source file: %v", err)
		}
	}

	for path, content := range dstFiles {
		if err := os.WriteFile(filepath.Join(dstDir, path), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write dest file: %v", err)
		}
	}

	engine := sync.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount

	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should find 1 file to sync (new.txt)
	if engine.Status.TotalFiles != 1 {
		t.Errorf("Expected 1 file to sync, got %d", engine.Status.TotalFiles)
	}

	// Perform sync
	if err := engine.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify new file was added
	if _, err := os.Stat(filepath.Join(dstDir, "new.txt")); os.IsNotExist(err) {
		t.Error("Expected new.txt to be copied to destination")
	}

	// Verify removed file is gone
	if _, err := os.Stat(filepath.Join(dstDir, "remove.txt")); !os.IsNotExist(err) {
		t.Error("Expected remove.txt to be deleted from destination")
	}

	// Verify kept file still exists
	if _, err := os.Stat(filepath.Join(dstDir, "keep.txt")); os.IsNotExist(err) {
		t.Error("Expected keep.txt to still exist in destination")
	}
}

// TestFluctuatingCount_SameFiles tests that when files match, nothing is synced
func TestFluctuatingCount_SameFiles(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Same files in both
	files := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	}

	for path, content := range files {
		if err := os.WriteFile(filepath.Join(srcDir, path), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write source file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dstDir, path), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write dest file: %v", err)
		}
	}

	engine := sync.NewEngine(srcDir, dstDir)
	engine.ChangeType = config.FluctuatingCount

	if err := engine.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should find 0 files to sync
	if engine.Status.TotalFiles != 0 {
		t.Errorf("Expected 0 files to sync, got %d", engine.Status.TotalFiles)
	}
}

