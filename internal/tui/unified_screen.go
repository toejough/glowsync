package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

// Phase represents the current workflow phase
type Phase int

const (
	PhaseInput Phase = iota
	PhaseScan
	PhaseCompare
	PhaseConfirm
	PhaseSync
	PhaseSummary
)

// String returns the phase name for timeline rendering
func (p Phase) String() string {
	switch p {
	case PhaseInput:
		return "input"
	case PhaseScan:
		return "scan"
	case PhaseCompare:
		return "compare"
	case PhaseConfirm:
		return "confirm"
	case PhaseSync:
		return "sync"
	case PhaseSummary:
		return "summary"
	default:
		return "input"
	}
}

// UnifiedScreen is a single-screen model that accumulates content as phases progress.
// Once a section appears, it persists for the lifetime of the app.
type UnifiedScreen struct {
	config *config.Config
	phase  Phase

	// All screens (value types - use has* flags for presence)
	input        screens.InputScreen
	analysis     screens.AnalysisScreen
	confirmation screens.ConfirmationScreen
	sync         screens.SyncScreen
	summary      screens.SummaryScreen

	// Presence flags
	hasInput        bool
	hasAnalysis     bool
	hasConfirmation bool
	hasSync         bool
	hasSummary      bool

	// Shared state
	engine  *syncengine.Engine
	logPath string
	width   int
	height  int
}

// Phase returns the current phase (for testing)
func (u *UnifiedScreen) Phase() Phase {
	return u.phase
}

// NewUnifiedScreen creates a new unified screen starting at input phase
func NewUnifiedScreen(cfg *config.Config) *UnifiedScreen {
	return &UnifiedScreen{
		config:   cfg,
		phase:    PhaseInput,
		input:    *screens.NewInputScreen(cfg),
		hasInput: true,
	}
}

// Init implements tea.Model
func (u *UnifiedScreen) Init() tea.Cmd {
	var cmds []tea.Cmd

	if u.hasInput {
		cmds = append(cmds, u.input.Init())
	}
	if u.hasAnalysis {
		cmds = append(cmds, u.analysis.Init())
	}

	return tea.Batch(cmds...)
}

// Update implements tea.Model
func (u *UnifiedScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle window size
	if windowMsg, ok := msg.(tea.WindowSizeMsg); ok {
		u.width = windowMsg.Width
		u.height = windowMsg.Height
		// Propagate to all active screens
		return u, u.propagateWindowSize(windowMsg)
	}

	// Handle phase transitions
	switch msg := msg.(type) {
	case shared.TransitionToAnalysisMsg:
		return u.transitionToAnalysis(msg)
	case shared.TransitionToConfirmationMsg:
		return u.transitionToConfirmation(msg)
	case shared.TransitionToSyncMsg:
		return u.transitionToSync(msg)
	case shared.TransitionToSummaryMsg:
		return u.transitionToSummary(msg)
	case shared.ConfirmSyncMsg:
		return u.transitionToSync(shared.TransitionToSyncMsg(msg))
	case shared.TransitionToInputMsg:
		// In unified mode, we don't go back - just reset phase if needed
		return u, nil
	}

	// Delegate to the active phase's screen
	return u.delegateToActiveScreen(msg)
}

// View implements tea.Model - renders all activated sections
func (u *UnifiedScreen) View() string {
	var sections []string

	// Timeline always at top
	sections = append(sections, shared.RenderTimeline(u.phase.String()))

	// During sync phase, show "Syncing: Source → Dest" header
	if u.phase == PhaseSync {
		sections = append(sections, shared.RenderLabel("Syncing: ")+"Source "+shared.RightArrow()+" Dest")
	}

	// Input section (always present)
	if u.hasInput {
		sections = append(sections, u.renderInputSection())
	}

	// Analysis section (once scan phase reached)
	if u.hasAnalysis {
		sections = append(sections, u.renderAnalysisSection())
	}

	// Confirmation section (once compare phase reached)
	if u.hasConfirmation {
		sections = append(sections, u.renderConfirmationSection())
	}

	// Sync section (once sync phase reached)
	if u.hasSync {
		sections = append(sections, u.renderSyncSection())
	}

	// Summary section (once done phase reached)
	if u.hasSummary {
		sections = append(sections, u.renderSummarySection())
	}

	// Add help text based on current phase
	sections = append(sections, u.renderHelpText())

	// Filter out empty sections before joining to avoid extra spacing
	var nonEmpty []string
	for _, s := range sections {
		if s != "" {
			nonEmpty = append(nonEmpty, s)
		}
	}
	content := strings.Join(nonEmpty, "\n\n")
	return shared.RenderBox(content, u.width, u.height)
}

// renderHelpText returns the appropriate help text for the current phase
func (u *UnifiedScreen) renderHelpText() string {
	switch u.phase {
	case PhaseInput:
		return "" // Input screen has its own help text
	case PhaseScan, PhaseCompare:
		return shared.RenderDim("Esc to go back • Ctrl+C to exit")
	case PhaseConfirm:
		return shared.RenderDim("Ready to sync? Press Enter to start • Esc to cancel")
	case PhaseSync:
		return shared.RenderDim("Esc or q to cancel • Ctrl+C to exit immediately")
	case PhaseSummary:
		return shared.RenderDim("Enter or q to exit • Esc for new session")
	default:
		return ""
	}
}

// ============================================================================
// Phase Transitions
// ============================================================================

