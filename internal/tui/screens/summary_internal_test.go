//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
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

func TestTruncatePathSummary(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SummaryScreen{}

	// Test short path (no truncation)
	result := screen.truncatePath("/short/path.txt", 100)
	g.Expect(result).Should(Equal("/short/path.txt"))

	// Test long path (truncation)
	longPath := "/very/long/path/to/some/file/that/needs/truncation/file.txt"
	result = screen.truncatePath(longPath, 20)
	g.Expect(result).Should(ContainSubstring("..."))
	g.Expect(len(result)).Should(BeNumerically("<=", 20))
}
