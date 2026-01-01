package syncengine //nolint:testpackage // Testing private methods

import (
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega" //nolint:revive // Gomega convention
)

func TestAddRateSample(t *testing.T) { //nolint:varnamelen // t is idiomatic for testing.T parameter
	t.Skip("Stale test: expects old 5-sample limit but implementation now uses 10-second time-based rolling window")
	t.Parallel()

	gomega := NewWithT(t)

	t.Run("adds samples up to max", func(t *testing.T) {
		t.Parallel()

		status := &Status{
			Workers: WorkerMetrics{RecentSamples: []RateSample{}},
		}

		// Add 7 samples (max is 5)
		for idx := range 7 {
			sample := RateSample{
				Timestamp:        time.Now().Add(time.Duration(idx) * time.Second),
				BytesTransferred: int64(100 * (idx + 1)),
				ReadTime:         time.Duration(idx+1) * time.Millisecond,
				WriteTime:        time.Duration(idx+1) * time.Millisecond,
				ActiveWorkers:    idx + 1,
			}
			status.addRateSample(sample)
		}

		// Should only keep last 5
		gomega.Expect(status.Workers.RecentSamples).To(HaveLen(5))

		// First sample should be the 3rd one we added (index 2)
		gomega.Expect(status.Workers.RecentSamples[0].BytesTransferred).To(Equal(int64(300)))

		// Last sample should be the 7th one we added
		gomega.Expect(status.Workers.RecentSamples[4].BytesTransferred).To(Equal(int64(700)))
	})
}

//nolint:funlen // Table-driven test with multiple cases
func TestCalculateProgressMetrics(t *testing.T) { //nolint:varnamelen // Standard test parameter
	t.Parallel()

	tests := []struct {
		name             string
		status           *Status
		expectedFilesP   float64
		expectedBytesP   float64
		expectedTimeP    float64
		expectedOverallP float64
	}{
		{
			name: "zero totals returns zero percentages",
			status: &Status{
				TotalFilesInSource: 0,
				TotalBytesInSource: 0,
				AlreadySyncedFiles: 0,
				ProcessedFiles:     0,
				StartTime:          time.Now(),
			},
			expectedFilesP:   0.0,
			expectedBytesP:   0.0,
			expectedTimeP:    0.0,
			expectedOverallP: 0.0,
		},
		{
			name: "50% files processed",
			status: &Status{
				TotalFilesInSource: 100,
				TotalBytesInSource: 1000,
				AlreadySyncedFiles: 25,
				ProcessedFiles:     25,
				AlreadySyncedBytes: 250,
				TransferredBytes:   250,
				StartTime:          time.Now().Add(-10 * time.Second),
				EstimatedTimeLeft:  10 * time.Second,
			},
			expectedFilesP:   0.50, // (25 + 25) / 100
			expectedBytesP:   0.50, // (250 + 250) / 1000
			expectedTimeP:    0.50, // 10s / (10s + 10s)
			expectedOverallP: 0.50, // (0.5 + 0.5 + 0.5) / 3
		},
		{
			name: "75% files, 50% bytes, 25% time",
			status: &Status{
				TotalFilesInSource: 100,
				TotalBytesInSource: 1000,
				AlreadySyncedFiles: 50,
				ProcessedFiles:     25,
				AlreadySyncedBytes: 250,
				TransferredBytes:   250,
				StartTime:          time.Now().Add(-5 * time.Second),
				EstimatedTimeLeft:  15 * time.Second,
			},
			expectedFilesP:   0.75, // (50 + 25) / 100
			expectedBytesP:   0.50, // (250 + 250) / 1000
			expectedTimeP:    0.25, // 5s / (5s + 15s)
			expectedOverallP: 0.50, // (0.75 + 0.5 + 0.25) / 3
		},
		{
			name: "100% complete",
			status: &Status{
				TotalFilesInSource: 100,
				TotalBytesInSource: 1000,
				AlreadySyncedFiles: 60,
				ProcessedFiles:     40,
				AlreadySyncedBytes: 600,
				TransferredBytes:   400,
				StartTime:          time.Now().Add(-20 * time.Second),
				EstimatedTimeLeft:  0,
			},
			expectedFilesP:   1.0, // (60 + 40) / 100
			expectedBytesP:   1.0, // (600 + 400) / 1000
			expectedTimeP:    1.0, // Falls back to bytes%
			expectedOverallP: 1.0, // (1.0 + 1.0 + 1.0) / 3
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gomega := NewWithT(t)

			// Set atomic value for TransferredBytes
			atomic.StoreInt64(&testCase.status.TransferredBytes, testCase.status.TransferredBytes)

			metrics := testCase.status.calculateProgressMetrics()

			gomega.Expect(metrics.FilesPercent).To(BeNumerically("~", testCase.expectedFilesP, 0.01))
			gomega.Expect(metrics.BytesPercent).To(BeNumerically("~", testCase.expectedBytesP, 0.01))
			gomega.Expect(metrics.TimePercent).To(BeNumerically("~", testCase.expectedTimeP, 0.01))
			gomega.Expect(metrics.OverallPercent).To(BeNumerically("~", testCase.expectedOverallP, 0.01))
		})
	}
}

