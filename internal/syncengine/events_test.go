package syncengine_test

import (
	"testing"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
)

// TestEventTypes_ScanEvents verifies scan phase event types exist with expected fields.
func TestEventTypes_ScanEvents(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// ScanStarted should have Target field
	scanStarted := syncengine.ScanStarted{Target: "source"}
	g.Expect(scanStarted.Target).To(Equal("source"))

	// ScanProgress should have Target and Count fields
	scanProgress := syncengine.ScanProgress{Target: "source", Count: 100}
	g.Expect(scanProgress.Target).To(Equal("source"))
	g.Expect(scanProgress.Count).To(Equal(100))

	// ScanComplete should have Target and Count fields
	scanComplete := syncengine.ScanComplete{Target: "dest", Count: 500}
	g.Expect(scanComplete.Target).To(Equal("dest"))
	g.Expect(scanComplete.Count).To(Equal(500))
}

// TestEventTypes_CompareEvents verifies compare phase event types exist with expected fields.
func TestEventTypes_CompareEvents(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// CompareStarted exists (no fields)
	compareStarted := syncengine.CompareStarted{}
	g.Expect(compareStarted).ToNot(BeNil())

	// CompareProgress should have Compared and Total fields
	compareProgress := syncengine.CompareProgress{Compared: 50, Total: 100}
	g.Expect(compareProgress.Compared).To(Equal(50))
	g.Expect(compareProgress.Total).To(Equal(100))

	// CompareComplete should have Plan field
	plan := &syncengine.SyncPlan{
		FilesToCopy:   100,
		FilesToDelete: 5,
		BytesToCopy:   1024 * 1024,
	}
	compareComplete := syncengine.CompareComplete{Plan: plan}
	g.Expect(compareComplete.Plan).To(Equal(plan))
	g.Expect(compareComplete.Plan.FilesToCopy).To(Equal(100))
	g.Expect(compareComplete.Plan.FilesToDelete).To(Equal(5))
	g.Expect(compareComplete.Plan.BytesToCopy).To(Equal(int64(1024 * 1024)))
}

// TestEventTypes_SyncEvents verifies sync phase event types exist with expected fields.
func TestEventTypes_SyncEvents(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// SyncStarted exists (no fields)
	syncStarted := syncengine.SyncStarted{}
	g.Expect(syncStarted).ToNot(BeNil())

	// SyncProgress should have file and byte progress fields
	syncProgress := syncengine.SyncProgress{
		FilesCopied: 10,
		FilesTotal:  100,
		BytesCopied: 1024,
		BytesTotal:  10240,
	}
	g.Expect(syncProgress.FilesCopied).To(Equal(10))
	g.Expect(syncProgress.FilesTotal).To(Equal(100))
	g.Expect(syncProgress.BytesCopied).To(Equal(int64(1024)))
	g.Expect(syncProgress.BytesTotal).To(Equal(int64(10240)))

	// SyncFileStarted should have Path and Size fields
	fileStarted := syncengine.SyncFileStarted{Path: "videos/test.mov", Size: 1024}
	g.Expect(fileStarted.Path).To(Equal("videos/test.mov"))
	g.Expect(fileStarted.Size).To(Equal(int64(1024)))

	// SyncFileComplete should have Path field
	fileComplete := syncengine.SyncFileComplete{Path: "videos/test.mov"}
	g.Expect(fileComplete.Path).To(Equal("videos/test.mov"))

	// SyncComplete should have Result field
	result := &syncengine.SyncResult{
		FilesCopied:  100,
		FilesDeleted: 5,
		BytesCopied:  10240,
		Errors:       []error{},
	}
	syncComplete := syncengine.SyncComplete{Result: result}
	g.Expect(syncComplete.Result).To(Equal(result))
	g.Expect(syncComplete.Result.FilesCopied).To(Equal(100))
}

// TestEventTypes_ErrorEvent verifies error event type exists with expected fields.
func TestEventTypes_ErrorEvent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// ErrorOccurred should have Phase and Err fields
	err := syncengine.ErrorOccurred{Phase: "scan", Err: nil}
	g.Expect(err.Phase).To(Equal("scan"))
	g.Expect(err.Err).To(BeNil())
}

// TestEventTypes_ImplementEventInterface verifies all events implement the Event interface.
func TestEventTypes_ImplementEventInterface(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// All event types should be assignable to Event interface
	var event syncengine.Event

	event = syncengine.ScanStarted{}
	g.Expect(event).ToNot(BeNil())

	event = syncengine.ScanProgress{}
	g.Expect(event).ToNot(BeNil())

	event = syncengine.ScanComplete{}
	g.Expect(event).ToNot(BeNil())

	event = syncengine.CompareStarted{}
	g.Expect(event).ToNot(BeNil())

	event = syncengine.CompareProgress{}
	g.Expect(event).ToNot(BeNil())

	event = syncengine.CompareComplete{}
	g.Expect(event).ToNot(BeNil())

	event = syncengine.SyncStarted{}
	g.Expect(event).ToNot(BeNil())

	event = syncengine.SyncProgress{}
	g.Expect(event).ToNot(BeNil())

	event = syncengine.SyncFileStarted{}
	g.Expect(event).ToNot(BeNil())

	event = syncengine.SyncFileComplete{}
	g.Expect(event).ToNot(BeNil())

	event = syncengine.SyncComplete{}
	g.Expect(event).ToNot(BeNil())

	event = syncengine.ErrorOccurred{}
	g.Expect(event).ToNot(BeNil())
}

// TestEventEmitter_Interface verifies EventEmitter interface exists.
func TestEventEmitter_Interface(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// EventEmitter interface should exist and have Emit method
	// This test verifies the interface contract by creating a mock implementation
	var emitter syncengine.EventEmitter = &mockEventEmitter{}
	g.Expect(emitter).ToNot(BeNil())

	// Should be able to call Emit with any Event
	emitter.Emit(syncengine.ScanStarted{Target: "source"})
}

// mockEventEmitter is a simple mock for testing the interface exists.
type mockEventEmitter struct {
	events []syncengine.Event
}

func (m *mockEventEmitter) Emit(event syncengine.Event) {
	m.events = append(m.events, event)
}
