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
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(shared.PrimaryColor)

	overallProg := progress.New(
		progress.WithDefaultGradient(),
	)

	fileProg := progress.New(
		progress.WithDefaultGradient(),
	)

	return &SyncScreen{
		engine:          engine,
		spinner:         s,
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

// ============================================================================
// Message Handlers
// ============================================================================

func (s SyncScreen) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	s.width = msg.Width
	s.height = msg.Height
	// Set progress bar widths
	progressWidth := msg.Width - 10
	if progressWidth < 20 {
		progressWidth = 20
	}
	if progressWidth > 100 {
		progressWidth = 100
	}
	s.overallProgress.Width = progressWidth
	s.fileProgress.Width = progressWidth
	return s, nil
}

func (s SyncScreen) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		// Cancel the sync
		s.cancelled = true
		if s.engine != nil {
			s.engine.Cancel()
		}
		return s, nil
	}
	return s, nil
}

func (s SyncScreen) handleSyncComplete() (tea.Model, tea.Cmd) {
	// Close the log
	if s.engine != nil {
		s.engine.CloseLog()
	}

	// Determine final state
	finalState := "complete"
	if s.cancelled {
		finalState = "cancelled"
	}

	// Transition to summary screen
	return s, func() tea.Msg {
		return shared.TransitionToSummaryMsg{
			FinalState: finalState,
			Err:        nil,
		}
	}
}

func (s SyncScreen) handleError(msg shared.ErrorMsg) (tea.Model, tea.Cmd) {
	// Close the log
	if s.engine != nil {
		s.engine.CloseLog()
	}

	// Transition to summary screen with error
	return s, func() tea.Msg {
		return shared.TransitionToSummaryMsg{
			FinalState: "error",
			Err:        msg.Err,
		}
	}
}

func (s SyncScreen) handleSpinnerTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

func (s SyncScreen) handleTick() (tea.Model, tea.Cmd) {
	// Update status from engine, but only every 200ms to reduce lock contention
	if s.engine != nil {
		now := time.Now()
		if now.Sub(s.lastUpdate) >= 200*time.Millisecond {
			s.status = s.engine.GetStatus()
			s.lastUpdate = now
		}
	}
	return s, tickCmd()
}

// ============================================================================
// Sync Start
// ============================================================================

func (s SyncScreen) startSync() tea.Cmd {
	return func() tea.Msg {
		if err := s.engine.Sync(); err != nil {
			s.engine.CloseLog()
			return shared.ErrorMsg{Err: err}
		}
		s.engine.CloseLog()
		return shared.SyncCompleteMsg{}
	}
}

// ============================================================================
// Rendering
// ============================================================================

func (s SyncScreen) renderSyncingView() string {
	var b strings.Builder

	// Show different title based on finalization phase
	if s.status != nil && s.status.FinalizationPhase == "updating_cache" {
		b.WriteString(shared.RenderTitle("📦 Finalizing..."))
		b.WriteString("\n\n")
		b.WriteString(shared.RenderLabel("Updating destination cache..."))
		b.WriteString("\n")
		b.WriteString(shared.RenderDim("(This helps the next sync run faster)"))
		b.WriteString("\n\n")
	} else {
		b.WriteString(shared.RenderTitle("📦 Syncing Files"))
		b.WriteString("\n\n")
	}

	if s.status == nil {
		b.WriteString(s.spinner.View())
		b.WriteString(" Starting sync...\n\n")
		return shared.RenderBox(b.String())
	}

	// Overall progress
	s.renderOverallProgress(&b)

	// Session progress
	s.renderSessionProgress(&b)

	// Statistics
	s.renderStatistics(&b)

	// File list
	s.renderFileList(&b)

	// Errors
	s.renderSyncingErrors(&b)

	// Help text
	b.WriteString("\n")
	if s.cancelled {
		b.WriteString(shared.RenderDim("Cancelling... waiting for workers to finish"))
	} else {
		b.WriteString(shared.RenderDim("Press Ctrl+C or q to cancel"))
	}

	return shared.RenderBox(b.String())
}

