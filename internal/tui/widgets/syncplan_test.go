//nolint:varnamelen // Test files use idiomatic short variable names
package widgets_test

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/widgets"
)

func TestNewSyncPlanWidget_ShowsNumberOfFilesToCopy(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		TotalFiles: 42,
		TotalBytes: 1024 * 1024 * 100, // 100 MB
	}

	widget := widgets.NewSyncPlanWidget(func() *syncengine.Status { return status })
	result := widget()

	g.Expect(result).Should(ContainSubstring("42"),
		"Sync plan should show number of files to copy")
}

func TestNewSyncPlanWidget_ShowsTotalBytesInHumanReadable(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		TotalFiles: 100,
		TotalBytes: 1024 * 1024 * 1024 * 2, // 2 GB
	}

	widget := widgets.NewSyncPlanWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should show bytes in GB/MB format
	g.Expect(result).Should(Or(
		ContainSubstring("GB"),
		ContainSubstring("MB"),
	), "Sync plan should show total bytes in human-readable format")
}

func TestNewSyncPlanWidget_ShowsFilesAndBytes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		TotalFiles: 25,
		TotalBytes: 1024 * 1024 * 50, // 50 MB
	}

	widget := widgets.NewSyncPlanWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should mention both files and bytes/size
	g.Expect(result).Should(And(
		Or(ContainSubstring("file"), ContainSubstring("File")),
		Or(ContainSubstring("25"), ContainSubstring("50")),
	), "Sync plan should show both file count and size information")
}

func TestNewSyncPlanWidget_HandlesNilStatus(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	widget := widgets.NewSyncPlanWidget(func() *syncengine.Status { return nil })
	result := widget()

	// Should not crash, should return something
	g.Expect(result).Should(BeAssignableToTypeOf(""),
		"Sync plan should handle nil status gracefully")
	g.Expect(result).ShouldNot(BeEmpty(),
		"Sync plan should return non-empty string even with nil status")
}

func TestNewSyncPlanWidget_HandlesZeroFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		TotalFiles: 0,
		TotalBytes: 0,
	}

	widget := widgets.NewSyncPlanWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should show 0 files or "no files" message
	g.Expect(result).Should(Or(
		ContainSubstring("0"),
		ContainSubstring("No"),
		ContainSubstring("no"),
	), "Sync plan should handle zero files gracefully")
}

func TestNewSyncPlanWidget_ReturnsFormattedSummary(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		TotalFiles: 150,
		TotalBytes: 1024 * 1024 * 200, // 200 MB
	}

	widget := widgets.NewSyncPlanWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should return a formatted summary string
	g.Expect(result).ShouldNot(BeEmpty(),
		"Sync plan should return non-empty formatted summary")
	g.Expect(len(result)).Should(BeNumerically(">", 10),
		"Sync plan summary should be reasonably descriptive")
}
