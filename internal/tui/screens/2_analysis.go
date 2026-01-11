package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/shared"
)

// completedPhase tracks a completed phase with its result
type completedPhase struct {
	text   string // e.g., "Counting files (quick check)"
	result string // e.g., "1,234 files"
}

// AnalysisScreen handles engine initialization and file analysis
type AnalysisScreen struct {
	config          *config.Config
	engine          *syncengine.Engine
	status          *syncengine.Status
	spinner         spinner.Model
	overallProgress progress.Model
	lastUpdate      time.Time
	logPath         string
	width           int
	height          int

	// Phase tracking - grouped by source/dest for display
	sourcePhases []completedPhase // Counting phases for source
	destPhases   []completedPhase // Counting phases for dest
	otherPhases  []string         // Scanning, comparing, etc.
	seenPhases   map[string]int   // Track occurrences for context labels

	// Event-based state (replaces polling-based state)
	eventBridge       *shared.EventBridge  // Bridge for engine events
	activeScanTargets map[string]bool      // Active scan targets (for parallel scanning)
	sourceFileCount   int                  // Source file count from ScanComplete event
	destFileCount     int                  // Dest file count from ScanComplete event
	syncPlan          *syncengine.SyncPlan // Plan from CompareComplete event

	// Live sync tracking (populated during sync phase)
	isLiveMode            bool               // True when sync is in progress
	liveStatus            *syncengine.Status // Live status from sync engine
	originalFilesToCopy   int                // Original FilesOnlyInSource at sync start
	originalFilesToDelete int                // Original FilesOnlyInDest at sync start
	originalFilesInBoth   int                // Original FilesInBoth at sync start
}

// CurrentScanTarget returns "source" or "dest" if that target is active, empty otherwise.
// For backwards compatibility with tests, returns first active target found.
func (s AnalysisScreen) CurrentScanTarget() string {
	if s.activeScanTargets["source"] {
		return "source"
	}
	if s.activeScanTargets["dest"] {
		return "dest"
	}
	return ""
}

// SourceFileCount returns the source file count from ScanComplete event.
func (s AnalysisScreen) SourceFileCount() int {
	return s.sourceFileCount
}

// DestFileCount returns the dest file count from ScanComplete event.
func (s AnalysisScreen) DestFileCount() int {
	return s.destFileCount
}

// SyncPlan returns the sync plan from CompareComplete event.
func (s AnalysisScreen) SyncPlan() *syncengine.SyncPlan {
	return s.syncPlan
}

// EnableLiveMode activates live count updates during sync.
// Call this when transitioning to sync phase.
func (s *AnalysisScreen) EnableLiveMode() {
	s.isLiveMode = true
	if s.syncPlan != nil {
		s.originalFilesToCopy = s.syncPlan.FilesOnlyInSource
		s.originalFilesToDelete = s.syncPlan.FilesOnlyInDest
		s.originalFilesInBoth = s.syncPlan.FilesInBoth
	}
}

// UpdateLiveStatus updates the live status during sync.
// Called by UnifiedScreen on each tick during sync phase.
func (s *AnalysisScreen) UpdateLiveStatus(status *syncengine.Status) {
	s.liveStatus = status
}

// NewAnalysisScreen creates a new analysis screen
func NewAnalysisScreen(cfg *config.Config) *AnalysisScreen {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(shared.PrimaryColor())

	// Use shared helper to ensure consistent configuration (ShowPercentage = false)
	overallProg := shared.NewProgressModel(0) // Width set later in resize

	return &AnalysisScreen{
		config:            cfg,
		spinner:           spin,
		overallProgress:   overallProg,
		lastUpdate:        time.Now(),
		seenPhases:        make(map[string]int),
		activeScanTargets: make(map[string]bool),
	}
}

// Init implements tea.Model
func (s AnalysisScreen) Init() tea.Cmd {
	return tea.Batch(
		s.spinner.Tick,
		s.initializeEngine(),
		shared.TickCmd(),
	)
}