func (s SyncScreen) renderOverallProgress(b *strings.Builder) {
	// Overall progress (all files including already synced)
	b.WriteString(shared.RenderLabel("Overall Progress (All Files):"))
	b.WriteString("\n")

	var totalOverallPercent float64
	if s.status.TotalBytesInSource > 0 {
		// Already synced bytes + transferred bytes this session
		totalProcessedBytes := s.status.AlreadySyncedBytes + s.status.TransferredBytes
		totalOverallPercent = float64(totalProcessedBytes) / float64(s.status.TotalBytesInSource)
	}
	b.WriteString(s.overallProgress.ViewAs(totalOverallPercent))
	b.WriteString("\n")

	totalProcessedFiles := s.status.AlreadySyncedFiles + s.status.ProcessedFiles
	b.WriteString(fmt.Sprintf("%d / %d files (%.1f%%) • %s / %s\n\n",
		totalProcessedFiles,
		s.status.TotalFilesInSource,
		totalOverallPercent*100,
		shared.FormatBytes(s.status.AlreadySyncedBytes+s.status.TransferredBytes),
		shared.FormatBytes(s.status.TotalBytesInSource)))
}

func (s SyncScreen) renderSessionProgress(b *strings.Builder) {
	// Session progress (only files being copied this session)
	b.WriteString(shared.RenderLabel("This Session:"))
	b.WriteString("\n")

	var sessionPercent float64
	if s.status.TotalBytes > 0 {
		sessionPercent = float64(s.status.TransferredBytes) / float64(s.status.TotalBytes)
	}
	b.WriteString(s.fileProgress.ViewAs(sessionPercent))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%d / %d files (%.1f%%) • %s / %s",
		s.status.ProcessedFiles,
		s.status.TotalFiles,
		sessionPercent*100,
		shared.FormatBytes(s.status.TransferredBytes),
		shared.FormatBytes(s.status.TotalBytes)))

	if s.status.FailedFiles > 0 {
		b.WriteString(fmt.Sprintf(" (%d failed)", s.status.FailedFiles))
	}
	b.WriteString("\n\n")
}

func (s SyncScreen) renderStatistics(b *strings.Builder) {
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
	b.WriteString(fmt.Sprintf("Workers: %d%s • Rate: %s • Elapsed: %s",
		s.status.ActiveWorkers,
		bottleneckInfo,
		shared.FormatRate(rate),
		shared.FormatDuration(elapsed)))

	if eta > 0 && s.status.ProcessedFiles < s.status.TotalFiles {
		b.WriteString(fmt.Sprintf(" • ETA: %s", shared.FormatDuration(eta)))
	}

	b.WriteString("\n\n")
}

func (s SyncScreen) getBottleneckInfo() string {
	if !s.status.AdaptiveMode || s.status.Bottleneck == "" {
		return ""
	}

	switch s.status.Bottleneck {
	case "source":
		return " 🔴 source-limited"
	case "destination":
		return " 🟡 dest-limited"
	case "balanced":
		return " 🟢 balanced"
	default:
		return ""
	}
}

func (s SyncScreen) calculateMaxFilesToShow() int {
	// Calculate how many file progress bars we can show based on available screen height
	// Count lines used so far (approximate)
	linesUsed := 0
	linesUsed += 2  // Title
	linesUsed += 4  // Overall progress section
	linesUsed += 4  // Session progress section
	linesUsed += 8  // Statistics section (varies, but estimate)
	linesUsed += 2  // Section header
	linesUsed += 5  // Error section (if shown)
	linesUsed += 2  // Bottom padding

	// Each file takes 3 lines (filename + progress bar + blank line)
	linesPerFile := 3
	availableLines := s.height - linesUsed
	if availableLines < 0 {
		availableLines = 0
	}
	maxFilesToShow := availableLines / linesPerFile
	if maxFilesToShow < 1 {
		maxFilesToShow = 1 // Always show at least 1 file
	}
	return maxFilesToShow
}

func (s SyncScreen) renderFileList(b *strings.Builder) {
	maxFilesToShow := s.calculateMaxFilesToShow()

	// Currently copying files with progress bars
	if len(s.status.CurrentFiles) > 0 {
		s.renderCurrentlyCopying(b, maxFilesToShow)
	} else {
		s.renderRecentFiles(b, maxFilesToShow)
	}
}

