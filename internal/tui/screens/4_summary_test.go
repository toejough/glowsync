//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens_test

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

// TestSummaryScreen_ActivityLog_InRightColumnCancelledView verifies right column contains activity log in cancelled view
func TestSummaryScreen_ActivityLog_InRightColumnCancelledView(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 5
	engine.Status.TotalFiles = 10
	engine.Status.CancelledFiles = 5

	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Activity log section should be present in right column
	g.Expect(view).To(ContainSubstring("Activity"),
		"Should have activity log section in right column for cancelled view")
}

// ============================================================================
// Activity Log Integration Tests
// ============================================================================

// TestSummaryScreen_ActivityLog_InRightColumnCompleteView verifies right column contains activity log in complete view
func TestSummaryScreen_ActivityLog_InRightColumnCompleteView(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 10
	engine.Status.TotalFiles = 10

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Activity log section should be present in right column
	g.Expect(view).To(ContainSubstring("Activity"),
		"Should have activity log section in right column for complete view")
}

// TestSummaryScreen_ActivityLog_InRightColumnErrorView verifies right column contains activity log in error view
func TestSummaryScreen_ActivityLog_InRightColumnErrorView(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	fatalErr := errors.New("fatal error")

	screen := screens.NewSummaryScreen(engine, shared.StateError, fatalErr, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Activity log section should be present in right column
	g.Expect(view).To(ContainSubstring("Activity"),
		"Should have activity log section in right column for error view")
}

// TestSummaryScreen_CancelledView_ErrorsWidgetBox verifies "Errors" widget box (conditional)
func TestSummaryScreen_CancelledView_ErrorsWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 5
	engine.Status.TotalFiles = 10
	engine.Status.CancelledFiles = 5
	engine.Status.Errors = []syncengine.FileError{
		{FilePath: "/test/file1.txt", Error: errors.New("timeout")},
	}

	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Widget box should have "Errors" title when errors exist
	g.Expect(view).To(ContainSubstring("Errors"),
		"Should have Errors widget box title when errors exist")

	// Should contain error details
	g.Expect(view).To(ContainSubstring("file1.txt"),
		"Errors should show error file paths")
	g.Expect(view).To(ContainSubstring("timeout"),
		"Errors should show error messages")
}

// TestSummaryScreen_CancelledView_PartialSummary verifies partial summary
func TestSummaryScreen_CancelledView_PartialSummary(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 5
	engine.Status.TotalFiles = 10
	engine.Status.CancelledFiles = 3
	engine.Status.FailedFiles = 2
	engine.Status.TransferredBytes = 1024
	engine.Status.TotalBytes = 2048

	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Should show partial summary with all counts
	g.Expect(view).To(ContainSubstring("Files completed"),
		"Should show completed count")
	g.Expect(view).To(ContainSubstring("5 / 10"),
		"Should show completed/total ratio")
	g.Expect(view).To(ContainSubstring("Files cancelled"),
		"Should show cancelled count")
	g.Expect(view).To(ContainSubstring("3"),
		"Should show cancelled count value")
	g.Expect(view).To(ContainSubstring("Files failed"),
		"Should show failed count")
	g.Expect(view).To(ContainSubstring("2"),
		"Should show failed count value")
}

// TestSummaryScreen_CancelledView_StatisticsWidgetBox verifies "Statistics" widget box
func TestSummaryScreen_CancelledView_StatisticsWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 5
	engine.Status.TotalFiles = 10
	engine.Status.ActiveWorkers = 4
	engine.Status.AdaptiveMode = true
	engine.Status.MaxWorkers = 8
	engine.Status.Bottleneck = "destination"

	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Widget box should have "Statistics" title
	g.Expect(view).To(ContainSubstring("Statistics"),
		"Should have Statistics widget box title")

	// Should contain worker count with bottleneck info
	g.Expect(view).To(ContainSubstring("Workers"),
		"Statistics should show worker count")
	g.Expect(view).To(ContainSubstring("dest-limited"),
		"Statistics should show bottleneck info")
}

