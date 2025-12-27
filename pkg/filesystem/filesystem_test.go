package filesystem_test

import (
	"os"
	"testing"
	"time"

	"github.com/joe/copy-files/pkg/filesystem"
)

//go:generate impgen filesystem.FileSystem

func TestMockFileSystem_CreateAndOpen(t *testing.T) {
	fs := filesystem.NewMockFileSystem()

	// Create a file
	content := []byte("test content")
	file, err := fs.Create("test.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	_, err = file.Write(content)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	_ = file.Close()

	// Read it back
	file, err = fs.Open("test.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	data := make([]byte, len(content))
	_, err = file.Read(data)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("Expected %q, got %q", content, data)
	}
}

func TestMockFileSystem_Stat(t *testing.T) {
	fs := filesystem.NewMockFileSystem()
	
	// Add a file
	content := []byte("test")
	modTime := time.Now().Add(-1 * time.Hour)
	fs.AddFile("test.txt", content, modTime)
	
	// Stat it
	info, err := fs.Stat("test.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	
	if info.Name() != "test.txt" {
		t.Errorf("Expected name test.txt, got %s", info.Name())
	}
	
	if info.Size() != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), info.Size())
	}
	
	if !info.ModTime().Equal(modTime) {
		t.Errorf("Expected modtime %v, got %v", modTime, info.ModTime())
	}
}

func TestMockFileSystem_Remove(t *testing.T) {
	fs := filesystem.NewMockFileSystem()
	
	// Add a file
	fs.AddFile("test.txt", []byte("test"), time.Now())
	
	// Remove it
	err := fs.Remove("test.txt")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	
	// Verify it's gone
	_, err = fs.Stat("test.txt")
	if err != os.ErrNotExist {
		t.Errorf("Expected ErrNotExist, got %v", err)
	}
}

func TestMockFileSystem_MkdirAll(t *testing.T) {
	fs := filesystem.NewMockFileSystem()
	
	// Create nested directories
	err := fs.MkdirAll("a/b/c", 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	
	// Verify they exist
	for _, path := range []string{"a", "a/b", "a/b/c"} {
		info, err := fs.Stat(path)
		if err != nil {
			t.Errorf("Stat(%s) failed: %v", path, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("Expected %s to be a directory", path)
		}
	}
}

func TestMockFileSystem_Walk(t *testing.T) {
	fs := filesystem.NewMockFileSystem()
	
	// Create a directory structure
	fs.AddDir("root", time.Now())
	fs.AddFile("root/file1.txt", []byte("content1"), time.Now())
	fs.AddFile("root/file2.txt", []byte("content2"), time.Now())
	fs.AddDir("root/subdir", time.Now())
	fs.AddFile("root/subdir/file3.txt", []byte("content3"), time.Now())
	
	// Scan the tree
	visited := []string{}
	scanner := fs.Scan("root")
	for info, ok := scanner.Next(); ok; info, ok = scanner.Next() {
		visited = append(visited, info.RelativePath)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	expected := []string{"file1.txt", "file2.txt", "subdir", "subdir/file3.txt"}
	if len(visited) != len(expected) {
		t.Errorf("Expected %d paths, got %d: %v", len(expected), len(visited), visited)
	}
}

func TestMockFileSystem_Chtimes(t *testing.T) {
	fs := filesystem.NewMockFileSystem()
	
	// Add a file
	oldTime := time.Now().Add(-2 * time.Hour)
	fs.AddFile("test.txt", []byte("test"), oldTime)
	
	// Change its modtime
	newTime := time.Now().Add(-1 * time.Hour)
	err := fs.Chtimes("test.txt", newTime, newTime)
	if err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}
	
	// Verify the change
	info, err := fs.Stat("test.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	
	if !info.ModTime().Equal(newTime) {
		t.Errorf("Expected modtime %v, got %v", newTime, info.ModTime())
	}
}

