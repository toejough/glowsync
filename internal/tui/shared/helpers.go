package shared

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/joe/copy-files/pkg/formatters"
)

// CalculateMaxPathWidth returns max path display width based on terminal width
func CalculateMaxPathWidth(terminalWidth int) int {
	return max(terminalWidth-ProgressLogThreshold, ProgressBarWidth)
}

// ============================================================================
// Formatting Functions
// These are used by multiple screens for consistent display
// ============================================================================

// FormatBytes formats bytes into human-readable format (e.g., "1.5 MB")
func FormatBytes(bytes int64) string {
	return formatters.FormatBytes(bytes)
}

// FormatDuration formats duration into human-readable format (e.g., "2m 30s")
func FormatDuration(duration time.Duration) string {
	duration = duration.Round(time.Second)
	hours := duration / time.Hour
	duration %= time.Hour
	minutes := duration / time.Minute
	duration %= time.Minute
	seconds := duration / time.Second

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}

	return fmt.Sprintf("%ds", seconds)
}

// FormatRate formats transfer rate into human-readable format (e.g., "5.2 MB/s")
func FormatRate(bytesPerSec float64) string {
	const unit = 1024.0
	if bytesPerSec < unit {
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}

	div, exp := unit, 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB/s", bytesPerSec/div, "KMGTPE"[exp])
}

// RenderEmptyListPlaceholder renders a dimmed placeholder message for empty lists
func RenderEmptyListPlaceholder(message string) string {
	return RenderDim(message)
}

// RenderPath returns a styled path string, truncated if necessary
func RenderPath(path string, style lipgloss.Style, maxWidth int) string {
	truncatedPath := TruncatePath(path, maxWidth)
	return style.Render(truncatedPath)
}

// RenderPathPlain returns a plain (unstyled) path string, truncated if necessary
func RenderPathPlain(path string, maxWidth int) string {
	return TruncatePath(path, maxWidth)
}

// ============================================================================
// Path Display Utilities
// These functions help format paths for display in constrained terminal widths
// ============================================================================

// TruncatePath truncates a path from the middle if it exceeds maxWidth
func TruncatePath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}
	// Truncate from the middle
	halfWidth := (maxWidth - ProgressEllipsisLength) / ProgressHalfDivisor

	return path[:halfWidth] + "..." + path[len(path)-halfWidth:]
}
