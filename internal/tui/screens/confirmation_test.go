package screens_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega"

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
