package shared_test

import (
	"testing"
	"time"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// TestEventBridge_ImplementsEventEmitter verifies the bridge implements EventEmitter.
func TestEventBridge_ImplementsEventEmitter(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bridge := shared.NewEventBridge()
	defer bridge.Close()

	// Should be assignable to EventEmitter
	var emitter syncengine.EventEmitter = bridge
	g.Expect(emitter).ToNot(BeNil())
}

// TestEventBridge_EmitSendsToChan verifies events are sent to the channel.
func TestEventBridge_EmitSendsToChan(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bridge := shared.NewEventBridge()
	defer bridge.Close()

	// Subscribe to events
	eventChan := bridge.Subscribe()

	// Emit an event
	bridge.Emit(syncengine.ScanStarted{Target: "source"})

	// Should receive the event wrapped in a message
	select {
	case msg := <-eventChan:
		eventMsg, ok := msg.(shared.EngineEventMsg)
		g.Expect(ok).To(BeTrue(), "Expected EngineEventMsg")

		scanStarted, ok := eventMsg.Event.(syncengine.ScanStarted)
		g.Expect(ok).To(BeTrue(), "Expected ScanStarted event")
		g.Expect(scanStarted.Target).To(Equal("source"))
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timed out waiting for event")
	}
}

// TestEventBridge_MultipleEvents verifies multiple events are received in order.
func TestEventBridge_MultipleEvents(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bridge := shared.NewEventBridge()
	defer bridge.Close()

	eventChan := bridge.Subscribe()

	// Emit multiple events
	bridge.Emit(syncengine.ScanStarted{Target: "source"})
	bridge.Emit(syncengine.ScanComplete{Target: "source", Count: 100})
	bridge.Emit(syncengine.ScanStarted{Target: "dest"})

	// Receive all three
	events := make([]syncengine.Event, 0, 3)
	for i := 0; i < 3; i++ {
		select {
		case msg := <-eventChan:
			eventMsg := msg.(shared.EngineEventMsg)
			events = append(events, eventMsg.Event)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Timed out waiting for event %d", i)
		}
	}

	g.Expect(len(events)).To(Equal(3))
	_, ok := events[0].(syncengine.ScanStarted)
	g.Expect(ok).To(BeTrue())
	_, ok = events[1].(syncengine.ScanComplete)
	g.Expect(ok).To(BeTrue())
	_, ok = events[2].(syncengine.ScanStarted)
	g.Expect(ok).To(BeTrue())
}

// TestEventBridge_CloseStopsChannel verifies Close stops the channel.
func TestEventBridge_CloseStopsChannel(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bridge := shared.NewEventBridge()
	eventChan := bridge.Subscribe()

	// Close the bridge
	bridge.Close()

	// Channel should be closed
	_, open := <-eventChan
	g.Expect(open).To(BeFalse(), "Channel should be closed")
}

// TestEventBridge_ListenCmd verifies the listen command works with bubble tea.
func TestEventBridge_ListenCmd(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bridge := shared.NewEventBridge()
	defer bridge.Close()

	// Get the listen command
	cmd := bridge.ListenCmd()
	g.Expect(cmd).ToNot(BeNil())

	// Emit an event
	go func() {
		time.Sleep(10 * time.Millisecond)
		bridge.Emit(syncengine.ScanStarted{Target: "source"})
	}()

	// Execute the command (blocks until event received)
	msg := cmd()
	g.Expect(msg).ToNot(BeNil())

	eventMsg, ok := msg.(shared.EngineEventMsg)
	g.Expect(ok).To(BeTrue())
	g.Expect(eventMsg.Event).ToNot(BeNil())
}
