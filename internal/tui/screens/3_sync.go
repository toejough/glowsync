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
	overallProgress progress.Model // Used for unified progress bar showing file count
	fileProgress    progress.Model // Used for individual file progress bars in file list
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
		shared.TickCmd(),
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
	case shared.TickMsg:
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
	// Count lines used by fixed UI elements based on actual rendering
	fixedLines := 0

	// Box vertical padding (top + bottom)
	fixedLines += 2

	// Title section: "📦 Syncing Files\n\n"
	fixedLines += 2

	// Unified progress section: label + bar + files + bytes + time + blank
	fixedLines += 6

	// Statistics section: workers line + speed line (conditional) + blank
	fixedLines += 3

	// File list section header: "Currently Copying (N):\n"
	fixedLines++

	// Help text at bottom: blank + help text
	fixedLines += 2

	// Reserve space for conditional error section (if shown)
	// Errors take: blank + header + error list (~3 errors × 2 lines each)
	if s.status != nil && len(s.status.Errors) > 0 {
		fixedLines += 8
	}

	// Reserve space for overflow message "... and X more files"
	fixedLines++

	// Calculate available lines for file list
	// Each file takes exactly 1 line: [spinner] [progress bar] [percentage] [path]
	availableLines := max(s.height-fixedLines, 1)

	return availableLines
}

func (s SyncScreen) getBottleneckInfo() string {
	if !s.status.AdaptiveMode || s.status.Bottleneck == "" {
		return ""
	}

	switch s.status.Bottleneck {
	case shared.StateSource:
		return " 🔴 source slow"
	case shared.StateDestination:
		return " 🟡 dest slow"
	case shared.StateBalanced:
		return " 🟢 optimal"
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

//nolint:exhaustive // Only handling specific key types
func (s SyncScreen) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		// Emergency exit - quit immediately
		return s, tea.Quit

	case tea.KeyEsc:
		// Cancel the sync gracefully
		s.cancelled = true
		if s.engine != nil {
			s.engine.Cancel()
		}

		return s, nil
	}

	// Handle other keys by string
	//nolint:gocritic // Single case switch is intentional for extensibility
	switch msg.String() {
	case "q":
		// Cancel the sync gracefully
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

			// Verbose instrumentation: log what the UI sees
			if s.status != nil && len(s.status.CurrentFiles) > 0 {
				var fileStatuses []string
				// Check for SMB contention (finalizing files blocking opening files)
				hasFinalizingFiles := false
				for _, f := range s.status.FilesToSync {
					if f.Status == "finalizing" {
						hasFinalizingFiles = true
						break
					}
				}

				for _, filePath := range s.status.CurrentFiles {
					// Find the file in FilesToSync to get progress
					for _, f := range s.status.FilesToSync {
						if f.RelativePath == filePath {
							if f.Status == "copying" {
								var percent float64
								if f.Size > 0 {
									percent = float64(f.Transferred) / float64(f.Size) * 100 //nolint:mnd // Percentage calculation
								}
								fileStatuses = append(fileStatuses, fmt.Sprintf("%s:%.1f%%", filePath, percent))
							} else if f.Status == "finalizing" {
								fileStatuses = append(fileStatuses, filePath+":finalizing")
							} else if f.Status == "opening" {
								if hasFinalizingFiles {
									fileStatuses = append(fileStatuses, filePath+":opening(SMB_BUSY)")
								} else {
									fileStatuses = append(fileStatuses, filePath+":opening")
								}
							}

							break
						}
					}
				}
				s.engine.LogVerbose(fmt.Sprintf("[PROGRESS] UI_POLL: files_copying=%d [%s]",
					len(s.status.CurrentFiles), strings.Join(fileStatuses, ", ")))
			}
		}
	}

	return s, shared.TickCmd()
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

	// File progress bars are kept small (max 20 chars) to leave room for file names
	fileProgressWidth := min(progressWidth, 20)
	s.fileProgress.Width = fileProgressWidth

	return s, nil
}

// ============================================================================
// Rendering
// ============================================================================

