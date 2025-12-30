package syncengine

import "time"

// Exported constants.
const (
	// MinPathDisplayWidth is the minimum width for displaying file paths.
	MinPathDisplayWidth = 20
	// NumProgressDimensions is the number of dimensions (files, bytes, time) averaged for overall progress.
	NumProgressDimensions = 3.0
	// ProgressPercentageScale converts 0-1 range to 0-100 range.
	ProgressPercentageScale = 100.0
)

// ProgressMetrics encapsulates all progress calculation results for display.
// These metrics are computed periodically by the engine and consumed by the UI
// to provide real-time feedback on sync progress.
type ProgressMetrics struct {
	// FilesPercent represents the percentage of files processed out of total files.
	// Calculated as: (AlreadySyncedFiles + ProcessedFiles) / TotalFilesInSource
	FilesPercent float64

	// BytesPercent represents the percentage of bytes transferred out of total bytes.
	// Calculated as: (AlreadySyncedBytes + TransferredBytes) / TotalBytesInSource
	BytesPercent float64

	// TimePercent represents the estimated time progress.
	// Calculated as: elapsed / (elapsed + estimatedTimeRemaining)
	TimePercent float64

	// OverallPercent is the average of FilesPercent, BytesPercent, and TimePercent.
	// This provides a unified progress metric that balances all three dimensions.
	OverallPercent float64
}

// RateSample represents a point-in-time performance measurement.
// Samples are collected when files complete and stored in a rolling window
// to track recent performance trends.
type RateSample struct {
	// Timestamp is when this sample was recorded.
	Timestamp time.Time

	// BytesTransferred is the number of bytes transferred in this sample period.
	BytesTransferred int64

	// ReadTime is the duration spent reading from source for this sample.
	ReadTime time.Duration

	// WriteTime is the duration spent writing to destination for this sample.
	WriteTime time.Duration

	// ActiveWorkers is the number of workers active when this sample was taken.
	ActiveWorkers int
}

// WorkerMetrics tracks per-worker performance using a rolling window approach.
// Metrics are calculated based on recent samples (typically last 5 files) to
// ensure responsiveness to changing network or disk performance conditions.
type WorkerMetrics struct {
	// ReadPercent is the percentage of time workers spent reading from source.
	// Calculated from recent samples in the rolling window.
	ReadPercent float64

	// WritePercent is the percentage of time workers spent writing to destination.
	// Calculated from recent samples in the rolling window.
	WritePercent float64

	// PerWorkerRate is the average transfer rate per active worker in bytes/sec.
	// Calculated from recent samples in the rolling window.
	PerWorkerRate float64

	// TotalRate is the aggregate transfer rate across all workers in bytes/sec.
	// Calculated from recent samples in the rolling window.
	TotalRate float64

	// RecentSamples maintains a rolling window of recent performance measurements.
	// Used to calculate the above metrics based on recent activity rather than
	// cumulative totals, ensuring metrics reflect current performance.
	RecentSamples []RateSample
}
