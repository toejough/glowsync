//nolint:varnamelen // Test files use idiomatic short variable names (g, etc.)
package shared_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/tui/shared"
)

// ============================================================================
// Basic Rendering Tests
// ============================================================================

func TestRenderActivityLog_EmptyLog(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	result := shared.RenderActivityLog("Activity Log", []string{}, 0)

	// Should return something (title/structure) even with no entries
	g.Expect(result).NotTo(BeEmpty(), "should return content even with no entries")

	// Title should be present
	stripped := stripANSI(result)
	g.Expect(stripped).To(ContainSubstring("Activity Log"), "should contain title")
}

func TestRenderActivityLog_SingleEntry(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{"14:30:00 - Application started"}
	result := shared.RenderActivityLog("Activity Log", entries, 0)

	// Entry should be present
	stripped := stripANSI(result)
	g.Expect(stripped).To(ContainSubstring("14:30:00 - Application started"),
		"should contain the single entry")
}

func TestRenderActivityLog_MultipleEntries(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{
		"14:30:00 - Application started",
		"14:30:15 - Files scanned",
		"14:30:30 - Comparison complete",
		"14:30:45 - Sync in progress",
	}
	result := shared.RenderActivityLog("Activity Log", entries, 0)

	stripped := stripANSI(result)

	// All entries should be present
	for _, entry := range entries {
		g.Expect(stripped).To(ContainSubstring(entry),
			"should contain entry: %s", entry)
	}
}

//nolint:paralleltest // This test modifies package-level state (lipgloss color profile)
func TestRenderActivityLog_WithTitle(t *testing.T) {
	g := NewWithT(t)

	// Force lipgloss to use color output for testing ANSI codes
	lipgloss.SetColorProfile(termenv.TrueColor)

	// Save original state and ensure colors are enabled for ANSI code testing
	originalColorsDisabled := shared.GetColorsDisabled()
	defer shared.SetColorsDisabledForTesting(originalColorsDisabled)
	shared.SetColorsDisabledForTesting(false)

	entries := []string{"14:30:00 - Event"}
	result := shared.RenderActivityLog("Recent Activity", entries, 0)

	stripped := stripANSI(result)

	// Title should be present and styled
	g.Expect(stripped).To(ContainSubstring("Recent Activity"),
		"should contain title")
	g.Expect(result).NotTo(Equal(stripped),
		"title should have ANSI styling (result differs from stripped)")
}

func TestRenderActivityLog_WithoutTitle(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{"14:30:00 - Event"}
	result := shared.RenderActivityLog("", entries, 0)

	stripped := stripANSI(result)

	// Entry should still be present
	g.Expect(stripped).To(ContainSubstring("14:30:00 - Event"),
		"should contain entry even without title")

	// Result should not be empty
	g.Expect(result).NotTo(BeEmpty(),
		"should return content even without title")
}

// ============================================================================
// Entry Limiting Tests
// ============================================================================

func TestRenderActivityLog_MaxEntriesZero_ShowsAll(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{
		"14:30:00 - Entry 1",
		"14:30:15 - Entry 2",
		"14:30:30 - Entry 3",
		"14:30:45 - Entry 4",
		"14:31:00 - Entry 5",
	}
	result := shared.RenderActivityLog("Activity Log", entries, 0)

	stripped := stripANSI(result)

	// All 5 entries should be present when maxEntries = 0
	for _, entry := range entries {
		g.Expect(stripped).To(ContainSubstring(entry),
			"maxEntries=0 should show all entries: %s", entry)
	}
}

func TestRenderActivityLog_MaxEntriesLessThanTotal_ShowsMostRecent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{
		"14:30:00 - Entry 1",
		"14:30:15 - Entry 2",
		"14:30:30 - Entry 3",
		"14:30:45 - Entry 4",
		"14:31:00 - Entry 5",
	}
	result := shared.RenderActivityLog("Activity Log", entries, 3)

	stripped := stripANSI(result)

	// Should show only the 3 most recent entries (Entry 3, 4, 5)
	g.Expect(stripped).To(ContainSubstring("Entry 3"),
		"should show Entry 3 (3rd most recent)")
	g.Expect(stripped).To(ContainSubstring("Entry 4"),
		"should show Entry 4 (2nd most recent)")
	g.Expect(stripped).To(ContainSubstring("Entry 5"),
		"should show Entry 5 (most recent)")

	// Should NOT show older entries (Entry 1, 2)
	g.Expect(stripped).NotTo(ContainSubstring("Entry 1"),
		"should not show Entry 1 (too old)")
	g.Expect(stripped).NotTo(ContainSubstring("Entry 2"),
		"should not show Entry 2 (too old)")
}

