package shared

import "github.com/charmbracelet/lipgloss"

// Exported constants organized by category for clarity.
const (
	// ============================================================================
	// UI Layout & Display
	// ============================================================================

	// DefaultPadding is the default padding for UI elements
	DefaultPadding = 2
	// ProgressBarWidth is the default width of progress bars
	ProgressBarWidth = 40
	// MaxProgressBarWidth is the maximum width for progress bars
	MaxProgressBarWidth = 100
	// ProgressLogThreshold is the margin for path display calculations
	ProgressLogThreshold = 20

	// ============================================================================
	// Time Intervals
	// ============================================================================

	// TickIntervalMs is the interval for tick messages in milliseconds
	TickIntervalMs = 100
	// StatusUpdateThrottleMs is the minimum interval between status updates in milliseconds
	StatusUpdateThrottleMs = 200
	// ProgressUpdateInterval is how often to update progress (every N files)
	ProgressUpdateInterval = 10

	// ============================================================================
	// Display Limits & Formatting
	// ============================================================================

	// ProgressEllipsisLength is the length of ellipsis for truncated paths
	ProgressEllipsisLength = 3
	// ProgressPercentageScale is the scale for percentage calculations (100 for percentages)
	ProgressPercentageScale = 100

	// ============================================================================
	// Mathematical Constants
	// ============================================================================

	// ProgressHalfDivisor is the divisor for calculating half values
	ProgressHalfDivisor = 2

	// ============================================================================
	// Keys & Symbols
	// ============================================================================

	// KeyCtrlC is the key binding for cancellation
	KeyCtrlC = "ctrl+c"
	// PromptArrow is the arrow character used in prompts
	PromptArrow = "▶ "

	// ============================================================================
	// State Constants
	// ============================================================================

	StateBalanced    = "balanced"
	StateCancelled   = "cancelled"
	StateComplete    = "complete"
	StateDestination = "destination"
	StateError       = "error"
	StateSource      = "source"
)

func AccentColor() lipgloss.Color { return lipgloss.Color(accentColorCode) }

// ============================================================================
// Box and Container Styles
// ============================================================================

// BoxStyle returns the style for boxes with padding
func BoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(AccentColor()).
		Padding(1, DefaultPadding)
}

// CompletionSelectedStyle returns the style for selected completion items
func CompletionSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(HighlightColor()).
		Bold(true)
}

// ============================================================================
// Completion Styles (for path completion)
// ============================================================================

// CompletionStyle returns the style for completion items
func CompletionStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(NormalColor())
}

func DimColor() lipgloss.Color { return lipgloss.Color(dimColorCode) }

// DimStyle returns the style for dimmed text
func DimStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(DimColor())
}

func ErrorColor() lipgloss.Color { return lipgloss.Color(errorColorCode) }

// ErrorStyle returns the style for error messages
func ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ErrorColor()).
		Bold(true)
}

// FileItemCompleteStyle returns the style for completed file items
func FileItemCompleteStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(SuccessColor())
}

// FileItemCopyingStyle returns the style for copying file items
func FileItemCopyingStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(WarningColor())
}

// FileItemErrorStyle returns the style for error file items
func FileItemErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ErrorColor())
}

// ============================================================================
// File Item Styles (for file lists)
// ============================================================================

// FileItemStyle returns the style for normal file items
func FileItemStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(NormalColor())
}

func HighlightColor() lipgloss.Color { return lipgloss.Color(highlightColorCode) }

// LabelStyle returns the style for labels
func LabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(HighlightColor()).
		Bold(true)
}

func NormalColor() lipgloss.Color { return lipgloss.Color(normalColorCode) }

// PrimaryColor returns the primary color for the UI
func PrimaryColor() lipgloss.Color { return lipgloss.Color(primaryColorCode) }

// RenderBox renders content in a box with consistent styling
func RenderBox(content string) string {
	return BoxStyle().Render(content)
}

// RenderDim renders dimmed text with consistent styling
func RenderDim(text string) string {
	return DimStyle().Render(text)
}

// RenderError renders an error message with consistent styling
func RenderError(text string) string {
	return ErrorStyle().Render(text)
}

// RenderLabel renders a label with consistent styling
func RenderLabel(text string) string {
	return LabelStyle().Render(text)
}

// RenderSubtitle renders a subtitle with consistent styling
func RenderSubtitle(text string) string {
	return SubtitleStyle().Render(text)
}

// RenderSuccess renders a success message with consistent styling
func RenderSuccess(text string) string {
	return SuccessStyle().Render(text)
}

// ============================================================================
// Helper Functions
// ============================================================================

// RenderTitle renders a title with consistent styling
func RenderTitle(text string) string {
	return TitleStyle().Render(text)
}

// RenderWarning renders a warning message with consistent styling
func RenderWarning(text string) string {
	return WarningStyle().Render(text)
}

// SubtitleStyle returns the style for subtitles
func SubtitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(SubtleColor()).
		MarginBottom(1)
}

func SubtleColor() lipgloss.Color { return lipgloss.Color(subtleColorCode) }

func SuccessColor() lipgloss.Color { return lipgloss.Color(successColorCode) }

// SuccessStyle returns the style for success messages
func SuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(SuccessColor()).
		Bold(true)
}

// ============================================================================
// Text Styles
// ============================================================================

// TitleStyle returns the style for titles
func TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor()).
		MarginBottom(1)
}

func WarningColor() lipgloss.Color { return lipgloss.Color(warningColorCode) }

// WarningStyle returns the style for warning messages
func WarningStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(WarningColor()).
		Bold(true)
}

// unexported constants.
const (
	accentColorCode    = "62"  // Blue
	dimColorCode       = "240" // Dark gray
	errorColorCode     = "196" // Red
	highlightColorCode = "86"  // Cyan
	normalColorCode    = "252" // Light gray
	// Primary colors
	primaryColorCode = "205" // Pink/purple
	subtleColorCode  = "241" // Medium gray
	successColorCode = "42"  // Green
	warningColorCode = "226"
)