// Update implements tea.Model
func (s AnalysisScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return s.handleWindowSize(msg)
	case tea.KeyMsg:
		return s.handleKeyMsg(msg)
	case shared.EngineInitializedMsg:
		return s.handleEngineInitialized(msg)
	case shared.AnalysisCompleteMsg:
		return s.handleAnalysisComplete()
	case shared.ErrorMsg:
		return s.handleError(msg)
	case spinner.TickMsg:
		return s.handleSpinnerTick(msg)
	case shared.TickMsg:
		return s.handleTick()
	case shared.EngineEventMsg:
		return s.handleEngineEvent(msg)
	}

	return s, nil
}

// View implements tea.Model
func (s AnalysisScreen) View() string {
	return s.renderAnalyzingView()
}

// RenderContent returns just the content without timeline header or box wrapper.
// Used by UnifiedScreen to compose multiple screen contents together.
func (s AnalysisScreen) RenderContent() string {
	return s.renderAnalyzingContent()
}

func (s AnalysisScreen) getAnalysisPhaseText() string {
	return s.getPhaseDisplayText(s.status.AnalysisPhase) + "..."
}

// getPhaseDisplayText returns the display text for a phase without trailing ellipsis.
func (s AnalysisScreen) getPhaseDisplayText(phase string) string {
	switch phase {
	case shared.PhaseCountingSource:
		return "Counting files in source"
	case shared.PhaseScanningSource:
		return "Scanning source directory"
	case shared.PhaseCountingDest:
		return "Counting files in destination"
	case shared.PhaseScanningDest:
		return "Scanning destination directory"
	case shared.PhaseComparing:
		return "Comparing files"
	case shared.StateComplete:
		return "Analysis complete"
	default:
		return "Initializing"
	}
}

func (s AnalysisScreen) handleAnalysisComplete() (tea.Model, tea.Cmd) {
	// Check if confirmation should be skipped
	if s.config.SkipConfirmation {
		// Skip confirmation and go directly to sync
		return s, func() tea.Msg {
			return shared.TransitionToSyncMsg{
				Engine:  s.engine,
				LogPath: s.logPath,
			}
		}
	}

	// Show confirmation screen
	return s, func() tea.Msg {
		return shared.TransitionToConfirmationMsg{
			Engine:  s.engine,
			LogPath: s.logPath,
		}
	}
}

func (s AnalysisScreen) handleEngineInitialized(msg shared.EngineInitializedMsg) (tea.Model, tea.Cmd) {
	// Store the engine and configure it
	s.engine = msg.Engine
	s.engine.Workers = s.config.Workers
	s.engine.AdaptiveMode = s.config.AdaptiveMode
	s.engine.ChangeType = s.config.TypeOfChange

	// Create event bridge and wire it to the engine
	s.eventBridge = shared.NewEventBridge()
	s.engine.SetEventEmitter(s.eventBridge)

	// Register status callback (still needed for progress display during polling transition)
	s.engine.RegisterStatusCallback(func(status *syncengine.Status) {
		s.status = status
	})

	// Capture engine in local variable for closures
	engine := s.engine

	// Determine log path
	s.logPath = os.Getenv("COPY_FILES_LOG")
	if s.logPath == "" {
		s.logPath = filepath.Join(os.TempDir(), "copy-files-debug.log")
	}

	return s, tea.Batch(
		func() tea.Msg {
			// Enable file logging for debugging (non-fatal if it fails)
			_ = engine.EnableFileLogging(s.logPath)

			return nil
		},
		func() tea.Msg {
			err := engine.Analyze()
			if err != nil {
				return shared.ErrorMsg{Err: err}
			}

			return shared.AnalysisCompleteMsg{}
		},
		// Start listening for engine events
		s.eventBridge.ListenCmd(),
	)
}

func (s AnalysisScreen) handleError(msg shared.ErrorMsg) (tea.Model, tea.Cmd) {
	// Transition to summary screen with error
	return s, func() tea.Msg {
		return shared.TransitionToSummaryMsg{
			FinalState: "error",
			Err:        msg.Err,
		}
	}
}

