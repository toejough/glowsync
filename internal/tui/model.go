package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/sync"
)

// Model represents the TUI state
type Model struct {
	// Configuration
	config *config.Config

	// Input phase (when needsInput is true)
	needsInput      bool
	sourceInput     textinput.Model
	destInput       textinput.Model
	focusIndex      int
	completions     []string
	completionIndex int
	showCompletions bool

	// Sync phase (when needsInput is false)
	engine          *sync.Engine
	status          *sync.Status
	overallProgress progress.Model
	fileProgress    progress.Model
	spinner         spinner.Model
	width           int
	height          int
	state           string // "input", "initializing", "analyzing", "syncing", "complete", "error", "cancelled", "cancelling"
	err             error
	quitting        bool
	cancelled       bool
	lastUpdate      time.Time
}

// StatusUpdateMsg is sent when sync status updates
type StatusUpdateMsg struct {
	Status *sync.Status
}

// InitializeEngineMsg is sent to trigger engine initialization
type InitializeEngineMsg struct{}

// EngineInitializedMsg is sent when the engine has been created
type EngineInitializedMsg struct {
	Engine *sync.Engine
}

// AnalysisStartedMsg is sent when analysis has started
type AnalysisStartedMsg struct{}

// AnalysisCompleteMsg is sent when analysis is complete
type AnalysisCompleteMsg struct{}

// SyncCompleteMsg is sent when sync is complete
type SyncCompleteMsg struct{}

// ErrorMsg is sent when an error occurs
type ErrorMsg struct {
	Err error
}

// NewModel creates a new TUI model
func NewModel(cfg *config.Config) Model {
	sourceInput := textinput.New()
	sourceInput.Placeholder = "/path/to/source"
	sourceInput.Focus()
	sourceInput.Prompt = "▶ "

	destInput := textinput.New()
	destInput.Placeholder = "/path/to/destination"
	destInput.Prompt = "  "

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	overallProg := progress.New(
		progress.WithDefaultGradient(),
	)

	fileProg := progress.New(
		progress.WithDefaultGradient(),
	)

	// Determine initial state based on whether we need input
	needsInput := cfg.InteractiveMode
	initialState := "input"
	if !needsInput {
		initialState = "initializing"
	}

	return Model{
		config:          cfg,
		needsInput:      needsInput,
		sourceInput:     sourceInput,
		destInput:       destInput,
		focusIndex:      0,
		overallProgress: overallProg,
		fileProgress:    fileProg,
		spinner:         s,
		state:           initialState,
		lastUpdate:      time.Now(),
	}
}

// Init initializes the model and returns initial commands
func (m Model) Init() tea.Cmd {
	// Always start with blink for text inputs
	// Update() will handle initialization logic based on state
	return textinput.Blink
}


