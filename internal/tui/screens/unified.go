package screens

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
	"github.com/joe/copy-files/internal/tui/widgets"
)

// WorkflowPhase represents the current phase of the workflow
type WorkflowPhase string

const (
	PhaseInput        WorkflowPhase = "input"
	PhaseAnalyzing    WorkflowPhase = "analyzing"
	PhaseConfirmation WorkflowPhase = "confirmation"
	PhaseSyncing      WorkflowPhase = "syncing"
	PhaseSummary      WorkflowPhase = "summary"
)

// UnifiedScreen is the single screen that manages the entire workflow
type UnifiedScreen struct {
	// Current phase
	phase WorkflowPhase

	// Widget registry
	widgets *shared.WidgetRegistry

	// Configuration and state
	config  *config.Config
	engine  *syncengine.Engine
	status  *syncengine.Status
	logPath string
	err     error

	// Input widgets (for input phase)
	sourceInput  textinput.Model
	destInput    textinput.Model
	patternInput textinput.Model
	focusIndex   int

	// Animated components
	spinner spinner.Model

	// Terminal dimensions
	width  int
	height int
}

// NewUnifiedScreen creates a new unified screen starting in input phase
func NewUnifiedScreen(cfg *config.Config) *UnifiedScreen {
	// Create input fields
	sourceInput := textinput.New()
	sourceInput.Placeholder = "Source path"
	sourceInput.Focus()
	sourceInput.CharLimit = 256
	sourceInput.Width = 50

	destInput := textinput.New()
	destInput.Placeholder = "Destination path"
	destInput.CharLimit = 256
	destInput.Width = 50

	patternInput := textinput.New()
	patternInput.Placeholder = "File pattern (optional, e.g. *.mov)"
	patternInput.CharLimit = 100
	patternInput.Width = 50

	// Create spinner
	spin := spinner.New()
	spin.Spinner = spinner.Dot

	return &UnifiedScreen{
		phase:        PhaseInput,
		widgets:      shared.NewWidgetRegistry(),
		config:       cfg,
		sourceInput:  sourceInput,
		destInput:    destInput,
		patternInput: patternInput,
		focusIndex:   0,
		spinner:      spin,
	}
}

// Init initializes the unified screen
func (s *UnifiedScreen) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		s.spinner.Tick,
		shared.TickCmd(),
	)
}

// Update handles all messages for the unified screen
func (s *UnifiedScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case tea.KeyMsg:
		return s.handleKeyMsg(msg)

	case shared.TickMsg:
		return s.handleTick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd

	case shared.StatusUpdateMsg:
		// Update status so widgets can access latest data
		s.status = msg.Status
		return s, nil

	case shared.AnalysisCompleteMsg:
		return s.handleAnalysisComplete()

	case shared.SyncCompleteMsg:
		return s.handleSyncComplete()

	case shared.ErrorMsg:
		s.err = msg.Err
		s.phase = PhaseSummary

		// Add Summary widget to display error
		s.widgets.Add(shared.NewWidget(
			"summary",
			shared.WidgetTypeSummary,
			widgets.NewSummaryWidget(s.getStatus, msg.Err),
		))

		return s, nil
	}

	return s, nil
}

// View renders the unified screen
func (s *UnifiedScreen) View() string {
	// Render based on current phase
	switch s.phase {
	case PhaseInput:
		return s.renderInputPhase()
	case PhaseAnalyzing:
		return s.renderAnalyzingPhase()
	case PhaseConfirmation:
		return s.renderConfirmationPhase()
	case PhaseSyncing:
		return s.renderSyncingPhase()
	case PhaseSummary:
		return s.renderSummaryPhase()
	default:
		return "Unknown phase"
	}
}

