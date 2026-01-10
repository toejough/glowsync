package shared

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joe/copy-files/internal/syncengine"
)

// EngineEventMsg wraps a syncengine.Event for use as a tea.Msg.
type EngineEventMsg struct {
	Event syncengine.Event
}

// EventBridge adapts syncengine events to bubble tea messages.
// It implements syncengine.EventEmitter and provides a channel for TUI consumption.
type EventBridge struct {
	eventChan chan tea.Msg
	closed    bool
}

// NewEventBridge creates a new event bridge.
func NewEventBridge() *EventBridge {
	return &EventBridge{
		eventChan: make(chan tea.Msg, 100), // Buffer to prevent blocking engine
	}
}

// Emit implements syncengine.EventEmitter.
// It wraps the event in EngineEventMsg and sends to the channel.
func (b *EventBridge) Emit(event syncengine.Event) {
	if b.closed {
		return
	}
	// Non-blocking send - if channel is full, skip event
	// (This shouldn't happen with adequate buffer and TUI processing)
	select {
	case b.eventChan <- EngineEventMsg{Event: event}:
	default:
		// Channel full, event dropped
	}
}

// Subscribe returns the event channel for receiving events.
func (b *EventBridge) Subscribe() <-chan tea.Msg {
	return b.eventChan
}

// ListenCmd returns a tea.Cmd that blocks until an event is received.
// Use this in Init() or after processing an event to continue listening.
func (b *EventBridge) ListenCmd() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-b.eventChan
		if !ok {
			return nil // Channel closed
		}
		return msg
	}
}

// Close closes the event channel.
// Call this when done with the bridge.
func (b *EventBridge) Close() {
	if !b.closed {
		b.closed = true
		close(b.eventChan)
	}
}
