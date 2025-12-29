package errors

import "strings"

// PatternMatcher matches error messages to categories using string patterns.
type PatternMatcher interface {
	Match(errorMsg string) ErrorCategory
}

// NewPatternMatcher creates a new PatternMatcher with predefined patterns.
func NewPatternMatcher() PatternMatcher {
	return &patternMatcher{
		patterns: map[ErrorCategory][]string{
			CategoryPermission: {
				"permission denied",
				"access denied",
				"operation not permitted",
			},
			CategoryDiskSpace: {
				"no space left on device",
				"disk full",
				"quota exceeded",
			},
			CategoryPath: {
				"no such file or directory",
				"file not found",
				"path does not exist",
			},
			CategoryDelete: {
				"directory not empty",
				"cannot remove",
			},
			CategoryCopy: {
				"short write",
				"input/output error",
				"i/o error",
			},
		},
	}
}

// patternMatcher is the concrete implementation of PatternMatcher.
type patternMatcher struct {
	patterns map[ErrorCategory][]string
}

// Match returns the error category based on pattern matching.
func (m *patternMatcher) Match(errorMsg string) ErrorCategory {
	lowerMsg := strings.ToLower(errorMsg)

	// Check each category's patterns
	for category, patterns := range m.patterns {
		for _, pattern := range patterns {
			if strings.Contains(lowerMsg, pattern) {
				return category
			}
		}
	}

	// No match found
	return CategoryUnknown
}