// handleKeyMsg routes key handling based on current phase
func (s *UnifiedScreen) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	//nolint:exhaustive // Default case handles other keys
	switch msg.Type {
	case tea.KeyCtrlC:
		return s, tea.Quit
	}

	// Phase-specific key handling
	switch s.phase {
	case PhaseInput:
		return s.handleInputPhaseKeys(msg)
	case PhaseAnalyzing:
		return s.handleAnalyzingPhaseKeys(msg)
	case PhaseConfirmation:
		return s.handleConfirmationPhaseKeys(msg)
	case PhaseSyncing:
		return s.handleSyncingPhaseKeys(msg)
	case PhaseSummary:
		return s.handleSummaryPhaseKeys(msg)
	}

	return s, nil
}

// handleTick updates animations and polls status
func (s *UnifiedScreen) handleTick() (tea.Model, tea.Cmd) {
	// Update all widget animations
	s.widgets.UpdateAll(0.016) // Assume ~60 FPS

	// Poll engine status if in active phases
	if s.engine != nil && (s.phase == PhaseAnalyzing || s.phase == PhaseSyncing) {
		s.status = s.engine.GetStatus()
	}

	return s, shared.TickCmd()
}

// renderInputPhase renders the input phase with text input fields
func (s *UnifiedScreen) renderInputPhase() string {
	var content string

	// Timeline header
	timeline := shared.RenderTimeline("input")
	content += timeline + "\n\n"

	// Phase indicator for testing
	content += "<!-- phase: input -->\n"

	// Title
	content += shared.RenderTitle("🚀 File Sync Setup") + "\n\n"

	// Input fields
	content += shared.RenderLabel("Source Path:") + "\n"
	content += s.sourceInput.View() + "\n\n"

	content += shared.RenderLabel("Destination Path:") + "\n"
	content += s.destInput.View() + "\n\n"

	content += shared.RenderLabel("File Pattern (optional):") + "\n"
	content += s.patternInput.View() + "\n\n"

	// Help text
	content += shared.RenderDim("Tab/Shift+Tab to navigate • Enter to start • Ctrl+C to exit")

	return shared.RenderBox(content, s.width)
}

func (s *UnifiedScreen) renderAnalyzingPhase() string {
	var content string

	// Timeline header (scan phase)
	timeline := shared.RenderTimeline("scan")
	content += timeline + "\n\n"

	// Phase indicator for testing
	content += "<!-- phase: scan -->\n"

	// Render all widgets in order
	content += s.renderAllWidgets()

	return shared.RenderBox(content, s.width)
}

func (s *UnifiedScreen) renderConfirmationPhase() string {
	var content string

	// Timeline header
	timeline := shared.RenderTimeline("compare")
	content += timeline + "\n\n"

	// Phase indicator for testing
	content += "<!-- phase: compare -->\n"

	// Render all widgets in order
	content += s.renderAllWidgets()

	return shared.RenderBox(content, s.width)
}

func (s *UnifiedScreen) renderSyncingPhase() string {
	var content string

	// Timeline header
	timeline := shared.RenderTimeline("sync")
	content += timeline + "\n\n"

	// Phase indicator for testing
	content += "<!-- phase: sync -->\n"

	// Render all widgets in order
	content += s.renderAllWidgets()

	return shared.RenderBox(content, s.width)
}

func (s *UnifiedScreen) renderSummaryPhase() string {
	var content string

	// Timeline header based on final state
	var phase string
	if s.err != nil {
		phase = "done_error"
	} else {
		phase = "done"
	}
	timeline := shared.RenderTimeline(phase)
	content += timeline + "\n\n"

	// Phase indicator for testing
	content += "<!-- phase: " + phase + " -->\n"

	// Render all widgets in order
	content += s.renderAllWidgets()

	return shared.RenderBox(content, s.width)
}

// renderAllWidgets renders all widgets with their animation states applied
func (s *UnifiedScreen) renderAllWidgets() string {
	var content strings.Builder

	for _, widget := range s.widgets.All() {
		// Get widget content
		widgetContent := widget.Render()

		// Apply animation effects
		offset := widget.GetOffset()
		opacity := widget.GetOpacity()

		// Apply fade style based on opacity
		styledContent := shared.ApplyFadeStyle(widgetContent, opacity)

		// Apply offset (add blank lines above if animating in)
		if offset < 0 {
			// Widget is above viewport, add blank lines to push it down
			blankLines := int(-offset)
			for i := 0; i < blankLines && i < 10; i++ {
				content.WriteString("\n")
			}
		}

		// Add the widget content
		content.WriteString(styledContent + "\n\n")
	}

	return content.String()
}

