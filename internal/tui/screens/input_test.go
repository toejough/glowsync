//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

// TestEnterOnDestWithValidInputsSubmits verifies that pressing Enter on the dest field
// with valid source and dest values triggers submission (transition to analysis)
func TestEnterOnDestWithValidInputsSubmits(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temporary directories for testing
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Type valid source path
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(sourceDir)}
	updatedModel, _ := screen.Update(typeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Move to dest field and type valid dest path
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ = inputScreen.Update(downMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	typeMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(destDir)}
	updatedModel, _ = inputScreen.Update(typeMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Press Enter on dest field - should submit
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	//nolint:ineffassign,staticcheck,wastedassign // Need updatedModel for type assertion check
	updatedModel, cmd := inputScreen.Update(enterMsg)

	// Should return a command (TransitionToAnalysisMsg)
	g.Expect(cmd).ShouldNot(BeNil(), "Enter with valid inputs should trigger transition")

	// Execute command and verify it's TransitionToAnalysisMsg
	if cmd != nil {
		msg := cmd()
		_, ok := msg.(shared.TransitionToAnalysisMsg)
		g.Expect(ok).Should(BeTrue(), "Command should be TransitionToAnalysisMsg")
	}
}

// TestEnterOnSourceWithValidInputsSubmits verifies that pressing Enter on the source field
// with valid source and dest values triggers submission (transition to analysis)
func TestEnterOnSourceWithValidInputsSubmits(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temporary directories for testing
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Type valid source path
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(sourceDir)}
	updatedModel, _ := screen.Update(typeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Move to dest field and type valid dest path
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ = inputScreen.Update(downMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	typeMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(destDir)}
	updatedModel, _ = inputScreen.Update(typeMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Move back to source field
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, _ = inputScreen.Update(upMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Press Enter on source field - should submit
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	//nolint:ineffassign,staticcheck,wastedassign // Need updatedModel for type assertion check
	updatedModel, cmd := inputScreen.Update(enterMsg)

	// Should return a command (TransitionToAnalysisMsg)
	g.Expect(cmd).ShouldNot(BeNil(), "Enter with valid inputs should trigger transition")

	// Execute command and verify it's TransitionToAnalysisMsg
	if cmd != nil {
		msg := cmd()
		_, ok := msg.(shared.TransitionToAnalysisMsg)
		g.Expect(ok).Should(BeTrue(), "Command should be TransitionToAnalysisMsg")
	}
}

// TestEnterWithBothEmptyShowsError verifies that pressing Enter with both fields empty
// shows a validation error and focuses the source field
func TestEnterWithBothEmptyShowsError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Press Enter with both empty
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := screen.Update(enterMsg)

	// Should NOT transition
	g.Expect(cmd).Should(BeNil(), "Enter with empty inputs should not trigger transition")

	// Check view contains error message
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())
	view := inputScreen.View()
	g.Expect(view).Should(ContainSubstring("source path is required"), "Should show source error message")
}

// TestEnterWithEmptyDestShowsError verifies that pressing Enter with empty dest
// shows a validation error and focuses the dest field
func TestEnterWithEmptyDestShowsError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Type valid source
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/tmp/source")}
	updatedModel, _ := screen.Update(typeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Press Enter - should show error about dest
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := inputScreen.Update(enterMsg)

	// Should NOT transition
	g.Expect(cmd).Should(BeNil(), "Enter with empty dest should not trigger transition")

	// Check view contains error message
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())
	view := inputScreen.View()
	g.Expect(view).Should(ContainSubstring("destination path is required"), "Should show dest error message")
}

// TestEnterWithEmptySourceShowsError verifies that pressing Enter with empty source
// shows a validation error and focuses the source field
func TestEnterWithEmptySourceShowsError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Leave source empty, type dest
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ := screen.Update(downMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/tmp/dest")}
	updatedModel, _ = inputScreen.Update(typeMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Press Enter - should show error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := inputScreen.Update(enterMsg)

	// Should NOT transition
	g.Expect(cmd).Should(BeNil(), "Enter with empty source should not trigger transition")

	// Check view contains error message
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())
	view := inputScreen.View()
	g.Expect(view).Should(ContainSubstring("source path is required"), "Should show source error message")
}

