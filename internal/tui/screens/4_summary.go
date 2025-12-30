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
		return shared.RenderBox("Unknown state")
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func (s SummaryScreen) getMaxPathWidth() int {
	return shared.CalculateMaxPathWidth(s.width)
}

func (s SummaryScreen) renderAdaptiveStats(builder *strings.Builder) {
	// Show adaptive concurrency stats if used
	if !s.status.AdaptiveMode || s.status.MaxWorkers == 0 {
		return
	}

	builder.WriteString("\n")
	builder.WriteString(shared.RenderLabel("Adaptive Concurrency:"))
	builder.WriteString("\n")
	fmt.Fprintf(builder, "Max workers used: %d\n", s.status.MaxWorkers)

	// Show bottleneck analysis
	if s.status.TotalReadTime > 0 || s.status.TotalWriteTime > 0 {
		totalIOTime := s.status.TotalReadTime + s.status.TotalWriteTime
		readPercent := float64(s.status.TotalReadTime) / float64(totalIOTime) * shared.ProgressPercentageScale
		writePercent := float64(s.status.TotalWriteTime) / float64(totalIOTime) * shared.ProgressPercentageScale

		fmt.Fprintf(builder, "I/O breakdown: %.1f%% read, %.1f%% write", readPercent, writePercent)

		if s.status.Bottleneck != "" {
			switch s.status.Bottleneck {
			case shared.StateSource:
				builder.WriteString(" (source-limited)")
			case shared.StateDestination:
				builder.WriteString(" (dest-limited)")
			case shared.StateBalanced:
				builder.WriteString(" (balanced)")
			}
		}

		builder.WriteString("\n")
	}
}

func (s SummaryScreen) renderCancelledErrors(builder *strings.Builder) {
	// Show error details if any
	if len(s.status.Errors) == 0 {
		return
	}

	builder.WriteString("\n")
	builder.WriteString(shared.RenderError("Errors:"))
	builder.WriteString("\n")

	// Use shared helper with other context (5 error limit for cancelled state)
	errorList := shared.RenderErrorList(shared.ErrorListConfig{
		Errors:  s.status.Errors,
		Context: shared.ContextOther,
	})
	builder.WriteString(errorList)
}

func (s SummaryScreen) renderCancelledStatistics(builder *strings.Builder) {
	builder.WriteString("\n")
	builder.WriteString(shared.RenderLabel("Statistics:"))
	builder.WriteString("\n")

	// Show worker count with bottleneck info
	if s.status.AdaptiveMode {
		bottleneckInfo := ""

		if s.status.Bottleneck != "" {
			switch s.status.Bottleneck {
			case "source":
				bottleneckInfo = " (source-limited)"
			case "destination":
				bottleneckInfo = " (dest-limited)"
			case "balanced":
				bottleneckInfo = " (balanced)"
			}
		}

		fmt.Fprintf(builder, "Workers: %d (max: %d)%s\n",
			s.status.ActiveWorkers,
			s.status.MaxWorkers,
			bottleneckInfo)
	} else {
		fmt.Fprintf(builder, "Workers: %d\n", s.status.ActiveWorkers)
	}
}

func (s SummaryScreen) renderCancelledSummary(builder *strings.Builder, elapsed time.Duration) {
	builder.WriteString(shared.RenderLabel("Summary:"))
	builder.WriteString("\n")
	fmt.Fprintf(builder, "Files completed: %d / %d\n", s.status.ProcessedFiles, s.status.TotalFiles)
	fmt.Fprintf(builder, "Bytes transferred: %s / %s\n",
		shared.FormatBytes(s.status.TransferredBytes),
		shared.FormatBytes(s.status.TotalBytes))

	if s.status.CancelledFiles > 0 {
		fmt.Fprintf(builder, "Files cancelled: %d\n", s.status.CancelledFiles)
	}

	if s.status.FailedFiles > 0 {
		fmt.Fprintf(builder, "Files failed: %d\n", s.status.FailedFiles)
	}

	fmt.Fprintf(builder, "Time elapsed: %s\n", shared.FormatDuration(elapsed))

	// Calculate average speed
	if elapsed.Seconds() > 0 && s.status.TransferredBytes > 0 {
		avgSpeed := float64(s.status.TransferredBytes) / elapsed.Seconds()
		fmt.Fprintf(builder, "Average speed: %s/s\n", shared.FormatBytes(int64(avgSpeed)))
	}
}

