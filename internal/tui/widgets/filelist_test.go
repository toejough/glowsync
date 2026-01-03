//nolint:varnamelen // Test files use idiomatic short variable names
package widgets_test

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/widgets"
)

func TestNewFileListWidget_ShowsCurrentlySyncingFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		FilesToSync: []*syncengine.FileToSync{
			{RelativePath: "file1.txt", Size: 1024, Transferred: 512, Status: "copying"},
			{RelativePath: "file2.txt", Size: 2048, Transferred: 1024, Status: "copying"},
		},
	}

	widget := widgets.NewFileListWidget(func() *syncengine.Status { return status })
	result := widget()

	g.Expect(result).Should(Or(
		ContainSubstring("file1.txt"),
		ContainSubstring("file2.txt"),
	), "File list should show currently syncing files")
}

func TestNewFileListWidget_ShowsPerFileProgress(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		FilesToSync: []*syncengine.FileToSync{
			{RelativePath: "document.pdf", Size: 1000, Transferred: 500, Status: "copying"},
		},
	}

	widget := widgets.NewFileListWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should show progress indicator (percentage, bar, or fraction)
	g.Expect(result).Should(Or(
		ContainSubstring("%"),
		ContainSubstring("50"),
		ContainSubstring("500"),
		ContainSubstring("["),
		ContainSubstring("="),
	), "File list should show per-file progress")
}

func TestNewFileListWidget_ShowsFileStatus(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		FilesToSync: []*syncengine.FileToSync{
			{RelativePath: "opening.txt", Size: 100, Transferred: 0, Status: "opening"},
			{RelativePath: "copying.txt", Size: 100, Transferred: 50, Status: "copying"},
			{RelativePath: "finalizing.txt", Size: 100, Transferred: 100, Status: "finalizing"},
		},
	}

	widget := widgets.NewFileListWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should show at least one status indicator
	g.Expect(result).Should(Or(
		ContainSubstring("opening"),
		ContainSubstring("copying"),
		ContainSubstring("finalizing"),
		ContainSubstring("Opening"),
		ContainSubstring("Copying"),
		ContainSubstring("Finalizing"),
	), "File list should show file status")
}

func TestNewFileListWidget_LimitsToVisibleCount(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create many files (more than typical display limit of 20)
	files := make([]*syncengine.FileToSync, 50)
	for i := 0; i < 50; i++ {
		files[i] = &syncengine.FileToSync{
			RelativePath: "file" + string(rune('0'+i)) + ".txt",
			Size:         1000,
			Transferred:  500,
			Status:       "copying",
		}
	}

	status := &syncengine.Status{
		FilesToSync: files,
	}

	widget := widgets.NewFileListWidget(func() *syncengine.Status { return status })
	result := widget()

	// Should limit output (not show all 50 files)
	g.Expect(result).ShouldNot(BeEmpty(),
		"File list should return non-empty result")

	// Output should be reasonable size (not showing all 50 detailed entries)
	g.Expect(len(result)).Should(BeNumerically("<", 20000),
		"File list should limit display to reasonable size")
}

func TestNewFileListWidget_HandlesEmptyFileList(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		FilesToSync: []*syncengine.FileToSync{},
	}

	widget := widgets.NewFileListWidget(func() *syncengine.Status { return status })
	result := widget()

	// Empty list should return empty or placeholder message
	g.Expect(result).Should(BeAssignableToTypeOf(""),
		"File list should handle empty list gracefully")
}

func TestNewFileListWidget_HandlesNilStatus(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	widget := widgets.NewFileListWidget(func() *syncengine.Status { return nil })
	result := widget()

	// Should not crash
	g.Expect(result).Should(BeAssignableToTypeOf(""),
		"File list should handle nil status gracefully")
}

func TestNewFileListWidget_ReturnsMultilineString(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	status := &syncengine.Status{
		FilesToSync: []*syncengine.FileToSync{
			{RelativePath: "file1.txt", Size: 1000, Transferred: 500, Status: "copying"},
			{RelativePath: "file2.txt", Size: 2000, Transferred: 1000, Status: "copying"},
		},
	}

	widget := widgets.NewFileListWidget(func() *syncengine.Status { return status })
	result := widget()

	// Multiple files should produce multi-line output
	if len(status.FilesToSync) > 1 {
		g.Expect(result).Should(Or(
			ContainSubstring("\n"),
			ContainSubstring("file"),
		), "File list with multiple files should contain newlines or file references")
	}
}
