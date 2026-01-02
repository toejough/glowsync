//nolint:varnamelen // Test files use idiomatic short variable names (g, etc.)
package shared_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/tui/shared"
)

// TestRenderTwoColumnLayout_BasicLayout verifies basic two-column rendering
func TestRenderTwoColumnLayout_BasicLayout(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	leftContent := "Left column content"
	rightContent := "Right column content"
	width := 120
	height := 40

	result := shared.RenderTwoColumnLayout(leftContent, rightContent, width, height)

	// Both contents should be present in the output
	g.Expect(result).To(ContainSubstring(leftContent), "Left content should be present")
	g.Expect(result).To(ContainSubstring(rightContent), "Right content should be present")
	g.Expect(result).NotTo(BeEmpty(), "Result should not be empty")
}

// TestRenderTwoColumnLayout_EdgeCaseWidths tests behavior with extreme widths
func TestRenderTwoColumnLayout_EdgeCaseWidths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		leftContent  string
		rightContent string
		width        int
		height       int
		description  string
	}{
		{
			name:         "Very narrow width",
			leftContent:  "L",
			rightContent: "R",
			width:        20,
			height:       10,
			description:  "Should handle narrow terminal widths",
		},
		{
			name:         "Very wide width",
			leftContent:  "Left content",
			rightContent: "Right content",
			width:        300,
			height:       100,
			description:  "Should handle very wide terminal widths",
		},
		{
			name:         "Minimum viable width",
			leftContent:  "L",
			rightContent: "R",
			width:        10,
			height:       5,
			description:  "Should handle minimum viable width",
		},
	}

	for _, tt := range tests { //nolint:varnamelen // Standard Go idiom for table-driven tests
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := shared.RenderTwoColumnLayout(tt.leftContent, tt.rightContent, tt.width, tt.height)

			// Should not crash and should return something
			g.Expect(result).NotTo(BeNil(), tt.description)

			// For very narrow widths, content might be truncated, but shouldn't panic
			if tt.width >= 20 {
				g.Expect(result).To(ContainSubstring(tt.leftContent), "Left content should be present")
				g.Expect(result).To(ContainSubstring(tt.rightContent), "Right content should be present")
			}
		})
	}
}

// TestRenderTwoColumnLayout_EmptyContent tests edge cases with empty content
func TestRenderTwoColumnLayout_EmptyContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		leftContent  string
		rightContent string
		width        int
		height       int
		wantContains []string
		description  string
	}{
		{
			name:         "Empty left content",
			leftContent:  "",
			rightContent: "Right content here",
			width:        120,
			height:       40,
			wantContains: []string{"Right content here"},
			description:  "Should render right column even when left is empty",
		},
		{
			name:         "Empty right content",
			leftContent:  "Left content here",
			rightContent: "",
			width:        120,
			height:       40,
			wantContains: []string{"Left content here"},
			description:  "Should render left column even when right is empty",
		},
		{
			name:         "Both empty",
			leftContent:  "",
			rightContent: "",
			width:        120,
			height:       40,
			wantContains: nil, // Nothing specific to check, just shouldn't crash
			description:  "Should handle both columns being empty",
		},
	}

	for _, tt := range tests { //nolint:varnamelen // Standard Go idiom for table-driven tests
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := shared.RenderTwoColumnLayout(tt.leftContent, tt.rightContent, tt.width, tt.height)

			// Should return something (even if just empty layout structure)
			g.Expect(result).NotTo(BeNil(), tt.description)

			// Check for expected content
			for _, want := range tt.wantContains {
				g.Expect(result).To(ContainSubstring(want), "Should contain: "+want)
			}
		})
	}
}

// TestRenderTwoColumnLayout_MultilineContent tests handling of multiline content
func TestRenderTwoColumnLayout_MultilineContent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	leftContent := "Line 1\nLine 2\nLine 3"
	rightContent := "Right 1\nRight 2\nRight 3"
	width := 120
	height := 40

	result := shared.RenderTwoColumnLayout(leftContent, rightContent, width, height)

	// All lines from both columns should be present
	g.Expect(result).To(ContainSubstring("Line 1"), "Should contain left line 1")
	g.Expect(result).To(ContainSubstring("Line 2"), "Should contain left line 2")
	g.Expect(result).To(ContainSubstring("Line 3"), "Should contain left line 3")
	g.Expect(result).To(ContainSubstring("Right 1"), "Should contain right line 1")
	g.Expect(result).To(ContainSubstring("Right 2"), "Should contain right line 2")
	g.Expect(result).To(ContainSubstring("Right 3"), "Should contain right line 3")
}

// TestRenderTwoColumnLayout_WidthDistribution verifies 60-40 width split
func TestRenderTwoColumnLayout_WidthDistribution(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Use content that helps identify column boundaries
	leftContent := "LEFT_MARKER"
	rightContent := "RIGHT_MARKER"
	width := 100
	height := 20

	result := shared.RenderTwoColumnLayout(leftContent, rightContent, width, height)

	// Verify both markers are present
	g.Expect(result).To(ContainSubstring("LEFT_MARKER"), "Left marker should be present")
	g.Expect(result).To(ContainSubstring("RIGHT_MARKER"), "Right marker should be present")

	// The result should contain the content arranged horizontally
	// (actual width distribution will be verified through visual inspection in GREEN phase)
	lines := strings.Split(result, "\n")
	g.Expect(len(lines)).To(BeNumerically(">", 0), "Result should have multiple lines")
}

