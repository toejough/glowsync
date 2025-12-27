package filesystem_test

//go:generate impgen filesystem.FileSystem
//go:generate impgen filesystem.FileScanner

import (
	"testing"
	"time"

	"github.com/joe/copy-files/pkg/filesystem"
)

// TestRealFileSystem tests the RealFileSystem implementation.
// Since RealFileSystem is a thin wrapper around os package functions,
// we test that the methods exist and can be called.
// We don't test the actual filesystem operations to avoid touching the real filesystem.
func TestRealFileSystem(t *testing.T) {
	t.Parallel()

	fs := filesystem.NewRealFileSystem()

	// Test that Scan returns a scanner (we won't iterate it)
	scanner := fs.Scan("/nonexistent")
	if scanner == nil {
		t.Error("Scan should return a non-nil scanner")
	}

	// Test error handling for nonexistent paths
	_, err := fs.Stat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Stat should return error for nonexistent path")
	}

	_, err = fs.Open("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Open should return error for nonexistent path")
	}

	// Test that Create fails for invalid paths (no permission to create in root)
	_, err = fs.Create("/invalid/path/that/cannot/be/created")
	if err == nil {
		t.Error("Create should return error for invalid path")
	}

	// Test that MkdirAll fails for invalid paths
	err = fs.MkdirAll("/invalid/path/that/cannot/be/created", 0o755)
	if err == nil {
		t.Error("MkdirAll should return error for invalid path")
	}

	// Test that Remove fails for nonexistent paths
	err = fs.Remove("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Remove should return error for nonexistent path")
	}

	// Test that Chtimes fails for nonexistent paths
	err = fs.Chtimes("/nonexistent/path/that/does/not/exist", time.Now(), time.Now())
	if err == nil {
		t.Error("Chtimes should return error for nonexistent path")
	}
}
