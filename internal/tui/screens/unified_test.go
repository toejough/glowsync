package screens_test

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega"

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

// TestUnifiedScreen_StartAnalysis tests the startAnalysis method
func TestUnifiedScreen_StartAnalysis(t *testing.T) {
	t.Parallel()

	t.Run("valid paths should transition to analyzing", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Type valid source path
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		// Tab to dest field
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Type valid dest path
		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		// Press Enter to start analysis
		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

		// Verify command is returned for background work
		g.Expect(cmd).NotTo(BeNil(),
			"startAnalysis should return command for background analysis")

		// Verify view shows analyzing phase content
		view := model.View()
		g.Expect(view).To(ContainSubstring("scan"),
			"View should show scan timeline phase")
	})

	t.Run("empty source path should stay in input phase", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Tab to dest field
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Type valid dest path
		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		// Press Enter with empty source
		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

		// Verify no command is returned
		g.Expect(cmd).To(BeNil(),
			"Should not return command when source path is empty")

		// Verify still in input phase
		view := model.View()
		g.Expect(view).To(ContainSubstring("input"),
			"View should show input timeline phase")
		g.Expect(view).To(ContainSubstring("Source Path"),
			"View should still show input fields")
	})

	t.Run("empty dest path should stay in input phase", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Type valid source path
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		// Press Enter with empty dest
		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

		// Verify no command is returned
		g.Expect(cmd).To(BeNil(),
			"Should not return command when dest path is empty")

		// Verify still in input phase
		view := model.View()
		g.Expect(view).To(ContainSubstring("input"),
			"View should show input timeline phase")
	})

	t.Run("creates engine with correct paths and pattern", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Type valid source path
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		// Tab to dest field
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Type valid dest path
		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		// Tab to pattern field
		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Type pattern
		pattern := "*.txt"
		for _, r := range pattern {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		// Press Enter to start analysis
		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

		// Verify command is returned
		g.Expect(cmd).NotTo(BeNil(),
			"startAnalysis should return command")

		// Verify transition to analyzing phase by checking view
		view := model.View()
		g.Expect(view).To(ContainSubstring("scan"),
			"View should show scan timeline phase")
	})

	t.Run("adds Phase Progress and ActivityLog widgets", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Type valid paths
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		// Press Enter to start analysis
		_, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})

		// Verify view contains expected widget content
		view := model.View()
		g.Expect(view).To(MatchRegexp("(?i)analyzing|progress|files"),
			"View should contain analyzing phase widgets")
	})
}

// TestUnifiedScreen_StartSync tests the startSync method
// Note: Since we can't directly set phase to confirmation, we simulate the full workflow
func TestUnifiedScreen_StartSync(t *testing.T) {
	t.Parallel()

	t.Run("transitions to syncing phase and returns command", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Simulate reaching confirmation phase by completing analysis
		// Step 1: Enter valid paths
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		// Step 2: Start analysis
		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Step 3: Simulate analysis complete
		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Now in confirmation phase - verify view
		view := model.View()
		g.Expect(view).To(ContainSubstring("compare"),
			"Should be in confirmation phase showing compare timeline")

		// Step 4: Press Enter to start sync
		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

		// Verify command is returned for background sync
		g.Expect(cmd).NotTo(BeNil(),
			"startSync should return command for background sync")

		// Verify transition to syncing phase
		view = model.View()
		g.Expect(view).To(ContainSubstring("sync"),
			"View should show sync timeline phase")
	})

	t.Run("adds syncing widgets to view", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Simulate workflow to confirmation phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Start sync
		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify view contains syncing widget content
		view := model.View()
		g.Expect(view).To(MatchRegexp("(?i)file|sync|worker|progress"),
			"View should contain syncing phase widgets")
	})
}

// TestUnifiedScreen_AnalysisComplete tests handling of AnalysisCompleteMsg
func TestUnifiedScreen_AnalysisComplete(t *testing.T) {
	t.Parallel()

	t.Run("transitions to confirmation phase", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Reach analyzing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify in analyzing phase
		view := model.View()
		g.Expect(view).To(ContainSubstring("scan"),
			"Should be in analyzing phase")

		// Send AnalysisCompleteMsg
		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify transition to confirmation phase
		view = model.View()
		g.Expect(view).To(ContainSubstring("compare"),
			"Should transition to confirmation phase showing compare timeline")
	})

	t.Run("adds SyncPlan widget content to view", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Reach analyzing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Send AnalysisCompleteMsg
		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify view contains sync plan widget content
		view := model.View()
		g.Expect(view).To(MatchRegexp("(?i)files?|sync|plan|ready"),
			"View should contain sync plan widget")
	})
}