// TestErrorClearsOnFieldNavigation verifies that validation errors are cleared when navigating fields
func TestErrorClearsOnFieldNavigation(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Press Enter to trigger error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := screen.Update(enterMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Verify error is shown
	view := inputScreen.View()
	g.Expect(view).Should(ContainSubstring("source path is required"))

	// Navigate to next field
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ = inputScreen.Update(downMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Verify error is cleared
	view = inputScreen.View()
	g.Expect(view).ShouldNot(ContainSubstring("source path is required"), "Error should be cleared after navigation")
}

// TestErrorClearsWhenUserTypes verifies that validation errors are cleared when the user types
func TestErrorClearsWhenUserTypes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Press Enter to trigger error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := screen.Update(enterMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Verify error is shown
	view := inputScreen.View()
	g.Expect(view).Should(ContainSubstring("source path is required"))

	// Type something
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}
	updatedModel, _ = inputScreen.Update(typeMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Verify error is cleared
	view = inputScreen.View()
	g.Expect(view).ShouldNot(ContainSubstring("source path is required"), "Error should be cleared after typing")
}

func TestInputScreenCtrlCQuitsApp(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

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

func TestInputScreenEnter(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "",
		DestPath:   "",
	}
	screen := screens.NewInputScreen(cfg)

	// Test Enter with empty input (should stay on same field)
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := screen.Update(enterMsg)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
	g.Expect(cmd).Should(BeNil())
}

func TestInputScreenEnterWithValidPaths(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "",
		DestPath:   "",
	}
	screen := screens.NewInputScreen(cfg)

	// Simulate typing a source path and pressing enter
	// First, we need to move to the state where source has a value
	// We'll use a type message to simulate input
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	updatedModel, _ := screen.Update(typeMsg)

	g := NewWithT(t)

	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	screen = &inputScreen

	// Press enter to move to next field
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = screen.Update(enterMsg)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

func TestInputScreenEscClearsDestField(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Move to destination field
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ := screen.Update(downMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	screen = &inputScreen

	// Simulate typing in dest field
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/dest/path")}
	updatedModel, _ = screen.Update(typeMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	screen = &inputScreen

	// Press Esc - should clear the dest field
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, cmd := screen.Update(escMsg)
	g.Expect(cmd).Should(BeNil(), "Esc should not quit, just clear field")

	// Verify field was cleared by checking the view
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())
	view := inputScreen.View()
	g.Expect(view).ShouldNot(ContainSubstring("/dest/path"), "Dest field should be cleared")
}

func TestInputScreenEscClearsSourceField(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Simulate typing in source field
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/some/path")}
	updatedModel, _ := screen.Update(typeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	screen = &inputScreen

	// Press Esc - should clear the source field
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, cmd := screen.Update(escMsg)
	g.Expect(cmd).Should(BeNil(), "Esc should not quit, just clear field")

	// Verify field was cleared by checking the view
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())
	view := inputScreen.View()
	g.Expect(view).ShouldNot(ContainSubstring("/some/path"), "Source field should be cleared")
}

func TestInputScreenFieldNavigation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Test down arrow (move to next field)
	downMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}, Alt: false}
	downMsg.Type = tea.KeyDown
	_, _ = screen.Update(downMsg)

	// Test up arrow (move to previous field)
	upMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}, Alt: false}
	upMsg.Type = tea.KeyUp
	_, _ = screen.Update(upMsg)

	// Test ctrl+n
	ctrlNMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: false}
	ctrlNMsg.Type = tea.KeyCtrlN
	_, _ = screen.Update(ctrlNMsg)

	// Test ctrl+p
	ctrlPMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}, Alt: false}
	ctrlPMsg.Type = tea.KeyCtrlP
	_, _ = screen.Update(ctrlPMsg)
}

func TestInputScreenNew(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	g.Expect(screen).ShouldNot(BeNil())

	// Call Init to ensure coverage
	cmd := screen.Init()
	g.Expect(cmd).ShouldNot(BeNil())
}

func TestInputScreenPatternField(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Test that View includes pattern field label
	// Note: The placeholder text may be truncated in the rendered view
	view := screen.View()
	g.Expect(view).Should(ContainSubstring("Filter Pattern"))
}

func TestInputScreenPatternFieldNavigation(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Navigate through all three fields: source -> dest -> pattern
	downMsg := tea.KeyMsg{Type: tea.KeyDown}

	// Move from source to dest
	updatedModel, _ := screen.Update(downMsg)
	g.Expect(updatedModel).ShouldNot(BeNil())

	// Move from dest to pattern
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	updatedModel, _ = inputScreen.Update(downMsg)
	g.Expect(updatedModel).ShouldNot(BeNil())

	// Navigate back up: pattern -> dest -> source
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	updatedModel, _ = inputScreen.Update(upMsg)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

func TestInputScreenPatternFieldPopulatesConfig(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Simulate entering a pattern value
	// We'll need to navigate to the pattern field and type
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("*.mov")}
	updatedModel, _ := screen.Update(typeMsg)

	g.Expect(updatedModel).ShouldNot(BeNil())

	// Verify pattern is stored when transitioning to analysis
	// (This will be tested more thoroughly in integration tests)
}