// ============================================================================
// Rendering - Cancelled
// ============================================================================

func (s SummaryScreen) renderCancelledView() string {
	var builder strings.Builder

	builder.WriteString(shared.RenderWarning("⚠ Sync Cancelled"))
	builder.WriteString("\n\n")

	if s.status != nil {
		// Use EndTime if available, otherwise fall back to current time
		endTime := s.status.EndTime
		if endTime.IsZero() {
			endTime = time.Now()
		}

		elapsed := endTime.Sub(s.status.StartTime)

		s.renderCancelledSummary(&builder, elapsed)
		s.renderCancelledStatistics(&builder)
		s.renderCancelledErrors(&builder)
	}

	builder.WriteString("\n")
	builder.WriteString(shared.RenderSubtitle("Press Enter or q to exit • Esc to start new session • Ctrl+C to exit"))
	builder.WriteString("\n")

	if s.logPath != "" {
		builder.WriteString(shared.RenderDim("Debug log saved to: " + shared.MakePathClickable(s.logPath)))
	}

	return shared.RenderBox(builder.String())
}

func (s SummaryScreen) renderCompleteErrors(builder *strings.Builder) {
	// Show error details if any
	if len(s.status.Errors) == 0 {
		return
	}

	builder.WriteString("\n")
	builder.WriteString(shared.RenderError("Errors:"))
	builder.WriteString("\n")

	// Use shared helper with complete state context (10 error limit)
	errorList := shared.RenderErrorList(shared.ErrorListConfig{
		Errors:  s.status.Errors,
		Context: shared.ContextComplete,
	})
	builder.WriteString(errorList)
}

func (s SummaryScreen) renderCompleteStatistics(builder *strings.Builder) {
	builder.WriteString("\n")
	builder.WriteString(shared.RenderLabel("Statistics:"))
	builder.WriteString("\n")

	// Show worker count
	if s.status.AdaptiveMode && s.status.MaxWorkers > 0 {
		fmt.Fprintf(builder, "Workers: %d (max: %d)\n", s.status.ActiveWorkers, s.status.MaxWorkers)
	} else {
		fmt.Fprintf(builder, "Workers: %d\n", s.status.ActiveWorkers)
	}

	// Show read/write speeds if available
	s.renderReadWriteSpeeds(builder)
}

func (s SummaryScreen) renderCompleteSummary(builder *strings.Builder, elapsed time.Duration) {
	// Overall summary
	builder.WriteString(shared.RenderLabel("Summary:"))
	builder.WriteString("\n")
	fmt.Fprintf(builder, "Total files in source: %d (%s)\n",
		s.status.TotalFilesInSource,
		shared.FormatBytes(s.status.TotalBytesInSource))

	if s.status.AlreadySyncedFiles > 0 {
		fmt.Fprintf(builder, "Already up-to-date: %d files (%s)\n",
			s.status.AlreadySyncedFiles,
			shared.FormatBytes(s.status.AlreadySyncedBytes))
	}

	builder.WriteString("\n")
	builder.WriteString(shared.RenderLabel("This Session:"))
	builder.WriteString("\n")

	// Show helpful message when zero files were synced
	if s.status.ProcessedFiles == 0 && s.status.TotalFiles == 0 {
		builder.WriteString(shared.RenderEmptyListPlaceholder("All files already up-to-date"))
		builder.WriteString("\n")
	} else {
		fmt.Fprintf(builder, "Files synced successfully: %d\n", s.status.ProcessedFiles)

		if s.status.CancelledFiles > 0 {
			fmt.Fprintf(builder, "Files cancelled: %d\n", s.status.CancelledFiles)
		}

		if s.status.FailedFiles > 0 {
			fmt.Fprintf(builder, "Files failed: %d\n", s.status.FailedFiles)
		}

		fmt.Fprintf(builder, "Total files to copy: %d\n", s.status.TotalFiles)
		fmt.Fprintf(builder, "Total bytes to copy: %s\n", shared.FormatBytes(s.status.TotalBytes))
	}
	fmt.Fprintf(builder, "Time elapsed: %s\n", shared.FormatDuration(elapsed))

	// Calculate average speed based on actual elapsed time
	if elapsed.Seconds() > 0 {
		avgSpeed := float64(s.status.TotalBytes) / elapsed.Seconds()
		fmt.Fprintf(builder, "Average speed: %s/s\n", shared.FormatBytes(int64(avgSpeed)))
	}
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

	// Default: all files already up-to-date
	builder.WriteString(shared.RenderSuccess(shared.SuccessSymbol() + " All files already up-to-date"))
}

