package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// SummaryScreen displays the final results
type SummaryScreen struct {
	status     *syncengine.Status
	finalState string // "complete", "cancelled", "error"
	err        error
	width      int
}

// NewSummaryScreen creates a new summary screen
func NewSummaryScreen(engine *syncengine.Engine, finalState string, err error) *SummaryScreen {
	var status *syncengine.Status
	if engine != nil {
		status = engine.GetStatus()
	}

	return &SummaryScreen{
		status:     status,
		finalState: finalState,
		err:        err,
	}
}

// Init implements tea.Model
func (s SummaryScreen) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (s SummaryScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
		case shared.KeyCtrlC, "q", "enter":
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
	maxWidth := max(s.width-shared.ProgressLogThreshold, shared.ProgressBarWidth)

	return maxWidth
}

func (s SummaryScreen) renderAdaptiveStats(b *strings.Builder) {
	// Show adaptive concurrency stats if used
	if !s.status.AdaptiveMode || s.status.MaxWorkers == 0 {
		return
	}

	b.WriteString("\n")
	b.WriteString(shared.RenderLabel("Adaptive Concurrency:"))
	b.WriteString("\n")
	fmt.Fprintf(b, "Max workers used: %d\n", s.status.MaxWorkers)

	// Show bottleneck analysis
	if s.status.TotalReadTime > 0 || s.status.TotalWriteTime > 0 {
		totalIOTime := s.status.TotalReadTime + s.status.TotalWriteTime
		readPercent := float64(s.status.TotalReadTime) / float64(totalIOTime) * shared.ProgressPercentageScale
		writePercent := float64(s.status.TotalWriteTime) / float64(totalIOTime) * shared.ProgressPercentageScale

		fmt.Fprintf(b, "I/O breakdown: %.1f%% read, %.1f%% write", readPercent, writePercent)

		if s.status.Bottleneck != "" {
			switch s.status.Bottleneck {
			case shared.StateSource:
				b.WriteString(" (source-limited)")
			case shared.StateDestination:
				b.WriteString(" (dest-limited)")
			case shared.StateBalanced:
				b.WriteString(" (balanced)")
			}
		}

		b.WriteString("\n")
	}
}

func (s SummaryScreen) renderCancelledErrors(b *strings.Builder) {
	// Show error details if any
	if len(s.status.Errors) == 0 {
		return
	}

	b.WriteString("\n")
	b.WriteString(shared.RenderError("Errors:"))
	b.WriteString("\n")

	// Show up to 5 errors
	maxErrors := 5
	for i, fileErr := range s.status.Errors {
		if i >= maxErrors {
			remaining := len(s.status.Errors) - maxErrors
			fmt.Fprintf(b, "... and %d more error(s)\n", remaining)

			break
		}

		fmt.Fprintf(b, "  ✗ %s: %v\n", fileErr.FilePath, fileErr.Error)
	}
}

func (s SummaryScreen) renderCancelledStatistics(b *strings.Builder) {
	b.WriteString("\n")
	b.WriteString(shared.RenderLabel("Statistics:"))
	b.WriteString("\n")

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

		fmt.Fprintf(b, "Workers: %d (max: %d)%s\n",
			s.status.ActiveWorkers,
			s.status.MaxWorkers,
			bottleneckInfo)
	} else {
		fmt.Fprintf(b, "Workers: %d\n", s.status.ActiveWorkers)
	}
}

func (s SummaryScreen) renderCancelledSummary(b *strings.Builder, elapsed time.Duration) {
	b.WriteString(shared.RenderLabel("Summary:"))
	b.WriteString("\n")
	fmt.Fprintf(b, "Files completed: %d / %d\n", s.status.ProcessedFiles, s.status.TotalFiles)
	fmt.Fprintf(b, "Bytes transferred: %s / %s\n",
		shared.FormatBytes(s.status.TransferredBytes),
		shared.FormatBytes(s.status.TotalBytes))

	if s.status.CancelledFiles > 0 {
		fmt.Fprintf(b, "Files cancelled: %d\n", s.status.CancelledFiles)
	}

	if s.status.FailedFiles > 0 {
		fmt.Fprintf(b, "Files failed: %d\n", s.status.FailedFiles)
	}

	fmt.Fprintf(b, "Time elapsed: %s\n", shared.FormatDuration(elapsed))

	// Calculate average speed
	if elapsed.Seconds() > 0 && s.status.TransferredBytes > 0 {
		avgSpeed := float64(s.status.TransferredBytes) / elapsed.Seconds()
		fmt.Fprintf(b, "Average speed: %s/s\n", shared.FormatBytes(int64(avgSpeed)))
	}
}