// TestSummaryScreen_CancelledView_StatisticsWithBottleneck verifies statistics with bottleneck info
func TestSummaryScreen_CancelledView_StatisticsWithBottleneck(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 5
	engine.Status.TotalFiles = 10
	engine.Status.AdaptiveMode = true
	engine.Status.ActiveWorkers = 4
	engine.Status.MaxWorkers = 8
	engine.Status.Bottleneck = "balanced"

	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Should show statistics with bottleneck info
	g.Expect(view).To(ContainSubstring("Statistics"),
		"Should show Statistics section")
	g.Expect(view).To(ContainSubstring("Workers"),
		"Should show worker count")
	g.Expect(view).To(ContainSubstring("balanced"),
		"Should show bottleneck info")
}

// ============================================================================
// Widget Boxes - Cancelled View
// ============================================================================

// TestSummaryScreen_CancelledView_SummaryWidgetBox verifies "Summary" widget box
func TestSummaryScreen_CancelledView_SummaryWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 5
	engine.Status.TotalFiles = 10
	engine.Status.TransferredBytes = 1024
	engine.Status.TotalBytes = 2048
	engine.Status.CancelledFiles = 3
	engine.Status.FailedFiles = 2

	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Widget box should have "Summary" title
	g.Expect(view).To(ContainSubstring("Summary"),
		"Should have Summary widget box title")

	// Should contain completed/cancelled/failed counts
	g.Expect(view).To(ContainSubstring("Files completed"),
		"Summary should show files completed")
	g.Expect(view).To(ContainSubstring("5"),
		"Summary should show completed count")
	g.Expect(view).To(ContainSubstring("Files cancelled"),
		"Summary should show cancelled count")
	g.Expect(view).To(ContainSubstring("3"),
		"Summary should show cancelled count value")
	g.Expect(view).To(ContainSubstring("Files failed"),
		"Summary should show failed count")
	g.Expect(view).To(ContainSubstring("2"),
		"Summary should show failed count value")
}

// ============================================================================
// Functional Preservation - Cancelled View
// ============================================================================

// TestSummaryScreen_CancelledView_WarningTitle verifies warning title
func TestSummaryScreen_CancelledView_WarningTitle(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 5
	engine.Status.TotalFiles = 10
	engine.Status.CancelledFiles = 5

	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Should show warning title
	g.Expect(view).To(ContainSubstring("Sync Cancelled"),
		"Should show 'Sync Cancelled' warning title")
}

// TestSummaryScreen_Common_ClickableLogPath verifies clickable log path at bottom
func TestSummaryScreen_Common_ClickableLogPath(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	logPath := "/tmp/test-debug.log"

	// Test in complete view
	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, logPath)
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())
	view := summaryScreen.View()
	g.Expect(view).To(ContainSubstring(logPath),
		"Complete view should show clickable log path")

	// Test in cancelled view
	screen = screens.NewSummaryScreen(engine, shared.StateCancelled, nil, logPath)
	updatedModel, _ = screen.Update(sizeMsg)
	summaryScreen, ok = updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())
	view = summaryScreen.View()
	g.Expect(view).To(ContainSubstring(logPath),
		"Cancelled view should show clickable log path")

	// Test in error view
	screen = screens.NewSummaryScreen(engine, shared.StateError, errors.New("error"), logPath)
	updatedModel, _ = screen.Update(sizeMsg)
	summaryScreen, ok = updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())
	view = summaryScreen.View()
	g.Expect(view).To(ContainSubstring(logPath),
		"Error view should show clickable log path")
}

// TestSummaryScreen_Common_HelpTextPresent verifies help text present in all views
func TestSummaryScreen_Common_HelpTextPresent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	logPath := "/tmp/test-debug.log"

	// Test in complete view
	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, logPath)
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())
	view := summaryScreen.View()
	g.Expect(view).To(ContainSubstring("Press Enter or q to exit"),
		"Complete view should show help text")
	g.Expect(view).To(ContainSubstring("Esc to start new session"),
		"Complete view should show Esc help text")

	// Test in cancelled view
	screen = screens.NewSummaryScreen(engine, shared.StateCancelled, nil, logPath)
	updatedModel, _ = screen.Update(sizeMsg)
	summaryScreen, ok = updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())
	view = summaryScreen.View()
	g.Expect(view).To(ContainSubstring("Press Enter or q to exit"),
		"Cancelled view should show help text")
	g.Expect(view).To(ContainSubstring("Esc to start new session"),
		"Cancelled view should show Esc help text")

	// Test in error view
	screen = screens.NewSummaryScreen(engine, shared.StateError, errors.New("error"), logPath)
	updatedModel, _ = screen.Update(sizeMsg)
	summaryScreen, ok = updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())
	view = summaryScreen.View()
	g.Expect(view).To(ContainSubstring("Press Enter or q to exit"),
		"Error view should show help text")
	g.Expect(view).To(ContainSubstring("Esc to start new session"),
		"Error view should show Esc help text")
}

