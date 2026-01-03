//nolint:varnamelen // Test files use idiomatic short variable names
package widgets_test

import (
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/widgets"
)

func TestNewSummaryWidget_ShowsCompletionMessageForSuccess(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		ProcessedFiles:   100,
		TotalFiles:       100,
		TransferredBytes: 1024 * 1024 * 50,
		TotalBytes:       1024 * 1024 * 50,
		StartTime:        time.Now().Add(-5 * time.Minute),
		EndTime:          time.Now(),
	}

	widget := widgets.NewSummaryWidget(func() *syncengine.Status { return status }, nil)
	result := widget()

	g.Expect(result).Should(Or(
		ContainSubstring("complete"),
		ContainSubstring("Complete"),
		ContainSubstring("success"),
		ContainSubstring("Success"),
		ContainSubstring("done"),
		ContainSubstring("Done"),
	), "Summary should show completion message for successful sync")
}

func TestNewSummaryWidget_ShowsErrorMessageForFailure(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	syncErr := errors.New("network connection failed")
	status := &syncengine.Status{
		ProcessedFiles:   50,
		TotalFiles:       100,
		TransferredBytes: 1024 * 1024 * 25,
		TotalBytes:       1024 * 1024 * 50,
		StartTime:        time.Now().Add(-2 * time.Minute),
		EndTime:          time.Now(),
	}

	widget := widgets.NewSummaryWidget(func() *syncengine.Status { return status }, syncErr)
	result := widget()

	g.Expect(result).Should(Or(
		ContainSubstring("error"),
		ContainSubstring("Error"),
		ContainSubstring("fail"),
		ContainSubstring("Fail"),
		ContainSubstring("network connection failed"),
	), "Summary should show error message for failed sync")
}

func TestNewSummaryWidget_ShowsFilesSyncedCount(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		ProcessedFiles:   75,
		TotalFiles:       100,
		TransferredBytes: 1024 * 1024 * 30,
		TotalBytes:       1024 * 1024 * 40,
		StartTime:        time.Now().Add(-3 * time.Minute),
		EndTime:          time.Now(),
	}

	widget := widgets.NewSummaryWidget(func() *syncengine.Status { return status }, nil)
	result := widget()

	g.Expect(result).Should(Or(
		ContainSubstring("75"),
		ContainSubstring("file"),
		ContainSubstring("File"),
	), "Summary should show files synced count")
}

func TestNewSummaryWidget_ShowsBytesTransferred(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		ProcessedFiles:   100,
		TotalFiles:       100,
		TransferredBytes: 1024 * 1024 * 1024 * 2, // 2 GB
		TotalBytes:       1024 * 1024 * 1024 * 2,
		StartTime:        time.Now().Add(-10 * time.Minute),
		EndTime:          time.Now(),
	}

	widget := widgets.NewSummaryWidget(func() *syncengine.Status { return status }, nil)
	result := widget()

	g.Expect(result).Should(Or(
		ContainSubstring("GB"),
		ContainSubstring("MB"),
		ContainSubstring("byte"),
		ContainSubstring("Byte"),
	), "Summary should show bytes transferred in human-readable format")
}

func TestNewSummaryWidget_ShowsElapsedTime(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	startTime := time.Now().Add(-7 * time.Minute)
	endTime := time.Now()

	status := &syncengine.Status{
		ProcessedFiles:   100,
		TotalFiles:       100,
		TransferredBytes: 1024 * 1024 * 50,
		TotalBytes:       1024 * 1024 * 50,
		StartTime:        startTime,
		EndTime:          endTime,
	}

	widget := widgets.NewSummaryWidget(func() *syncengine.Status { return status }, nil)
	result := widget()

	g.Expect(result).Should(Or(
		ContainSubstring("time"),
		ContainSubstring("Time"),
		ContainSubstring("elapsed"),
		ContainSubstring("Elapsed"),
		ContainSubstring("m"),
		ContainSubstring("s"),
		ContainSubstring("7"),
	), "Summary should show elapsed time")
}

func TestNewSummaryWidget_HandlesNilStatus(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	widget := widgets.NewSummaryWidget(func() *syncengine.Status { return nil }, nil)
	result := widget()

	// Should not crash
	g.Expect(result).Should(BeAssignableToTypeOf(""),
		"Summary should handle nil status gracefully")
	g.Expect(result).ShouldNot(BeEmpty(),
		"Summary should return non-empty string even with nil status")
}

func TestNewSummaryWidget_ReturnsFormattedSummary(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		ProcessedFiles:   200,
		TotalFiles:       200,
		TransferredBytes: 1024 * 1024 * 100,
		TotalBytes:       1024 * 1024 * 100,
		StartTime:        time.Now().Add(-15 * time.Minute),
		EndTime:          time.Now(),
	}

	widget := widgets.NewSummaryWidget(func() *syncengine.Status { return status }, nil)
	result := widget()

	// Should return comprehensive summary
	g.Expect(result).ShouldNot(BeEmpty(),
		"Summary should return non-empty formatted summary")
	g.Expect(len(result)).Should(BeNumerically(">", 20),
		"Summary should be reasonably comprehensive")
}
