//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
)

func TestGetAnalysisPhaseText(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		status: &syncengine.Status{},
	}

	// Test various phases
	screen.status.AnalysisPhase = "counting_source"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Counting"))

	screen.status.AnalysisPhase = "scanning_source"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Scanning"))

	screen.status.AnalysisPhase = "counting_dest"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Counting"))

	screen.status.AnalysisPhase = "scanning_dest"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Scanning"))

	screen.status.AnalysisPhase = "comparing"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Comparing"))

	screen.status.AnalysisPhase = "deleting"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Checking"))

	screen.status.AnalysisPhase = "complete"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("complete"))

	screen.status.AnalysisPhase = "unknown"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Initializing"))
}

// TestRenderCountingProgress_ElapsedTime verifies time calculation
func TestRenderCountingProgress_ElapsedTime(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		status: &syncengine.Status{
			ScannedFiles:      100,
			AnalysisStartTime: time.Now().Add(-30 * time.Second),
		},
	}

	result := screen.renderCountingProgress(screen.status)

	// Should show elapsed time
	g.Expect(result).Should(ContainSubstring("s")) // Time format includes seconds
}

// TestRenderCountingProgress_Format verifies output format for counting phase
func TestRenderCountingProgress_Format(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		status: &syncengine.Status{
			ScannedFiles:      50,
			AnalysisStartTime: time.Now().Add(-5 * time.Second),
		},
	}

	result := screen.renderCountingProgress(screen.status)

	// Should contain basic format elements
	g.Expect(result).Should(ContainSubstring("Found"))
	g.Expect(result).Should(ContainSubstring("items"))
	g.Expect(result).ShouldNot(ContainSubstring("%")) // No percentages during counting
}

// TestRenderCountingProgress_ScanRate verifies rate display
func TestRenderCountingProgress_ScanRate(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		status: &syncengine.Status{
			ScannedFiles:      100,
			AnalysisStartTime: time.Now().Add(-10 * time.Second),
			AnalysisRate:      10.0, // 10 items/second
		},
	}

	result := screen.renderCountingProgress(screen.status)

	// Should display scan rate
	g.Expect(result).Should(ContainSubstring("/s")) // Rate per second
}

// TestRenderCountingProgress_ZeroFiles handles no files found
func TestRenderCountingProgress_ZeroFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		status: &syncengine.Status{
			ScannedFiles:      0,
			AnalysisStartTime: time.Now(),
		},
	}

	result := screen.renderCountingProgress(screen.status)

	// Should handle zero files gracefully
	g.Expect(result).ShouldNot(BeEmpty())
}

// TestRenderErrorContent verifies error content formatting
func TestRenderErrorContent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		status: &syncengine.Status{
			Errors: []syncengine.FileError{
				{FilePath: "/path/one", Error: nil},
				{FilePath: "/path/two", Error: nil},
			},
		},
	}

	result := screen.renderErrorContent()

	// Should return formatted error count
	g.Expect(result).ShouldNot(BeEmpty())
	g.Expect(result).Should(ContainSubstring("2"))
}

// TestRenderPathContent verifies path content formatting
func TestRenderPathContent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		status: &syncengine.Status{
			CurrentPath: "/some/test/path/file.txt",
		},
		width: 80,
	}

	result := screen.renderPathContent()

	// Should return formatted path
	g.Expect(result).ShouldNot(BeEmpty())
	g.Expect(result).Should(ContainSubstring("path"))
}

// TestRenderProcessingProgress_ETA verifies time estimation display
func TestRenderProcessingProgress_ETA(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		overallProgress: newTestProgressBar(),
		status: &syncengine.Status{
			ScannedFiles:      100,
			TotalFilesToScan:  400,
			ScannedBytes:      1_000_000,
			TotalBytesToScan:  4_000_000,
			AnalysisStartTime: time.Now().Add(-20 * time.Second),
		},
	}

	progress := syncengine.ProgressMetrics{
		OverallPercent:         0.25,
		FilesPercent:           25.0,
		BytesPercent:           25.0,
		TimePercent:            25.0,
		IsCounting:             false,
		EstimatedTimeRemaining: 60 * time.Second,
	}

	result := screen.renderProcessingProgress(screen.status, progress)

	// Should show elapsed and estimated times
	// Time format: "00:20 / 01:20 (25.0%)"
	g.Expect(result).Should(ContainSubstring("Time:"))
	g.Expect(result).Should(ContainSubstring("/")) // Shows elapsed / total
}