// handleEngineEvent processes events from the engine via EventBridge.
func (s AnalysisScreen) handleEngineEvent(msg shared.EngineEventMsg) (tea.Model, tea.Cmd) {
	switch evt := msg.Event.(type) {
	case syncengine.ScanStarted:
		// Add target to active set (supports parallel scanning)
		if s.activeScanTargets == nil {
			s.activeScanTargets = make(map[string]bool)
		}
		s.activeScanTargets[evt.Target] = true
	case syncengine.ScanComplete:
		// Remove target from active set
		delete(s.activeScanTargets, evt.Target)
		switch evt.Target {
		case "source":
			s.sourceFileCount = evt.Count
			// Record completed phase with guaranteed-correct count from event
			s.seenPhases["source"]++
			label := s.getCountingLabel(s.seenPhases["source"])
			result := fmt.Sprintf("%d files", evt.Count)
			s.sourcePhases = append(s.sourcePhases, completedPhase{text: label, result: result})
		case "dest":
			s.destFileCount = evt.Count
			// Record completed phase with guaranteed-correct count from event
			s.seenPhases["dest"]++
			label := s.getCountingLabel(s.seenPhases["dest"])
			result := fmt.Sprintf("%d files", evt.Count)
			s.destPhases = append(s.destPhases, completedPhase{text: label, result: result})
		}
	case syncengine.CompareStarted:
		// No longer adding "Comparing files" - timeline shows state
	case syncengine.CompareComplete:
		s.syncPlan = evt.Plan
		// Comparison results are rendered directly from syncPlan, not added to otherPhases
	}

	// Continue listening for more events
	var cmd tea.Cmd
	if s.eventBridge != nil {
		cmd = s.eventBridge.ListenCmd()
	}
	return s, cmd
}

//nolint:exhaustive // Only handling specific key types
func (s AnalysisScreen) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	//nolint:exhaustive // Default case handles all other keys
	switch msg.Type {
	case tea.KeyCtrlC:
		// Emergency exit - quit immediately
		return s, tea.Quit

	case tea.KeyEsc:
		// Cancel analysis if running
		if s.engine != nil {
			s.engine.Cancel()
		}

		// Transition back to input screen
		return s, func() tea.Msg {
			return shared.TransitionToInputMsg{}
		}
	default:
		// Ignore all other keys
		return s, nil
	}
}

func (s AnalysisScreen) handleSpinnerTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	s.spinner, cmd = s.spinner.Update(msg)

	return s, cmd
}

func (s AnalysisScreen) handleTick() (tea.Model, tea.Cmd) {
	// Update status from engine for live progress display
	// Note: Phase tracking now done via events, not polling
	if s.engine != nil {
		now := time.Now()
		if now.Sub(s.lastUpdate) >= shared.StatusUpdateThrottleMs*time.Millisecond {
			s.status = s.engine.GetStatus()
			s.lastUpdate = now
		}
	}

	return s, shared.TickCmd()
}

// getCountingLabel returns the label for a counting phase based on occurrence.
func (s AnalysisScreen) getCountingLabel(occurrence int) string {
	switch occurrence {
	case 1:
		return "Counting (quick check)"
	case 2:
		return "Counting (full scan)"
	default:
		return "Counting"
	}
}

// ============================================================================
// Message Handlers
// ============================================================================

func (s AnalysisScreen) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	s.width = msg.Width
	s.height = msg.Height

	// Set progress bar width
	//nolint:lll // Complex width calculation with multiple constants
	progressWidth := min(max(msg.Width-shared.ProgressUpdateInterval, shared.ProgressLogThreshold), shared.MaxProgressBarWidth)

	s.overallProgress.Width = progressWidth

	return s, nil
}

// ============================================================================
// Engine Initialization
// ============================================================================