// ============================================================================
// Functional Preservation - Common
// ============================================================================

// TestSummaryScreen_Common_KeyboardShortcuts verifies keyboard shortcuts
func TestSummaryScreen_Common_KeyboardShortcuts(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Test Enter key
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := screen.Update(enterMsg)
	g.Expect(cmd).ShouldNot(BeNil(), "Enter should return quit command")

	// Test 'q' key
	qMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd = screen.Update(qMsg)
	g.Expect(cmd).ShouldNot(BeNil(), "q should return quit command")

	// Test Esc key
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd = screen.Update(escMsg)
	g.Expect(cmd).ShouldNot(BeNil(), "Esc should return transition command")
	msg := cmd()
	g.Expect(msg).Should(BeAssignableToTypeOf(shared.TransitionToInputMsg{}),
		"Esc should send TransitionToInputMsg to start new session")

	// Test Ctrl+C key
	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd = screen.Update(ctrlCMsg)
	g.Expect(cmd).ShouldNot(BeNil(), "Ctrl+C should return quit command")
	msg = cmd()
	g.Expect(msg).Should(BeAssignableToTypeOf(tea.QuitMsg{}),
		"Ctrl+C should send tea.QuitMsg")
}

// TestSummaryScreen_Common_RingBellOnSuccess verifies ring bell on success
func TestSummaryScreen_Common_RingBellOnSuccess(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.FailedFiles = 0

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Call Init() which should trigger bell
	cmd := screen.Init()
	g.Expect(cmd).Should(BeNil(),
		"Init should not return command, just ring bell (fmt.Print)")
}

// TestSummaryScreen_CompleteView_AdaptiveStatsDisplay verifies adaptive stats display
func TestSummaryScreen_CompleteView_AdaptiveStatsDisplay(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 10
	engine.Status.AdaptiveMode = true
	engine.Status.MaxWorkers = 8
	engine.Status.TotalReadTime = 2 * time.Second
	engine.Status.TotalWriteTime = 3 * time.Second
	engine.Status.Bottleneck = "source"

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Should show adaptive concurrency stats
	g.Expect(view).To(ContainSubstring("Adaptive Concurrency"),
		"Should show Adaptive Concurrency section")
	g.Expect(view).To(ContainSubstring("Max workers used"),
		"Should show max workers stat")
	g.Expect(view).To(ContainSubstring("I/O breakdown"),
		"Should show I/O breakdown")
	g.Expect(view).To(ContainSubstring("source-limited"),
		"Should show bottleneck analysis")
}

// TestSummaryScreen_CompleteView_AdaptiveStatsWidgetBox verifies "Adaptive Stats" widget box (conditional)
func TestSummaryScreen_CompleteView_AdaptiveStatsWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 10
	engine.Status.TotalFiles = 10
	engine.Status.AdaptiveMode = true
	engine.Status.MaxWorkers = 8
	engine.Status.TotalReadTime = 2 * time.Second
	engine.Status.TotalWriteTime = 3 * time.Second
	engine.Status.Bottleneck = "source"

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Widget box should have "Adaptive Concurrency" title when adaptive mode used
	g.Expect(view).To(ContainSubstring("Adaptive Concurrency"),
		"Should have Adaptive Concurrency widget box title when adaptive mode used")

	// Should contain adaptive stats
	g.Expect(view).To(ContainSubstring("Max workers used"),
		"Adaptive Stats should show max workers")
	g.Expect(view).To(ContainSubstring("8"),
		"Adaptive Stats should show max workers value")
}