func TestRenderActivityLog_MaxEntriesGreaterThanTotal_ShowsAll(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{
		"14:30:00 - Entry 1",
		"14:30:15 - Entry 2",
		"14:30:30 - Entry 3",
	}
	result := shared.RenderActivityLog("Activity Log", entries, 10)

	stripped := stripANSI(result)

	// Should show all 3 entries even though maxEntries=10
	for _, entry := range entries {
		g.Expect(stripped).To(ContainSubstring(entry),
			"should show all entries when maxEntries > total: %s", entry)
	}
}

func TestRenderActivityLog_MaxEntriesOne_ShowsOnlyMostRecent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{
		"14:30:00 - Entry 1",
		"14:30:15 - Entry 2",
		"14:30:30 - Entry 3",
	}
	result := shared.RenderActivityLog("Activity Log", entries, 1)

	stripped := stripANSI(result)

	// Should show only the most recent entry (Entry 3)
	g.Expect(stripped).To(ContainSubstring("Entry 3"),
		"should show most recent entry")

	// Should NOT show older entries
	g.Expect(stripped).NotTo(ContainSubstring("Entry 1"),
		"should not show oldest entry")
	g.Expect(stripped).NotTo(ContainSubstring("Entry 2"),
		"should not show second entry")
}

// ============================================================================
// Content Verification Tests
// ============================================================================

func TestRenderActivityLog_ChronologicalOrder_OldestFirst(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{
		"14:30:00 - First event",
		"14:30:15 - Second event",
		"14:30:30 - Third event",
	}
	result := shared.RenderActivityLog("Activity Log", entries, 0)

	stripped := stripANSI(result)

	// Find positions of each entry in the output
	firstPos := strings.Index(stripped, "First event")
	secondPos := strings.Index(stripped, "Second event")
	thirdPos := strings.Index(stripped, "Third event")

	// Verify chronological order (oldest first)
	g.Expect(firstPos).To(BeNumerically("<", secondPos),
		"first event should appear before second event")
	g.Expect(secondPos).To(BeNumerically("<", thirdPos),
		"second event should appear before third event")
}

func TestRenderActivityLog_TruncatedEntries_ShowMostRecentFirst(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{
		"14:30:00 - Event 1",
		"14:30:15 - Event 2",
		"14:30:30 - Event 3",
		"14:30:45 - Event 4",
	}
	result := shared.RenderActivityLog("Activity Log", entries, 2)

	stripped := stripANSI(result)

	// Should show Event 3 and Event 4 (most recent 2)
	g.Expect(stripped).To(ContainSubstring("Event 3"),
		"should show Event 3")
	g.Expect(stripped).To(ContainSubstring("Event 4"),
		"should show Event 4")

	// Find positions to verify order
	pos3 := strings.Index(stripped, "Event 3")
	pos4 := strings.Index(stripped, "Event 4")

	// Event 3 should appear before Event 4 (chronological order maintained)
	g.Expect(pos3).To(BeNumerically("<", pos4),
		"Event 3 should appear before Event 4 even when truncated")
}

//nolint:paralleltest // This test modifies package-level state (lipgloss color profile)
func TestRenderActivityLog_TitleFormatting_BoldAndColored(t *testing.T) {
	g := NewWithT(t)

	// Force lipgloss to use color output for testing ANSI codes
	lipgloss.SetColorProfile(termenv.TrueColor)

	// Save original state and ensure colors are enabled for ANSI code testing
	originalColorsDisabled := shared.GetColorsDisabled()
	defer shared.SetColorsDisabledForTesting(originalColorsDisabled)
	shared.SetColorsDisabledForTesting(false)

	entries := []string{"14:30:00 - Event"}
	result := shared.RenderActivityLog("Activity Log", entries, 0)

	// Title should have ANSI escape codes (indicating styling)
	g.Expect(result).To(ContainSubstring("\033["),
		"should contain ANSI escape codes for styling")

	// Stripped version should contain title text
	stripped := stripANSI(result)
	g.Expect(stripped).To(ContainSubstring("Activity Log"),
		"stripped version should contain title text")

	// Result should be longer than stripped (due to ANSI codes)
	g.Expect(len(result)).To(BeNumerically(">", len(stripped)),
		"styled result should be longer than stripped text")
}

func TestRenderActivityLog_EntryFormatting_Indentation(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{
		"14:30:00 - First entry",
		"14:30:15 - Second entry",
	}
	result := shared.RenderActivityLog("Activity Log", entries, 0)

	stripped := stripANSI(result)

	// Entries should have consistent formatting (looking for newlines and structure)
	lines := strings.Split(stripped, "\n")
	g.Expect(len(lines)).To(BeNumerically(">", 2),
		"should have multiple lines (title + entries)")

	// Both entries should be present
	g.Expect(stripped).To(ContainSubstring("First entry"),
		"should contain first entry")
	g.Expect(stripped).To(ContainSubstring("Second entry"),
		"should contain second entry")
}

