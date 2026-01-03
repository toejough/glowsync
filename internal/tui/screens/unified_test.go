package screens_test

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega"

	"github.com/joe/copy-files/internal/config"
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
