//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers
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