// handleInputPhaseKeys handles keys during input phase
func (s *UnifiedScreen) handleInputPhaseKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	//nolint:exhaustive // Default case handles other keys
	switch msg.Type {
	case tea.KeyEnter:
		// Validate and transition to analyzing phase
		return s.startAnalysis()

	case tea.KeyTab, tea.KeyShiftTab:
		// Navigate between inputs
		if msg.Type == tea.KeyTab {
			s.focusIndex = (s.focusIndex + 1) % 3
		} else {
			s.focusIndex = (s.focusIndex - 1 + 3) % 3
		}

		// Update focus
		cmds := s.updateInputFocus()
		return s, tea.Batch(cmds...)

	default:
		// Update the focused input
		switch s.focusIndex {
		case 0:
			s.sourceInput, cmd = s.sourceInput.Update(msg)
		case 1:
			s.destInput, cmd = s.destInput.Update(msg)
		case 2:
			s.patternInput, cmd = s.patternInput.Update(msg)
		}
		return s, cmd
	}
}

func (s *UnifiedScreen) handleAnalyzingPhaseKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	//nolint:exhaustive // Default case handles other keys
	switch msg.Type {
	case tea.KeyEsc:
		// Cancel analysis and return to input
		if s.engine != nil {
			s.engine.Cancel()
		}
		return s.transitionToInput()
	}
	return s, nil
}

func (s *UnifiedScreen) handleConfirmationPhaseKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	//nolint:exhaustive // Default case handles other keys
	switch msg.Type {
	case tea.KeyEnter:
		// Start sync
		return s.startSync()
	case tea.KeyEsc:
		// Return to input
		return s.transitionToInput()
	}
	return s, nil
}

func (s *UnifiedScreen) handleSyncingPhaseKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	//nolint:exhaustive // Default case handles other keys
	switch msg.Type {
	case tea.KeyEsc, tea.KeyRunes:
		if msg.Type == tea.KeyRunes && msg.String() == "q" {
			// Cancel sync
			if s.engine != nil {
				s.engine.Cancel()
			}
		}
	}
	return s, nil
}

func (s *UnifiedScreen) handleSummaryPhaseKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	//nolint:exhaustive // Default case handles other keys
	switch msg.Type {
	case tea.KeyEnter, tea.KeyRunes:
		if msg.Type == tea.KeyRunes && msg.String() == "q" {
			return s, tea.Quit
		} else if msg.Type == tea.KeyEnter {
			return s, tea.Quit
		}
	case tea.KeyEsc:
		// Start new session
		return s.transitionToInput()
	}
	return s, nil
}

// updateInputFocus updates which input field has focus
func (s *UnifiedScreen) updateInputFocus() []tea.Cmd {
	cmds := make([]tea.Cmd, 3)

	if s.focusIndex == 0 {
		cmds[0] = s.sourceInput.Focus()
		s.destInput.Blur()
		s.patternInput.Blur()
	} else if s.focusIndex == 1 {
		s.sourceInput.Blur()
		cmds[1] = s.destInput.Focus()
		s.patternInput.Blur()
	} else {
		s.sourceInput.Blur()
		s.destInput.Blur()
		cmds[2] = s.patternInput.Focus()
	}

	return cmds
}

