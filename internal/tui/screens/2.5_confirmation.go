package screens

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// ConfirmationScreen displays analysis results and asks for confirmation before sync
type ConfirmationScreen struct {
	engine  *syncengine.Engine
	logPath string
	width   int
	height  int
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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height

		return s, nil
	case tea.KeyMsg:
		return s.handleKeyMsg(msg)
	}

	return s, nil
}

// View renders the confirmation screen
func (s ConfirmationScreen) View() string {
	// Timeline header + content + box wrapper
	var builder strings.Builder
	builder.WriteString(shared.RenderTimeline("compare"))
	builder.WriteString("\n\n")
	builder.WriteString(s.RenderContent())
	return shared.RenderBox(builder.String(), s.width, s.height)
}

// RenderContent returns just the content without timeline header or box wrapper.
// Used by UnifiedScreen to compose multiple screen contents together.
func (s ConfirmationScreen) RenderContent() string {
	var builder strings.Builder

	// Get status from engine
	status := s.engine.GetStatus()

	// Title
	builder.WriteString(shared.RenderTitle("Analysis Complete"))
	builder.WriteString("\n\n")

	// Statistics
	builder.WriteString(shared.RenderLabel("Files to sync: "))
	builder.WriteString(strconv.Itoa(status.TotalFiles))
	builder.WriteString("\n")

	builder.WriteString(shared.RenderLabel("Total size: "))
	builder.WriteString(shared.FormatBytes(status.TotalBytes))
	builder.WriteString("\n")

	// Filter indicator (if pattern is set)
	if s.engine.FilePattern != "" {
		builder.WriteString("\n")
		builder.WriteString(shared.RenderLabel("Filtering by: "))
		builder.WriteString(s.engine.FilePattern)
		builder.WriteString("\n")
	}

	// Empty state handling - context-aware messages
	if status.TotalFiles == 0 {
		builder.WriteString("\n")
		if s.engine.FilePattern != "" {
			// Filter applied but no matches
			builder.WriteString(shared.RenderEmptyListPlaceholder("No files match your filter"))
		} else {
			// No filter - could be empty source or already synced
			builder.WriteString(shared.RenderEmptyListPlaceholder("All files already synced"))
		}
		builder.WriteString("\n")
	}

	// Show errors if any occurred during analysis
	if len(status.Errors) > 0 {
		builder.WriteString("\n")
		builder.WriteString(shared.RenderError("Errors during analysis:"))
		builder.WriteString("\n")

		// Use shared helper with in-progress context (3 error limit with "see summary" message)
		errorList := shared.RenderErrorList(shared.ErrorListConfig{
			Errors:  status.Errors,
			Context: shared.ContextInProgress,
		})
		builder.WriteString(errorList)
	}

	// Help text
	builder.WriteString("\n")
	builder.WriteString(shared.RenderDim("Press Enter to begin sync • Esc to cancel • Ctrl+C to exit"))

	return builder.String()
}

func (s ConfirmationScreen) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	//nolint:exhaustive // Default case handles all other keys
	switch msg.Type {
	case tea.KeyCtrlC:
		// Emergency exit - quit immediately
		return s, tea.Quit

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