// TestSummaryScreen_CompleteView_EmptyStateMessage verifies empty state message
func TestSummaryScreen_CompleteView_EmptyStateMessage(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 0
	engine.Status.TotalFiles = 0

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Should show empty state message
	g.Expect(view).To(ContainSubstring("All files already up-to-date"),
		"Should show empty state message when no files synced")
}

// TestSummaryScreen_CompleteView_ErrorDisplayForPartialFailures verifies error display for partial failures
func TestSummaryScreen_CompleteView_ErrorDisplayForPartialFailures(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 8
	engine.Status.TotalFiles = 10
	engine.Status.FailedFiles = 2
	engine.Status.Errors = []syncengine.FileError{
		{FilePath: "/test/file1.txt", Error: errors.New("error 1")},
		{FilePath: "/test/file2.txt", Error: errors.New("error 2")},
	}

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Should show error title
	g.Expect(view).To(ContainSubstring("Complete with Errors"),
		"Should show 'Complete with Errors' title")

	// Should show errors
	g.Expect(view).To(ContainSubstring("Errors"),
		"Should show Errors section")
	g.Expect(view).To(ContainSubstring("file1.txt"),
		"Should display first error")
	g.Expect(view).To(ContainSubstring("file2.txt"),
		"Should display second error")
}

// TestSummaryScreen_CompleteView_ErrorsWidgetBox verifies "Errors" widget box (conditional)
func TestSummaryScreen_CompleteView_ErrorsWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 8
	engine.Status.TotalFiles = 10
	engine.Status.FailedFiles = 2
	engine.Status.Errors = []syncengine.FileError{
		{FilePath: "/test/file1.txt", Error: errors.New("permission denied")},
		{FilePath: "/test/file2.txt", Error: errors.New("disk full")},
	}

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Widget box should have "Errors" title when errors exist
	g.Expect(view).To(ContainSubstring("Errors"),
		"Should have Errors widget box title when errors exist")

	// Should contain error details
	g.Expect(view).To(ContainSubstring("file1.txt"),
		"Errors should show error file paths")
	g.Expect(view).To(ContainSubstring("permission denied"),
		"Errors should show error messages")
}

// TestSummaryScreen_CompleteView_RecentlyCompletedFilesList verifies recently completed files list
func TestSummaryScreen_CompleteView_RecentlyCompletedFilesList(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 10
	engine.Status.RecentlyCompleted = []string{
		"/test/file1.txt",
		"/test/file2.txt",
		"/test/file3.txt",
	}

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Should show recently completed files
	g.Expect(view).To(ContainSubstring("Recently Completed"),
		"Should show Recently Completed section")
	g.Expect(view).To(ContainSubstring("file1.txt"),
		"Should display first file")
	g.Expect(view).To(ContainSubstring("file2.txt"),
		"Should display second file")
	g.Expect(view).To(ContainSubstring("file3.txt"),
		"Should display third file")
}

// TestSummaryScreen_CompleteView_RecentlyCompletedWidgetBox verifies "Recently Completed" widget box (conditional)
func TestSummaryScreen_CompleteView_RecentlyCompletedWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 10
	engine.Status.TotalFiles = 10
	engine.Status.RecentlyCompleted = []string{
		"/test/file1.txt",
		"/test/file2.txt",
		"/test/file3.txt",
	}

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Widget box should have "Recently Completed" title when files present
	g.Expect(view).To(ContainSubstring("Recently Completed"),
		"Should have Recently Completed widget box title when files present")

	// Should contain file paths
	g.Expect(view).To(ContainSubstring("file1.txt"),
		"Recently Completed should show file paths")
	g.Expect(view).To(ContainSubstring("file2.txt"),
		"Recently Completed should show file paths")
}

// TestSummaryScreen_CompleteView_StatisticsWidgetBox verifies "Statistics" widget box
func TestSummaryScreen_CompleteView_StatisticsWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 10
	engine.Status.TotalFiles = 10
	engine.Status.ActiveWorkers = 4
	engine.Status.TransferredBytes = 2048

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Widget box should have "Statistics" title
	g.Expect(view).To(ContainSubstring("Statistics"),
		"Should have Statistics widget box title")

	// Should contain worker count
	g.Expect(view).To(ContainSubstring("Workers"),
		"Statistics should show worker count")
	g.Expect(view).To(ContainSubstring("4"),
		"Statistics should show worker count value")
}

