// Package tui provides terminal user interface components.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputModel represents the input screen model
type InputModel struct {
	sourceInput     textinput.Model
	destInput       textinput.Model
	focusIndex      int
	Submitted       bool
	SourcePath      string
	DestPath        string
	completions     []string
	completionIndex int
	showCompletions bool
}

// NewInputModel creates a new input model
func NewInputModel() InputModel {
	sourceInput := textinput.New()
	sourceInput.Placeholder = "/path/to/source"
	sourceInput.Focus()
	sourceInput.CharLimit = 256
	sourceInput.Width = 60
	sourceInput.Prompt = "▶ "
	
	destInput := textinput.New()
	destInput.Placeholder = "/path/to/destination"
	destInput.CharLimit = 256
	destInput.Width = 60
	destInput.Prompt = "  "
	
	return InputModel{
		sourceInput: sourceInput,
		destInput:   destInput,
		focusIndex:  0,
	}
}

// Init initializes the input model
func (m InputModel) Init() tea.Cmd {
	return textinput.Blink
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

// findCommonPrefix finds the longest common prefix among strings
func findCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}

// Update handles input model updates
func (m InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
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
			// Path completion - cycle forward
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
			} else {
				// Cycle forward through completions
				if len(m.completions) > 0 {
					m.completionIndex = (m.completionIndex + 1) % len(m.completions)
				}
			}

			// Apply completion
			if len(m.completions) > 0 {
				if len(m.completions) == 1 {
					// Single match - complete it
					if m.focusIndex == 0 {
						m.sourceInput.SetValue(m.completions[0])
						m.sourceInput.CursorEnd()
					} else {
						m.destInput.SetValue(m.completions[0])
						m.destInput.CursorEnd()
					}
					m.showCompletions = false
				} else {
					// Multiple matches - show current one
					if m.focusIndex == 0 {
						m.sourceInput.SetValue(m.completions[m.completionIndex])
						m.sourceInput.CursorEnd()
					} else {
						m.destInput.SetValue(m.completions[m.completionIndex])
						m.destInput.CursorEnd()
					}
				}
			}
			return m, nil

		case "shift+tab":
			// Path completion - cycle backward
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
			return m, nil

		case "right":
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
				return m, nil
			}
			// Otherwise, let the textinput handle it (move cursor right)
			m.showCompletions = false

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
				// Submit
				m.Submitted = true
				m.SourcePath = m.sourceInput.Value()
				m.DestPath = m.destInput.Value()
				return m, tea.Quit
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

// View renders the input view
func (m InputModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(80)

	content := titleStyle.Render("🚀 File Sync Tool") + "\n\n" +
		subtitleStyle.Render("Configure your sync operation") + "\n\n" +
		labelStyle.Render("Source Path:") + "\n" +
		m.sourceInput.View() + "\n"

	// Show completion list for source
	if m.focusIndex == 0 && m.showCompletions && len(m.completions) > 0 {
		content += formatCompletionList(m.completions, m.completionIndex, hintStyle) + "\n"
	}

	content += "\n" +
		labelStyle.Render("Destination Path:") + "\n" +
		m.destInput.View() + "\n"

	// Show completion list for dest
	if m.focusIndex == 1 && m.showCompletions && len(m.completions) > 0 {
		content += formatCompletionList(m.completions, m.completionIndex, hintStyle) + "\n"
	}

	content += "\n" +
		subtitleStyle.Render("Tab/Shift+Tab to cycle • → to accept & continue • ↑↓ to switch fields • Enter to continue • Ctrl+C to quit")

	return boxStyle.Render(content)
}

// formatCompletionList formats the completion list for display
func formatCompletionList(completions []string, currentIndex int, style lipgloss.Style) string {
	if len(completions) == 0 {
		return ""
	}

	maxShow := 8
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	var lines []string

	if len(completions) == 1 {
		// Single completion - just show it
		base := filepath.Base(completions[0])
		lines = append(lines, style.Render("  → "+base))
	} else if len(completions) <= maxShow {
		// Show all completions
		lines = append(lines, style.Render(fmt.Sprintf("  %d matches:", len(completions))))
		for i, comp := range completions {
			base := filepath.Base(comp)
			if i == currentIndex {
				lines = append(lines, selectedStyle.Render("  ▶ "+base))
			} else {
				lines = append(lines, style.Render("    "+base))
			}
		}
	} else {
		// Show a window around current selection
		lines = append(lines, style.Render(fmt.Sprintf("  %d matches (showing %d):", len(completions), maxShow)))

		// Calculate window
		start := currentIndex - maxShow/2
		if start < 0 {
			start = 0
		}
		end := start + maxShow
		if end > len(completions) {
			end = len(completions)
			start = end - maxShow
			if start < 0 {
				start = 0
			}
		}

		// Show ellipsis if not at start
		if start > 0 {
			lines = append(lines, style.Render("    ..."))
		}

		// Show window
		for i := start; i < end; i++ {
			base := filepath.Base(completions[i])
			if i == currentIndex {
				lines = append(lines, selectedStyle.Render("  ▶ "+base))
			} else {
				lines = append(lines, style.Render("    "+base))
			}
		}

		// Show ellipsis if not at end
		if end < len(completions) {
			lines = append(lines, style.Render("    ..."))
		}
	}

	return strings.Join(lines, "\n")
}