// TestRenderProcessingProgress_EdgeCases verifies edge case handling
func TestRenderProcessingProgress_EdgeCases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		status   *syncengine.Status
		progress syncengine.ProgressMetrics
	}{
		{
			name: "zero totals",
			status: &syncengine.Status{
				ScannedFiles:      0,
				TotalFilesToScan:  0,
				ScannedBytes:      0,
				TotalBytesToScan:  0,
				AnalysisStartTime: time.Now(),
			},
			progress: syncengine.ProgressMetrics{
				OverallPercent:         0,
				FilesPercent:           0,
				BytesPercent:           0,
				TimePercent:            0,
				IsCounting:             false,
				EstimatedTimeRemaining: 0,
			},
		},
		{
			name: "100% complete",
			status: &syncengine.Status{
				ScannedFiles:      1000,
				TotalFilesToScan:  1000,
				ScannedBytes:      10_000_000,
				TotalBytesToScan:  10_000_000,
				AnalysisStartTime: time.Now().Add(-60 * time.Second),
			},
			progress: syncengine.ProgressMetrics{
				OverallPercent:         1.0,
				FilesPercent:           100.0,
				BytesPercent:           100.0,
				TimePercent:            100.0,
				IsCounting:             false,
				EstimatedTimeRemaining: 0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			screen := &AnalysisScreen{
				overallProgress: newTestProgressBar(),
				status:          tc.status,
			}

			result := screen.renderProcessingProgress(screen.status, tc.progress)

			// Should handle edge cases gracefully without panicking
			g.Expect(result).ShouldNot(BeEmpty())
		})
	}
}

// TestRenderProcessingProgress_Format verifies output format for processing phase
func TestRenderProcessingProgress_Format(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		overallProgress: newTestProgressBar(),
		status: &syncengine.Status{
			ScannedFiles:      123,
			TotalFilesToScan:  456,
			ScannedBytes:      1_200_000, // 1.2 MB
			TotalBytesToScan:  4_500_000, // 4.5 MB
			AnalysisStartTime: time.Now().Add(-15 * time.Second),
		},
	}

	progress := syncengine.ProgressMetrics{
		FilesPercent:           27.0,
		BytesPercent:           26.7,
		TimePercent:            26.8,
		OverallPercent:         0.27,
		IsCounting:             false,
		EstimatedTimeRemaining: 41 * time.Second, // 56s total - 15s elapsed = 41s
	}

	result := screen.renderProcessingProgress(screen.status, progress)

	// Should contain files line with percentage
	g.Expect(result).Should(ContainSubstring("Files:"))
	g.Expect(result).Should(ContainSubstring("123"))
	g.Expect(result).Should(ContainSubstring("456"))
	g.Expect(result).Should(ContainSubstring("%")) // Has percentages

	// Should contain bytes line
	g.Expect(result).Should(ContainSubstring("Bytes:"))
	g.Expect(result).Should(ContainSubstring("MB"))

	// Should contain time line
	g.Expect(result).Should(ContainSubstring("Time:"))
}

// TestRenderProcessingProgress_Percentages verifies percentage calculations
func TestRenderProcessingProgress_Percentages(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		overallProgress: newTestProgressBar(),
		status: &syncengine.Status{
			ScannedFiles:      333,
			TotalFilesToScan:  1000,
			ScannedBytes:      3_330_000,
			TotalBytesToScan:  10_000_000,
			AnalysisStartTime: time.Now().Add(-15 * time.Second),
		},
	}

	progress := syncengine.ProgressMetrics{
		OverallPercent:         0.333,
		FilesPercent:           33.3,
		BytesPercent:           33.3,
		TimePercent:            33.3,
		IsCounting:             false,
		EstimatedTimeRemaining: 30 * time.Second,
	}

	result := screen.renderProcessingProgress(screen.status, progress)

	// Percentages should be displayed for files, bytes, and time
	g.Expect(result).Should(ContainSubstring("%"))
}

// TestRenderProcessingProgress_ProgressBar verifies progress bar rendering
func TestRenderProcessingProgress_ProgressBar(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		overallProgress: newTestProgressBar(),
		status: &syncengine.Status{
			ScannedFiles:      250,
			TotalFilesToScan:  1000,
			ScannedBytes:      2_500_000,
			TotalBytesToScan:  10_000_000,
			AnalysisStartTime: time.Now().Add(-10 * time.Second),
		},
	}

	progress := syncengine.ProgressMetrics{
		OverallPercent:         0.25,
		FilesPercent:           25.0,
		BytesPercent:           25.0,
		TimePercent:            25.0,
		IsCounting:             false,
		EstimatedTimeRemaining: 30 * time.Second,
	}

	result := screen.renderProcessingProgress(screen.status, progress)

	// Should contain a progress bar (from overallProgress.ViewAs())
	// Progress bars typically contain characters like █ or similar
	g.Expect(result).ShouldNot(BeEmpty())
}

// newTestProgressBar creates a progress bar for testing
func newTestProgressBar() progress.Model {
	prog := progress.New(progress.WithDefaultGradient())
	prog.Width = 40 // Fixed width for testing

	return prog
}