func (s AnalysisScreen) initializeEngine() tea.Cmd {
	return func() tea.Msg {
		engine, err := syncengine.NewEngine(s.config.SourcePath, s.config.DestPath)
		if err != nil {
			return shared.ErrorMsg{Err: fmt.Errorf("failed to initialize engine: %w", err)}
		}

		engine.FilePattern = s.config.FilePattern
		engine.Verbose = s.config.Verbose

		return shared.EngineInitializedMsg{
			Engine: engine,
		}
	}
}

// renderAnalysisLog removed (Issue #40). With parallel scanning, log entries from
// source/dest goroutines interleave and become confusing. The source/dest sections
// and comparison results section now provide the meaningful status information.

func (s AnalysisScreen) renderAnalysisProgress(builder *strings.Builder) {
	// Note: Counting progress display removed (Issue #39).
	// With parallel scanning, ScannedFiles is unreliable (race conditions between goroutines).
	// The source/dest sections already show accurate counts from events.
	// Processing progress also removed (Issue #36) - comparison results section provides
	// the meaningful information about what's happening.
	_ = builder // silence unused parameter warning
}

func (s AnalysisScreen) renderAnalyzingView() string {
	// Timeline header + content + help text + box wrapper (standalone mode)
	var builder strings.Builder
	builder.WriteString(shared.RenderTimeline("scan"))
	builder.WriteString("\n\n")
	builder.WriteString(s.renderAnalyzingContent())
	builder.WriteString("\n")
	builder.WriteString(shared.RenderDim("Esc to go back • Ctrl+C to exit"))
	return shared.RenderBox(builder.String(), s.width, s.height)
}

// renderAnalyzingContent returns just the analysis content without timeline or box.
func (s AnalysisScreen) renderAnalyzingContent() string {
	var builder strings.Builder

	if s.status == nil {
		// Still initializing - show paths with initializing spinner
		builder.WriteString(shared.RenderLabel("Source: "))
		builder.WriteString(s.config.SourcePath)
		builder.WriteString("\n")
		builder.WriteString("  ")
		builder.WriteString(s.spinner.View())
		builder.WriteString(" Initializing...\n")

		builder.WriteString(shared.RenderLabel("Dest: "))
		builder.WriteString(s.config.DestPath)
		builder.WriteString("\n")
		builder.WriteString("  ")
		builder.WriteString(shared.RenderDim("⋯ Waiting"))
		builder.WriteString("\n")

		// Note: Help text removed - shown by active screen in unified view
		return builder.String()
	}

	// Show source section - both scan in parallel so no waiting needed
	s.renderPathSection(&builder, "Source", s.config.SourcePath, s.sourcePhases, s.isSourcePhaseActive(), false)

	// Show "missing from dest" (to copy) - highlighted since it's an action item
	if s.syncPlan != nil && s.syncPlan.FilesOnlyInSource > 0 {
		s.renderMissingFromDestLine(&builder)
	}

	// Show dest section - both scan in parallel so no waiting needed
	s.renderPathSection(&builder, "Dest", s.config.DestPath, s.destPhases, s.isDestPhaseActive(), false)

	// Show "missing from source" (to delete) - highlighted since it's an action item
	if s.syncPlan != nil && s.syncPlan.FilesOnlyInDest > 0 {
		s.renderMissingFromSourceLine(&builder)
	}

	// Comparison results section - files in both locations (no action needed, so dim)
	if s.syncPlan != nil && s.syncPlan.FilesInBoth > 0 {
		s.renderInBothLine(&builder)
	}

	// Other completed phases (if any remaining)
	for _, phase := range s.otherPhases {
		builder.WriteString(shared.SuccessSymbol())
		builder.WriteString(" ")
		builder.WriteString(shared.RenderDim(phase))
		builder.WriteString("\n")
	}

	// Show current phase if it's not a source/dest phase
	if s.isOtherPhaseActive() {
		phaseText := s.getAnalysisPhaseText()
		builder.WriteString(s.spinner.View())
		builder.WriteString(" ")
		builder.WriteString(phaseText)
		builder.WriteString("\n")
	}

	builder.WriteString("\n")

	// Show scan progress with progress bar or count
	s.renderAnalysisProgress(&builder)

	// Note: "Current:" path display removed - was confusing, source/dest sections provide context

	// Show errors if any
	if len(s.status.Errors) > 0 {
		builder.WriteString(shared.RenderError(fmt.Sprintf("⚠ Errors: %d", len(s.status.Errors))))
		builder.WriteString("\n\n")
	}

	// Note: Activity log removed (Issue #40). With parallel scanning, log entries interleave
	// and become confusing. Source/dest sections and comparison results provide the
	// meaningful status information.

	// Note: Help text removed - shown by active screen in unified view
	return builder.String()
}

