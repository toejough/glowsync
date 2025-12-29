//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

func TestConfirmationScreen_Update_EnterKey(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create test engine and screen
	engine := syncengine.NewEngine("/test/source", "/test/dest")
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
	engine := syncengine.NewEngine("/test/source", "/test/dest")
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
	engine := syncengine.NewEngine("/test/source", "/test/dest")
	logPath := "/tmp/test-debug.log"

	// Create confirmation screen
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Get the view output
	output := screen.View()

	// Verify output contains expected elements
	g.Expect(output).Should(ContainSubstring("Analysis Complete"), "Expected title to be present")
	g.Expect(output).Should(ContainSubstring("Press Enter to begin sync"), "Expected help text for Enter")
	g.Expect(output).Should(ContainSubstring("Esc to cancel"), "Expected help text for Esc")
}

func TestNewConfirmationScreen(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a test engine
	engine := syncengine.NewEngine("/test/source", "/test/dest")
	logPath := "/tmp/test-debug.log"

	// Create confirmation screen
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Verify screen is created
	g.Expect(screen).ShouldNot(BeNil(), "Expected screen to be created")
}

func TestConfirmationScreen_Update_CtrlCKey(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create test engine and screen
	engine := syncengine.NewEngine("/test/source", "/test/dest")
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

func TestConfirmationScreen_View_WithFilterPattern(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a test engine with filter pattern
	engine := syncengine.NewEngine("/test/source", "/test/dest")
	engine.FilePattern = "*.mov"
	logPath := "/tmp/test-debug.log"

	// Create confirmation screen
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Get the view output
	output := screen.View()

	// Verify output contains filter indicator
	g.Expect(output).Should(ContainSubstring("Filtering by:"), "Expected filter label to be present")
	g.Expect(output).Should(ContainSubstring("*.mov"), "Expected filter pattern to be displayed")
}

func TestConfirmationScreen_View_WithoutFilterPattern(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a test engine without filter pattern
	engine := syncengine.NewEngine("/test/source", "/test/dest")
	engine.FilePattern = ""
	logPath := "/tmp/test-debug.log"

	// Create confirmation screen
	screen := screens.NewConfirmationScreen(engine, logPath)

	// Get the view output
	output := screen.View()

	// Verify output does NOT contain filter indicator
	g.Expect(output).ShouldNot(ContainSubstring("Filtering by:"), "Expected no filter label when pattern is empty")
}
