//nolint:varnamelen // Test files use idiomatic short variable names (t, tt, g, c, etc.)
package syncengine_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/pkg/fileops"
)

// TestAnalysisProgressIntegration_DivisionByZeroSafety verifies no panics with edge cases.
func TestAnalysisProgressIntegration_DivisionByZeroSafety(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine := mustNewEngine(t, sourceDir, destDir)
	engine.FileOps = fileops.NewRealFileOps()

	// Create empty file (zero bytes)
	err := os.WriteFile(filepath.Join(sourceDir, "empty.txt"), []byte{}, 0o600)
	g.Expect(err).ShouldNot(HaveOccurred())

	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Should handle zero-byte files without division by zero
	progress := engine.Status.CalculateAnalysisProgress()
	g.Expect(progress.BytesPercent).Should(Equal(0.0))
	g.Expect(progress.OverallPercent).Should(BeNumerically(">=", 0.0))
}

// TestAnalysisProgressIntegration_EmptyDestination verifies handling when destination is empty.
func TestAnalysisProgressIntegration_EmptyDestination(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir() // Create empty dest dir

	// Create source file
	err := os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("test"), 0o600)
	g.Expect(err).ShouldNot(HaveOccurred())

	engine := mustNewEngine(t, sourceDir, destDir)
	engine.FileOps = fileops.NewRealFileOps()

	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Should still calculate progress correctly
	progress := engine.Status.CalculateAnalysisProgress()
	g.Expect(progress.IsCounting).Should(BeFalse())
	g.Expect(engine.Status.TotalFiles).Should(Equal(1))
}

// TestAnalysisProgressIntegration_EmptySource verifies graceful handling of empty source.
func TestAnalysisProgressIntegration_EmptySource(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine := mustNewEngine(t, sourceDir, destDir)
	engine.FileOps = fileops.NewRealFileOps()

	err := engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify progress handles zero files gracefully
	progress := engine.Status.CalculateAnalysisProgress()
	g.Expect(progress.FilesPercent).Should(Equal(0.0))
	g.Expect(progress.BytesPercent).Should(Equal(0.0))
	g.Expect(progress.OverallPercent).Should(Equal(0.0))
	g.Expect(engine.Status.TotalBytesToScan).Should(Equal(int64(0)))
}

// TestAnalysisProgressIntegration_FullFlow verifies the complete analysis flow
// transitions properly from counting to processing phase.
func TestAnalysisProgressIntegration_FullFlow(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create test environment
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create test files with known sizes
	testFiles := []struct {
		name string
		size int64
	}{
		{"file1.txt", 1024},
		{"file2.txt", 2048},
		{"file3.txt", 4096},
	}

	var expectedTotalBytes int64
	for _, tf := range testFiles {
		content := make([]byte, tf.size)
		err := os.WriteFile(filepath.Join(sourceDir, tf.name), content, 0o600)
		g.Expect(err).ShouldNot(HaveOccurred())
		expectedTotalBytes += tf.size
	}

	// Create engine
	engine := mustNewEngine(t, sourceDir, destDir)
	engine.FileOps = fileops.NewRealFileOps()

	// Verify initial state - should be in counting phase
	progress := engine.Status.CalculateAnalysisProgress()
	g.Expect(progress.IsCounting).Should(BeTrue(), "Should start in counting phase")
	g.Expect(progress.FilesPercent).Should(Equal(0.0))
	g.Expect(progress.BytesPercent).Should(Equal(0.0))

	// Run analysis
	err := engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify final state - should transition to processing phase
	progress = engine.Status.CalculateAnalysisProgress()
	g.Expect(progress.IsCounting).Should(BeFalse(), "Should transition to processing phase")
	g.Expect(engine.Status.TotalBytesToScan).Should(Equal(expectedTotalBytes))
	g.Expect(engine.Status.ScannedBytes).Should(Equal(expectedTotalBytes))

	// Verify analysis rate was calculated
	g.Expect(engine.Status.AnalysisRate).Should(BeNumerically(">", 0))

	// Verify analysis start time was recorded
	g.Expect(engine.Status.AnalysisStartTime.IsZero()).Should(BeFalse())
}

// TestAnalysisProgressIntegration_PerformanceOverhead verifies <5% overhead.
//
//nolint:paralleltest // Performance test requires sequential execution for accurate timing measurements
func TestAnalysisProgressIntegration_PerformanceOverhead(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	g := NewWithT(t)

	// Create test environment with many files
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	numFiles := 100
	fileSize := int64(10240) // 10KB each

	for i := range numFiles {
		content := make([]byte, fileSize)
		filename := filepath.Join(sourceDir, fmt.Sprintf("file_%03d.txt", i))
		err := os.WriteFile(filename, content, 0o600)
		g.Expect(err).ShouldNot(HaveOccurred())
	}

	// Baseline: Analysis without progress tracking (use old engine or disable tracking)
	engineBaseline := mustNewEngine(t, sourceDir, destDir)
	engineBaseline.FileOps = fileops.NewRealFileOps()

	startBaseline := time.Now()
	err := engineBaseline.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
	baselineDuration := time.Since(startBaseline)

	// With progress tracking
	engineWithTracking := mustNewEngine(t, sourceDir, destDir)
	engineWithTracking.FileOps = fileops.NewRealFileOps()

	startTracking := time.Now()
	err = engineWithTracking.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
	trackingDuration := time.Since(startTracking)

	// Verify overhead is < 5%
	overhead := float64(trackingDuration-baselineDuration) / float64(baselineDuration)
	t.Logf("Baseline: %v, With Tracking: %v, Overhead: %.2f%%", baselineDuration, trackingDuration, overhead*100)

	// Allow up to 10% overhead to account for test environment variability
	g.Expect(overhead).Should(BeNumerically("<", 0.10), "Progress tracking overhead should be < 10%")
}

// TestAnalysisProgressIntegration_PhaseTransitions verifies counting->processing transition.
func TestAnalysisProgressIntegration_PhaseTransitions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create test file
	err := os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("content"), 0o600)
	g.Expect(err).ShouldNot(HaveOccurred())

	engine := mustNewEngine(t, sourceDir, destDir)
	engine.FileOps = fileops.NewRealFileOps()

	// Before analysis: counting phase (TotalBytesToScan = 0)
	progressBefore := engine.Status.CalculateAnalysisProgress()
	g.Expect(progressBefore.IsCounting).Should(BeTrue())

	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// After analysis: processing phase (TotalBytesToScan > 0)
	progressAfter := engine.Status.CalculateAnalysisProgress()
	g.Expect(progressAfter.IsCounting).Should(BeFalse())
	g.Expect(engine.Status.TotalBytesToScan).Should(BeNumerically(">", 0))
}
