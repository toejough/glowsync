//go:build integration

package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
)

// eventCollector collects events for verification.
type eventCollector struct {
	events []syncengine.Event
}

func (c *eventCollector) Emit(event syncengine.Event) {
	c.events = append(c.events, event)
}

// TestIntegration_FullScan_EmitsCorrectEvents verifies that a full scan emits
// the expected events with correct counts.
func TestIntegration_FullScan_EmitsCorrectEvents(t *testing.T) {
	g := NewWithT(t)

	// Create test directories with known file counts
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create 10 files in source
	for i := 0; i < 10; i++ {
		path := filepath.Join(sourceDir, "file"+string(rune('a'+i))+".txt")
		err := os.WriteFile(path, []byte("content"), 0644)
		g.Expect(err).ShouldNot(HaveOccurred())
	}

	// Create 3 files in dest (some overlap, some orphans)
	for i := 0; i < 3; i++ {
		path := filepath.Join(destDir, "file"+string(rune('a'+i))+".txt")
		err := os.WriteFile(path, []byte("content"), 0644)
		g.Expect(err).ShouldNot(HaveOccurred())
	}

	// Create engine and run analysis
	engine, err := syncengine.NewEngine(sourceDir, destDir)
	g.Expect(err).ShouldNot(HaveOccurred())
	defer engine.Close()

	collector := &eventCollector{}
	engine.SetEventEmitter(collector)

	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify events were emitted
	g.Expect(len(collector.events)).To(BeNumerically(">=", 6),
		"Expected at least: ScanStarted(source), ScanComplete(source), "+
			"ScanStarted(dest), ScanComplete(dest), CompareStarted, CompareComplete")

	// Verify source scan events
	var sourceScanComplete *syncengine.ScanComplete
	for _, evt := range collector.events {
		if sc, ok := evt.(syncengine.ScanComplete); ok && sc.Target == "source" {
			sourceScanComplete = &sc
			break
		}
	}
	g.Expect(sourceScanComplete).ToNot(BeNil(), "Expected ScanComplete for source")
	g.Expect(sourceScanComplete.Count).To(Equal(10),
		"Source scan should report exactly 10 files (the actual count)")

	// Verify dest scan events
	var destScanComplete *syncengine.ScanComplete
	for _, evt := range collector.events {
		if sc, ok := evt.(syncengine.ScanComplete); ok && sc.Target == "dest" {
			destScanComplete = &sc
			break
		}
	}
	g.Expect(destScanComplete).ToNot(BeNil(), "Expected ScanComplete for dest")
	g.Expect(destScanComplete.Count).To(Equal(3),
		"Dest scan should report exactly 3 files (the actual count)")

	// Verify compare complete event
	var compareComplete *syncengine.CompareComplete
	for _, evt := range collector.events {
		if cc, ok := evt.(syncengine.CompareComplete); ok {
			compareComplete = &cc
			break
		}
	}
	g.Expect(compareComplete).ToNot(BeNil(), "Expected CompareComplete event")
	g.Expect(compareComplete.Plan).ToNot(BeNil(), "CompareComplete should have a plan")
	// 7 files need to be copied (10 source - 3 already in dest)
	g.Expect(compareComplete.Plan.FilesToCopy).To(Equal(7),
		"Plan should show 7 files to copy (10 source - 3 already synced)")
}

// TestIntegration_EmptyDirs_MonotonicCountOptimization verifies that empty dirs
// trigger the monotonic count optimization (no full scan events emitted).
func TestIntegration_EmptyDirs_MonotonicCountOptimization(t *testing.T) {
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine, err := syncengine.NewEngine(sourceDir, destDir)
	g.Expect(err).ShouldNot(HaveOccurred())
	defer engine.Close()

	collector := &eventCollector{}
	engine.SetEventEmitter(collector)

	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// With MonotonicCount mode (default), empty dirs match (0 == 0)
	// so the optimization succeeds and no scan events are emitted
	var hasScanComplete bool
	for _, evt := range collector.events {
		if _, ok := evt.(syncengine.ScanComplete); ok {
			hasScanComplete = true
			break
		}
	}
	g.Expect(hasScanComplete).To(BeFalse(),
		"Empty dirs should trigger monotonic count optimization - no full scan events")
}

// TestIntegration_LargerFileSet_CountsMatch verifies counts with more files.
func TestIntegration_LargerFileSet_CountsMatch(t *testing.T) {
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create 100 files in nested directories
	numDirs := 10
	numFilesPerDir := 10
	expectedFileCount := numDirs * numFilesPerDir    // 100 files
	expectedScanCount := expectedFileCount + numDirs // files + directories
	for i := 0; i < numDirs; i++ {
		subdir := filepath.Join(sourceDir, "subdir"+string(rune('0'+i)))
		err := os.MkdirAll(subdir, 0755)
		g.Expect(err).ShouldNot(HaveOccurred())

		for j := 0; j < numFilesPerDir; j++ {
			path := filepath.Join(subdir, "file"+string(rune('0'+j))+".txt")
			err := os.WriteFile(path, []byte("test content"), 0644)
			g.Expect(err).ShouldNot(HaveOccurred())
		}
	}

	engine, err := syncengine.NewEngine(sourceDir, destDir)
	g.Expect(err).ShouldNot(HaveOccurred())
	defer engine.Close()

	collector := &eventCollector{}
	engine.SetEventEmitter(collector)

	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify source scan count (includes files AND directories)
	var sourceScanComplete *syncengine.ScanComplete
	for _, evt := range collector.events {
		if sc, ok := evt.(syncengine.ScanComplete); ok && sc.Target == "source" {
			sourceScanComplete = &sc
			break
		}
	}
	g.Expect(sourceScanComplete).ToNot(BeNil())
	g.Expect(sourceScanComplete.Count).To(Equal(expectedScanCount),
		"Source count includes files and directories")

	// Verify plan shows only files need copying (not directories)
	var compareComplete *syncengine.CompareComplete
	for _, evt := range collector.events {
		if cc, ok := evt.(syncengine.CompareComplete); ok {
			compareComplete = &cc
			break
		}
	}
	g.Expect(compareComplete).ToNot(BeNil())
	g.Expect(compareComplete.Plan.FilesToCopy).To(Equal(expectedFileCount),
		"Plan should show only files to copy (not directories)")
}
