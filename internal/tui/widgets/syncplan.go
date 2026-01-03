package widgets

import (
	"fmt"

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// NewSyncPlanWidget creates a widget that displays the sync plan summary.
// Returns a closure that formats the sync plan from the status.
func NewSyncPlanWidget(getStatus func() *syncengine.Status) func() string {
	return func() string {
		status := getStatus()
		if status == nil {
			return "Files to sync: 0\nTotal size: 0 B"
		}

		return fmt.Sprintf("Files to sync: %d\nTotal size: %s",
			status.TotalFiles,
			shared.FormatBytes(status.TotalBytes))
	}
}
