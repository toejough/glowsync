package shared

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Exported constants.
const (
	// DefaultPadding is the default padding for UI elements
	DefaultPadding = 2
	// KeyCtrlC is the key binding for cancellation
	KeyCtrlC = "ctrl+c"
	// MaxProgressBarWidth is the maximum width for progress bars
	MaxProgressBarWidth = 100
	// PhaseComparing indicates files are being compared
	PhaseComparing      = "comparing"
	PhaseCountingDest   = "counting_dest"   // PhaseCountingDest indicates destination files are being counted
	PhaseCountingSource = "counting_source" // PhaseCountingSource indicates source files are being counted
	PhaseDeleting       = "deleting"        // PhaseDeleting indicates files are being deleted
	PhaseScanningDest   = "scanning_dest"   // PhaseScanningDest indicates destination directory is being scanned
	PhaseScanningSource = "scanning_source" // PhaseScanningSource indicates source directory is being scanned
	// ProgressBarWidth is the default width of progress bars
	ProgressBarWidth = 40
	// ProgressEllipsisLength is the length of ellipsis for truncated paths
	ProgressEllipsisLength = 3
	// ProgressHalfDivisor is the divisor for calculating half values
	ProgressHalfDivisor = 2
	// ProgressLogThreshold is the margin for path display calculations
	ProgressLogThreshold = 20
	// ProgressPercentageScale is the scale for percentage calculations (100 for percentages)
	ProgressPercentageScale = 100
	// ProgressUpdateInterval is how often to update progress (every N files)
	ProgressUpdateInterval = 10
	// StateBalanced indicates balanced load between source and destination
	StateBalanced = "balanced"
	// StateCancelled indicates the operation was cancelled
	StateCancelled   = "cancelled"
	StateComplete    = "complete"    // StateComplete indicates successful completion
	StateDestination = "destination" // StateDestination indicates destination is the bottleneck
	StateError       = "error"       // StateError indicates an error occurred
	StateSource      = "source"      // StateSource indicates source is the bottleneck
	// StatusUpdateThrottleMs is the minimum interval between status updates in milliseconds
	StatusUpdateThrottleMs = 200
	// TickIntervalMs is the interval for tick messages in milliseconds
	TickIntervalMs = 100
)

func AccentColor() lipgloss.Color {
	if colorsDisabled {
		return lipgloss.Color("")
	}

	return lipgloss.Color(accentColorCode)
}

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

func DimColor() lipgloss.Color {
	if colorsDisabled {
		return lipgloss.Color("")
	}

	return lipgloss.Color(dimColorCode)
}

// DimStyle returns the style for dimmed text
func DimStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(DimColor())
}

func ErrorColor() lipgloss.Color {
	if colorsDisabled {
		return lipgloss.Color("")
	}

	return lipgloss.Color(errorColorCode)
}

// ErrorStyle returns the style for error messages
func ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ErrorColor()).
		Bold(true)
}

// ============================================================================
// Symbol Helper Functions (with ASCII fallbacks)
// ============================================================================

// ErrorSymbol returns an X with ASCII fallback
func ErrorSymbol() string {
	if unicodeDisabled {
		return "[X]"
	}

	return "✗"
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

func HighlightColor() lipgloss.Color {
	if colorsDisabled {
		return lipgloss.Color("")
	}

	return lipgloss.Color(highlightColorCode)
}

// LabelStyle returns the style for labels
func LabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(HighlightColor()).
		Bold(true)
}

// MakePathClickable wraps a file path with OSC 8 hyperlink escape codes
// to make it clickable in supported terminals (iTerm2, Terminal.app, etc.)
func MakePathClickable(path string) string {
	// OSC 8 format: \033]8;;file://path\033\\text\033]8;;\033\\
	return fmt.Sprintf("\033]8;;file://%s\033\\%s\033]8;;\033\\", path, path)
}

func NormalColor() lipgloss.Color {
	if colorsDisabled {
		return lipgloss.Color("")
	}

	return lipgloss.Color(normalColorCode)
}

// PendingSymbol returns a circle symbol with ASCII fallback
func PendingSymbol() string {
	if unicodeDisabled {
		return "[ ]"
	}

	return "○"
}

// PrimaryColor returns the primary color for the UI
func PrimaryColor() lipgloss.Color {
	if colorsDisabled {
		return lipgloss.Color("")
	}

	return lipgloss.Color(primaryColorCode)
}

// PromptArrow returns the prompt arrow symbol with ASCII fallback
func PromptArrow() string {
	if unicodeDisabled {
		return "> "
	}

	return "▶ "
}

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

// RightArrow returns a right arrow with ASCII fallback
func RightArrow() string {
	if unicodeDisabled {
		return "->"
	}

	return "→"
}

// SubtitleStyle returns the style for subtitles
func SubtitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(SubtleColor()).
		MarginBottom(1)
}

func SubtleColor() lipgloss.Color {
	if colorsDisabled {
		return lipgloss.Color("")
	}

	return lipgloss.Color(subtleColorCode)
}

func SuccessColor() lipgloss.Color {
	if colorsDisabled {
		return lipgloss.Color("")
	}

	return lipgloss.Color(successColorCode)
}

// SuccessStyle returns the style for success messages
func SuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(SuccessColor()).
		Bold(true)
}

// SuccessSymbol returns a checkmark with ASCII fallback
func SuccessSymbol() string {
	if unicodeDisabled {
		return "[OK]"
	}

	return "✓"
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

func WarningColor() lipgloss.Color {
	if colorsDisabled {
		return lipgloss.Color("")
	}

	return lipgloss.Color(warningColorCode)
}

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

// unexported variables.
var (
	// colorsDisabled is true when NO_COLOR is set or TERM=dumb
	//nolint:gochecknoglobals // Required for terminal capability detection
	colorsDisabled = os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
	// unicodeDisabled is true when terminal doesn't support Unicode
	//nolint:gochecknoglobals // Required for terminal capability detection
	unicodeDisabled = os.Getenv("TERM") == "dumb" || os.Getenv("LANG") == "C"
)

// GetColorsDisabled returns the current value of colorsDisabled for testing purposes.
func GetColorsDisabled() bool {
	return colorsDisabled
}

// SetColorsDisabledForTesting sets the colorsDisabled variable for testing purposes.
// This should only be used in tests to control the behavior of color-dependent functions.
func SetColorsDisabledForTesting(disabled bool) {
	colorsDisabled = disabled
}