// renderPathSection renders a path with its associated phases.
// Always outputs exactly header + status line for consistent layout (prevents flicker).
func (s AnalysisScreen) renderPathSection(builder *strings.Builder, label, path string, phases []completedPhase, isActive bool, showWaiting bool) {
	// Path header
	builder.WriteString(shared.RenderLabel(label + ": "))
	builder.WriteString(path)
	builder.WriteString("\n")

	// Completed phases for this path
	for _, phase := range phases {
		builder.WriteString("  ")
		builder.WriteString(shared.SuccessSymbol())
		builder.WriteString(" ")
		builder.WriteString(shared.RenderDim(phase.text))
		builder.WriteString(shared.RenderDim(" → "))
		builder.WriteString(shared.RenderDim(phase.result))
		builder.WriteString("\n")
	}

	// Always show a status line for consistent layout
	if isActive {
		// Current active phase
		builder.WriteString("  ")
		builder.WriteString(s.spinner.View())
		builder.WriteString(" ")
		builder.WriteString(s.getCurrentPhaseLabel())
		if s.status != nil {
			fmt.Fprintf(builder, "... %d files", s.status.ScannedFiles)
		}
		builder.WriteString("\n")
	} else if len(phases) == 0 && showWaiting {
		// Not active and no completed phases - show waiting placeholder
		// Only show for dest while source is being processed
		builder.WriteString("  ")
		builder.WriteString(shared.RenderDim("⋯ Waiting"))
		builder.WriteString("\n")
	}
	// If not active but has phases, the phases already provide structure
}

// isSourcePhaseActive returns true if source is currently being scanned.
// Uses event-based tracking (activeScanTargets) for accuracy with fast operations.
func (s AnalysisScreen) isSourcePhaseActive() bool {
	return s.activeScanTargets["source"]
}

// isDestPhaseActive returns true if dest is currently being scanned.
// Uses event-based tracking (activeScanTargets) for accuracy with fast operations.
func (s AnalysisScreen) isDestPhaseActive() bool {
	return s.activeScanTargets["dest"]
}

// isOtherPhaseActive returns true if the current phase is not source/dest specific
// and is a phase we want to show a spinner for.
func (s AnalysisScreen) isOtherPhaseActive() bool {
	if s.status == nil {
		return false
	}
	// Only show spinner for "comparing" phase
	// Other phases: source/dest scans shown separately, "deleting" covered by comparison results
	return s.status.AnalysisPhase == shared.PhaseComparing
}

// getCurrentPhaseLabel returns the label for the current active phase.
func (s AnalysisScreen) getCurrentPhaseLabel() string {
	if s.status == nil {
		return "Initializing"
	}

	switch s.status.AnalysisPhase {
	case shared.PhaseCountingSource:
		return s.getCountingLabel(s.seenPhases["source"] + 1)
	case shared.PhaseCountingDest:
		return s.getCountingLabel(s.seenPhases["dest"] + 1)
	case shared.PhaseScanningSource, shared.PhaseScanningDest:
		return "Scanning"
	default:
		return s.getPhaseDisplayText(s.status.AnalysisPhase)
	}
}

// ============================================================================
// Rendering
// ============================================================================

