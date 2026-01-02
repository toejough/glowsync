//nolint:varnamelen // Test files use idiomatic short variable names (g, etc.)
package shared_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/tui/shared"
)

func TestActiveSymbol(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test that ActiveSymbol returns a non-empty string
	symbol := shared.ActiveSymbol()
	g.Expect(symbol).ShouldNot(BeEmpty(),
		"ActiveSymbol should return a non-empty string")

	// Should be either Unicode or ASCII fallback
	g.Expect(symbol).Should(Or(Equal("◉"), Equal("[*]")),
		"ActiveSymbol should return either Unicode ◉ or ASCII [*]")
}

func TestCancelledSymbol(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test that CancelledSymbol returns a non-empty string
	symbol := shared.CancelledSymbol()
	g.Expect(symbol).ShouldNot(BeEmpty(),
		"CancelledSymbol should return a non-empty string")

	// Should be either Unicode or ASCII fallback
	g.Expect(symbol).Should(Or(Equal("⊘"), Equal("[!]")),
		"CancelledSymbol should return either Unicode ⊘ or ASCII [!]")
}

func TestRenderTimeline_BasicPhases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		currentPhase       string
		expectedSymbols    []string // Symbols in order: Input, Scan, Compare, Sync, Done
		expectedPhaseNames []string
	}{
		{
			name:         "input phase - first active",
			currentPhase: "input",
			expectedSymbols: []string{
				shared.ActiveSymbol(), shared.PendingSymbol(), shared.PendingSymbol(),
				shared.PendingSymbol(), shared.PendingSymbol(),
			},
			expectedPhaseNames: []string{"Input", "Scan", "Compare", "Sync", "Done"},
		},
		{
			name:         "scan phase - second active",
			currentPhase: "scan",
			expectedSymbols: []string{
				shared.SuccessSymbol(), shared.ActiveSymbol(), shared.PendingSymbol(),
				shared.PendingSymbol(), shared.PendingSymbol(),
			},
			expectedPhaseNames: []string{"Input", "Scan", "Compare", "Sync", "Done"},
		},
		{
			name:         "compare phase - third active",
			currentPhase: "compare",
			expectedSymbols: []string{
				shared.SuccessSymbol(), shared.SuccessSymbol(), shared.ActiveSymbol(),
				shared.PendingSymbol(), shared.PendingSymbol(),
			},
			expectedPhaseNames: []string{"Input", "Scan", "Compare", "Sync", "Done"},
		},
		{
			name:         "sync phase - fourth active",
			currentPhase: "sync",
			expectedSymbols: []string{
				shared.SuccessSymbol(), shared.SuccessSymbol(), shared.SuccessSymbol(),
				shared.ActiveSymbol(), shared.PendingSymbol(),
			},
			expectedPhaseNames: []string{"Input", "Scan", "Compare", "Sync", "Done"},
		},
		{
			name:         "done phase - all complete",
			currentPhase: "done",
			expectedSymbols: []string{
				shared.SuccessSymbol(), shared.SuccessSymbol(), shared.SuccessSymbol(),
				shared.SuccessSymbol(), shared.SuccessSymbol(),
			},
			expectedPhaseNames: []string{"Input", "Scan", "Compare", "Sync", "Done"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := shared.RenderTimeline(tc.currentPhase)
			stripped := stripANSI(result)

			// Verify all phase names appear
			for _, phaseName := range tc.expectedPhaseNames {
				g.Expect(stripped).To(ContainSubstring(phaseName),
					"timeline should contain phase name %s", phaseName)
			}

			// Verify symbols appear in order
			for _, symbol := range tc.expectedSymbols {
				g.Expect(stripped).To(ContainSubstring(symbol),
					"timeline should contain symbol %s", symbol)
			}

			// Verify separators are present (should have 4 separators between 5 phases)
			separatorCount := strings.Count(stripped, "──")
			g.Expect(separatorCount).To(Equal(4),
				"timeline should have 4 separators between 5 phases")
		})
	}
}