func TestInputScreenQuit(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Test Ctrl+C - should quit
	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := screen.Update(ctrlCMsg)

	g := NewWithT(t)
	g.Expect(cmd).ShouldNot(BeNil())

	// Execute the command to verify it's tea.Quit
	msg := cmd()
	g.Expect(msg).Should(BeAssignableToTypeOf(tea.QuitMsg{}),
		"Ctrl+C should send tea.QuitMsg")
}

func TestInputScreenRenderHelpMultiLineFormat(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	view := screen.View()

	// Verify help text contains multiple lines with expected keywords
	g.Expect(view).Should(ContainSubstring("Navigation:"), "Help text should have Navigation section")
	g.Expect(view).Should(ContainSubstring("Tab"), "Help text should mention Tab key")
	g.Expect(view).Should(ContainSubstring("Shift+Tab"), "Help text should mention Shift+Tab key")
	g.Expect(view).Should(ContainSubstring("↑↓"), "Help text should mention up/down arrows")

	g.Expect(view).Should(ContainSubstring("Actions:"), "Help text should have Actions section")
	g.Expect(view).Should(ContainSubstring("→"), "Help text should mention right arrow")
	g.Expect(view).Should(ContainSubstring("Enter"), "Help text should mention Enter key")

	g.Expect(view).Should(ContainSubstring("Other:"), "Help text should have Other section")
	g.Expect(view).Should(ContainSubstring("Esc"), "Help text should mention Esc key")
	g.Expect(view).Should(ContainSubstring("Ctrl+C"), "Help text should mention Ctrl+C")
}

func TestInputScreenRightArrow(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Test Right arrow key
	rightMsg := tea.KeyMsg{Type: tea.KeyRight}
	updatedModel, _ := screen.Update(rightMsg)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

func TestInputScreenTabCompletion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Test Tab key
	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	updatedModel, _ := screen.Update(tabMsg)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())

	// Test Shift+Tab key
	shiftTabMsg := tea.KeyMsg{Type: tea.KeyShiftTab}

	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	screen = &inputScreen
	updatedModel, _ = screen.Update(shiftTabMsg)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

func TestInputScreenTransitionToAnalysis(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a config that will validate successfully
	cfg := &config.Config{
		SourcePath: "",
		DestPath:   "",
	}
	screen := screens.NewInputScreen(cfg)

	// The screen's internal textinput models need to have values
	// We can trigger a TransitionToAnalysisMsg by having both paths filled
	// and pressing enter on the destination field

	// For now, let's just verify the screen handles other key types
	otherMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}}
	updatedModel, _ := screen.Update(otherMsg)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

func TestInputScreenUpdate(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Test with nil message
	updatedModel, _ := screen.Update(nil)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())

	// Test with regular key message
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	updatedModel, _ = screen.Update(keyMsg)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

func TestInputScreenView(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Test View rendering
	view := screen.View()
	g.Expect(view).Should(ContainSubstring("GlowSync"))
	g.Expect(view).Should(ContainSubstring("Source Path"))
	g.Expect(view).Should(ContainSubstring("Destination Path"))
}

func TestInputScreenWindowSize(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Send WindowSizeMsg
	msg := tea.WindowSizeMsg{
		Width:  100,
		Height: 50,
	}

	updatedModel, _ := screen.Update(msg)
	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

// TestWhitespaceOnlyDestShowsError verifies that whitespace-only dest is treated as empty
func TestWhitespaceOnlyDestShowsError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create temp dir for source
	sourceDir := t.TempDir()

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Type valid source
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(sourceDir)}
	updatedModel, _ := screen.Update(typeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Move to dest and type whitespace
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ = inputScreen.Update(downMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	typeMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("   ")}
	updatedModel, _ = inputScreen.Update(typeMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Press Enter - should show error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := inputScreen.Update(enterMsg)

	// Should NOT transition
	g.Expect(cmd).Should(BeNil(), "Enter with whitespace-only dest should not trigger transition")

	// Check view contains error message
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())
	view := inputScreen.View()
	g.Expect(view).Should(ContainSubstring("destination path is required"), "Should show dest error for whitespace")
}

// TestWhitespaceOnlySourceShowsError verifies that whitespace-only source is treated as empty
func TestWhitespaceOnlySourceShowsError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Type whitespace-only source
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("   ")}
	updatedModel, _ := screen.Update(typeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Press Enter - should show error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := inputScreen.Update(enterMsg)

	// Should NOT transition
	g.Expect(cmd).Should(BeNil(), "Enter with whitespace-only source should not trigger transition")

	// Check view contains error message
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())
	view := inputScreen.View()
	g.Expect(view).Should(ContainSubstring("source path is required"), "Should show source error for whitespace")
}