func (s AnalysisScreen) renderProcessingProgress(
	status *syncengine.Status,
	progress syncengine.ProgressMetrics,
) string {
	var builder strings.Builder

	// Progress bar using overall percentage
	builder.WriteString(s.overallProgress.ViewAs(progress.OverallPercent))
	builder.WriteString("\n")

	// Files line: "Files: 123 / 456 (27.0%)"
	fmt.Fprintf(&builder, "Files: %d / %d (%.1f%%)\n",
		status.ScannedFiles,
		status.TotalFilesToScan,
		progress.FilesPercent)

	// Bytes line: "Bytes: 1.2 MB / 4.5 MB (26.7%)"
	fmt.Fprintf(&builder, "Bytes: %s / %s (%.1f%%)\n",
		shared.FormatBytes(status.ScannedBytes),
		shared.FormatBytes(status.TotalBytesToScan),
		progress.BytesPercent)

	// Time line: "Time: 00:15 / 00:56 (26.8%)"
	var elapsed time.Duration
	if !status.AnalysisStartTime.IsZero() {
		elapsed = time.Since(status.AnalysisStartTime)
	}

	totalTime := elapsed + progress.EstimatedTimeRemaining
	fmt.Fprintf(&builder, "Time: %s / %s (%.1f%%)\n",
		shared.FormatDuration(elapsed),
		shared.FormatDuration(totalTime),
		progress.TimePercent)

	return builder.String()
}

// ============================================================================
// Live Mode Count Rendering
// ============================================================================

// renderMissingFromDestLine renders the "Missing from dest" count with live updates.
func (s AnalysisScreen) renderMissingFromDestLine(builder *strings.Builder) {
	builder.WriteString("  ")

	if s.isLiveMode && s.liveStatus != nil {
		// Live mode: show "original → current" with progress
		remaining := s.originalFilesToCopy - s.liveStatus.ProcessedFiles
		if remaining < 0 {
			remaining = 0
		}

		if remaining != s.originalFilesToCopy {
			// Count has changed - show transition
			builder.WriteString(shared.RenderActionItem(fmt.Sprintf(
				"Missing from dest: %d %s %d files (copying...)",
				s.originalFilesToCopy, shared.RightArrow(), remaining)))
		} else {
			// No change yet
			builder.WriteString(shared.RenderActionItem(fmt.Sprintf(
				"Missing from dest: %d files — to copy",
				s.originalFilesToCopy)))
		}
		builder.WriteString("\n")

		// Show file-level progress for currently copying files
		s.renderActiveFilesProgress(builder)

		// Show completed count if any files copied
		if s.liveStatus.ProcessedFiles > 0 {
			builder.WriteString("    ")
			builder.WriteString(shared.RenderSuccess(fmt.Sprintf(
				"%d copied %s", s.liveStatus.ProcessedFiles, shared.SuccessSymbol())))
			builder.WriteString("\n")
		}
	} else {
		// Analysis mode: static count
		builder.WriteString(shared.RenderActionItem(fmt.Sprintf(
			"Missing from dest: %d files (%s) — to copy",
			s.syncPlan.FilesOnlyInSource,
			shared.FormatBytes(s.syncPlan.BytesOnlyInSource))))
		builder.WriteString("\n")
	}
}

// renderMissingFromSourceLine renders the "Missing from source" count with live updates.
func (s AnalysisScreen) renderMissingFromSourceLine(builder *strings.Builder) {
	builder.WriteString("  ")

	if s.isLiveMode && s.liveStatus != nil {
		// Live mode: show "original → current" with progress
		remaining := s.originalFilesToDelete - s.liveStatus.FilesDeleted
		if remaining < 0 {
			remaining = 0
		}

		if remaining != s.originalFilesToDelete {
			// Count has changed - show transition
			builder.WriteString(shared.RenderActionItem(fmt.Sprintf(
				"Missing from source: %d %s %d files (cleaning...)",
				s.originalFilesToDelete, shared.RightArrow(), remaining)))
		} else {
			// No change yet (deletion happens during analysis, so usually already complete)
			builder.WriteString(shared.RenderActionItem(fmt.Sprintf(
				"Missing from source: %d files — to delete",
				s.originalFilesToDelete)))
		}

		// Show completed count if any files deleted
		if s.liveStatus.FilesDeleted > 0 {
			builder.WriteString("\n    ")
			builder.WriteString(shared.RenderSuccess(fmt.Sprintf(
				"%d deleted %s", s.liveStatus.FilesDeleted, shared.SuccessSymbol())))
		}
	} else {
		// Analysis mode: static count
		builder.WriteString(shared.RenderActionItem(fmt.Sprintf(
			"Missing from source: %d files (%s) — to delete",
			s.syncPlan.FilesOnlyInDest,
			shared.FormatBytes(s.syncPlan.BytesOnlyInDest))))
	}
	builder.WriteString("\n")
}

