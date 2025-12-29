//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
)

func TestGetAnalysisPhaseText(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		status: &syncengine.Status{},
	}

	// Test various phases
	screen.status.AnalysisPhase = "counting_source"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Counting"))

	screen.status.AnalysisPhase = "scanning_source"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Scanning"))

	screen.status.AnalysisPhase = "counting_dest"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Counting"))

	screen.status.AnalysisPhase = "scanning_dest"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Scanning"))

	screen.status.AnalysisPhase = "comparing"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Comparing"))

	screen.status.AnalysisPhase = "deleting"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Checking"))

	screen.status.AnalysisPhase = "complete"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("complete"))

	screen.status.AnalysisPhase = "unknown"
	g.Expect(screen.getAnalysisPhaseText()).Should(ContainSubstring("Initializing"))
}

func TestRenderAnalysisLog(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		status: &syncengine.Status{
			AnalysisLog: []string{"log entry 1", "log entry 2"},
		},
	}

	var builder strings.Builder
	screen.renderAnalysisLog(&builder)
	result := builder.String()
	g.Expect(result).Should(ContainSubstring("Activity Log"))
	g.Expect(result).Should(ContainSubstring("log entry 1"))

	// Test with empty log
	screen.status.AnalysisLog = []string{}

	builder.Reset()
	screen.renderAnalysisLog(&builder)
	result = builder.String()
	g.Expect(result).Should(BeEmpty())
}

func TestRenderAnalysisProgress(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &AnalysisScreen{
		status: &syncengine.Status{},
	}

	// Test counting phase
	screen.status.AnalysisPhase = "counting_source"
	screen.status.ScannedFiles = 100

	var builder strings.Builder
	screen.renderAnalysisProgress(&builder)
	result := builder.String()
	g.Expect(result).Should(ContainSubstring("Found"))

	// Test scanning phase with total
	screen.status.AnalysisPhase = "scanning_source"
	screen.status.TotalFilesToScan = 1000
	screen.status.ScannedFiles = 500

	builder.Reset()
	screen.renderAnalysisProgress(&builder)
	result = builder.String()
	g.Expect(result).ShouldNot(BeEmpty())

	// Test scanning phase without total
	screen.status.TotalFilesToScan = 0
	screen.status.ScannedFiles = 50

	builder.Reset()
	screen.renderAnalysisProgress(&builder)
	result = builder.String()
	g.Expect(result).Should(ContainSubstring("Processed"))
}
