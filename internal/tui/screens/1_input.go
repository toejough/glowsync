package screens

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/tui/shared"
)

// InputScreen handles path input from the user
type InputScreen struct {
	config          *config.Config
	sourceInput     textinput.Model
	destInput       textinput.Model
	focusIndex      int
	completions     []string
	completionIndex int
	showCompletions bool
	validationError string
}

// NewInputScreen creates a new input screen
func NewInputScreen(cfg *config.Config) *InputScreen {
	sourceInput := textinput.New()
	sourceInput.Placeholder = "/path/to/source"
	sourceInput.Focus()
	sourceInput.Prompt = shared.PromptArrow()

	destInput := textinput.New()
	destInput.Placeholder = "/path/to/destination"
	destInput.Prompt = "  "

	return &InputScreen{
		config:      cfg,
		sourceInput: sourceInput,
		destInput:   destInput,
		focusIndex:  0,
	}
}

// Init implements tea.Model
func (s InputScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model
func (s InputScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return s.handleWindowSize(msg)
	case tea.KeyMsg:
		return s.handleKeyMsg(msg)
	}

	// Update the focused input
	var cmd tea.Cmd
	if s.focusIndex == 0 {
		s.sourceInput, cmd = s.sourceInput.Update(msg)
	} else {
		s.destInput, cmd = s.destInput.Update(msg)
	}

	return s, cmd
}

// View implements tea.Model
func (s InputScreen) View() string {
	return s.renderInputView()
}

func (s InputScreen) applyCompletion(completion string) InputScreen {
	if s.focusIndex == 0 {
		s.sourceInput.SetValue(completion)
		s.sourceInput.CursorEnd()
	} else {
		s.destInput.SetValue(completion)
		s.destInput.CursorEnd()
	}

	return s
}

func (s InputScreen) calculateCompletionWindow(currentIndex, maxShow, totalCount int) (start, end int) {
	start = max(currentIndex-maxShow/shared.ProgressHalfDivisor, 0)

	end = start + maxShow
	if end > totalCount {
		end = totalCount
		start = max(end-maxShow, 0)
	}

	return start, end
}

func (s InputScreen) formatAllCompletions(completions []string, currentIndex int) []string {
	lines := []string{shared.CompletionStyle().Render("  " + strings.Repeat("─", shared.ProgressBarWidth))}

	for i, comp := range completions {
		base := getBaseName(comp)
		if i == currentIndex {
			lines = append(lines, shared.CompletionSelectedStyle().Render("  ▶ "+base))
		} else {
			lines = append(lines, shared.CompletionStyle().Render("    "+base))
		}
	}

	return lines
}

func (s InputScreen) formatCompletionList(completions []string, currentIndex int) string {
	if len(completions) == 0 {
		return ""
	}

	maxShow := 8

	var lines []string

	switch {
	case len(completions) == 1:
		lines = s.formatSingleCompletion(completions[0])
	case len(completions) <= maxShow:
		lines = s.formatAllCompletions(completions, currentIndex)
	default:
		lines = s.formatWindowedCompletions(completions, currentIndex, maxShow)
	}

	return strings.Join(lines, "\n")
}

func (s InputScreen) formatSingleCompletion(completion string) []string {
	base := getBaseName(completion)
	return []string{shared.CompletionStyle().Render("  → " + base)}
}

func (s InputScreen) formatWindowedCompletions(completions []string, currentIndex, maxShow int) []string {
	lines := []string{shared.CompletionStyle().Render("  " + strings.Repeat("─", shared.ProgressBarWidth))}

	start, end := s.calculateCompletionWindow(currentIndex, maxShow, len(completions))

	// Show ellipsis if not at start
	if start > 0 {
		lines = append(lines, shared.CompletionStyle().Render("    ..."))
	}

	// Show window
	for i := start; i < end; i++ {
		base := getBaseName(completions[i])
		if i == currentIndex {
			lines = append(lines, shared.CompletionSelectedStyle().Render("  ▶ "+base))
		} else {
			lines = append(lines, shared.CompletionStyle().Render("    "+base))
		}
	}

	// Show ellipsis if not at end
	if end < len(completions) {
		lines = append(lines, shared.CompletionStyle().Render("    ..."))
	}

	return lines
}

