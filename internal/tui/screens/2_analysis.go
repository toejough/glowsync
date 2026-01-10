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
	lastPhase    string           // Last seen phase, to detect transitions
	lastCount    int              // File count when phase started (to capture result)

	// Event-based state (replaces polling-based state)
	eventBridge      *shared.EventBridge   // Bridge for engine events
	currentScanTarget string               // Current scan target from events
	sourceFileCount  int                   // Source file count from ScanComplete event
	destFileCount    int                   // Dest file count from ScanComplete event
	syncPlan         *syncengine.SyncPlan  // Plan from CompareComplete event
}

// CurrentScanTarget returns the current scan target from events.
func (s AnalysisScreen) CurrentScanTarget() string {
	return s.currentScanTarget
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

// NewAnalysisScreen creates a new analysis screen
func NewAnalysisScreen(cfg *config.Config) *AnalysisScreen {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(shared.PrimaryColor())

	// Use shared helper to ensure consistent configuration (ShowPercentage = false)
	overallProg := shared.NewProgressModel(0) // Width set later in resize

	return &AnalysisScreen{
		config:          cfg,
		spinner:         spin,
		overallProgress: overallProg,
		lastUpdate:      time.Now(),
		seenPhases:      make(map[string]int),
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
	case shared.PhaseDeleting:
		return "Checking for files to delete"
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
		s.currentScanTarget = evt.Target
	case syncengine.ScanComplete:
		s.currentScanTarget = "" // Clear current target
		switch evt.Target {
		case "source":
			s.sourceFileCount = evt.Count
		case "dest":
			s.destFileCount = evt.Count
		}
	case syncengine.CompareStarted:
		// Could update UI state if needed
	case syncengine.CompareComplete:
		s.syncPlan = evt.Plan
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
	// Update status from engine, but only every 200ms to reduce lock contention
	if s.engine != nil {
		now := time.Now()
		if now.Sub(s.lastUpdate) >= shared.StatusUpdateThrottleMs*time.Millisecond {
			s.status = s.engine.GetStatus()
			s.lastUpdate = now
			s.updatePhaseTracking()
		}
	}

	return s, shared.TickCmd()
}

// updatePhaseTracking tracks phase transitions and captures file counts.
// Extracted for testability.
func (s *AnalysisScreen) updatePhaseTracking() {
	if s.status == nil {
		return
	}

	currentPhase := s.status.AnalysisPhase
	if currentPhase != s.lastPhase {
		if s.lastPhase != "" {
			// Record completed phase with the count we tracked
			s.recordCompletedPhase(s.lastPhase, s.lastCount)
		}
		// Start tracking count for new phase - capture current count
		// (don't reset to 0, as the status might already have a count)
		s.lastCount = s.status.ScannedFiles
		s.lastPhase = currentPhase
	} else {
		// Same phase - track the highest count seen
		if s.status.ScannedFiles > s.lastCount {
			s.lastCount = s.status.ScannedFiles
		}
	}
}

// recordCompletedPhase adds a completed phase to the appropriate list with its result.
func (s *AnalysisScreen) recordCompletedPhase(phase string, count int) {
	// For counting phases, if we didn't capture a count (polling missed it),
	// try to get the count from the status totals set by the engine
	actualCount := count
	if count == 0 && s.status != nil {
		switch phase {
		case shared.PhaseCountingSource:
			if s.status.TotalFilesInSource > 0 {
				actualCount = s.status.TotalFilesInSource
			}
		case shared.PhaseCountingDest:
			if s.status.TotalFilesInDest > 0 {
				actualCount = s.status.TotalFilesInDest
			}
		}
	}

	result := fmt.Sprintf("%d files", actualCount)

	// Determine phase category and label
	switch phase {
	case shared.PhaseCountingSource:
		s.seenPhases["source"]++
		label := s.getCountingLabel(s.seenPhases["source"])
		s.sourcePhases = append(s.sourcePhases, completedPhase{text: label, result: result})

	case shared.PhaseCountingDest:
		s.seenPhases["dest"]++
		label := s.getCountingLabel(s.seenPhases["dest"])
		s.destPhases = append(s.destPhases, completedPhase{text: label, result: result})

	case shared.PhaseScanningSource:
		s.sourcePhases = append(s.sourcePhases, completedPhase{text: "Scanning", result: result})

	case shared.PhaseScanningDest:
		s.destPhases = append(s.destPhases, completedPhase{text: "Scanning", result: result})

	default:
		// Other phases (comparing, deleting, etc.)
		s.otherPhases = append(s.otherPhases, s.getPhaseDisplayText(phase))
	}
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

func (s AnalysisScreen) renderAnalysisLog(builder *strings.Builder) {
	if len(s.status.AnalysisLog) == 0 {
		return
	}

	builder.WriteString(shared.RenderLabel("Activity Log:"))
	builder.WriteString("\n")

	// Show last 10 log entries
	startIdx := 0
	if len(s.status.AnalysisLog) > shared.ProgressUpdateInterval {
		startIdx = len(s.status.AnalysisLog) - shared.ProgressUpdateInterval
	}

	for i := startIdx; i < len(s.status.AnalysisLog); i++ {
		fmt.Fprintf(builder, "  %s\n", s.status.AnalysisLog[i])
	}
}

func (s AnalysisScreen) renderAnalysisProgress(builder *strings.Builder) {
	if s.engine == nil || s.status == nil {
		return
	}

	// Calculate progress metrics
	progress := s.status.CalculateAnalysisProgress()

	// Route to appropriate renderer based on phase
	if progress.IsCounting {
		builder.WriteString(s.renderCountingProgress(s.status))
	} else {
		builder.WriteString(s.renderProcessingProgress(s.status, progress))
	}
}

func (s AnalysisScreen) renderAnalyzingView() string {
	// Timeline header + content + box wrapper
	var builder strings.Builder
	builder.WriteString(shared.RenderTimeline("scan"))
	builder.WriteString("\n\n")
	builder.WriteString(s.renderAnalyzingContent())
	return shared.RenderBox(builder.String(), s.width, s.height)
}

// renderAnalyzingContent returns just the analysis content without timeline or box.
func (s AnalysisScreen) renderAnalyzingContent() string {
	var builder strings.Builder

	builder.WriteString(shared.RenderTitle("🔍 Scanning Files"))
	builder.WriteString("\n\n")

	// Always show source section with its phases (even while initializing)
	s.renderPathSection(&builder, "Source", s.config.SourcePath, s.sourcePhases, s.isSourcePhaseActive())

	// Always show dest section with its phases
	s.renderPathSection(&builder, "Dest", s.config.DestPath, s.destPhases, s.isDestPhaseActive())

	if s.status == nil {
		builder.WriteString(s.spinner.View())
		builder.WriteString(" Initializing...\n\n")

		return builder.String()
	}

	// Other completed phases (comparing, deleting, etc.)
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

	// Show current path being scanned
	if s.status.CurrentPath != "" {
		s.renderCurrentPathSection(&builder)
		builder.WriteString("\n")
	}

	// Show errors if any
	if len(s.status.Errors) > 0 {
		builder.WriteString(shared.RenderError(fmt.Sprintf("⚠ Errors: %d", len(s.status.Errors))))
		builder.WriteString("\n\n")
	}

	// Show analysis log
	s.renderAnalysisLog(&builder)

	// Show help text
	builder.WriteString("\n")
	builder.WriteString(shared.RenderDim("Press Esc to change paths • Ctrl+C to exit"))

	return builder.String()
}

// renderPathSection renders a path with its associated phases.
func (s AnalysisScreen) renderPathSection(builder *strings.Builder, label, path string, phases []completedPhase, isActive bool) {
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

	// Current active phase for this path
	if isActive {
		builder.WriteString("  ")
		builder.WriteString(s.spinner.View())
		builder.WriteString(" ")
		builder.WriteString(s.getCurrentPhaseLabel())
		if s.status != nil {
			fmt.Fprintf(builder, "... %d files", s.status.ScannedFiles)
		}
		builder.WriteString("\n")
	}
}

// isSourcePhaseActive returns true if the current phase is a source phase.
func (s AnalysisScreen) isSourcePhaseActive() bool {
	if s.status == nil {
		return false
	}
	return s.status.AnalysisPhase == shared.PhaseCountingSource ||
		s.status.AnalysisPhase == shared.PhaseScanningSource
}

// isDestPhaseActive returns true if the current phase is a dest phase.
func (s AnalysisScreen) isDestPhaseActive() bool {
	if s.status == nil {
		return false
	}
	return s.status.AnalysisPhase == shared.PhaseCountingDest ||
		s.status.AnalysisPhase == shared.PhaseScanningDest
}

// isOtherPhaseActive returns true if the current phase is not source/dest specific.
func (s AnalysisScreen) isOtherPhaseActive() bool {
	if s.status == nil {
		return false
	}
	return !s.isSourcePhaseActive() && !s.isDestPhaseActive() &&
		s.status.AnalysisPhase != shared.StateComplete
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

func (s AnalysisScreen) renderCountingProgress(status *syncengine.Status) string {
	var builder strings.Builder

	// Show elapsed time
	var elapsed time.Duration
	if !status.AnalysisStartTime.IsZero() {
		elapsed = time.Since(status.AnalysisStartTime)
	}

	// Show items found
	builder.WriteString(fmt.Sprintf("Found: %d items", status.ScannedFiles))

	// Show scan rate if available
	if status.AnalysisRate > 0 {
		builder.WriteString(fmt.Sprintf(" (%.1f items/s)", status.AnalysisRate))
	}

	builder.WriteString("\n")

	// Show elapsed time
	if elapsed > 0 {
		builder.WriteString(fmt.Sprintf("Elapsed: %s\n", shared.FormatDuration(elapsed)))
	}

	builder.WriteString("\n")
	builder.WriteString(s.spinner.View())
	builder.WriteString(" Counting...")

	return builder.String()
}

func (s AnalysisScreen) renderCurrentPathSection(builder *strings.Builder) {
	maxWidth := shared.CalculateMaxPathWidth(s.width)
	truncatedPath := shared.RenderPathPlain(s.status.CurrentPath, maxWidth)
	fmt.Fprintf(builder, "Current: %s\n", truncatedPath)
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
