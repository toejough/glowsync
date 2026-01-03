//nolint:varnamelen // Test files use idiomatic short variable names
package widgets_test

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/tui/widgets"
)

func TestNewPhaseWidget_AnalyzingPhase_ShowsAnalyzingMessage(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	widget := widgets.NewPhaseWidget("analyzing")
	result := widget()

	g.Expect(result).Should(ContainSubstring("Analyzing"),
		"Phase widget should show 'Analyzing' message during analyzing phase")
}

func TestNewPhaseWidget_ConfirmationPhase_ShowsPreparingMessage(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	widget := widgets.NewPhaseWidget("confirmation")
	result := widget()

	g.Expect(result).Should(ContainSubstring("Preparing"),
		"Phase widget should show 'Preparing' message during confirmation phase")
}

func TestNewPhaseWidget_SyncingPhase_ShowsSyncingMessage(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	widget := widgets.NewPhaseWidget("syncing")
	result := widget()

	g.Expect(result).Should(ContainSubstring("Syncing"),
		"Phase widget should show 'Syncing' message during syncing phase")
}

func TestNewPhaseWidget_UnknownPhase_ReturnsEmptyString(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	widget := widgets.NewPhaseWidget("unknown")
	result := widget()

	g.Expect(result).Should(Equal(""),
		"Phase widget should return empty string for unknown phase")
}

func TestNewPhaseWidget_ReturnsNonEmptyForValidPhases(t *testing.T) {
	t.Parallel()

	testCases := []struct{
		name  string
		phase string
	}{
		{"analyzing phase", "analyzing"},
		{"confirmation phase", "confirmation"},
		{"syncing phase", "syncing"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			widget := widgets.NewPhaseWidget(tc.phase)
			result := widget()

			g.Expect(result).ShouldNot(BeEmpty(),
				"Phase widget should return non-empty string for valid phase: "+tc.phase)
		})
	}
}