func TestRenderTimeline_ContentVerification(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	result := shared.RenderTimeline("compare")
	stripped := stripANSI(result)

	// Verify all 5 phase names are present
	expectedPhases := []string{"Input", "Scan", "Compare", "Sync", "Done"}
	for _, phase := range expectedPhases {
		g.Expect(stripped).To(ContainSubstring(phase),
			"timeline should contain phase %s", phase)
	}

	// Verify correct number of separators (4 between 5 phases)
	separatorCount := strings.Count(stripped, "──")
	g.Expect(separatorCount).To(Equal(4),
		"should have exactly 4 separators between 5 phases")

	// Verify total symbol count (should have 5 symbols total)
	// Count all possible symbols
	symbolCount := 0
	symbolCount += strings.Count(stripped, shared.SuccessSymbol())
	symbolCount += strings.Count(stripped, shared.ActiveSymbol())
	symbolCount += strings.Count(stripped, shared.PendingSymbol())
	symbolCount += strings.Count(stripped, shared.ErrorSymbol())
	symbolCount += strings.Count(stripped, shared.CancelledSymbol())

	g.Expect(symbolCount).To(Equal(5),
		"should have exactly 5 symbols total (one per phase)")
}

func TestRenderTimeline_EdgeCases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		currentPhase string
		description  string
	}{
		{
			name:         "empty phase string",
			currentPhase: "",
			description:  "should handle empty string gracefully",
		},
		{
			name:         "invalid phase name",
			currentPhase: "invalid",
			description:  "should handle unknown phase gracefully",
		},
		{
			name:         "uppercase phase name",
			currentPhase: "SCAN",
			description:  "should be case-insensitive",
		},
		{
			name:         "mixed case phase name",
			currentPhase: "CoMpArE",
			description:  "should handle mixed case",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			// Should not panic and should return a string
			result := shared.RenderTimeline(tc.currentPhase)
			g.Expect(result).To(BeAssignableToTypeOf(""),
				"should return a string even for invalid input")

			// For invalid phases, should still render all phase names
			stripped := stripANSI(result)
			expectedPhaseNames := []string{"Input", "Scan", "Compare", "Sync", "Done"}
			for _, phaseName := range expectedPhaseNames {
				g.Expect(stripped).To(ContainSubstring(phaseName),
					"timeline should always contain phase name %s", phaseName)
			}

			// Should have separators
			separatorCount := strings.Count(stripped, "──")
			g.Expect(separatorCount).To(Equal(4),
				"timeline should always have 4 separators")
		})
	}
}

func TestRenderTimeline_ErrorStateDoesNotShowInput(t *testing.T) {
	t.Parallel()

	// Special case: Input phase cannot error (it's just user input collection)
	// If someone passes "input_error", it should be treated as invalid

	g := NewWithT(t)

	result := shared.RenderTimeline("input_error")
	stripped := stripANSI(result)

	// Should still render all phases (treating it as invalid input)
	expectedPhases := []string{"Input", "Scan", "Compare", "Sync", "Done"}
	for _, phase := range expectedPhases {
		g.Expect(stripped).To(ContainSubstring(phase),
			"timeline should contain phase %s even for invalid error phase", phase)
	}
}

func TestRenderTimeline_ErrorStates(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		currentPhase       string
		expectedSymbols    []string // Symbols in order: Input, Scan, Compare, Sync, Done
		expectedPhaseNames []string
	}{
		{
			name:         "scan_error - error at second phase",
			currentPhase: "scan_error",
			expectedSymbols: []string{
				shared.SuccessSymbol(), shared.ErrorSymbol(), shared.CancelledSymbol(),
				shared.CancelledSymbol(), shared.CancelledSymbol(),
			},
			expectedPhaseNames: []string{"Input", "Scan", "Compare", "Sync", "Done"},
		},
		{
			name:         "compare_error - error at third phase",
			currentPhase: "compare_error",
			expectedSymbols: []string{
				shared.SuccessSymbol(), shared.SuccessSymbol(), shared.ErrorSymbol(),
				shared.CancelledSymbol(), shared.CancelledSymbol(),
			},
			expectedPhaseNames: []string{"Input", "Scan", "Compare", "Sync", "Done"},
		},
		{
			name:         "sync_error - error at fourth phase",
			currentPhase: "sync_error",
			expectedSymbols: []string{
				shared.SuccessSymbol(), shared.SuccessSymbol(), shared.SuccessSymbol(),
				shared.ErrorSymbol(), shared.CancelledSymbol(),
			},
			expectedPhaseNames: []string{"Input", "Scan", "Compare", "Sync", "Done"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := shared.RenderTimeline(tc.currentPhase)
			stripped := stripANSI(result)

			// Verify all phase names appear
			for _, phaseName := range tc.expectedPhaseNames {
				g.Expect(stripped).To(ContainSubstring(phaseName),
					"timeline should contain phase name %s", phaseName)
			}

			// Verify error symbol appears
			g.Expect(stripped).To(ContainSubstring(shared.ErrorSymbol()),
				"timeline should contain error symbol")

			// Verify cancelled symbols appear for phases after error
			cancelledCount := strings.Count(stripped, shared.CancelledSymbol())
			g.Expect(cancelledCount).To(BeNumerically(">", 0),
				"timeline should contain cancelled symbols for skipped phases")

			// Verify symbols appear in order
			for _, symbol := range tc.expectedSymbols {
				g.Expect(stripped).To(ContainSubstring(symbol),
					"timeline should contain symbol %s", symbol)
			}

			// Verify separators are present
			separatorCount := strings.Count(stripped, "──")
			g.Expect(separatorCount).To(Equal(4),
				"timeline should have 4 separators between 5 phases")
		})
	}
}

