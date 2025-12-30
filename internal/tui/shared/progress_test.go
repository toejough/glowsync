package shared_test

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/tui/shared"
)

func TestRenderASCIIProgress_HundredPercent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	result := shared.RenderASCIIProgress(1.0, 40)
	expected := "[========================================] 100%"

	g.Expect(result).To(Equal(expected), "100%% progress should show full bar")
}

func TestRenderASCIIProgress_MidRange(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	result := shared.RenderASCIIProgress(0.45, 40)
	expected := "[================>                       ] 45%"

	g.Expect(result).To(Equal(expected), "45%% progress should show arrow at correct position")
}

func TestRenderASCIIProgress_NarrowWidths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		percent  float64
		width    int
		expected string
	}{
		{
			name:     "Width 5, 50%",
			percent:  0.50,
			width:    5,
			expected: "[=>   ] 50%",
		},
		{
			name:     "Width 3, 50%",
			percent:  0.50,
			width:    3,
			expected: "[>  ] 50%",
		},
		{
			name:     "Width 2, 50%",
			percent:  0.50,
			width:    2,
			expected: "[> ] 50%",
		},
		{
			name:     "Width 1, 50%",
			percent:  0.50,
			width:    1,
			expected: "[>] 50%",
		},
	}

	for _, tt := range tests { //nolint:varnamelen // Standard Go idiom for table-driven tests
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := shared.RenderASCIIProgress(tt.percent, tt.width)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestRenderASCIIProgress_VariousPercentages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		percent  float64
		width    int
		expected string
	}{
		{
			name:     "25% at width 40",
			percent:  0.25,
			width:    40,
			expected: "[========>                               ] 25%",
		},
		{
			name:     "75% at width 40",
			percent:  0.75,
			width:    40,
			expected: "[============================>           ] 75%",
		},
		{
			name:     "50% at width 20",
			percent:  0.50,
			width:    20,
			expected: "[========>           ] 50%",
		},
	}

	for _, tt := range tests { //nolint:varnamelen // Standard Go idiom for table-driven tests
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := shared.RenderASCIIProgress(tt.percent, tt.width)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestRenderASCIIProgress_ZeroPercent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	result := shared.RenderASCIIProgress(0.0, 40)
	expected := "[                                        ] 0%"

	g.Expect(result).To(Equal(expected), "0%% progress should show empty bar")
}

//nolint:paralleltest // This test modifies package-level state (colorsDisabled variable)
func TestRenderProgress_WithASCIIFallback(t *testing.T) {
	g := NewWithT(t) //nolint:varnamelen // Standard Gomega pattern

	// Note: This test is NOT parallel because it modifies package-level state
	// by temporarily overriding the colorsDisabled variable

	// Save original state
	originalColorsDisabled := shared.GetColorsDisabled()
	defer shared.SetColorsDisabledForTesting(originalColorsDisabled)

	// Force colors disabled
	shared.SetColorsDisabledForTesting(true)

	// Create a progress model (width is what matters for ASCII rendering)
	model := shared.NewProgressModel(40)

	// Test with 45% progress
	result := shared.RenderProgress(model, 0.45)
	expected := "[================>                       ] 45%"

	g.Expect(result).To(Equal(expected), "RenderProgress should use ASCII fallback when colors disabled")
}

//nolint:paralleltest // This test modifies package-level state (colorsDisabled variable)
func TestRenderProgress_WithBubbleTeaProgress(t *testing.T) {
	g := NewWithT(t) //nolint:varnamelen // Standard Gomega pattern

	// Note: This test is NOT parallel because it modifies package-level state
	// by temporarily overriding the colorsDisabled variable

	// Save original state
	originalColorsDisabled := shared.GetColorsDisabled()
	defer shared.SetColorsDisabledForTesting(originalColorsDisabled)

	// Force colors enabled
	shared.SetColorsDisabledForTesting(false)

	// Create a progress model
	model := shared.NewProgressModel(40)

	// Test with 45% progress
	result := shared.RenderProgress(model, 0.45)

	// We can't easily verify the exact Bubble Tea output, but we can verify:
	// 1. It's not empty
	// 2. It's different from the ASCII version (contains ANSI codes or Unicode)
	g.Expect(result).NotTo(BeEmpty(), "RenderProgress should return non-empty output")

	// The ASCII version would be exactly this string:
	asciiVersion := "[================>                       ] 45%"
	g.Expect(result).NotTo(Equal(asciiVersion), "RenderProgress should use styled output when colors enabled, not ASCII")
}
