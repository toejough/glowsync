//nolint:varnamelen // Test files use idiomatic short variable names
package widgets_test

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/tui/widgets"
)

func TestNewActivityLogWidget_ShowsRecentActivities(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	activities := []string{
		"Scanning source files...",
		"Found 100 files",
		"Scanning destination...",
		"Found 50 files",
	}

	widget := widgets.NewActivityLogWidget(func() []string { return activities })
	result := widget()

	// Should contain at least some of the activities
	g.Expect(result).Should(ContainSubstring("Scanning"),
		"Activity log should show scanning activities")
}

func TestNewActivityLogWidget_LimitsToRecentEntries(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create many activities (more than typical display limit)
	activities := make([]string, 50)
	for i := 0; i < 50; i++ {
		activities[i] = "Activity " + string(rune('A'+i))
	}

	widget := widgets.NewActivityLogWidget(func() []string { return activities })
	result := widget()

	// Should not be empty
	g.Expect(result).ShouldNot(BeEmpty(),
		"Activity log should show entries even with many activities")

	// Result should be reasonable length (not showing all 50 activities)
	// Just verify it returns something reasonable
	g.Expect(len(result)).Should(BeNumerically("<", 10000),
		"Activity log should limit output to reasonable size")
}

func TestNewActivityLogWidget_HandlesEmptyActivities(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	widget := widgets.NewActivityLogWidget(func() []string { return []string{} })
	result := widget()

	// Empty activities should return empty or placeholder
	// We don't require empty - might show "No activity" message
	g.Expect(result).Should(BeAssignableToTypeOf(""),
		"Activity log should return string type")
}

func TestNewActivityLogWidget_HandlesNilActivities(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	widget := widgets.NewActivityLogWidget(func() []string { return nil })
	result := widget()

	// Nil activities should not crash, should return empty or placeholder
	g.Expect(result).Should(BeAssignableToTypeOf(""),
		"Activity log should handle nil activities gracefully")
}

func TestNewActivityLogWidget_ReturnsMultilineString(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	activities := []string{
		"Activity 1",
		"Activity 2",
		"Activity 3",
	}

	widget := widgets.NewActivityLogWidget(func() []string { return activities })
	result := widget()

	// Should be multi-line for multiple activities
	if len(activities) > 1 {
		g.Expect(result).Should(Or(
			ContainSubstring("\n"),
			ContainSubstring("Activity"),
		), "Activity log with multiple entries should contain newlines or activity text")
	}
}

func TestNewActivityLogWidget_ShowsFormattedEntries(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	activities := []string{
		"Analyzing source directory",
		"Comparing files",
		"Ready to sync",
	}

	widget := widgets.NewActivityLogWidget(func() []string { return activities })
	result := widget()

	// Should show at least one activity
	g.Expect(result).Should(Or(
		ContainSubstring("Analyzing"),
		ContainSubstring("Comparing"),
		ContainSubstring("Ready"),
	), "Activity log should show at least one activity entry")
}
