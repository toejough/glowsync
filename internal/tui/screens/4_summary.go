package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
	"github.com/joe/copy-files/pkg/errors"
)

// SummaryScreen displays the final results
type SummaryScreen struct {
	status     *syncengine.Status
	finalState string // "complete", "cancelled", "error"
	err        error
	width      int
	height     int
	logPath    string
}

// NewSummaryScreen creates a new summary screen
func NewSummaryScreen(engine *syncengine.Engine, finalState string, err error, logPath string) *SummaryScreen {
	var status *syncengine.Status
	if engine != nil {
		status = engine.GetStatus()
	}

	return &SummaryScreen{
		status:     status,
		finalState: finalState,
		err:        err,
		logPath:    logPath,
	}
}

// Init implements tea.Model
func (s SummaryScreen) Init() tea.Cmd {
	// Ring bell for successful completion (delight factor for long-running operations)
	if s.finalState == "complete" && (s.status == nil || s.status.FailedFiles == 0) {
		fmt.Print("\a")
	}

	return nil
}

// Update implements tea.Model
func (s SummaryScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height

		return s, nil
	case tea.KeyMsg:
		//nolint:exhaustive // Only handling specific key types
		switch msg.Type {
		case tea.KeyCtrlC:
			// Emergency exit - quit immediately
			return s, tea.Quit
		case tea.KeyEsc:
			// Return to input screen for a new session
			return s, func() tea.Msg {
				return shared.TransitionToInputMsg{}
			}
		}

		// Handle other keys by string
		switch msg.String() {
		case "q", "enter":
			return s, tea.Quit
		}
	}

	return s, nil
}

// View implements tea.Model
func (s SummaryScreen) View() string {
	switch s.finalState {
	case shared.StateComplete:
		return s.renderCompleteView()
	case shared.StateCancelled:
		return s.renderCancelledView()
	case shared.StateError:
		return s.renderErrorView()
	default:
		return shared.RenderBox("Unknown state", s.width, s.height)
	}
}

// RenderContent returns just the content without timeline header or box wrapper.
// Used by UnifiedScreen to compose multiple screen contents together.
func (s SummaryScreen) RenderContent() string {
	switch s.finalState {
	case shared.StateComplete:
		return s.renderCompleteContent()
	case shared.StateCancelled:
		return s.renderCancelledContent()
	case shared.StateError:
		return s.renderErrorContent()
	default:
		return "Unknown state"
	}
}

// ============================================================================
// Rendering - Cancelled
// ============================================================================

func (s SummaryScreen) renderCancelledView() string {
	// Timeline header + content + help text + box wrapper (standalone mode)
	var builder strings.Builder
	builder.WriteString(shared.RenderTimeline("sync_error"))
	builder.WriteString("\n\n")
	builder.WriteString(s.renderCancelledContent())
	builder.WriteString("\n")
	builder.WriteString(shared.RenderSubtitle("Enter or q to exit • Esc for new session"))
	return shared.RenderBox(builder.String(), s.width, s.height)
}

// renderCancelledContent returns just the cancelled content without timeline or box.
func (s SummaryScreen) renderCancelledContent() string {
	var builder strings.Builder

	builder.WriteString(shared.RenderWarning("⚠ Sync Cancelled"))

	// Show errors if any (important feedback)
	if s.status != nil {
		s.renderCancelledErrors(&builder)
	}

	if s.logPath != "" {
		builder.WriteString("\n\n")
		builder.WriteString(shared.RenderDim("Debug log saved to: " + shared.MakePathClickable(s.logPath)))
	}

	return builder.String()
}

func (s SummaryScreen) renderCancelledErrors(builder *strings.Builder) {
	// Show error details if any
	if len(s.status.Errors) == 0 {
		return
	}

	builder.WriteString("\n\n")
	builder.WriteString(shared.RenderError("Errors:"))
	builder.WriteString("\n")

	// Use shared helper with other context (5 error limit for cancelled state)
	errorList := shared.RenderErrorList(shared.ErrorListConfig{
		Errors:  s.status.Errors,
		Context: shared.ContextOther,
	})
	builder.WriteString(errorList)
}

// ============================================================================
// Rendering - Complete
// ============================================================================

func (s SummaryScreen) renderCompleteView() string {
	// Timeline header + content + help text + box wrapper (standalone mode)
	var builder strings.Builder
	builder.WriteString(shared.RenderTimeline("done"))
	builder.WriteString("\n\n")
	builder.WriteString(s.renderCompleteContent())
	builder.WriteString("\n")
	builder.WriteString(shared.RenderSubtitle("Enter or q to exit • Esc for new session"))
	return shared.RenderBox(builder.String(), s.width, s.height)
}

// renderCompleteContent returns just the complete content without timeline or box.
func (s SummaryScreen) renderCompleteContent() string {
	var builder strings.Builder

	// Show different title based on whether there were errors
	s.renderCompleteTitle(&builder)

	// Show errors if any (important feedback)
	if s.status != nil {
		s.renderCompleteErrors(&builder)
	}

	if s.logPath != "" {
		builder.WriteString("\n\n")
		builder.WriteString(shared.RenderDim("Debug log saved to: " + shared.MakePathClickable(s.logPath)))
	}

	return builder.String()
}

