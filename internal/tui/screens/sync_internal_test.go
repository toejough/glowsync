//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

func TestCalculateMaxFilesToShow(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		height: 50,
	}

	maxFiles := screen.calculateMaxFilesToShow()
	g.Expect(maxFiles).Should(BeNumerically(">=", 1))
}

func TestCalculateMaxFilesToShow_WithUnifiedProgress(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// With unified progress, we reclaim ~7 lines (4 from overall + 4 from session - 1 for unified)
	// Old: 2 (title) + 4 (overall) + 4 (session) + 8 (stats) + 2 (header) + 5 (errors) + 2 (padding) = 27 lines
	// New: 2 (title) + 5 (unified) + 8 (stats) + 2 (header) + 5 (errors) + 2 (padding) = 24 lines
	// Reclaimed: 3 lines (1 line = 1/3 of a file entry, so ~1 more file can be shown)

	oldScreen := &SyncScreen{height: 50}
	// Simulate old calculation
	oldLinesUsed := 2 + 4 + 4 + 8 + 2 + 5 + 2 // 27 lines
	oldAvailable := oldScreen.height - oldLinesUsed
	oldMaxFiles := oldAvailable / 3 // 23 / 3 = 7 files

	newScreen := &SyncScreen{height: 50}
	newMaxFiles := newScreen.calculateMaxFilesToShow()

	// New implementation should allow more files to be shown
	g.Expect(newMaxFiles).Should(BeNumerically(">", oldMaxFiles))
}

func TestGetBottleneckInfo(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			AdaptiveMode: true,
		},
	}

	// Test source bottleneck
	screen.status.Bottleneck = "source"
	result := screen.getBottleneckInfo()
	g.Expect(result).Should(ContainSubstring("source slow"))

	// Test destination bottleneck
	screen.status.Bottleneck = "destination"
	result = screen.getBottleneckInfo()
	g.Expect(result).Should(ContainSubstring("dest slow"))

	// Test balanced (optimal)
	screen.status.Bottleneck = "balanced"
	result = screen.getBottleneckInfo()
	g.Expect(result).Should(ContainSubstring("optimal"))

	// Test no adaptive mode
	screen.status.AdaptiveMode = false
	result = screen.getBottleneckInfo()
	g.Expect(result).Should(BeEmpty())
}

func TestGetMaxPathWidth(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		width: 100,
	}

	width := screen.getMaxPathWidth()
	g.Expect(width).Should(BeNumerically(">", 0))
}

func TestNewSyncScreen_InitializesProgressBars(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := NewSyncScreen(nil)

	// Should initialize overallProgress for unified bar
	g.Expect(screen.overallProgress).ShouldNot(BeNil())

	// Should initialize fileProgress for per-file progress
	g.Expect(screen.fileProgress).ShouldNot(BeNil())
}

func TestRenderCurrentlyCopying(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			CurrentFiles: []string{"file1.txt"},
			FilesToSync: []*syncengine.FileToSync{
				{RelativePath: "file1.txt", Status: "copying", Size: 1024, Transferred: 512},
			},
		},
	}

	var builder strings.Builder
	screen.renderCurrentlyCopying(&builder, 5)
	result := builder.String()
	g.Expect(result).Should(ContainSubstring("Currently Copying"))
}

func TestRenderFileList(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			CurrentFiles: []string{"file1.txt", "file2.txt"},
			FilesToSync: []*syncengine.FileToSync{
				{RelativePath: "file1.txt", Status: "copying", Size: 1024, Transferred: 512},
			},
		},
		height: 50,
	}

	var builder strings.Builder
	screen.renderFileList(&builder)
	result := builder.String()
	g.Expect(result).ShouldNot(BeEmpty())
}

func TestRenderRecentFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			FilesToSync: []*syncengine.FileToSync{
				{RelativePath: "file1.txt", Status: "complete"},
				{RelativePath: "file2.txt", Status: "complete"},
			},
		},
	}

	var builder strings.Builder
	screen.renderRecentFiles(&builder, 5)
	result := builder.String()
	g.Expect(result).Should(ContainSubstring("Recent Files"))
}

