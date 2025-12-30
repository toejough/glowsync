package shared_test

import (
	"errors"
	"strings"
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

func TestRenderErrorList_Empty(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	config := shared.ErrorListConfig{
		Errors:  []syncengine.FileError{},
		Context: shared.ContextInProgress,
	}

	result := shared.RenderErrorList(config)
	g.Expect(result).Should(BeEmpty())
}

func TestRenderErrorList_InProgressContext_UnderLimit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fileErrors := []syncengine.FileError{
		{FilePath: "/path/to/file1.txt", Error: errors.New("permission denied")},
		{FilePath: "/path/to/file2.txt", Error: errors.New("file not found")},
	}

	config := shared.ErrorListConfig{
		Errors:  fileErrors,
		Context: shared.ContextInProgress,
	}

	result := shared.RenderErrorList(config)

	// Should contain both errors
	g.Expect(result).Should(ContainSubstring("file1.txt"))
	g.Expect(result).Should(ContainSubstring("file2.txt"))
	g.Expect(result).Should(ContainSubstring("permission denied"))
	g.Expect(result).Should(ContainSubstring("file not found"))

	// Should NOT contain overflow message
	g.Expect(result).ShouldNot(ContainSubstring("... and"))
	g.Expect(result).ShouldNot(ContainSubstring("more (see summary)"))
	g.Expect(result).ShouldNot(ContainSubstring("more error(s)"))
}

func TestRenderErrorList_InProgressContext_AtLimit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fileErrors := []syncengine.FileError{
		{FilePath: "/path/to/file1.txt", Error: errors.New("error 1")},
		{FilePath: "/path/to/file2.txt", Error: errors.New("error 2")},
		{FilePath: "/path/to/file3.txt", Error: errors.New("error 3")},
	}

	config := shared.ErrorListConfig{
		Errors:  fileErrors,
		Context: shared.ContextInProgress,
	}

	result := shared.RenderErrorList(config)

	// Should show all 3 errors (at limit)
	g.Expect(result).Should(ContainSubstring("file1.txt"))
	g.Expect(result).Should(ContainSubstring("file2.txt"))
	g.Expect(result).Should(ContainSubstring("file3.txt"))

	// Should NOT contain overflow message when exactly at limit
	g.Expect(result).ShouldNot(ContainSubstring("... and"))
	g.Expect(result).ShouldNot(ContainSubstring("more (see summary)"))
	g.Expect(result).ShouldNot(ContainSubstring("more error(s)"))
}

func TestRenderErrorList_InProgressContext_OverLimit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fileErrors := []syncengine.FileError{
		{FilePath: "/path/to/file1.txt", Error: errors.New("error 1")},
		{FilePath: "/path/to/file2.txt", Error: errors.New("error 2")},
		{FilePath: "/path/to/file3.txt", Error: errors.New("error 3")},
		{FilePath: "/path/to/file4.txt", Error: errors.New("error 4")},
		{FilePath: "/path/to/file5.txt", Error: errors.New("error 5")},
	}

	config := shared.ErrorListConfig{
		Errors:  fileErrors,
		Context: shared.ContextInProgress,
	}

	result := shared.RenderErrorList(config)

	// Should show first 3 errors only
	g.Expect(result).Should(ContainSubstring("file1.txt"))
	g.Expect(result).Should(ContainSubstring("file2.txt"))
	g.Expect(result).Should(ContainSubstring("file3.txt"))

	// Should NOT show errors beyond limit
	g.Expect(result).ShouldNot(ContainSubstring("file4.txt"))
	g.Expect(result).ShouldNot(ContainSubstring("file5.txt"))

	// Should show in-progress overflow message
	g.Expect(result).Should(ContainSubstring("... and 2 more (see summary)"))
}

func TestRenderErrorList_CompleteContext_OverLimit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create 12 errors (over the limit of 10)
	fileErrors := make([]syncengine.FileError, 12)
	for i := range 12 {
		fileErrors[i] = syncengine.FileError{
			FilePath: "/path/to/file" + string(rune('0'+i)) + ".txt",
			Error:    errors.New("error"),
		}
	}

	config := shared.ErrorListConfig{
		Errors:  fileErrors,
		Context: shared.ContextComplete,
	}

	result := shared.RenderErrorList(config)

	// Should show first 10 errors (count error symbols ✗)
	g.Expect(strings.Count(result, "✗")).Should(Equal(10))

	// Should show complete context overflow message
	g.Expect(result).Should(ContainSubstring("... and 2 more error(s)"))
	g.Expect(result).ShouldNot(ContainSubstring("see summary"))
}

