//nolint:varnamelen // Test files use idiomatic short variable names (ok, etc.)
package tui_test

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

func TestAppModelStoresLogPath(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		InteractiveMode: true,
	}

	model := tui.NewAppModel(cfg)

	// Create an engine for the transition
	engine := mustNewEngine(t, "/source", "/dest")

	// Send TransitionToSyncMsg with log path
	msg := shared.TransitionToSyncMsg{
		Engine:  engine,
		LogPath: "/tmp/test-debug.log",
	}

	updatedModel, _ := model.Update(msg)

	appModel, ok := updatedModel.(tui.AppModel)
	g.Expect(ok).Should(BeTrue(), "Expected updatedModel to be AppModel")

	// Verify the log path was stored (we'll need a getter method)
	g.Expect(appModel.LogPath()).Should(Equal("/tmp/test-debug.log"))
}

func TestAppModelTransitionToAnalysis(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		InteractiveMode: true,
	}

	model := tui.NewAppModel(cfg)

	// Send TransitionToAnalysisMsg to trigger transition
	msg := shared.TransitionToAnalysisMsg{
		SourcePath: "/test/source",
		DestPath:   "/test/dest",
	}

	updatedModel, _ := model.Update(msg)

	g := NewWithT(t)

	appModel, ok := updatedModel.(tui.AppModel)
	g.Expect(ok).Should(BeTrue(), "Expected updatedModel to be AppModel")

	model = &appModel

	// Verify we transitioned to AnalysisScreen
	_, isAnalysisScreen := model.CurrentScreen().(*screens.AnalysisScreen)
	g.Expect(isAnalysisScreen).Should(BeTrue(), "Expected AnalysisScreen after TransitionToAnalysisMsg")
}

func TestAppModelTransitionToConfirmation(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Start with config
	cfg := &config.Config{
		InteractiveMode: true,
		SourcePath:      "/source",
		DestPath:        "/dest",
	}

	model := tui.NewAppModel(cfg)

	// Create a test engine
	engine := mustNewEngine(t, "/test/source", "/test/dest")
	logPath := "/tmp/test-debug.log"

	// Send TransitionToConfirmationMsg
	confirmMsg := shared.TransitionToConfirmationMsg{
		Engine:  engine,
		LogPath: logPath,
	}
	updatedModel, _ := model.Update(confirmMsg)
	appModel, ok := updatedModel.(tui.AppModel)
	g.Expect(ok).Should(BeTrue(), "Expected updatedModel to be AppModel")

	model = &appModel

	// Verify we transitioned to ConfirmationScreen
	_, isConfirmationScreen := model.CurrentScreen().(*screens.ConfirmationScreen)
	g.Expect(isConfirmationScreen).Should(BeTrue(), "Expected ConfirmationScreen after TransitionToConfirmationMsg")
}

func TestAppModelTransitionToInput(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Start with config
	cfg := &config.Config{
		InteractiveMode: true,
		SourcePath:      "/source",
		DestPath:        "/dest",
	}

	model := tui.NewAppModel(cfg)

	// Transition to analysis first
	analysisMsg := shared.TransitionToAnalysisMsg{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	updatedModel, _ := model.Update(analysisMsg)
	appModel, ok := updatedModel.(tui.AppModel)
	g.Expect(ok).Should(BeTrue(), "Expected updatedModel to be AppModel after analysis transition")

	model = &appModel

	// Verify we're on analysis screen
	_, isAnalysisScreen := model.CurrentScreen().(*screens.AnalysisScreen)
	g.Expect(isAnalysisScreen).Should(BeTrue(), "Expected AnalysisScreen after TransitionToAnalysisMsg")

	// Now transition back to input with Esc
	inputMsg := shared.TransitionToInputMsg{}
	updatedModel, _ = model.Update(inputMsg)
	appModel, ok = updatedModel.(tui.AppModel)
	g.Expect(ok).Should(BeTrue(), "Expected updatedModel to be AppModel after input transition")

	model = &appModel

	// Verify we're back on input screen
	_, isInputScreen := model.CurrentScreen().(*screens.InputScreen)
	g.Expect(isInputScreen).Should(BeTrue(), "Expected InputScreen after TransitionToInputMsg")
}

func TestAppModelTransitionToSummary(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		InteractiveMode: true,
	}

	model := tui.NewAppModel(cfg)

	// Create an engine first
	engine := mustNewEngine(t, "/source", "/dest")

	// Send TransitionToSyncMsg first to set the engine
	syncMsg := shared.TransitionToSyncMsg{
		Engine: engine,
	}

	updatedModel, _ := model.Update(syncMsg)

	g := NewWithT(t)

	appModel, ok := updatedModel.(tui.AppModel)
	g.Expect(ok).Should(BeTrue(), "Expected updatedModel to be AppModel after sync transition")

	model = &appModel

	// Send TransitionToSummaryMsg to trigger transition
	summaryMsg := shared.TransitionToSummaryMsg{
		FinalState: "complete",
		Err:        nil,
	}

	updatedModel, _ = model.Update(summaryMsg)

	appModel, ok = updatedModel.(tui.AppModel)
	g.Expect(ok).Should(BeTrue(), "Expected updatedModel to be AppModel after summary transition")

	model = &appModel

	// Verify we transitioned to SummaryScreen
	_, isSummaryScreen := model.CurrentScreen().(*screens.SummaryScreen)
	g.Expect(isSummaryScreen).Should(BeTrue(), "Expected SummaryScreen after TransitionToSummaryMsg")
}

func TestAppModelTransitionToSync(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		InteractiveMode: true,
	}

	model := tui.NewAppModel(cfg)

	// Create an engine for the transition
	engine := mustNewEngine(t, "/source", "/dest")

	// Send TransitionToSyncMsg to trigger transition
	msg := shared.TransitionToSyncMsg{
		Engine: engine,
	}

	updatedModel, _ := model.Update(msg)

	g := NewWithT(t)

	appModel, ok := updatedModel.(tui.AppModel)
	g.Expect(ok).Should(BeTrue(), "Expected updatedModel to be AppModel")

	model = &appModel

	// Verify we transitioned to SyncScreen
	_, isSyncScreen := model.CurrentScreen().(*screens.SyncScreen)
	g.Expect(isSyncScreen).Should(BeTrue(), "Expected SyncScreen after TransitionToSyncMsg")
}

func TestNewAppModelInteractiveMode(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test: When InteractiveMode is true, should start with InputScreen
	cfg := &config.Config{
		InteractiveMode: true,
	}

	model := tui.NewAppModel(cfg)

	// Verify the initial screen is an InputScreen
	_, isInputScreen := model.CurrentScreen().(*screens.InputScreen)
	g.Expect(isInputScreen).Should(BeTrue(), "Expected InputScreen when InteractiveMode is true")

	// Call methods to ensure coverage
	_ = model.Init()
	_, _ = model.Update(nil)
	_ = model.View()
}

func TestNewAppModelNonInteractiveMode(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test: When InteractiveMode is false, should start with AnalysisScreen
	cfg := &config.Config{
		InteractiveMode: false,
		SourcePath:      "/source",
		DestPath:        "/dest",
	}

	model := tui.NewAppModel(cfg)

	// Verify the initial screen is an AnalysisScreen
	_, isAnalysisScreen := model.CurrentScreen().(*screens.AnalysisScreen)
	g.Expect(isAnalysisScreen).Should(BeTrue(), "Expected AnalysisScreen when InteractiveMode is false")

	// Call methods to ensure coverage
	_ = model.Init()
	_, _ = model.Update(nil)
	_ = model.View()
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
