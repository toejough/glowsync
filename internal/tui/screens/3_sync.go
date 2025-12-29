package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// SyncScreen handles the file synchronization process
type SyncScreen struct {
	engine          *syncengine.Engine
	status          *syncengine.Status
	spinner         spinner.Model
	overallProgress progress.Model
	fileProgress    progress.Model
	width           int
	height          int
	cancelled       bool
	lastUpdate      time.Time
}

// NewSyncScreen creates a new sync screen
func NewSyncScreen(engine *syncengine.Engine) *SyncScreen {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(shared.PrimaryColor())

	overallProg := progress.New(
		progress.WithDefaultGradient(),
	)

	fileProg := progress.New(
		progress.WithDefaultGradient(),
	)

	return &SyncScreen{
		engine:          engine,
		spinner:         spin,
		overallProgress: overallProg,
		fileProgress:    fileProg,
		lastUpdate:      time.Now(),
	}
}

// Init implements tea.Model
func (s SyncScreen) Init() tea.Cmd {
	return tea.Batch(
		s.spinner.Tick,
		s.startSync(),
		tickCmd(),
	)
}

// Update implements tea.Model
func (s SyncScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return s.handleWindowSize(msg)
	case tea.KeyMsg:
		return s.handleKeyMsg(msg)
	case shared.SyncCompleteMsg:
		return s.handleSyncComplete()
	case shared.ErrorMsg:
		return s.handleError(msg)
	case spinner.TickMsg:
		return s.handleSpinnerTick(msg)
	case tickMsg:
		return s.handleTick()
	}

	return s, nil
}

// View implements tea.Model
func (s SyncScreen) View() string {
	return s.renderSyncingView()
}

func (s SyncScreen) calculateMaxFilesToShow() int {
	// Calculate how many file progress bars we can show based on available screen height
	// Count lines used so far (approximate)
	linesUsed := 0
	linesUsed += 2 // Title
	linesUsed += 4 // Overall progress section
	linesUsed += 4 // Session progress section
	linesUsed += 8 // Statistics section (varies, but estimate)
	linesUsed += 2 // Section header
	linesUsed += 5 // Error section (if shown)
	linesUsed += 2 // Bottom padding

	// Each file takes 3 lines (filename + progress bar + blank line)
	linesPerFile := 3
	availableLines := max(s.height-linesUsed, 0)
	maxFilesToShow := max(availableLines/linesPerFile, 1) // Always show at least 1 file

	return maxFilesToShow
}

func (s SyncScreen) getBottleneckInfo() string {
	if !s.status.AdaptiveMode || s.status.Bottleneck == "" {
		return ""
	}

	switch s.status.Bottleneck {
	case shared.StateSource:
		return " 🔴 source-limited"
	case shared.StateDestination:
		return " 🟡 dest-limited"
	case shared.StateBalanced:
		return " 🟢 balanced"
	default:
		return ""
	}
}

func (s SyncScreen) getMaxPathWidth() int {
	return shared.CalculateMaxPathWidth(s.width)
}

func (s SyncScreen) handleError(msg shared.ErrorMsg) (tea.Model, tea.Cmd) {
	// Close the log
	if s.engine != nil {
		s.engine.CloseLog()
	}

	// Transition to summary screen with error
	return s, func() tea.Msg {
		return shared.TransitionToSummaryMsg{
			FinalState: shared.StateError,
			Err:        msg.Err,
		}
	}
}

func (s SyncScreen) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case shared.KeyCtrlC, "q":
		// Cancel the sync
		s.cancelled = true
		if s.engine != nil {
			s.engine.Cancel()
		}

		return s, nil
	}

	return s, nil
}

func (s SyncScreen) handleSpinnerTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	s.spinner, cmd = s.spinner.Update(msg)

	return s, cmd
}

func (s SyncScreen) handleSyncComplete() (tea.Model, tea.Cmd) {
	// Close the log
	if s.engine != nil {
		s.engine.CloseLog()
	}

	// Determine final state
	finalState := shared.StateComplete
	if s.cancelled {
		finalState = shared.StateCancelled
	}

	// Transition to summary screen
	return s, func() tea.Msg {
		return shared.TransitionToSummaryMsg{
			FinalState: finalState,
			Err:        nil,
		}
	}
}

func (s SyncScreen) handleTick() (tea.Model, tea.Cmd) {
	// Update status from engine, but only every 200ms to reduce lock contention
	if s.engine != nil {
		now := time.Now()
		if now.Sub(s.lastUpdate) >= shared.StatusUpdateThrottleMs*time.Millisecond {
			s.status = s.engine.GetStatus()
			s.lastUpdate = now
		}
	}

	return s, tickCmd()
}

// ============================================================================
// Message Handlers
// ============================================================================