func TestRenderRecentFiles_Empty(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			FilesToSync: []*syncengine.FileToSync{},
		},
	}

	var builder strings.Builder
	screen.renderRecentFiles(&builder, 5)
	result := builder.String()

	// Should NOT show "Recent Files:" header when list is empty
	g.Expect(result).ShouldNot(ContainSubstring("Recent Files"))
}

func TestRenderStatistics(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			StartTime:        time.Now().Add(-10 * time.Second),
			TransferredBytes: 1024 * 1024,
			TotalBytes:       2 * 1024 * 1024,
			ProcessedFiles:   50,
			TotalFiles:       100,
			ActiveWorkers:    4,
		},
	}

	var builder strings.Builder
	screen.renderStatistics(&builder)
	result := builder.String()
	g.Expect(result).Should(ContainSubstring("Workers"))
	g.Expect(result).Should(ContainSubstring("Elapsed"))
}

func TestRenderSyncingErrors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	testErr := errors.New("test error")

	screen := &SyncScreen{
		status: &syncengine.Status{
			Errors: []syncengine.FileError{
				{FilePath: "/path/to/file1.txt", Error: testErr},
			},
		},
		width: 100,
	}

	var builder strings.Builder
	screen.renderSyncingErrors(&builder)
	result := builder.String()
	g.Expect(result).Should(ContainSubstring("Errors"))

	// Test with no errors
	screen.status.Errors = []syncengine.FileError{}

	builder.Reset()
	screen.renderSyncingErrors(&builder)
	result = builder.String()
	g.Expect(result).Should(BeEmpty())
}

func TestRenderSyncingErrors_AtLimit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create exactly 3 errors (at the limit)
	fileErrors := []syncengine.FileError{
		{FilePath: "/path/to/file1.txt", Error: errors.New("error 1")},
		{FilePath: "/path/to/file2.txt", Error: errors.New("error 2")},
		{FilePath: "/path/to/file3.txt", Error: errors.New("error 3")},
	}

	screen := &SyncScreen{
		status: &syncengine.Status{
			Errors: fileErrors,
		},
		width: 100,
	}

	var builder strings.Builder
	screen.renderSyncingErrors(&builder)
	result := builder.String()

	// Should show all 3 errors
	errorCount := strings.Count(result, "✗")
	g.Expect(errorCount).Should(Equal(3))

	// Should NOT show overflow message when exactly at limit
	g.Expect(result).ShouldNot(ContainSubstring("... and"))
	g.Expect(result).ShouldNot(ContainSubstring("more (see summary)"))
}

func TestRenderSyncingErrors_ErrorLimit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create 5 errors to test the limit of 3
	fileErrors := make([]syncengine.FileError, 5)
	for i := range 5 {
		fileErrors[i] = syncengine.FileError{
			FilePath: fmt.Sprintf("/path/to/file%d.txt", i+1),
			Error:    fmt.Errorf("error %d", i+1),
		}
	}

	screen := &SyncScreen{
		status: &syncengine.Status{
			Errors: fileErrors,
		},
		width: 100,
	}

	var builder strings.Builder
	screen.renderSyncingErrors(&builder)
	result := builder.String()

	// Should show first 3 errors only (using error symbol count)
	errorCount := strings.Count(result, "✗")
	g.Expect(errorCount).Should(Equal(3), "Should display exactly 3 errors")

	// Should show overflow message
	g.Expect(result).Should(ContainSubstring("... and 2 more (see summary)"))

	// Should NOT show the old completion screen message
	g.Expect(result).ShouldNot(ContainSubstring("completion screen"))
}

