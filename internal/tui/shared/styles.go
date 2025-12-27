package shared

import "github.com/charmbracelet/lipgloss"

// ============================================================================
// Color Palette
// ============================================================================

var (
	// Primary colors
	PrimaryColor = lipgloss.Color("205") // Pink/purple
	AccentColor  = lipgloss.Color("62")  // Blue

	// Status colors
	SuccessColor = lipgloss.Color("42")  // Green
	ErrorColor   = lipgloss.Color("196") // Red
	WarningColor = lipgloss.Color("226") // Yellow

	// Text colors
	DimColor    = lipgloss.Color("240") // Dark gray
	NormalColor = lipgloss.Color("252") // Light gray
	SubtleColor = lipgloss.Color("241") // Medium gray
	HighlightColor = lipgloss.Color("86") // Cyan
)

// ============================================================================
// Text Styles
// ============================================================================

var (
	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(SubtleColor).
			MarginBottom(1)

	// Label and content styles
	LabelStyle = lipgloss.NewStyle().
			Foreground(HighlightColor).
			Bold(true)

	DimStyle = lipgloss.NewStyle().
			Foreground(DimColor)

	// Status styles
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)
)

// ============================================================================
// Box and Container Styles
// ============================================================================

var (
	// Box with border
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(AccentColor).
			Padding(1, 2)

	// Inline box (no padding)
	InlineBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(AccentColor)
)

// ============================================================================
// File Item Styles (for file lists)
// ============================================================================

var (
	FileItemStyle = lipgloss.NewStyle().
			Foreground(NormalColor)

	FileItemCompleteStyle = lipgloss.NewStyle().
				Foreground(SuccessColor)

	FileItemCopyingStyle = lipgloss.NewStyle().
				Foreground(WarningColor)

	FileItemErrorStyle = lipgloss.NewStyle().
				Foreground(ErrorColor)
)

// ============================================================================
// Completion Styles (for path completion)
// ============================================================================

var (
	CompletionStyle = lipgloss.NewStyle().
			Foreground(NormalColor)

	CompletionSelectedStyle = lipgloss.NewStyle().
				Foreground(HighlightColor).
				Bold(true)
)

// ============================================================================
// Helper Functions
// ============================================================================

// RenderTitle renders a title with consistent styling
func RenderTitle(text string) string {
	return TitleStyle.Render(text)
}

// RenderSubtitle renders a subtitle with consistent styling
func RenderSubtitle(text string) string {
	return SubtitleStyle.Render(text)
}

// RenderLabel renders a label with consistent styling
func RenderLabel(text string) string {
	return LabelStyle.Render(text)
}

// RenderError renders an error message with consistent styling
func RenderError(text string) string {
	return ErrorStyle.Render(text)
}

// RenderSuccess renders a success message with consistent styling
func RenderSuccess(text string) string {
	return SuccessStyle.Render(text)
}

// RenderWarning renders a warning message with consistent styling
func RenderWarning(text string) string {
	return WarningStyle.Render(text)
}

// RenderDim renders dimmed text with consistent styling
func RenderDim(text string) string {
	return DimStyle.Render(text)
}

// RenderBox renders content in a box with consistent styling
func RenderBox(content string) string {
	return BoxStyle.Render(content)
}