func (s SyncScreen) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	s.width = msg.Width
	s.height = msg.Height
	// Set progress bar widths
	progressWidth := max(msg.Width-shared.ProgressUpdateInterval, shared.ProgressLogThreshold)
	progressWidth = min(progressWidth, shared.MaxProgressBarWidth)

	s.overallProgress.Width = progressWidth
	s.fileProgress.Width = progressWidth

	return s, nil
}

func (s SyncScreen) renderCurrentlyCopying(builder *strings.Builder, maxFilesToShow int) {
	// Count how many files are actually copying and display them
	totalCopying := 0
	filesDisplayed := 0

	builder.WriteString(shared.RenderLabel(fmt.Sprintf("Currently Copying (%d):", len(s.status.CurrentFiles))))
	builder.WriteString("\n")

	// Display up to maxFilesToShow files
	for _, file := range s.status.FilesToSync {
		if file.Status == "copying" {
			totalCopying++

			if filesDisplayed < maxFilesToShow {
				fmt.Fprintf(builder, "%s %s\n", s.spinner.View(), shared.FileItemCopyingStyle().Render(file.RelativePath))

				// Show progress bar for this file
				var filePercent float64
				if file.Size > 0 {
					filePercent = float64(file.Transferred) / float64(file.Size)
				}

				builder.WriteString("  ")
				builder.WriteString(s.fileProgress.ViewAs(filePercent))
				builder.WriteString("\n")

				filesDisplayed++
			}
		}
	}

	// Show how many more files are being copied but not displayed
	if totalCopying > filesDisplayed {
		builder.WriteString(shared.RenderDim(fmt.Sprintf("... and %d more files\n", totalCopying-filesDisplayed)))
	}
}

func (s SyncScreen) renderFileList(builder *strings.Builder) {
	maxFilesToShow := s.calculateMaxFilesToShow()

	// Currently copying files with progress bars
	if len(s.status.CurrentFiles) > 0 {
		s.renderCurrentlyCopying(builder, maxFilesToShow)
	} else {
		s.renderRecentFiles(builder, maxFilesToShow)
	}
}

func (s SyncScreen) renderOverallProgress(builder *strings.Builder) {
	// Overall progress (all files including already synced)
	builder.WriteString(shared.RenderLabel("Overall Progress (All Files):"))
	builder.WriteString("\n")

	var totalOverallPercent float64

	if s.status.TotalBytesInSource > 0 {
		// Already synced bytes + transferred bytes this session
		totalProcessedBytes := s.status.AlreadySyncedBytes + s.status.TransferredBytes
		totalOverallPercent = float64(totalProcessedBytes) / float64(s.status.TotalBytesInSource)
	}

	builder.WriteString(s.overallProgress.ViewAs(totalOverallPercent))
	builder.WriteString("\n")

	totalProcessedFiles := s.status.AlreadySyncedFiles + s.status.ProcessedFiles
	fmt.Fprintf(builder, "%d / %d files (%.1f%%) • %s / %s\n\n",
		totalProcessedFiles,
		s.status.TotalFilesInSource,
		totalOverallPercent*shared.ProgressPercentageScale,
		shared.FormatBytes(s.status.AlreadySyncedBytes+s.status.TransferredBytes),
		shared.FormatBytes(s.status.TotalBytesInSource))
}

func (s SyncScreen) renderRecentFiles(builder *strings.Builder, maxFilesToShow int) {
	// Show recent files when nothing is currently copying
	builder.WriteString(shared.RenderLabel("Recent Files:"))
	builder.WriteString("\n")

	maxFiles := min(maxRecentFilesToShow, maxFilesToShow)
	startIdx := max(len(s.status.FilesToSync)-maxFiles, 0)

	for i := startIdx; i < len(s.status.FilesToSync) && i < startIdx+maxFiles; i++ {
		file := s.status.FilesToSync[i]

		var (
			style lipgloss.Style
			icon  string
		)

		switch file.Status {
		case "complete":
			style = shared.FileItemCompleteStyle()
			icon = "✓"
		case "copying":
			style = shared.FileItemCopyingStyle()
			icon = s.spinner.View()
		case "error":
			style = shared.FileItemErrorStyle()
			icon = "✗"
		default:
			style = shared.FileItemStyle()
			icon = "○"
		}

		fmt.Fprintf(builder, "%s %s\n", icon, style.Render(file.RelativePath))
	}
}

func (s SyncScreen) renderSessionProgress(builder *strings.Builder) {
	// Session progress (only files being copied this session)
	builder.WriteString(shared.RenderLabel("This Session:"))
	builder.WriteString("\n")

	var sessionPercent float64
	if s.status.TotalBytes > 0 {
		sessionPercent = float64(s.status.TransferredBytes) / float64(s.status.TotalBytes)
	}

	builder.WriteString(s.fileProgress.ViewAs(sessionPercent))
	builder.WriteString("\n")
	fmt.Fprintf(builder, "%d / %d files (%.1f%%) • %s / %s",
		s.status.ProcessedFiles,
		s.status.TotalFiles,
		sessionPercent*shared.ProgressPercentageScale,
		shared.FormatBytes(s.status.TransferredBytes),
		shared.FormatBytes(s.status.TotalBytes))

	if s.status.FailedFiles > 0 {
		fmt.Fprintf(builder, " (%d failed)", s.status.FailedFiles)
	}

	builder.WriteString("\n\n")
}

