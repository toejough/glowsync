//nolint:varnamelen // Test files use idiomatic short variable names
package widgets_test

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/widgets"
)

func TestNewProgressWidget_ShowsFilesScannedAndTotal(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		TotalFiles:     100,
		ProcessedFiles: 42,
		TotalBytes:     1024 * 1024 * 100, // 100 MB
		TransferredBytes: 1024 * 1024 * 42,  // 42 MB
	}

	widget := widgets.NewProgressWidget(func() *syncengine.Status { return status })
	result := widget()

	g.Expect(result).Should(ContainSubstring("42"),
		"Progress widget should show files scanned count")
	g.Expect(result).Should(ContainSubstring("100"),
		"Progress widget should show total files count")
}

func TestNewProgressWidget_ShowsBytesInHumanReadableFormat(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		TotalFiles:       100,
		ProcessedFiles:   50,
		TotalBytes:       1024 * 1024 * 1024, // 1 GB
		TransferredBytes: 1024 * 1024 * 512,  // 512 MB
	}

	widget := widgets.NewProgressWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should show bytes in MB or GB format, not raw bytes
	g.Expect(result).Should(Or(
		ContainSubstring("MB"),
		ContainSubstring("GB"),
	), "Progress widget should show bytes in human-readable format (MB/GB)")
}

func TestNewProgressWidget_ShowsPercentage(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		TotalFiles:       100,
		ProcessedFiles:   50,
		TotalBytes:       1000,
		TransferredBytes: 500,
	}

	widget := widgets.NewProgressWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should show percentage (50%)
	g.Expect(result).Should(Or(
		ContainSubstring("50"),
		ContainSubstring("%"),
	), "Progress widget should show percentage progress")
}

func TestNewProgressWidget_HandlesZeroTotals_NoNaN(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		TotalFiles:       0,
		ProcessedFiles:   0,
		TotalBytes:       0,
		TransferredBytes: 0,
	}

	widget := widgets.NewProgressWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should not contain NaN or Inf
	g.Expect(result).ShouldNot(ContainSubstring("NaN"),
		"Progress widget should not show NaN for 0/0 division")
	g.Expect(result).ShouldNot(ContainSubstring("Inf"),
		"Progress widget should not show Infinity for 0/0 division")

	// Should show 0% or similar
	g.Expect(result).ShouldNot(BeEmpty(),
		"Progress widget should return non-empty string even with zero totals")
}

func TestNewProgressWidget_HandlesNilStatus_ReturnsDefault(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	widget := widgets.NewProgressWidget(func() *syncengine.Status { return nil })
	result := widget()

	// Should show default/placeholder when status is nil
	g.Expect(result).Should(Or(
		ContainSubstring("Progress"),
		ContainSubstring("0"),
	), "Progress widget should show default message when status is nil")
	g.Expect(result).ShouldNot(BeEmpty(),
		"Progress widget should return non-empty string even when status is nil")
}

func TestNewProgressWidget_ReturnsFormattedString(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		TotalFiles:       200,
		ProcessedFiles:   100,
		TotalBytes:       2000,
		TransferredBytes: 1000,
	}

	widget := widgets.NewProgressWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should contain some structured format (numbers/separators)
	g.Expect(result).ShouldNot(BeEmpty(),
		"Progress widget should return non-empty formatted string")
}