// ============================================================================
// Rendering - Cancelled
// ============================================================================

func (s SummaryScreen) renderCancelledView() string {
	var b strings.Builder

	b.WriteString(shared.RenderWarning("⚠ Sync Cancelled"))
	b.WriteString("\n\n")

	if s.status != nil {
		// Use EndTime if available, otherwise fall back to current time
		endTime := s.status.EndTime
		if endTime.IsZero() {
			endTime = time.Now()
		}

		elapsed := endTime.Sub(s.status.StartTime)

		s.renderCancelledSummary(&b, elapsed)
		s.renderCancelledStatistics(&b)
		s.renderCancelledErrors(&b)
	}

	b.WriteString("\n")
	b.WriteString(shared.RenderSubtitle("Press Enter or Ctrl+C to exit"))
	b.WriteString("\n")
	b.WriteString(shared.RenderDim("Debug log saved to: copy-files-debug.log"))

	return shared.RenderBox(b.String())
}

func (s SummaryScreen) renderCompleteErrors(b *strings.Builder) {
	// Show error details if any
	if len(s.status.Errors) == 0 {
		return
	}

	b.WriteString("\n")
	b.WriteString(shared.RenderError("Errors:"))
	b.WriteString("\n")

	// Show up to 10 errors
	maxErrors := 10
	for i, fileErr := range s.status.Errors {
		if i >= maxErrors {
			remaining := len(s.status.Errors) - maxErrors
			fmt.Fprintf(b, "... and %d more error(s)\n", remaining)

			break
		}

		fmt.Fprintf(b, "  ✗ %s: %v\n", fileErr.FilePath, fileErr.Error)
	}
}

func (s SummaryScreen) renderCompleteStatistics(b *strings.Builder) {
	b.WriteString("\n")
	b.WriteString(shared.RenderLabel("Statistics:"))
	b.WriteString("\n")

	// Show worker count
	if s.status.AdaptiveMode && s.status.MaxWorkers > 0 {
		fmt.Fprintf(b, "Workers: %d (max: %d)\n", s.status.ActiveWorkers, s.status.MaxWorkers)
	} else {
		fmt.Fprintf(b, "Workers: %d\n", s.status.ActiveWorkers)
	}

	// Show read/write speeds if available
	s.renderReadWriteSpeeds(b)
}

func (s SummaryScreen) renderCompleteSummary(b *strings.Builder, elapsed time.Duration) {
	// Overall summary
	b.WriteString(shared.RenderLabel("Summary:"))
	b.WriteString("\n")
	fmt.Fprintf(b, "Total files in source: %d (%s)\n",
		s.status.TotalFilesInSource,
		shared.FormatBytes(s.status.TotalBytesInSource))

	if s.status.AlreadySyncedFiles > 0 {
		fmt.Fprintf(b, "Already up-to-date: %d files (%s)\n",
			s.status.AlreadySyncedFiles,
			shared.FormatBytes(s.status.AlreadySyncedBytes))
	}

	b.WriteString("\n")
	b.WriteString(shared.RenderLabel("This Session:"))
	b.WriteString("\n")

	fmt.Fprintf(b, "Files synced successfully: %d\n", s.status.ProcessedFiles)

	if s.status.CancelledFiles > 0 {
		fmt.Fprintf(b, "Files cancelled: %d\n", s.status.CancelledFiles)
	}

	if s.status.FailedFiles > 0 {
		fmt.Fprintf(b, "Files failed: %d\n", s.status.FailedFiles)
	}

	fmt.Fprintf(b, "Total files to copy: %d\n", s.status.TotalFiles)
	fmt.Fprintf(b, "Total bytes to copy: %s\n", shared.FormatBytes(s.status.TotalBytes))
	fmt.Fprintf(b, "Time elapsed: %s\n", shared.FormatDuration(elapsed))

	// Calculate average speed based on actual elapsed time
	if elapsed.Seconds() > 0 {
		avgSpeed := float64(s.status.TotalBytes) / elapsed.Seconds()
		fmt.Fprintf(b, "Average speed: %s/s\n", shared.FormatBytes(int64(avgSpeed)))
	}
}

