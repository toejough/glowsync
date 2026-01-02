package shared

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ActiveSymbol returns a circled dot symbol with ASCII fallback
func ActiveSymbol() string {
	if unicodeDisabled {
		return "[*]"
	}

	return "◉"
}

// CancelledSymbol returns a cancelled/prohibited symbol with ASCII fallback
func CancelledSymbol() string {
	if unicodeDisabled {
		return "[!]"
	}

	return "⊘"
}

// RenderTimeline renders the phase progression timeline for the header.
// Shows 5 phases: Input, Scan, Compare, Sync, Done
// Phases before current show ✓ (completed)
// Current phase shows ◉ (active)
// Phases after current show ○ (pending)
// Error phases (e.g., "scan_error") show ✗ at error point, ⊘ for skipped phases
func RenderTimeline(currentPhase string) string {
	// Normalize input - trim whitespace and convert to lowercase for case-insensitive matching
	phase := strings.ToLower(strings.TrimSpace(currentPhase))

	// Check for error suffix (e.g., "scan_error" → "scan" + isError=true)
	isError := strings.HasSuffix(phase, "_error")
	if isError {
		phase = strings.TrimSuffix(phase, "_error")
	}

	// Define all phases in order with their display names and matching keys
	type phaseDefinition struct {
		name string
		key  string
	}

	phases := []phaseDefinition{
		{"Input", "input"},
		{"Scan", "scan"},
		{"Compare", "compare"},
		{"Sync", "sync"},
		{"Done", "done"},
	}

	// Find the index of the current phase
	currentIdx := -1
	for i, phaseInfo := range phases {
		if phaseInfo.key == phase {
			currentIdx = i
			break
		}
	}

	// If phase is invalid/unknown, default to input (first phase)
	if currentIdx == -1 {
		currentIdx = 0
	}

	// Build timeline parts (preallocate with capacity for all 5 phases)
	parts := make([]string, 0, len(phases))

	for phaseIdx, phaseInfo := range phases {
		var symbol string
		var style lipgloss.Style

		// Determine symbol and style based on phase state
		switch {
		case isError && phaseIdx == currentIdx:
			// Error occurred at this phase
			symbol = ErrorSymbol()
			style = lipgloss.NewStyle().Foreground(ErrorColor())
		case isError && phaseIdx > currentIdx:
			// Phase was cancelled/skipped due to earlier error
			symbol = CancelledSymbol()
			style = DimStyle()
		case phaseIdx < currentIdx:
			// Phase was completed successfully
			symbol = SuccessSymbol()
			style = lipgloss.NewStyle().Foreground(SuccessColor())
		case phaseIdx == currentIdx && currentIdx == len(phases)-1:
			// Special case: "done" phase shows as complete, not active
			symbol = SuccessSymbol()
			style = lipgloss.NewStyle().Foreground(SuccessColor())
		case phaseIdx == currentIdx:
			// Current active phase
			symbol = ActiveSymbol()
			style = lipgloss.NewStyle().Foreground(PrimaryColor())
		default:
			// Future pending phase
			symbol = PendingSymbol()
			style = DimStyle()
		}

		// Format as "symbol PhaseName" and apply style
		rendered := style.Render(symbol + " " + phaseInfo.name)
		parts = append(parts, rendered)
	}

	// Join all parts with separator " ── "
	separator := DimStyle().Render(" ── ")

	return strings.Join(parts, separator)
}
