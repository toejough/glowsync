package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joe/copy-files/internal/sync"
)

type tickMsg time.Time

// tickCmd creates a tick command for regular updates
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// initializeEngine creates the sync engine and starts the analysis
func (m Model) initializeEngine() tea.Cmd {
	// Create a command that will initialize the engine
	return func() tea.Msg {
		return EngineInitializedMsg{
			Engine: sync.NewEngine(m.config.SourcePath, m.config.DestPath),
		}
	}
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If in input mode, handle input-specific updates
	if m.needsInput {
		return m.updateInput(msg)
	}

	// Otherwise, handle sync-specific updates
	switch msg := msg.(type) {
	case InitializeEngineMsg:
		// Initialize the engine and start the process
		m.state = "initializing"
		return m, m.initializeEngine()

	case EngineInitializedMsg:
		// Store the engine and configure it
		m.engine = msg.Engine
		m.engine.Workers = m.config.Workers
		m.engine.AdaptiveMode = m.config.AdaptiveMode
		m.engine.UseCache = m.config.UseCache

		// Register status callback
		m.engine.RegisterStatusCallback(func(status *sync.Status) {
			m.status = status
		})

		// Capture engine in local variable for closures
		engine := m.engine

		// Start spinner, analysis, and ticking
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg {
				// Enable file logging for debugging
				logPath := "copy-files-debug.log"
				if err := engine.EnableFileLogging(logPath); err != nil {
					// Non-fatal, just continue without file logging
				}
				// Signal that analysis has started
				return AnalysisStartedMsg{}
			},
			func() tea.Msg {
				if err := engine.Analyze(); err != nil {
					return ErrorMsg{Err: err}
				}
				return AnalysisCompleteMsg{}
			},
			tickCmd(),
		)

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Set progress bar widths to use most of available width (minus padding and borders)
		// Leave some margin for box borders and padding
		progressWidth := msg.Width - 10
		if progressWidth < 20 {
			progressWidth = 20
		}
		// Cap at a reasonable maximum for readability
		if progressWidth > 100 {
			progressWidth = 100
		}
		m.overallProgress.Width = progressWidth
		m.fileProgress.Width = progressWidth
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

// updateInput handles updates when in input mode
func (m Model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Set input widths to use most of the available width (minus padding and borders)
		inputWidth := msg.Width - 10
		if inputWidth < 20 {
			inputWidth = 20
		}
		m.sourceInput.Width = inputWidth
		m.destInput.Width = inputWidth
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "ctrl+n", "down":
			// Move to next field
			if m.focusIndex == 0 {
				m.focusIndex = 1
				m.sourceInput.Blur()
				m.sourceInput.Prompt = "  "
				m.destInput.Focus()
				m.destInput.Prompt = "▶ "
			}
			m.showCompletions = false
			return m, nil

		case "ctrl+p", "up":
			// Move to previous field
			if m.focusIndex == 1 {
				m.focusIndex = 0
				m.destInput.Blur()
				m.destInput.Prompt = "  "
				m.sourceInput.Focus()
				m.sourceInput.Prompt = "▶ "
			}
			m.showCompletions = false
			return m, nil

		case "tab":
			return m.handleTabCompletion(), nil

		case "shift+tab":
			return m.handleShiftTabCompletion(), nil

		case "right":
			return m.handleRightArrow(), nil

		case "enter":
			m.showCompletions = false
			if m.focusIndex == 0 && m.sourceInput.Value() != "" {
				// Move to destination input
				m.focusIndex = 1
				m.sourceInput.Blur()
				m.sourceInput.Prompt = "  "
				m.destInput.Focus()
				m.destInput.Prompt = "▶ "
				return m, nil
			} else if m.focusIndex == 1 && m.destInput.Value() != "" {
				// Submit - transition to sync phase
				return m.transitionToSync()
			}

		default:
			// Any other key resets completion state
			m.showCompletions = false
		}
	}

	// Update the focused input
	if m.focusIndex == 0 {
		m.sourceInput, cmd = m.sourceInput.Update(msg)
	} else {
		m.destInput, cmd = m.destInput.Update(msg)
	}

	return m, cmd
}

