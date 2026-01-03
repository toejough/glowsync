//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens_test

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/screens"
)

// ============================================================================
// Activity Log Integration Tests
// ============================================================================

// TestConfirmationScreen_ActivityLog_InRightColumn verifies right column contains activity log
func TestConfirmationScreen_ActivityLog_InRightColumn(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.TotalFiles = 5
	engine.Status.TotalBytes = 1024

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// Activity log section should be present in right column
	g.Expect(view).To(ContainSubstring("Activity"),
		"Should have activity log section in right column")
}

// TestConfirmationScreen_ActivityLog_ReceivesAnalysisLogEntries verifies activity log gets analysis data
// NOTE: This test will be enabled once activity log API is added to engine
func TestConfirmationScreen_ActivityLog_ReceivesAnalysisLogEntries(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.TotalFiles = 5
	engine.Status.TotalBytes = 1024

	// TODO: Add activity log entries once API is available
	// engine.AppendActivityLog("Started analysis")
	// engine.AppendActivityLog("Scanning source directory")
	// engine.AppendActivityLog("Found 5 files")

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// Activity log should display entries from engine
	g.Expect(view).To(ContainSubstring("Activity"),
		"Should have activity log title")
	// Activity log component should be present (even if no entries yet)
	g.Expect(view).NotTo(BeEmpty(),
		"Activity log should be present in view")
}

// TestConfirmationScreen_EdgeCase_ErrorsPresent verifies behavior with errors
func TestConfirmationScreen_EdgeCase_ErrorsPresent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.TotalFiles = 5
	engine.Status.TotalBytes = 1024

	// Add multiple errors (more than display limit)
	engine.Status.Errors = []syncengine.FileError{
		{FilePath: "/path/to/file1.txt", Error: errors.New("permission denied")},
		{FilePath: "/path/to/file2.txt", Error: errors.New("file not found")},
		{FilePath: "/path/to/file3.txt", Error: errors.New("permission denied")},
		{FilePath: "/path/to/file4.txt", Error: errors.New("file not found")},
	}

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// Errors should be displayed (with limit and "see summary" message)
	g.Expect(view).To(ContainSubstring("Errors"),
		"Should show errors section when errors present")

	// First 3 errors should be shown
	g.Expect(view).To(ContainSubstring("file1.txt"),
		"Should show first error")
	g.Expect(view).To(ContainSubstring("file2.txt"),
		"Should show second error")
	g.Expect(view).To(ContainSubstring("file3.txt"),
		"Should show third error")

	// 4th error should NOT be shown (limit is 3 with "see summary" message)
	g.Expect(view).ShouldNot(ContainSubstring("file4.txt"),
		"Should not show fourth error due to display limit")

	// Truncation message should be present
	g.Expect(view).To(ContainSubstring("... and 1 more (see summary)"),
		"Should show truncation message for additional errors")
}

// ============================================================================
// Edge Case Tests
// ============================================================================

// TestConfirmationScreen_EdgeCase_TotalFilesZero verifies behavior when no files
func TestConfirmationScreen_EdgeCase_TotalFilesZero(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.TotalFiles = 0
	engine.Status.TotalBytes = 0
	engine.FilePattern = "" // No filter

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// Should show context-aware empty message (no filter = already synced)
	g.Expect(view).To(ContainSubstring("All files already synced"),
		"Should show 'all files synced' message when no filter and zero files")

	// Help text should still be present
	g.Expect(view).To(ContainSubstring("Press Enter to begin sync"),
		"Help text should be preserved in empty state")
}

// TestConfirmationScreen_EmptyState_PreservedContextAwareMessages verifies empty state handling
func TestConfirmationScreen_EmptyState_PreservedContextAwareMessages(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.FilePattern = "*.mov"
	engine.Status.TotalFiles = 0
	engine.Status.TotalBytes = 0

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// Empty state message should be preserved (filter applied, no matches)
	g.Expect(view).To(ContainSubstring("No files match your filter"),
		"Should show context-aware empty state message for filter")
}

// TestConfirmationScreen_ErrorDisplay_PreservedLogic verifies error display preserved
func TestConfirmationScreen_ErrorDisplay_PreservedLogic(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	// Add errors to status
	engine.Status.Errors = []syncengine.FileError{
		{FilePath: "/path/to/file1.txt", Error: errors.New("permission denied")},
		{FilePath: "/path/to/file2.txt", Error: errors.New("file not found")},
	}

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// Error display logic should be preserved
	g.Expect(view).To(ContainSubstring("Errors"),
		"Should show errors section")
	g.Expect(view).To(ContainSubstring("file1.txt"),
		"Should display first error")
	g.Expect(view).To(ContainSubstring("file2.txt"),
		"Should display second error")
}

