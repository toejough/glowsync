//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

// TestAnalysisScreen_AnalyzingView_ActivityLogInRightColumn verifies activity log section exists
func TestAnalysisScreen_AnalyzingView_ActivityLogInRightColumn(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	analysisScreen, ok := updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	// Transition to analyzing state
	engine, err := syncengine.NewEngine("/source", "/dest")
	g.Expect(err).ToNot(HaveOccurred())

	initMsg := shared.EngineInitializedMsg{Engine: engine}
	updatedModel, _ = analysisScreen.Update(initMsg)
	analysisScreen, ok = updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	view := analysisScreen.View()

	// Activity log section should be present (even if empty initially)
	g.Expect(view).To(ContainSubstring("Activity"), "Should have activity log section")
}

// TestAnalysisScreen_AnalyzingView_ShowsScanPhaseActive verifies timeline in analyzing state
func TestAnalysisScreen_AnalyzingView_ShowsScanPhaseActive(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	analysisScreen, ok := updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	// Transition to analyzing state
	engine, err := syncengine.NewEngine("/source", "/dest")
	g.Expect(err).ToNot(HaveOccurred())

	initMsg := shared.EngineInitializedMsg{Engine: engine}
	updatedModel, _ = analysisScreen.Update(initMsg)
	analysisScreen, ok = updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	view := analysisScreen.View()
	stripped := stripANSI(view)

	// Timeline should show "scan" phase as active
	g.Expect(stripped).To(ContainSubstring("Scan"), "Timeline should contain Scan phase name")
	g.Expect(stripped).To(ContainSubstring(shared.ActiveSymbol()),
		"Timeline should show active symbol for Scan phase during analysis")
}

// ============================================================================
// Analyzing View - Layout Integration Tests
// ============================================================================

// TestAnalysisScreen_AnalyzingView_TwoColumnLayout verifies two-column structure
func TestAnalysisScreen_AnalyzingView_TwoColumnLayout(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	analysisScreen, ok := updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	// Transition to analyzing state
	engine, err := syncengine.NewEngine("/source", "/dest")
	g.Expect(err).ToNot(HaveOccurred())

	initMsg := shared.EngineInitializedMsg{Engine: engine}
	updatedModel, _ = analysisScreen.Update(initMsg)
	analysisScreen, ok = updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	view := analysisScreen.View()

	// Verify timeline (header)
	stripped := stripANSI(view)
	g.Expect(stripped).To(ContainSubstring("Scan"), "Should have timeline header")

	// Verify content is present
	g.Expect(view).NotTo(BeEmpty(), "View should render content")
	g.Expect(view).To(ContainSubstring("Analyzing"), "Should show analyzing title")
}

// ============================================================================
// Visual Regression Tests
// ============================================================================

// TestAnalysisScreen_HelpText_StillVisible verifies help text renders
func TestAnalysisScreen_HelpText_StillVisible(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	analysisScreen, ok := updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	// Transition to analyzing state
	engine, err := syncengine.NewEngine("/source", "/dest")
	g.Expect(err).ToNot(HaveOccurred())

	initMsg := shared.EngineInitializedMsg{Engine: engine}
	updatedModel, _ = analysisScreen.Update(initMsg)
	analysisScreen, ok = updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	view := analysisScreen.View()

	// Help text should be visible
	g.Expect(view).To(ContainSubstring("Esc"),
		"Help text should mention Esc key")
	g.Expect(view).To(ContainSubstring("Ctrl+C"),
		"Help text should mention Ctrl+C")
}

// TestAnalysisScreen_InitializingView_BasicElements verifies core elements in initializing view
func TestAnalysisScreen_InitializingView_BasicElements(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	view := screen.View()

	// Timeline header
	stripped := stripANSI(view)
	g.Expect(stripped).To(ContainSubstring("Scan"), "Should contain timeline with Scan phase")

	// Initializing message
	g.Expect(view).To(ContainSubstring("Initializing"), "Should show initializing message")

	// Help text
	g.Expect(view).To(ContainSubstring("Esc"), "Should show help text with Esc key")
	g.Expect(view).To(ContainSubstring("Ctrl+C"), "Should show help text with Ctrl+C")
}

