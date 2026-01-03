package widgets

import (
	"fmt"
	"time"

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// NewSummaryWidget creates a widget that displays the sync summary.
// Returns a closure that formats the summary from the status and error.
func NewSummaryWidget(getStatus func() *syncengine.Status, err error) func() string {
	return func() string {
		status := getStatus()
		if status == nil {
			if err != nil {
				return fmt.Sprintf("Error: %v", err)
			}

			return "No status available"
		}

		// Calculate elapsed time
		var elapsed time.Duration
		if !status.EndTime.IsZero() {
			elapsed = status.EndTime.Sub(status.StartTime)
		} else if !status.StartTime.IsZero() {
			elapsed = time.Since(status.StartTime)
		}

		// Build summary based on error state
		if err != nil {
			return fmt.Sprintf("Error: %v\n\nFiles synced: %d\nBytes transferred: %s\nTime elapsed: %s",
				err,
				status.ProcessedFiles,
				shared.FormatBytes(status.TransferredBytes),
				shared.FormatDuration(elapsed))
		}

		// Success message
		filesWord := "files"
		if status.ProcessedFiles == 1 {
			filesWord = "file"
		}

		return fmt.Sprintf("Sync complete!\n\nFiles synced: %d %s\nBytes transferred: %s\nTime elapsed: %s",
			status.ProcessedFiles,
			filesWord,
			shared.FormatBytes(status.TransferredBytes),
			shared.FormatDuration(elapsed))
	}
}
