//nolint:varnamelen // Test files use idiomatic short variable names
package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// TestAnalysisPhaseTrackingViaEvents tests that file counts are properly captured
// via engine events (not polling).
func TestAnalysisPhaseTrackingViaEvents(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/test/source",
		DestPath:   "/test/dest",
	}

	screen := NewAnalysisScreen(cfg)

	// Simulate receiving ScanStarted event for source
	model, _ := screen.Update(shared.EngineEventMsg{
		Event: syncengine.ScanStarted{Target: "source"},
	})
	screen = toAnalysisScreen(model)
	g.Expect(screen.currentScanTarget).To(Equal("source"))

	// Simulate receiving ScanComplete event for source with 500 files
	model, _ = screen.Update(shared.EngineEventMsg{
		Event: syncengine.ScanComplete{Target: "source", Count: 500},
	})
	screen = toAnalysisScreen(model)

	// Verify source phase was recorded with correct count from event
	g.Expect(screen.sourcePhases).To(HaveLen(1), "should have one source phase recorded")
	g.Expect(screen.sourcePhases[0].text).To(Equal("Counting (quick check)"))
	g.Expect(screen.sourcePhases[0].result).To(Equal("500 files"))
	g.Expect(screen.sourceFileCount).To(Equal(500))

	// Simulate ScanStarted for dest
	model, _ = screen.Update(shared.EngineEventMsg{
		Event: syncengine.ScanStarted{Target: "dest"},
	})
	screen = toAnalysisScreen(model)
	g.Expect(screen.currentScanTarget).To(Equal("dest"))

	// Simulate ScanComplete for dest with 300 files
	model, _ = screen.Update(shared.EngineEventMsg{
		Event: syncengine.ScanComplete{Target: "dest", Count: 300},
	})
	screen = toAnalysisScreen(model)

	// Verify dest phase was recorded
	g.Expect(screen.destPhases).To(HaveLen(1), "should have one dest phase recorded")
	g.Expect(screen.destPhases[0].text).To(Equal("Counting (quick check)"))
	g.Expect(screen.destPhases[0].result).To(Equal("300 files"))
	g.Expect(screen.destFileCount).To(Equal(300))

	// Simulate another source scan (full scan after monotonic check failed)
	model, _ = screen.Update(shared.EngineEventMsg{
		Event: syncengine.ScanComplete{Target: "source", Count: 500},
	})
	screen = toAnalysisScreen(model)

	// Verify second source counting phase was recorded as "full scan"
	g.Expect(screen.sourcePhases).To(HaveLen(2), "should have two source phases recorded")
	g.Expect(screen.sourcePhases[1].text).To(Equal("Counting (full scan)"))
	g.Expect(screen.sourcePhases[1].result).To(Equal("500 files"))

	// Verify the view contains the expected output
	screen.width = 80
	screen.height = 24
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

func toAnalysisScreen(model tea.Model) *AnalysisScreen {
	s := model.(AnalysisScreen)
	return &s
}

// TestEventBasedCountsAreGuaranteedCorrect tests that event-based counts
// are always correct, even when polling would miss them.
func TestEventBasedCountsAreGuaranteedCorrect(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/test/source",
		DestPath:   "/test/dest",
	}

	screen := NewAnalysisScreen(cfg)

	// This test simulates the scenario that caused the original bug:
	// - Engine scans 12108 files
	// - Polling-based TUI might miss the final count (showing 8500)
	// - Event-based TUI gets the exact count from ScanComplete event

	// Simulate receiving ScanComplete with the exact count
	model, _ := screen.Update(shared.EngineEventMsg{
		Event: syncengine.ScanComplete{Target: "source", Count: 12108},
	})
	screen = toAnalysisScreen(model)

	// The count should be exactly what the engine reported
	g.Expect(screen.sourceFileCount).To(Equal(12108),
		"Event-based count should match exactly what engine reported")
	g.Expect(screen.sourcePhases[0].result).To(Equal("12108 files"),
		"Displayed count should match exactly what engine reported")
}

// TestCompareCompleteEventStoresSyncPlan tests that CompareComplete stores the plan.
func TestCompareCompleteEventStoresSyncPlan(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/test/source",
		DestPath:   "/test/dest",
	}

	screen := NewAnalysisScreen(cfg)

	// Simulate receiving CompareComplete with a sync plan
	plan := &syncengine.SyncPlan{
		FilesToCopy:   100,
		FilesToDelete: 5,
		BytesToCopy:   1024 * 1024 * 10, // 10 MB
	}
	model, _ := screen.Update(shared.EngineEventMsg{
		Event: syncengine.CompareComplete{Plan: plan},
	})
	screen = toAnalysisScreen(model)

	// Verify the plan was stored
	g.Expect(screen.syncPlan).ToNot(BeNil())
	g.Expect(screen.syncPlan.FilesToCopy).To(Equal(100))
	g.Expect(screen.syncPlan.FilesToDelete).To(Equal(5))
	g.Expect(screen.syncPlan.BytesToCopy).To(Equal(int64(1024 * 1024 * 10)))
}
