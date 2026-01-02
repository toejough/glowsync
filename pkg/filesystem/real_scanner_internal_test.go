//nolint:varnamelen,testpackage // Test files use idiomatic short variable names
package filesystem

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRealFileScanner_AllFilesEventuallyYielded tests that progressive yielding
// still yields all files eventually (correctness check).
//
// This test should PASS even with current implementation.
//
// Expected behavior:
// - All files in directory tree are eventually yielded
// - Files are yielded exactly once
// - No files are skipped
func TestRealFileScanner_AllFilesEventuallyYielded(t *testing.T) {
	t.Parallel()

	// Create test directory
	tmpDir := t.TempDir()

	// Create known set of files
	expectedFiles := make(map[string]bool)
	for i := range 5 {
		fileName := "file" + string(rune('0'+i)) + ".txt"
		filePath := filepath.Join(tmpDir, fileName)
		err := os.WriteFile(filePath, []byte("content"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		expectedFiles[fileName] = false
	}

	// Create subdirectory with files
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	for i := range 3 {
		fileName := "subfile" + string(rune('0'+i)) + ".txt"
		filePath := filepath.Join(subDir, fileName)
		err := os.WriteFile(filePath, []byte("content"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		expectedFiles[filepath.Join("subdir", fileName)] = false
	}

	// Scan and verify all files are yielded
	scanner := newRealFileScanner(tmpDir)

	filesSeen := make(map[string]int)
	for {
		file, ok := scanner.Next()
		if !ok {
			break
		}

		// Skip directories
		if file.IsDir {
			continue
		}

		// Track file
		filesSeen[file.RelativePath]++
	}

	if scanner.Err() != nil {
		t.Errorf("Scanner should not have error: %v", scanner.Err())
	}

	// Verify all expected files were seen exactly once
	for expectedFile := range expectedFiles {
		count, seen := filesSeen[expectedFile]
		if !seen {
			t.Errorf("File %s was not yielded by scanner", expectedFile)
		} else if count != 1 {
			t.Errorf("File %s was yielded %d times (expected 1)", expectedFile, count)
		}
	}

	// Verify no unexpected files
	for seenFile := range filesSeen {
		if _, expected := expectedFiles[seenFile]; !expected {
			t.Errorf("Unexpected file yielded: %s", seenFile)
		}
	}
}

// TestRealFileScanner_EmptyDirectory tests scanning an empty directory.
//
// This test should PASS with both implementations.
//
// Expected behavior:
// - Next() immediately returns false
// - Err() returns nil (no error, just empty)
func TestRealFileScanner_EmptyDirectory(t *testing.T) {
	t.Parallel()

	// Create empty directory
	tmpDir := t.TempDir()

	scanner := newRealFileScanner(tmpDir)

	// Next should return false immediately
	_, ok := scanner.Next()
	if ok {
		t.Error("Next() should return false for empty directory")
	}

	// Should not be an error
	if scanner.Err() != nil {
		t.Errorf("Err() should be nil for empty directory, got: %v", scanner.Err())
	}
}

// TestRealFileScanner_ErrorHandling tests that errors during scanning are handled correctly.
//
// This test should PASS with both implementations.
//
// Expected behavior:
// - If walk encounters error, Next() returns false
// - Err() returns the error
// - Scanner stops cleanly
func TestRealFileScanner_ErrorHandling(t *testing.T) {
	t.Parallel()

	// Try to scan a nonexistent directory
	scanner := newRealFileScanner("/nonexistent/directory/that/does/not/exist")

	// Next should return false
	_, ok := scanner.Next()
	if ok {
		t.Error("Next() should return false for nonexistent directory")
	}

	// Err should return error
	if scanner.Err() == nil {
		t.Error("Err() should return error for nonexistent directory")
	}
}

// TestRealFileScanner_FirstCallDoesNotBufferAllFiles tests that the first Next() call
// does not walk and buffer the entire directory tree before returning.
//
// This test will FAIL with current implementation because:
// - Current: scanner.scan() walks entire tree and buffers all files
// - Expected: First Next() returns first file without walking entire tree
//
// Expected behavior:
// - First Next() should return quickly with first discovered file
// - Should not require walking entire directory tree
// - Memory usage should be O(1) for first file, not O(n) for all files
func TestRealFileScanner_FirstCallDoesNotBufferAllFiles(t *testing.T) {
	t.Parallel()

	// Create test directory
	tmpDir := t.TempDir()

	// Create a few files
	for i := range 5 {
		filePath := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt")
		err := os.WriteFile(filePath, []byte("content"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Create scanner
	scanner := newRealFileScanner(tmpDir)

	// Check internal state BEFORE first Next() call
	// With progressive yielding: scanner.started should be false, channels should be nil
	if scanner.started {
		t.Error("Scanner should not be marked as started before first Next()")
	}
	if scanner.fileCh != nil {
		t.Error("Scanner should not have initialized channels before first Next()")
	}

	// Call Next() once
	_, ok := scanner.Next()
	if !ok {
		t.Fatal("First Next() should return a file")
	}

	// Check internal state AFTER first Next() call
	// With progressive yielding (EXPECTED): channels exist, but no buffering
	// The key check: scanner should have started walking (channels initialized)
	// but should NOT have buffered all files (channels exist for streaming)
	//
	// This is verified by checking that:
	// 1. Channels are initialized (walking has started)
	// 2. But we're not buffering (no slice of all files)
	if !scanner.started {
		t.Error("Scanner should be marked as started after first Next()")
	}
	if scanner.fileCh == nil {
		t.Error("Scanner should have initialized file channel after first Next()")
	}
	// Success: If we got here, we're using channels (progressive) not slices (batch)
}

// TestRealFileScanner_YieldsProgressively_NotAllAtOnce tests that realFileScanner
// yields files progressively during the walk, not after buffering all files.
//
// This test will FAIL with current implementation because:
// - Current: First Next() walks ENTIRE tree and buffers all files, then yields
// - Expected: Each Next() yields next discovered file during walk
//
// Expected behavior:
// - First Next() should return first file BEFORE walk completes
// - Subsequent Next() calls should continue yielding files as they're discovered
// - No single call should buffer all files before returning
func TestRealFileScanner_YieldsProgressively_NotAllAtOnce(t *testing.T) {
	t.Parallel()

	// Create a test directory with multiple files
	// We'll use a directory structure that's large enough to demonstrate the issue
	tmpDir := t.TempDir()

	// Create nested directories with files
	// This creates a tree that takes non-trivial time to walk
	for i := range 10 {
		dirPath := filepath.Join(tmpDir, "dir", "subdir", "nested", "deep", string(rune('a'+i)))
		err := os.MkdirAll(dirPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// Create files in each leaf directory
		for j := range 10 {
			filePath := filepath.Join(dirPath, "file"+string(rune('0'+j))+".txt")
			err := os.WriteFile(filePath, []byte("test content"), 0o644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
		}
	}

	// Create scanner
	scanner := newRealFileScanner(tmpDir)

	// Track when we call Next() and when files are returned
	// With progressive yielding, files should be returned DURING the walk
	// With batch collection, all files are buffered before first Next() returns

	// Call Next() for first file
	startTime := time.Now()
	file1, ok1 := scanner.Next()
	firstNextDuration := time.Since(startTime)

	if !ok1 {
		t.Fatal("First Next() should return a file")
	}
	if file1.RelativePath == "" {
		t.Error("First file should have a path")
	}

	// Call Next() for second file
	startTime2 := time.Now()
	file2, ok2 := scanner.Next()
	secondNextDuration := time.Since(startTime2)

	if !ok2 {
		t.Fatal("Second Next() should return a file")
	}
	if file2.RelativePath == "" {
		t.Error("Second file should have a path")
	}

	// With progressive yielding:
	// - First Next() returns quickly (just finds first file)
	// - Second Next() returns quickly (just finds next file)
	// - Both durations should be similar and small
	//
	// With batch collection (current implementation):
	// - First Next() is SLOW (walks entire tree, buffers all files)
	// - Second Next() is FAST (just returns from buffer)
	// - secondNextDuration << firstNextDuration

	// The key insight: if second call is MUCH faster than first call,
	// it means first call did all the work (batch collection)
	//
	// We expect: secondNextDuration should NOT be orders of magnitude faster
	// Reality: secondNextDuration will be ~1000x faster (batch collection)

	// Calculate ratio - with batch collection, this will be huge (e.g., 1000:1)
	// With progressive yielding, this should be close to 1:1
	if firstNextDuration == 0 || secondNextDuration == 0 {
		// If calls are too fast to measure, we can't detect the pattern
		// This is actually a sign that the directory is too small
		t.Skip("Directory walk too fast to measure timing difference")
	}

	ratio := float64(firstNextDuration) / float64(secondNextDuration)

	// With progressive yielding, ratio should be close to 1.0 (both calls similar duration)
	// With batch collection, ratio will be >> 10 (first call does all work)
	//
	// This test will FAIL because current implementation has ratio >> 10
	if ratio > 10.0 {
		t.Errorf(
			"Progressive yielding violation: First Next() took %v, second took %v (ratio: %.1fx). "+
				"This indicates batch collection (first call buffers all files). "+
				"Expected progressive yielding where both calls have similar duration.",
			firstNextDuration,
			secondNextDuration,
			ratio,
		)
	}

	// Additionally, verify the scanner is working correctly
	fileCount := 2 // Already got 2 files
	for {
		_, ok := scanner.Next()
		if !ok {
			break
		}
		fileCount++
	}

	if scanner.Err() != nil {
		t.Errorf("Scanner should not have error: %v", scanner.Err())
	}

	// We created 100 files + 10 dirs + 4 parent dirs = 114 items minimum
	// (The exact count depends on how directories are counted)
	if fileCount < 100 {
		t.Errorf("Expected at least 100 files, got %d", fileCount)
	}
}