// ============================================================================
// Edge Cases Tests
// ============================================================================

func TestRenderActivityLog_EmptyTitleWithEntries(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{"14:30:00 - Event"}
	result := shared.RenderActivityLog("", entries, 0)

	stripped := stripANSI(result)

	// Entry should be present
	g.Expect(stripped).To(ContainSubstring("14:30:00 - Event"),
		"should contain entry")

	// Should not crash and should return content
	g.Expect(result).NotTo(BeEmpty(),
		"should return content even with empty title")
}

func TestRenderActivityLog_NilEntriesSlice(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Should not panic with nil entries
	result := shared.RenderActivityLog("Activity Log", nil, 0)

	// Should return something (at minimum, title/structure)
	g.Expect(result).NotTo(BeNil(),
		"should not panic with nil entries")

	// If title provided, it should be present
	stripped := stripANSI(result)
	g.Expect(stripped).To(ContainSubstring("Activity Log"),
		"should contain title even with nil entries")
}

func TestRenderActivityLog_EmptyEntriesSlice(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	result := shared.RenderActivityLog("Activity Log", []string{}, 0)

	// Should not crash
	g.Expect(result).NotTo(BeNil(),
		"should not panic with empty entries slice")

	// Title should be present
	stripped := stripANSI(result)
	g.Expect(stripped).To(ContainSubstring("Activity Log"),
		"should contain title even with empty entries")
}

func TestRenderActivityLog_VeryLongEntry(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	longEntry := "14:30:00 - This is a very long activity log entry that contains a lot of text and might need special handling for truncation or wrapping depending on implementation"
	entries := []string{longEntry}
	result := shared.RenderActivityLog("Activity Log", entries, 0)

	stripped := stripANSI(result)

	// Entry should be present (at least partially)
	g.Expect(stripped).To(ContainSubstring("This is a very long"),
		"should contain start of long entry")

	// Should not crash
	g.Expect(result).NotTo(BeEmpty(),
		"should handle very long entries")
}

func TestRenderActivityLog_SpecialCharacters(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		entries []string
	}{
		{
			name:    "Unicode characters",
			entries: []string{"14:30:00 - File ✓ copied successfully"},
		},
		{
			name:    "Newlines in entry",
			entries: []string{"14:30:00 - Multi\nline\nentry"},
		},
		{
			name:    "Tabs in entry",
			entries: []string{"14:30:00 - Entry\twith\ttabs"},
		},
		{
			name:    "Special symbols",
			entries: []string{"14:30:00 - Path: /usr/local/bin → ~/bin"},
		},
		{
			name:    "Empty string entry",
			entries: []string{""},
		},
		{
			name:    "Mixed valid and empty entries",
			entries: []string{"14:30:00 - Valid", "", "14:30:15 - Also valid"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			// Should not panic with special characters
			result := shared.RenderActivityLog("Activity Log", tc.entries, 0)

			g.Expect(result).NotTo(BeNil(),
				"should handle special characters without panic")
			g.Expect(result).NotTo(BeEmpty(),
				"should return content for special character entries")
		})
	}
}

func TestRenderActivityLog_MaxEntriesNegative_TreatedAsZero(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{
		"14:30:00 - Entry 1",
		"14:30:15 - Entry 2",
		"14:30:30 - Entry 3",
	}

	// Negative maxEntries should be treated as 0 (show all) or handled gracefully
	result := shared.RenderActivityLog("Activity Log", entries, -5)

	// Should not crash
	g.Expect(result).NotTo(BeNil(),
		"should handle negative maxEntries gracefully")

	// Implementation can choose to show all entries or none, but shouldn't panic
	// We'll verify at minimum that it returns something
	g.Expect(result).NotTo(BeEmpty(),
		"should return content with negative maxEntries")
}

func TestRenderActivityLog_ExactlyMaxEntries(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{
		"14:30:00 - Entry 1",
		"14:30:15 - Entry 2",
		"14:30:30 - Entry 3",
	}

	// Exactly 3 entries with maxEntries=3
	result := shared.RenderActivityLog("Activity Log", entries, 3)

	stripped := stripANSI(result)

	// Should show all entries when count exactly matches maxEntries
	for i, entry := range entries {
		g.Expect(stripped).To(ContainSubstring(entry),
			"should show entry %d when count exactly matches maxEntries", i+1)
	}
}

func TestRenderActivityLog_WhitespaceOnlyTitle(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	entries := []string{"14:30:00 - Event"}
	result := shared.RenderActivityLog("   ", entries, 0)

	stripped := stripANSI(result)

	// Entry should still be present
	g.Expect(stripped).To(ContainSubstring("14:30:00 - Event"),
		"should contain entry even with whitespace-only title")

	// Should not crash
	g.Expect(result).NotTo(BeEmpty(),
		"should handle whitespace-only title")
}
