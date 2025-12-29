package screens

import (
	"fmt"
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
}

// NewAnalysisScreen creates a new analysis screen
func NewAnalysisScreen(cfg *config.Config) *AnalysisScreen {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(shared.PrimaryColor())

	overallProg := progress.New(
		progress.WithDefaultGradient(),
	)

	return &AnalysisScreen{
		config:          cfg,
		spinner:         s,
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
		tickCmd(),
	)
}

// Update implements tea.Model
func (s AnalysisScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return s.handleWindowSize(msg)
	case shared.EngineInitializedMsg:
		return s.handleEngineInitialized(msg)
	case shared.AnalysisCompleteMsg:
		return s.handleAnalysisComplete()
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
func (s AnalysisScreen) View() string {
	if s.state == "initializing" {
		return s.renderInitializingView()
	}

	return s.renderAnalyzingView()
}

func (s AnalysisScreen) getAnalysisPhaseText() string {
	switch s.status.AnalysisPhase {
	case "counting_source":
		return "Counting files in source..."
	case "scanning_source":
		return "Scanning source directory..."
	case "counting_dest":
		return "Counting files in destination..."
	case "scanning_dest":
		return "Scanning destination directory..."
	case "comparing":
		return "Comparing files to determine sync plan..."
	case "deleting":
		return "Checking for files to delete..."
	case shared.StateComplete:
		return "Analysis complete!"
	default:
		return "Initializing..."
	}
}

func (s AnalysisScreen) handleAnalysisComplete() (tea.Model, tea.Cmd) {
	// Transition to sync screen
	return s, func() tea.Msg {
		return shared.TransitionToSyncMsg{
			Engine: s.engine,
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

	return s, tea.Batch(
		func() tea.Msg {
			// Enable file logging for debugging (non-fatal if it fails)
			logPath := "copy-files-debug.log"
			_ = engine.EnableFileLogging(logPath)

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

func (s AnalysisScreen) handleSpinnerTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	s.spinner, cmd = s.spinner.Update(msg)

	return s, cmd
}

func (s AnalysisScreen) handleTick() (tea.Model, tea.Cmd) {
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
// Message Handlers
// ============================================================================

func (s AnalysisScreen) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	// Set progress bar width
	progressWidth := min(max(msg.Width-shared.ProgressUpdateInterval, shared.ProgressLogThreshold), shared.ProgressDetailedLogInterval)

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

func (s AnalysisScreen) renderAnalysisLog(b *strings.Builder) {
	if len(s.status.AnalysisLog) == 0 {
		return
	}

	b.WriteString(shared.RenderLabel("Activity Log:"))
	b.WriteString("\n")

	// Show last 10 log entries
	startIdx := 0
	if len(s.status.AnalysisLog) > shared.ProgressUpdateInterval {
		startIdx = len(s.status.AnalysisLog) - shared.ProgressUpdateInterval
	}

	for i := startIdx; i < len(s.status.AnalysisLog); i++ {
		fmt.Fprintf(b, "  %s\n", s.status.AnalysisLog[i])
	}
}

func (s AnalysisScreen) renderAnalysisProgress(b *strings.Builder) {
	switch s.status.AnalysisPhase {
	case "counting_source", "counting_dest":
		// Counting phase - show count so far
		if s.status.ScannedFiles > 0 {
			fmt.Fprintf(b, "Found: %d items so far...\n\n", s.status.ScannedFiles)
		}
	case "scanning_source", "scanning_dest", "comparing", "deleting":
		if s.status.TotalFilesToScan > 0 {
			// Show progress bar
			scanPercent := float64(s.status.ScannedFiles) / float64(s.status.TotalFilesToScan)
			b.WriteString(s.overallProgress.ViewAs(scanPercent))
			b.WriteString("\n")
			fmt.Fprintf(b, "%d / %d items (%.1f%%)\n\n",
				s.status.ScannedFiles,
				s.status.TotalFilesToScan,
				scanPercent*shared.ProgressPercentageScale)
		} else if s.status.ScannedFiles > 0 {
			// Fallback: show count without progress bar
			fmt.Fprintf(b, "Processed: %d items\n\n", s.status.ScannedFiles)
		}
	}
}

func (s AnalysisScreen) renderAnalyzingView() string {
	var b strings.Builder

	b.WriteString(shared.RenderTitle("🔍 Analyzing Files"))
	b.WriteString("\n\n")

	if s.status == nil {
		b.WriteString(s.spinner.View())
		b.WriteString(" Scanning directories and comparing files...\n\n")

		return shared.RenderBox(b.String())
	}

	// Show current phase
	phaseText := s.getAnalysisPhaseText()
	b.WriteString(s.spinner.View())
	b.WriteString(" ")
	b.WriteString(shared.RenderLabel(phaseText))
	b.WriteString("\n\n")

	// Show scan progress with progress bar or count
	s.renderAnalysisProgress(&b)

	// Show current path being scanned
	if s.status.CurrentPath != "" {
		b.WriteString(fmt.Sprintf("Current: %s\n", s.status.CurrentPath))
		b.WriteString("\n")
	}

	// Show errors if any
	if len(s.status.Errors) > 0 {
		b.WriteString(shared.RenderError(fmt.Sprintf("⚠ Errors: %d", len(s.status.Errors))))
		b.WriteString("\n\n")
	}

	// Show analysis log
	s.renderAnalysisLog(&b)

	return shared.RenderBox(b.String())
}

// ============================================================================
// Rendering
// ============================================================================

func (s AnalysisScreen) renderInitializingView() string {
	var b strings.Builder

	b.WriteString(shared.RenderTitle("🚀 Starting Copy Files"))
	b.WriteString("\n\n")

	b.WriteString(s.spinner.View())
	b.WriteString(" ")
	b.WriteString(shared.RenderLabel("Initializing..."))
	b.WriteString("\n\n")

	b.WriteString(shared.RenderDim("Setting up file logging and preparing to analyze directories"))
	b.WriteString("\n")

	return shared.RenderBox(b.String())
}

// ============================================================================
// Tick Command
// ============================================================================

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(shared.ProgressDetailedLogInterval*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
