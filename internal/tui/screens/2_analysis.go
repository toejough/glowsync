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
	fileProgress    progress.Model // For per-file progress bars during sync
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

	// Buffer zones to prevent jitter when worker counts change
	maxCopyingLines  int // High water mark for copying section line count
	maxCleaningLines int // High water mark for cleaning section line count

	// Deletion timing (tracked by TUI since engine StartTime is for copying)
	deletionStartTime time.Time
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

	// Track deletion start time (first time we see deletion in progress)
	if s.deletionStartTime.IsZero() && status.FilesToDelete > 0 && !status.DeletionComplete {
		s.deletionStartTime = time.Now()
	}

	// Track high-water mark for copying section lines (prevents jitter)
	activeFiles := 0
	for _, file := range status.FilesToSync {
		if file.Status == "copying" || file.Status == "opening" || file.Status == "finalizing" {
			activeFiles++
		}
	}

	copyingLines := s.calculateCopyingSectionLines(activeFiles, status)
	if copyingLines > s.maxCopyingLines {
		s.maxCopyingLines = copyingLines
	}

	// Track high-water mark for cleaning section lines
	cleaningLines := s.calculateCleaningSectionLines(status)
	if cleaningLines > s.maxCleaningLines {
		s.maxCleaningLines = cleaningLines
	}
}

// calculateCopyingSectionLines returns the number of lines the copying section will use.
func (s AnalysisScreen) calculateCopyingSectionLines(activeFiles int, status *syncengine.Status) int {
	lines := 0
	if activeFiles > 0 {
		lines = 1  // Header
		lines++    // Progress bar
		lines += 3 // Files, Bytes, Time lines
		lines++    // Blank line
		// Speed line (if present)
		if status.Workers.TotalRate > 0 {
			lines++
		}
		lines++    // Blank line after stats
		lines++    // "Currently Copying" header
		lines += min(activeFiles, 5)
		if activeFiles > 5 {
			lines++ // Overflow
		}
	}
	lines++ // "X copied" line
	return lines
}

// calculateCleaningSectionLines returns the number of lines the cleaning section will use.
func (s AnalysisScreen) calculateCleaningSectionLines(status *syncengine.Status) int {
	if status.FilesToDelete == 0 {
		return 0
	}

	lines := 1     // Header
	lines++        // Progress bar
	lines += 3     // Files, Bytes, Time lines
	lines++        // Blank line
	deletingFiles := len(status.CurrentlyDeleting)
	if deletingFiles > 0 {
		lines++ // "Currently Deleting" header
		lines += min(deletingFiles, 3)
		if deletingFiles > 3 {
			lines++ // Overflow
		}
	}
	lines++ // "X deleted" line
	return lines
}