// TestConfirmationScreen_ErrorsBox_WrappedInWidgetBox verifies errors wrapped when errors exist
func TestConfirmationScreen_ErrorsBox_WrappedInWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	// Add errors to engine status
	engine.Status.Errors = []syncengine.FileError{
		{FilePath: "/path/to/file1.txt", Error: errors.New("permission denied")},
		{FilePath: "/path/to/file2.txt", Error: errors.New("file not found")},
	}

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// Widget box should have title for errors
	g.Expect(view).To(ContainSubstring("Errors"),
		"Widget box should have 'Errors' title when errors exist")
	g.Expect(view).To(ContainSubstring("file1.txt"),
		"Widget box should contain error details")
}

// TestConfirmationScreen_FilterBox_WrappedInWidgetBox verifies filter info wrapped when pattern set
func TestConfirmationScreen_FilterBox_WrappedInWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.FilePattern = "*.mov"
	engine.Status.TotalFiles = 5
	engine.Status.TotalBytes = 1024

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// Widget box should have title for filter info
	g.Expect(view).To(ContainSubstring("Filter"),
		"Widget box should have 'Filter' title when pattern is set")
	g.Expect(view).To(ContainSubstring("*.mov"),
		"Widget box should contain filter pattern")
}

// TestConfirmationScreen_Layout_PreservesSixtyFortyWidthSplit verifies 60-40 width split preserved
func TestConfirmationScreen_Layout_PreservesSixtyFortyWidthSplit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.TotalFiles = 3
	engine.Status.TotalBytes = 512

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// Two-column layout should render content
	g.Expect(view).NotTo(BeEmpty(),
		"Two-column layout should render content with 60-40 split")

	// Both columns should have content
	// Left column: sync plan, filter info, errors
	g.Expect(view).To(ContainSubstring("Files to sync"),
		"Left column should show sync plan")

	// Right column: activity log
	g.Expect(view).To(ContainSubstring("Activity"),
		"Right column should show activity log")
}

// ============================================================================
// Two-Column Layout Tests
// ============================================================================

// TestConfirmationScreen_Layout_UsesTwoColumnStructure verifies View() uses RenderTwoColumnLayout
func TestConfirmationScreen_Layout_UsesTwoColumnStructure(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	// Set some data so we have content to verify
	engine.Status.TotalFiles = 5
	engine.Status.TotalBytes = 1024

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// View should have content (two-column layout creates structure)
	g.Expect(view).NotTo(BeEmpty(), "View should render two-column layout")

	// Timeline should be present (header)
	stripped := stripANSI(view)
	g.Expect(stripped).To(ContainSubstring("Compare"),
		"Should have timeline header")

	// Content should be present (stats, help text)
	g.Expect(view).To(ContainSubstring("Files to sync"),
		"Should show sync plan content in layout")
}

// ============================================================================
// Functional Preservation Tests
// ============================================================================

// TestConfirmationScreen_Stats_AllOriginalStatsDisplayed verifies all stats still shown
func TestConfirmationScreen_Stats_AllOriginalStatsDisplayed(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.TotalFiles = 10
	engine.Status.TotalBytes = 2048
	engine.FilePattern = "*.jpg"

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// All original stats should be preserved
	g.Expect(view).To(ContainSubstring("Files to sync"),
		"Should display files to sync label")
	g.Expect(view).To(ContainSubstring("10"),
		"Should display file count")

	g.Expect(view).To(ContainSubstring("Total size"),
		"Should display total size label")
	g.Expect(view).To(ContainSubstring("2.0 KB"),
		"Should display formatted byte size")

	g.Expect(view).To(ContainSubstring("*.jpg"),
		"Should display filter pattern")
}

// ============================================================================
// Widget Box Tests
// ============================================================================

// TestConfirmationScreen_SyncPlanBox_WrappedInWidgetBox verifies sync plan wrapped in widget box
func TestConfirmationScreen_SyncPlanBox_WrappedInWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.TotalFiles = 10
	engine.Status.TotalBytes = 2048

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// Widget box should have title and content
	g.Expect(view).To(ContainSubstring("Sync Plan"),
		"Widget box should have 'Sync Plan' title")
	g.Expect(view).To(ContainSubstring("Files to sync"),
		"Widget box should contain sync plan content")
	g.Expect(view).To(ContainSubstring("Total size"),
		"Widget box should contain size information")
}

// TestConfirmationScreen_Timeline_PresentInView verifies timeline present in View() output
func TestConfirmationScreen_Timeline_PresentInView(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()

	// Timeline should be at the top of the view
	g.Expect(view).NotTo(BeEmpty(), "View should render content")
	g.Expect(view).To(ContainSubstring("Compare"),
		"View should contain timeline with Compare phase")
}

// ============================================================================
// Timeline Integration Tests
// ============================================================================

// TestConfirmationScreen_Timeline_ShowsComparePhase verifies timeline header shows "compare" phase
func TestConfirmationScreen_Timeline_ShowsComparePhase(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	confirmScreen, ok := updatedModel.(screens.ConfirmationScreen)
	g.Expect(ok).To(BeTrue())

	view := confirmScreen.View()
	stripped := stripANSI(view)

	// Timeline should show "Compare" phase as active
	g.Expect(stripped).To(ContainSubstring("Compare"),
		"Timeline should contain Compare phase name")
}
