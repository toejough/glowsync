package syncengine

// Event is the interface implemented by all sync engine events.
type Event interface {
	isEvent()
}

// EventEmitter is the interface for emitting events.
type EventEmitter interface {
	Emit(event Event)
}

// Scan phase events

// ScanStarted is emitted when scanning begins for a target (source or dest).
type ScanStarted struct {
	Target string // "source" or "dest"
}

func (ScanStarted) isEvent() {}

// ScanProgress is emitted periodically during scanning.
type ScanProgress struct {
	Target string
	Count  int
}

func (ScanProgress) isEvent() {}

// ScanComplete is emitted when scanning finishes for a target.
type ScanComplete struct {
	Target string
	Count  int
}

func (ScanComplete) isEvent() {}

// Compare phase events

// CompareStarted is emitted when file comparison begins.
type CompareStarted struct{}

func (CompareStarted) isEvent() {}

// CompareProgress is emitted periodically during comparison.
type CompareProgress struct {
	Compared int
	Total    int
}

func (CompareProgress) isEvent() {}

// CompareComplete is emitted when comparison finishes with the sync plan.
type CompareComplete struct {
	Plan *SyncPlan
}

func (CompareComplete) isEvent() {}

// SyncPlan contains the results of analysis - what needs to be synced.
type SyncPlan struct {
	FilesToCopy   int
	FilesToDelete int
	BytesToCopy   int64

	// Comparison counts for TUI display
	FilesInBoth        int   // Files that exist in both source and dest
	FilesOnlyInSource  int   // Files that exist only in source (new files)
	FilesOnlyInDest    int   // Files that exist only in dest (orphans)
	BytesInBoth        int64 // Bytes of files in both (no action needed)
	BytesOnlyInSource  int64 // Bytes to copy (same as BytesToCopy)
	BytesOnlyInDest    int64 // Bytes to delete
}

// Sync phase events

// SyncStarted is emitted when sync execution begins.
type SyncStarted struct{}

func (SyncStarted) isEvent() {}

// SyncProgress is emitted periodically during sync.
type SyncProgress struct {
	FilesCopied int
	FilesTotal  int
	BytesCopied int64
	BytesTotal  int64
}

func (SyncProgress) isEvent() {}

// SyncFileStarted is emitted when a file copy begins.
type SyncFileStarted struct {
	Path string
	Size int64
}

func (SyncFileStarted) isEvent() {}

// SyncFileComplete is emitted when a file copy finishes.
type SyncFileComplete struct {
	Path string
}

func (SyncFileComplete) isEvent() {}

// SyncComplete is emitted when sync execution finishes.
type SyncComplete struct {
	Result *SyncResult
}

func (SyncComplete) isEvent() {}

// SyncResult contains the results of sync execution.
type SyncResult struct {
	FilesCopied  int
	FilesDeleted int
	BytesCopied  int64
	Errors       []error
}

// Error events

// ErrorOccurred is emitted when an error occurs during any phase.
type ErrorOccurred struct {
	Phase string
	Err   error
}

func (ErrorOccurred) isEvent() {}
