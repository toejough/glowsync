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

// ============================================================================
// Helper Functions
// ============================================================================

func (s SummaryScreen) getMaxPathWidth() int {
	return shared.CalculateMaxPathWidth(s.width)
}

// renderAdaptiveStatsContent builds the content for the Adaptive Concurrency widget box
func (s SummaryScreen) renderAdaptiveStatsContent() string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "Max workers used: %d\n", s.status.MaxWorkers)

	// Show bottleneck analysis
	if s.status.TotalReadTime > 0 || s.status.TotalWriteTime > 0 {
		totalIOTime := s.status.TotalReadTime + s.status.TotalWriteTime
		readPercent := float64(s.status.TotalReadTime) / float64(totalIOTime) * shared.ProgressPercentageScale
		writePercent := float64(s.status.TotalWriteTime) / float64(totalIOTime) * shared.ProgressPercentageScale

		fmt.Fprintf(&builder, "I/O breakdown: %.1f%% read, %.1f%% write", readPercent, writePercent)

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
	}

	return builder.String()
}

// renderAdditionalErrorsContent builds the content for the Additional Errors widget box
func (s SummaryScreen) renderAdditionalErrorsContent() string {
	// Use shared helper with other context (5 error limit for error state)
	errorList := shared.RenderErrorList(shared.ErrorListConfig{
		Errors:  s.status.Errors,
		Context: shared.ContextOther,
	})

	return errorList
}

// renderCancelledErrorsContent builds the content for the Errors widget box
func (s SummaryScreen) renderCancelledErrorsContent() string {
	// Use shared helper with other context (5 error limit for cancelled state)
	errorList := shared.RenderErrorList(shared.ErrorListConfig{
		Errors:  s.status.Errors,
		Context: shared.ContextOther,
	})

	return errorList
}

// renderCancelledLeftColumn builds the left column content with widget boxes for cancelled view
func (s SummaryScreen) renderCancelledLeftColumn(leftWidth int) string {
	var content string

	// Title
	content = shared.RenderWarning("⚠ Sync Cancelled") + "\n\n"

	// Early return if no status
	if s.status == nil {
		content += shared.RenderDim("No status information available") + "\n\n"
		content += shared.RenderDim("Press Enter or q to exit • Esc to start new session • Ctrl+C to exit")

		return content
	}

	// Calculate elapsed time for widget content
	endTime := s.status.EndTime
	if endTime.IsZero() {
		endTime = time.Now()
	}

	elapsed := endTime.Sub(s.status.StartTime)

	// Widget: Summary
	summaryContent := s.renderCancelledSummaryContent(elapsed)
	content += shared.RenderWidgetBox("Summary", summaryContent, leftWidth) + "\n\n"

	// Widget: Statistics
	statisticsContent := s.renderCancelledStatisticsContent()
	content += shared.RenderWidgetBox("Statistics", statisticsContent, leftWidth) + "\n\n"

	// Widget: Errors (conditional)
	if len(s.status.Errors) > 0 {
		errorsContent := s.renderCancelledErrorsContent()
		content += shared.RenderWidgetBox("Errors", errorsContent, leftWidth) + "\n\n"
	}

	// Help text at bottom of left column
	content += shared.RenderDim("Press Enter or q to exit • Esc to start new session • Ctrl+C to exit") + "\n"

	// Clickable log path
	if s.logPath != "" {
		content += shared.RenderDim("Debug log saved to: " + shared.MakePathClickable(s.logPath))
	}

	return content
}

// renderCancelledRightColumn builds the right column content with activity log for cancelled view
func (s SummaryScreen) renderCancelledRightColumn() string {
	// Use status activity log if available, otherwise empty
	var activityEntries []string
	if s.status != nil {
		activityEntries = s.status.AnalysisLog
	}

	// Render activity log with last 10 entries
	const maxLogEntries = 10

	return shared.RenderActivityLog("Activity", activityEntries, maxLogEntries)
}

// renderCancelledStatisticsContent builds the content for the Statistics widget box
func (s SummaryScreen) renderCancelledStatisticsContent() string {
	var builder strings.Builder

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

		fmt.Fprintf(&builder, "Workers: %d (max: %d)%s",
			s.status.ActiveWorkers,
			s.status.MaxWorkers,
			bottleneckInfo)
	} else {
		fmt.Fprintf(&builder, "Workers: %d", s.status.ActiveWorkers)
	}

	return builder.String()
}

// ============================================================================
// Widget Content Helpers - Cancelled View
// ============================================================================