func TestRenderSyncingErrors_UnderLimit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create only 2 errors (under the limit of 3)
	fileErrors := []syncengine.FileError{
		{FilePath: "/path/to/file1.txt", Error: errors.New("error 1")},
		{FilePath: "/path/to/file2.txt", Error: errors.New("error 2")},
	}

	screen := &SyncScreen{
		status: &syncengine.Status{
			Errors: fileErrors,
		},
		width: 100,
	}

	var builder strings.Builder
	screen.renderSyncingErrors(&builder)
	result := builder.String()

	// Should show all 2 errors
	errorCount := strings.Count(result, "✗")
	g.Expect(errorCount).Should(Equal(2))

	// Should NOT show overflow message
	g.Expect(result).ShouldNot(ContainSubstring("... and"))
	g.Expect(result).ShouldNot(ContainSubstring("more (see summary)"))
}

func TestRenderSyncingErrors_WithEnrichedErrors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	permissionErr := errors.New("permission denied: /path/to/file1.txt")

	screen := &SyncScreen{
		status: &syncengine.Status{
			Errors: []syncengine.FileError{
				{FilePath: "/path/to/file1.txt", Error: permissionErr},
			},
		},
		width: 100,
	}

	var builder strings.Builder
	screen.renderSyncingErrors(&builder)
	result := builder.String()

	// Should show error message
	g.Expect(result).Should(ContainSubstring("Errors"))
	g.Expect(result).Should(ContainSubstring("permission denied"))

	// Should show actionable suggestions
	g.Expect(result).Should(ContainSubstring("ls -la"))

	// Should show category or other enrichment
	// (The exact format depends on implementation, but we expect more than just the raw error)
}

func TestRenderSyncingErrors_WithMultipleEnrichedErrors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	permissionErr := errors.New("permission denied")
	diskErr := errors.New("no space left on device")

	screen := &SyncScreen{
		status: &syncengine.Status{
			Errors: []syncengine.FileError{
				{FilePath: "/path/to/file1.txt", Error: permissionErr},
				{FilePath: "/path/to/file2.txt", Error: diskErr},
			},
		},
		width: 100,
	}

	var builder strings.Builder
	screen.renderSyncingErrors(&builder)
	result := builder.String()

	// Should show both errors
	g.Expect(result).Should(ContainSubstring("permission denied"))
	g.Expect(result).Should(ContainSubstring("no space left"))

	// Should show suggestions for both error types
	g.Expect(result).Should(ContainSubstring("ls -la")) // permission suggestion
	g.Expect(result).Should(ContainSubstring("df"))     // disk space suggestion
}

func TestRenderSyncingView_CompleteIntegration(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := NewSyncScreen(nil)
	screen.status = &syncengine.Status{
		TotalFilesInSource: 100,
		AlreadySyncedFiles: 30,
		ProcessedFiles:     20,
		TotalBytesInSource: 10 * 1024 * 1024,
		AlreadySyncedBytes: 3 * 1024 * 1024,
		TransferredBytes:   2 * 1024 * 1024,
		TotalBytes:         7 * 1024 * 1024,
		FailedFiles:        2,
		StartTime:          time.Now().Add(-10 * time.Second),
		ActiveWorkers:      4,
		FilesToSync: []*syncengine.FileToSync{
			{RelativePath: "file1.txt", Status: "complete"},
			{RelativePath: "file2.txt", Status: "complete"},
		},
	}
	screen.height = 50
	screen.width = 100

	result := screen.View()

	// Verify unified progress bar is present
	g.Expect(result).Should(ContainSubstring("Progress:"))
	g.Expect(result).Should(ContainSubstring("Files: 50 / 100"))
	g.Expect(result).Should(ContainSubstring("2 failed"))
	g.Expect(result).Should(ContainSubstring("Bytes: 5.0 MB / 10.0 MB"))
	g.Expect(result).Should(ContainSubstring("Time:"))

	// Verify statistics are present
	g.Expect(result).Should(ContainSubstring("Workers:"))
	g.Expect(result).Should(ContainSubstring("Elapsed:"))

	// Verify file list is present
	g.Expect(result).Should(ContainSubstring("Recent Files:"))

	// Verify no old progress bars
	g.Expect(result).ShouldNot(ContainSubstring("Overall Progress (All Files)"))
	g.Expect(result).ShouldNot(ContainSubstring("This Session:"))

	// Verify help text
	g.Expect(result).Should(ContainSubstring("Press Esc or q to cancel"))
}

