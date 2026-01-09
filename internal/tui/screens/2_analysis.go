package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// AnalysisScreen handles engine initialization and file analysis
type AnalysisScreen struct {
	config          *config.Config
	engine          *syncengine.Engine
	status          *syncengine.Status
	spinner         spinner.Model
	overallProgress progress.Model
	state           string // "initializing" or "analyzing"
	lastUpdate      time.Time
	logPath         string
	width           int
	height          int
}

// NewAnalysisScreen creates a new analysis screen
func NewAnalysisScreen(cfg *config.Config) *AnalysisScreen {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(shared.PrimaryColor())

	// Use shared helper to ensure consistent configuration (ShowPercentage = false)
	overallProg := shared.NewProgressModel(0) // Width set later in resize

	return &AnalysisScreen{
		config:          cfg,
		spinner:         spin,
		overallProgress: overallProg,
		state:           "initializing",
		lastUpdate:      time.Now(),
	}
}

// Init implements tea.Model
func (s AnalysisScreen) Init() tea.Cmd {
	return tea.Batch(
		s.spinner.Tick,
		s.initializeEngine(),
		shared.TickCmd(),
	)
}

// Update implements tea.Model
func (s AnalysisScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return s.handleWindowSize(msg)
	case tea.KeyMsg:
		return s.handleKeyMsg(msg)
	case shared.EngineInitializedMsg:
		return s.handleEngineInitialized(msg)
	case shared.AnalysisCompleteMsg:
		return s.handleAnalysisComplete()
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
func (s AnalysisScreen) View() string {
	if s.state == "initializing" {
		return s.renderInitializingView()
	}

	return s.renderAnalyzingView()
}

// RenderContent returns just the content without timeline header or box wrapper.
// Used by UnifiedScreen to compose multiple screen contents together.
func (s AnalysisScreen) RenderContent() string {
	if s.state == "initializing" {
		return s.renderInitializingContent()
	}

	return s.renderAnalyzingContent()
}

func (s AnalysisScreen) getAnalysisPhaseText() string {
	switch s.status.AnalysisPhase {
	case shared.PhaseCountingSource:
		return "Counting files in source..."
	case shared.PhaseScanningSource:
		return "Scanning source directory..."
	case shared.PhaseCountingDest:
		return "Counting files in destination..."
	case shared.PhaseScanningDest:
		return "Scanning destination directory..."
	case shared.PhaseComparing:
		return "Comparing files to determine sync plan..."
	case shared.PhaseDeleting:
		return "Checking for files to delete..."
	case shared.StateComplete:
		return "Analysis complete!"
	default:
		return "Initializing..."
	}
}

func (s AnalysisScreen) handleAnalysisComplete() (tea.Model, tea.Cmd) {
	// Check if confirmation should be skipped
	if s.config.SkipConfirmation {
		// Skip confirmation and go directly to sync
		return s, func() tea.Msg {
			return shared.TransitionToSyncMsg{
				Engine:  s.engine,
				LogPath: s.logPath,
			}
		}
	}

	// Show confirmation screen
	return s, func() tea.Msg {
		return shared.TransitionToConfirmationMsg{
			Engine:  s.engine,
			LogPath: s.logPath,
		}
	}
}

func (s AnalysisScreen) handleEngineInitialized(msg shared.EngineInitializedMsg) (tea.Model, tea.Cmd) {
	// Store the engine and configure it
	s.engine = msg.Engine
	s.engine.Workers = s.config.Workers
	s.engine.AdaptiveMode = s.config.AdaptiveMode
	s.engine.ChangeType = s.config.TypeOfChange

	// Register status callback
	s.engine.RegisterStatusCallback(func(status *syncengine.Status) {
		s.status = status
	})

	// Capture engine in local variable for closures
	engine := s.engine

	// Start analysis
	s.state = "analyzing"

	// Determine log path
	s.logPath = os.Getenv("COPY_FILES_LOG")
	if s.logPath == "" {
		s.logPath = filepath.Join(os.TempDir(), "copy-files-debug.log")
	}

	return s, tea.Batch(
		func() tea.Msg {
			// Enable file logging for debugging (non-fatal if it fails)
			_ = engine.EnableFileLogging(s.logPath)

			return nil
		},
		func() tea.Msg {
			err := engine.Analyze()
			if err != nil {
				return shared.ErrorMsg{Err: err}
			}

			return shared.AnalysisCompleteMsg{}
		},
	)
}

func (s AnalysisScreen) handleError(msg shared.ErrorMsg) (tea.Model, tea.Cmd) {
	// Transition to summary screen with error
	return s, func() tea.Msg {
		return shared.TransitionToSummaryMsg{
			FinalState: "error",
			Err:        msg.Err,
		}
	}
}

//nolint:exhaustive // Only handling specific key types
func (s AnalysisScreen) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	//nolint:exhaustive // Default case handles all other keys
	switch msg.Type {
	case tea.KeyCtrlC:
		// Emergency exit - quit immediately
		return s, tea.Quit

	case tea.KeyEsc:
		// Cancel analysis if running
		if s.engine != nil {
			s.engine.Cancel()
		}

		// Transition back to input screen
		return s, func() tea.Msg {
			return shared.TransitionToInputMsg{}
		}
	default:
		// Ignore all other keys
		return s, nil
	}
}