// NewAnalysisScreen creates a new analysis screen
func NewAnalysisScreen(cfg *config.Config) *AnalysisScreen {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(shared.PrimaryColor())

	// Use shared helper to ensure consistent configuration (ShowPercentage = false)
	overallProg := shared.NewProgressModel(0) // Width set later in resize
	fileProg := shared.NewProgressModel(0)    // For per-file progress during sync

	return &AnalysisScreen{
		config:            cfg,
		spinner:           spin,
		overallProgress:   overallProg,
		fileProgress:      fileProg,
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

	// File progress bars are kept small (max 20 chars) to leave room for file names
	fileProgressWidth := min(progressWidth, 20) //nolint:mnd // UI display limit for progress bar width
	s.fileProgress.Width = fileProgressWidth

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

	// Buffer line between Source and Dest sections
	builder.WriteString("\n")

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
// During live sync mode, this expands to show the full copying section with progress.
func (s AnalysisScreen) renderMissingFromDestLine(builder *strings.Builder) {
	builder.WriteString("  ")

	if s.isLiveMode && s.liveStatus != nil {
		// Live mode: show full copying section
		s.renderCopyingSection(builder)
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
// During live sync mode, this expands to show the full cleaning section with progress.
func (s AnalysisScreen) renderMissingFromSourceLine(builder *strings.Builder) {
	builder.WriteString("  ")

	if s.isLiveMode && s.liveStatus != nil {
		// Live mode: show full cleaning section (symmetric with copying section)
		s.renderCleaningSection(builder)
	} else {
		// Analysis mode: static count
		builder.WriteString(shared.RenderActionItem(fmt.Sprintf(
			"Missing from source: %d files (%s) — to delete",
			s.syncPlan.FilesOnlyInDest,
			shared.FormatBytes(s.syncPlan.BytesOnlyInDest))))
		builder.WriteString("\n")
	}
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

// ============================================================================
// Live Sync Copying Section (moved from SyncScreen)
// ============================================================================

// Indent for items under Source/Dest sections.
const sectionIndent = "  "

// renderCopyingSection renders the full copying progress section during live sync.
func (s AnalysisScreen) renderCopyingSection(builder *strings.Builder) {
	// Show remaining count with transition arrow
	remaining := max(s.originalFilesToCopy-s.liveStatus.ProcessedFiles, 0)

	if remaining != s.originalFilesToCopy {
		builder.WriteString(shared.RenderActionItem(fmt.Sprintf(
			"Copying: %d %s %d files remaining",
			s.originalFilesToCopy, shared.RightArrow(), remaining)))
	} else {
		builder.WriteString(shared.RenderActionItem(fmt.Sprintf(
			"Copying: %d files",
			s.originalFilesToCopy)))
	}
	builder.WriteString("\n\n")

	// Progress bar and stats
	s.renderSyncProgress(builder)

	// Statistics (workers, speed)
	s.renderSyncStatistics(builder)

	// Currently copying files with progress bars
	s.renderCurrentlyCopying(builder)

	// Show completed count (always at same position for stability)
	builder.WriteString(sectionIndent)
	if s.liveStatus.ProcessedFiles > 0 {
		builder.WriteString(shared.RenderSuccess(fmt.Sprintf(
			"%d copied %s", s.liveStatus.ProcessedFiles, shared.SuccessSymbol())))
	}
	builder.WriteString("\n")

	// Add padding to reach high-water mark (prevents jitter when worker count changes)
	activeFiles := 0
	for _, file := range s.liveStatus.FilesToSync {
		if file.Status == "copying" || file.Status == "opening" || file.Status == "finalizing" {
			activeFiles++
		}
	}
	currentLines := s.calculateCopyingSectionLines(activeFiles, s.liveStatus)
	for range s.maxCopyingLines - currentLines {
		builder.WriteString("\n")
	}
}

// renderSyncProgress renders progress bar with files, bytes, and time.
func (s AnalysisScreen) renderSyncProgress(builder *strings.Builder) {
	// Progress bar using overall percentage
	builder.WriteString(sectionIndent)
	builder.WriteString(shared.RenderProgress(s.overallProgress, s.liveStatus.Progress.OverallPercent))
	builder.WriteString("\n")

	// Files line with percentage
	totalProcessedFiles := s.liveStatus.AlreadySyncedFiles + s.liveStatus.ProcessedFiles
	builder.WriteString(sectionIndent)
	fmt.Fprintf(builder, "Files: %d / %d (%.1f%%)",
		totalProcessedFiles,
		s.liveStatus.TotalFilesInSource,
		s.liveStatus.Progress.FilesPercent*shared.ProgressPercentageScale)

	if s.liveStatus.FailedFiles > 0 {
		fmt.Fprintf(builder, " • %d failed", s.liveStatus.FailedFiles)
	}
	builder.WriteString("\n")

	// Bytes line with percentage
	totalProcessedBytes := s.liveStatus.AlreadySyncedBytes + s.liveStatus.TransferredBytes
	builder.WriteString(sectionIndent)
	fmt.Fprintf(builder, "Bytes: %s / %s (%.1f%%)",
		shared.FormatBytes(totalProcessedBytes),
		shared.FormatBytes(s.liveStatus.TotalBytesInSource),
		s.liveStatus.Progress.BytesPercent*shared.ProgressPercentageScale)
	builder.WriteString("\n")

	// Time line: elapsed / estimated (percentage)
	elapsed := time.Since(s.liveStatus.StartTime)
	totalEstimated := elapsed + s.liveStatus.EstimatedTimeLeft

	builder.WriteString(sectionIndent)
	fmt.Fprintf(builder, "Time: %s / %s (%.1f%%)",
		shared.FormatDuration(elapsed),
		shared.FormatDuration(totalEstimated),
		s.liveStatus.Progress.TimePercent*shared.ProgressPercentageScale)
	builder.WriteString("\n\n")
}

// renderSyncStatistics renders worker count and speed.
func (s AnalysisScreen) renderSyncStatistics(builder *strings.Builder) {
	// Worker count
	builder.WriteString(sectionIndent)
	fmt.Fprintf(builder, "Workers: %d", s.liveStatus.ActiveWorkers)

	// Read/write percentage (from rolling window)
	if s.liveStatus.Workers.ReadPercent > 0 || s.liveStatus.Workers.WritePercent > 0 {
		fmt.Fprintf(builder, " • R:%.0f%% / W:%.0f%%",
			s.liveStatus.Workers.ReadPercent,
			s.liveStatus.Workers.WritePercent)
	}
	builder.WriteString("\n")

	// Per-worker and total rates
	if s.liveStatus.Workers.TotalRate > 0 {
		builder.WriteString(sectionIndent)
		fmt.Fprintf(builder, "Speed: %s/worker • %s total",
			shared.FormatRate(s.liveStatus.Workers.PerWorkerRate),
			shared.FormatRate(s.liveStatus.Workers.TotalRate))
		builder.WriteString("\n")
	}
	builder.WriteString("\n")
}

// renderCurrentlyCopying renders active files with colorful progress bars.
//
//nolint:cyclop // Complex rendering logic for file status display
func (s AnalysisScreen) renderCurrentlyCopying(builder *strings.Builder) {
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

	builder.WriteString(sectionIndent)
	builder.WriteString(shared.RenderLabel(fmt.Sprintf("Currently Copying (%d):", len(activeFiles))))
	builder.WriteString("\n")

	// Calculate available width for path display
	boxOverhead := 6
	contentWidth := s.width - boxOverhead
	progressBarWidth := s.fileProgress.Width
	percentageWidth := 8 // " (100.0%)"
	spacing := 4         // spaces between components
	fixedWidth := progressBarWidth + percentageWidth + spacing
	maxPathWidth := max(contentWidth-fixedWidth, 20) //nolint:mnd // Minimum path width

	// Display up to 5 files with progress bars
	maxFilesToShow := 5 //nolint:mnd // Reasonable limit for file list
	filesDisplayed := 0

	for _, file := range activeFiles {
		if filesDisplayed >= maxFilesToShow {
			break
		}

		// Calculate file progress percentage based on status
		var filePercent float64
		var statusMsg string

		switch file.Status {
		case "opening":
			filePercent = 0
			statusMsg = "waiting for dest"
		case "copying":
			if file.Size > 0 {
				filePercent = float64(file.Transferred) / float64(file.Size)
			}
			statusMsg = "copying"
		case "finalizing":
			filePercent = 1.0
			statusMsg = "finalizing"
		}

		// Truncate path to fit available width
		truncPath := shared.TruncatePath(file.RelativePath, maxPathWidth)

		// Format: [indent] [progress bar] [percentage] [path] [status]
		builder.WriteString(sectionIndent)
		fmt.Fprintf(builder, "%s %5.1f%% %s %s\n",
			shared.RenderProgress(s.fileProgress, filePercent),
			filePercent*shared.ProgressPercentageScale,
			shared.FileItemCopyingStyle().Render(truncPath),
			shared.RenderDim(statusMsg))

		filesDisplayed++
	}

	// Show overflow message
	if len(activeFiles) > filesDisplayed {
		builder.WriteString(sectionIndent)
		builder.WriteString(shared.RenderDim(fmt.Sprintf("... and %d more files\n", len(activeFiles)-filesDisplayed)))
	}
}

// ============================================================================
// Live Sync Cleaning Section (symmetric with Copying Section)
// ============================================================================

// renderCleaningSection renders the full cleaning progress section during live sync.
// This is symmetric with renderCopyingSection for the Source section.
func (s AnalysisScreen) renderCleaningSection(builder *strings.Builder) {
	// Show remaining count with transition arrow (mirrors copying section format)
	remaining := max(s.originalFilesToDelete-s.liveStatus.FilesDeleted, 0)

	if remaining != s.originalFilesToDelete {
		// Count has changed - show transition
		builder.WriteString(shared.RenderActionItem(fmt.Sprintf(
			"Cleaning: %d %s %d files remaining",
			s.originalFilesToDelete, shared.RightArrow(), remaining)))
	} else {
		// No change yet
		builder.WriteString(shared.RenderActionItem(fmt.Sprintf(
			"Cleaning: %d files",
			s.originalFilesToDelete)))
	}
	builder.WriteString("\n\n")

	// Progress bar and stats
	s.renderDeletionProgress(builder)

	// Currently deleting files (if any)
	s.renderCurrentlyDeleting(builder)

	// Show completed count (always at same position for stability)
	builder.WriteString(sectionIndent)
	if s.liveStatus.FilesDeleted > 0 {
		builder.WriteString(shared.RenderSuccess(fmt.Sprintf(
			"%d deleted %s", s.liveStatus.FilesDeleted, shared.SuccessSymbol())))
	}
	builder.WriteString("\n")

	// Add padding to reach high-water mark (prevents jitter when deletion count changes)
	currentLines := s.calculateCleaningSectionLines(s.liveStatus)
	for range s.maxCleaningLines - currentLines {
		builder.WriteString("\n")
	}
}

// renderDeletionProgress renders progress bar with files and bytes for deletion.
func (s AnalysisScreen) renderDeletionProgress(builder *strings.Builder) {
	// Calculate progress percentage
	var progressPercent float64
	if s.liveStatus.FilesToDelete > 0 {
		progressPercent = float64(s.liveStatus.FilesDeleted) / float64(s.liveStatus.FilesToDelete)
	} else if s.liveStatus.DeletionComplete {
		progressPercent = 1.0
	}

	// Progress bar
	builder.WriteString(sectionIndent)
	builder.WriteString(shared.RenderProgress(s.overallProgress, progressPercent))
	builder.WriteString("\n")

	// Files line with percentage
	builder.WriteString(sectionIndent)
	fmt.Fprintf(builder, "Files: %d / %d (%.1f%%)",
		s.liveStatus.FilesDeleted,
		s.liveStatus.FilesToDelete,
		progressPercent*shared.ProgressPercentageScale)

	if s.liveStatus.DeletionErrors > 0 {
		fmt.Fprintf(builder, " • %d failed", s.liveStatus.DeletionErrors)
	}
	builder.WriteString("\n")

	// Bytes line with percentage
	var bytesPercent float64
	if s.liveStatus.BytesToDelete > 0 {
		bytesPercent = float64(s.liveStatus.BytesDeleted) / float64(s.liveStatus.BytesToDelete)
	} else if s.liveStatus.DeletionComplete {
		bytesPercent = 1.0
	}

	builder.WriteString(sectionIndent)
	fmt.Fprintf(builder, "Bytes: %s / %s (%.1f%%)",
		shared.FormatBytes(s.liveStatus.BytesDeleted),
		shared.FormatBytes(s.liveStatus.BytesToDelete),
		bytesPercent*shared.ProgressPercentageScale)
	builder.WriteString("\n")

	// Time line: elapsed / estimated (percentage)
	var elapsed time.Duration
	if !s.deletionStartTime.IsZero() {
		elapsed = time.Since(s.deletionStartTime)
	}

	// Estimate remaining time based on progress
	var totalEstimated time.Duration
	if progressPercent > 0 && progressPercent < 1.0 {
		totalEstimated = time.Duration(float64(elapsed) / progressPercent)
	} else if s.liveStatus.DeletionComplete {
		totalEstimated = elapsed
	} else {
		totalEstimated = elapsed // Can't estimate yet
	}

	builder.WriteString(sectionIndent)
	fmt.Fprintf(builder, "Time: %s / %s (%.1f%%)",
		shared.FormatDuration(elapsed),
		shared.FormatDuration(totalEstimated),
		progressPercent*shared.ProgressPercentageScale)
	builder.WriteString("\n\n")
}

// renderCurrentlyDeleting renders files currently being deleted (if any).
func (s AnalysisScreen) renderCurrentlyDeleting(builder *strings.Builder) {
	if len(s.liveStatus.CurrentlyDeleting) == 0 {
		return
	}

	builder.WriteString(sectionIndent)
	builder.WriteString(shared.RenderLabel(fmt.Sprintf("Currently Deleting (%d):", len(s.liveStatus.CurrentlyDeleting))))
	builder.WriteString("\n")

	// Calculate available width for path display
	boxOverhead := 6
	contentWidth := s.width - boxOverhead
	maxPathWidth := max(contentWidth-len(sectionIndent)-4, 20) //nolint:mnd // Minimum path width

	// Display up to 5 files
	maxFilesToShow := 5 //nolint:mnd // Reasonable limit for file list
	filesDisplayed := 0

	for _, filePath := range s.liveStatus.CurrentlyDeleting {
		if filesDisplayed >= maxFilesToShow {
			break
		}

		truncPath := shared.TruncatePath(filePath, maxPathWidth)

		builder.WriteString(sectionIndent)
		builder.WriteString(shared.FileItemErrorStyle().Render("✗ " + truncPath))
		builder.WriteString(" ")
		builder.WriteString(shared.RenderDim("deleting"))
		builder.WriteString("\n")

		filesDisplayed++
	}

	// Show overflow message
	if len(s.liveStatus.CurrentlyDeleting) > filesDisplayed {
		builder.WriteString(sectionIndent)
		builder.WriteString(shared.RenderDim(fmt.Sprintf("... and %d more files\n", len(s.liveStatus.CurrentlyDeleting)-filesDisplayed)))
	}
}
