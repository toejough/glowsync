//nolint:varnamelen // Test files use idiomatic short variable names (t, tt, etc.)
package syncengine_test

import (
	"testing"

	"github.com/joe/copy-files/internal/syncengine"
)

func TestGlobFilterInvalidPattern(t *testing.T) {
	t.Parallel()

	// Test that invalid patterns don't panic but return false
	filter := syncengine.NewGlobFilter("[invalid")
	result := filter.ShouldInclude("test.txt")

	if result {
		t.Error("Invalid pattern should not match files")
	}
}

//nolint:funlen // Test function with comprehensive table-driven test cases
func TestGlobFilterShouldInclude(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pattern     string
		path        string
		shouldMatch bool
		description string
	}{
		// Empty pattern tests
		{
			name:        "empty pattern matches all",
			pattern:     "",
			path:        "any/file.txt",
			shouldMatch: true,
			description: "Empty pattern should match any file",
		},

		// Simple wildcard tests
		{
			name:        "simple extension match",
			pattern:     "*.mov",
			path:        "video.mov",
			shouldMatch: true,
			description: "Should match files with .mov extension",
		},
		{
			name:        "simple extension no match",
			pattern:     "*.mov",
			path:        "video.mp4",
			shouldMatch: false,
			description: "Should not match files without .mov extension",
		},

		// Case-insensitive tests
		{
			name:        "case insensitive match uppercase pattern",
			pattern:     "*.MOV",
			path:        "video.mov",
			shouldMatch: true,
			description: "Should match case-insensitively (uppercase pattern)",
		},
		{
			name:        "case insensitive match uppercase file",
			pattern:     "*.mov",
			path:        "VIDEO.MOV",
			shouldMatch: true,
			description: "Should match case-insensitively (uppercase file)",
		},
		{
			name:        "case insensitive match mixed case",
			pattern:     "*.MoV",
			path:        "ViDeO.mOv",
			shouldMatch: true,
			description: "Should match case-insensitively (mixed case)",
		},

		// Double star (recursive) tests
		{
			name:        "double star matches nested files",
			pattern:     "**/*.mov",
			path:        "dir1/dir2/video.mov",
			shouldMatch: true,
			description: "** should match files in nested directories",
		},
		{
			name:        "double star at start",
			pattern:     "**/*.mov",
			path:        "video.mov",
			shouldMatch: true,
			description: "** should match files in root directory too",
		},
		{
			name:        "double star in middle",
			pattern:     "videos/**/final.mov",
			path:        "videos/2023/december/final.mov",
			shouldMatch: true,
			description: "** in middle should match nested paths",
		},

		// Brace expansion tests
		{
			name:        "brace expansion first option",
			pattern:     "*.{mov,mp4}",
			path:        "video.mov",
			shouldMatch: true,
			description: "Brace expansion should match first option",
		},
		{
			name:        "brace expansion second option",
			pattern:     "*.{mov,mp4}",
			path:        "video.mp4",
			shouldMatch: true,
			description: "Brace expansion should match second option",
		},
		{
			name:        "brace expansion no match",
			pattern:     "*.{mov,mp4}",
			path:        "video.avi",
			shouldMatch: false,
			description: "Brace expansion should not match other extensions",
		},
		{
			name:        "brace expansion with double star",
			pattern:     "**/*.{mov,mp4,avi}",
			path:        "videos/vacation.avi",
			shouldMatch: true,
			description: "Combining ** and brace expansion",
		},

		// Directory pattern tests
		{
			name:        "specific directory match",
			pattern:     "videos/*.mov",
			path:        "videos/clip.mov",
			shouldMatch: true,
			description: "Should match files in specific directory",
		},
		{
			name:        "specific directory no match wrong dir",
			pattern:     "videos/*.mov",
			path:        "photos/clip.mov",
			shouldMatch: false,
			description: "Should not match files in wrong directory",
		},
		{
			name:        "specific directory no match nested",
			pattern:     "videos/*.mov",
			path:        "videos/2023/clip.mov",
			shouldMatch: false,
			description: "Single * should not match nested directories",
		},

		// Complex patterns
		{
			name:        "question mark single char",
			pattern:     "file?.txt",
			path:        "file1.txt",
			shouldMatch: true,
			description: "? should match single character",
		},
		{
			name:        "question mark no match multiple",
			pattern:     "file?.txt",
			path:        "file12.txt",
			shouldMatch: false,
			description: "? should not match multiple characters",
		},
		{
			name:        "character class",
			pattern:     "file[0-9].txt",
			path:        "file5.txt",
			shouldMatch: true,
			description: "Character class should match range",
		},
		{
			name:        "character class no match",
			pattern:     "file[0-9].txt",
			path:        "filea.txt",
			shouldMatch: false,
			description: "Character class should not match outside range",
		},

		// Edge cases
		{
			name:        "root level file",
			pattern:     "*.txt",
			path:        "readme.txt",
			shouldMatch: true,
			description: "Should match root level files",
		},
		{
			name:        "deeply nested path",
			pattern:     "**/*.mov",
			path:        "a/b/c/d/e/f/g/video.mov",
			shouldMatch: true,
			description: "Should match deeply nested paths",
		},
		{
			name:        "empty path",
			pattern:     "*.mov",
			path:        "",
			shouldMatch: false,
			description: "Should not match empty path",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			filter := syncengine.NewGlobFilter(testCase.pattern)
			result := filter.ShouldInclude(testCase.path)

			if result != testCase.shouldMatch {
				t.Errorf("%s\n  Pattern: %s\n  Path: %s\n  Expected: %v\n  Got: %v",
					testCase.description,
					testCase.pattern,
					testCase.path,
					testCase.shouldMatch,
					result,
				)
			}
		})
	}
}
