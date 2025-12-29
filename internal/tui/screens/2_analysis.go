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
}

// NewAnalysisScreen creates a new analysis screen
func NewAnalysisScreen(cfg *config.Config) *AnalysisScreen {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(shared.PrimaryColor())

	overallProg := progress.New(
		progress.WithDefaultGradient(),
	)

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

func (s AnalysisScreen) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		// Cancel analysis if running
		if s.engine != nil {
			s.engine.Cancel()
		}

		// Transition back to input screen
		return s, func() tea.Msg {
			return shared.TransitionToInputMsg{}
		}
	}

	return s, nil
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
		return shared.EngineInitializedMsg{
			Engine: syncengine.NewEngine(s.config.SourcePath, s.config.DestPath),
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
	switch s.status.AnalysisPhase {
	case shared.PhaseCountingSource, shared.PhaseCountingDest:
		// Counting phase - show count so far
		if s.status.ScannedFiles > 0 {
			fmt.Fprintf(builder, "Found: %d items so far...\n\n", s.status.ScannedFiles)
		}
	case shared.PhaseScanningSource, shared.PhaseScanningDest, shared.PhaseComparing, shared.PhaseDeleting:
		if s.status.TotalFilesToScan > 0 {
			// Show progress bar
			scanPercent := float64(s.status.ScannedFiles) / float64(s.status.TotalFilesToScan)
			builder.WriteString(s.overallProgress.ViewAs(scanPercent))
			builder.WriteString("\n")
			fmt.Fprintf(builder, "%d / %d items (%.1f%%)\n\n",
				s.status.ScannedFiles,
				s.status.TotalFilesToScan,
				scanPercent*shared.ProgressPercentageScale)
		} else if s.status.ScannedFiles > 0 {
			// Fallback: show count without progress bar
			fmt.Fprintf(builder, "Processed: %d items\n\n", s.status.ScannedFiles)
		}
	}
}

func (s AnalysisScreen) renderAnalyzingView() string {
	var builder strings.Builder

	builder.WriteString(shared.RenderTitle("🔍 Analyzing Files"))
	builder.WriteString("\n\n")

	if s.status == nil {
		builder.WriteString(s.spinner.View())
		builder.WriteString(" Scanning directories and comparing files...\n\n")

		return shared.RenderBox(builder.String())
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
		builder.WriteString(fmt.Sprintf("Current: %s\n", s.status.CurrentPath))
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
	builder.WriteString(shared.RenderDim("Press Esc to change paths"))

	return shared.RenderBox(builder.String())
}

// ============================================================================
// Rendering
// ============================================================================

func (s AnalysisScreen) renderInitializingView() string {
	var builder strings.Builder

	builder.WriteString(shared.RenderTitle("🚀 Starting Copy Files"))
	builder.WriteString("\n\n")

	builder.WriteString(s.spinner.View())
	builder.WriteString(" ")
	builder.WriteString(shared.RenderLabel("Initializing..."))
	builder.WriteString("\n\n")

	builder.WriteString(shared.RenderDim("Setting up file logging and preparing to analyze directories"))
	builder.WriteString("\n\n")

	builder.WriteString(shared.RenderDim("Press Esc to change paths"))

	return shared.RenderBox(builder.String())
}
