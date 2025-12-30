//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens

import (
	"errors"
	"fmt"
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

func TestGetMaxPathWidthSummary(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SummaryScreen{
		width: 100,
	}

	width := screen.getMaxPathWidth()
	g.Expect(width).Should(BeNumerically(">", 0))
}

func TestRenderErrorView_WithEnrichedAdditionalErrors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	mainErr := errors.New("some error")
	diskErr := errors.New("no space left on device")

	screen := &SummaryScreen{
		finalState: "error",
		err:        mainErr,
		status: &syncengine.Status{
			Errors: []syncengine.FileError{
				{FilePath: "/path/to/file.txt", Error: diskErr},
			},
		},
	}

	result := screen.renderErrorView()

	// Should show disk space error
	g.Expect(result).Should(ContainSubstring("no space left"))

	// Should show disk space suggestions
	g.Expect(result).Should(ContainSubstring("df"))
}

func TestRenderErrorView_WithEnrichedMainError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	permissionErr := errors.New("permission denied: /path/to/file.txt")
	screen := &SummaryScreen{
		finalState: "error",
		err:        permissionErr,
	}

	result := screen.renderErrorView()

	// Should show the error
	g.Expect(result).Should(ContainSubstring("permission denied"))

	// Should show actionable suggestions
	g.Expect(result).Should(ContainSubstring("ls -la"))
}

func TestSummaryScreen_CancelledState_ErrorDisplayLimit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create 10 errors
	fileErrors := make([]syncengine.FileError, 10)
	for i := range 10 {
		fileErrors[i] = syncengine.FileError{
			FilePath: fmt.Sprintf("/path/to/file%d.txt", i),
			Error:    fmt.Errorf("error %d", i),
		}
	}

	screen := &SummaryScreen{
		finalState: "cancelled",
		status: &syncengine.Status{
			Errors: fileErrors,
		},
	}

	result := screen.renderCancelledView()

	// Should show first 5 errors
	g.Expect(result).Should(ContainSubstring("error 0"))
	g.Expect(result).Should(ContainSubstring("error 4"))

	// Should NOT show 6th error and beyond in full
	g.Expect(result).ShouldNot(ContainSubstring("error 5"))
	g.Expect(result).ShouldNot(ContainSubstring("error 9"))

	// Should show truncation message
	g.Expect(result).Should(ContainSubstring("... and 5 more error(s)"))
}

func TestSummaryScreen_CompleteState_ErrorDisplayLimit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create 15 errors
	fileErrors := make([]syncengine.FileError, 15)
	for i := range 15 {
		fileErrors[i] = syncengine.FileError{
			FilePath: fmt.Sprintf("/path/to/file%d.txt", i),
			Error:    fmt.Errorf("error %d", i),
		}
	}

	screen := &SummaryScreen{
		finalState: "complete",
		status: &syncengine.Status{
			Errors: fileErrors,
		},
	}

	result := screen.renderCompleteView()

	// Should show first 10 errors
	g.Expect(result).Should(ContainSubstring("error 0"))
	g.Expect(result).Should(ContainSubstring("error 9"))

	// Should NOT show 11th error and beyond in full
	g.Expect(result).ShouldNot(ContainSubstring("error 10"))
	g.Expect(result).ShouldNot(ContainSubstring("error 14"))

	// Should show truncation message
	g.Expect(result).Should(ContainSubstring("... and 5 more error(s)"))
}

func TestSummaryScreen_ErrorState_ErrorDisplayLimit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	mainErr := errors.New("main sync error")

	// Create 8 additional errors
	fileErrors := make([]syncengine.FileError, 8)
	for i := range 8 {
		fileErrors[i] = syncengine.FileError{
			FilePath: fmt.Sprintf("/path/to/file%d.txt", i),
			Error:    fmt.Errorf("error %d", i),
		}
	}

	screen := &SummaryScreen{
		finalState: "error",
		err:        mainErr,
		status: &syncengine.Status{
			Errors: fileErrors,
		},
	}

	result := screen.renderErrorView()

	// Should show main error
	g.Expect(result).Should(ContainSubstring("main sync error"))

	// Should show first 5 additional errors
	g.Expect(result).Should(ContainSubstring("error 0"))
	g.Expect(result).Should(ContainSubstring("error 4"))

	// Should NOT show 6th error and beyond in full
	g.Expect(result).ShouldNot(ContainSubstring("error 5"))
	g.Expect(result).ShouldNot(ContainSubstring("error 7"))

	// Should show truncation message for additional errors
	g.Expect(result).Should(ContainSubstring("... and 3 more error(s)"))
}

func TestTruncatePathSummary(t *testing.T) {
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