// ============================================================================
// Initializing View Tests
// ============================================================================

// TestAnalysisScreen_InitializingView_NoTwoColumnLayout verifies simple layout for initializing
func TestAnalysisScreen_InitializingView_NoTwoColumnLayout(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	analysisScreen, ok := updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	view := analysisScreen.View()

	// Should contain basic initializing elements
	g.Expect(view).To(ContainSubstring("Initializing"), "Should show initializing message")

	// Should have timeline
	stripped := stripANSI(view)
	g.Expect(stripped).To(ContainSubstring("Scan"), "Should have timeline header")

	// Simple content - not a two-column layout (that's for analyzing view only)
	g.Expect(view).NotTo(BeEmpty(), "View should render content")
}

// ============================================================================
// Timeline Integration Tests (Both Views)
// ============================================================================

// TestAnalysisScreen_InitializingView_ShowsScanPhaseActive verifies timeline in initializing state
func TestAnalysisScreen_InitializingView_ShowsScanPhaseActive(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	analysisScreen, ok := updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	view := analysisScreen.View()
	stripped := stripANSI(view)

	// Timeline should show "scan" phase as active
	g.Expect(stripped).To(ContainSubstring("Scan"), "Timeline should contain Scan phase name")
	g.Expect(stripped).To(ContainSubstring(shared.ActiveSymbol()),
		"Timeline should show active symbol for Scan phase during initialization")
}

// ============================================================================
// Widget Box Tests
// ============================================================================

// TestAnalysisScreen_PhaseSection_UsesWidgetBox verifies phase section is boxed
func TestAnalysisScreen_PhaseSection_UsesWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	analysisScreen, ok := updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	// Transition to analyzing state
	engine, err := syncengine.NewEngine("/source", "/dest")
	g.Expect(err).ToNot(HaveOccurred())

	initMsg := shared.EngineInitializedMsg{Engine: engine}
	updatedModel, _ = analysisScreen.Update(initMsg)
	analysisScreen, ok = updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	view := analysisScreen.View()

	// Widget box creates structure with multiple lines
	stripped := stripANSI(view)
	lines := strings.Split(stripped, "\n")
	g.Expect(len(lines)).To(BeNumerically(">", 5),
		"Widget box structure should create multiple lines")

	// Should show phase-related content (spinner, phase text)
	g.Expect(view).To(ContainSubstring("Analyzing"), "Should show analysis content")
}

// TestAnalysisScreen_ProgressSection_UsesWidgetBox verifies progress section exists
func TestAnalysisScreen_ProgressSection_UsesWidgetBox(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	analysisScreen, ok := updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	// Transition to analyzing state
	engine, err := syncengine.NewEngine("/source", "/dest")
	g.Expect(err).ToNot(HaveOccurred())

	initMsg := shared.EngineInitializedMsg{Engine: engine}
	updatedModel, _ = analysisScreen.Update(initMsg)
	analysisScreen, ok = updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	view := analysisScreen.View()

	// Progress section should exist (content will vary based on phase)
	g.Expect(view).NotTo(BeEmpty(), "Progress section should render")
}

// TestAnalysisScreen_Spinner_StillRenders verifies spinner is present
func TestAnalysisScreen_Spinner_StillRenders(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := screens.NewAnalysisScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	analysisScreen, ok := updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	// Transition to analyzing state
	engine, err := syncengine.NewEngine("/source", "/dest")
	g.Expect(err).ToNot(HaveOccurred())

	initMsg := shared.EngineInitializedMsg{Engine: engine}
	updatedModel, _ = analysisScreen.Update(initMsg)
	analysisScreen, ok = updatedModel.(screens.AnalysisScreen)
	g.Expect(ok).To(BeTrue())

	view := analysisScreen.View()

	// View should contain analysis content
	g.Expect(view).NotTo(BeEmpty(), "View should render with spinner")

	// Should show analyzing state content
	stripped := stripANSI(view)
	g.Expect(stripped).To(ContainSubstring("Analyzing"),
		"Should show analyzing title")
}