func (s SyncScreen) renderCurrentlyCopying(b *strings.Builder, maxFilesToShow int) {
	// Count how many files are actually copying and display them
	totalCopying := 0
	filesDisplayed := 0

	b.WriteString(shared.RenderLabel(fmt.Sprintf("Currently Copying (%d):", len(s.status.CurrentFiles))))
	b.WriteString("\n")

	// Display up to maxFilesToShow files
	for _, file := range s.status.FilesToSync {
		if file.Status == "copying" {
			totalCopying++

			if filesDisplayed < maxFilesToShow {
				b.WriteString(fmt.Sprintf("%s %s\n", s.spinner.View(), shared.FileItemCopyingStyle.Render(file.RelativePath)))

				// Show progress bar for this file
				var filePercent float64
				if file.Size > 0 {
					filePercent = float64(file.Transferred) / float64(file.Size)
				}
				b.WriteString("  ")
				b.WriteString(s.fileProgress.ViewAs(filePercent))
				b.WriteString("\n")

				filesDisplayed++
			}
		}
	}

	// Show how many more files are being copied but not displayed
	if totalCopying > filesDisplayed {
		b.WriteString(shared.RenderDim(fmt.Sprintf("... and %d more files\n", totalCopying-filesDisplayed)))
	}
}

func (s SyncScreen) renderRecentFiles(b *strings.Builder, maxFilesToShow int) {
	// Show recent files when nothing is currently copying
	b.WriteString(shared.RenderLabel("Recent Files:"))
	b.WriteString("\n")

	maxFiles := 5
	if maxFiles > maxFilesToShow {
		maxFiles = maxFilesToShow
	}
	startIdx := len(s.status.FilesToSync) - maxFiles
	if startIdx < 0 {
		startIdx = 0
	}

	for i := startIdx; i < len(s.status.FilesToSync) && i < startIdx+maxFiles; i++ {
		file := s.status.FilesToSync[i]
		var style lipgloss.Style
		var icon string

		switch file.Status {
		case "complete":
			style = shared.FileItemCompleteStyle
			icon = "✓"
		case "copying":
			style = shared.FileItemCopyingStyle
			icon = s.spinner.View()
		case "error":
			style = shared.FileItemErrorStyle
			icon = "✗"
		default:
			style = shared.FileItemStyle
			icon = "○"
		}

		b.WriteString(fmt.Sprintf("%s %s\n", icon, style.Render(file.RelativePath)))
	}
}

func (s SyncScreen) renderSyncingErrors(b *strings.Builder) {
	// Show errors if any
	if len(s.status.Errors) == 0 {
		return
	}

	b.WriteString("\n")
	b.WriteString(shared.RenderError(fmt.Sprintf("⚠ Errors (%d):", len(s.status.Errors))))
	b.WriteString("\n")

	// Show up to 5 most recent errors
	maxErrors := 5
	startIdx := 0
	if len(s.status.Errors) > maxErrors {
		startIdx = len(s.status.Errors) - maxErrors
	}

	maxWidth := s.getMaxPathWidth()
	for i := startIdx; i < len(s.status.Errors); i++ {
		fileErr := s.status.Errors[i]
		b.WriteString(fmt.Sprintf("  ✗ %s\n", s.truncatePath(fileErr.FilePath, maxWidth)))
		// Truncate error message if too long
		errMsg := fileErr.Error.Error()
		if len(errMsg) > maxWidth {
			errMsg = errMsg[:maxWidth-3] + "..."
		}
		b.WriteString(fmt.Sprintf("    %s\n", errMsg))
	}

	if len(s.status.Errors) > maxErrors {
		b.WriteString(fmt.Sprintf("  ... and %d more (see completion screen)\n", len(s.status.Errors)-maxErrors))
	}
}

func (s SyncScreen) getMaxPathWidth() int {
	maxWidth := s.width - 20
	if maxWidth < 40 {
		maxWidth = 40
	}
	return maxWidth
}

func (s SyncScreen) truncatePath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}
	// Truncate from the middle
	halfWidth := (maxWidth - 3) / 2
	return path[:halfWidth] + "..." + path[len(path)-halfWidth:]
}