func TestRenderSyncingView_UsesUnifiedProgress(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			TotalFilesInSource: 100,
			AlreadySyncedFiles: 30,
			ProcessedFiles:     20,
			TotalBytesInSource: 10 * 1024 * 1024,
			AlreadySyncedBytes: 3 * 1024 * 1024,
			TransferredBytes:   2 * 1024 * 1024,
			TotalBytes:         7 * 1024 * 1024,
			FailedFiles:        2,
			StartTime:          time.Now().Add(-10 * time.Second),
			ActiveWorkers:      4,
		},
		height: 50,
	}

	result := screen.renderSyncingView()

	// Should use unified progress (single "Progress:" label)
	g.Expect(result).Should(ContainSubstring("Progress:"))

	// Should NOT have separate "Overall Progress" and "This Session" sections
	g.Expect(result).ShouldNot(ContainSubstring("Overall Progress (All Files)"))
	g.Expect(result).ShouldNot(ContainSubstring("This Session:"))

	// Should show file count with new format
	g.Expect(result).Should(ContainSubstring("Files: 50 / 100"))
}

func TestRenderUnifiedProgress(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			TotalFilesInSource: 100,
			AlreadySyncedFiles: 30,
			ProcessedFiles:     20,
			TotalBytesInSource: 10 * 1024 * 1024, // 10 MB
			AlreadySyncedBytes: 3 * 1024 * 1024,  // 3 MB
			TransferredBytes:   2 * 1024 * 1024,  // 2 MB
			TotalBytes:         7 * 1024 * 1024,  // 7 MB (files to sync this session)
			FailedFiles:        2,
			StartTime:          time.Now().Add(-10 * time.Second),
			ActiveWorkers:      4,
		},
	}

	var builder strings.Builder
	screen.renderUnifiedProgress(&builder)
	result := builder.String()

	// Should show file progress prominently
	g.Expect(result).Should(ContainSubstring("50 / 100")) // Total processed files
	g.Expect(result).Should(ContainSubstring("Files:"))   // New format prefix

	// Should show bytes line
	g.Expect(result).Should(ContainSubstring("MB"))
	g.Expect(result).Should(ContainSubstring("Bytes:")) // New format prefix

	// Should show time line
	g.Expect(result).Should(ContainSubstring("Time:")) // New format prefix

	// Should show failed files count
	g.Expect(result).Should(ContainSubstring("2 failed"))
}

func TestRenderUnifiedProgress_NoFailedFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			TotalFilesInSource: 50,
			AlreadySyncedFiles: 20,
			ProcessedFiles:     10,
			TotalBytesInSource: 5 * 1024 * 1024,
			AlreadySyncedBytes: 2 * 1024 * 1024,
			TransferredBytes:   1 * 1024 * 1024,
			FailedFiles:        0,
			StartTime:          time.Now().Add(-5 * time.Second),
		},
	}

	var builder strings.Builder
	screen.renderUnifiedProgress(&builder)
	result := builder.String()

	// Should not show failed files when there are none
	g.Expect(result).ShouldNot(ContainSubstring("failed"))
}

func TestRenderUnifiedProgress_ZeroTotalFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			TotalFilesInSource: 0,
			ProcessedFiles:     0,
		},
	}

	var builder strings.Builder
	screen.renderUnifiedProgress(&builder)
	result := builder.String()

	// Should not crash with zero division
	g.Expect(result).ShouldNot(BeEmpty())
	g.Expect(result).Should(ContainSubstring("0 / 0"))
}

func TestTruncatePath(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test short path (no truncation)
	result := shared.TruncatePath("/short/path.txt", 100)
	g.Expect(result).Should(Equal("/short/path.txt"))

	// Test long path (truncation)
	longPath := "/very/long/path/to/some/file/that/needs/truncation/file.txt"
	result = shared.TruncatePath(longPath, 20)
	g.Expect(result).Should(ContainSubstring("..."))
	g.Expect(len(result)).Should(BeNumerically("<=", 20))
}
