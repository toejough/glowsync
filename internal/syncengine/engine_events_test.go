//go:generate go run github.com/toejough/imptest/impgen --dependency syncengine.EventEmitter

package syncengine_test

import (
	"os"
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
)

// TestEngine_SetEventEmitter verifies that an EventEmitter can be set on the engine.
func TestEngine_SetEventEmitter(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a mock emitter
	emitter := &testEventEmitter{}

	// Create engine with temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine, err := syncengine.NewEngine(sourceDir, destDir)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Should be able to set the event emitter
	engine.SetEventEmitter(emitter)

	// Verify emitter was set (by checking it's returned)
	g.Expect(engine.GetEventEmitter()).To(Equal(emitter))
}

// TestEngine_NilEmitterIsValid verifies that a nil emitter is valid (no-op).
func TestEngine_NilEmitterIsValid(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	engine, err := syncengine.NewEngine(sourceDir, destDir)
	g.Expect(err).ShouldNot(HaveOccurred())

	// By default, emitter should be nil
	g.Expect(engine.GetEventEmitter()).To(BeNil())

	// Engine should work fine without an emitter set
	// (will test this more in scan event tests)
}

// testEventEmitter is a simple test double for capturing events.
type testEventEmitter struct {
	events []syncengine.Event
}

func (e *testEventEmitter) Emit(event syncengine.Event) {
	e.events = append(e.events, event)
}

// TestEngine_Analyze_EmitsScanEvents verifies that Analyze emits scan events.
func TestEngine_Analyze_EmitsScanEvents(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create source with some files
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a test file in source
	createTestFile(t, sourceDir, "test.txt", "hello")

	engine, err := syncengine.NewEngine(sourceDir, destDir)
	g.Expect(err).ShouldNot(HaveOccurred())

	emitter := &testEventEmitter{}
	engine.SetEventEmitter(emitter)

	// Run analysis
	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Verify we got scan events in the right order
	g.Expect(len(emitter.events)).To(BeNumerically(">=", 4), "Expected at least ScanStarted/Complete for source and dest")

	// First event should be ScanStarted for source
	scanStarted, ok := emitter.events[0].(syncengine.ScanStarted)
	g.Expect(ok).To(BeTrue(), "First event should be ScanStarted")
	g.Expect(scanStarted.Target).To(Equal("source"))

	// Should have ScanComplete for source
	var sourceComplete *syncengine.ScanComplete
	for _, evt := range emitter.events {
		if sc, ok := evt.(syncengine.ScanComplete); ok && sc.Target == "source" {
			sourceComplete = &sc
			break
		}
	}
	g.Expect(sourceComplete).ToNot(BeNil(), "Expected ScanComplete for source")
	g.Expect(sourceComplete.Count).To(Equal(1), "Source should have 1 file")

	// Should have ScanStarted for dest
	var destStarted *syncengine.ScanStarted
	for _, evt := range emitter.events {
		if ss, ok := evt.(syncengine.ScanStarted); ok && ss.Target == "dest" {
			destStarted = &ss
			break
		}
	}
	g.Expect(destStarted).ToNot(BeNil(), "Expected ScanStarted for dest")

	// Should have ScanComplete for dest
	var destComplete *syncengine.ScanComplete
	for _, evt := range emitter.events {
		if sc, ok := evt.(syncengine.ScanComplete); ok && sc.Target == "dest" {
			destComplete = &sc
			break
		}
	}
	g.Expect(destComplete).ToNot(BeNil(), "Expected ScanComplete for dest")
	g.Expect(destComplete.Count).To(Equal(0), "Dest should have 0 files")
}

// TestEngine_Analyze_EmitsCompareEvents verifies that Analyze emits compare events.
func TestEngine_Analyze_EmitsCompareEvents(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create source with some files
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	createTestFile(t, sourceDir, "test.txt", "hello")

	engine, err := syncengine.NewEngine(sourceDir, destDir)
	g.Expect(err).ShouldNot(HaveOccurred())

	emitter := &testEventEmitter{}
	engine.SetEventEmitter(emitter)

	// Run analysis
	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Should have CompareStarted
	var compareStarted *syncengine.CompareStarted
	for _, evt := range emitter.events {
		if cs, ok := evt.(syncengine.CompareStarted); ok {
			compareStarted = &cs
			break
		}
	}
	g.Expect(compareStarted).ToNot(BeNil(), "Expected CompareStarted event")

	// Should have CompareComplete with plan
	var compareComplete *syncengine.CompareComplete
	for _, evt := range emitter.events {
		if cc, ok := evt.(syncengine.CompareComplete); ok {
			compareComplete = &cc
			break
		}
	}
	g.Expect(compareComplete).ToNot(BeNil(), "Expected CompareComplete event")
	g.Expect(compareComplete.Plan).ToNot(BeNil(), "CompareComplete should have a plan")
	g.Expect(compareComplete.Plan.FilesToCopy).To(Equal(1), "Plan should show 1 file to copy")
}

// TestEngine_Analyze_NoEventsWithNilEmitter verifies no panic when emitter is nil.
func TestEngine_Analyze_NoEventsWithNilEmitter(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	createTestFile(t, sourceDir, "test.txt", "hello")

	engine, err := syncengine.NewEngine(sourceDir, destDir)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Don't set an emitter - should not panic
	err = engine.Analyze()
	g.Expect(err).ShouldNot(HaveOccurred())
}

// createTestFile creates a test file with the given content.
func createTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := dir + "/" + name
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
}