// ============================================================================
// Rendering - Complete
// ============================================================================

func (s SummaryScreen) renderCompleteView() string {
	var builder strings.Builder

	// Show different title based on whether there were errors
	s.renderCompleteTitle(&builder)

	builder.WriteString("\n\n")

	if s.status != nil {
		// Use EndTime if available, otherwise fall back to current time
		endTime := s.status.EndTime
		if endTime.IsZero() {
			endTime = time.Now()
		}

		elapsed := endTime.Sub(s.status.StartTime)

		s.renderCompleteSummary(&builder, elapsed)
		s.renderCompleteStatistics(&builder)
		s.renderRecentlyCompleted(&builder)
		s.renderAdaptiveStats(&builder)
		s.renderCompleteErrors(&builder)
	}

	builder.WriteString("\n")
	builder.WriteString(shared.RenderSubtitle("Press Enter or q to exit • Esc to start new session • Ctrl+C to exit"))
	builder.WriteString("\n")

	if s.logPath != "" {
		builder.WriteString(shared.RenderDim("Debug log saved to: " + shared.MakePathClickable(s.logPath)))
	}

	return shared.RenderBox(builder.String())
}

// ============================================================================
// Rendering - Error
// ============================================================================

func (s SummaryScreen) renderErrorView() string {
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

	builder.WriteString(shared.RenderSubtitle("Press Enter or q to exit • Esc to start new session • Ctrl+C to exit"))
	builder.WriteString("\n")

	if s.logPath != "" {
		builder.WriteString(shared.RenderDim("Debug log saved to: " + shared.MakePathClickable(s.logPath)))
	}

	return shared.RenderBox(builder.String())
}

func (s SummaryScreen) renderReadWriteSpeeds(builder *strings.Builder) {
	if !s.status.AdaptiveMode || s.status.TotalReadTime == 0 || s.status.TotalWriteTime == 0 {
		return
	}

	totalIOTime := s.status.TotalReadTime + s.status.TotalWriteTime
	if totalIOTime == 0 || s.status.TransferredBytes == 0 {
		return
	}

	// Calculate effective speeds based on time spent
	readSpeed := float64(s.status.TransferredBytes) / s.status.TotalReadTime.Seconds()
	writeSpeed := float64(s.status.TransferredBytes) / s.status.TotalWriteTime.Seconds()

	fmt.Fprintf(builder, "Read speed: %s/s • Write speed: %s/s\n",
		shared.FormatBytes(int64(readSpeed)),
		shared.FormatBytes(int64(writeSpeed)))
}

func (s SummaryScreen) renderRecentlyCompleted(builder *strings.Builder) {
	if len(s.status.RecentlyCompleted) == 0 {
		return
	}

	builder.WriteString("\n")
	builder.WriteString(shared.RenderLabel("Recently Completed:"))
	builder.WriteString("\n")

	maxWidth := s.getMaxPathWidth()
	for _, file := range s.status.RecentlyCompleted {
		fmt.Fprintf(builder, "  %s %s\n",
			shared.SuccessSymbol(),
			shared.RenderPath(file, shared.FileItemCompleteStyle(), maxWidth))
	}
}
