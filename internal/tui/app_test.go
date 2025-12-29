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

func TestAppModelTransitionToSync(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		InteractiveMode: true,
	}

	model := tui.NewAppModel(cfg)

	// Create an engine for the transition
	engine := syncengine.NewEngine("/source", "/dest")

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

func TestAppModelTransitionToSummary(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		InteractiveMode: true,
	}

	model := tui.NewAppModel(cfg)

	// Create an engine first
	engine := syncengine.NewEngine("/source", "/dest")

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
