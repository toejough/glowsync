package syncengine

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// FileFilter defines the interface for filtering files during sync
type FileFilter interface {
	// ShouldInclude returns true if the file at the given relative path should be included in the sync
	ShouldInclude(relativePath string) bool
}

// GlobFilter implements FileFilter using glob patterns
type GlobFilter struct {
	normalizedPattern string
	isEmpty           bool
}

// NewGlobFilter creates a new GlobFilter with the given pattern
// Empty pattern matches all files
func NewGlobFilter(pattern string) *GlobFilter {
	normalized := strings.ToLower(pattern)

	return &GlobFilter{
		normalizedPattern: normalized,
		isEmpty:           pattern == "",
	}
}

// ShouldInclude returns true if the file should be included based on the glob pattern
// Case-insensitive matching
func (f *GlobFilter) ShouldInclude(relativePath string) bool {
	// Empty pattern matches all files
	if f.isEmpty {
		return true
	}

	// Convert path to lowercase for case-insensitive matching
	normalizedPath := strings.ToLower(relativePath)

	// Use doublestar for glob matching with Fish-style patterns
	matched, err := doublestar.Match(f.normalizedPattern, normalizedPath)
	if err != nil {
		// If pattern is invalid, don't match
		return false
	}

	return matched
}