func (u *UnifiedScreen) transitionToAnalysis(msg shared.TransitionToAnalysisMsg) (tea.Model, tea.Cmd) {
	u.config.SourcePath = msg.SourcePath
	u.config.DestPath = msg.DestPath
	u.phase = PhaseScan
	u.analysis = *screens.NewAnalysisScreen(u.config)
	u.hasAnalysis = true

	return u, tea.Batch(
		u.analysis.Init(),
		u.windowSizeCmd(),
	)
}

func (u *UnifiedScreen) transitionToConfirmation(msg shared.TransitionToConfirmationMsg) (tea.Model, tea.Cmd) {
	u.engine = msg.Engine
	u.logPath = msg.LogPath
	u.phase = PhaseConfirm
	u.confirmation = *screens.NewConfirmationScreen(msg.Engine, msg.LogPath)
	u.hasConfirmation = true

	return u, tea.Batch(
		u.confirmation.Init(),
		u.windowSizeCmd(),
	)
}

func (u *UnifiedScreen) transitionToSync(msg shared.TransitionToSyncMsg) (tea.Model, tea.Cmd) {
	u.engine = msg.Engine
	u.logPath = msg.LogPath
	u.phase = PhaseSync
	u.sync = *screens.NewSyncScreen(msg.Engine)
	u.hasSync = true

	// Enable live mode on analysis screen for dynamic count updates
	if u.hasAnalysis {
		u.analysis.EnableLiveMode()
	}

	return u, tea.Batch(
		u.sync.Init(),
		u.windowSizeCmd(),
	)
}

func (u *UnifiedScreen) transitionToSummary(msg shared.TransitionToSummaryMsg) (tea.Model, tea.Cmd) {
	u.phase = PhaseSummary
	u.summary = *screens.NewSummaryScreen(u.engine, msg.FinalState, msg.Err, u.logPath)
	u.hasSummary = true

	return u, tea.Batch(
		u.summary.Init(),
		u.windowSizeCmd(),
	)
}

// ============================================================================
// Message Delegation
// ============================================================================

func (u *UnifiedScreen) delegateToActiveScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// During sync phase, update analysis screen with live status on each tick
	if u.phase == PhaseSync && u.hasAnalysis {
		if _, ok := msg.(shared.TickMsg); ok && u.engine != nil {
			status := u.engine.GetStatus()
			u.analysis.UpdateLiveStatus(status)
		}
	}

	// Delegate to the screen for the current phase
	switch u.phase {
	case PhaseInput:
		if u.hasInput {
			var model tea.Model
			model, cmd = u.input.Update(msg)
			u.input = model.(screens.InputScreen)
		}
	case PhaseScan, PhaseCompare:
		// Both scan and compare phases use the analysis screen
		if u.hasAnalysis {
			var model tea.Model
			model, cmd = u.analysis.Update(msg)
			u.analysis = model.(screens.AnalysisScreen)
		}
	case PhaseConfirm:
		if u.hasConfirmation {
			var model tea.Model
			model, cmd = u.confirmation.Update(msg)
			u.confirmation = model.(screens.ConfirmationScreen)
		}
	case PhaseSync:
		if u.hasSync {
			var model tea.Model
			model, cmd = u.sync.Update(msg)
			u.sync = model.(screens.SyncScreen)
		}
	case PhaseSummary:
		if u.hasSummary {
			var model tea.Model
			model, cmd = u.summary.Update(msg)
			u.summary = model.(screens.SummaryScreen)
		}
	}

	return u, cmd
}

func (u *UnifiedScreen) propagateWindowSize(msg tea.WindowSizeMsg) tea.Cmd {
	var cmds []tea.Cmd

	if u.hasInput {
		model, cmd := u.input.Update(msg)
		u.input = model.(screens.InputScreen)
		cmds = append(cmds, cmd)
	}
	if u.hasAnalysis {
		model, cmd := u.analysis.Update(msg)
		u.analysis = model.(screens.AnalysisScreen)
		cmds = append(cmds, cmd)
	}
	if u.hasConfirmation {
		model, cmd := u.confirmation.Update(msg)
		u.confirmation = model.(screens.ConfirmationScreen)
		cmds = append(cmds, cmd)
	}
	if u.hasSync {
		model, cmd := u.sync.Update(msg)
		u.sync = model.(screens.SyncScreen)
		cmds = append(cmds, cmd)
	}
	if u.hasSummary {
		model, cmd := u.summary.Update(msg)
		u.summary = model.(screens.SummaryScreen)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

func (u *UnifiedScreen) windowSizeCmd() tea.Cmd {
	return func() tea.Msg {
		return tea.WindowSizeMsg{Width: u.width, Height: u.height}
	}
}

// ============================================================================
// Section Renderers - Extract content from each screen's View
// ============================================================================

func (u *UnifiedScreen) renderInputSection() string {
	if !u.hasInput {
		return ""
	}
	// Only show input during input phase - analysis screen shows paths after
	if u.phase == PhaseInput {
		return u.input.RenderContent()
	}
	return ""
}

func (u *UnifiedScreen) renderAnalysisSection() string {
	if !u.hasAnalysis {
		return ""
	}
	return u.analysis.RenderContent()
}

func (u *UnifiedScreen) renderConfirmationSection() string {
	if !u.hasConfirmation {
		return ""
	}
	return u.confirmation.RenderContent()
}

func (u *UnifiedScreen) renderSyncSection() string {
	if !u.hasSync {
		return ""
	}
	return u.sync.RenderContent()
}

func (u *UnifiedScreen) renderSummarySection() string {
	if !u.hasSummary {
		return ""
	}
	return u.summary.RenderContent()
}
