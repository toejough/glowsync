package sync_test

import (
	"os"
	"path/filepath"
	"testing"

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
	engine := sync.NewEngine(srcDir, dstDir)
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

