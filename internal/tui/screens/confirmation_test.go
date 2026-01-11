//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens_test

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

func TestConfirmationScreen_Update_CtrlCKey(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create test engine and screen
	engine := mustNewEngine(t, "/test/source", "/test/dest")
	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Press Ctrl+C key
	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := screen.Update(ctrlCMsg)

	// Should return tea.Quit command
	g.Expect(cmd).ShouldNot(BeNil(), "Ctrl+C should return a quit command")

	// Execute the command to verify it's tea.Quit
	msg := cmd()
	g.Expect(msg).Should(BeAssignableToTypeOf(tea.QuitMsg{}),
		"Ctrl+C should send tea.QuitMsg")
}

func TestConfirmationScreen_Update_EnterKey(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create test engine and screen
	engine := mustNewEngine(t, "/test/source", "/test/dest")
	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Press Enter key
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := screen.Update(enterMsg)

	// Should return a command that sends ConfirmSyncMsg
	g.Expect(cmd).ShouldNot(BeNil(), "Enter key should return a command")

	// Execute the command to get the message
	msg := cmd()
	g.Expect(msg).Should(BeAssignableToTypeOf(shared.ConfirmSyncMsg{}),
		"Enter key should send ConfirmSyncMsg")
}

func TestConfirmationScreen_Update_EscapeKey(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create test engine and screen
	engine := mustNewEngine(t, "/test/source", "/test/dest")
	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Press Esc key
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := screen.Update(escMsg)

	// Should return a command that sends TransitionToInputMsg
	g.Expect(cmd).ShouldNot(BeNil(), "Esc key should return a command")

	// Execute the command to get the message
	msg := cmd()
	g.Expect(msg).Should(BeAssignableToTypeOf(shared.TransitionToInputMsg{}),
		"Esc key should send TransitionToInputMsg")
}

func TestConfirmationScreen_View(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a test engine
	engine := mustNewEngine(t, "/test/source", "/test/dest")
	logPath := "/tmp/test-debug.log"

	// Create confirmation screen
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Get the view output
	output := screen.View()

	// Verify output contains expected elements
	// Note: "Files to sync" removed - now shown in analysis screen as "To copy: N files"
	g.Expect(output).Should(ContainSubstring("Press Enter to start"), "Expected help text for Enter")
	g.Expect(output).Should(ContainSubstring("Esc to cancel"), "Expected help text for Esc")
}

func TestConfirmationScreen_View_EmptyNoFilter(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create engine with no filter and no files (source empty or already synced)
	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.FilePattern = ""
	engine.Status.TotalFiles = 0
	engine.Status.TotalBytes = 0

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	output := screen.View()

	// Should show context-aware empty message
	// Could be "Source directory is empty" or "All files already synced"
	// For now, we'll test for a generic helpful message
	g.Expect(output).Should(Or(
		ContainSubstring("Source directory is empty"),
		ContainSubstring("All files already synced"),
	), "Should show helpful empty state message")
}

func TestConfirmationScreen_View_EmptyState_PreservesControls(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create engine with no files
	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.Status.TotalFiles = 0

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	output := screen.View()

	// Should still show help text even when empty
	g.Expect(output).Should(ContainSubstring("Press Enter to start"),
		"Should show Enter key help")
	g.Expect(output).Should(ContainSubstring("Esc to cancel"),
		"Should show Esc key help")
}

func TestConfirmationScreen_View_EmptyWithFilter(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create engine with filter applied but no matching files
	engine := mustNewEngine(t, "/test/source", "/test/dest")
	engine.FilePattern = "*.mov"
	engine.Status.TotalFiles = 0
	engine.Status.TotalBytes = 0

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	output := screen.View()

	// Should show filter-specific message
	g.Expect(output).Should(ContainSubstring("No files match your filter"),
		"Should show filter-specific empty message")
	g.Expect(output).Should(ContainSubstring("*.mov"),
		"Should show the filter pattern")
}

func TestConfirmationScreen_View_ErrorDisplayLimit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a test engine with more than 3 errors
	engine := mustNewEngine(t, "/test/source", "/test/dest")

	// Simulate 6 errors during analysis
	// Need to modify engine.Status.Errors directly (not the copy from GetStatus())
	engine.Status.Errors = []syncengine.FileError{
		{FilePath: "/path/to/file1.txt", Error: errors.New("error 1")},
		{FilePath: "/path/to/file2.txt", Error: errors.New("error 2")},
		{FilePath: "/path/to/file3.txt", Error: errors.New("error 3")},
		{FilePath: "/path/to/file4.txt", Error: errors.New("error 4")},
		{FilePath: "/path/to/file5.txt", Error: errors.New("error 5")},
		{FilePath: "/path/to/file6.txt", Error: errors.New("error 6")},
	}

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	output := screen.View()

	// Should show first 3 errors
	g.Expect(output).Should(ContainSubstring("error 1"), "Should show first error")
	g.Expect(output).Should(ContainSubstring("error 2"), "Should show second error")
	g.Expect(output).Should(ContainSubstring("error 3"), "Should show third error")

	// Should NOT show 4th error and beyond
	g.Expect(output).ShouldNot(ContainSubstring("error 4"), "Should not show fourth error")
	g.Expect(output).ShouldNot(ContainSubstring("error 5"), "Should not show fifth error")
	g.Expect(output).ShouldNot(ContainSubstring("error 6"), "Should not show sixth error")

	// Should show truncation message with "see summary"
	g.Expect(output).Should(ContainSubstring("... and 3 more (see summary)"),
		"Should show truncation message pointing to summary")
}

func TestConfirmationScreen_View_WithErrors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a test engine with errors from analysis phase
	engine := mustNewEngine(t, "/test/source", "/test/dest")

	// Simulate errors that occurred during analysis
	// Need to modify engine.Status.Errors directly (not the copy from GetStatus())
	engine.Status.Errors = []syncengine.FileError{
		{FilePath: "/path/to/file1.txt", Error: errors.New("error 1")},
		{FilePath: "/path/to/file2.txt", Error: errors.New("error 2")},
	}

	logPath := "/tmp/test-debug.log"
	screen := screens.NewConfirmationScreen(engine, logPath)

	output := screen.View()

	// Should display errors during analysis
	g.Expect(output).Should(ContainSubstring("error 1"), "Should show first error")
	g.Expect(output).Should(ContainSubstring("error 2"), "Should show second error")
}

func TestNewConfirmationScreen(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a test engine
	engine := mustNewEngine(t, "/test/source", "/test/dest")
	logPath := "/tmp/test-debug.log"

	// Create confirmation screen
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Verify screen is created
	g.Expect(screen).ShouldNot(BeNil(), "Expected screen to be created")
}

// mustNewEngine creates a new engine and fails the test if there's an error
func mustNewEngine(t *testing.T, source, dest string) *syncengine.Engine {
	t.Helper()
	engine, err := syncengine.NewEngine(source, dest)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	return engine
}