// renderCancelledSummaryContent builds the content for the Summary widget box
func (s SummaryScreen) renderCancelledSummaryContent(elapsed time.Duration) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "Files completed: %d / %d\n", s.status.ProcessedFiles, s.status.TotalFiles)
	fmt.Fprintf(&builder, "Bytes transferred: %s / %s\n",
		shared.FormatBytes(s.status.TransferredBytes),
		shared.FormatBytes(s.status.TotalBytes))

	if s.status.CancelledFiles > 0 {
		fmt.Fprintf(&builder, "Files cancelled: %d\n", s.status.CancelledFiles)
	}

	if s.status.FailedFiles > 0 {
		fmt.Fprintf(&builder, "Files failed: %d\n", s.status.FailedFiles)
	}

	fmt.Fprintf(&builder, "Time elapsed: %s\n", shared.FormatDuration(elapsed))

	// Calculate average speed
	if elapsed.Seconds() > 0 && s.status.TransferredBytes > 0 {
		avgSpeed := float64(s.status.TransferredBytes) / elapsed.Seconds()
		fmt.Fprintf(&builder, "Average speed: %s/s", shared.FormatBytes(int64(avgSpeed)))
	}

	return builder.String()
}

// ============================================================================
// Rendering - Cancelled
// ============================================================================

func (s SummaryScreen) renderCancelledView() string {
	// Timeline shows "done" phase for cancelled view
	timeline := shared.RenderTimeline("done")

	// Calculate left column width (60% of total width)
	leftWidth := int(float64(s.width) * 0.6) //nolint:mnd // 60-40 split is standard layout ratio from design

	// Build left and right column content
	leftContent := s.renderCancelledLeftColumn(leftWidth)
	rightContent := s.renderCancelledRightColumn()

	// Combine columns using two-column layout
	mainContent := shared.RenderTwoColumnLayout(leftContent, rightContent, s.width, s.height)

	// Final assembly: timeline + main content wrapped in box
	output := timeline + "\n\n" + mainContent

	return shared.RenderBox(output, s.width, s.height)
}

// renderCompleteErrorsContent builds the content for the Errors widget box
func (s SummaryScreen) renderCompleteErrorsContent() string {
	// Use shared helper with complete state context (10 error limit)
	errorList := shared.RenderErrorList(shared.ErrorListConfig{
		Errors:  s.status.Errors,
		Context: shared.ContextComplete,
	})

	return errorList
}

// renderCompleteLeftColumn builds the left column content with widget boxes for complete view
func (s SummaryScreen) renderCompleteLeftColumn(leftWidth int) string {
	var content string

	// Title (celebratory message or error title)
	var titleBuilder strings.Builder
	s.renderCompleteTitle(&titleBuilder)
	content = titleBuilder.String() + "\n\n"

	// Early return if no status
	if s.status == nil {
		content += shared.RenderDim("No status information available") + "\n\n"
		content += shared.RenderDim("Press Enter or q to exit • Esc to start new session • Ctrl+C to exit")

		return content
	}

	// Calculate elapsed time for widget content
	endTime := s.status.EndTime
	if endTime.IsZero() {
		endTime = time.Now()
	}

	elapsed := endTime.Sub(s.status.StartTime)

	// Widget: Summary
	summaryContent := s.renderCompleteSummaryContent(elapsed)
	content += shared.RenderWidgetBox("Summary", summaryContent, leftWidth) + "\n\n"

	// Widget: This Session
	thisSessionContent := s.renderCompleteThisSessionContent(elapsed)
	content += shared.RenderWidgetBox("This Session", thisSessionContent, leftWidth) + "\n\n"

	// Widget: Statistics
	statisticsContent := s.renderCompleteStatisticsContent()
	content += shared.RenderWidgetBox("Statistics", statisticsContent, leftWidth) + "\n\n"

	// Widget: Recently Completed (conditional)
	if len(s.status.RecentlyCompleted) > 0 {
		recentlyCompletedContent := s.renderRecentlyCompletedContent()
		content += shared.RenderWidgetBox("Recently Completed", recentlyCompletedContent, leftWidth) + "\n\n"
	}

	// Widget: Adaptive Concurrency (conditional)
	if s.status.AdaptiveMode && s.status.MaxWorkers > 0 {
		adaptiveStatsContent := s.renderAdaptiveStatsContent()
		content += shared.RenderWidgetBox("Adaptive Concurrency", adaptiveStatsContent, leftWidth) + "\n\n"
	}

	// Widget: Errors (conditional)
	if len(s.status.Errors) > 0 {
		errorsContent := s.renderCompleteErrorsContent()
		content += shared.RenderWidgetBox("Errors", errorsContent, leftWidth) + "\n\n"
	}

	// Help text at bottom of left column
	content += shared.RenderDim("Press Enter or q to exit • Esc to start new session • Ctrl+C to exit") + "\n"

	// Clickable log path
	if s.logPath != "" {
		content += shared.RenderDim("Debug log saved to: " + shared.MakePathClickable(s.logPath))
	}

	return content
}

