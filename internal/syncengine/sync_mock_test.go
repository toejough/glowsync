package syncengine_test

import (
	"testing"
	"time"

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/pkg/fileops"
	"github.com/joe/copy-files/pkg/filesystem"
)

// TestEngine_WithMockFS demonstrates testing the sync engine without real filesystem I/O
func TestEngine_WithMockFS_SimpleSync(t *testing.T) {
	// Create a mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	// Set up source directory with test files
	baseTime := time.Now()
	mockFS.AddDir("source", baseTime)
	mockFS.AddFile("source/file1.txt", []byte("content1"), baseTime.Add(-1*time.Hour))
	mockFS.AddFile("source/file2.txt", []byte("content2"), baseTime.Add(-2*time.Hour))
	mockFS.AddDir("source/subdir", baseTime)
	mockFS.AddFile("source/subdir/file3.txt", []byte("content3"), baseTime.Add(-3*time.Hour))
	
	// Set up empty destination directory
	mockFS.AddDir("dest", baseTime)
	
	// Create engine with mock filesystem
	engine := syncengine.NewEngine("source", "dest")
	engine.FileOps = fileops.NewFileOps(mockFS)
	engine.ChangeType = config.FluctuatingCount // Use FluctuatingCount for simplicity
	
	// Analyze
	err := engine.Analyze()
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	
	// Verify analysis results
	status := engine.GetStatus()
	if status.TotalFiles != 3 { // file1.txt, file2.txt, subdir/file3.txt
		t.Errorf("Expected 3 files to sync, got %d", status.TotalFiles)
	}
	
	// Sync
	err = engine.Sync()
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	
	// Verify files were copied
	expectedFiles := []string{
		"dest/file1.txt",
		"dest/file2.txt",
		"dest/subdir/file3.txt",
	}
	
	for _, path := range expectedFiles {
		if !mockFS.Exists(path) {
			t.Errorf("Expected file %s to exist after sync", path)
		}
	}
	
	// Verify content
	content, _, err := mockFS.GetFile("dest/file1.txt")
	if err != nil {
		t.Fatalf("Failed to get file: %v", err)
	}
	if string(content) != "content1" {
		t.Errorf("Expected content1, got %s", string(content))
	}
}

// TestEngine_WithMockFS_ModifiedFile demonstrates detecting modified files
func TestEngine_WithMockFS_ModifiedFile(t *testing.T) {
	mockFS := filesystem.NewMockFileSystem()
	
	// Set up source and dest with same file but different content
	baseTime := time.Now()
	mockFS.AddDir("source", baseTime)
	mockFS.AddFile("source/file.txt", []byte("new content"), baseTime)
	
	mockFS.AddDir("dest", baseTime)
	mockFS.AddFile("dest/file.txt", []byte("old content"), baseTime.Add(-1*time.Hour))
	
	// Create engine
	engine := syncengine.NewEngine("source", "dest")
	engine.FileOps = fileops.NewFileOps(mockFS)
	engine.ChangeType = config.Content // Use Content mode to detect changes
	
	// Analyze
	err := engine.Analyze()
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	
	// Should detect 1 file needs syncing
	status := engine.GetStatus()
	if status.TotalFiles != 1 {
		t.Errorf("Expected 1 file to sync, got %d", status.TotalFiles)
	}
	
	// Sync
	err = engine.Sync()
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	
	// Verify content was updated
	content, _, err := mockFS.GetFile("dest/file.txt")
	if err != nil {
		t.Fatalf("Failed to get file: %v", err)
	}
	if string(content) != "new content" {
		t.Errorf("Expected 'new content', got %s", string(content))
	}
}

// TestEngine_WithMockFS_NoChanges demonstrates detecting when no sync is needed
func TestEngine_WithMockFS_NoChanges(t *testing.T) {
	mockFS := filesystem.NewMockFileSystem()
	
	// Set up identical source and dest
	baseTime := time.Now()
	content := []byte("same content")
	
	mockFS.AddDir("source", baseTime)
	mockFS.AddFile("source/file.txt", content, baseTime)
	
	mockFS.AddDir("dest", baseTime)
	mockFS.AddFile("dest/file.txt", content, baseTime)
	
	// Create engine
	engine := syncengine.NewEngine("source", "dest")
	engine.FileOps = fileops.NewFileOps(mockFS)
	engine.ChangeType = config.Content
	
	// Analyze
	err := engine.Analyze()
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	
	// Should detect 0 files need syncing
	status := engine.GetStatus()
	if status.TotalFiles != 0 {
		t.Errorf("Expected 0 files to sync, got %d", status.TotalFiles)
	}
	
	// Sync should be a no-op
	err = engine.Sync()
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	
	// Verify file wasn't modified
	_, modTime, err := mockFS.GetFile("dest/file.txt")
	if err != nil {
		t.Fatalf("Failed to get file: %v", err)
	}
	if !modTime.Equal(baseTime) {
		t.Errorf("File modtime should not have changed")
	}
}