// ============================================================================
// Rendering - Complete
// ============================================================================

func (s SummaryScreen) renderCompleteView() string {
	var b strings.Builder

	// Show different title based on whether there were errors
	if s.status != nil && s.status.FailedFiles > 0 {
		b.WriteString(shared.RenderError("⚠ Sync Complete with Errors"))
	} else {
		b.WriteString(shared.RenderSuccess("✓ Sync Complete!"))
	}

	b.WriteString("\n\n")

	if s.status != nil {
		// Use EndTime if available, otherwise fall back to current time
		endTime := s.status.EndTime
		if endTime.IsZero() {
			endTime = time.Now()
		}

		elapsed := endTime.Sub(s.status.StartTime)

		s.renderCompleteSummary(&b, elapsed)
		s.renderCompleteStatistics(&b)
		s.renderRecentlyCompleted(&b)
		s.renderAdaptiveStats(&b)
		s.renderCompleteErrors(&b)
	}

	b.WriteString("\n")
	b.WriteString(shared.RenderSubtitle("Press Enter or Ctrl+C to exit"))
	b.WriteString("\n")
	b.WriteString(shared.RenderDim("Debug log saved to: copy-files-debug.log"))

	return shared.RenderBox(b.String())
}

// ============================================================================
// Rendering - Error
// ============================================================================

func (s SummaryScreen) renderErrorView() string {
	var b strings.Builder

	b.WriteString(shared.RenderError("✗ Sync Failed"))
	b.WriteString("\n\n")

	if s.err != nil {
		b.WriteString(shared.RenderLabel("Error:"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("%v\n", s.err))
		b.WriteString("\n")
	}

	if s.status != nil {
		// Show partial progress if any
		if s.status.ProcessedFiles > 0 {
			b.WriteString(shared.RenderLabel("Partial Progress:"))
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("Files completed: %d\n", s.status.ProcessedFiles))
			b.WriteString(fmt.Sprintf("Bytes transferred: %s\n", shared.FormatBytes(s.status.TransferredBytes)))
			b.WriteString("\n")
		}

		// Show errors if any
		if len(s.status.Errors) > 0 {
			b.WriteString(shared.RenderError("Additional Errors:"))
			b.WriteString("\n")

			// Show up to 5 errors
			maxErrors := 5
			for i, fileErr := range s.status.Errors {
				if i >= maxErrors {
					remaining := len(s.status.Errors) - maxErrors
					b.WriteString(fmt.Sprintf("... and %d more error(s)\n", remaining))

					break
				}

				b.WriteString(fmt.Sprintf("  ✗ %s: %v\n", fileErr.FilePath, fileErr.Error))
			}

			b.WriteString("\n")
		}
	}

	b.WriteString(shared.RenderSubtitle("Press Enter or Ctrl+C to exit"))
	b.WriteString("\n")
	b.WriteString(shared.RenderDim("Debug log saved to: copy-files-debug.log"))

	return shared.RenderBox(b.String())
}

func (s SummaryScreen) renderReadWriteSpeeds(b *strings.Builder) {
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

	fmt.Fprintf(b, "Read speed: %s/s • Write speed: %s/s\n",
		shared.FormatBytes(int64(readSpeed)),
		shared.FormatBytes(int64(writeSpeed)))
}

func (s SummaryScreen) renderRecentlyCompleted(b *strings.Builder) {
	if len(s.status.RecentlyCompleted) == 0 {
		return
	}

	b.WriteString("\n")
	b.WriteString(shared.RenderLabel("Recently Completed:"))
	b.WriteString("\n")

	maxWidth := s.getMaxPathWidth()
	for _, file := range s.status.RecentlyCompleted {
		fmt.Fprintf(b, "  ✓ %s\n", s.truncatePath(file, maxWidth))
	}
}

func (s SummaryScreen) truncatePath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}
	// Truncate from the middle
	halfWidth := (maxWidth - shared.ProgressEllipsisLength) / shared.ProgressHalfDivisor

	return path[:halfWidth] + "..." + path[len(path)-halfWidth:]
}
