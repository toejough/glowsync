package fileops

import (
	"testing"
	"time"

	"github.com/joe/copy-files/pkg/filesystem"
)

// TestFileOps_ScanDirectory_WithMockFS demonstrates testing without real filesystem I/O
func TestFileOps_ScanDirectory_WithMockFS(t *testing.T) {
	// Create a mock filesystem
	mockFS := filesystem.NewMockFileSystem()
	
	// Set up test data in memory (no actual files created!)
	baseTime := time.Now()
	mockFS.AddDir("testdir", baseTime)
	mockFS.AddFile("testdir/file1.txt", []byte("content1"), baseTime.Add(-1*time.Hour))
	mockFS.AddFile("testdir/file2.txt", []byte("content2"), baseTime.Add(-2*time.Hour))
	mockFS.AddDir("testdir/subdir", baseTime)
	mockFS.AddFile("testdir/subdir/file3.txt", []byte("content3"), baseTime.Add(-3*time.Hour))
	
	// Create FileOps with mock filesystem
	fo := NewFileOps(mockFS)
	
	// Scan the directory
	files, err := fo.ScanDirectory("testdir")
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}
	
	// Verify results
	expectedFiles := []string{"file1.txt", "file2.txt", "subdir", "subdir/file3.txt"}
	if len(files) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d", len(expectedFiles), len(files))
	}
	
	for _, expected := range expectedFiles {
		if _, exists := files[expected]; !exists {
			t.Errorf("Expected file %s not found", expected)
		}
	}
	
	// Verify file details
	file1 := files["file1.txt"]
	if file1 == nil {
		t.Fatal("file1.txt not found")
	}
	if file1.Size != 8 { // len("content1")
		t.Errorf("Expected size 8, got %d", file1.Size)
	}
	if !file1.ModTime.Equal(baseTime.Add(-1 * time.Hour)) {
		t.Errorf("Expected modtime %v, got %v", baseTime.Add(-1*time.Hour), file1.ModTime)
	}
}

// TestFileOps_CountFiles_WithMockFS demonstrates counting files without real filesystem
func TestFileOps_CountFiles_WithMockFS(t *testing.T) {
	mockFS := filesystem.NewMockFileSystem()
	
	// Set up test data
	baseTime := time.Now()
	mockFS.AddDir("testdir", baseTime)
	mockFS.AddFile("testdir/file1.txt", []byte("content1"), baseTime)
	mockFS.AddFile("testdir/file2.txt", []byte("content2"), baseTime)
	mockFS.AddDir("testdir/subdir", baseTime)
	mockFS.AddFile("testdir/subdir/file3.txt", []byte("content3"), baseTime)
	
	fo := NewFileOps(mockFS)
	
	// Count files
	count, err := fo.CountFiles("testdir")
	if err != nil {
		t.Fatalf("CountFiles failed: %v", err)
	}
	
	// Should count: file1.txt, file2.txt, subdir, subdir/file3.txt = 4
	if count != 4 {
		t.Errorf("Expected 4 files, got %d", count)
	}
}

// TestFileOps_ComputeFileHash_WithMockFS demonstrates hash computation without real files
func TestFileOps_ComputeFileHash_WithMockFS(t *testing.T) {
	mockFS := filesystem.NewMockFileSystem()
	
	// Add a file with known content
	content := []byte("test content for hashing")
	mockFS.AddFile("testfile.txt", content, time.Now())
	
	fo := NewFileOps(mockFS)
	
	// Compute hash
	hash1, err := fo.ComputeFileHash("testfile.txt")
	if err != nil {
		t.Fatalf("ComputeFileHash failed: %v", err)
	}
	
	// Compute hash again - should be the same
	hash2, err := fo.ComputeFileHash("testfile.txt")
	if err != nil {
		t.Fatalf("ComputeFileHash failed: %v", err)
	}
	
	if hash1 != hash2 {
		t.Errorf("Hashes don't match: %s != %s", hash1, hash2)
	}
	
	// Change the content
	mockFS.AddFile("testfile.txt", []byte("different content"), time.Now())
	
	// Hash should be different now
	hash3, err := fo.ComputeFileHash("testfile.txt")
	if err != nil {
		t.Fatalf("ComputeFileHash failed: %v", err)
	}
	
	if hash1 == hash3 {
		t.Errorf("Hashes should be different after content change")
	}
}

// TestFileOps_CompareFilesBytes_WithMockFS demonstrates byte comparison without real files
func TestFileOps_CompareFilesBytes_WithMockFS(t *testing.T) {
	mockFS := filesystem.NewMockFileSystem()
	
	// Add two identical files
	content := []byte("identical content")
	mockFS.AddFile("file1.txt", content, time.Now())
	mockFS.AddFile("file2.txt", content, time.Now())
	
	fo := NewFileOps(mockFS)
	
	// Compare - should be identical
	identical, err := fo.CompareFilesBytes("file1.txt", "file2.txt")
	if err != nil {
		t.Fatalf("CompareFilesBytes failed: %v", err)
	}
	
	if !identical {
		t.Error("Files should be identical")
	}
	
	// Change one file
	mockFS.AddFile("file2.txt", []byte("different content"), time.Now())
	
	// Compare again - should be different
	identical, err = fo.CompareFilesBytes("file1.txt", "file2.txt")
	if err != nil {
		t.Fatalf("CompareFilesBytes failed: %v", err)
	}
	
	if identical {
		t.Error("Files should be different")
	}
}

// TestFileOps_CopyFile_WithMockFS demonstrates file copying without real filesystem
func TestFileOps_CopyFile_WithMockFS(t *testing.T) {
	mockFS := filesystem.NewMockFileSystem()
	
	// Add source file
	sourceContent := []byte("content to copy")
	sourceTime := time.Now().Add(-1 * time.Hour)
	mockFS.AddFile("source.txt", sourceContent, sourceTime)
	
	fo := NewFileOps(mockFS)
	
	// Copy the file
	written, err := fo.CopyFile("source.txt", "dest.txt", nil)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}
	
	if written != int64(len(sourceContent)) {
		t.Errorf("Expected %d bytes written, got %d", len(sourceContent), written)
	}
	
	// Verify destination file exists and has correct content
	destContent, destTime, err := mockFS.GetFile("dest.txt")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}
	
	if string(destContent) != string(sourceContent) {
		t.Errorf("Content mismatch: expected %q, got %q", sourceContent, destContent)
	}
	
	// Verify modtime was preserved
	if !destTime.Equal(sourceTime) {
		t.Errorf("Modtime not preserved: expected %v, got %v", sourceTime, destTime)
	}
}