// startAnalysis validates inputs and starts the analysis phase
func (s *UnifiedScreen) startAnalysis() (tea.Model, tea.Cmd) {
	// Validate paths - stay in input phase if invalid
	sourcePath := s.sourceInput.Value()
	destPath := s.destInput.Value()

	if sourcePath == "" || destPath == "" {
		return s, nil
	}

	// Create engine
	engine, err := syncengine.NewEngine(sourcePath, destPath)
	if err != nil {
		s.err = err
		s.phase = PhaseSummary
		return s, nil
	}

	s.engine = engine
	s.engine.FilePattern = s.patternInput.Value()
	s.engine.Verbose = s.config.Verbose
	s.engine.Workers = s.config.Workers
	s.engine.AdaptiveMode = s.config.AdaptiveMode
	s.engine.ChangeType = s.config.TypeOfChange

	// Add analyzing widgets with animation
	s.widgets.Add(shared.NewWidget(
		"phase",
		shared.WidgetTypePhase,
		widgets.NewPhaseWidget("analyzing"),
	))

	s.widgets.Add(shared.NewWidget(
		"progress",
		shared.WidgetTypeProgress,
		widgets.NewProgressWidget(s.getStatus),
	))

	s.widgets.Add(shared.NewWidget(
		"activitylog",
		shared.WidgetTypeActivityLog,
		widgets.NewActivityLogWidget(s.getActivities),
	))

	// Transition to analyzing phase
	s.phase = PhaseAnalyzing

	// Start background analysis
	return s, s.runAnalysis()
}

// runAnalysis returns a command that runs analysis in the background
func (s *UnifiedScreen) runAnalysis() tea.Cmd {
	engine := s.engine

	return func() tea.Msg {
		err := engine.Analyze()
		if err != nil {
			return shared.ErrorMsg{Err: err}
		}

		return shared.AnalysisCompleteMsg{}
	}
}

// startSync begins the sync phase
func (s *UnifiedScreen) startSync() (tea.Model, tea.Cmd) {
	// Add syncing widgets with animation
	s.widgets.Add(shared.NewWidget(
		"filelist",
		shared.WidgetTypeFileList,
		widgets.NewFileListWidget(s.getStatus),
	))

	s.widgets.Add(shared.NewWidget(
		"workerstats",
		shared.WidgetTypeWorkerStats,
		widgets.NewWorkerStatsWidget(s.getStatus),
	))

	// Transition to syncing phase
	s.phase = PhaseSyncing

	// Start background sync
	return s, s.runSync()
}

// runSync returns a command that runs sync in the background
func (s *UnifiedScreen) runSync() tea.Cmd {
	engine := s.engine

	return func() tea.Msg {
		err := engine.Sync()
		if err != nil {
			return shared.ErrorMsg{Err: err}
		}

		return shared.SyncCompleteMsg{}
	}
}

// handleAnalysisComplete handles the AnalysisCompleteMsg
func (s *UnifiedScreen) handleAnalysisComplete() (tea.Model, tea.Cmd) {
	// Add SyncPlan widget
	s.widgets.Add(shared.NewWidget(
		"syncplan",
		shared.WidgetTypeSyncPlan,
		widgets.NewSyncPlanWidget(s.getStatus),
	))

	// Transition to confirmation phase
	s.phase = PhaseConfirmation

	return s, nil
}

// handleSyncComplete handles the SyncCompleteMsg
func (s *UnifiedScreen) handleSyncComplete() (tea.Model, tea.Cmd) {
	// Add Summary widget
	s.widgets.Add(shared.NewWidget(
		"summary",
		shared.WidgetTypeSummary,
		widgets.NewSummaryWidget(s.getStatus, nil),
	))

	// Transition to summary phase
	s.phase = PhaseSummary

	return s, nil
}

// transitionToInput resets to input phase
func (s *UnifiedScreen) transitionToInput() (tea.Model, tea.Cmd) {
	s.phase = PhaseInput
	s.widgets = shared.NewWidgetRegistry() // Clear all widgets
	s.engine = nil
	s.status = nil
	s.err = nil
	return s, nil
}

// getStatus returns current engine status
func (s *UnifiedScreen) getStatus() *syncengine.Status {
	// Return stored status if available (from StatusUpdateMsg)
	if s.status != nil {
		return s.status
	}

	// Otherwise poll engine directly
	if s.engine == nil {
		return nil
	}

	return s.engine.GetStatus()
}

// getActivities returns recent activity log entries
func (s *UnifiedScreen) getActivities() []string {
	// TODO: Implement activity tracking
	// For now, return empty slice
	return []string{}
}
