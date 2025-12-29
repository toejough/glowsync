//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers
)

func TestApplyCompletion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &InputScreen{
		focusIndex: 0,
	}
	screen.sourceInput.SetValue("")

	// Apply completion to source
	updated := screen.applyCompletion("/path/to/file.txt")
	g.Expect(updated.sourceInput.Value()).Should(Equal("/path/to/file.txt"))

	// Apply completion to dest
	screen.focusIndex = 1
	screen.destInput.SetValue("")
	updated = screen.applyCompletion("/dest/path")
	g.Expect(updated.destInput.Value()).Should(Equal("/dest/path"))
}

func TestCalculateCompletionWindow(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &InputScreen{}

	// Test window calculation
	start, end := screen.calculateCompletionWindow(5, 8, 20)
	g.Expect(start).Should(BeNumerically(">=", 0))
	g.Expect(end).Should(BeNumerically("<=", 20))
	g.Expect(end - start).Should(BeNumerically("<=", 8))
}

func TestExpandHomePath(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test empty path
	result := expandHomePath("")
	g.Expect(result).Should(Equal("."))

	// Test tilde expansion
	result = expandHomePath("~/test")
	g.Expect(result).ShouldNot(Equal("~/test")) // Should expand

	// Test regular path
	result = expandHomePath("/regular/path")
	g.Expect(result).Should(Equal("/regular/path"))
}

func TestFormatAllCompletions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &InputScreen{}
	completions := []string{"/path/file1.txt", "/path/file2.txt"}
	result := screen.formatAllCompletions(completions, 0)
	g.Expect(len(result)).Should(BeNumerically(">", len(completions))) // Includes separator
}

func TestFormatCompletionList(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &InputScreen{}

	// Test with no completions
	result := screen.formatCompletionList([]string{}, 0)
	g.Expect(result).Should(BeEmpty())

	// Test with single completion
	result = screen.formatCompletionList([]string{"/path/to/file.txt"}, 0)
	g.Expect(result).Should(ContainSubstring("file.txt"))

	// Test with multiple completions (less than max)
	completions := []string{"/path/file1.txt", "/path/file2.txt", "/path/file3.txt"}
	result = screen.formatCompletionList(completions, 0)
	g.Expect(result).ShouldNot(BeEmpty())

	// Test with many completions (more than max to trigger windowing)
	const numCompletions = 20

	manyCompletions := make([]string, numCompletions)

	for i := range manyCompletions {
		manyCompletions[i] = "/path/file" + string(rune('0'+i)) + ".txt"
	}

	result = screen.formatCompletionList(manyCompletions, 5)
	g.Expect(result).ShouldNot(BeEmpty())
}

func TestFormatSingleCompletion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &InputScreen{}
	result := screen.formatSingleCompletion("/path/to/file.txt")
	g.Expect(result).Should(HaveLen(1))
	g.Expect(result[0]).Should(ContainSubstring("file.txt"))
}

func TestFormatWindowedCompletions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &InputScreen{}

	const numCompletions = 20

	completions := make([]string, numCompletions)

	for i := range completions {
		completions[i] = "/path/file" + string(rune('0'+i)) + ".txt"
	}

	result := screen.formatWindowedCompletions(completions, 5, 8)
	g.Expect(result).ShouldNot(BeEmpty())
}

func TestGetBaseName(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test various paths
	g.Expect(getBaseName("/path/to/file.txt")).Should(Equal("file.txt"))
	g.Expect(getBaseName("/path/to/dir/")).Should(Equal("dir/"))
	g.Expect(getBaseName("file.txt")).Should(Equal("file.txt"))
	g.Expect(getBaseName("/")).Should(Equal("/"))
}

func TestGetPathCompletions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test with current directory
	completions := getPathCompletions(".")
	g.Expect(completions).ShouldNot(BeNil())
}

func TestParseCompletionPath(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test with trailing slash
	dir, prefix := parseCompletionPath("/path/to/")
	g.Expect(dir).Should(Equal("/path/to/"))
	g.Expect(prefix).Should(Equal(""))

	// Test without trailing slash
	dir, prefix = parseCompletionPath("/path/to/file")
	g.Expect(dir).Should(Equal("/path/to"))
	g.Expect(prefix).Should(Equal("file"))
}

func TestShouldIncludeEntry(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test hidden file filtering
	g.Expect(shouldIncludeEntry(".hidden", "")).Should(BeFalse())
	g.Expect(shouldIncludeEntry(".hidden", ".")).Should(BeTrue())

	// Test prefix matching
	g.Expect(shouldIncludeEntry("test.txt", "test")).Should(BeTrue())
	g.Expect(shouldIncludeEntry("other.txt", "test")).Should(BeFalse())

	// Test empty prefix
	g.Expect(shouldIncludeEntry("anything.txt", "")).Should(BeTrue())
}
