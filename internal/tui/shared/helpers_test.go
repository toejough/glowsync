package shared_test

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/tui/shared"
)

func TestFormatBytes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test various byte sizes
	g.Expect(shared.FormatBytes(500)).Should(Equal("500 B"))
	g.Expect(shared.FormatBytes(1024)).Should(Equal("1.0 KB"))
	g.Expect(shared.FormatBytes(1024 * 1024)).Should(Equal("1.0 MB"))
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test various durations
	result := shared.FormatDuration(30 * 1e9) // 30 seconds in nanoseconds
	g.Expect(result).ShouldNot(BeEmpty())
}

func TestFormatRate(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test various rates
	g.Expect(shared.FormatRate(500)).Should(Equal("500 B/s"))
	g.Expect(shared.FormatRate(1024)).Should(ContainSubstring("KB/s"))
	g.Expect(shared.FormatRate(1024 * 1024)).Should(ContainSubstring("MB/s"))
}

func TestRenderPath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		path     string
		maxWidth int
	}{
		{
			name:     "short path with style",
			path:     "short/path.txt",
			maxWidth: 50,
		},
		{
			name:     "long path truncated and styled",
			path:     "very/long/path/that/exceeds/the/maximum/width/file.txt",
			maxWidth: 30,
		},
		{
			name:     "empty path with style",
			path:     "",
			maxWidth: 50,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			gomega := NewWithT(t)

			// Test with FileItemCompleteStyle
			result := shared.RenderPath(testCase.path, shared.FileItemCompleteStyle(), testCase.maxWidth)

			// The result should contain the styled path
			// We can't test exact output due to ANSI codes, but we can verify:
			// 1. Result is not empty if path is not empty
			// 2. Result contains the truncated path
			if testCase.path != "" {
				gomega.Expect(result).ShouldNot(BeEmpty())
				// Should contain the truncated version
				gomega.Expect(result).Should(ContainSubstring(shared.RenderPathPlain(testCase.path, testCase.maxWidth)))
			}
		})
	}
}

func TestRenderPathPlain(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		path     string
		maxWidth int
		expected string
	}{
		{
			name:     "short path fits within width",
			path:     "short/path.txt",
			maxWidth: 50,
			expected: "short/path.txt",
		},
		{
			name:     "path exactly at max width",
			path:     "exactly/twenty/chars",
			maxWidth: 20,
			expected: "exactly/twenty/chars",
		},
		{
			name:     "long path truncated from middle",
			path:     "very/long/path/that/exceeds/the/maximum/width/file.txt",
			maxWidth: 30,
			expected: "very/long/pat...idth/file.txt",
		},
		{
			name:     "very long path with small width",
			path:     "extremely/long/path/with/many/directories/and/subdirectories/file.txt",
			maxWidth: 20,
			expected: "extremel...file.txt",
		},
		{
			name:     "empty path",
			path:     "",
			maxWidth: 50,
			expected: "",
		},
		{
			name:     "single character path",
			path:     "x",
			maxWidth: 50,
			expected: "x",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			gomega := NewWithT(t)

			result := shared.RenderPathPlain(testCase.path, testCase.maxWidth)
			gomega.Expect(result).Should(Equal(testCase.expected))
			// Verify result doesn't exceed maxWidth
			gomega.Expect(len(result)).Should(BeNumerically("<=", testCase.maxWidth))
		})
	}
}
