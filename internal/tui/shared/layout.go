package shared

import "github.com/charmbracelet/lipgloss"

// RenderTwoColumnLayout renders content in two columns with 60-40 width split.
// Left column receives ~60% of width, right column receives ~40%.
// Columns are joined horizontally using lipgloss.
func RenderTwoColumnLayout(leftContent, rightContent string, width, height int) string {
	// Calculate column widths (60% left, 40% right)
	leftWidth := int(float64(width) * 0.6)
	rightWidth := width - leftWidth

	// Create styled columns with explicit dimensions
	leftStyle := lipgloss.NewStyle().Width(leftWidth).Height(height)
	rightStyle := lipgloss.NewStyle().Width(rightWidth).Height(height)

	// Join columns horizontally, aligned at top
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftStyle.Render(leftContent),
		rightStyle.Render(rightContent),
	)
}

// RenderWidgetBox renders content in a titled box with borders.
// Title is bold and colored using PrimaryColor.
// Content is wrapped in a box with borders.
// Width accounts for padding (width - 4 for borders and padding).
func RenderWidgetBox(title, content string, width int) string {
	const widthOverhead = 4 // Account for borders (2) and padding (2)

	// Style the title with bold and primary color
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor())

	// Create box with specified width (accounting for borders/padding)
	boxStyle := BoxStyle().Width(width - widthOverhead)

	// Combine title and content, then wrap in box
	rendered := titleStyle.Render(title) + "\n" + content

	return boxStyle.Render(rendered)
}
