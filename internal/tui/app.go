package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

// AppModel is the top-level router that manages screen transitions
type AppModel struct {
	config        *config.Config
	currentScreen tea.Model
	engine        *syncengine.Engine
}

// NewAppModel creates a new app model
func NewAppModel(cfg *config.Config) *AppModel {
	var initialScreen tea.Model

	// If paths are provided via command-line flags, skip input screen
	if cfg.InteractiveMode {
		initialScreen = screens.NewInputScreen(cfg)
	} else {
		initialScreen = screens.NewAnalysisScreen(cfg)
	}

	return &AppModel{
		config:        cfg,
		currentScreen: initialScreen,
	}
}

// CurrentScreen returns the current screen (for testing)
func (a AppModel) CurrentScreen() tea.Model {
	return a.currentScreen
}

// Init implements tea.Model
func (a AppModel) Init() tea.Cmd {
	return a.currentScreen.Init()
}

// Update implements tea.Model
func (a AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check for transition messages first
	switch msg := msg.(type) {
	case shared.TransitionToAnalysisMsg:
		return a.transitionToAnalysis(msg)
	case shared.TransitionToSyncMsg:
		return a.transitionToSync(msg)
	case shared.TransitionToSummaryMsg:
		return a.transitionToSummary(msg)
	}

	// Otherwise, pass the message to the current screen
	var cmd tea.Cmd

	a.currentScreen, cmd = a.currentScreen.Update(msg)

	return a, cmd
}

// View implements tea.Model
func (a AppModel) View() string {
	return a.currentScreen.View()
}

// ============================================================================
// Screen Transitions
// ============================================================================

func (a AppModel) transitionToAnalysis(msg shared.TransitionToAnalysisMsg) (tea.Model, tea.Cmd) {
	// Update config with paths
	a.config.SourcePath = msg.SourcePath
	a.config.DestPath = msg.DestPath

	// Create analysis screen
	a.currentScreen = screens.NewAnalysisScreen(a.config)

	return a, a.currentScreen.Init()
}

func (a AppModel) transitionToSummary(msg shared.TransitionToSummaryMsg) (tea.Model, tea.Cmd) {
	// Create summary screen
	a.currentScreen = screens.NewSummaryScreen(a.engine, msg.FinalState, msg.Err)

	return a, a.currentScreen.Init()
}

func (a AppModel) transitionToSync(msg shared.TransitionToSyncMsg) (tea.Model, tea.Cmd) {
	// Store engine reference
	a.engine = msg.Engine

	// Create sync screen
	a.currentScreen = screens.NewSyncScreen(a.engine)

	return a, a.currentScreen.Init()
}
