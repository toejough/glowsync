package shared

import (
	"fmt"
	"strings"

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/pkg/errors"
)

// Error display limits for different screen contexts
const (
	// ErrorLimitInProgress is for screens showing ongoing operations (sync, confirmation)
	ErrorLimitInProgress = 3

	// ErrorLimitComplete is for the summary screen when sync completed
	ErrorLimitComplete = 10

	// ErrorLimitOther is for summary screen in cancelled/error states
	ErrorLimitOther = 5
)

// ErrorDisplayContext defines the context in which errors are being displayed
type ErrorDisplayContext int

const (
	// ContextInProgress indicates errors shown during sync or confirmation
	ContextInProgress ErrorDisplayContext = iota
	// ContextComplete indicates errors shown after successful sync completion
	ContextComplete
	// ContextOther indicates errors shown after cancellation or error
	ContextOther
)

// ErrorListConfig holds configuration for rendering error lists
type ErrorListConfig struct {
	// Errors is the list of file errors to display
	Errors []syncengine.FileError

	// Context determines the display limit and overflow message
	Context ErrorDisplayContext

	// MaxWidth is the maximum width for path and error message display
	MaxWidth int

	// TruncatePathFunc is the function to use for truncating paths
	TruncatePathFunc func(string, int) string
}

// RenderErrorList renders a list of errors with appropriate limits and formatting
// based on the display context. Returns the rendered error list as a string.
func RenderErrorList(config ErrorListConfig) string {
	if len(config.Errors) == 0 {
		return ""
	}

	var builder strings.Builder
	enricher := errors.NewEnricher()

	// Determine limit based on context
	limit := getErrorLimit(config.Context)

	// Render up to the limit
	for i, fileErr := range config.Errors {
		if i >= limit {
			remaining := len(config.Errors) - limit
			overflowMsg := getOverflowMessage(config.Context, remaining)
			fmt.Fprintf(&builder, "%s\n", overflowMsg)

			break
		}

		// Enrich error with actionable suggestions
		enrichedErr := enricher.Enrich(fileErr.Error, fileErr.FilePath)

		// Truncate path if needed
		displayPath := fileErr.FilePath
		if config.TruncatePathFunc != nil && config.MaxWidth > 0 {
			displayPath = config.TruncatePathFunc(fileErr.FilePath, config.MaxWidth)
		}

		// Render error with path
		fmt.Fprintf(&builder, "  %s %s\n",
			ErrorSymbol(),
			FileItemErrorStyle().Render(displayPath))

		// Truncate error message if needed
		errMsg := enrichedErr.Error()
		if config.MaxWidth > 0 && len(errMsg) > config.MaxWidth {
			errMsg = errMsg[:config.MaxWidth-3] + "..."
		}

		fmt.Fprintf(&builder, "    %s\n", errMsg)

		// Show suggestions if available
		suggestions := errors.FormatSuggestions(enrichedErr)
		if suggestions != "" {
			indentedSuggestions := "    " + strings.ReplaceAll(suggestions, "\n", "\n    ")
			fmt.Fprintf(&builder, "%s\n", indentedSuggestions)
		}
	}

	return builder.String()
}

// getErrorLimit returns the error display limit for a given context
func getErrorLimit(context ErrorDisplayContext) int {
	switch context {
	case ContextInProgress:
		return ErrorLimitInProgress
	case ContextComplete:
		return ErrorLimitComplete
	case ContextOther:
		return ErrorLimitOther
	default:
		return ErrorLimitOther
	}
}

// getOverflowMessage returns the appropriate message when error limit is exceeded
func getOverflowMessage(context ErrorDisplayContext, remaining int) string {
	switch context {
	case ContextInProgress:
		return fmt.Sprintf("  ... and %d more (see summary)", remaining)
	case ContextComplete, ContextOther:
		return fmt.Sprintf("... and %d more error(s)", remaining)
	default:
		return fmt.Sprintf("... and %d more error(s)", remaining)
	}
}
