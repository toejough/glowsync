package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// ConfirmationScreen displays analysis results and asks for confirmation before sync
type ConfirmationScreen struct {
	engine  *syncengine.Engine
	logPath string
}

// NewConfirmationScreen creates a new confirmation screen
func NewConfirmationScreen(engine *syncengine.Engine, logPath string) *ConfirmationScreen {
	return &ConfirmationScreen{
		engine:  engine,
		logPath: logPath,
	}
}

// Init initializes the confirmation screen
func (s ConfirmationScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages for the confirmation screen
func (s ConfirmationScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		return s.handleKeyMsg(keyMsg)
	}

	return s, nil
}

// View renders the confirmation screen
func (s ConfirmationScreen) View() string {
	var builder strings.Builder

	// Get status from engine
	status := s.engine.GetStatus()

	// Title
	builder.WriteString(shared.RenderTitle("Analysis Complete"))
	builder.WriteString("\n\n")

	// Statistics
	builder.WriteString(shared.RenderLabel("Files to sync: "))
	builder.WriteString(fmt.Sprintf("%d", status.TotalFiles))
	builder.WriteString("\n")

	builder.WriteString(shared.RenderLabel("Total size: "))
	builder.WriteString(shared.FormatBytes(status.TotalBytes))
	builder.WriteString("\n")

	// Help text
	builder.WriteString("\n")
	builder.WriteString(shared.RenderDim("Press Enter to begin sync, Esc to cancel"))

	return builder.String()
}

func (s ConfirmationScreen) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Confirm and proceed to sync
		return s, func() tea.Msg {
			return shared.ConfirmSyncMsg{
				Engine:  s.engine,
				LogPath: s.logPath,
			}
		}

	case tea.KeyEsc:
		// Cancel and return to input screen
		return s, func() tea.Msg {
			return shared.TransitionToInputMsg{}
		}

	default:
		// Ignore other keys
		return s, nil
	}
}