func (s InputScreen) handleEnter() (tea.Model, tea.Cmd) {
	s.showCompletions = false
	if s.focusIndex == 0 && s.sourceInput.Value() != "" {
		// Move to destination input
		return s.moveToNextField()
	} else if s.focusIndex == 1 && s.destInput.Value() != "" {
		// Validate paths
		s.config.SourcePath = s.sourceInput.Value()
		s.config.DestPath = s.destInput.Value()

		err := s.config.ValidatePaths()
		if err != nil {
			s.validationError = err.Error()
			return s, nil
		}

		// Paths are valid - trigger transition to analysis
		return s, func() tea.Msg {
			return shared.TransitionToAnalysisMsg{
				SourcePath: s.config.SourcePath,
				DestPath:   s.config.DestPath,
			}
		}
	}

	return s, nil
}

func (s InputScreen) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle special keys using msg.Type (consistent with other screens)
	switch msg.Type {
	case tea.KeyCtrlC:
		// Emergency exit - quit immediately
		return s, tea.Quit
	case tea.KeyEsc:
		// Clear the current input field
		if s.focusIndex == 0 {
			s.sourceInput.SetValue("")
		} else {
			s.destInput.SetValue("")
		}
		s.showCompletions = false
		s.validationError = ""
		return s, nil
	case tea.KeyDown:
		return s.moveToNextField()
	case tea.KeyUp:
		return s.moveToPreviousField()
	case tea.KeyTab:
		return s.handleTabCompletion(), nil
	case tea.KeyShiftTab:
		return s.handleShiftTabCompletion(), nil
	case tea.KeyRight:
		return s.handleRightArrow(), nil
	case tea.KeyEnter:
		return s.handleEnter()
	}

	// Handle string-based keys for ctrl+n, ctrl+p
	switch msg.String() {
	case "ctrl+n":
		return s.moveToNextField()
	case "ctrl+p":
		return s.moveToPreviousField()
	default:
		s.showCompletions = false
		s.validationError = "" // Clear error when user types
	}

	// Update the focused input
	var cmd tea.Cmd
	if s.focusIndex == 0 {
		s.sourceInput, cmd = s.sourceInput.Update(msg)
	} else {
		s.destInput, cmd = s.destInput.Update(msg)
	}

	return s, cmd
}

func (s InputScreen) handleRightArrow() InputScreen {
	// If showing completions, accept current and continue to next segment
	if s.showCompletions && len(s.completions) > 0 {
		currentCompletion := s.completions[s.completionIndex]
		s = s.applyCompletion(currentCompletion)

		// Reset completion state and get new completions for next segment
		s.showCompletions = false

		s.completions = getPathCompletions(currentCompletion)
		if len(s.completions) > 0 {
			s.completionIndex = 0
			s.showCompletions = true
			s = s.applyCompletion(s.completions[0])
		}

		return s
	}
	// Otherwise, let the textinput handle it (move cursor right)
	s.showCompletions = false

	return s
}

func (s InputScreen) handleShiftTabCompletion() InputScreen {
	if s.showCompletions && len(s.completions) > 0 {
		// Cycle backward through completions
		s.completionIndex--
		if s.completionIndex < 0 {
			s.completionIndex = len(s.completions) - 1
		}

		s = s.applyCompletion(s.completions[s.completionIndex])
	}

	return s
}

// ============================================================================
// Path Completion
// ============================================================================

func (s InputScreen) handleTabCompletion() InputScreen {
	var currentValue string
	if s.focusIndex == 0 {
		currentValue = s.sourceInput.Value()
	} else {
		currentValue = s.destInput.Value()
	}

	// Get completions if we don't have them or if this is first tab
	if !s.showCompletions {
		s.completions = getPathCompletions(currentValue)
		s.completionIndex = 0
		s.showCompletions = true

		// If only one match, complete it immediately and hide list
		if len(s.completions) == 1 {
			s = s.applyCompletion(s.completions[0])
			s.showCompletions = false
		}
	} else if len(s.completions) > 0 {
		// Already showing completions - cycle forward through them
		s.completionIndex = (s.completionIndex + 1) % len(s.completions)
		s = s.applyCompletion(s.completions[s.completionIndex])
	}

	return s
}

// ============================================================================
// Message Handlers
// ============================================================================

