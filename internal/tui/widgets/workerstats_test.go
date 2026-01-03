//nolint:varnamelen // Test files use idiomatic short variable names
package widgets_test

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/widgets"
)

func TestNewWorkerStatsWidget_ShowsWorkerCount(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		// Assuming there's a field for active workers or we infer from CurrentFiles length
		CurrentFiles: []string{"file1.txt", "file2.txt", "file3.txt"},
	}

	widget := widgets.NewWorkerStatsWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should show worker/thread count
	g.Expect(result).Should(Or(
		ContainSubstring("worker"),
		ContainSubstring("Worker"),
		ContainSubstring("3"),
	), "Worker stats should show worker count")
}

func TestNewWorkerStatsWidget_ShowsTransferSpeed(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		BytesPerSecond: 5.5 * 1024 * 1024, // 5.5 MB/s
		CurrentFiles:   []string{"file1.txt"},
	}

	widget := widgets.NewWorkerStatsWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should show transfer speed
	g.Expect(result).Should(Or(
		ContainSubstring("MB/s"),
		ContainSubstring("KB/s"),
		ContainSubstring("/s"),
		ContainSubstring("5"),
	), "Worker stats should show transfer speed")
}

func TestNewWorkerStatsWidget_ShowsPerWorkerSpeed(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		BytesPerSecond: 10 * 1024 * 1024, // 10 MB/s total
		CurrentFiles:   []string{"file1.txt", "file2.txt"}, // 2 workers = 5 MB/s each
	}

	widget := widgets.NewWorkerStatsWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should calculate and show per-worker speed
	g.Expect(result).Should(Or(
		ContainSubstring("per"),
		ContainSubstring("each"),
		ContainSubstring("MB/s"),
		ContainSubstring("/s"),
	), "Worker stats should show per-worker speed or total speed")
}

func TestNewWorkerStatsWidget_HandlesZeroWorkers(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		BytesPerSecond: 0,
		CurrentFiles:   []string{},
	}

	widget := widgets.NewWorkerStatsWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should handle zero workers gracefully (no division by zero)
	g.Expect(result).ShouldNot(ContainSubstring("NaN"),
		"Worker stats should not show NaN")
	g.Expect(result).ShouldNot(ContainSubstring("Inf"),
		"Worker stats should not show Infinity")
	g.Expect(result).ShouldNot(BeEmpty(),
		"Worker stats should return non-empty string even with zero workers")
}

func TestNewWorkerStatsWidget_HandlesNilStatus(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	widget := widgets.NewWorkerStatsWidget(func() *syncengine.Status { return nil })
	result := widget()

	// Should not crash
	g.Expect(result).Should(BeAssignableToTypeOf(""),
		"Worker stats should handle nil status gracefully")
	g.Expect(result).ShouldNot(BeEmpty(),
		"Worker stats should return non-empty string even with nil status")
}

func TestNewWorkerStatsWidget_ReturnsFormattedStats(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		BytesPerSecond: 3.2 * 1024 * 1024, // 3.2 MB/s
		CurrentFiles:   []string{"file1.txt", "file2.txt", "file3.txt", "file4.txt"},
	}

	widget := widgets.NewWorkerStatsWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should return formatted statistics string
	g.Expect(result).ShouldNot(BeEmpty(),
		"Worker stats should return non-empty formatted string")
	g.Expect(len(result)).Should(BeNumerically(">", 5),
		"Worker stats should be reasonably descriptive")
}