// renderCompleteRightColumn builds the right column content with activity log for complete view
func (s SummaryScreen) renderCompleteRightColumn() string {
	// Use status activity log if available, otherwise empty
	var activityEntries []string
	if s.status != nil {
		activityEntries = s.status.AnalysisLog
	}

	// Render activity log with last 10 entries
	const maxLogEntries = 10

	return shared.RenderActivityLog("Activity", activityEntries, maxLogEntries)
}

// renderCompleteStatisticsContent builds the content for the Statistics widget box
func (s SummaryScreen) renderCompleteStatisticsContent() string {
	var builder strings.Builder

	// Show worker count
	if s.status.AdaptiveMode && s.status.MaxWorkers > 0 {
		fmt.Fprintf(&builder, "Workers: %d (max: %d)\n", s.status.ActiveWorkers, s.status.MaxWorkers)
	} else {
		fmt.Fprintf(&builder, "Workers: %d\n", s.status.ActiveWorkers)
	}

	// Show read/write speeds if available
	s.renderReadWriteSpeeds(&builder)

	return builder.String()
}

// ============================================================================
// Widget Content Helpers - Complete View
// ============================================================================

// renderCompleteSummaryContent builds the content for the Summary widget box
func (s SummaryScreen) renderCompleteSummaryContent(elapsed time.Duration) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "Total files in source: %d (%s)\n",
		s.status.TotalFilesInSource,
		shared.FormatBytes(s.status.TotalBytesInSource))

	if s.status.AlreadySyncedFiles > 0 {
		fmt.Fprintf(&builder, "Already up-to-date: %d files (%s)",
			s.status.AlreadySyncedFiles,
			shared.FormatBytes(s.status.AlreadySyncedBytes))
	}

	return builder.String()
}