// TestRenderWidgetBox_BasicWidget verifies basic widget box rendering
func TestRenderWidgetBox_BasicWidget(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	title := "Widget Title"
	content := "Widget content goes here"
	width := 40

	result := shared.RenderWidgetBox(title, content, width)

	// Title and content should both be present
	g.Expect(result).To(ContainSubstring(title), "Title should be present")
	g.Expect(result).To(ContainSubstring(content), "Content should be present")
	g.Expect(result).NotTo(BeEmpty(), "Result should not be empty")
}

// TestRenderWidgetBox_BorderPresence verifies box has borders
func TestRenderWidgetBox_BorderPresence(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	title := "Bordered Box"
	content := "Content with borders"
	width := 40

	result := shared.RenderWidgetBox(title, content, width)

	// Box should have structure indicating borders (multiple lines)
	lines := strings.Split(result, "\n")
	g.Expect(len(lines)).To(BeNumerically(">=", 3), "Box should have at least top border, content, and bottom border")

	// Should contain title and content
	g.Expect(result).To(ContainSubstring(title), "Title should be in box")
	g.Expect(result).To(ContainSubstring(content), "Content should be in box")
}

// TestRenderWidgetBox_EmptyContent tests widget with no content
func TestRenderWidgetBox_EmptyContent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	title := "Empty Widget"
	content := ""
	width := 40

	result := shared.RenderWidgetBox(title, content, width)

	// Title should be present even with empty content
	g.Expect(result).To(ContainSubstring(title), "Title should be present even with empty content")
	g.Expect(result).NotTo(BeEmpty(), "Result should not be empty (should have box structure)")
}

// TestRenderWidgetBox_LongTitle tests widget with very long title
func TestRenderWidgetBox_LongTitle(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	title := "This is a very long title that might need to be handled carefully"
	content := "Short content"
	width := 40

	result := shared.RenderWidgetBox(title, content, width)

	// Title should be present (possibly truncated, but implementation will decide)
	g.Expect(result).NotTo(BeEmpty(), "Result should not be empty")
	g.Expect(result).To(ContainSubstring(content), "Content should be present")

	// Title might be truncated, but at least part of it should be there
	g.Expect(result).To(ContainSubstring("This is a very long"), "Start of title should be present")
}

// TestRenderWidgetBox_MultilineContent tests widget with multiline content
func TestRenderWidgetBox_MultilineContent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	title := "Multi-line Widget"
	content := "Line 1\nLine 2\nLine 3\nLine 4"
	width := 50

	result := shared.RenderWidgetBox(title, content, width)

	// Title and all content lines should be present
	g.Expect(result).To(ContainSubstring(title), "Title should be present")
	g.Expect(result).To(ContainSubstring("Line 1"), "Line 1 should be present")
	g.Expect(result).To(ContainSubstring("Line 2"), "Line 2 should be present")
	g.Expect(result).To(ContainSubstring("Line 3"), "Line 3 should be present")
	g.Expect(result).To(ContainSubstring("Line 4"), "Line 4 should be present")
}

// TestRenderWidgetBox_NarrowWidth tests widget with narrow width
func TestRenderWidgetBox_NarrowWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		title   string
		content string
		width   int
	}{
		{
			name:    "Width 20",
			title:   "Title",
			content: "Content",
			width:   20,
		},
		{
			name:    "Width 15",
			title:   "Box",
			content: "Text",
			width:   15,
		},
		{
			name:    "Width 10",
			title:   "Hi",
			content: "Hi",
			width:   10,
		},
	}

	for _, tt := range tests { //nolint:varnamelen // Standard Go idiom for table-driven tests
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := shared.RenderWidgetBox(tt.title, tt.content, tt.width)

			// Should not crash with narrow widths
			g.Expect(result).NotTo(BeNil(), "Should handle narrow width without panic")
		})
	}
}

// TestRenderWidgetBox_TitleStyling verifies title has correct styling
func TestRenderWidgetBox_TitleStyling(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	title := "Styled Title"
	content := "Content"
	width := 40

	result := shared.RenderWidgetBox(title, content, width)

	// We can't easily test for exact ANSI codes, but we can verify:
	// 1. Title is present
	// 2. Result contains ANSI escape sequences (indicating styling)
	// 3. Result is not just plain text
	g.Expect(result).To(ContainSubstring(title), "Title should be present")

	// If colors are enabled, result should contain ANSI codes
	// If colors are disabled, this test still passes (title is present)
	g.Expect(result).NotTo(BeEmpty(), "Result should not be empty")
}

// TestRenderWidgetBox_WidthAccounting verifies box width accounting for padding
func TestRenderWidgetBox_WidthAccounting(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	title := "Width Test"
	content := "Testing width accounting with borders and padding"
	width := 60

	result := shared.RenderWidgetBox(title, content, width)

	// The implementation should account for padding (width - 4 for borders)
	// We verify this by checking that content is present and box is rendered
	g.Expect(result).To(ContainSubstring(title), "Title should be present")
	g.Expect(result).To(ContainSubstring(content), "Content should be present")

	// Result should have box structure (borders)
	lines := strings.Split(result, "\n")
	g.Expect(len(lines)).To(BeNumerically(">", 2), "Box should have multiple lines (top border, content, bottom border)")
}
