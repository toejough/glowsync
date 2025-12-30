package shared

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
)

// NewProgressModel creates a new progress bar model with the specified width.
// This is a helper function for creating progress bars with consistent styling.
func NewProgressModel(width int) progress.Model {
	progressBar := progress.New(progress.WithDefaultGradient())
	progressBar.Width = width
	progressBar.ShowPercentage = false // We render percentage ourselves

	// Apply custom colors if not disabled
	if !colorsDisabled {
		progressBar.EmptyColor = dimColorCode
		progressBar.FullColor = accentColorCode
	}

	return progressBar
}

// RenderASCIIProgress renders a progress bar in ASCII format.
// percent should be between 0.0 and 1.0, width is the total width of the bar.
// Returns a string like: "[=========>          ] 45%"
func RenderASCIIProgress(percent float64, width int) string {
	// Calculate percentage for display
	pct := int(percent * ProgressPercentageScale)

	// Calculate how many characters should be filled
	filled := int(percent * float64(width))

	// Build the progress bar
	var bar strings.Builder
	bar.WriteString("[")

	// Constants for arrow rendering
	const (
		minWideBarWidth    = 3 // Minimum width to show equals before arrow
		arrowSpaceReserved = 2 // Space reserved for arrow and spacing in wide bars
	)

	// Write filled portion
	switch {
	case filled >= width:
		// At 100%, fill completely with =
		bar.WriteString(strings.Repeat("=", width))
	case percent > 0:
		// Between 0% and 100%, show arrow with optional = chars before it
		// The arrow represents the progress point, with = chars showing what's complete
		// For wider bars (filled >= 3), use filled-2 to leave room for the arrow
		// For narrow bars (filled < 3), use filled-1 to ensure arrow is shown
		var equalsCount int
		if filled >= minWideBarWidth {
			equalsCount = filled - arrowSpaceReserved
		} else {
			equalsCount = max(0, filled-1)
		}

		spacesCount := width - equalsCount - 1

		bar.WriteString(strings.Repeat("=", equalsCount))
		bar.WriteString(">")
		bar.WriteString(strings.Repeat(" ", spacesCount))
	default:
		// At 0%, just spaces
		bar.WriteString(strings.Repeat(" ", width))
	}

	bar.WriteString("]")

	return fmt.Sprintf("%s %d%%", bar.String(), pct)
}

// RenderProgress is a wrapper that renders progress using either Bubble Tea's progress bar
// or an ASCII fallback depending on terminal capabilities.
// When NO_COLOR is set or TERM=dumb, it uses the ASCII fallback.
// Otherwise, it delegates to the Bubble Tea progress model.
func RenderProgress(model progress.Model, percent float64) string {
	if colorsDisabled {
		// Use ASCII fallback when colors are disabled
		return RenderASCIIProgress(percent, model.Width)
	}

	// Use Bubble Tea's styled progress bar
	return model.ViewAs(percent)
}