func (s SyncScreen) renderStatistics(builder *strings.Builder) {
	// Calculate transfer rate
	elapsed := time.Since(s.status.StartTime)

	var rate float64
	if elapsed.Seconds() > 0 {
		rate = float64(s.status.TransferredBytes) / elapsed.Seconds()
	}

	// Calculate ETA
	var eta time.Duration

	if rate > 0 {
		remainingBytes := s.status.TotalBytes - s.status.TransferredBytes
		eta = time.Duration(float64(remainingBytes)/rate) * time.Second
	}

	// Worker count with bottleneck info
	bottleneckInfo := s.getBottleneckInfo()
	fmt.Fprintf(builder, "Workers: %d%s • Rate: %s • Elapsed: %s",
		s.status.ActiveWorkers,
		bottleneckInfo,
		shared.FormatRate(rate),
		shared.FormatDuration(elapsed))

	if eta > 0 && s.status.ProcessedFiles < s.status.TotalFiles {
		fmt.Fprintf(builder, " • ETA: %s", shared.FormatDuration(eta))
	}

	builder.WriteString("\n\n")
}

func (s SyncScreen) renderSyncingErrors(builder *strings.Builder) {
	// Show errors if any
	if len(s.status.Errors) == 0 {
		return
	}

	builder.WriteString("\n")
	builder.WriteString(shared.RenderError(fmt.Sprintf("⚠ Errors (%d):", len(s.status.Errors))))
	builder.WriteString("\n")

	// Show up to 5 most recent errors
	maxErrors := 5

	startIdx := 0
	if len(s.status.Errors) > maxErrors {
		startIdx = len(s.status.Errors) - maxErrors
	}

	maxWidth := s.getMaxPathWidth()
	for i := startIdx; i < len(s.status.Errors); i++ {
		fileErr := s.status.Errors[i]
		fmt.Fprintf(builder, "  ✗ %s\n", s.truncatePath(fileErr.FilePath, maxWidth))
		// Truncate error message if too long
		errMsg := fileErr.Error.Error()
		if len(errMsg) > maxWidth {
			errMsg = errMsg[:maxWidth-3] + "..."
		}

		fmt.Fprintf(builder, "    %s\n", errMsg)
	}

	if len(s.status.Errors) > maxErrors {
		fmt.Fprintf(builder, "  ... and %d more (see completion screen)\n", len(s.status.Errors)-maxErrors)
	}
}

// ============================================================================
// Rendering
// ============================================================================

func (s SyncScreen) renderSyncingView() string {
	var builder strings.Builder

	// Show different title based on finalization phase
	if s.status != nil && s.status.FinalizationPhase == "updating_cache" {
		builder.WriteString(shared.RenderTitle("📦 Finalizing..."))
		builder.WriteString("\n\n")
		builder.WriteString(shared.RenderLabel("Updating destination cache..."))
		builder.WriteString("\n")
		builder.WriteString(shared.RenderDim("(This helps the next sync run faster)"))
		builder.WriteString("\n\n")
	} else {
		builder.WriteString(shared.RenderTitle("📦 Syncing Files"))
		builder.WriteString("\n\n")
	}

	if s.status == nil {
		builder.WriteString(s.spinner.View())
		builder.WriteString(" Starting sync...\n\n")

		return shared.RenderBox(builder.String())
	}

	// Overall progress
	s.renderOverallProgress(&builder)

	// Session progress
	s.renderSessionProgress(&builder)

	// Statistics
	s.renderStatistics(&builder)

	// File list
	s.renderFileList(&builder)

	// Errors
	s.renderSyncingErrors(&builder)

	// Help text
	builder.WriteString("\n")

	if s.cancelled {
		builder.WriteString(shared.RenderDim("Cancelling... waiting for workers to finish"))
	} else {
		builder.WriteString(shared.RenderDim("Press Ctrl+C or q to cancel"))
	}

	return shared.RenderBox(builder.String())
}

// ============================================================================
// Sync Start
// ============================================================================

func (s SyncScreen) startSync() tea.Cmd {
	return func() tea.Msg {
		err := s.engine.Sync()
		if err != nil {
			s.engine.CloseLog()
			return shared.ErrorMsg{Err: err}
		}

		s.engine.CloseLog()

		return shared.SyncCompleteMsg{}
	}
}

func (s SyncScreen) truncatePath(path string, maxWidth int) string {
	return shared.TruncatePath(path, maxWidth)
}

// unexported constants.
const (
	// maxRecentFilesToShow is the maximum number of recent files to display
	maxRecentFilesToShow = 5
)