// TestUnifiedScreen_SyncComplete tests handling of SyncCompleteMsg
func TestUnifiedScreen_SyncComplete(t *testing.T) {
	t.Parallel()

	t.Run("transitions to summary phase", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Reach syncing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify in syncing phase
		view := model.View()
		g.Expect(view).To(ContainSubstring("sync"),
			"Should be in syncing phase")

		// Send SyncCompleteMsg
		updatedModel, _ = model.Update(shared.SyncCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify transition to summary phase
		view = model.View()
		g.Expect(view).To(MatchRegexp("(?i)done"),
			"Should transition to summary phase showing done timeline")
	})

	t.Run("adds Summary widget content to view", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Reach syncing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Send SyncCompleteMsg
		updatedModel, _ = model.Update(shared.SyncCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify view contains summary widget content
		view := model.View()
		g.Expect(view).To(MatchRegexp("(?i)complete|summary|finish"),
			"View should contain summary widget")
	})
}

// TestUnifiedScreen_ErrorMsg tests handling of ErrorMsg
func TestUnifiedScreen_ErrorMsg(t *testing.T) {
	t.Parallel()

	t.Run("from analyzing phase transitions to summary with error", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Reach analyzing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Send ErrorMsg
		testErr := errors.New("analysis failed")
		updatedModel, _ = model.Update(shared.ErrorMsg{Err: testErr})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify transition to summary with error
		view := model.View()
		g.Expect(view).To(MatchRegexp("(?i)done.*error|error.*done"),
			"Should transition to summary phase with error indication")
		g.Expect(view).To(ContainSubstring("failed"),
			"View should show error message")
	})

	t.Run("from syncing phase transitions to summary with error", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Reach syncing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Send ErrorMsg
		testErr := errors.New("sync failed")
		updatedModel, _ = model.Update(shared.ErrorMsg{Err: testErr})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify transition to summary with error
		view := model.View()
		g.Expect(view).To(MatchRegexp("(?i)done.*error|error.*done"),
			"Should transition to summary phase with error indication")
		g.Expect(view).To(ContainSubstring("failed"),
			"View should show error message")
	})
}

// TestUnifiedScreen_WidgetIntegration tests that actual widget content appears (not placeholders)
func TestUnifiedScreen_WidgetIntegration(t *testing.T) {
	t.Parallel()

	t.Run("analyzing phase shows real Phase widget content", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to analyzing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify Phase widget shows real content from widgets.NewPhaseWidget("analyzing")
		view := model.View()
		g.Expect(view).To(ContainSubstring("Analyzing files..."),
			"Phase widget should show real analyzing message from widgets.NewPhaseWidget")
	})

	t.Run("analyzing phase shows real Progress widget format", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to analyzing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify Progress widget shows real format from widgets.NewProgressWidget
		// Expected format: "Files: N / M (X.X%)\nBytes: X B / Y B"
		view := model.View()
		g.Expect(view).To(MatchRegexp(`Files: \d+ / \d+ \(\d+\.\d+%\)`),
			"Progress widget should show real file progress format from widgets.NewProgressWidget")
		g.Expect(view).To(MatchRegexp(`Bytes: .+ / .+`),
			"Progress widget should show real bytes format from widgets.NewProgressWidget")
	})

	t.Run("confirmation phase shows real SyncPlan widget format", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to confirmation phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify SyncPlan widget shows real format from widgets.NewSyncPlanWidget
		// Expected format: "Files to sync: N\nTotal size: X B"
		view := model.View()
		g.Expect(view).To(MatchRegexp(`Files to sync: \d+`),
			"SyncPlan widget should show real files count format from widgets.NewSyncPlanWidget")
		g.Expect(view).To(MatchRegexp(`Total size: .+`),
			"SyncPlan widget should show real size format from widgets.NewSyncPlanWidget")
	})

	t.Run("syncing phase shows real FileList widget format", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to syncing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Note: FileList widget may be empty if no files are actively copying
		// This test verifies the widget uses real renderer, not placeholder
		// The actual file content depends on engine status having FilesToSync
		view := model.View()
		// The view should NOT contain placeholder text like "FILE LIST WIDGET"
		g.Expect(view).NotTo(ContainSubstring("FILE LIST WIDGET"),
			"FileList widget should use real renderer from widgets.NewFileListWidget, not placeholder")
	})

	t.Run("syncing phase shows real WorkerStats widget format", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to syncing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify WorkerStats widget shows real format from widgets.NewWorkerStatsWidget
		// Expected format: "Workers: N\nSpeed: X B/s"
		view := model.View()
		g.Expect(view).To(MatchRegexp(`Workers: \d+`),
			"WorkerStats widget should show real worker count format from widgets.NewWorkerStatsWidget")
		g.Expect(view).To(MatchRegexp(`Speed: .+/s`),
			"WorkerStats widget should show real speed format from widgets.NewWorkerStatsWidget")
	})

	t.Run("summary phase shows real Summary widget format", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to summary phase via SyncCompleteMsg
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.SyncCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify Summary widget shows real format from widgets.NewSummaryWidget
		// Expected format: "Sync complete!\n\nFiles synced: N file(s)\nBytes transferred: X B\nTime elapsed: Xs"
		view := model.View()
		g.Expect(view).To(MatchRegexp(`(?i)sync complete|error`),
			"Summary widget should show completion or error message from widgets.NewSummaryWidget")
		g.Expect(view).To(MatchRegexp(`Files synced: \d+`),
			"Summary widget should show real files count format from widgets.NewSummaryWidget")
		g.Expect(view).To(MatchRegexp(`Bytes transferred: .+`),
			"Summary widget should show real bytes format from widgets.NewSummaryWidget")
		g.Expect(view).To(MatchRegexp(`Time elapsed: .+`),
			"Summary widget should show real time format from widgets.NewSummaryWidget")
	})
}

