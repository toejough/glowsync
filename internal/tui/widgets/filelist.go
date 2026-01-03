package widgets

import (
	"fmt"
	"strings"

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

const (
	maxVisibleFiles    = 20
	progressBarWidth   = 20
	statusCopying      = "copying"
	statusOpening      = "opening"
	statusFinalizing   = "finalizing"
	percentageScale    = 100
)

// NewFileListWidget creates a widget that displays currently syncing files with progress.
// Returns a closure that formats the file list from the status.
func NewFileListWidget(getStatus func() *syncengine.Status) func() string {
	return func() string {
		status := getStatus()
		if status == nil || len(status.FilesToSync) == 0 {
			return ""
		}

		var builder strings.Builder
		filesShown := 0

		for _, file := range status.FilesToSync {
			if filesShown >= maxVisibleFiles {
				break
			}

			// Only show files that are actively being processed
			switch file.Status {
			case statusCopying:
				var percent float64
				if file.Size > 0 {
					percent = float64(file.Transferred) / float64(file.Size)
				}

				progressBar := shared.RenderASCIIProgress(percent, progressBarWidth)
				fmt.Fprintf(&builder, "[%s] %.1f%% %s\n",
					progressBar,
					percent*percentageScale,
					file.RelativePath)

				filesShown++
			case statusOpening:
				fmt.Fprintf(&builder, "Opening: %s\n", file.RelativePath)
				filesShown++
			case statusFinalizing:
				fmt.Fprintf(&builder, "Finalizing: %s\n", file.RelativePath)
				filesShown++
			}
		}

		return builder.String()
	}
}