// renderInBothLine renders the "In both" count with live updates.
func (s AnalysisScreen) renderInBothLine(builder *strings.Builder) {
	if s.isLiveMode && s.liveStatus != nil {
		// "In both" increases as copies complete
		currentInBoth := s.originalFilesInBoth + s.liveStatus.ProcessedFiles

		if currentInBoth != s.originalFilesInBoth {
			// Count has changed - show transition
			builder.WriteString(shared.RenderDim(fmt.Sprintf(
				"In both: %d %s %d files — synced",
				s.originalFilesInBoth, shared.RightArrow(), currentInBoth)))
		} else {
			// No change yet
			builder.WriteString(shared.RenderDim(fmt.Sprintf(
				"In both: %d files — no action needed",
				s.originalFilesInBoth)))
		}
	} else {
		// Analysis mode: static count
		builder.WriteString(shared.RenderDim(fmt.Sprintf(
			"In both: %d files (%s) — no action needed",
			s.syncPlan.FilesInBoth,
			shared.FormatBytes(s.syncPlan.BytesInBoth))))
	}
	builder.WriteString("\n")
}

// renderActiveFilesProgress renders the currently copying files with progress bars.
func (s AnalysisScreen) renderActiveFilesProgress(builder *strings.Builder) {
	if s.liveStatus == nil || len(s.liveStatus.FilesToSync) == 0 {
		return
	}

	// Find active files (copying, opening, finalizing)
	var activeFiles []*syncengine.FileToSync
	for _, file := range s.liveStatus.FilesToSync {
		if file.Status == "copying" || file.Status == "opening" || file.Status == "finalizing" {
			activeFiles = append(activeFiles, file)
		}
	}

	if len(activeFiles) == 0 {
		return
	}

	// Show up to 3 active files with progress
	maxFiles := 3
	for i, file := range activeFiles {
		if i >= maxFiles {
			break
		}

		// Calculate progress percentage
		var percent float64
		if file.Size > 0 {
			percent = float64(file.Transferred) / float64(file.Size)
		}

		// Determine status label
		var statusLabel string
		switch file.Status {
		case "opening":
			statusLabel = "waiting"
		case "copying":
			statusLabel = ""
		case "finalizing":
			statusLabel = "finalizing"
			percent = 1.0
		}

		// Render: "    [████░░░░░░] 40% filename.mov"
		builder.WriteString("    ")
		builder.WriteString(s.renderMiniProgressBar(percent))
		builder.WriteString(" ")
		builder.WriteString(shared.TruncatePath(file.RelativePath, 30)) //nolint:mnd // Reasonable path width
		if statusLabel != "" {
			builder.WriteString(" ")
			builder.WriteString(shared.RenderDim(statusLabel))
		}
		builder.WriteString("\n")
	}

	// Show overflow
	if len(activeFiles) > maxFiles {
		builder.WriteString(shared.RenderDim(fmt.Sprintf("    ... and %d more\n", len(activeFiles)-maxFiles)))
	}
}

// renderMiniProgressBar renders a compact progress bar.
func (s AnalysisScreen) renderMiniProgressBar(percent float64) string {
	const barWidth = 10
	filled := int(percent * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	return fmt.Sprintf("[%s%s] %3.0f%%",
		strings.Repeat("█", filled),
		strings.Repeat("░", empty),
		percent*100) //nolint:mnd // Percentage conversion
}