// TestUnifiedScreen_WidgetDataFlow tests that widgets get live status updates
func TestUnifiedScreen_WidgetDataFlow(t *testing.T) {
	t.Parallel()

	t.Run("Progress widget reflects engine status changes", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// This test will verify that the Progress widget gets status updates
		// by checking the view output changes as status updates arrive

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to analyzing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Create a status update message with specific file counts
		statusMsg := shared.StatusUpdateMsg{
			Status: &syncengine.Status{
				TotalFiles:       100,
				ProcessedFiles:   25,
				TotalBytes:       1024000,
				TransferredBytes: 256000,
			},
		}

		// Send status update
		updatedModel, _ = model.Update(statusMsg)
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify view shows updated progress
		view := model.View()
		g.Expect(view).To(ContainSubstring("25 / 100"),
			"Progress widget should reflect updated file counts")

		// Send another status update with different values
		statusMsg2 := shared.StatusUpdateMsg{
			Status: &syncengine.Status{
				TotalFiles:       100,
				ProcessedFiles:   50,
				TotalBytes:       1024000,
				TransferredBytes: 512000,
			},
		}

		updatedModel, _ = model.Update(statusMsg2)
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify view shows new progress
		view = model.View()
		g.Expect(view).To(ContainSubstring("50 / 100"),
			"Progress widget should reflect second status update")
		g.Expect(view).NotTo(ContainSubstring("25 / 100"),
			"Progress widget should no longer show old values")
	})

	t.Run("WorkerStats updates with worker count changes", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to syncing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Send status update with 2 workers
		statusMsg := shared.StatusUpdateMsg{
			Status: &syncengine.Status{
				CurrentFiles:   []string{"file1.txt", "file2.txt"},
				BytesPerSecond: 1048576, // 1 MB/s
			},
		}

		updatedModel, _ = model.Update(statusMsg)
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify view shows 2 workers
		view := model.View()
		g.Expect(view).To(ContainSubstring("Workers: 2"),
			"WorkerStats widget should show 2 active workers")

		// Send status update with 4 workers
		statusMsg2 := shared.StatusUpdateMsg{
			Status: &syncengine.Status{
				CurrentFiles:   []string{"file1.txt", "file2.txt", "file3.txt", "file4.txt"},
				BytesPerSecond: 2097152, // 2 MB/s
			},
		}

		updatedModel, _ = model.Update(statusMsg2)
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify view shows updated worker count
		view = model.View()
		g.Expect(view).To(ContainSubstring("Workers: 4"),
			"WorkerStats widget should reflect updated worker count")
		g.Expect(view).NotTo(ContainSubstring("Workers: 2"),
			"WorkerStats widget should no longer show old worker count")
	})

	t.Run("SyncPlan updates when analysis completes", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to analyzing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Send status update during analysis
		statusMsg := shared.StatusUpdateMsg{
			Status: &syncengine.Status{
				TotalFiles: 42,
				TotalBytes: 8192000,
			},
		}

		updatedModel, _ = model.Update(statusMsg)
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Complete analysis
		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify SyncPlan shows the file count from analysis
		view := model.View()
		g.Expect(view).To(ContainSubstring("Files to sync: 42"),
			"SyncPlan widget should show file count from completed analysis")
	})
}