// ============================================================================
// Functional Preservation - Complete View
// ============================================================================

// TestSummaryScreen_CompleteView_SuccessTitleWithStats verifies success title with stats
func TestSummaryScreen_CompleteView_SuccessTitleWithStats(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.StartTime = time.Now().Add(-5 * time.Second)
	engine.Status.EndTime = time.Now()
	engine.Status.ProcessedFiles = 10
	engine.Status.TotalFiles = 10
	engine.Status.TransferredBytes = 2048

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Should show celebratory success message with stats
	g.Expect(view).To(ContainSubstring("Successfully synchronized"),
		"Should show success title with stats")
	g.Expect(view).To(ContainSubstring("10"),
		"Success title should include file count")
	g.Expect(view).To(ContainSubstring("2.0 KB"),
		"Success title should include byte size")
}

// ============================================================================
// Widget Boxes - Complete View
// ============================================================================

// TestSummaryScreen_CompleteView_SummaryWidgetBox verifies "Summary" widget box
func TestSummaryScreen_CompleteView_SummaryWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.TotalFilesInSource = 15
	engine.Status.TotalBytesInSource = 4096
	engine.Status.AlreadySyncedFiles = 5
	engine.Status.AlreadySyncedBytes = 1024

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Widget box should have "Summary" title
	g.Expect(view).To(ContainSubstring("Summary"),
		"Should have Summary widget box title")

	// Should contain total files and already synced info
	g.Expect(view).To(ContainSubstring("Total files in source"),
		"Summary should show total files in source")
	g.Expect(view).To(ContainSubstring("15"),
		"Summary should show file count")
	g.Expect(view).To(ContainSubstring("Already up-to-date"),
		"Summary should show already synced files")
}

// TestSummaryScreen_CompleteView_ThisSessionWidgetBox verifies "This Session" widget box
func TestSummaryScreen_CompleteView_ThisSessionWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 10
	engine.Status.TotalFiles = 10
	engine.Status.TransferredBytes = 2048
	engine.Status.TotalBytes = 2048

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Widget box should have "This Session" title
	g.Expect(view).To(ContainSubstring("This Session"),
		"Should have This Session widget box title")

	// Should contain files synced and bytes transferred
	g.Expect(view).To(ContainSubstring("Files synced successfully"),
		"This Session should show files synced")
	g.Expect(view).To(ContainSubstring("10"),
		"This Session should show file count")
}

// TestSummaryScreen_ErrorView_AdditionalErrorsWidgetBox verifies "Additional Errors" widget box (conditional)
func TestSummaryScreen_ErrorView_AdditionalErrorsWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.Errors = []syncengine.FileError{
		{FilePath: "/test/file1.txt", Error: errors.New("error 1")},
		{FilePath: "/test/file2.txt", Error: errors.New("error 2")},
	}
	fatalErr := errors.New("fatal error")

	screen := screens.NewSummaryScreen(engine, shared.StateError, fatalErr, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Widget box should have "Additional Errors" title when errors exist
	g.Expect(view).To(ContainSubstring("Additional Errors"),
		"Should have Additional Errors widget box title when errors exist")

	// Should contain error details
	g.Expect(view).To(ContainSubstring("file1.txt"),
		"Additional Errors should show error file paths")
	g.Expect(view).To(ContainSubstring("file2.txt"),
		"Additional Errors should show error file paths")
}

// ============================================================================
// Widget Boxes - Error View
// ============================================================================

// TestSummaryScreen_ErrorView_ErrorDetailsWidgetBox verifies "Error Details" widget box
func TestSummaryScreen_ErrorView_ErrorDetailsWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	fatalErr := errors.New("permission denied")

	screen := screens.NewSummaryScreen(engine, shared.StateError, fatalErr, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Widget box should have "Error Details" title
	g.Expect(view).To(ContainSubstring("Error Details"),
		"Should have Error Details widget box title")

	// Should contain enriched error message
	g.Expect(view).To(ContainSubstring("permission denied"),
		"Error Details should show error message")

	// Should contain suggestions (enriched error)
	g.Expect(view).To(ContainSubstring("•"),
		"Error Details should show actionable suggestions with bullets")
}

