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

	// Use shared helper to ensure consistent configuration (ShowPercentage = false)
	overallProg := shared.NewProgressModel(0) // Width set later in resize
	fileProg := shared.NewProgressModel(0)    // Width set later in resize

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

// RenderContent returns just the content without timeline header or box wrapper.
// Used by UnifiedScreen to compose multiple screen contents together.
func (s SyncScreen) RenderContent() string {
	if s.cancelled {
		return s.renderCancellationContent()
	}

	return s.renderSyncingContent()
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

//nolint:cyclop,gocognit,nestif // TUI state management requires complex branching for status updates and rendering
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
					if f.Status == statusFinalizing {
						hasFinalizingFiles = true
						break
					}
				}

				for _, filePath := range s.status.CurrentFiles {
					// Find the file in FilesToSync to get progress
					for _, f := range s.status.FilesToSync { //nolint:varnamelen // f is idiomatic iterator for file
						if f.RelativePath == filePath {
							if f.Status == statusCopying { //nolint:gocritic,staticcheck,lll // if-else chain clearer than switch for status checks with different conditions
								var percent float64
								if f.Size > 0 {
									percent = float64(f.Transferred) / float64(f.Size) * 100 //nolint:mnd // Percentage calculation
								}
								fileStatuses = append(fileStatuses, fmt.Sprintf("%s:%.1f%%", filePath, percent))
							} else if f.Status == statusFinalizing {
								fileStatuses = append(fileStatuses, filePath+":finalizing")
							} else if f.Status == statusOpening {
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
	fileProgressWidth := min(progressWidth, 20) //nolint:mnd // UI display limit for progress bar width
	s.fileProgress.Width = fileProgressWidth

	return s, nil
}

// ============================================================================
// Rendering
// ============================================================================

func (s SyncScreen) renderCancellationProgress() string {
	// Timeline header + content + box wrapper
	var builder strings.Builder
	builder.WriteString(shared.RenderTimeline("sync"))
	builder.WriteString("\n\n")
	builder.WriteString(s.renderCancellationContent())
	return shared.RenderBox(builder.String(), s.width, s.height)
}

// renderCancellationContent returns just the cancellation content without timeline or box.
func (s SyncScreen) renderCancellationContent() string {
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

	return builder.String()
}

// Note: renderCurrentlyCopying, renderFileList, renderRecentFiles, renderStatistics
// moved to AnalysisScreen for unified live sync display

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

	// Timeline header + content + help text + box wrapper (standalone mode)
	var builder strings.Builder
	builder.WriteString(shared.RenderTimeline("sync"))
	builder.WriteString("\n\n")
	builder.WriteString(s.renderSyncingContent())
	builder.WriteString("\n")
	builder.WriteString(shared.RenderDim("Esc or q to cancel • Ctrl+C to exit immediately"))
	return shared.RenderBox(builder.String(), s.width, s.height)
}

// renderSyncingContent returns just the sync content without timeline or box.
func (s SyncScreen) renderSyncingContent() string {
	var builder strings.Builder

	// Note: "Syncing: Source → Dest" header now shown by UnifiedScreen

	// Show finalization message when in that phase
	if s.status != nil && s.status.FinalizationPhase == statusComplete {
		builder.WriteString(shared.RenderLabel("Finalizing: "))
		builder.WriteString("Updating destination cache...")
		builder.WriteString("\n")
		builder.WriteString(shared.RenderDim("(This helps the next sync run faster)"))
		builder.WriteString("\n\n")
	}

	if s.status == nil {
		builder.WriteString(s.spinner.View())
		builder.WriteString(" Starting sync...\n\n")

		return builder.String()
	}

	// Show "Starting sync..." until actual sync activity begins
	// This prevents a confusing jump to high progress (e.g., 79%) from already-synced files
	syncActivityStarted := s.status.ProcessedFiles > 0 ||
		s.status.TransferredBytes > 0 ||
		len(s.status.CurrentFiles) > 0

	if !syncActivityStarted {
		builder.WriteString(s.spinner.View())
		builder.WriteString(" Starting sync...\n\n")

		return builder.String()
	}

	// Note: Copying section (progress bars, workers, files) now shown in analysis screen
	// with live-updating counts

	// Errors only - all other sync info is now in the analysis section
	s.renderSyncingErrors(&builder)

	// Note: Help text removed - shown by unified screen based on active phase
	return builder.String()
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
	statusComplete       = "complete"
	statusCopying        = "copying"
	statusFinalizing     = "finalizing"
	statusOpening        = "opening"
)