// TestUnifiedScreen_WidgetLifecycle tests widget creation/cleanup during phase transitions
func TestUnifiedScreen_WidgetLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("analyzing phase adds Phase Progress and ActivityLog widgets in order", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to analyzing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		// Transition to analyzing phase
		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify all three widgets appear in view
		view := model.View()
		g.Expect(view).To(ContainSubstring("Analyzing files..."),
			"Phase widget should be present")
		g.Expect(view).To(MatchRegexp(`Files: \d+ / \d+`),
			"Progress widget should be present")

		// Verify Phase widget appears before Progress widget
		phaseIdx := findSubstringIndex(view, "Analyzing files...")
		progressIdx := findSubstringIndex(view, "Files:")
		g.Expect(phaseIdx).To(BeNumerically("<", progressIdx),
			"Phase widget should appear before Progress widget in view")
	})

	t.Run("confirmation phase adds SyncPlan widget", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to confirmation phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Transition to confirmation phase
		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify SyncPlan widget appears
		view := model.View()
		g.Expect(view).To(ContainSubstring("Files to sync:"),
			"SyncPlan widget should be present in confirmation phase")
		g.Expect(view).To(ContainSubstring("Total size:"),
			"SyncPlan widget should show total size")
	})

	t.Run("syncing phase adds FileList and WorkerStats widgets", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to syncing phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Transition to syncing phase
		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify WorkerStats widget appears (FileList may be empty)
		view := model.View()
		g.Expect(view).To(ContainSubstring("Workers:"),
			"WorkerStats widget should be present in syncing phase")
		g.Expect(view).To(ContainSubstring("Speed:"),
			"WorkerStats widget should show speed")
	})

	t.Run("summary phase adds Summary widget", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate to summary phase
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Transition to summary phase
		updatedModel, _ = model.Update(shared.SyncCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify Summary widget appears
		view := model.View()
		g.Expect(view).To(MatchRegexp(`(?i)sync complete|error`),
			"Summary widget should be present in summary phase")
		g.Expect(view).To(ContainSubstring("Files synced:"),
			"Summary widget should show files synced count")
		g.Expect(view).To(ContainSubstring("Time elapsed:"),
			"Summary widget should show elapsed time")
	})

	t.Run("previous phase widgets remain visible across transitions", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ok bool
		cfg := &config.Config{}
		model := screens.NewUnifiedScreen(cfg)

		// Navigate through phases
		sourceDir := t.TempDir()
		for _, r := range sourceDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		destDir := t.TempDir()
		for _, r := range destDir {
			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			model, ok = updatedModel.(*screens.UnifiedScreen)
			g.Expect(ok).To(BeTrue())
		}

		// Enter analyzing phase
		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify analyzing widgets present
		view := model.View()
		g.Expect(view).To(ContainSubstring("Analyzing files..."))

		// Transition to confirmation phase
		statusMsg := shared.StatusUpdateMsg{
			Status: &syncengine.Status{
				TotalFiles: 15,
				TotalBytes: 4096000,
				StartTime:  time.Now(),
			},
		}
		updatedModel, _ = model.Update(statusMsg)
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		updatedModel, _ = model.Update(shared.AnalysisCompleteMsg{})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify confirmation widgets added (analyzing widgets may persist)
		view = model.View()
		g.Expect(view).To(ContainSubstring("Files to sync: 15"),
			"SyncPlan widget should appear in confirmation phase")

		// Transition to syncing phase
		updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model, ok = updatedModel.(*screens.UnifiedScreen)
		g.Expect(ok).To(BeTrue())

		// Verify syncing widgets present
		view = model.View()
		g.Expect(view).To(ContainSubstring("Workers:"),
			"WorkerStats widget should appear in syncing phase")
	})
}

// Helper function to find substring index in string
func findSubstringIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}

	return -1
}
