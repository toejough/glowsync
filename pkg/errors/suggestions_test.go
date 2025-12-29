package errors_test

import (
	"testing"

	"github.com/joe/copy-files/pkg/errors"
)

func TestSuggestionGenerator_CopyErrors(t *testing.T) {
	t.Parallel()

	gen := errors.NewSuggestionGenerator()
	suggestions := gen.Generate(errors.CategoryCopy, "/source/file.txt")

	if len(suggestions) == 0 {
		t.Fatal("expected suggestions for copy errors, got none")
	}

	// Should contain I/O related suggestions
	foundCopySuggestion := false

	for _, suggestion := range suggestions {
		if containsSubstring(suggestion, "retry") || containsSubstring(suggestion, "space") ||
			containsSubstring(suggestion, "disk") {
			foundCopySuggestion = true

			break
		}
	}

	if !foundCopySuggestion {
		t.Errorf("expected copy/I/O suggestion, got: %v", suggestions)
	}
}

func TestSuggestionGenerator_DeleteErrors(t *testing.T) {
	t.Parallel()

	gen := errors.NewSuggestionGenerator()
	suggestions := gen.Generate(errors.CategoryDelete, "/path/to/directory")

	if len(suggestions) == 0 {
		t.Fatal("expected suggestions for delete errors, got none")
	}

	// Should contain directory handling suggestions
	foundDeleteSuggestion := false

	for _, suggestion := range suggestions {
		if containsSubstring(suggestion, "directory") || containsSubstring(suggestion, "empty") {
			foundDeleteSuggestion = true

			break
		}
	}

	if !foundDeleteSuggestion {
		t.Errorf("expected directory/delete suggestion, got: %v", suggestions)
	}
}

func TestSuggestionGenerator_DiskSpaceErrors(t *testing.T) {
	t.Parallel()

	gen := errors.NewSuggestionGenerator()
	suggestions := gen.Generate(errors.CategoryDiskSpace, "/path/to/file.txt")

	if len(suggestions) == 0 {
		t.Fatal("expected suggestions for disk space errors, got none")
	}

	// Should contain disk space checking suggestions
	foundDiskSpaceSuggestion := false

	for _, suggestion := range suggestions {
		if containsSubstring(suggestion, "df") || containsSubstring(suggestion, "space") {
			foundDiskSpaceSuggestion = true

			break
		}
	}

	if !foundDiskSpaceSuggestion {
		t.Errorf("expected disk space suggestion, got: %v", suggestions)
	}
}

func TestSuggestionGenerator_EmptyPath(t *testing.T) {
	t.Parallel()

	gen := errors.NewSuggestionGenerator()
	suggestions := gen.Generate(errors.CategoryPermission, "")

	if len(suggestions) == 0 {
		t.Fatal("expected suggestions even with empty path, got none")
	}

	// Should still provide suggestions, just without path-specific details
	for _, suggestion := range suggestions {
		if suggestion == "" {
			t.Error("suggestion should not be empty string")
		}
	}
}

func TestSuggestionGenerator_PathErrors(t *testing.T) {
	t.Parallel()

	gen := errors.NewSuggestionGenerator()
	suggestions := gen.Generate(errors.CategoryPath, "/missing/path/file.txt")

	if len(suggestions) == 0 {
		t.Fatal("expected suggestions for path errors, got none")
	}

	// Should contain path verification suggestions
	foundPathSuggestion := false

	for _, suggestion := range suggestions {
		if containsSubstring(suggestion, "path") || containsSubstring(suggestion, "exist") {
			foundPathSuggestion = true

			break
		}
	}

	if !foundPathSuggestion {
		t.Errorf("expected path verification suggestion, got: %v", suggestions)
	}
}

func TestSuggestionGenerator_PermissionErrors(t *testing.T) {
	t.Parallel()

	gen := errors.NewSuggestionGenerator()
	suggestions := gen.Generate(errors.CategoryPermission, "/path/to/file.txt")

	if len(suggestions) == 0 {
		t.Fatal("expected suggestions for permission errors, got none")
	}

	// Should contain path-specific suggestions
	foundPathSuggestion := false

	for _, suggestion := range suggestions {
		if containsSubstring(suggestion, "/path/to/file.txt") || containsSubstring(suggestion, "ls -la") {
			foundPathSuggestion = true

			break
		}
	}

	if !foundPathSuggestion {
		t.Errorf("expected at least one suggestion with path or ls command, got: %v", suggestions)
	}
}

func TestSuggestionGenerator_UnknownErrors(t *testing.T) {
	t.Parallel()

	gen := errors.NewSuggestionGenerator()
	suggestions := gen.Generate(errors.CategoryUnknown, "/path/to/file.txt")

	if len(suggestions) == 0 {
		t.Fatal("expected suggestions for unknown errors, got none")
	}

	// Should contain generic helpful suggestions
	foundGenericSuggestion := false

	for _, suggestion := range suggestions {
		if containsSubstring(suggestion, "check") || containsSubstring(suggestion, "verify") {
			foundGenericSuggestion = true

			break
		}
	}

	if !foundGenericSuggestion {
		t.Errorf("expected generic helpful suggestion, got: %v", suggestions)
	}
}

// Helper function to check if string contains substring (case-insensitive).
func containsSubstring(str, substr string) bool {
	return len(str) >= len(substr) && findSubstring(str, substr)
}

func findSubstring(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true

		for j := range len(needle) {
			haystackChar := haystack[i+j]
			needleChar := needle[j]
			// Simple case-insensitive comparison
			if haystackChar >= 'A' && haystackChar <= 'Z' {
				haystackChar = haystackChar - 'A' + 'a'
			}

			if needleChar >= 'A' && needleChar <= 'Z' {
				needleChar = needleChar - 'A' + 'a'
			}

			if haystackChar != needleChar {
				match = false

				break
			}
		}

		if match {
			return true
		}
	}

	return false
}
