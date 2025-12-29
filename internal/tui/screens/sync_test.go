//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens_test

import (
	"errors"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner" //nolint:depguard // Needed for TUI testing
	tea "github.com/charmbracelet/bubbletea"   //nolint:depguard // Needed for TUI testing
	. "github.com/onsi/gomega"                 //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

func TestSyncScreenCancelled(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSyncScreen(engine)

	// Cancel the sync
	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updatedModel, _ := screen.Update(ctrlCMsg)

	g := NewWithT(t)

	syncScreen, ok := updatedModel.(screens.SyncScreen)
	g.Expect(ok).Should(BeTrue())

	screen = &syncScreen

	// Then complete it
	completeMsg := shared.SyncCompleteMsg{}
	updatedModel, cmd := screen.Update(completeMsg)

	g.Expect(updatedModel).ShouldNot(BeNil())
	g.Expect(cmd).ShouldNot(BeNil())
}

func TestSyncScreenError(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSyncScreen(engine)

	// Send ErrorMsg
	msg := shared.ErrorMsg{
		Err: errors.New("test error"),
	}

	updatedModel, cmd := screen.Update(msg)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
	g.Expect(cmd).ShouldNot(BeNil())
}

func TestSyncScreenKeyMsg(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSyncScreen(engine)

	// Test Ctrl+C
	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updatedModel, _ := screen.Update(ctrlCMsg)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())

	// Test 'q' key
	qMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	updatedModel, _ = screen.Update(qMsg)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

func TestSyncScreenNew(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSyncScreen(engine)

	g.Expect(screen).ShouldNot(BeNil())

	// Call Init to ensure coverage
	cmd := screen.Init()
	g.Expect(cmd).ShouldNot(BeNil())
}

func TestSyncScreenSpinnerTick(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSyncScreen(engine)

	// Send spinner.TickMsg
	msg := spinner.TickMsg{
		Time: time.Now(),
		ID:   1,
	}

	updatedModel, _ := screen.Update(msg)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

func TestSyncScreenSyncComplete(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSyncScreen(engine)

	// Send SyncCompleteMsg
	msg := shared.SyncCompleteMsg{}

	updatedModel, cmd := screen.Update(msg)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
	g.Expect(cmd).ShouldNot(BeNil())
}

func TestSyncScreenUpdate(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSyncScreen(engine)

	// Test with nil message
	updatedModel, _ := screen.Update(nil)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

func TestSyncScreenUsesSymbolHelpers(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Verify that symbol helper functions are available and return non-empty strings
	// The actual rendering with errors is tested through integration tests
	g.Expect(shared.ErrorSymbol()).ShouldNot(BeEmpty())
	g.Expect(shared.SuccessSymbol()).ShouldNot(BeEmpty())
	g.Expect(shared.PendingSymbol()).ShouldNot(BeEmpty())
}

func TestSyncScreenView(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSyncScreen(engine)

	// Test View rendering
	view := screen.View()
	g.Expect(view).Should(ContainSubstring("Syncing Files"))
}

func TestSyncScreenViewWithStatus(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSyncScreen(engine)

	// Initialize to trigger status updates
	_ = screen.Init()

	// Update with WindowSizeMsg to set dimensions for rendering
	sizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := screen.Update(sizeMsg)

	g := NewWithT(t)

	syncScreen, ok := updatedModel.(screens.SyncScreen)
	g.Expect(ok).Should(BeTrue())

	screen = &syncScreen

	// Get view which exercises rendering functions
	view := screen.View()
	g.Expect(view).ShouldNot(BeEmpty())
}

func TestSyncScreenWindowSize(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSyncScreen(engine)

	// Send WindowSizeMsg
	msg := tea.WindowSizeMsg{
		Width:  100,
		Height: 50,
	}

	updatedModel, _ := screen.Update(msg)
	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

func TestSyncScreenEscCancelsAndTransitionsToSummary(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSyncScreen(engine)

	// Press Esc key - should cancel and mark as cancelled
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ := screen.Update(escMsg)

	syncScreen, ok := updatedModel.(screens.SyncScreen)
	g.Expect(ok).Should(BeTrue())

	screen = &syncScreen

	// Then complete the sync
	completeMsg := shared.SyncCompleteMsg{}
	_, cmd := screen.Update(completeMsg)

	g.Expect(cmd).ShouldNot(BeNil(), "Esc should trigger cancellation and transition to summary")

	// Execute the command to get the transition message
	msg := cmd()
	transitionMsg, ok := msg.(shared.TransitionToSummaryMsg)
	g.Expect(ok).Should(BeTrue(), "Should transition to summary screen")
	g.Expect(transitionMsg.FinalState).Should(Equal(shared.StateCancelled),
		"Final state should be cancelled")
}

func TestSyncScreenCtrlCQuitsApp(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSyncScreen(engine)

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
