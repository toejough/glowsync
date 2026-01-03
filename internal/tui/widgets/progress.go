package widgets

import (
	"fmt"

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// NewProgressWidget creates a widget that displays file sync progress.
// Returns a closure that formats the current progress from the status.
func NewProgressWidget(getStatus func() *syncengine.Status) func() string {
	return func() string {
		status := getStatus()
		if status == nil {
			return "Progress: 0 / 0 files (0%)\n0 B / 0 B"
		}

		// Calculate percentage, handling division by zero
		var percent float64
		if status.TotalFiles > 0 {
			percent = float64(status.ProcessedFiles) / float64(status.TotalFiles) * 100 //nolint:mnd // Percentage calculation
		}

		// Format progress message
		return fmt.Sprintf("Files: %d / %d (%.1f%%)\nBytes: %s / %s",
			status.ProcessedFiles,
			status.TotalFiles,
			percent,
			shared.FormatBytes(status.TransferredBytes),
			shared.FormatBytes(status.TotalBytes))
	}
}