// ============================================================================
// Functional Preservation - Error View
// ============================================================================

// TestSummaryScreen_ErrorView_ErrorTitleAndEnrichedMessage verifies error title and enriched message
func TestSummaryScreen_ErrorView_ErrorTitleAndEnrichedMessage(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	fatalErr := errors.New("permission denied")

	screen := screens.NewSummaryScreen(engine, shared.StateError, fatalErr, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Should show error title
	g.Expect(view).To(ContainSubstring("Sync Failed"),
		"Should show 'Sync Failed' error title")

	// Should show enriched error message
	g.Expect(view).To(ContainSubstring("permission denied"),
		"Should show error message")
}

// TestSummaryScreen_ErrorView_PartialProgressWhenFilesCompleted verifies partial progress when files completed
func TestSummaryScreen_ErrorView_PartialProgressWhenFilesCompleted(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 3
	engine.Status.TransferredBytes = 1024
	fatalErr := errors.New("fatal error")

	screen := screens.NewSummaryScreen(engine, shared.StateError, fatalErr, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Should show partial progress
	g.Expect(view).To(ContainSubstring("Partial Progress"),
		"Should show Partial Progress section")
	g.Expect(view).To(ContainSubstring("Files completed"),
		"Should show files completed in partial progress")
	g.Expect(view).To(ContainSubstring("3"),
		"Should show completed file count")
	g.Expect(view).To(ContainSubstring("Bytes transferred"),
		"Should show bytes transferred in partial progress")
	g.Expect(view).To(ContainSubstring("1.0 KB"),
		"Should show transferred bytes formatted")
}

// TestSummaryScreen_ErrorView_PartialProgressWidgetBox verifies "Partial Progress" widget box (conditional)
func TestSummaryScreen_ErrorView_PartialProgressWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 3
	engine.Status.TransferredBytes = 1024
	fatalErr := errors.New("fatal error")

	screen := screens.NewSummaryScreen(engine, shared.StateError, fatalErr, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Widget box should have "Partial Progress" title when progress made
	g.Expect(view).To(ContainSubstring("Partial Progress"),
		"Should have Partial Progress widget box title when files completed")

	// Should contain progress info
	g.Expect(view).To(ContainSubstring("Files completed"),
		"Partial Progress should show files completed")
	g.Expect(view).To(ContainSubstring("3"),
		"Partial Progress should show completed count")
	g.Expect(view).To(ContainSubstring("Bytes transferred"),
		"Partial Progress should show bytes transferred")
}

// TestSummaryScreen_ErrorView_SuggestionsDisplay verifies suggestions display
func TestSummaryScreen_ErrorView_SuggestionsDisplay(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	fatalErr := errors.New("permission denied")

	screen := screens.NewSummaryScreen(engine, shared.StateError, fatalErr, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// Should show actionable suggestions
	g.Expect(view).To(ContainSubstring("•"),
		"Should show bullet-formatted suggestions")
	g.Expect(view).To(Or(
		ContainSubstring("permissions"),
		ContainSubstring("ls -la"),
		ContainSubstring("privileged user"),
	), "Should show actionable suggestions for permission error")
}

// TestSummaryScreen_Layout_CancelledViewUsesTwoColumnLayout verifies cancelled view uses RenderTwoColumnLayout
func TestSummaryScreen_Layout_CancelledViewUsesTwoColumnLayout(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 5
	engine.Status.TotalFiles = 10
	engine.Status.CancelledFiles = 5
	engine.Status.TransferredBytes = 1024

	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// View should have content (two-column layout creates structure)
	g.Expect(view).NotTo(BeEmpty(), "View should render two-column layout")

	// Left column should have summary content
	g.Expect(view).To(ContainSubstring("Summary"),
		"Left column should show summary widget box")

	// Right column should have activity log
	g.Expect(view).To(ContainSubstring("Activity"),
		"Right column should show activity log")
}

// ============================================================================
// Two-Column Layout Tests
// ============================================================================

// TestSummaryScreen_Layout_CompleteViewUsesTwoColumnLayout verifies complete view uses RenderTwoColumnLayout
func TestSummaryScreen_Layout_CompleteViewUsesTwoColumnLayout(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 10
	engine.Status.TotalFiles = 10
	engine.Status.TransferredBytes = 2048

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// View should have content (two-column layout creates structure)
	g.Expect(view).NotTo(BeEmpty(), "View should render two-column layout")

	// Left column should have summary content
	g.Expect(view).To(ContainSubstring("Summary"),
		"Left column should show summary widget box")

	// Right column should have activity log
	g.Expect(view).To(ContainSubstring("Activity"),
		"Right column should show activity log")
}

// TestSummaryScreen_Layout_ErrorViewUsesTwoColumnLayout verifies error view uses RenderTwoColumnLayout
func TestSummaryScreen_Layout_ErrorViewUsesTwoColumnLayout(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	fatalErr := errors.New("fatal sync error")

	screen := screens.NewSummaryScreen(engine, shared.StateError, fatalErr, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()

	// View should have content (two-column layout creates structure)
	g.Expect(view).NotTo(BeEmpty(), "View should render two-column layout")

	// Left column should have error details
	g.Expect(view).To(ContainSubstring("Error Details"),
		"Left column should show error details widget box")

	// Right column should have activity log
	g.Expect(view).To(ContainSubstring("Activity"),
		"Right column should show activity log")
}

// TestSummaryScreen_Timeline_ShowsDoneErrorPhaseInCompleteViewWithErrors verifies timeline shows "done_error" phase
func TestSummaryScreen_Timeline_ShowsDoneErrorPhaseInCompleteViewWithErrors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 8
	engine.Status.TotalFiles = 10
	engine.Status.FailedFiles = 2
	engine.Status.Errors = []syncengine.FileError{
		{FilePath: "/test/file1.txt", Error: errors.New("error 1")},
		{FilePath: "/test/file2.txt", Error: errors.New("error 2")},
	}

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()
	stripped := stripANSI(view)

	// Timeline should show "Done" phase (with error styling if supported)
	g.Expect(stripped).To(ContainSubstring("Done"),
		"Timeline should show Done phase even with errors")
}

// TestSummaryScreen_Timeline_ShowsDoneErrorPhaseInErrorView verifies timeline shows "done_error" phase in error view
func TestSummaryScreen_Timeline_ShowsDoneErrorPhaseInErrorView(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	fatalErr := errors.New("fatal sync error")

	screen := screens.NewSummaryScreen(engine, shared.StateError, fatalErr, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()
	stripped := stripANSI(view)

	// Timeline should show "Done" phase (with error styling)
	g.Expect(stripped).To(ContainSubstring("Done"),
		"Timeline should show Done phase in error view")
}

// TestSummaryScreen_Timeline_ShowsDonePhaseInCancelledView verifies timeline shows done phase in cancelled view
func TestSummaryScreen_Timeline_ShowsDonePhaseInCancelledView(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 5
	engine.Status.TotalFiles = 10
	engine.Status.CancelledFiles = 5

	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()
	stripped := stripANSI(view)

	// Timeline should show "Done" phase
	g.Expect(stripped).To(ContainSubstring("Done"),
		"Timeline should show Done phase in cancelled view")
}

// ============================================================================
// Timeline Integration Tests
// ============================================================================

// TestSummaryScreen_Timeline_ShowsDonePhaseInCompleteView verifies timeline shows "done" phase in complete view
func TestSummaryScreen_Timeline_ShowsDonePhaseInCompleteView(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.ProcessedFiles = 10
	engine.Status.TotalFiles = 10

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	summaryScreen, ok := updatedModel.(screens.SummaryScreen)
	g.Expect(ok).To(BeTrue())

	view := summaryScreen.View()
	stripped := stripANSI(view)

	// Timeline should show "Done" phase
	g.Expect(stripped).To(ContainSubstring("Done"),
		"Timeline should show Done phase in complete view")
}
