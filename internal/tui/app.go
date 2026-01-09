package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

// AppModel is the top-level model that uses UnifiedScreen for single-screen flow
type AppModel struct {
	config        *config.Config
	currentScreen tea.Model
	engine        *syncengine.Engine
	logPath       string
	width         int
	height        int
}

// NewAppModel creates a new app model with UnifiedScreen
func NewAppModel(cfg *config.Config) *AppModel {
	// Use unified screen for single-screen flow
	unifiedScreen := NewUnifiedScreen(cfg)

	// If not interactive mode, auto-transition to analysis
	if !cfg.InteractiveMode {
		// Trigger transition to analysis phase
		unifiedScreen.phase = PhaseScan
		unifiedScreen.analysis = *screens.NewAnalysisScreen(cfg)
		unifiedScreen.hasAnalysis = true
	}

	return &AppModel{
		config:        cfg,
		currentScreen: unifiedScreen,
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

// LogPath returns the debug log path (for testing)
func (a AppModel) LogPath() string {
	return a.logPath
}

// Update implements tea.Model
func (a AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Capture window size
	if windowMsg, ok := msg.(tea.WindowSizeMsg); ok {
		a.width = windowMsg.Width
		a.height = windowMsg.Height
	}

	// Track engine and logPath from transitions (for LogPath() getter)
	switch msg := msg.(type) {
	case shared.TransitionToConfirmationMsg:
		a.engine = msg.Engine
		a.logPath = msg.LogPath
	case shared.TransitionToSyncMsg:
		a.engine = msg.Engine
		a.logPath = msg.LogPath
	}

	// Delegate everything to the unified screen
	var cmd tea.Cmd
	a.currentScreen, cmd = a.currentScreen.Update(msg)

	return a, cmd
}

// View implements tea.Model
func (a AppModel) View() string {
	return a.currentScreen.View()
}
