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
	// Render timeline header showing "compare" phase as active
	timeline := shared.RenderTimeline("compare")

	// Calculate left column width (60% of total width)
	leftWidth := int(float64(s.width) * 0.6) //nolint:mnd // 60-40 split is standard layout ratio from design

	// Build left and right column content
	leftContent := s.renderLeftColumn(leftWidth)
	rightContent := s.renderRightColumn()

	// Combine columns using two-column layout
	mainContent := shared.RenderTwoColumnLayout(leftContent, rightContent, s.width, s.height)

	// Final assembly: timeline + main content wrapped in box
	output := timeline + "\n\n" + mainContent

	return shared.RenderBox(output, s.width, s.height)
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

// renderErrorContent builds the content for the errors widget box
func (s ConfirmationScreen) renderErrorContent(status *syncengine.Status) string {
	var builder strings.Builder

	builder.WriteString(shared.RenderError("Errors during analysis:"))
	builder.WriteString("\n")

	// Use shared helper with in-progress context (3 error limit with "see summary" message)
	errorList := shared.RenderErrorList(shared.ErrorListConfig{
		Errors:  status.Errors,
		Context: shared.ContextInProgress,
	})
	builder.WriteString(errorList)

	return builder.String()
}

// renderFilterContent builds the content for the filter widget box
func (s ConfirmationScreen) renderFilterContent() string {
	return s.engine.FilePattern
}

// renderLeftColumn builds the left column content with widget boxes
func (s ConfirmationScreen) renderLeftColumn(leftWidth int) string {
	var content string

	// Get status from engine
	status := s.engine.GetStatus()

	// Title
	content = shared.RenderTitle("Analysis Complete") + "\n\n"

	// Sync Plan widget box
	syncPlanContent := s.renderSyncPlanContent(status)
	content += shared.RenderWidgetBox("Sync Plan", syncPlanContent, leftWidth) + "\n\n"

	// Filter widget box (conditional - only if pattern is set)
	if s.engine.FilePattern != "" {
		filterContent := s.renderFilterContent()
		content += shared.RenderWidgetBox("Filter", filterContent, leftWidth) + "\n\n"
	}

	// Errors widget box (conditional - only if errors exist)
	if len(status.Errors) > 0 {
		errorContent := s.renderErrorContent(status)
		content += shared.RenderWidgetBox("Errors", errorContent, leftWidth) + "\n\n"
	}

	// Help text at bottom of left column
	content += shared.RenderDim("Press Enter to begin sync • Esc to cancel • Ctrl+C to exit")

	return content
}

// renderRightColumn builds the right column content with activity log
func (s ConfirmationScreen) renderRightColumn() string {
	// Get status from engine
	status := s.engine.GetStatus()

	// Use status.AnalysisLog directly if available, otherwise empty
	var activityEntries []string
	if status != nil {
		activityEntries = status.AnalysisLog
	}

	// Render activity log with last 10 entries
	const maxLogEntries = 10

	return shared.RenderActivityLog("Activity", activityEntries, maxLogEntries)
}

// renderSyncPlanContent builds the content for the sync plan widget box
func (s ConfirmationScreen) renderSyncPlanContent(status *syncengine.Status) string {
	var builder strings.Builder

	// Files to sync
	builder.WriteString(shared.RenderLabel("Files to sync: "))
	builder.WriteString(strconv.Itoa(status.TotalFiles))
	builder.WriteString("\n")

	// Total size
	builder.WriteString(shared.RenderLabel("Total size: "))
	builder.WriteString(shared.FormatBytes(status.TotalBytes))
	builder.WriteString("\n")

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
	}

	return builder.String()
}
