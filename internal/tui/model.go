package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joe/copy-files/internal/sync"
)

// Model represents the TUI state
type Model struct {
	engine          *sync.Engine
	status          *sync.Status
	overallProgress progress.Model
	fileProgress    progress.Model
	spinner         spinner.Model
	width           int
	height          int
	state           string // "initializing", "analyzing", "syncing", "complete", "error", "cancelled", "cancelling"
	err             error
	quitting        bool
	cancelled       bool
	lastUpdate      time.Time
}

// StatusUpdateMsg is sent when sync status updates
type StatusUpdateMsg struct {
	Status *sync.Status
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
func NewModel(engine *sync.Engine) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	overallProg := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(60),
	)

	fileProg := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(60),
	)

	m := Model{
		engine:          engine,
		overallProgress: overallProg,
		fileProgress:    fileProg,
		spinner:         s,
		state:           "initializing",
		lastUpdate:      time.Now(),
	}

	// Register status callback
	engine.RegisterStatusCallback(func(status *sync.Status) {
		m.status = status
	})

	return m
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startAnalysis(),
		tickCmd(),
	)
}

// tickCmd creates a tick command for regular updates
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type tickMsg time.Time

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.overallProgress.Width = minInt(msg.Width-20, 80)
		m.fileProgress.Width = minInt(msg.Width-20, 80)
		return m, nil

	case tickMsg:
		// Update status from engine, but only every 200ms to reduce lock contention
		// This prevents blocking workers who are trying to update progress
		if m.engine != nil && m.state != "complete" && m.state != "error" && m.state != "cancelled" {
			now := time.Now()
			if now.Sub(m.lastUpdate) >= 200*time.Millisecond {
				status := m.engine.GetStatus()
				m.status = &status
				m.lastUpdate = now
			}
		}
		// Always continue ticking for animations and time updates
		return m, tickCmd()

	case AnalysisStartedMsg:
		m.state = "analyzing"
		return m, nil

	case AnalysisCompleteMsg:
		m.state = "syncing"
		return m, m.startSync()

	case SyncCompleteMsg:
		// If we were cancelling, transition to cancelled state
		// Otherwise, transition to complete state
		if m.state == "cancelling" {
			m.state = "cancelled"
		} else {
			m.state = "complete"
		}
		// Get final status one last time
		if m.engine != nil {
			m.engine.CloseLog()
			status := m.engine.GetStatus()
			m.status = &status
		}
		return m, nil

	case ErrorMsg:
		m.err = msg.Err
		// If we were cancelling and got a cancellation error, go to cancelled state
		if m.cancelled && (msg.Err.Error() == "analysis cancelled" || msg.Err.Error() == "sync cancelled") {
			m.state = "cancelled"
			if m.engine != nil {
				m.engine.CloseLog()
				status := m.engine.GetStatus()
				m.status = &status
			}
		} else {
			m.state = "error"
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		// If already in a final state, quit immediately
		if m.state == "complete" || m.state == "error" || m.state == "cancelled" {
			m.quitting = true
			return m, tea.Quit
		}

		// Otherwise, cancel the sync
		m.cancelled = true
		m.state = "cancelling"
		// Signal the engine to stop
		if m.engine != nil {
			m.engine.Cancel()
		}
		// Don't close log yet - wait for workers to finish
		// The sync completion will handle final state transition
		return m, nil

	case "enter":
		if m.state == "complete" || m.state == "error" || m.state == "cancelled" {
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

// startAnalysis starts the analysis process
func (m Model) startAnalysis() tea.Cmd {
	return func() tea.Msg {
		// Signal that we're starting
		// This happens immediately before any blocking work

		// Enable file logging for debugging
		logPath := "copy-files-debug.log"
		if err := m.engine.EnableFileLogging(logPath); err != nil {
			// Non-fatal, just continue without file logging
		}

		// Send a message that analysis has started
		// We'll do this by returning a batch command
		return tea.Batch(
			func() tea.Msg { return AnalysisStartedMsg{} },
			func() tea.Msg {
				if err := m.engine.Analyze(); err != nil {
					return ErrorMsg{Err: err}
				}
				return AnalysisCompleteMsg{}
			},
		)()
	}
}

// startSync starts the sync process
func (m Model) startSync() tea.Cmd {
	return func() tea.Msg {
		if err := m.engine.Sync(); err != nil {
			m.engine.CloseLog()
			return ErrorMsg{Err: err}
		}
		m.engine.CloseLog()
		return SyncCompleteMsg{}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