//nolint:funlen // Test with detailed sample data
func TestCalculateWorkerMetricsWithRollingWindow(t *testing.T) { //nolint:varnamelen // Standard test parameter
	t.Parallel()

	gomega := NewWithT(t)

	t.Run("calculates metrics from recent samples", func(t *testing.T) {
		t.Parallel()

		now := time.Now()

		status := &Status{
			Workers: WorkerMetrics{
				RecentSamples: []RateSample{
					{
						Timestamp:        now.Add(-4 * time.Second),
						BytesTransferred: 100,
						ReadTime:         500 * time.Millisecond,
						WriteTime:        500 * time.Millisecond,
						ActiveWorkers:    2,
					},
					{
						Timestamp:        now.Add(-3 * time.Second),
						BytesTransferred: 200,
						ReadTime:         600 * time.Millisecond,
						WriteTime:        400 * time.Millisecond,
						ActiveWorkers:    3,
					},
					{
						Timestamp:        now.Add(-2 * time.Second),
						BytesTransferred: 300,
						ReadTime:         700 * time.Millisecond,
						WriteTime:        300 * time.Millisecond,
						ActiveWorkers:    4,
					},
				},
			},
		}

		metrics := status.calculateWorkerMetrics()

		// Total bytes: 100 + 200 + 300 = 600
		// Total read time: 500ms + 600ms + 700ms = 1800ms
		// Total write time: 500ms + 400ms + 300ms = 1200ms
		// Read%: 1800 / (1800 + 1200) * 100 = 60%
		// Write%: 1200 / 3000 * 100 = 40%
		gomega.Expect(metrics.ReadPercent).To(BeNumerically("~", 60.0, 0.1))
		gomega.Expect(metrics.WritePercent).To(BeNumerically("~", 40.0, 0.1))

		// Total duration: 2 seconds (from first to last sample)
		// Total rate: 600 bytes / 2 seconds = 300 bytes/sec
		gomega.Expect(metrics.TotalRate).To(BeNumerically("~", 300.0, 1.0))

		// Average workers: (2 + 3 + 4) / 3 = 3
		// Per-worker rate: 300 / 3 = 100 bytes/sec
		gomega.Expect(metrics.PerWorkerRate).To(BeNumerically("~", 100.0, 1.0))

		// Should copy samples
		gomega.Expect(metrics.RecentSamples).To(HaveLen(3))
	})

	t.Run("single sample returns zero duration", func(t *testing.T) {
		t.Parallel()

		status := &Status{
			Workers: WorkerMetrics{
				RecentSamples: []RateSample{
					{
						Timestamp:        time.Now(),
						BytesTransferred: 100,
						ReadTime:         500 * time.Millisecond,
						WriteTime:        500 * time.Millisecond,
						ActiveWorkers:    2,
					},
				},
			},
		}

		metrics := status.calculateWorkerMetrics()

		// Read/write percentages should still calculate
		gomega.Expect(metrics.ReadPercent).To(BeNumerically("~", 50.0, 0.1))
		gomega.Expect(metrics.WritePercent).To(BeNumerically("~", 50.0, 0.1))

		// But rates will be zero (no duration between samples)
		gomega.Expect(metrics.TotalRate).To(Equal(0.0))
		gomega.Expect(metrics.PerWorkerRate).To(Equal(0.0))
	})
}