func (s AnalysisScreen) handleSpinnerTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	s.spinner, cmd = s.spinner.Update(msg)

	return s, cmd
}

func (s AnalysisScreen) handleTick() (tea.Model, tea.Cmd) {
	// Update status from engine, but only every 200ms to reduce lock contention
	if s.engine != nil {
		now := time.Now()
		if now.Sub(s.lastUpdate) >= shared.StatusUpdateThrottleMs*time.Millisecond {
			s.status = s.engine.GetStatus()
			s.lastUpdate = now
		}
	}

	return s, shared.TickCmd()
}

// ============================================================================
// Message Handlers
// ============================================================================

func (s AnalysisScreen) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	s.width = msg.Width
	s.height = msg.Height

	// Set progress bar width
	//nolint:lll // Complex width calculation with multiple constants
	progressWidth := min(max(msg.Width-shared.ProgressUpdateInterval, shared.ProgressLogThreshold), shared.MaxProgressBarWidth)

	s.overallProgress.Width = progressWidth

	return s, nil
}

// ============================================================================
// Engine Initialization
// ============================================================================

func (s AnalysisScreen) initializeEngine() tea.Cmd {
	return func() tea.Msg {
		engine, err := syncengine.NewEngine(s.config.SourcePath, s.config.DestPath)
		if err != nil {
			return shared.ErrorMsg{Err: fmt.Errorf("failed to initialize engine: %w", err)}
		}

		engine.FilePattern = s.config.FilePattern
		engine.Verbose = s.config.Verbose

		return shared.EngineInitializedMsg{
			Engine: engine,
		}
	}
}

func (s AnalysisScreen) renderAnalysisLog(builder *strings.Builder) {
	if len(s.status.AnalysisLog) == 0 {
		return
	}

	builder.WriteString(shared.RenderLabel("Activity Log:"))
	builder.WriteString("\n")

	// Show last 10 log entries
	startIdx := 0
	if len(s.status.AnalysisLog) > shared.ProgressUpdateInterval {
		startIdx = len(s.status.AnalysisLog) - shared.ProgressUpdateInterval
	}

	for i := startIdx; i < len(s.status.AnalysisLog); i++ {
		fmt.Fprintf(builder, "  %s\n", s.status.AnalysisLog[i])
	}
}

func (s AnalysisScreen) renderAnalysisProgress(builder *strings.Builder) {
	if s.engine == nil || s.status == nil {
		return
	}

	// Calculate progress metrics
	progress := s.status.CalculateAnalysisProgress()

	// Route to appropriate renderer based on phase
	if progress.IsCounting {
		builder.WriteString(s.renderCountingProgress(s.status))
	} else {
		builder.WriteString(s.renderProcessingProgress(s.status, progress))
	}
}

func (s AnalysisScreen) renderAnalyzingView() string {
	// Timeline header + content + box wrapper
	var builder strings.Builder
	builder.WriteString(shared.RenderTimeline("scan"))
	builder.WriteString("\n\n")
	builder.WriteString(s.renderAnalyzingContent())
	return shared.RenderBox(builder.String(), s.width, s.height)
}