// transitionToSync transitions from input mode to sync mode
func (m Model) transitionToSync() (tea.Model, tea.Cmd) {
	// Set paths in config
	m.config.SourcePath = m.sourceInput.Value()
	m.config.DestPath = m.destInput.Value()

	// Validate paths
	if err := m.config.ValidatePaths(); err != nil {
		m.err = err
		m.state = "error"
		m.needsInput = false
		return m, nil
	}

	// Transition to sync mode
	m.needsInput = false
	m.state = "initializing"

	// Initialize engine and start sync
	return m, m.initializeEngine()
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

// handleTabCompletion handles tab key for path completion
func (m Model) handleTabCompletion() Model {
	var currentValue string
	if m.focusIndex == 0 {
		currentValue = m.sourceInput.Value()
	} else {
		currentValue = m.destInput.Value()
	}

	// Get completions if we don't have them or if this is first tab
	if !m.showCompletions {
		m.completions = getPathCompletions(currentValue)
		m.completionIndex = 0
		m.showCompletions = true

		// If only one match, complete it immediately and hide list
		if len(m.completions) == 1 {
			if m.focusIndex == 0 {
				m.sourceInput.SetValue(m.completions[0])
				m.sourceInput.CursorEnd()
			} else {
				m.destInput.SetValue(m.completions[0])
				m.destInput.CursorEnd()
			}
			m.showCompletions = false
		}
		// If multiple matches, show the list but DON'T change the input yet
		// Let the user see what they typed and the available options
	} else {
		// Already showing completions - cycle forward through them
		if len(m.completions) > 0 {
			m.completionIndex = (m.completionIndex + 1) % len(m.completions)
			// Now apply the selected completion
			if m.focusIndex == 0 {
				m.sourceInput.SetValue(m.completions[m.completionIndex])
				m.sourceInput.CursorEnd()
			} else {
				m.destInput.SetValue(m.completions[m.completionIndex])
				m.destInput.CursorEnd()
			}
		}
	}
	return m
}

// handleShiftTabCompletion handles shift+tab for backward completion cycling
func (m Model) handleShiftTabCompletion() Model {
	if m.showCompletions && len(m.completions) > 0 {
		// Cycle backward through completions
		m.completionIndex--
		if m.completionIndex < 0 {
			m.completionIndex = len(m.completions) - 1
		}

		// Apply completion
		if m.focusIndex == 0 {
			m.sourceInput.SetValue(m.completions[m.completionIndex])
			m.sourceInput.CursorEnd()
		} else {
			m.destInput.SetValue(m.completions[m.completionIndex])
			m.destInput.CursorEnd()
		}
	}
	return m
}

// handleRightArrow handles right arrow for accepting completion and continuing
func (m Model) handleRightArrow() Model {
	// If showing completions, accept current and continue to next segment
	if m.showCompletions && len(m.completions) > 0 {
		currentCompletion := m.completions[m.completionIndex]

		// Set the value
		if m.focusIndex == 0 {
			m.sourceInput.SetValue(currentCompletion)
			m.sourceInput.CursorEnd()
		} else {
			m.destInput.SetValue(currentCompletion)
			m.destInput.CursorEnd()
		}

		// Reset completion state and get new completions for next segment
		m.showCompletions = false
		m.completions = getPathCompletions(currentCompletion)
		if len(m.completions) > 0 {
			m.completionIndex = 0
			m.showCompletions = true

			// Apply first completion of next segment
			if m.focusIndex == 0 {
				m.sourceInput.SetValue(m.completions[0])
				m.sourceInput.CursorEnd()
			} else {
				m.destInput.SetValue(m.completions[0])
				m.destInput.CursorEnd()
			}
		}
		return m
	}
	// Otherwise, let the textinput handle it (move cursor right)
	m.showCompletions = false
	return m
}

// getPathCompletions returns possible path completions for the given input
func getPathCompletions(input string) []string {
	if input == "" {
		input = "."
	}

	// Expand ~ to home directory
	if strings.HasPrefix(input, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			input = filepath.Join(home, input[1:])
		}
	}

	// Get the directory and prefix to search
	dir := filepath.Dir(input)
	prefix := filepath.Base(input)

	// If input ends with /, we're completing in that directory
	if strings.HasSuffix(input, string(filepath.Separator)) {
		dir = input
		prefix = ""
	}

	// Read directory entries
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var completions []string
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless prefix starts with .
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}

		// Check if name matches prefix
		if prefix == "" || strings.HasPrefix(name, prefix) {
			fullPath := filepath.Join(dir, name)

			// Add trailing slash for directories
			if entry.IsDir() {
				fullPath += string(filepath.Separator)
			}

			completions = append(completions, fullPath)
		}
	}

	sort.Strings(completions)
	return completions
}
