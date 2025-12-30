package errors

import (
	"errors"
	"regexp"
	"strings"
)

// Enricher enriches standard errors with actionable suggestions.
type Enricher interface {
	Enrich(err error, affectedPath string) error
}

// NewEnricher creates a new Enricher with default pattern matcher and suggestion generator.
func NewEnricher() Enricher {
	return &enricher{
		matcher:   NewPatternMatcher(),
		generator: NewSuggestionGenerator(),
	}
}

// unexported variables.
var (
	//nolint:gochecknoglobals // Compiled regexes shared across all enricher instances for performance
	pathExtractionPatterns = []*regexp.Regexp{
		// Unix/Linux paths (absolute and relative)
		regexp.MustCompile(`\b\w+\s+([./][^\s:]+):`),
		// Windows paths with backslashes
		regexp.MustCompile(`\b\w+\s+([A-Za-z]:\\[^\s:]+):`),
		// Windows paths with forward slashes
		regexp.MustCompile(`\b\w+\s+([A-Za-z]:/[^\s:]+):`),
	}
)

// enricher is the concrete implementation of Enricher.
type enricher struct {
	matcher   PatternMatcher
	generator SuggestionGenerator
}

// Enrich takes a standard error and enriches it with category and actionable suggestions.
// If the error is already an ActionableError, it is returned unchanged.
// If affectedPath is empty, attempts to extract a path from the error message.
func (e *enricher) Enrich(err error, affectedPath string) error {
	// If already actionable, return as-is
	var actionableErr ActionableError
	if errors.As(err, &actionableErr) {
		return actionableErr
	}

	errMsg := err.Error()

	// If no path provided, try to extract from error message
	if affectedPath == "" {
		affectedPath = extractPath(errMsg)
	}

	// Match error message to category
	category := e.matcher.Match(errMsg)

	// Generate suggestions for the category
	suggestions := e.generator.Generate(category, affectedPath)

	// Create and return actionable error
	return NewActionableError(
		errMsg,
		category,
		suggestions,
		affectedPath,
	)
}

// extractPath attempts to extract a file path from common Go error message formats.
// Returns empty string if no path is found.
//
// This function recognizes standard Go error formats like:
//   - "open /path/to/file: permission denied"
//   - "stat /var/log/app.log: no such file or directory"
//   - "remove C:\Windows\temp\data: directory not empty"
//
// The extracted path is used to provide more personalized suggestions
// (e.g., "Check permissions for /specific/path" instead of generic advice).
//
// Performance: Uses regex matching with O(n) complexity where n is the error message length.
// Patterns are pre-compiled at package initialization and cached for efficiency.
// Error enrichment is only performed at error display time, not in hot paths.
//
// Visual Testing Notes:
// - Tested with TUI integration tests showing proper formatting with bullet points
// - Suggestions display correctly in 80-character terminal width
// - Long paths in suggestions don't break formatting (lipgloss handles wrapping)
// - Multiple suggestions are clearly separated and readable
func extractPath(errorMsg string) string {
	// Common Go error patterns: "operation /path/to/file: error description"
	// Examples:
	// - "open /home/user/file.txt: permission denied"
	// - "stat /var/log/app.log: no such file or directory"
	// - "remove /tmp/data: directory not empty"

	for _, pattern := range pathExtractionPatterns {
		if matches := pattern.FindStringSubmatch(errorMsg); len(matches) > 1 {
			path := strings.TrimSpace(matches[1])
			if path != "" {
				return path
			}
		}
	}

	return ""
}