func TestRenderErrorList_OtherContext_OverLimit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create 7 errors (over the limit of 5)
	fileErrors := make([]syncengine.FileError, 7)
	for i := range 7 {
		fileErrors[i] = syncengine.FileError{
			FilePath: "/path/to/file" + string(rune('0'+i)) + ".txt",
			Error:    errors.New("error"),
		}
	}

	config := shared.ErrorListConfig{
		Errors:  fileErrors,
		Context: shared.ContextOther,
	}

	result := shared.RenderErrorList(config)

	// Should show first 5 errors (count error symbols ✗)
	g.Expect(strings.Count(result, "✗")).Should(Equal(5))

	// Should show other context overflow message
	g.Expect(result).Should(ContainSubstring("... and 2 more error(s)"))
	g.Expect(result).ShouldNot(ContainSubstring("see summary"))
}

func TestRenderErrorList_WithPathTruncation(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	longPath := "/very/long/path/to/some/deeply/nested/directory/structure/file.txt"
	fileErrors := []syncengine.FileError{
		{FilePath: longPath, Error: errors.New("test error")},
	}

	truncateFunc := func(path string, maxWidth int) string {
		if len(path) <= maxWidth {
			return path
		}

		return path[:10] + "..." + path[len(path)-10:]
	}

	config := shared.ErrorListConfig{
		Errors:           fileErrors,
		Context:          shared.ContextInProgress,
		MaxWidth:         30,
		TruncatePathFunc: truncateFunc,
	}

	result := shared.RenderErrorList(config)

	// Should contain truncated path on the main error line
	lines := strings.Split(result, "\n")
	var errorLine string
	for _, line := range lines {
		if strings.Contains(line, "✗") {
			errorLine = line

			break
		}
	}
	g.Expect(errorLine).Should(ContainSubstring("/very/long...e/file.txt"))

	// Full path may appear in suggestions (which is helpful), so we verify
	// the truncation worked on the error display line specifically
	g.Expect(errorLine).ShouldNot(ContainSubstring("/path/to/some/deeply/nested/directory/structure/"))
}

func TestRenderErrorList_WithErrorMessageTruncation(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	longError := "This is a very long error message that exceeds the maximum width and should be truncated"
	fileErrors := []syncengine.FileError{
		{FilePath: "/path/to/file.txt", Error: errors.New(longError)},
	}

	config := shared.ErrorListConfig{
		Errors:   fileErrors,
		Context:  shared.ContextInProgress,
		MaxWidth: 50,
	}

	result := shared.RenderErrorList(config)

	// Should contain truncated error message ending with "..."
	lines := strings.Split(result, "\n")
	var errorLine string
	for _, line := range lines {
		if strings.Contains(line, "This is a very long") {
			errorLine = line

			break
		}
	}

	g.Expect(errorLine).ShouldNot(BeEmpty())
	g.Expect(errorLine).Should(ContainSubstring("..."))
	g.Expect(len(strings.TrimSpace(errorLine))).Should(BeNumerically("<=", 50))
}

func TestRenderErrorList_FormattingStructure(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fileErrors := []syncengine.FileError{
		{FilePath: "/path/to/file.txt", Error: errors.New("test error")},
	}

	config := shared.ErrorListConfig{
		Errors:  fileErrors,
		Context: shared.ContextInProgress,
	}

	result := shared.RenderErrorList(config)

	// Should contain error symbol
	g.Expect(result).Should(ContainSubstring("✗"))

	// Should have proper indentation structure
	lines := strings.Split(result, "\n")
	g.Expect(len(lines)).Should(BeNumerically(">=", 2))

	// First line should be the file path with error symbol
	g.Expect(lines[0]).Should(ContainSubstring("✗"))
	g.Expect(lines[0]).Should(ContainSubstring("file.txt"))

	// Second line should be the error message (indented)
	g.Expect(lines[1]).Should(MatchRegexp(`^\s+test error`))
}
