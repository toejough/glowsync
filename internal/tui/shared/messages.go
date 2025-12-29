package shared

import (
	"github.com/joe/copy-files/internal/syncengine"
)

// AnalysisCompleteMsg is sent when analysis is complete
type AnalysisCompleteMsg struct{}

// AnalysisStartedMsg is sent when analysis has started
type AnalysisStartedMsg struct{}

// EngineInitializedMsg is sent when the engine has been created
type EngineInitializedMsg struct {
	Engine *syncengine.Engine
}

// ErrorMsg is sent when an error occurs
type ErrorMsg struct {
	Err error
}

// ============================================================================
// Internal Messages
// These messages are used within screens for internal state management
// ============================================================================

// InitializeEngineMsg is sent to trigger engine initialization
type InitializeEngineMsg struct{}

// StatusUpdateMsg is sent when sync status updates
type StatusUpdateMsg struct {
	Status *syncengine.Status
}

// SyncCompleteMsg is sent when sync is complete
type SyncCompleteMsg struct{}

// ============================================================================
// Transition Messages
// These messages trigger screen transitions and are handled by AppModel
// ============================================================================

// TransitionToAnalysisMsg is sent by InputScreen when paths are validated
type TransitionToAnalysisMsg struct {
	SourcePath string
	DestPath   string
}

// TransitionToSummaryMsg is sent by SyncScreen or AnalysisScreen when done
type TransitionToSummaryMsg struct {
	FinalState string // "complete", "cancelled", "error"
	Err        error  // only set if FinalState is "error"
}

// TransitionToSyncMsg is sent by AnalysisScreen when analysis completes
type TransitionToSyncMsg struct {
	Engine *syncengine.Engine
}