func (s SyncScreen) renderCancellationProgress() string {
	var builder strings.Builder

	// Header with spinner
	builder.WriteString(shared.RenderTitle("🚫 Cancelling Sync"))
	builder.WriteString("\n\n")

	// Spinner with status message
	builder.WriteString(s.spinner.View())
	builder.WriteString(" Waiting for workers to finish...\n\n")

	// Show active worker count
	activeWorkers := 0
	if s.status != nil {
		activeWorkers = int(s.status.ActiveWorkers)
	}

	fmt.Fprintf(&builder, "Active workers: %d\n\n", activeWorkers)

	// Show files being finalized
	builder.WriteString(shared.RenderLabel("Files being finalized:"))
	builder.WriteString("\n")

	if s.status != nil && len(s.status.CurrentFiles) > 0 {
		// Show up to 3 files
		maxFiles := 3
		filesToShow := min(len(s.status.CurrentFiles), maxFiles)

		for i := range filesToShow {
			builder.WriteString("  • ")
			builder.WriteString(s.status.CurrentFiles[i])
			builder.WriteString("\n")
		}

		// Show overflow message if there are more files
		if len(s.status.CurrentFiles) > maxFiles {
			builder.WriteString(shared.RenderDim(fmt.Sprintf("  ... and %d more files\n", len(s.status.CurrentFiles)-maxFiles)))
		}
	} else {
		builder.WriteString(shared.RenderDim("  (none)\n"))
	}

	builder.WriteString("\n")

	// Force-quit hint
	builder.WriteString(shared.RenderDim("Press Ctrl+C to force quit"))
	builder.WriteString("\n")

	return shared.RenderBox(builder.String(), s.width, s.height)
}

