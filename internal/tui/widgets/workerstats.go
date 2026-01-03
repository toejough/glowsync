package widgets

import (
	"fmt"

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// NewWorkerStatsWidget creates a widget that displays worker statistics.
// Returns a closure that formats worker stats from the status.
func NewWorkerStatsWidget(getStatus func() *syncengine.Status) func() string {
	return func() string {
		status := getStatus()
		if status == nil {
			return "Workers: 0\nSpeed: 0 B/s"
		}

		workerCount := len(status.CurrentFiles)
		totalSpeed := status.BytesPerSecond

		// Format worker count
		result := fmt.Sprintf("Workers: %d\n", workerCount)

		// Format total speed
		result += fmt.Sprintf("Speed: %s", shared.FormatRate(totalSpeed))

		// Add per-worker speed if we have workers
		if workerCount > 0 {
			perWorkerSpeed := totalSpeed / float64(workerCount)
			result += fmt.Sprintf(" (%s per worker)", shared.FormatRate(perWorkerSpeed))
		}

		return result
	}
}