func (s InputScreen) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	// Set input widths to use most of the available width (minus padding and borders)
	inputWidth := max(msg.Width-shared.ProgressUpdateInterval, shared.ProgressLogThreshold)
	s.sourceInput.Width = inputWidth
	s.destInput.Width = inputWidth

	return s, nil
}

// ============================================================================
// Field Navigation
// ============================================================================

func (s InputScreen) moveToNextField() (tea.Model, tea.Cmd) {
	if s.focusIndex == 0 {
		s.focusIndex = 1
		s.sourceInput.Blur()
		s.sourceInput.Prompt = "  "
		s.destInput.Focus()
		s.destInput.Prompt = shared.PromptArrow()
	}

	s.showCompletions = false
	s.validationError = "" // Clear error when navigating

	return s, nil
}

func (s InputScreen) moveToPreviousField() (tea.Model, tea.Cmd) {
	if s.focusIndex == 1 {
		s.focusIndex = 0
		s.destInput.Blur()
		s.destInput.Prompt = "  "
		s.sourceInput.Focus()
		s.sourceInput.Prompt = shared.PromptArrow()
	}

	s.showCompletions = false
	s.validationError = "" // Clear error when navigating

	return s, nil
}

// ============================================================================
// Rendering
// ============================================================================

func (s InputScreen) renderInputView() string {
	content := shared.RenderTitle("🚀 File Sync Tool") + "\n\n" +
		shared.RenderSubtitle("Configure your sync operation") + "\n\n" +
		shared.RenderLabel("Source Path:") + "\n" +
		s.sourceInput.View() + "\n"

	// Show completion list for source
	if s.focusIndex == 0 && s.showCompletions && len(s.completions) > 0 {
		content += s.formatCompletionList(s.completions, s.completionIndex) + "\n"
	}

	content += "\n" +
		shared.RenderLabel("Destination Path:") + "\n" +
		s.destInput.View() + "\n"

	// Show completion list for dest
	if s.focusIndex == 1 && s.showCompletions && len(s.completions) > 0 {
		content += s.formatCompletionList(s.completions, s.completionIndex) + "\n"
	}

	// Show validation error if present
	if s.validationError != "" {
		content += "\n" + shared.RenderError("Error: "+s.validationError) + "\n"
	}

	content += "\n" +
		shared.RenderSubtitle("Tab/Shift+Tab to cycle • → to accept & continue • ↑↓ to switch fields • Enter to continue • Esc to clear field • Ctrl+C to exit") //nolint:lll // Help text with keyboard shortcuts

	return shared.RenderBox(content)
}

func expandHomePath(input string) string {
	if input == "" {
		return "."
	}

	// Expand ~ to home directory
	if strings.HasPrefix(input, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, input[1:])
		}
	}

	return input
}

func getBaseName(path string) string {
	// Remove trailing slash if present
	trimmed := strings.TrimSuffix(path, "/")

	// Simple basename extraction
	idx := strings.LastIndex(trimmed, "/")
	if idx == -1 {
		// No slash found - return the whole path (with trailing slash if it was a dir)
		if strings.HasSuffix(path, "/") {
			return trimmed + "/"
		}

		return path
	}

	// Extract basename and restore trailing slash if it was a directory
	base := trimmed[idx+1:]
	if strings.HasSuffix(path, "/") {
		return base + "/"
	}

	return base
}

// ============================================================================
// Path Completion Helpers
// ============================================================================

func getPathCompletions(input string) []string {
	input = expandHomePath(input)
	dir, prefix := parseCompletionPath(input)

	// Read directory entries
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	completions := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()

		if !shouldIncludeEntry(name, prefix) {
			continue
		}

		fullPath := filepath.Join(dir, name)

		// Add trailing slash for directories
		if entry.IsDir() {
			fullPath += string(filepath.Separator)
		}

		completions = append(completions, fullPath)
	}

	sort.Strings(completions)

	return completions
}

func parseCompletionPath(input string) (dir, prefix string) {
	dir = filepath.Dir(input)
	prefix = filepath.Base(input)

	// If input ends with /, we're completing in that directory
	if strings.HasSuffix(input, string(filepath.Separator)) {
		dir = input
		prefix = ""
	}

	return dir, prefix
}

func shouldIncludeEntry(name, prefix string) bool {
	// Skip hidden files unless prefix starts with .
	if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
		return false
	}

	// Check if name matches prefix
	return prefix == "" || strings.HasPrefix(name, prefix)
}