func (s SyncScreen) renderCurrentlyCopying(builder *strings.Builder, maxFilesToShow int) {
	// Count how many files are actually copying and display them
	totalCopying := 0
	filesDisplayed := 0

	builder.WriteString(shared.RenderLabel(fmt.Sprintf("Currently Copying (%d):", len(s.status.CurrentFiles))))
	builder.WriteString("\n")

	// Calculate available width for path display
	// Format: [spinner(1)] [space] [progress bar(dynamic)] [space] [percentage(8)] [space] [path]

	// Account for box border (1+1) and padding (2+2)
	boxOverhead := 6
	contentWidth := s.width - boxOverhead

	// Calculate fixed width for non-path elements
	spinnerWidth := 1
	progressBarWidth := s.fileProgress.Width // Use actual dynamic progress bar width
	percentageWidth := 8                     // " (100.0%)"
	spacing := 4                             // spaces between components
	fixedWidth := spinnerWidth + progressBarWidth + percentageWidth + spacing

	// Calculate available width for path
	maxPathWidth := max(contentWidth-fixedWidth, syncengine.MinPathDisplayWidth)

	// Check if any files are finalizing (for SMB contention detection)
	hasFinalizingFiles := false
	for _, file := range s.status.FilesToSync {
		if file.Status == "finalizing" {
			hasFinalizingFiles = true
			break
		}
	}

	// Display up to maxFilesToShow files
	for _, file := range s.status.FilesToSync {
		if file.Status == "copying" {
			totalCopying++

			if filesDisplayed < maxFilesToShow {
				// Calculate file progress percentage
				var filePercent float64
				if file.Size > 0 {
					filePercent = float64(file.Transferred) / float64(file.Size)
				}

				// Truncate path to fit available width
				truncPath := shared.TruncatePath(file.RelativePath, maxPathWidth)

				// Single line: [spinner] [progress bar] [percentage] [path]
				fmt.Fprintf(builder, "%s %s (%.1f%%) %s\n",
					s.spinner.View(),
					shared.RenderProgress(s.fileProgress, filePercent),
					filePercent*shared.ProgressPercentageScale,
					shared.FileItemCopyingStyle().Render(truncPath))

				filesDisplayed++
			}
		} else if file.Status == "finalizing" {
			totalCopying++

			if filesDisplayed < maxFilesToShow {
				// Truncate path to fit available width
				truncPath := shared.TruncatePath(file.RelativePath, maxPathWidth)

				// Single line: [spinner] Finalizing [path]
				fmt.Fprintf(builder, "%s Finalizing %s\n",
					s.spinner.View(),
					shared.FileItemCopyingStyle().Render(truncPath))

				filesDisplayed++
			}
		} else if file.Status == "opening" {
			totalCopying++

			if filesDisplayed < maxFilesToShow {
				// Truncate path to fit available width
				truncPath := shared.TruncatePath(file.RelativePath, maxPathWidth)

				// Show SMB contention message if other files are finalizing
				var statusMsg string
				if hasFinalizingFiles {
					statusMsg = "Waiting (SMB busy)"
				} else {
					statusMsg = "Opening file"
				}

				// Single line: [spinner] [status message] [path]
				fmt.Fprintf(builder, "%s %s %s\n",
					s.spinner.View(),
					statusMsg,
					shared.FileItemCopyingStyle().Render(truncPath))

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

func (s SyncScreen) renderRecentFiles(builder *strings.Builder, maxFilesToShow int) {
	// Show recent files when nothing is currently copying
	// Only show header if there are files to display
	if len(s.status.FilesToSync) == 0 {
		return
	}

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
			icon = shared.SuccessSymbol()
		case "copying":
			style = shared.FileItemCopyingStyle()
			icon = s.spinner.View()
		case "error":
			style = shared.FileItemErrorStyle()
			icon = shared.ErrorSymbol()
		default:
			style = shared.FileItemStyle()
			icon = shared.PendingSymbol()
		}

		fmt.Fprintf(builder, "%s %s\n", icon, style.Render(file.RelativePath))
	}
}

func (s SyncScreen) renderStatistics(builder *strings.Builder) {
	// Worker count with bottleneck info and read/write percentage
	bottleneckInfo := s.getBottleneckInfo()
	fmt.Fprintf(builder, "Workers: %d%s",
		s.status.ActiveWorkers,
		bottleneckInfo)

	// Read/write percentage (from rolling window)
	if s.status.Workers.ReadPercent > 0 || s.status.Workers.WritePercent > 0 {
		fmt.Fprintf(builder, " • R:%.0f%% / W:%.0f%%",
			s.status.Workers.ReadPercent,
			s.status.Workers.WritePercent)
	}

	builder.WriteString("\n")

	// Per-worker and total rates on separate line (from rolling window)
	if s.status.Workers.TotalRate > 0 {
		fmt.Fprintf(builder, "Speed: %s/worker • %s total",
			shared.FormatRate(s.status.Workers.PerWorkerRate),
			shared.FormatRate(s.status.Workers.TotalRate))
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
}

func (s SyncScreen) renderSyncingErrors(builder *strings.Builder) {
	// Show errors if any
	if len(s.status.Errors) == 0 {
		return
	}

	builder.WriteString("\n")
	builder.WriteString(shared.RenderError(fmt.Sprintf("⚠ Errors (%d):", len(s.status.Errors))))
	builder.WriteString("\n")

	// Use shared error rendering with in-progress context (3 error limit)
	config := shared.ErrorListConfig{
		Errors:           s.status.Errors,
		Context:          shared.ContextInProgress,
		MaxWidth:         s.getMaxPathWidth(),
		TruncatePathFunc: shared.TruncatePath,
	}

	errorList := shared.RenderErrorList(config)
	builder.WriteString(errorList)
}

func (s SyncScreen) renderSyncingView() string {
	// If cancelled, show cancellation progress view
	if s.cancelled {
		return s.renderCancellationProgress()
	}

	var builder strings.Builder

	// Show different title based on finalization phase
	if s.status != nil && s.status.FinalizationPhase == "complete" {
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

		return shared.RenderBox(builder.String(), s.width, s.height)
	}

	// Unified progress (replaces separate overall and session progress)
	s.renderUnifiedProgress(&builder)

	// Statistics
	s.renderStatistics(&builder)

	// File list
	s.renderFileList(&builder)

	// Errors
	s.renderSyncingErrors(&builder)

	// Help text
	builder.WriteString("\n")
	builder.WriteString(shared.RenderDim("Press Esc or q to cancel sync • Ctrl+C to exit immediately"))

	return shared.RenderBox(builder.String(), s.width, s.height)
}

func (s SyncScreen) renderUnifiedProgress(builder *strings.Builder) {
	// Unified progress bar - shows average of files%, bytes%, and time%
	builder.WriteString(shared.RenderLabel("Progress:"))
	builder.WriteString("\n")

	// Render progress bar using overall percentage (average of three metrics)
	builder.WriteString(shared.RenderProgress(s.overallProgress, s.status.Progress.OverallPercent))
	builder.WriteString("\n")

	// Files line with percentage
	totalProcessedFiles := s.status.AlreadySyncedFiles + s.status.ProcessedFiles
	fmt.Fprintf(builder, "Files: %d / %d (%.1f%%)",
		totalProcessedFiles,
		s.status.TotalFilesInSource,
		s.status.Progress.FilesPercent*shared.ProgressPercentageScale)

	if s.status.FailedFiles > 0 {
		fmt.Fprintf(builder, " • %d failed", s.status.FailedFiles)
	}

	builder.WriteString("\n")

	// Bytes line with percentage (no rate/ETA - moved to worker line)
	totalProcessedBytes := s.status.AlreadySyncedBytes + s.status.TransferredBytes
	fmt.Fprintf(builder, "Bytes: %s / %s (%.1f%%)",
		shared.FormatBytes(totalProcessedBytes),
		shared.FormatBytes(s.status.TotalBytesInSource),
		s.status.Progress.BytesPercent*shared.ProgressPercentageScale)

	builder.WriteString("\n")

	// Time line: elapsed / estimated (percentage)
	elapsed := time.Since(s.status.StartTime)
	totalEstimated := elapsed + s.status.EstimatedTimeLeft

	fmt.Fprintf(builder, "Time: %s / %s (%.1f%%)",
		shared.FormatDuration(elapsed),
		shared.FormatDuration(totalEstimated),
		s.status.Progress.TimePercent*shared.ProgressPercentageScale)

	builder.WriteString("\n\n")
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

// unexported constants.
const (
	// maxRecentFilesToShow is the maximum number of recent files to display
	maxRecentFilesToShow = 5
)