// renderAnalyzingContent returns just the analysis content without timeline or box.
func (s AnalysisScreen) renderAnalyzingContent() string {
	var builder strings.Builder

	builder.WriteString(shared.RenderTitle("🔍 Scanning Files"))
	builder.WriteString("\n\n")

	if s.status == nil {
		builder.WriteString(s.spinner.View())
		builder.WriteString(" Scanning directories and comparing files...\n\n")

		return builder.String()
	}

	// Show current phase
	phaseText := s.getAnalysisPhaseText()
	builder.WriteString(s.spinner.View())
	builder.WriteString(" ")
	builder.WriteString(shared.RenderLabel(phaseText))
	builder.WriteString("\n\n")

	// Show scan progress with progress bar or count
	s.renderAnalysisProgress(&builder)

	// Show current path being scanned
	if s.status.CurrentPath != "" {
		s.renderCurrentPathSection(&builder)
		builder.WriteString("\n")
	}

	// Show errors if any
	if len(s.status.Errors) > 0 {
		builder.WriteString(shared.RenderError(fmt.Sprintf("⚠ Errors: %d", len(s.status.Errors))))
		builder.WriteString("\n\n")
	}

	// Show analysis log
	s.renderAnalysisLog(&builder)

	// Show help text
	builder.WriteString("\n")
	builder.WriteString(shared.RenderDim("Press Esc to change paths • Ctrl+C to exit"))

	return builder.String()
}

func (s AnalysisScreen) renderCountingProgress(status *syncengine.Status) string {
	var builder strings.Builder

	// Show elapsed time
	var elapsed time.Duration
	if !status.AnalysisStartTime.IsZero() {
		elapsed = time.Since(status.AnalysisStartTime)
	}

	// Show items found
	builder.WriteString(fmt.Sprintf("Found: %d items", status.ScannedFiles))

	// Show scan rate if available
	if status.AnalysisRate > 0 {
		builder.WriteString(fmt.Sprintf(" (%.1f items/s)", status.AnalysisRate))
	}

	builder.WriteString("\n")

	// Show elapsed time
	if elapsed > 0 {
		builder.WriteString(fmt.Sprintf("Elapsed: %s\n", shared.FormatDuration(elapsed)))
	}

	builder.WriteString("\n")
	builder.WriteString(s.spinner.View())
	builder.WriteString(" Counting...")

	return builder.String()
}

func (s AnalysisScreen) renderCurrentPathSection(builder *strings.Builder) {
	maxWidth := shared.CalculateMaxPathWidth(s.width)
	truncatedPath := shared.RenderPathPlain(s.status.CurrentPath, maxWidth)
	fmt.Fprintf(builder, "Current: %s\n", truncatedPath)
}

// ============================================================================
// Rendering
// ============================================================================

func (s AnalysisScreen) renderInitializingView() string {
	// Timeline header + content + box wrapper
	var builder strings.Builder
	builder.WriteString(shared.RenderTimeline("scan"))
	builder.WriteString("\n\n")
	builder.WriteString(s.renderInitializingContent())
	return shared.RenderBox(builder.String(), s.width, s.height)
}

// renderInitializingContent returns just the initializing content without timeline or box.
func (s AnalysisScreen) renderInitializingContent() string {
	var builder strings.Builder

	builder.WriteString(shared.RenderTitle("🚀 Starting Copy Files"))
	builder.WriteString("\n\n")

	builder.WriteString(s.spinner.View())
	builder.WriteString(" ")
	builder.WriteString(shared.RenderLabel("Initializing..."))
	builder.WriteString("\n\n")

	builder.WriteString(shared.RenderDim("Setting up file logging and preparing to analyze directories"))
	builder.WriteString("\n\n")

	builder.WriteString(shared.RenderDim("Press Esc to change paths • Ctrl+C to exit"))

	return builder.String()
}

func (s AnalysisScreen) renderProcessingProgress(
	status *syncengine.Status,
	progress syncengine.ProgressMetrics,
) string {
	var builder strings.Builder

	// Progress bar using overall percentage
	builder.WriteString(s.overallProgress.ViewAs(progress.OverallPercent))
	builder.WriteString("\n")

	// Files line: "Files: 123 / 456 (27.0%)"
	fmt.Fprintf(&builder, "Files: %d / %d (%.1f%%)\n",
		status.ScannedFiles,
		status.TotalFilesToScan,
		progress.FilesPercent)

	// Bytes line: "Bytes: 1.2 MB / 4.5 MB (26.7%)"
	fmt.Fprintf(&builder, "Bytes: %s / %s (%.1f%%)\n",
		shared.FormatBytes(status.ScannedBytes),
		shared.FormatBytes(status.TotalBytesToScan),
		progress.BytesPercent)

	// Time line: "Time: 00:15 / 00:56 (26.8%)"
	var elapsed time.Duration
	if !status.AnalysisStartTime.IsZero() {
		elapsed = time.Since(status.AnalysisStartTime)
	}

	totalTime := elapsed + progress.EstimatedTimeRemaining
	fmt.Fprintf(&builder, "Time: %s / %s (%.1f%%)\n",
		shared.FormatDuration(elapsed),
		shared.FormatDuration(totalTime),
		progress.TimePercent)

	return builder.String()
}
