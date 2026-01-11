//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// Note: TestCalculateMaxFilesToShow, TestGetBottleneckInfo removed - functions moved to AnalysisScreen

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

// Note: TestRenderCurrentlyCopying, TestRenderFileList, TestRenderRecentFiles, TestRenderStatistics
// removed - functions moved to AnalysisScreen

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

// Note: TestRenderSyncingView_CompleteIntegration, TestRenderSyncingView_UsesUnifiedProgress,
// TestRenderUnifiedProgress tests removed - progress rendering moved to AnalysisScreen

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