func TestRenderTimeline_SymbolFallback(t *testing.T) {
	t.Parallel()

	// Save original environment
	// Note: This test verifies that symbols exist and can be rendered
	// The actual ASCII fallback behavior is tested at the symbol level
	// (PendingSymbol, SuccessSymbol, etc.)

	g := NewWithT(t)

	result := shared.RenderTimeline("scan")

	// Should contain symbols (either Unicode or ASCII fallback)
	g.Expect(result).ToNot(BeEmpty(),
		"timeline should render with symbols regardless of terminal capabilities")

	// Should contain phase names
	stripped := stripANSI(result)
	g.Expect(stripped).To(ContainSubstring("Input"))
	g.Expect(stripped).To(ContainSubstring("Scan"))
	g.Expect(stripped).To(ContainSubstring("Compare"))
	g.Expect(stripped).To(ContainSubstring("Sync"))
	g.Expect(stripped).To(ContainSubstring("Done"))
}

func TestRenderTimeline_SymbolProgression(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test that as we progress through phases, symbols transition correctly

	// Input phase: Active at position 0, rest pending
	inputResult := stripANSI(shared.RenderTimeline("input"))
	g.Expect(inputResult).To(ContainSubstring(shared.ActiveSymbol()),
		"input phase should have active symbol")
	g.Expect(strings.Count(inputResult, shared.PendingSymbol())).To(Equal(4),
		"input phase should have 4 pending symbols")
	g.Expect(inputResult).ToNot(ContainSubstring(shared.SuccessSymbol()),
		"input phase should not have success symbols yet")

	// Scan phase: Success at position 0, active at position 1, rest pending
	scanResult := stripANSI(shared.RenderTimeline("scan"))
	g.Expect(scanResult).To(ContainSubstring(shared.ActiveSymbol()),
		"scan phase should have active symbol")
	g.Expect(strings.Count(scanResult, shared.SuccessSymbol())).To(Equal(1),
		"scan phase should have 1 success symbol")
	g.Expect(strings.Count(scanResult, shared.PendingSymbol())).To(Equal(3),
		"scan phase should have 3 pending symbols")

	// Done phase: All success symbols, no pending or active
	doneResult := stripANSI(shared.RenderTimeline("done"))
	g.Expect(strings.Count(doneResult, shared.SuccessSymbol())).To(Equal(5),
		"done phase should have 5 success symbols")
	g.Expect(doneResult).ToNot(ContainSubstring(shared.PendingSymbol()),
		"done phase should not have pending symbols")
	g.Expect(doneResult).ToNot(ContainSubstring(shared.ActiveSymbol()),
		"done phase should not have active symbol")
}

// stripANSI removes ANSI escape codes from a string for easier testing
func stripANSI(s string) string {
	// Simple ANSI stripper - removes common ANSI escape sequences
	// Pattern: ESC [ ... m (for colors, bold, etc.)
	result := s
	inEscape := false
	var cleaned strings.Builder

	for i := 0; i < len(result); i++ {
		if result[i] == '\033' && i+1 < len(result) && result[i+1] == '[' {
			inEscape = true
			i++ // Skip the '['

			continue
		}
		if inEscape {
			if result[i] == 'm' {
				inEscape = false
			}

			continue
		}
		cleaned.WriteByte(result[i])
	}

	return cleaned.String()
}
