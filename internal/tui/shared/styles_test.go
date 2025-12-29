//nolint:varnamelen // Test files use idiomatic short variable names (g, etc.)
package shared_test

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/tui/shared"
)

func TestRenderFunctions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test render functions
	g.Expect(shared.RenderBox("test")).Should(ContainSubstring("test"))
	g.Expect(shared.RenderDim("test")).Should(ContainSubstring("test"))
	g.Expect(shared.RenderError("test")).Should(ContainSubstring("test"))
	g.Expect(shared.RenderLabel("test")).Should(ContainSubstring("test"))
	g.Expect(shared.RenderSubtitle("test")).Should(ContainSubstring("test"))
	g.Expect(shared.RenderSuccess("test")).Should(ContainSubstring("test"))
	g.Expect(shared.RenderTitle("test")).Should(ContainSubstring("test"))
	g.Expect(shared.RenderWarning("test")).Should(ContainSubstring("test"))
}

func TestStyles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test style functions don't crash
	_ = shared.BoxStyle()
	_ = shared.CompletionSelectedStyle()
	_ = shared.CompletionStyle()
	_ = shared.DimStyle()
	_ = shared.ErrorStyle()
	_ = shared.FileItemCompleteStyle()
	_ = shared.FileItemCopyingStyle()
	_ = shared.FileItemErrorStyle()
	_ = shared.FileItemStyle()
	_ = shared.LabelStyle()
	_ = shared.SubtitleStyle()
	_ = shared.SuccessStyle()
	_ = shared.TitleStyle()
	_ = shared.WarningStyle()

	// Test color functions
	g.Expect(shared.AccentColor()).ShouldNot(BeEmpty())
	g.Expect(shared.DimColor()).ShouldNot(BeEmpty())
	g.Expect(shared.ErrorColor()).ShouldNot(BeEmpty())
	g.Expect(shared.HighlightColor()).ShouldNot(BeEmpty())
	g.Expect(shared.NormalColor()).ShouldNot(BeEmpty())
	g.Expect(shared.PrimaryColor()).ShouldNot(BeEmpty())
	g.Expect(shared.SubtleColor()).ShouldNot(BeEmpty())
	g.Expect(shared.SuccessColor()).ShouldNot(BeEmpty())
	g.Expect(shared.WarningColor()).ShouldNot(BeEmpty())
}