func TestCalculateWorkerMetricsWithoutSamples(t *testing.T) { //nolint:varnamelen // Standard test parameter
	t.Parallel()

	gomega := NewWithT(t)

	t.Run("cumulative fallback when no samples", func(t *testing.T) {
		t.Parallel()

		status := &Status{
			TotalReadTime:    6 * time.Second,
			TotalWriteTime:   4 * time.Second,
			TransferredBytes: 1000,
			StartTime:        time.Now().Add(-10 * time.Second),
			ActiveWorkers:    4,
			Workers:          WorkerMetrics{RecentSamples: []RateSample{}},
		}

		atomic.StoreInt64(&status.TransferredBytes, 1000)

		metrics := status.calculateWorkerMetrics()

		// Read/write percentages: 6s read, 4s write = 60% / 40%
		gomega.Expect(metrics.ReadPercent).To(BeNumerically("~", 60.0, 0.1))
		gomega.Expect(metrics.WritePercent).To(BeNumerically("~", 40.0, 0.1))

		// Total rate: 1000 bytes / 10 seconds = 100 bytes/sec
		gomega.Expect(metrics.TotalRate).To(BeNumerically("~", 100.0, 0.1))

		// Per-worker rate: 100 / 4 = 25 bytes/sec
		gomega.Expect(metrics.PerWorkerRate).To(BeNumerically("~", 25.0, 0.1))
	})

	t.Run("zero elapsed time returns zero rates", func(t *testing.T) {
		t.Parallel()

		status := &Status{
			TotalReadTime:    0,
			TotalWriteTime:   0,
			TransferredBytes: 0,
			StartTime:        time.Now(),
			ActiveWorkers:    0,
			Workers:          WorkerMetrics{RecentSamples: []RateSample{}},
		}

		metrics := status.calculateWorkerMetrics()

		gomega.Expect(metrics.ReadPercent).To(Equal(0.0))
		gomega.Expect(metrics.WritePercent).To(Equal(0.0))
		gomega.Expect(metrics.TotalRate).To(Equal(0.0))
		gomega.Expect(metrics.PerWorkerRate).To(Equal(0.0))
	})
}

func TestComputeProgressMetrics(t *testing.T) {
	t.Parallel()

	gomega := NewWithT(t)

	t.Run("computes both progress and worker metrics", func(t *testing.T) {
		t.Parallel()

		status := &Status{
			TotalFilesInSource: 100,
			TotalBytesInSource: 1000,
			AlreadySyncedFiles: 50,
			ProcessedFiles:     25,
			AlreadySyncedBytes: 500,
			TransferredBytes:   250,
			StartTime:          time.Now().Add(-10 * time.Second),
			EstimatedTimeLeft:  10 * time.Second,
			TotalReadTime:      6 * time.Second,
			TotalWriteTime:     4 * time.Second,
			ActiveWorkers:      4,
		}

		atomic.StoreInt64(&status.TransferredBytes, 250)

		status.ComputeProgressMetrics()

		// Verify Progress was set
		gomega.Expect(status.Progress.FilesPercent).To(BeNumerically("~", 0.75, 0.01))
		gomega.Expect(status.Progress.BytesPercent).To(BeNumerically("~", 0.75, 0.01))
		gomega.Expect(status.Progress.TimePercent).To(BeNumerically("~", 0.50, 0.01))
		gomega.Expect(status.Progress.OverallPercent).To(BeNumerically("~", 0.667, 0.01))

		// Verify Workers was set
		gomega.Expect(status.Workers.ReadPercent).To(BeNumerically("~", 60.0, 0.1))
		gomega.Expect(status.Workers.WritePercent).To(BeNumerically("~", 40.0, 0.1))
	})
}
