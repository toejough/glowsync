// Package errors provides actionable error handling with context-aware suggestions.
//
// This package enriches standard Go errors with categorization and actionable suggestions
// to help users resolve issues quickly. It automatically detects error types (permission,
// disk space, path, etc.) and provides specific guidance based on the error category.
//
// Basic Usage:
//
//	enricher := errors.NewEnricher()
//	err := os.Open("/restricted/file.txt")
//	if err != nil {
//	    actionableErr := enricher.Enrich(err, "/restricted/file.txt")
//	    // Display error with suggestions
//	    fmt.Println(actionableErr.Error())
//	    for _, suggestion := range actionableErr.(errors.ActionableError).Suggestions() {
//	        fmt.Println(" -", suggestion)
//	    }
//	}
//
// The enricher automatically extracts paths from error messages when not explicitly provided:
//
//	err := errors.New("open /home/user/file.txt: permission denied")
//	enriched := enricher.Enrich(err, "") // Path will be extracted from error message
//
// Integration with TUI:
//
//	The FormatSuggestions helper formats suggestions with bullet points for display:
//	formatted := errors.FormatSuggestions(actionableErr)
//	fmt.Println(formatted) // Displays: "Try these solutions:\n  • suggestion 1\n  • suggestion 2"
package errors

import "strings"

// Exported constants.
const (
	CategoryCopy       ErrorCategory = "copy"
	CategoryDelete     ErrorCategory = "delete"
	CategoryDiskSpace  ErrorCategory = "disk_space"
	CategoryPath       ErrorCategory = "path"
	CategoryPermission ErrorCategory = "permission"
	CategoryUnknown    ErrorCategory = "unknown"
)

// ActionableError represents an error with actionable suggestions for the user.
type ActionableError interface {
	error
	OriginalError() string
	Category() ErrorCategory
	Suggestions() []string
	AffectedPath() string
}

// NewActionableError creates a new ActionableError with the given details.
func NewActionableError(
	originalError string,
	category ErrorCategory,
	suggestions []string,
	affectedPath string,
) ActionableError {
	return &actionableError{
		originalError: originalError,
		category:      category,
		suggestions:   suggestions,
		affectedPath:  affectedPath,
	}
}

// ErrorCategory represents the type of error that occurred.
type ErrorCategory string

// FormatSuggestions formats the suggestions from an ActionableError as a bulleted list
// for display in the TUI. Returns empty string if the error is nil or has no suggestions.
func FormatSuggestions(err error) string {
	if err == nil {
		return ""
	}

	actionable, ok := err.(ActionableError)
	if !ok {
		return ""
	}

	suggestions := actionable.Suggestions()
	if len(suggestions) == 0 {
		return ""
	}

	// Format as bulleted list with two-space indent
	// Use strings.Builder for efficient string concatenation
	var builder strings.Builder
	for i, suggestion := range suggestions {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString("  • ")
		builder.WriteString(suggestion)
	}

	return builder.String()
}

// actionableError is the concrete implementation of ActionableError.
type actionableError struct {
	originalError string
	category      ErrorCategory
	suggestions   []string
	affectedPath  string
}

// AffectedPath returns the file path affected by this error.
func (e *actionableError) AffectedPath() string {
	return e.affectedPath
}

// Category returns the error category.
func (e *actionableError) Category() ErrorCategory {
	return e.category
}

// Error implements the error interface.
func (e *actionableError) Error() string {
	return e.originalError
}

// OriginalError returns the original error message.
func (e *actionableError) OriginalError() string {
	return e.originalError
}

// Suggestions returns the list of actionable suggestions.
func (e *actionableError) Suggestions() []string {
	return e.suggestions
}
