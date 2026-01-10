//go:generate go run github.com/toejough/imptest/impgen --dependency syncengine.EventEmitter

package syncengine_test

import (
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