func (s SummaryScreen) renderCompleteTitle(builder *strings.Builder) {
	// Show error title if there were failures
	if s.status != nil && s.status.FailedFiles > 0 {
		builder.WriteString(shared.RenderError("⚠ Sync Complete with Errors"))

		return
	}

	// Show celebratory success message with stats if files were synced
	if s.status != nil && s.status.ProcessedFiles > 0 {
		elapsed := time.Since(s.status.StartTime)
		if !s.status.EndTime.IsZero() {
			elapsed = s.status.EndTime.Sub(s.status.StartTime)
		}

		// Format file count with proper pluralization
		filesWord := "file"
		if s.status.ProcessedFiles != 1 {
			filesWord = "files"
		}

		message := fmt.Sprintf("%s Successfully synchronized %d %s (%s) in %s",
			shared.SuccessSymbol(),
			s.status.ProcessedFiles,
			filesWord,
			shared.FormatBytes(s.status.TransferredBytes),
			shared.FormatDuration(elapsed))
		builder.WriteString(shared.RenderSuccess(message))

		return
	}

	// Show message if files were deleted (even if none were copied)
	if s.status != nil && s.status.FilesDeleted > 0 {
		filesWord := "file"
		if s.status.FilesDeleted != 1 {
			filesWord = "files"
		}

		message := fmt.Sprintf("%s Cleaned up %d orphaned %s from destination",
			shared.SuccessSymbol(),
			s.status.FilesDeleted,
			filesWord)
		builder.WriteString(shared.RenderSuccess(message))

		return
	}

	// Default: all files already up-to-date
	builder.WriteString(shared.RenderSuccess(shared.SuccessSymbol() + " All files already up-to-date"))
}

func (s SummaryScreen) renderCompleteErrors(builder *strings.Builder) {
	// Show error details if any
	if len(s.status.Errors) == 0 {
		return
	}

	builder.WriteString("\n\n")
	builder.WriteString(shared.RenderError("Errors:"))
	builder.WriteString("\n")

	// Use shared helper with complete state context (10 error limit)
	errorList := shared.RenderErrorList(shared.ErrorListConfig{
		Errors:  s.status.Errors,
		Context: shared.ContextComplete,
	})
	builder.WriteString(errorList)
}

// ============================================================================
// Rendering - Error
// ============================================================================

func (s SummaryScreen) renderErrorView() string {
	// Timeline header + content + help text + box wrapper (standalone mode)
	var builder strings.Builder
	builder.WriteString(shared.RenderTimeline("done_error"))
	builder.WriteString("\n\n")
	builder.WriteString(s.renderErrorContent())
	builder.WriteString("\n")
	builder.WriteString(shared.RenderSubtitle("Enter or q to exit • Esc for new session"))
	return shared.RenderBox(builder.String(), s.width, s.height)
}

// renderErrorContent returns just the error content without timeline or box.
func (s SummaryScreen) renderErrorContent() string {
	var builder strings.Builder

	builder.WriteString(shared.RenderError(shared.ErrorSymbol() + " Sync Failed"))
	builder.WriteString("\n\n")

	// Create enricher for actionable error messages
	enricher := errors.NewEnricher()

	if s.err != nil {
		builder.WriteString(shared.RenderLabel("Error:"))
		builder.WriteString("\n")

		// Enrich the main error
		enrichedErr := enricher.Enrich(s.err, "")

		builder.WriteString(fmt.Sprintf("%v\n", enrichedErr))

		// Show suggestions if available
		suggestions := errors.FormatSuggestions(enrichedErr)
		if suggestions != "" {
			builder.WriteString(suggestions)
			builder.WriteString("\n")
		}

		builder.WriteString("\n")
	}

	if s.status != nil {
		// Show partial progress if any
		if s.status.ProcessedFiles > 0 {
			builder.WriteString(shared.RenderLabel("Partial Progress:"))
			builder.WriteString("\n")
			builder.WriteString(fmt.Sprintf("Files completed: %d\n", s.status.ProcessedFiles))
			builder.WriteString(fmt.Sprintf("Bytes transferred: %s\n", shared.FormatBytes(s.status.TransferredBytes)))
			builder.WriteString("\n")
		}

		// Show errors if any
		if len(s.status.Errors) > 0 {
			builder.WriteString(shared.RenderError("Additional Errors:"))
			builder.WriteString("\n")

			// Use shared helper with other context (5 error limit for error state)
			errorList := shared.RenderErrorList(shared.ErrorListConfig{
				Errors:  s.status.Errors,
				Context: shared.ContextOther,
			})
			builder.WriteString(errorList)

			builder.WriteString("\n")
		}
	}

	if s.logPath != "" {
		builder.WriteString(shared.RenderDim("Debug log saved to: " + shared.MakePathClickable(s.logPath)))
	}

	return builder.String()
}
