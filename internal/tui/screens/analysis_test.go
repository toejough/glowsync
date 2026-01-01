//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens_test

import (
	"errors"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

func TestAnalysisScreenAnalysisComplete(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Send AnalysisCompleteMsg
	msg := shared.AnalysisCompleteMsg{}

	updatedModel, cmd := screen.Update(msg)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
	g.Expect(cmd).ShouldNot(BeNil())
}

func TestAnalysisScreenCtrlCQuitsApp(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

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

func TestAnalysisScreenEngineInitialized(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
		Workers:    4,
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Create an engine
	engine := mustNewEngine(t, "/source", "/dest")

	// Send EngineInitializedMsg
	msg := shared.EngineInitializedMsg{
		Engine: engine,
	}

	updatedModel, cmd := screen.Update(msg)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
	g.Expect(cmd).ShouldNot(BeNil())

	// Verify view after engine initialization
	analysisScreen, ok := updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).Should(BeTrue())

	view := analysisScreen.View()
	g.Expect(view).ShouldNot(BeEmpty())
}

func TestAnalysisScreenError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Send ErrorMsg
	msg := shared.ErrorMsg{
		Err: errors.New("test error"),
	}

	updatedModel, cmd := screen.Update(msg)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
	g.Expect(cmd).ShouldNot(BeNil())
}

func TestAnalysisScreenEscKeyReturnsToInput(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Press Esc key
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := screen.Update(escMsg)

	// Should return a command that sends TransitionToInputMsg
	g.Expect(cmd).ShouldNot(BeNil(), "Esc key should return a transition command")

	// Execute the command to get the message
	msg := cmd()
	g.Expect(msg).Should(BeAssignableToTypeOf(shared.TransitionToInputMsg{}),
		"Esc key should send TransitionToInputMsg")
}

func TestAnalysisScreenNew(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	g.Expect(screen).ShouldNot(BeNil())

	// Call Init to ensure coverage
	cmd := screen.Init()
	g.Expect(cmd).ShouldNot(BeNil())
}

func TestAnalysisScreenRenderingWithStatus(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Initialize with engine
	engine := mustNewEngine(t, "/source", "/dest")
	initMsg := shared.EngineInitializedMsg{Engine: engine}
	updatedModel, _ := screen.Update(initMsg)

	analysisScreen, ok := updatedModel.(screens.AnalysisScreen)
	g := NewWithT(t)
	g.Expect(ok).Should(BeTrue())

	screen = &analysisScreen

	// Get the view which should now show analysis state
	view := screen.View()
	g.Expect(view).Should(ContainSubstring("Analyzing Files"))
}

func TestAnalysisScreenSpinnerTick(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Send spinner.TickMsg
	msg := spinner.TickMsg{
		Time: time.Now(),
		ID:   1,
	}

	updatedModel, _ := screen.Update(msg)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

func TestAnalysisScreenTickMsg(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Initialize the screen with an engine first
	engine := mustNewEngine(t, "/source", "/dest")
	initMsg := shared.EngineInitializedMsg{Engine: engine}
	updatedModel, _ := screen.Update(initMsg)

	analysisScreen, ok := updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).Should(BeTrue())

	screen = &analysisScreen

	// Create a tickMsg using a time.Time value
	// We need to use the actual type from the screens package
	// Since tickMsg is unexported, we can't create it directly
	// But we can test the Update method with nil which will exercise other paths
	updatedModel, _ = screen.Update(nil)
	g.Expect(updatedModel).ShouldNot(BeNil())
}

func TestAnalysisScreenView(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Test View rendering in initializing state
	view := screen.View()
	g.Expect(view).Should(ContainSubstring("Starting Copy Files"))

	// Initialize the screen
	_ = screen.Init()

	// Update to trigger state change
	view = screen.View()
	g.Expect(view).ShouldNot(BeEmpty())
}

func TestAnalysisScreenWindowSize(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Send WindowSizeMsg
	msg := tea.WindowSizeMsg{
		Width:  100,
		Height: 50,
	}

	updatedModel, _ := screen.Update(msg)
	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())
}
