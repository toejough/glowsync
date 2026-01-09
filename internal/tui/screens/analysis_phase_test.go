//nolint:varnamelen // Test files use idiomatic short variable names
package screens

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// TestAnalysisPhaseTracking tests that file counts are properly captured
// and displayed when phases transition.
func TestAnalysisPhaseTracking(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/test/source",
		DestPath:   "/test/dest",
	}

	screen := NewAnalysisScreen(cfg)

	// Simulate engine initialization - create a minimal engine-like setup
	// We'll manually set the status to simulate what the engine would report
	screen.status = &syncengine.Status{
		AnalysisPhase: shared.PhaseCountingSource,
		ScannedFiles:  0,
	}
	screen.lastUpdate = time.Now().Add(-time.Second) // Ensure throttle doesn't block

	// Simulate first tick - counting_source with 100 files
	screen.status.ScannedFiles = 100
	screen = simulateTick(screen, shared.PhaseCountingSource, 100)

	// Simulate more ticks in same phase - count increases
	screen = simulateTick(screen, shared.PhaseCountingSource, 250)
	screen = simulateTick(screen, shared.PhaseCountingSource, 500)

	// Verify lastCount is tracking the max
	g.Expect(screen.lastCount).To(Equal(500), "lastCount should track highest count seen")

	// Now transition to counting_dest - this should record the source phase
	screen = simulateTick(screen, shared.PhaseCountingDest, 0)

	// Verify source phase was recorded with correct count
	g.Expect(screen.sourcePhases).To(HaveLen(1), "should have one source phase recorded")
	g.Expect(screen.sourcePhases[0].text).To(Equal("Counting (quick check)"))
	g.Expect(screen.sourcePhases[0].result).To(Equal("500 files"))

	// Continue counting dest
	screen = simulateTick(screen, shared.PhaseCountingDest, 100)
	screen = simulateTick(screen, shared.PhaseCountingDest, 300)

	// Transition to counting_source again (full scan after monotonic check failed)
	screen = simulateTick(screen, shared.PhaseCountingSource, 0)

	// Verify dest phase was recorded
	g.Expect(screen.destPhases).To(HaveLen(1), "should have one dest phase recorded")
	g.Expect(screen.destPhases[0].text).To(Equal("Counting (quick check)"))
	g.Expect(screen.destPhases[0].result).To(Equal("300 files"))

	// Full scan of source
	screen = simulateTick(screen, shared.PhaseCountingSource, 500)
	screen = simulateTick(screen, shared.PhaseScanningSource, 0)

	// Verify second source counting phase was recorded as "full scan"
	g.Expect(screen.sourcePhases).To(HaveLen(2), "should have two source phases recorded")
	g.Expect(screen.sourcePhases[1].text).To(Equal("Counting (full scan)"))
	g.Expect(screen.sourcePhases[1].result).To(Equal("500 files"))

	// Verify the view contains the expected output
	view := screen.View()
	g.Expect(view).To(ContainSubstring("500 files"), "view should show file count")
	g.Expect(view).To(ContainSubstring("300 files"), "view should show dest file count")
	g.Expect(view).To(ContainSubstring("quick check"), "view should show quick check label")
	g.Expect(view).To(ContainSubstring("full scan"), "view should show full scan label")
}

// TestAnalysisPhaseTrackingInView verifies the rendered output format
func TestAnalysisPhaseTrackingInView(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/my/source/path",
		DestPath:   "/my/dest/path",
	}

	screen := NewAnalysisScreen(cfg)
	screen.width = 80
	screen.height = 24

	// Set up completed phases manually to test rendering
	screen.sourcePhases = []completedPhase{
		{text: "Counting (quick check)", result: "1234 files"},
		{text: "Counting (full scan)", result: "1234 files"},
		{text: "Scanning", result: "1234 files"},
	}
	screen.destPhases = []completedPhase{
		{text: "Counting (quick check)", result: "567 files"},
	}
	screen.status = &syncengine.Status{
		AnalysisPhase: shared.PhaseCountingDest,
		ScannedFiles:  100,
	}
	screen.lastPhase = shared.PhaseCountingDest
	screen.seenPhases = map[string]int{"source": 2, "dest": 1}

	view := screen.View()

	// Verify structure - source path with its phases
	g.Expect(view).To(ContainSubstring("Source:"))
	g.Expect(view).To(ContainSubstring("/my/source/path"))
	g.Expect(view).To(ContainSubstring("1234 files"))

	// Verify structure - dest path with its phases
	g.Expect(view).To(ContainSubstring("Dest:"))
	g.Expect(view).To(ContainSubstring("/my/dest/path"))
	g.Expect(view).To(ContainSubstring("567 files"))

	// Verify the order - source should come before dest in the output
	sourceIdx := strings.Index(view, "Source:")
	destIdx := strings.Index(view, "Dest:")
	g.Expect(sourceIdx).To(BeNumerically("<", destIdx), "Source should appear before Dest")
}

// simulateTick simulates what happens when the engine reports a new status.
// Calls updatePhaseTracking directly since that's the logic we're testing.
func simulateTick(screen *AnalysisScreen, phase string, count int) *AnalysisScreen {
	// Update status to simulate what the engine would report
	screen.status = &syncengine.Status{
		AnalysisPhase: phase,
		ScannedFiles:  count,
	}

	// Call the phase tracking logic directly
	screen.updatePhaseTracking()

	return screen
}

// TestAnalysisPhaseTrackingViaUpdate tests the full Update path
// to ensure state is preserved through the value receiver chain.
func TestAnalysisPhaseTrackingViaUpdate(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/test/source",
		DestPath:   "/test/dest",
	}

	// Create a mock engine that returns controlled status values
	engine, err := syncengine.NewEngine("/tmp", "/tmp")
	g.Expect(err).ShouldNot(HaveOccurred())

	screen := NewAnalysisScreen(cfg)
	screen.engine = engine
	screen.lastUpdate = time.Now().Add(-time.Second) // Bypass throttle

	// Manually set the engine's status to simulate counting_source with 500 files
	engine.Status.AnalysisPhase = shared.PhaseCountingSource
	engine.Status.ScannedFiles = 500

	// Send tick through Update
	model, _ := screen.Update(shared.TickMsg{})
	screen = toAnalysisScreen(model)

	// Verify state was captured
	g.Expect(screen.lastPhase).To(Equal(shared.PhaseCountingSource))
	g.Expect(screen.lastCount).To(Equal(500))

	// Update count in same phase
	screen.lastUpdate = time.Now().Add(-time.Second) // Bypass throttle
	engine.Status.ScannedFiles = 1000
	model, _ = screen.Update(shared.TickMsg{})
	screen = toAnalysisScreen(model)

	g.Expect(screen.lastCount).To(Equal(1000), "should track highest count")

	// Transition to counting_dest
	screen.lastUpdate = time.Now().Add(-time.Second)
	engine.Status.AnalysisPhase = shared.PhaseCountingDest
	engine.Status.ScannedFiles = 0
	model, _ = screen.Update(shared.TickMsg{})
	screen = toAnalysisScreen(model)

	// Verify source phase was recorded with correct count
	g.Expect(screen.sourcePhases).To(HaveLen(1))
	g.Expect(screen.sourcePhases[0].result).To(Equal("1000 files"))

	// Check view contains the count
	screen.width = 80
	screen.height = 24
	view := screen.View()
	g.Expect(view).To(ContainSubstring("1000 files"))
}

func toAnalysisScreen(model tea.Model) *AnalysisScreen {
	s := model.(AnalysisScreen)
	return &s
}