// renderCompleteThisSessionContent builds the content for the This Session widget box
func (s SummaryScreen) renderCompleteThisSessionContent(elapsed time.Duration) string {
	var builder strings.Builder

	// Show helpful message when zero files were synced
	if s.status.ProcessedFiles == 0 && s.status.TotalFiles == 0 {
		builder.WriteString(shared.RenderEmptyListPlaceholder("All files already up-to-date"))

		return builder.String()
	}

	fmt.Fprintf(&builder, "Files synced successfully: %d\n", s.status.ProcessedFiles)

	if s.status.CancelledFiles > 0 {
		fmt.Fprintf(&builder, "Files cancelled: %d\n", s.status.CancelledFiles)
	}

	if s.status.FailedFiles > 0 {
		fmt.Fprintf(&builder, "Files failed: %d\n", s.status.FailedFiles)
	}

	fmt.Fprintf(&builder, "Total files to copy: %d\n", s.status.TotalFiles)
	fmt.Fprintf(&builder, "Total bytes to copy: %s\n", shared.FormatBytes(s.status.TotalBytes))
	fmt.Fprintf(&builder, "Time elapsed: %s\n", shared.FormatDuration(elapsed))

	// Calculate average speed based on actual elapsed time
	if elapsed.Seconds() > 0 {
		avgSpeed := float64(s.status.TotalBytes) / elapsed.Seconds()
		fmt.Fprintf(&builder, "Average speed: %s/s", shared.FormatBytes(int64(avgSpeed)))
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

	// Default: all files already up-to-date
	builder.WriteString(shared.RenderSuccess(shared.SuccessSymbol() + " All files already up-to-date"))
}

// ============================================================================
// Rendering - Complete
// ============================================================================

func (s SummaryScreen) renderCompleteView() string {
	// Determine timeline phase based on errors
	phase := "done"
	if s.status != nil && s.status.FailedFiles > 0 {
		phase = "done_error"
	}

	timeline := shared.RenderTimeline(phase)

	// Calculate left column width (60% of total width)
	leftWidth := int(float64(s.width) * 0.6) //nolint:mnd // 60-40 split is standard layout ratio from design

	// Build left and right column content
	leftContent := s.renderCompleteLeftColumn(leftWidth)
	rightContent := s.renderCompleteRightColumn()

	// Combine columns using two-column layout
	mainContent := shared.RenderTwoColumnLayout(leftContent, rightContent, s.width, s.height)

	// Final assembly: timeline + main content wrapped in box
	output := timeline + "\n\n" + mainContent

	return shared.RenderBox(output, s.width, s.height)
}

// ============================================================================
// Widget Content Helpers - Error View
// ============================================================================

// renderErrorDetailsContent builds the content for the Error Details widget box
func (s SummaryScreen) renderErrorDetailsContent() string {
	var builder strings.Builder

	// Create enricher for actionable error messages
	enricher := errors.NewEnricher()

	if s.err != nil {
		// Enrich the main error
		enrichedErr := enricher.Enrich(s.err, "")

		builder.WriteString(fmt.Sprintf("%v\n", enrichedErr))

		// Show suggestions if available
		suggestions := errors.FormatSuggestions(enrichedErr)
		if suggestions != "" {
			builder.WriteString(suggestions)
		}
	}

	return builder.String()
}

// renderErrorLeftColumn builds the left column content with widget boxes for error view
func (s SummaryScreen) renderErrorLeftColumn(leftWidth int) string {
	var content string

	// Title
	content = shared.RenderError(shared.ErrorSymbol()+" Sync Failed") + "\n\n"

	// Widget: Error Details
	errorDetailsContent := s.renderErrorDetailsContent()
	content += shared.RenderWidgetBox("Error Details", errorDetailsContent, leftWidth) + "\n\n"

	// Widget: Partial Progress (conditional)
	if s.status != nil && s.status.ProcessedFiles > 0 {
		partialProgressContent := s.renderPartialProgressContent()
		content += shared.RenderWidgetBox("Partial Progress", partialProgressContent, leftWidth) + "\n\n"
	}

	// Widget: Additional Errors (conditional)
	if s.status != nil && len(s.status.Errors) > 0 {
		additionalErrorsContent := s.renderAdditionalErrorsContent()
		content += shared.RenderWidgetBox("Additional Errors", additionalErrorsContent, leftWidth) + "\n\n"
	}

	// Help text at bottom of left column
	content += shared.RenderDim("Press Enter or q to exit • Esc to start new session • Ctrl+C to exit") + "\n"

	// Clickable log path
	if s.logPath != "" {
		content += shared.RenderDim("Debug log saved to: " + shared.MakePathClickable(s.logPath))
	}

	return content
}

// renderErrorRightColumn builds the right column content with activity log for error view
func (s SummaryScreen) renderErrorRightColumn() string {
	// Use status activity log if available, otherwise empty
	var activityEntries []string
	if s.status != nil {
		activityEntries = s.status.AnalysisLog
	}

	// Render activity log with last 10 entries
	const maxLogEntries = 10

	return shared.RenderActivityLog("Activity", activityEntries, maxLogEntries)
}

// ============================================================================
// Rendering - Error
// ============================================================================

func (s SummaryScreen) renderErrorView() string {
	// Timeline shows "done_error" phase for error view
	timeline := shared.RenderTimeline("done_error")

	// Calculate left column width (60% of total width)
	leftWidth := int(float64(s.width) * 0.6) //nolint:mnd // 60-40 split is standard layout ratio from design

	// Build left and right column content
	leftContent := s.renderErrorLeftColumn(leftWidth)
	rightContent := s.renderErrorRightColumn()

	// Combine columns using two-column layout
	mainContent := shared.RenderTwoColumnLayout(leftContent, rightContent, s.width, s.height)

	// Final assembly: timeline + main content wrapped in box
	output := timeline + "\n\n" + mainContent

	return shared.RenderBox(output, s.width, s.height)
}

// renderPartialProgressContent builds the content for the Partial Progress widget box
func (s SummaryScreen) renderPartialProgressContent() string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "Files completed: %d\n", s.status.ProcessedFiles)
	fmt.Fprintf(&builder, "Bytes transferred: %s", shared.FormatBytes(s.status.TransferredBytes))

	return builder.String()
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

// renderRecentlyCompletedContent builds the content for the Recently Completed widget box
func (s SummaryScreen) renderRecentlyCompletedContent() string {
	var builder strings.Builder

	maxWidth := s.getMaxPathWidth()
	for _, file := range s.status.RecentlyCompleted {
		fmt.Fprintf(&builder, "%s %s\n",
			shared.SuccessSymbol(),
			shared.RenderPath(file, shared.FileItemCompleteStyle(), maxWidth))
	}

	return builder.String()
}
