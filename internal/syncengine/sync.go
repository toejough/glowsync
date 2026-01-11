// Package syncengine provides file synchronization functionality.
package syncengine

//go:generate impgen --target syncengine.Engine.Cancel
//go:generate impgen --target syncengine.Engine.EnableFileLogging
//go:generate impgen --target syncengine.Engine.CloseLog

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/pkg/fileops"
	"github.com/joe/copy-files/pkg/filesystem"
	"github.com/joe/copy-files/pkg/formatters"
)

// Exported constants.
const (
	// AdaptiveScalingHighThreshold is the threshold above which we consider removing workers (110%)
	AdaptiveScalingHighThreshold = 1.10
	// AdaptiveScalingIdleThreshold is the threshold for considering a worker idle (60%)
	AdaptiveScalingIdleThreshold = 0.60
	// AdaptiveScalingLowThreshold is the threshold below which we consider adding workers (90%)
	AdaptiveScalingLowThreshold = 0.90
	// AdaptiveScalingMinIdleTime is the minimum time a worker must be idle before removal (20 seconds)
	AdaptiveScalingMinIdleTime = 20
	// BytesPerKilobyte is the number of bytes in a kilobyte
	BytesPerKilobyte = 1024
	// LogSampleLimit is the maximum number of items to show in log samples
	LogSampleLimit = 10
	// LogSampleSize is the number of sample items to log when showing examples
	LogSampleSize = 5
	// MaxErrorsBeforeAbort is the maximum number of errors before we stop the sync
	MaxErrorsBeforeAbort = 10
	// PercentageScale is the scale factor for converting ratios to percentages
	PercentageScale = 100
	// RecentlyCompletedLimit is the maximum number of recently completed files to track
	RecentlyCompletedLimit = 10
	// WorkerChannelBufferSize is the buffer size for worker control and job channels
	WorkerChannelBufferSize = 100
)

// Exported variables.
var (
	ErrAnalysisCancelled = errors.New("analysis cancelled")
	ErrDeleteFailed      = errors.New("delete failed")
	ErrFilesFailed       = errors.New("file(s) failed to sync")
	ErrSyncAborted       = errors.New("sync aborted")
	ErrTooManyErrors     = errors.New("too many errors, aborting sync")
)

// AdaptiveScalingState holds the state for hill climbing adaptive scaling algorithm
type AdaptiveScalingState struct {
	LastThroughput float64   // Total system throughput in bytes/sec
	LastAdjustment int       // Last direction: +1 (added worker), -1 (removed), 0 (no change)
	LastCheckTime  time.Time // Time of last evaluation
}

// Engine handles the synchronization process
type Engine struct {
	SourcePath      string
	DestPath        string
	FilePattern     string // Optional file pattern filter (e.g., "*.mov")
	Status          *Status
	Workers         int               // Number of concurrent workers (default: 4, 0 = adaptive)
	AdaptiveMode    bool              // Enable adaptive concurrency scaling
	ChangeType      config.ChangeType // Type of changes expected (default: MonotonicCount)
	Verbose         bool              // Enable verbose progress logging
	FileOps         *fileops.FileOps  // File operations (for dependency injection)
	TimeProvider    TimeProvider      // Time provider (for dependency injection)
	emitter         EventEmitter      // Event emitter for TUI communication (optional)
	statusCallbacks []func(*Status)
	mu              sync.RWMutex
	cancelChan      chan struct{} // Channel to signal cancellation
	cancelOnce      sync.Once     // Ensure Cancel() is only called once
	logFile         *os.File      // Optional log file for debugging
	logMu           sync.Mutex    // Mutex for log file writes
	closeFunc       func()        // Function to close SFTP connections (if any)
	desiredWorkers  int32         // Target worker count for adaptive scaling (atomic)
	sourceResizable filesystem.ResizablePool
	destResizable   filesystem.ResizablePool

	// File maps from analysis phase (stored for deletion during sync)
	analysisSourceFiles map[string]*fileops.FileInfo
	analysisDestFiles   map[string]*fileops.FileInfo
}

// NewEngine creates a new sync engine.
// Supports both local paths and SFTP URLs (sftp://user@host:port/path).
// Returns (*Engine, error) where error indicates filesystem setup failure.
func NewEngine(source, dest string) (*Engine, error) {
	// Create filesystems for source and destination
	// Returns: sourceFS, destFS, srcPath, dstPath, closer, err
	sourceFS, destFS, srcPath, dstPath, closer, err := filesystem.CreateFileSystemPair(source, dest)
	if err != nil {
		return nil, fmt.Errorf("failed to create filesystems: %w", err)
	}

	engine := &Engine{
		SourcePath:   srcPath,
		DestPath:     dstPath,
		TimeProvider: &RealTimeProvider{},
		Workers:      config.DefaultMaxWorkers,                 // Default to 4 concurrent workers
		ChangeType:   config.MonotonicCount,                    // Default to monotonic count
		FileOps:      fileops.NewDualFileOps(sourceFS, destFS), // Support cross-filesystem operations
		Status: &Status{
			StartTime: time.Now(),
		},
		statusCallbacks: make([]func(*Status), 0),
		cancelChan:      make(chan struct{}),
		closeFunc:       closer, // Store closer to clean up SFTP connections
	}

	// Detect if filesystems implement ResizablePool
	if resizable, ok := sourceFS.(filesystem.ResizablePool); ok {
		engine.sourceResizable = resizable
	}
	if resizable, ok := destFS.(filesystem.ResizablePool); ok {
		engine.destResizable = resizable
	}

	return engine, nil
}

// SetEventEmitter sets the event emitter for TUI communication.
// The emitter is optional - if nil, no events will be emitted.
func (e *Engine) SetEventEmitter(emitter EventEmitter) {
	e.emitter = emitter
}

// GetEventEmitter returns the current event emitter.
func (e *Engine) GetEventEmitter() EventEmitter {
	return e.emitter
}

// emit sends an event if an emitter is configured.
// Safe to call even when emitter is nil.
func (e *Engine) emit(event Event) {
	if e.emitter != nil {
		e.emitter.Emit(event)
	}
}

// Analyze scans source and destination to determine what needs to be synced
func (e *Engine) Analyze() error {
	e.logAnalysis("Starting analysis...")

	err := e.checkCancellation()
	if err != nil {
		return err
	}

	// Try monotonic-count optimization if applicable
	optimized, err := e.tryMonotonicCountOptimization()
	if err != nil {
		return err
	}

	if optimized {
		return nil
	}

	// Scan source and destination directories in parallel
	var sourceFiles, destFiles map[string]*fileops.FileInfo
	var sourceErr, destErr error
	var wg sync.WaitGroup
	wg.Add(2) //nolint:mnd // Two parallel scans

	// Emit ScanStarted for both immediately
	e.emit(ScanStarted{Target: "source"})
	e.emit(ScanStarted{Target: "dest"})

	go func() {
		defer wg.Done()
		sourceFiles, sourceErr = e.scanSourceDirectory()
		if sourceErr == nil {
			e.emit(ScanComplete{Target: "source", Count: len(sourceFiles)})
		}
	}()

	go func() {
		defer wg.Done()
		destFiles, destErr = e.scanDestinationDirectory()
		if destErr == nil {
			e.emit(ScanComplete{Target: "dest", Count: len(destFiles)})
		}
	}()

	wg.Wait()

	// Check for errors after both complete
	if sourceErr != nil {
		return sourceErr
	}
	if destErr != nil {
		return destErr
	}

	err = e.checkCancellation()
	if err != nil {
		return err
	}

	e.logSamplePaths(sourceFiles, destFiles)

	// Compare files and determine which need sync
	e.emit(CompareStarted{})
	err = e.compareAndPlanSync(sourceFiles, destFiles)
	if err != nil {
		return err
	}

	// Store file maps for deletion during sync phase
	e.analysisSourceFiles = sourceFiles
	e.analysisDestFiles = destFiles

	// Count orphaned items (for plan display) but don't delete yet - deletion happens during sync
	e.countOrphanedItemsForPlan(sourceFiles, destFiles)

	e.finalizeAnalysis()

	// Emit compare complete with sync plan
	e.Status.mu.RLock()
	plan := &SyncPlan{
		FilesToCopy:       len(e.Status.FilesToSync),
		FilesToDelete:     e.Status.FilesOnlyInDest,
		BytesToCopy:       e.Status.TotalBytes,
		FilesInBoth:       e.Status.FilesInBoth,
		FilesOnlyInSource: e.Status.FilesOnlyInSource,
		FilesOnlyInDest:   e.Status.FilesOnlyInDest,
		BytesInBoth:       e.Status.BytesInBoth,
		BytesOnlyInSource: e.Status.BytesOnlyInSource,
		BytesOnlyInDest:   e.Status.BytesOnlyInDest,
	}
	e.Status.mu.RUnlock()
	e.emit(CompareComplete{Plan: plan})

	return nil
}

// Cancel stops the sync operation gracefully
func (e *Engine) Cancel() {
	e.cancelOnce.Do(func() {
		close(e.cancelChan)
	})
}

// Close cleans up resources, including SFTP connections if any.
// Should be called when done with the engine.
func (e *Engine) Close() {
	e.CloseLog()
	if e.closeFunc != nil {
		e.closeFunc()
	}
}

// CloseLog closes the log file if open
func (e *Engine) CloseLog() {
	if e.logFile != nil {
		e.logToFile(fmt.Sprintf("\n=== Sync Log Ended: %s ===", time.Now().Format(time.RFC3339)))
		_ = e.logFile.Close()
		e.logFile = nil
	}
}

// EnableFileLogging enables logging to a file for debugging
func (e *Engine) EnableFileLogging(logPath string) error {
	f, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	e.logFile = f
	e.logToFile(fmt.Sprintf("=== Sync Log Started: %s ===", time.Now().Format(time.RFC3339)))
	e.logToFile("Source: " + e.SourcePath)
	e.logToFile("Destination: " + e.DestPath)
	e.logToFile(fmt.Sprintf("Workers: %d, Adaptive: %v, ChangeType: %v", e.Workers, e.AdaptiveMode, e.ChangeType))
	e.logToFile("")

	return nil
}

// EvaluateAndScale evaluates current performance and decides whether to add workers
//
//nolint:lll,revive // Long function signature with many parameters, currentProcessedFiles reserved for future use
func (e *Engine) EvaluateAndScale(state *AdaptiveScalingState, currentProcessedFiles, currentWorkers int, currentBytes int64, maxWorkers int, workerControl chan bool) {
	now := e.TimeProvider.Now()

	// Calculate current total throughput using rolling window metrics (hill climbing algorithm)
	if !state.LastCheckTime.IsZero() { //nolint:nestif // Nested conditions required for hill climbing algorithm
		elapsed := now.Sub(state.LastCheckTime).Seconds()

		if elapsed > 0 {
			var currentThroughput float64

			// Try to use smoothed total throughput from rolling window
			workerMetrics := e.Status.calculateWorkerMetrics()

			// Need at least 2 samples for meaningful comparison
			if len(e.Status.Workers.RecentSamples) >= 2 { //nolint:mnd // Minimum samples needed
				// Use smoothed total rate from rolling window
				currentThroughput = workerMetrics.TotalRate

				//nolint:lll // Log message with multiple formatted values
				e.logToFile(fmt.Sprintf("HillClimbing: Evaluation at %.1fs - %d workers, total throughput: %.2f MB/s (prev: %.2f MB/s) [%d samples]",
					elapsed, currentWorkers, currentThroughput/BytesPerKilobyte/BytesPerKilobyte, state.LastThroughput/BytesPerKilobyte/BytesPerKilobyte, len(e.Status.Workers.RecentSamples)))
			} else {
				// Fall back to raw point-to-point calculation when insufficient samples
				currentThroughput = float64(currentBytes) / elapsed

				//nolint:lll // Log message with multiple formatted values
				e.logToFile(fmt.Sprintf("HillClimbing: Evaluation at %.1fs - %d workers, total throughput: %.2f MB/s (prev: %.2f MB/s) [raw - need more samples]",
					elapsed, currentWorkers, currentThroughput/BytesPerKilobyte/BytesPerKilobyte, state.LastThroughput/BytesPerKilobyte/BytesPerKilobyte))
			}

			// Make scaling decision using hill climbing algorithm based on total throughput
			newState := e.HillClimbingScalingDecision(state, currentThroughput, currentWorkers, maxWorkers, workerControl)

			// Update state with new values from hill climbing
			*state = *newState
		}
	} else {
		// First check - initialize state and let hill climbing handle it on next evaluation
		state.LastCheckTime = now
		e.logToFile(fmt.Sprintf("HillClimbing: Initial baseline - starting with %d workers", currentWorkers))
	}
}

//nolint:funlen,cyclop // Complex status copying requires comprehensive field copying and multiple conditions
func (e *Engine) GetStatus() *Status {
	e.Status.mu.Lock()
	defer e.Status.mu.Unlock()

	// Compute progress metrics before copying
	e.Status.ComputeProgressMetrics()

	// Create a new status without the mutex
	status := &Status{
		TotalFiles:         e.Status.TotalFiles,
		ProcessedFiles:     e.Status.ProcessedFiles,
		FailedFiles:        e.Status.FailedFiles,
		CancelledFiles:     e.Status.CancelledFiles,
		TotalBytes:         e.Status.TotalBytes,
		TransferredBytes:   atomic.LoadInt64(&e.Status.TransferredBytes),
		CurrentFile:        e.Status.CurrentFile,
		CurrentFileBytes:   e.Status.CurrentFileBytes,
		CurrentFileTotal:   e.Status.CurrentFileTotal,
		StartTime:          e.Status.StartTime,
		EndTime:            e.Status.EndTime,
		BytesPerSecond:     e.Status.BytesPerSecond,
		EstimatedTimeLeft:  e.Status.EstimatedTimeLeft,
		CompletionTime:     e.Status.CompletionTime,
		TotalFilesInSource: e.Status.TotalFilesInSource,
		TotalFilesInDest:   e.Status.TotalFilesInDest,
		TotalBytesInSource: e.Status.TotalBytesInSource,
		AlreadySyncedFiles: e.Status.AlreadySyncedFiles,
		AlreadySyncedBytes: e.Status.AlreadySyncedBytes,
		AnalysisPhase:      e.Status.AnalysisPhase,
		ScannedFiles:       e.Status.ScannedFiles,
		TotalFilesToScan:   e.Status.TotalFilesToScan,
		CurrentPath:        e.Status.CurrentPath,
		ActiveWorkers:      atomic.LoadInt32(&e.Status.ActiveWorkers),
		MaxWorkers:         e.Status.MaxWorkers,
		AdaptiveMode:       e.Status.AdaptiveMode,
		TotalReadTime:      e.Status.TotalReadTime,
		TotalWriteTime:     e.Status.TotalWriteTime,
		Bottleneck:         e.Status.Bottleneck,
		Progress:           e.Status.Progress,
		Workers:            e.Status.Workers,
		FinalizationPhase:  e.Status.FinalizationPhase,
	}

	// Copy CurrentFiles slice (small, actively displayed)
	status.CurrentFiles = make([]string, len(e.Status.CurrentFiles))
	copy(status.CurrentFiles, e.Status.CurrentFiles)

	// Copy Errors slice (usually small)
	status.Errors = make([]FileError, len(e.Status.Errors))
	copy(status.Errors, e.Status.Errors)

	// Copy CancelledCopies slice (usually small)
	status.CancelledCopies = make([]string, len(e.Status.CancelledCopies))
	copy(status.CancelledCopies, e.Status.CancelledCopies)

	// Copy AnalysisLog slice (capped at ~10 entries)
	status.AnalysisLog = make([]string, len(e.Status.AnalysisLog))
	copy(status.AnalysisLog, e.Status.AnalysisLog)

	// Copy RecentlyCompleted slice
	status.RecentlyCompleted = make([]string, len(e.Status.RecentlyCompleted))
	copy(status.RecentlyCompleted, e.Status.RecentlyCompleted)

	// Copy analysis progress tracking fields
	status.ScannedBytes = e.Status.ScannedBytes
	status.TotalBytesToScan = e.Status.TotalBytesToScan
	status.AnalysisStartTime = e.Status.AnalysisStartTime
	status.AnalysisRate = e.Status.AnalysisRate

	// Copy separate source/dest scan progress fields
	status.SourceScannedFiles = e.Status.SourceScannedFiles
	status.SourceTotalFiles = e.Status.SourceTotalFiles
	status.DestScannedFiles = e.Status.DestScannedFiles
	status.DestTotalFiles = e.Status.DestTotalFiles

	// Copy comparison result fields
	status.FilesInBoth = e.Status.FilesInBoth
	status.FilesOnlyInSource = e.Status.FilesOnlyInSource
	status.FilesOnlyInDest = e.Status.FilesOnlyInDest
	status.BytesInBoth = e.Status.BytesInBoth
	status.BytesOnlyInSource = e.Status.BytesOnlyInSource
	status.BytesOnlyInDest = e.Status.BytesOnlyInDest

	// Copy deletion progress tracking fields
	status.FilesToDelete = e.Status.FilesToDelete
	status.FilesDeleted = e.Status.FilesDeleted
	status.BytesToDelete = e.Status.BytesToDelete
	status.BytesDeleted = e.Status.BytesDeleted
	status.DeletionComplete = e.Status.DeletionComplete
	status.DeletionErrors = e.Status.DeletionErrors

	// Copy CurrentlyDeleting slice
	status.CurrentlyDeleting = make([]string, len(e.Status.CurrentlyDeleting))
	copy(status.CurrentlyDeleting, e.Status.CurrentlyDeleting)

	// Only copy recently active files from FilesToSync to reduce lock time
	// PRIORITY: Always include files in CurrentFiles (actively being worked on)
	// THEN: Fill remaining slots with recently completed files for context
	// This prevents holding the lock for milliseconds copying 1000+ file pointers
	status.FilesToSync = make([]*FileToSync, 0, AdaptiveScalingMinIdleTime)

	// Create a map of CurrentFiles for O(1) lookup
	currentFilesMap := make(map[string]bool, len(e.Status.CurrentFiles))
	for _, currentFile := range e.Status.CurrentFiles {
		currentFilesMap[currentFile] = true
	}

	// Step 1: Add ALL files from CurrentFiles first (these are actively being worked on)
	for _, file := range e.Status.FilesToSync {
		if currentFilesMap[file.RelativePath] {
			status.FilesToSync = append(status.FilesToSync, file)
		}
	}

	// Step 2: Fill remaining slots (up to 20 total) with recently active files
	maxRecent := AdaptiveScalingMinIdleTime // 20 files total
	remainingSlots := maxRecent - len(status.FilesToSync)

	if remainingSlots > 0 {
		recentCount := 0
		// Iterate backwards from end (most recent files)
		for i := len(e.Status.FilesToSync) - 1; i >= 0 && recentCount < remainingSlots; i-- {
			file := e.Status.FilesToSync[i]
			// Skip if already added (was in CurrentFiles)
			if currentFilesMap[file.RelativePath] {
				continue
			}
			//nolint:lll // Condition checks multiple status values for UI display filtering
			if file.Status == fileStatusOpening || file.Status == fileStatusCopying || file.Status == fileStatusFinalizing || file.Status == fileStatusComplete || file.Status == fileStatusError {
				status.FilesToSync = append([]*FileToSync{file}, status.FilesToSync...)
				recentCount++
			}
		}
	}

	return status
}

// HillClimbingScalingDecision makes a scaling decision using hill climbing algorithm.
// It tracks total system throughput (bytes/sec) and adjusts workers based on:
// - Initial: optimistically add worker
// - Improved (>5%): continue in same direction
// - Degraded (<-5%): reverse direction
// - Flat (±5%): random perturbation
//
//nolint:cyclop,gocognit,nestif,funlen,gocritic // Hill climbing algorithm requires complex branching logic
func (e *Engine) HillClimbingScalingDecision(
	state *AdaptiveScalingState,
	currentThroughput float64,
	currentWorkers, maxWorkers int,
	workerControl chan bool,
) *AdaptiveScalingState {
	const (
		improvementThreshold = 1.05 // >5% improvement
		degradationThreshold = 0.95 // <5% degradation
	)

	// Initialize desiredWorkers to currentWorkers if not yet set
	if atomic.LoadInt32(&e.desiredWorkers) == 0 {
		atomic.StoreInt32(&e.desiredWorkers, int32(currentWorkers)) //nolint:gosec // Small value, no overflow risk
	}

	// Determine adjustment direction
	var adjustment int

	if state.LastThroughput == 0 {
		// First measurement - optimistically add worker
		adjustment = 1
		e.logToFile(fmt.Sprintf("HillClimbing: First measurement (%.2f MB/s), optimistically adding worker",
			currentThroughput/BytesPerKilobyte/BytesPerKilobyte))
	} else {
		// Calculate throughput ratio
		throughputRatio := currentThroughput / state.LastThroughput

		// Get current desired to check boundaries
		currentDesired := int(atomic.LoadInt32(&e.desiredWorkers))

		if throughputRatio > improvementThreshold {
			// Throughput improved >5% - continue in same direction
			// Special case: if last adjustment was 0 (stayed at boundary), use random perturbation
			if state.LastAdjustment == 0 {
				adjustment = rand.Intn(2)*2 - 1 //nolint:gosec,mnd // Random perturbation for hill climbing (non-crypto use)
				e.logToFile(fmt.Sprintf("HillClimbing: Throughput improved (+%.1f%%), last was boundary hold, random %+d",
					(throughputRatio-1)*PercentageScale, adjustment))
			} else {
				adjustment = state.LastAdjustment
				e.logToFile(fmt.Sprintf("HillClimbing: Throughput improved (+%.1f%%), continuing direction %+d",
					(throughputRatio-1)*PercentageScale, adjustment))
			}
		} else if throughputRatio < degradationThreshold {
			// Throughput degraded >5% - normally reverse direction
			// Special case: if we're at min/max boundary and were heading towards it,
			// don't reverse (to avoid immediate oscillation at boundaries)
			if (currentDesired == 1 && state.LastAdjustment == -1) ||
				(currentDesired == maxWorkers && state.LastAdjustment == 1) {
				// At boundary, don't reverse - stay put
				adjustment = 0
				e.logToFile(fmt.Sprintf("HillClimbing: Throughput degraded (%.1f%%) at boundary (%d workers), staying put",
					(throughputRatio-1)*PercentageScale, currentDesired))
			} else {
				// Normal case: reverse direction
				// Special case: if last adjustment was 0 (stayed at boundary), use random perturbation
				if state.LastAdjustment == 0 {
					adjustment = rand.Intn(2)*2 - 1 //nolint:gosec,mnd // Random perturbation for hill climbing (non-crypto use)
					e.logToFile(fmt.Sprintf("HillClimbing: Throughput degraded (%.1f%%), last was boundary hold, random %+d",
						(throughputRatio-1)*PercentageScale, adjustment))
				} else {
					adjustment = -state.LastAdjustment
					e.logToFile(fmt.Sprintf("HillClimbing: Throughput degraded (%.1f%%), reversing direction to %+d",
						(throughputRatio-1)*PercentageScale, adjustment))
				}
			}
		} else {
			// Throughput flat (±5%) - random perturbation
			// Use simple random: rand.Intn(2) gives 0 or 1, multiply by 2 gives 0 or 2, subtract 1 gives -1 or 1
			adjustment = rand.Intn(2)*2 - 1 //nolint:gosec,mnd // Non-crypto random perturbation for hill climbing
			e.logToFile(fmt.Sprintf("HillClimbing: Throughput flat (%.1f%%), random perturbation %+d",
				(throughputRatio-1)*PercentageScale, adjustment))
		}
	}

	// Execute adjustment with bounds checking
	if adjustment != 0 {
		// Get current desired workers
		currentDesired := atomic.LoadInt32(&e.desiredWorkers)

		// Calculate new desired with bounds
		newDesired := int(currentDesired) + adjustment
		if newDesired < 1 {
			newDesired = 1
		} else if newDesired > maxWorkers {
			newDesired = maxWorkers
		}

		// Only apply if within bounds
		if newDesired == int(currentDesired) {
			// Hit a bound, no change
			e.logToFile(fmt.Sprintf("HillClimbing: Bounded at %d workers (min: 1, max: %d)", newDesired, maxWorkers))
			adjustment = 0 // No actual adjustment made
		} else {
			// Apply the adjustment
			atomic.StoreInt32(&e.desiredWorkers, int32(newDesired)) //nolint:gosec // Small value, no overflow risk
			e.resizePools(newDesired)

			if adjustment > 0 {
				// Add worker - but only if actual count is below target
				// (Workers may not have exited yet from previous removal)
				if currentWorkers < newDesired {
					workerControl <- true
					e.logToFile(fmt.Sprintf("HillClimbing: Adding worker (desired: %d -> %d, active: %d)",
						currentDesired, newDesired, currentWorkers))
				} else {
					e.logToFile(fmt.Sprintf("HillClimbing: Target %d -> %d, %d workers active (waiting)",
						currentDesired, newDesired, currentWorkers))
				}
			} else {
				// Remove worker (workers will self-exit when they notice)
				e.logToFile(fmt.Sprintf("HillClimbing: Removing worker (desired: %d -> %d, active: %d)",
					currentDesired, newDesired, currentWorkers))
			}
		}
	}

	// Return updated state
	return &AdaptiveScalingState{
		LastThroughput: currentThroughput,
		LastAdjustment: adjustment,
		LastCheckTime:  e.TimeProvider.Now(),
	}
}

// LogVerbose logs verbose progress information (only when Verbose is enabled)
func (e *Engine) LogVerbose(message string) {
	if !e.Verbose {
		return
	}

	e.logToFile(message)
}

// MakeScalingDecision decides whether to add workers based on per-worker speed comparison
//
//nolint:lll // Long function signature with many parameters
func (e *Engine) MakeScalingDecision(lastPerWorkerSpeed, currentPerWorkerSpeed float64, currentWorkers, maxWorkers int, workerControl chan bool) {
	// First measurement - add a worker to test
	if lastPerWorkerSpeed == 0 {
		if currentWorkers < maxWorkers {
			newDesired := atomic.AddInt32(&e.desiredWorkers, 1)
			e.resizePools(int(newDesired))
			workerControl <- true

			e.logToFile(fmt.Sprintf("Adaptive: First measurement complete, adding worker (%d -> %d)",
				currentWorkers, currentWorkers+1))
		}

		return
	}

	speedRatio := currentPerWorkerSpeed / lastPerWorkerSpeed

	// Per-worker speed decreased - remove a worker
	if speedRatio < AdaptiveScalingLowThreshold {
		// Decrement desired worker count (workers will self-terminate)
		newDesired := atomic.AddInt32(&e.desiredWorkers, -1)
		if newDesired < 1 {
			// Don't go below 1 worker
			atomic.StoreInt32(&e.desiredWorkers, 1)
			newDesired = 1
		}
		e.resizePools(int(newDesired))

		e.logToFile(fmt.Sprintf("Adaptive: ↓ Per-worker speed decreased (-%.1f%%), removing worker (%d -> %d)",
			(1-speedRatio)*PercentageScale, currentWorkers, newDesired))

		return
	}

	// Per-worker speed maintained or improved - add a worker
	if currentWorkers >= maxWorkers {
		return
	}

	newDesired := atomic.AddInt32(&e.desiredWorkers, 1)
	e.resizePools(int(newDesired))
	workerControl <- true

	if speedRatio >= AdaptiveScalingHighThreshold {
		e.logToFile(fmt.Sprintf("Adaptive: ↑ Per-worker speed improved (+%.1f%%), adding worker (%d -> %d)",
			(speedRatio-1)*PercentageScale, currentWorkers, currentWorkers+1))
	} else {
		e.logToFile(fmt.Sprintf("Adaptive: → Per-worker speed stable, adding worker to test (%d -> %d)",
			currentWorkers, currentWorkers+1))
	}
}

// RegisterStatusCallback registers a callback for status updates
func (e *Engine) RegisterStatusCallback(callback func(*Status)) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.statusCallbacks = append(e.statusCallbacks, callback)
}

// Sync performs the actual synchronization using parallel workers
func (e *Engine) Sync() error {
	if e.AdaptiveMode {
		return e.syncAdaptive()
	}

	return e.syncFixed()
}

// applyFileFilter applies the file pattern filter to the given files
func (e *Engine) applyFileFilter(files map[string]*fileops.FileInfo) map[string]*fileops.FileInfo {
	filter := NewGlobFilter(e.FilePattern)
	filtered := make(map[string]*fileops.FileInfo)

	for relativePath, info := range files {
		if filter.ShouldInclude(relativePath) {
			filtered[relativePath] = info
		}
	}

	return filtered
}

func (e *Engine) checkCancellation() error {
	select {
	case <-e.cancelChan:
		return ErrAnalysisCancelled
	default:
		return nil
	}
}

func (e *Engine) collectDirectoriesToDelete(sourceFiles, destFiles map[string]*fileops.FileInfo) []dirToDelete {
	var dirsToRemove []dirToDelete

	for relPath, dstFile := range destFiles {
		if dstFile.IsDir {
			if _, exists := sourceFiles[relPath]; !exists {
				// Count depth by number of path separators
				depth := strings.Count(relPath, string(filepath.Separator))
				dirsToRemove = append(dirsToRemove, dirToDelete{relPath: relPath, depth: depth})
			}
		}
	}

	// Sort by depth descending (deepest first)
	sort.Slice(dirsToRemove, func(i, j int) bool {
		return dirsToRemove[i].depth > dirsToRemove[j].depth
	})

	return dirsToRemove
}

func (e *Engine) collectErrors(errors chan error) (*[]error, *sync.Mutex, *sync.WaitGroup) {
	allErrors := make([]error, 0)

	var (
		errorsMu sync.Mutex
		errorsWg sync.WaitGroup
	)

	errorsWg.Go(func() {
		for err := range errors {
			errorsMu.Lock()

			allErrors = append(allErrors, err)

			errorsMu.Unlock()
		}
	})

	return &allErrors, &errorsMu, &errorsWg
}

func (e *Engine) compareAndPlanSync(sourceFiles, destFiles map[string]*fileops.FileInfo) error {
	e.initializeComparisonStatus()

	comparedCount := 0
	needSyncCount := 0
	alreadySyncedCount := 0
	filesInBoth := 0
	filesOnlyInSource := 0
	var bytesInBoth int64
	var bytesOnlyInSource int64

	for relPath, srcFile := range sourceFiles {
		// Check for cancellation periodically (every 100 files)
		if comparedCount%100 == 0 {
			select {
			case <-e.cancelChan:
				return ErrAnalysisCancelled
			default:
			}
		}

		if srcFile.IsDir {
			continue // Skip directories
		}

		dstFile := destFiles[relPath]

		// Track comparison counts and bytes
		if dstFile != nil {
			filesInBoth++
			bytesInBoth += srcFile.Size
		} else {
			filesOnlyInSource++
			bytesOnlyInSource += srcFile.Size
		}

		// Determine if file needs sync based on ChangeType
		needsSync := e.determineIfFileNeedsSync(relPath, srcFile, dstFile, comparedCount)

		// Update counters
		if needsSync {
			needSyncCount++
		} else {
			alreadySyncedCount++
		}

		// Prepare log message
		logMsg := e.prepareComparisonLogMessage(needsSync, needSyncCount, alreadySyncedCount, relPath, srcFile, dstFile)

		// Update status
		comparedCount++
		e.updateStatusForFile(relPath, srcFile, needsSync, comparedCount)

		// Log outside the lock
		if logMsg != "" {
			e.logAnalysis(logMsg)
		}

		// Log and notify every 100 files to avoid spam
		if comparedCount%100 == 0 {
			e.logAnalysis(fmt.Sprintf("Compared %d / %d files...", comparedCount, len(sourceFiles)))
			e.notifyStatusUpdate()
		}
	}

	e.logComparisonSummary(sourceFiles, destFiles)

	// Store comparison counts in Status for event emission
	e.Status.mu.Lock()
	e.Status.FilesInBoth = filesInBoth
	e.Status.FilesOnlyInSource = filesOnlyInSource
	e.Status.BytesInBoth = bytesInBoth
	e.Status.BytesOnlyInSource = bytesOnlyInSource
	e.Status.mu.Unlock()

	return nil
}

func (e *Engine) compareFilesByteByByte(relPath string, comparedCount int) bool {
	srcPath := filepath.Join(e.SourcePath, relPath)
	dstPath := filepath.Join(e.DestPath, relPath)

	identical, err := e.FileOps.CompareFilesBytes(srcPath, dstPath)
	if err != nil {
		e.logAnalysis(fmt.Sprintf("  ⚠ Failed to compare bytes for %s: %v", relPath, err))
		return true // Assume needs sync if we can't compare
	}

	needsSync := !identical

	// Log first few comparisons for debugging
	if comparedCount < LogSampleSize {
		if needsSync {
			e.logAnalysis("  → Bytes differ: " + relPath)
		} else {
			e.logAnalysis("  ✓ Bytes match: " + relPath)
		}
	}

	return needsSync
}

// determineIfFileNeedsSync checks if a file needs to be synced based on the ChangeType mode.
// Returns true if the file needs sync, false otherwise.
func (e *Engine) compareFilesWithHash(relPath string, comparedCount int) bool {
	srcPath := filepath.Join(e.SourcePath, relPath)
	dstPath := filepath.Join(e.DestPath, relPath)

	srcHash, err := e.FileOps.ComputeFileHash(srcPath)
	if err != nil {
		e.logAnalysis(fmt.Sprintf("  ⚠ Failed to compute source hash for %s: %v", relPath, err))
		return true // Assume needs sync if we can't compute hash
	}

	dstHash, err := e.FileOps.ComputeFileHash(dstPath)
	if err != nil {
		e.logAnalysis(fmt.Sprintf("  ⚠ Failed to compute dest hash for %s: %v", relPath, err))
		return true // Assume needs sync if we can't compute hash
	}

	// Compare hashes
	needsSync := (srcHash != dstHash)

	// Log first few hash comparisons for debugging
	if comparedCount < LogSampleSize {
		if needsSync {
			e.logAnalysis(fmt.Sprintf("  → Hash mismatch: %s (src=%s... dst=%s...)",
				relPath, srcHash[:8], dstHash[:8]))
		} else {
			e.logAnalysis(fmt.Sprintf("  ✓ Hash match: %s (%s...)", relPath, srcHash[:8]))
		}
	}

	return needsSync
}

func (e *Engine) countAndLogOrphanedItems(sourceFiles, destFiles map[string]*fileops.FileInfo) (int, int) {
	filesToDelete, dirsToDelete, bytesToDelete := countOrphanedItems(sourceFiles, destFiles)

	// Store orphan count and bytes in Status for CompareComplete event and deletion tracking
	e.Status.mu.Lock()
	e.Status.FilesOnlyInDest = filesToDelete
	e.Status.BytesOnlyInDest = bytesToDelete
	e.Status.FilesToDelete = filesToDelete
	e.Status.BytesToDelete = bytesToDelete
	e.Status.FilesDeleted = 0
	e.Status.BytesDeleted = 0
	e.Status.CurrentlyDeleting = nil
	e.Status.DeletionComplete = false
	e.Status.DeletionErrors = 0
	e.Status.mu.Unlock()

	if filesToDelete == 0 && dirsToDelete == 0 {
		return 0, 0
	}

	//nolint:lll // Log message with descriptive text
	e.logAnalysis(fmt.Sprintf("Found %d files and %d directories in destination that don't exist in source", filesToDelete, dirsToDelete))
	e.logOrphanedItemsSample(sourceFiles, destFiles)

	return filesToDelete, dirsToDelete
}

// countOrphanedItemsForPlan counts orphaned items during analysis (for plan display)
// without actually deleting them. Deletion happens during sync phase.
func (e *Engine) countOrphanedItemsForPlan(sourceFiles, destFiles map[string]*fileops.FileInfo) {
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = "planning"
	e.Status.mu.Unlock()

	// Count and log orphaned items (sets status fields for plan display)
	e.countAndLogOrphanedItems(sourceFiles, destFiles)
}

// createProgressCallback creates a progress callback for file copying with throttling
//
//nolint:funlen // Complex progress tracking logic requires multiple state updates
func (e *Engine) createProgressCallback(fileToSync *FileToSync) func(int64, int64, string) {
	var (
		previousBytes  int64
		lastNotifyTime time.Time
		lastSampleTime time.Time // Zero value means first callback will add sample immediately
		sampleBytes    int64
	)

	return func(bytesTransferred, _ int64, _ string) {
		// Calculate delta without lock
		delta := bytesTransferred - previousBytes
		previousBytes = bytesTransferred

		// Use atomic add for the most frequently updated field
		atomic.AddInt64(&e.Status.TransferredBytes, delta)

		// Update per-file progress without full lock
		fileToSync.Transferred = bytesTransferred

		// Transition from "opening" to "copying" on first callback
		if fileToSync.Status == fileStatusOpening {
			e.Status.mu.Lock()
			fileToSync.Status = fileStatusCopying
			e.Status.mu.Unlock()
			e.notifyStatusUpdate()
		}

		// Only acquire lock every 100ms to reduce contention
		now := time.Now()
		throttled := now.Sub(lastNotifyTime) < 100*time.Millisecond //nolint:mnd // Throttle interval

		// Add rate sample every 1 second during transfer
		if now.Sub(lastSampleTime) >= 1*time.Second {
			sampleBytes += delta

			sample := RateSample{
				Timestamp:        now,
				BytesTransferred: sampleBytes,
				ReadTime:         0, // Not available during transfer
				WriteTime:        0, // Not available during transfer
				ActiveWorkers:    int(atomic.LoadInt32(&e.Status.ActiveWorkers)),
			}

			e.Status.mu.Lock()
			e.Status.addRateSample(sample)
			e.Status.mu.Unlock()

			lastSampleTime = now
			sampleBytes = 0 // Reset for next sample
		} else {
			sampleBytes += delta // Accumulate bytes for next sample
		}

		// Verbose instrumentation: log every progress callback
		if e.Verbose {
			var percent float64
			if fileToSync.Size > 0 {
				percent = float64(bytesTransferred) / float64(fileToSync.Size) * 100 //nolint:mnd // Percentage calculation
			}
			throttledStr := "no"
			if throttled {
				throttledStr = "yes"
			}
			e.LogVerbose(fmt.Sprintf("[PROGRESS] CALLBACK: %s transferred=%d/%d (%.1f%%) throttled=%s",
				fileToSync.RelativePath, bytesTransferred, fileToSync.Size, percent, throttledStr))
		}

		if throttled {
			return
		}

		lastNotifyTime = now

		e.Status.mu.Lock()

		// Update current file display (for single file view)
		e.Status.CurrentFile = fileToSync.RelativePath
		e.Status.CurrentFileBytes = bytesTransferred
		e.Status.CurrentFileTotal = fileToSync.Size

		// Calculate transfer speed and ETA based on overall progress
		elapsed := time.Since(e.Status.StartTime).Seconds()
		if elapsed > 0 {
			transferredBytes := atomic.LoadInt64(&e.Status.TransferredBytes)

			e.Status.BytesPerSecond = float64(transferredBytes) / elapsed
			if e.Status.BytesPerSecond > 0 {
				remainingBytes := e.Status.TotalBytes - transferredBytes
				e.Status.EstimatedTimeLeft = time.Duration(float64(remainingBytes)/e.Status.BytesPerSecond) * time.Second
				e.Status.CompletionTime = time.Now().Add(e.Status.EstimatedTimeLeft)
			}
		}

		e.Status.mu.Unlock()
		// No notifyStatusUpdate() call - reduces lock contention
	}
}

func (e *Engine) deleteDirectory(relPath string, deletedCount int) error {
	dstPath := filepath.Join(e.DestPath, relPath)

	// Log first 10 directory deletions for debugging
	if deletedCount < LogSampleLimit {
		e.logAnalysis(fmt.Sprintf("  → Deleting directory: %s (not in source)", relPath))
	}

	err := e.FileOps.RemoveFromDest(dstPath)
	if err != nil {
		// Track error instead of failing
		e.Status.mu.Lock()
		e.Status.Errors = append(e.Status.Errors, FileError{
			FilePath: relPath,
			Error:    fmt.Errorf("failed to delete directory: %w", err),
		})
		errorCount := len(e.Status.Errors)
		e.Status.mu.Unlock()

		e.logAnalysis(fmt.Sprintf("✗ Error deleting directory %s: %v", relPath, err))

		// Check if we've hit the error limit
		if errorCount >= MaxErrorsBeforeAbort {
			return fmt.Errorf("%w (%d)", ErrTooManyErrors, errorCount)
		}

		return ErrDeleteFailed // Signal error but continue
	}

	return nil
}

// deleteFile deletes a single file from destination and tracks progress
func (e *Engine) deleteFile(relPath string, fileSize int64, deletedCount int) error {
	dstPath := filepath.Join(e.DestPath, relPath)

	// Log first 10 deletions for debugging
	if deletedCount < LogSampleLimit {
		e.logAnalysis(fmt.Sprintf("  → Deleting: %s (not in source)", relPath))
	}

	// Track currently deleting file
	e.Status.mu.Lock()
	e.Status.CurrentlyDeleting = append(e.Status.CurrentlyDeleting, relPath)
	e.Status.mu.Unlock()

	err := e.FileOps.RemoveFromDest(dstPath)

	// Remove from currently deleting list
	e.Status.mu.Lock()
	for i, f := range e.Status.CurrentlyDeleting {
		if f == relPath {
			e.Status.CurrentlyDeleting = append(e.Status.CurrentlyDeleting[:i], e.Status.CurrentlyDeleting[i+1:]...)

			break
		}
	}
	e.Status.mu.Unlock()

	if err != nil {
		// Track error instead of failing
		e.Status.mu.Lock()
		e.Status.Errors = append(e.Status.Errors, FileError{
			FilePath: relPath,
			Error:    fmt.Errorf("failed to delete: %w", err),
		})
		e.Status.DeletionErrors++
		errorCount := len(e.Status.Errors)
		e.Status.mu.Unlock()

		e.logAnalysis(fmt.Sprintf("✗ Error deleting %s: %v", relPath, err))

		// Check if we've hit the error limit
		if errorCount >= MaxErrorsBeforeAbort {
			return fmt.Errorf("%w (%d)", ErrTooManyErrors, errorCount)
		}

		return ErrDeleteFailed // Signal error but continue
	}

	// Track successful deletion
	e.Status.mu.Lock()
	e.Status.FilesDeleted++
	e.Status.BytesDeleted += fileSize
	e.Status.mu.Unlock()

	return nil
}

// deleteOrphanedDirectories deletes directories from destination that don't exist in source
//
//nolint:lll // Long function signature with map parameters
func (e *Engine) deleteOrphanedDirectories(sourceFiles, destFiles map[string]*fileops.FileInfo, dirsToDelete int) error {
	if dirsToDelete == 0 {
		return nil
	}

	e.logAnalysis(fmt.Sprintf("Deleting %d orphaned directories...", dirsToDelete))

	dirsToRemove := e.collectDirectoriesToDelete(sourceFiles, destFiles)

	deletedDirCount := 0
	deleteDirErrorCount := 0

	// Delete directories
	for _, dir := range dirsToRemove {
		// Check for cancellation
		select {
		case <-e.cancelChan:
			return ErrAnalysisCancelled
		default:
		}

		err := e.deleteDirectory(dir.relPath, deletedDirCount)
		if err != nil {
			if errors.Is(err, ErrDeleteFailed) {
				deleteDirErrorCount++
			} else {
				return err // Error limit reached
			}
		} else {
			deletedDirCount++
		}
	}

	// Summary of directory deletion
	e.logDirectoryDeletionSummary(deletedDirCount, deleteDirErrorCount)

	return nil
}

func (e *Engine) deleteOrphanedFiles(sourceFiles, destFiles map[string]*fileops.FileInfo, filesToDelete int) error {
	if filesToDelete == 0 {
		// Mark deletion as complete even when there's nothing to delete
		e.Status.mu.Lock()
		e.Status.DeletionComplete = true
		e.Status.mu.Unlock()

		return nil
	}

	deletedCount := 0
	deleteErrorCount := 0
	checkedCount := 0

	// Delete files first (before directories)
	for relPath, dstFile := range destFiles {
		// Check for cancellation periodically (every 100 files)
		if checkedCount%100 == 0 {
			select {
			case <-e.cancelChan:
				return ErrAnalysisCancelled
			default:
			}
		}

		if dstFile.IsDir {
			continue
		}

		checkedCount++
		e.updateDeletionStatus(checkedCount, relPath)

		var err error

		deletedCount, deleteErrorCount, err = e.processOrphanedFile(relPath, sourceFiles, dstFile.Size, deletedCount, deleteErrorCount)
		if err != nil {
			return err
		}

		// Notify every 100 files
		if checkedCount%100 == 0 {
			e.notifyStatusUpdate()
		}
	}

	// Mark file deletion as complete
	e.Status.mu.Lock()
	e.Status.DeletionComplete = true
	e.Status.mu.Unlock()

	// Summary of file deletion phase
	e.logFileDeletionSummary(deletedCount, deleteErrorCount)

	return nil
}

// deleteOrphanedItems deletes files and directories from destination that don't exist in source

// Count and log orphaned items

// Delete files first (before directories)

// Delete directories (in reverse depth order, deepest first)

// performDeletionsDuringSync deletes orphaned files/directories during sync phase
// Uses file maps stored during analysis phase.
func (e *Engine) performDeletionsDuringSync() error {
	sourceFiles := e.analysisSourceFiles
	destFiles := e.analysisDestFiles

	// If no file maps available (shouldn't happen), skip deletion
	if sourceFiles == nil || destFiles == nil {
		return nil
	}

	// Get file count from status (set during analysis)
	e.Status.mu.RLock()
	filesToDelete := e.Status.FilesToDelete
	e.Status.mu.RUnlock()

	// Count directories to delete
	dirsToDelete := 0
	for relPath, dstFile := range destFiles {
		if dstFile.IsDir {
			if _, exists := sourceFiles[relPath]; !exists {
				dirsToDelete++
			}
		}
	}

	// Skip if nothing to delete
	if filesToDelete == 0 && dirsToDelete == 0 {
		e.Status.mu.Lock()
		e.Status.DeletionComplete = true
		e.Status.mu.Unlock()

		return nil
	}

	e.logToFile(fmt.Sprintf("Starting deletion phase: %d files, %d directories", filesToDelete, dirsToDelete))

	// Delete files first (before directories)
	err := e.deleteOrphanedFiles(sourceFiles, destFiles, filesToDelete)
	if err != nil {
		return err
	}

	// Delete directories (in reverse depth order, deepest first)
	err = e.deleteOrphanedDirectories(sourceFiles, destFiles, dirsToDelete)
	if err != nil {
		return err
	}

	return nil
}

func (e *Engine) determineIfFileNeedsSync(relPath string, srcFile, dstFile *fileops.FileInfo, comparedCount int) bool {
	switch e.ChangeType {
	case config.Content:
		// For Content mode, use full comparison (size + modtime)
		return fileops.FilesNeedSync(srcFile, dstFile)
	case config.MonotonicCount, config.FluctuatingCount:
		// For count-based modes, only check if file exists (path comparison)
		return dstFile == nil
	case config.DeviousContent:
		// For devious-content mode, always compare hashes
		if dstFile == nil {
			return true
		}

		return e.compareFilesWithHash(relPath, comparedCount)
	case config.Paranoid:
		// For paranoid mode, perform byte-by-byte comparison
		if dstFile == nil {
			return true
		}

		return e.compareFilesByteByByte(relPath, comparedCount)
	}

	return false
}

// distributeJobs sends all files to the job queue with cancellation support
func (e *Engine) distributeJobs(jobs chan *FileToSync) {
	go func() {
		for _, fileToSync := range e.Status.FilesToSync {
			select {
			case <-e.cancelChan:
				close(jobs)
				return
			case jobs <- fileToSync:
			}
		}

		close(jobs)
	}()
}

func (e *Engine) enqueueFilesForSync(jobs chan *FileToSync) {
	go func() {
		for _, fileToSync := range e.Status.FilesToSync {
			select {
			case <-e.cancelChan:
				close(jobs)
				return
			case jobs <- fileToSync:
			}
		}

		close(jobs)
	}()
}

func (e *Engine) finalizeAnalysis() {
	e.Status.mu.Lock()
	e.Status.TotalFiles = len(e.Status.FilesToSync)
	e.Status.AnalysisPhase = phaseComplete
	e.Status.mu.Unlock()

	e.logAnalysis("Analysis complete!")
	e.notifyStatusUpdate()
}

func (e *Engine) finalizeSyncPhase() {
	e.Status.mu.Lock()
	e.Status.EndTime = time.Now()
	processedFiles := e.Status.ProcessedFiles
	totalFiles := e.Status.TotalFiles
	e.Status.mu.Unlock()

	e.logToFile(fmt.Sprintf("Sync phase complete: %d / %d files copied", processedFiles, totalFiles))

	e.Status.mu.Lock()
	e.Status.FinalizationPhase = phaseComplete
	e.Status.mu.Unlock()
	e.notifyStatusUpdate()
}

func (e *Engine) handleCopyError(fileToSync *FileToSync, copyErr error) error {
	// Check if this was a cancellation vs an actual error
	if errors.Is(copyErr, fileops.ErrCopyCancelled) {
		fileToSync.Status = "cancelled"
		e.Status.CancelledFiles++
		e.Status.CancelledCopies = append(e.Status.CancelledCopies, fileToSync.RelativePath)
	} else {
		fileToSync.Status = "error"
		fileToSync.Error = copyErr
		e.Status.FailedFiles++
		e.Status.Errors = append(e.Status.Errors, FileError{
			FilePath: fileToSync.RelativePath,
			Error:    copyErr,
		})
	}

	return fmt.Errorf("failed to copy %s: %w", fileToSync.RelativePath, copyErr)
}

func (e *Engine) handleCopyResult(fileToSync *FileToSync, stats *fileops.CopyStats, copyErr error) error {
	e.Status.mu.Lock()

	// Track read/write times for bottleneck detection
	e.updateBottleneckDetection(stats)

	// Add rolling window sample for metrics calculation
	if stats != nil {
		sample := RateSample{
			Timestamp:        time.Now(),
			BytesTransferred: stats.BytesCopied,
			ReadTime:         stats.ReadTime,
			WriteTime:        stats.WriteTime,
			ActiveWorkers:    int(atomic.LoadInt32(&e.Status.ActiveWorkers)),
		}
		e.Status.addRateSample(sample)
	}

	// Remove from currently copying files
	e.removeFromCurrentFiles(fileToSync.RelativePath)

	if copyErr != nil {
		err := e.handleCopyError(fileToSync, copyErr)
		e.Status.mu.Unlock()

		return err
	}

	e.handleCopySuccess(fileToSync)

	e.Status.mu.Unlock()
	e.notifyStatusUpdate()

	return nil
}

func (e *Engine) handleCopySuccess(fileToSync *FileToSync) {
	fileToSync.Status = fileStatusComplete
	e.Status.ProcessedFiles++

	// Add to recently completed (keep last 10)
	e.Status.RecentlyCompleted = append(e.Status.RecentlyCompleted, fileToSync.RelativePath)
	if len(e.Status.RecentlyCompleted) > RecentlyCompletedLimit {
		e.Status.RecentlyCompleted = e.Status.RecentlyCompleted[len(e.Status.RecentlyCompleted)-RecentlyCompletedLimit:]
	}

	// Log first 10 completed files
	if e.Status.ProcessedFiles <= RecentlyCompletedLimit {
		e.logToFile(fmt.Sprintf("  ✓ Copied: %s (%s)", fileToSync.RelativePath, formatters.FormatBytes(fileToSync.Size)))
	}
}

// compareAndPlanSync compares source and destination files to determine which need sync
func (e *Engine) initializeComparisonStatus() {
	e.Status.mu.Lock()
	e.Status.FilesToSync = make([]*FileToSync, 0)
	e.Status.TotalBytes = 0
	e.Status.TotalFilesInSource = 0
	e.Status.TotalBytesInSource = 0
	e.Status.AlreadySyncedFiles = 0
	e.Status.AlreadySyncedBytes = 0
	e.Status.mu.Unlock()
}

// logAnalysis adds a message to the analysis log
func (e *Engine) logAnalysis(message string) {
	e.Status.mu.Lock()
	// Keep only the last 20 log entries
	maxLogEntries := 20

	e.Status.AnalysisLog = append(e.Status.AnalysisLog, message)
	if len(e.Status.AnalysisLog) > maxLogEntries {
		e.Status.AnalysisLog = e.Status.AnalysisLog[len(e.Status.AnalysisLog)-maxLogEntries:]
	}

	e.Status.mu.Unlock()

	e.notifyStatusUpdate()

	// Also write to log file if enabled
	e.logToFile(message)
}

func (e *Engine) logComparisonSummary(sourceFiles, destFiles map[string]*fileops.FileInfo) {
	e.Status.mu.Lock()
	totalFiles := len(e.Status.FilesToSync)
	totalBytes := e.Status.TotalBytes
	alreadySynced := e.Status.AlreadySyncedFiles
	e.Status.mu.Unlock()

	e.logAnalysis(fmt.Sprintf("Found %d files to sync (%s total)", totalFiles, formatters.FormatBytes(totalBytes)))

	if alreadySynced > 0 {
		e.logAnalysis(fmt.Sprintf("%d files already up-to-date", alreadySynced))
	}

	// Log diagnostic info about the comparison
	e.logAnalysis(fmt.Sprintf("Comparison summary: %d source files, %d dest files, %d need sync, %d already synced",
		len(sourceFiles), len(destFiles), totalFiles, alreadySynced))
}

func (e *Engine) logDirectoryDeletionSummary(deletedCount, errorCount int) {
	if deletedCount == 0 && errorCount == 0 {
		return
	}

	switch {
	case errorCount == 0:
		e.logAnalysis(fmt.Sprintf("✓ Deleted %d directories from destination", deletedCount))
	case deletedCount == 0:
		e.logAnalysis(fmt.Sprintf("✗ Failed to delete all %d directories (see errors below)", errorCount))
	default:
		e.logAnalysis(fmt.Sprintf("Deleted %d directories, failed to delete %d directories (see errors below)",
			deletedCount, errorCount))
	}
}

func (e *Engine) logFileDeletionSummary(deletedCount, deleteErrorCount int) {
	if deletedCount == 0 && deleteErrorCount == 0 {
		return
	}

	switch {
	case deleteErrorCount == 0:
		e.logAnalysis(fmt.Sprintf("✓ Deleted %d files from destination", deletedCount))
	case deletedCount == 0:
		e.logAnalysis(fmt.Sprintf("✗ Failed to delete all %d files (see errors below)", deleteErrorCount))
	default:
		e.logAnalysis(fmt.Sprintf("Deleted %d files, failed to delete %d files (see errors below)",
			deletedCount, deleteErrorCount))
	}
}

// logOrphanedItemsSample logs a sample of orphaned items for debugging.
func (e *Engine) logOrphanedItemsSample(sourceFiles, destFiles map[string]*fileops.FileInfo) {
	loggedDeletes := 0

	e.logAnalysis("Sample destination items not in source:")

	for relPath, dstFile := range destFiles {
		if _, exists := sourceFiles[relPath]; !exists {
			if loggedDeletes >= LogSampleSize {
				break
			}

			itemType := "file"
			if dstFile.IsDir {
				itemType = "dir"
			}

			e.logAnalysis(fmt.Sprintf("  Dest only (%s): %s", itemType, relPath))

			loggedDeletes++
		}
	}
}

// logSamplePaths logs sample paths from source and destination for debugging
func (e *Engine) logSamplePaths(sourceFiles, destFiles map[string]*fileops.FileInfo) {
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = "comparing"
	e.Status.ScannedFiles = 0
	e.Status.TotalFilesToScan = len(sourceFiles)
	e.Status.mu.Unlock()

	e.logAnalysis(fmt.Sprintf("Comparing files to determine sync plan (%d files to compare)...", len(sourceFiles)))

	// Log sample of source paths for debugging
	sourceCount := 0

	e.logAnalysis("Sample source paths:")

	for relPath := range sourceFiles {
		if sourceCount < LogSampleSize {
			e.logAnalysis("  Source: " + relPath)

			sourceCount++
		} else {
			break
		}
	}

	// Log sample of destination paths for debugging
	destCount := 0

	e.logAnalysis("Sample destination paths:")

	for relPath := range destFiles {
		if destCount < LogSampleSize {
			e.logAnalysis("  Dest: " + relPath)

			destCount++
		} else {
			break
		}
	}
}

// logToFile writes a message to the log file (if enabled)
func (e *Engine) logToFile(message string) {
	if e.logFile != nil {
		e.logMu.Lock()
		defer e.logMu.Unlock()

		timestamp := time.Now().Format("15:04:05.000")
		_, _ = fmt.Fprintf(e.logFile, "[%s] %s\n", timestamp, message)
	}
}

// markFileCompleteWithoutCopy marks a file as complete without actually copying it
func (e *Engine) markFileCompleteWithoutCopy(fileToSync *FileToSync) {
	e.Status.mu.Lock()
	fileToSync.Status = fileStatusComplete
	fileToSync.Transferred = fileToSync.Size
	e.Status.ProcessedFiles++
	atomic.AddInt64(&e.Status.TransferredBytes, fileToSync.Size)

	// Remove from currently copying files
	for i, f := range e.Status.CurrentFiles {
		if f == fileToSync.RelativePath {
			e.Status.CurrentFiles = append(e.Status.CurrentFiles[:i], e.Status.CurrentFiles[i+1:]...)
			break
		}
	}

	e.Status.mu.Unlock()
	e.notifyStatusUpdate()
}

// notifyStatusUpdate notifies all registered callbacks
func (e *Engine) notifyStatusUpdate() {
	e.mu.RLock()
	callbacks := make([]func(*Status), len(e.statusCallbacks))
	copy(callbacks, e.statusCallbacks)
	e.mu.RUnlock()

	for _, callback := range callbacks {
		callback(e.Status)
	}
}

// prepareAlreadySyncedLogMessage prepares a log message for files already synced.
//
//nolint:lll // Long function signature with multiple parameters
func (e *Engine) prepareAlreadySyncedLogMessage(alreadySyncedCount int, relPath string, srcFile *fileops.FileInfo) string {
	if alreadySyncedCount > LogSampleLimit {
		return ""
	}

	return fmt.Sprintf("  ✓ Already synced: %s (size=%d, modtime=%s)",
		relPath, srcFile.Size, srcFile.ModTime.Format(time.RFC3339Nano))
}

// prepareComparisonLogMessage prepares a log message for file comparison results
//
//nolint:lll // Long function signature with multiple parameters
func (e *Engine) prepareComparisonLogMessage(needsSync bool, needSyncCount, alreadySyncedCount int, relPath string, srcFile, dstFile *fileops.FileInfo) string {
	if needsSync {
		return e.prepareNeedSyncLogMessage(needSyncCount, relPath, srcFile, dstFile)
	}

	return e.prepareAlreadySyncedLogMessage(alreadySyncedCount, relPath, srcFile)
}

// prepareNeedSyncLogMessage prepares a log message for files that need sync.
//
//nolint:lll // Long function signature with multiple parameters
func (e *Engine) prepareNeedSyncLogMessage(needSyncCount int, relPath string, srcFile, dstFile *fileops.FileInfo) string {
	if needSyncCount > LogSampleLimit {
		return ""
	}

	if dstFile == nil {
		return fmt.Sprintf("  → Need sync: %s (destination missing)", relPath)
	}

	reason := formatSyncReason(srcFile, dstFile)

	return fmt.Sprintf("  → Need sync: %s (%s)", relPath, reason)
}

// processFileDeletion handles the deletion of a single orphaned file.
func (e *Engine) processFileDeletion(relPath string, fileSize int64, deletedCount int) (deleted bool, err error) {
	err = e.deleteFile(relPath, fileSize, deletedCount)
	if err != nil {
		if errors.Is(err, ErrDeleteFailed) {
			return false, nil // Count as error but continue
		}

		return false, err // Error limit reached
	}

	return true, nil
}

// processOrphanedFile processes a single file for deletion if it's orphaned.
//
//nolint:lll // Long function signature with map parameter and multiple return values
func (e *Engine) processOrphanedFile(relPath string, sourceFiles map[string]*fileops.FileInfo, fileSize int64, deletedCount, deleteErrorCount int) (newDeletedCount, newDeleteErrorCount int, err error) {
	if _, exists := sourceFiles[relPath]; !exists {
		// Delete this file from destination
		deleted, err := e.processFileDeletion(relPath, fileSize, deletedCount)
		if err != nil {
			return deletedCount, deleteErrorCount, err
		}

		if deleted {
			return deletedCount + 1, deleteErrorCount, nil
		}

		return deletedCount, deleteErrorCount + 1, nil
	}

	return deletedCount, deleteErrorCount, nil
}

func (e *Engine) removeFromCurrentFiles(relativePath string) {
	for i, f := range e.Status.CurrentFiles {
		if f == relativePath {
			e.Status.CurrentFiles = append(e.Status.CurrentFiles[:i], e.Status.CurrentFiles[i+1:]...)

			// Verbose instrumentation: log when file is removed from CurrentFiles
			e.LogVerbose("[PROGRESS] FILE_END: " + relativePath)

			break
		}
	}
}

// resizePools calls ResizePool on source and dest if they implement ResizablePool
func (e *Engine) resizePools(targetSize int) {
	if e.sourceResizable != nil {
		e.sourceResizable.ResizePool(targetSize)
	}
	if e.destResizable != nil {
		e.destResizable.ResizePool(targetSize)
	}
}

// scanDestinationDirectory scans the destination directory and returns file information.
func (e *Engine) scanDestinationDirectory() (map[string]*fileops.FileInfo, error) {
	e.logAnalysis("Scanning destination: " + e.DestPath)

	// Update analysis phase
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = phaseCountingDest
	e.Status.DestScannedFiles = 0
	e.Status.DestTotalFiles = 0
	e.Status.mu.Unlock()

	// Notify before accessing (may block on slow/remote filesystems)
	e.logAnalysis("Accessing destination...")
	e.notifyStatusUpdate()

	// Scan destination directory with progress
	//nolint:dupl,lll // Duplicate scanning callbacks, long function signature
	destFiles, err := e.FileOps.ScanDestDirectoryWithProgress(e.DestPath, func(path string, scannedCount int, totalCount int, fileSize int64) {
		e.Status.mu.Lock()
		e.Status.DestScannedFiles = scannedCount
		e.Status.DestTotalFiles = totalCount
		// Accumulate scanned bytes
		e.Status.ScannedBytes += fileSize

		// Update phase when we transition from counting to scanning
		if totalCount > 0 && e.Status.AnalysisPhase == phaseCountingDest {
			e.Status.AnalysisPhase = "scanning_dest"
		}

		e.Status.mu.Unlock()

		e.notifyStatusUpdate()
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to scan destination: %w", err)
	}

	if destFiles == nil {
		destFiles = make(map[string]*fileops.FileInfo)

		e.logAnalysis("Destination directory does not exist (will be created)")
	} else {
		// Update TotalFilesInDest so TUI can use it as fallback if polling missed final count
		e.Status.mu.Lock()
		e.Status.TotalFilesInDest = len(destFiles)
		e.Status.ScannedFiles = len(destFiles) // Ensure final count is visible before phase change
		e.Status.mu.Unlock()
		e.notifyStatusUpdate()

		e.logAnalysis(fmt.Sprintf("Destination scan complete: %d items found", len(destFiles)))
	}

	return destFiles, nil
}

// scanSourceDirectory scans the source directory and returns file information.
//
//nolint:cyclop,dupl,funlen // Directory scanning logic
func (e *Engine) scanSourceDirectory() (map[string]*fileops.FileInfo, error) {
	e.logAnalysis("Scanning source: " + e.SourcePath)

	// Update analysis phase and start time
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = phaseCountingSource
	e.Status.SourceScannedFiles = 0
	e.Status.SourceTotalFiles = 0
	if e.Status.AnalysisStartTime.IsZero() {
		e.Status.AnalysisStartTime = e.TimeProvider.Now()
	}
	e.Status.mu.Unlock()

	// Notify before accessing (may block on slow/remote filesystems)
	e.logAnalysis("Accessing source...")
	e.notifyStatusUpdate()

	// Scan source directory with progress
	//nolint:lll // Anonymous function with parameters as part of method call
	sourceFiles, err := e.FileOps.ScanDirectoryWithProgress(e.SourcePath, func(path string, scannedCount int, totalCount int, fileSize int64) {
		e.Status.mu.Lock()
		e.Status.SourceScannedFiles = scannedCount
		e.Status.SourceTotalFiles = totalCount
		// Accumulate scanned bytes
		e.Status.ScannedBytes += fileSize

		// Calculate analysis rate if we have elapsed time
		if !e.Status.AnalysisStartTime.IsZero() {
			elapsed := e.TimeProvider.Now().Sub(e.Status.AnalysisStartTime).Seconds()
			if elapsed > 0 {
				e.Status.AnalysisRate = float64(scannedCount) / elapsed
			}
		}

		// Update phase when we transition from counting to scanning
		if totalCount > 0 && e.Status.AnalysisPhase == phaseCountingSource {
			e.Status.AnalysisPhase = "scanning_source"
		}

		e.Status.mu.Unlock()

		e.notifyStatusUpdate()
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan source: %w", err)
	}

	// Apply file pattern filter if specified
	if e.FilePattern != "" {
		sourceFiles = e.applyFileFilter(sourceFiles)
		e.logAnalysis(fmt.Sprintf("After filtering by pattern '%s': %d items remain", e.FilePattern, len(sourceFiles)))
	}

	// Calculate total bytes to scan
	var totalBytes int64
	for _, fileInfo := range sourceFiles {
		if !fileInfo.IsDir {
			totalBytes += fileInfo.Size
		}
	}

	e.Status.mu.Lock()
	e.Status.TotalBytesToScan = totalBytes
	e.Status.mu.Unlock()

	// Update TotalFilesInSource so TUI can use it as fallback if polling missed final count
	e.Status.mu.Lock()
	e.Status.TotalFilesInSource = len(sourceFiles)
	e.Status.ScannedFiles = len(sourceFiles) // Ensure final count is visible before phase change
	e.Status.mu.Unlock()
	e.notifyStatusUpdate()

	e.logAnalysis(fmt.Sprintf("Source scan complete: %d items found, %s total",
		len(sourceFiles), formatters.FormatBytes(totalBytes)))

	return sourceFiles, nil
}

// startAdaptiveScaling starts a goroutine that monitors performance and adjusts worker count
func (e *Engine) startAdaptiveScaling(done chan struct{}, jobs chan *FileToSync, workerControl chan bool) {
	// Use different algorithms for adaptive vs fixed mode
	if !e.AdaptiveMode {
		// Fixed mode - no scaling
		return
	}

	go func() {
		ticker := e.TimeProvider.NewTicker(1 * time.Second)
		defer ticker.Stop()

		// Time-based scaling algorithm - continuously dynamic
		state := &AdaptiveScalingState{}
		maxWorkers := len(e.Status.FilesToSync) // Cap at total files

		e.logToFile("HillClimbing: Starting with 1 worker, will adjust based on total system throughput")

		for {
			select {
			case <-done:
				return
			case <-ticker.C():
				e.Status.mu.RLock()
				currentProcessedFiles := e.Status.ProcessedFiles
				currentWorkers := int(atomic.LoadInt32(&e.Status.ActiveWorkers))
				e.Status.mu.RUnlock()
				currentBytes := atomic.LoadInt64(&e.Status.TransferredBytes)

				// Only scale if we have pending work
				pendingWork := len(jobs)
				if pendingWork == 0 {
					continue
				}

				const evaluationInterval = 10 * time.Second

				// Check if enough time has elapsed since last evaluation
				if time.Since(state.LastCheckTime) >= evaluationInterval {
					e.EvaluateAndScale(state, currentProcessedFiles, currentWorkers, currentBytes, maxWorkers, workerControl)
				}
			}
		}
	}()
}

func (e *Engine) startFixedWorkers(numWorkers int, jobs chan *FileToSync, errors chan error) *sync.WaitGroup {
	var wg sync.WaitGroup //nolint:varnamelen // wg is idiomatic for WaitGroup
	for range numWorkers {
		wg.Go(func() {
			for fileToSync := range jobs {
				// Check for cancellation
				select {
				case <-e.cancelChan:
					return
				default:
				}

				err := e.syncFile(fileToSync)
				if err != nil {
					// syncFile already updated status and error tracking
					// Just send error to channel for counting
					e.notifyStatusUpdate()

					errors <- err
				}
			}
		})
	}

	return &wg
}

// startWorkerControl starts a goroutine that manages adding workers dynamically
//
//nolint:lll,varnamelen // Long function signature with channel parameters; wg is idiomatic for WaitGroup
func (e *Engine) startWorkerControl(wg *sync.WaitGroup, jobs <-chan *FileToSync, errors chan<- error, workerControl chan bool) {
	go func() {
		for add := range workerControl {
			if add {
				wg.Add(1)

				go e.worker(wg, jobs, errors)

				e.Status.mu.Lock()

				newActive := atomic.AddInt32(&e.Status.ActiveWorkers, 1)
				if int(newActive) > e.Status.MaxWorkers {
					e.Status.MaxWorkers = int(newActive)
				}

				e.Status.mu.Unlock()
				e.notifyStatusUpdate()
			}
		}
	}()
}

// syncAdaptive uses adaptive concurrency that scales based on throughput
//
//nolint:funlen // Complex adaptive scaling logic requires sequential steps
func (e *Engine) syncAdaptive() error {
	e.logToFile("Starting sync phase (adaptive mode)...")

	// Perform deletions first (before copying)
	if err := e.performDeletionsDuringSync(); err != nil {
		return err
	}

	e.logToFile(fmt.Sprintf("Files to sync: %d", len(e.Status.FilesToSync)))

	e.Status.mu.Lock()
	e.Status.StartTime = time.Now()
	e.Status.AdaptiveMode = true
	e.Status.mu.Unlock()

	// Create channels for work distribution
	jobs := make(chan *FileToSync, WorkerChannelBufferSize) // Buffered channel for pending work
	errors := make(chan error, len(e.Status.FilesToSync))
	done := make(chan struct{})

	// Start with initial workers
	initialWorkers := 4
	if e.Workers > 0 {
		initialWorkers = e.Workers
	}

	if initialWorkers > len(e.Status.FilesToSync) {
		initialWorkers = len(e.Status.FilesToSync)
	}

	// Worker management
	var wg sync.WaitGroup //nolint:varnamelen // wg is idiomatic for WaitGroup

	workerControl := make(chan bool, WorkerChannelBufferSize) // true = add worker, false = remove worker
	activeWorkers := 0

	// Start with 1 worker for adaptive mode, or all workers for fixed mode
	startWorkers := initialWorkers
	if e.AdaptiveMode {
		startWorkers = 1
	}

	for range startWorkers {
		wg.Add(1)

		activeWorkers++

		go e.worker(&wg, jobs, errors)
	}

	e.Status.mu.Lock()
	atomic.StoreInt32(&e.Status.ActiveWorkers, int32(activeWorkers)) //nolint:gosec // Small value, no overflow risk
	e.Status.MaxWorkers = activeWorkers
	e.Status.mu.Unlock()
	e.notifyStatusUpdate()

	// Initialize desired worker count for adaptive scaling
	atomic.StoreInt32(&e.desiredWorkers, int32(activeWorkers)) //nolint:gosec // Bounded, no overflow
	e.resizePools(activeWorkers)

	// Start background goroutines for adaptive scaling, worker control, and job distribution
	e.startAdaptiveScaling(done, jobs, workerControl)
	e.startWorkerControl(&wg, jobs, errors, workerControl)
	e.distributeJobs(jobs)

	// Collect errors concurrently to avoid blocking workers
	allErrors, errorsMu, errorsWg := e.collectErrors(errors)

	// Wait for all workers to complete
	wg.Wait()
	close(done)
	close(workerControl)
	close(errors)

	// Wait for error collector to finish
	errorsWg.Wait()

	// Record completion time
	e.Status.mu.Lock()
	e.Status.EndTime = time.Now()
	e.Status.mu.Unlock()

	// Check if we hit the error limit
	e.Status.mu.RLock()
	errorCount := len(e.Status.Errors)
	e.Status.mu.RUnlock()

	if errorCount >= MaxErrorsBeforeAbort {
		return fmt.Errorf("%w: too many errors (%d errors, limit is %d)", ErrSyncAborted, errorCount, MaxErrorsBeforeAbort)
	}

	// Mark finalization as complete
	e.Status.mu.Lock()
	e.Status.FinalizationPhase = phaseComplete
	e.Status.mu.Unlock()
	e.notifyStatusUpdate()

	// Return combined error if any failures occurred (but didn't hit limit)
	errorsMu.Lock()

	errorCountFromChannel := len(*allErrors)

	errorsMu.Unlock()

	if errorCountFromChannel > 0 {
		return fmt.Errorf("%w: %d (see error details in completion screen)", ErrFilesFailed, errorCountFromChannel)
	}

	return nil
}

func (e *Engine) syncFile(fileToSync *FileToSync) error {
	srcPath := filepath.Join(e.SourcePath, fileToSync.RelativePath)
	dstPath := filepath.Join(e.DestPath, fileToSync.RelativePath)

	e.Status.mu.Lock()
	e.Status.CurrentFile = fileToSync.RelativePath
	e.Status.CurrentFiles = append(e.Status.CurrentFiles, fileToSync.RelativePath)
	fileToSync.Status = fileStatusOpening
	e.Status.mu.Unlock()
	e.notifyStatusUpdate()

	// Verbose instrumentation: log when file enters opening state
	e.LogVerbose(fmt.Sprintf("[PROGRESS] FILE_START: %s (size=%d)", fileToSync.RelativePath, fileToSync.Size))

	// Try hash optimization for Content mode
	optimized, err := e.tryHashOptimization(fileToSync, srcPath, dstPath)
	if err != nil {
		return err
	}

	if optimized {
		return nil
	}

	// Copy the file with timing stats (pass cancel channel for mid-copy cancellation)
	progressCallback := e.createProgressCallback(fileToSync)

	// Create callback to mark file as finalizing when data transfer completes
	onDataComplete := func() {
		e.Status.mu.Lock()
		fileToSync.Status = fileStatusFinalizing
		e.Status.mu.Unlock()
		e.notifyStatusUpdate()
	}

	stats, err := e.FileOps.CopyFileWithStats(srcPath, dstPath, progressCallback, e.cancelChan, onDataComplete)

	// Update bottleneck detection and handle copy result
	return e.handleCopyResult(fileToSync, stats, err)
}

// syncFixed uses a fixed number of workers
func (e *Engine) syncFixed() error {
	e.logToFile("Starting sync phase...")

	// Perform deletions first (before copying)
	if err := e.performDeletionsDuringSync(); err != nil {
		return err
	}

	e.logToFile(fmt.Sprintf("Files to sync: %d", len(e.Status.FilesToSync)))

	e.Status.mu.Lock()
	e.Status.StartTime = time.Now()
	e.Status.AdaptiveMode = false
	e.Status.mu.Unlock()

	// Create channels for work distribution
	jobs := make(chan *FileToSync, len(e.Status.FilesToSync))
	errors := make(chan error, len(e.Status.FilesToSync))

	// Determine number of workers (don't exceed number of files)
	numWorkers := e.Workers
	numWorkers = min(numWorkers, len(e.Status.FilesToSync))
	numWorkers = max(numWorkers, 1)

	e.Status.mu.Lock()
	atomic.StoreInt32(&e.Status.ActiveWorkers, int32(numWorkers)) //nolint:gosec // Bounded, no overflow
	e.Status.MaxWorkers = numWorkers
	e.Status.mu.Unlock()

	// Start worker pool
	wg := e.startFixedWorkers(numWorkers, jobs, errors) //nolint:varnamelen // wg is idiomatic for WaitGroup

	// Send all files to the job queue
	e.enqueueFilesForSync(jobs)

	// Collect errors concurrently
	allErrors, errorsMu, errorsWg := e.collectErrors(errors)

	// Wait for all workers to complete
	wg.Wait()
	close(errors)

	// Wait for error collector to finish
	errorsWg.Wait()

	// Finalize sync phase
	e.finalizeSyncPhase()

	// Return combined error if any failures occurred
	errorsMu.Lock()

	errorCount := len(*allErrors)

	var firstError error
	if errorCount > 0 {
		firstError = (*allErrors)[0]
	}

	errorsMu.Unlock()

	if errorCount > 0 {
		return fmt.Errorf("%w: %d (first error: %w)", ErrFilesFailed, errorCount, firstError)
	}

	return nil
}

// syncFile synchronizes a single file
// tryHashOptimization checks if hashes match in Content mode and just updates modtime if so.
// Returns true if optimization was applied (no copy needed), false if copy is needed.
func (e *Engine) tryHashOptimization(fileToSync *FileToSync, srcPath, dstPath string) (bool, error) {
	// Only applicable in Content mode
	if e.ChangeType != config.Content {
		return false, nil
	}

	// Check if destination file exists
	_, err := e.FileOps.Stat(dstPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // Destination doesn't exist, need to copy
		}

		return false, fmt.Errorf("failed to stat destination file: %w", err)
	}

	// Both files exist, compute hashes
	srcHash, err := e.FileOps.ComputeFileHash(srcPath)
	if err != nil {
		return false, fmt.Errorf("failed to compute source hash: %w", err)
	}

	dstHash, err := e.FileOps.ComputeFileHash(dstPath)
	if err != nil {
		return false, fmt.Errorf("failed to compute destination hash: %w", err)
	}

	// If hashes differ, need to copy
	if srcHash != dstHash {
		e.logAnalysis(fmt.Sprintf("  → Hashes differ for %s - copying file", fileToSync.RelativePath))
		return false, nil
	}

	// Hashes match - just update modtime
	e.logAnalysis(fmt.Sprintf("  ✓ Hashes match for %s - updating modtime only", fileToSync.RelativePath))

	// Get source modtime
	srcInfo, err := e.FileOps.Stat(srcPath)
	if err != nil {
		return false, fmt.Errorf("failed to stat source file: %w", err)
	}

	// Update destination modtime
	err = e.FileOps.Chtimes(dstPath, srcInfo.ModTime(), srcInfo.ModTime())
	if err != nil {
		return false, fmt.Errorf("failed to update modtime: %w", err)
	}

	// Mark file as complete without copying
	e.markFileCompleteWithoutCopy(fileToSync)

	return true, nil
}

// tryMonotonicCountOptimization checks if file counts match in monotonic-count mode.
// Returns true if optimization succeeded (counts match), false if full scan is needed.
//
//nolint:funlen // Optimization logic includes multiple validation and counting steps
func (e *Engine) tryMonotonicCountOptimization() (bool, error) {
	if e.ChangeType != config.MonotonicCount {
		return false, nil
	}

	e.logAnalysis("Monotonic-count mode: checking file counts...")

	// Count source files
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = phaseCountingSource
	e.Status.SourceScannedFiles = 0
	e.Status.SourceTotalFiles = 0
	e.Status.mu.Unlock()

	e.emit(ScanStarted{Target: "source"})
	e.logAnalysis("Accessing source...")
	e.notifyStatusUpdate()

	sourceCount, err := e.FileOps.CountFilesWithProgress(e.SourcePath, func(path string, count int) {
		e.Status.mu.Lock()
		e.Status.ScannedFiles = count
		e.Status.SourceScannedFiles = count // Update source-specific counter for TUI
		e.Status.CurrentPath = path
		e.Status.mu.Unlock()

		if count%10 == 0 {
			e.logAnalysis(fmt.Sprintf("Counting source files: %d so far...", count))
		}

		e.notifyStatusUpdate()
	})
	if err != nil {
		return false, fmt.Errorf("failed to count source files: %w", err)
	}

	e.logAnalysis(fmt.Sprintf("Source file count: %d", sourceCount))
	e.emit(ScanComplete{Target: "source", Count: sourceCount})

	// Store source count immediately so TUI can display it
	e.Status.mu.Lock()
	e.Status.TotalFilesInSource = sourceCount
	e.Status.mu.Unlock()

	// Count destination files
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = phaseCountingDest
	e.Status.ScannedFiles = 0
	e.Status.DestScannedFiles = 0
	e.Status.DestTotalFiles = 0
	e.Status.mu.Unlock()

	e.emit(ScanStarted{Target: "dest"})
	e.logAnalysis("Accessing destination...")
	e.notifyStatusUpdate()

	destCount, err := e.FileOps.CountDestFilesWithProgress(e.DestPath, func(path string, count int) {
		e.Status.mu.Lock()
		e.Status.ScannedFiles = count
		e.Status.DestScannedFiles = count // Update dest-specific counter for TUI
		e.Status.CurrentPath = path
		e.Status.mu.Unlock()

		if count%10 == 0 {
			e.logAnalysis(fmt.Sprintf("Counting dest files: %d so far...", count))
		}

		e.notifyStatusUpdate()
	})
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to count destination files: %w", err)
	}

	if os.IsNotExist(err) {
		destCount = 0
	}

	e.logAnalysis(fmt.Sprintf("Destination file count: %d", destCount))
	e.emit(ScanComplete{Target: "dest", Count: destCount})

	// Store dest count immediately so TUI can display it
	e.Status.mu.Lock()
	e.Status.TotalFilesInDest = destCount
	e.Status.mu.Unlock()

	// Update status with counts (TotalFilesInSource was already set above)
	e.Status.mu.Lock()
	e.Status.TotalFilesInSource = sourceCount
	e.Status.mu.Unlock()

	// If counts match, assume everything is fine (monotonic-count optimization)
	if sourceCount == destCount {
		e.logAnalysis("✓ File counts match - assuming directories are in sync (monotonic-count mode)")
		e.Status.mu.Lock()
		e.Status.AnalysisPhase = phaseComplete
		e.Status.TotalFiles = 0
		e.Status.TotalBytes = 0
		e.Status.FilesToSync = []*FileToSync{}
		e.Status.mu.Unlock()
		e.notifyStatusUpdate()

		return true, nil
	}

	e.logAnalysis(fmt.Sprintf("✗ File counts differ (%d vs %d) - proceeding with full scan", sourceCount, destCount))

	return false, nil
}

// handleCopyResult processes the result of a file copy operation
func (e *Engine) updateBottleneckDetection(stats *fileops.CopyStats) {
	if stats == nil {
		return
	}

	e.Status.TotalReadTime += stats.ReadTime
	e.Status.TotalWriteTime += stats.WriteTime

	// Determine bottleneck based on cumulative times
	totalTime := e.Status.TotalReadTime + e.Status.TotalWriteTime
	if totalTime == 0 {
		return
	}

	readPercent := float64(e.Status.TotalReadTime) / float64(totalTime)
	writePercent := float64(e.Status.TotalWriteTime) / float64(totalTime)

	// If one side is taking >60% of the time, it's the bottleneck
	switch {
	case readPercent > AdaptiveScalingIdleThreshold:
		e.Status.Bottleneck = "source"
	case writePercent > AdaptiveScalingIdleThreshold:
		e.Status.Bottleneck = "destination"
	default:
		e.Status.Bottleneck = "balanced"
	}
}

// updateDeletionStatus updates the engine status for file deletion progress.
func (e *Engine) updateDeletionStatus(checkedCount int, relPath string) {
	e.Status.mu.Lock()
	e.Status.ScannedFiles = checkedCount
	e.Status.CurrentPath = relPath
	e.Status.mu.Unlock()
}

func (e *Engine) updateStatusForFile(relPath string, srcFile *fileops.FileInfo, needsSync bool, comparedCount int) {
	e.Status.mu.Lock()
	defer e.Status.mu.Unlock()

	// Count all files in source
	e.Status.TotalFilesInSource++
	e.Status.TotalBytesInSource += srcFile.Size

	if needsSync {
		fileToSync := &FileToSync{
			RelativePath: relPath,
			Size:         srcFile.Size,
			Status:       "pending",
		}
		e.Status.FilesToSync = append(e.Status.FilesToSync, fileToSync)
		e.Status.TotalBytes += srcFile.Size
	} else {
		// File is already synced
		e.Status.AlreadySyncedFiles++
		e.Status.AlreadySyncedBytes += srcFile.Size
	}

	// Update progress
	e.Status.ScannedFiles = comparedCount
	e.Status.CurrentPath = relPath
}

// worker is a worker goroutine that processes files from the jobs channel
func (e *Engine) worker(wg *sync.WaitGroup, jobs <-chan *FileToSync, errors chan<- error) {
	defer wg.Done()

	for fileToSync := range jobs {
		// Check for cancellation
		select {
		case <-e.cancelChan:
			return
		default:
		}

		err := e.syncFile(fileToSync)
		if err != nil {
			// syncFile already updated status and error tracking
			// Just send error to channel and check error limit
			e.notifyStatusUpdate()

			errors <- err

			// Check if we've hit the error limit (only count actual errors, not cancellations)
			e.Status.mu.RLock()
			errorCount := len(e.Status.Errors)
			e.Status.mu.RUnlock()

			if errorCount >= MaxErrorsBeforeAbort {
				// Stop processing more files
				return
			}
		}

		// Check if we should scale down (CAS-based worker removal)
		// This prevents stampede - only one worker wins the CAS and exits
		for {
			currentActive := atomic.LoadInt32(&e.Status.ActiveWorkers)
			desired := atomic.LoadInt32(&e.desiredWorkers)

			if currentActive <= desired {
				// At or below target, keep working
				break
			}

			// Try to atomically decrement activeWorkers
			if atomic.CompareAndSwapInt32(&e.Status.ActiveWorkers, currentActive, currentActive-1) {
				// Success: We won the race to decrement
				// This worker should exit
				e.logToFile(fmt.Sprintf("Worker exiting: scaled down %d -> %d", currentActive, currentActive-1))
				return
			}
			// CAS failed: Another worker already decremented, retry the check
		}
	}
}

// FileError represents an error that occurred while syncing a file
type FileError struct {
	FilePath string
	Error    error
}

// FileToSync represents a file that needs to be synchronized
type FileToSync struct {
	RelativePath string
	Size         int64
	Transferred  int64
	Status       string // "pending", "copying", "complete", "error"
	Error        error
}

// Status represents the current status of synchronization
type Status struct {
	TotalFiles        int
	ProcessedFiles    int
	FailedFiles       int // Number of files that failed to sync (excluding cancelled)
	CancelledFiles    int // Number of files cancelled during copy
	TotalBytes        int64
	TransferredBytes  int64
	CurrentFile       string   // Most recently updated file (for display)
	CurrentFiles      []string // All files currently being copied
	RecentlyCompleted []string // Recently completed files (for display)
	CurrentFileBytes  int64
	CurrentFileTotal  int64
	StartTime         time.Time
	EndTime           time.Time // Actual completion time
	BytesPerSecond    float64
	EstimatedTimeLeft time.Duration
	CompletionTime    time.Time // Estimated completion time
	FilesToSync       []*FileToSync
	Errors            []FileError // All errors encountered during sync (excluding cancellations)
	CancelledCopies   []string    // Files that were cancelled during copy

	// Overall statistics (including already-synced files)
	TotalFilesInSource int   // Total files found in source
	TotalFilesInDest   int   // Total files found in destination
	TotalBytesInSource int64 // Total bytes in source
	AlreadySyncedFiles int   // Files that were already up-to-date
	AlreadySyncedBytes int64 // Bytes that were already up-to-date

	// Comparison counts (for TUI display)
	FilesInBoth       int   // Files that exist in both source and dest
	FilesOnlyInSource int   // Files that exist only in source (new files)
	FilesOnlyInDest   int   // Files that exist only in dest (orphans)
	BytesInBoth       int64 // Bytes of files in both (no action needed)
	BytesOnlyInSource int64 // Bytes to copy
	BytesOnlyInDest   int64 // Bytes to delete

	// Deletion progress tracking
	FilesToDelete     int      // Total orphaned files to delete
	FilesDeleted      int      // Files successfully deleted so far
	BytesToDelete     int64    // Total bytes to delete
	BytesDeleted      int64    // Bytes deleted so far
	CurrentlyDeleting []string // Files currently being deleted
	DeletionComplete  bool     // Whether deletion phase is complete
	DeletionErrors    int      // Number of deletion errors

	// Analysis progress
	//nolint:lll // Inline comment listing all possible phase values
	AnalysisPhase    string   // "counting_source", "scanning_source", "counting_dest", "scanning_dest", "comparing", "planning", "complete"
	ScannedFiles     int      // Number of files scanned/compared so far (legacy, use Source/Dest specific)
	TotalFilesToScan int      // Total files to scan/compare (0 if unknown/counting)
	CurrentPath      string   // Current path being analyzed
	AnalysisLog      []string // Recent analysis activities

	// Separate source/dest scan progress (for parallel scanning)
	SourceScannedFiles int // Files scanned in source so far
	SourceTotalFiles   int // Total files in source (0 if still counting)
	DestScannedFiles   int // Files scanned in dest so far
	DestTotalFiles     int // Total files in dest (0 if still counting)

	// Analysis progress tracking for time estimation
	ScannedBytes      int64     // Bytes scanned so far
	TotalBytesToScan  int64     // Total bytes to scan (0 if unknown)
	AnalysisStartTime time.Time // When analysis started
	AnalysisRate      float64   // Items per second (rolling)

	// Concurrency tracking
	ActiveWorkers int32 // Current number of active workers (atomic)
	MaxWorkers    int   // Maximum workers reached
	AdaptiveMode  bool  // Whether adaptive concurrency is enabled

	// Performance tracking (for bottleneck detection)
	TotalReadTime  time.Duration // Total time spent reading from source
	TotalWriteTime time.Duration // Total time spent writing to destination
	Bottleneck     string        // "source", "destination", or "balanced"

	// Progress metrics (pre-computed for UI display)
	Progress ProgressMetrics // Pre-computed progress percentages
	Workers  WorkerMetrics   // Pre-computed worker performance metrics

	// Cleanup/finalization status
	FinalizationPhase string // "updating_cache", "complete", or empty

	mu sync.RWMutex
}

// CalculateAnalysisProgress calculates progress metrics for the analysis phase
//
//nolint:cyclop,nestif,mnd // Progress calculation requires multiple conditions and percentage constants
func (s *Status) CalculateAnalysisProgress() ProgressMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// During counting: TotalBytesToScan == 0
	// Note: We only check TotalBytesToScan because TotalFilesToScan is a temporary
	// counter that gets reset during each scan phase and doesn't reliably indicate
	// whether analysis is complete.
	if s.TotalBytesToScan == 0 {
		return ProgressMetrics{
			IsCounting:             true,
			FilesPercent:           0,
			BytesPercent:           0,
			TimePercent:            0,
			OverallPercent:         0,
			EstimatedTimeRemaining: 0,
		}
	}

	// During processing: Calculate percentages
	filesPercent := 0.0
	if s.TotalFilesToScan > 0 {
		filesPercent = (float64(s.ScannedFiles) / float64(s.TotalFilesToScan)) * 100
	}

	bytesPercent := 0.0
	if s.TotalBytesToScan > 0 {
		bytesPercent = (float64(s.ScannedBytes) / float64(s.TotalBytesToScan)) * 100
	}

	// Calculate time-based percentage
	timePercent := 0.0
	estimatedTimeRemaining := time.Duration(0)

	if !s.AnalysisStartTime.IsZero() {
		elapsed := time.Since(s.AnalysisStartTime).Seconds()
		if elapsed > 0 && s.AnalysisRate > 0 && s.TotalFilesToScan > 0 {
			// Estimate total time based on rate
			estimatedTotal := float64(s.TotalFilesToScan) / s.AnalysisRate
			if estimatedTotal > 0 {
				timePercent = (elapsed / estimatedTotal) * 100
				remainingSeconds := estimatedTotal - elapsed
				if remainingSeconds > 0 {
					estimatedTimeRemaining = time.Duration(remainingSeconds) * time.Second
				}
			}
		}
	}

	// Calculate overall as average of the three metrics
	overallPercent := (filesPercent + bytesPercent + timePercent) / 3.0

	// Clamp to 0-100 range
	if overallPercent < 0 {
		overallPercent = 0
	}

	if overallPercent > 100 {
		overallPercent = 100
	}

	return ProgressMetrics{
		IsCounting:             false,
		FilesPercent:           filesPercent,
		BytesPercent:           bytesPercent,
		TimePercent:            timePercent,
		OverallPercent:         overallPercent,
		EstimatedTimeRemaining: estimatedTimeRemaining,
	}
}

// ComputeProgressMetrics calculates all progress metrics and updates the
// Progress and Workers fields in the Status struct.
// Must be called with the Status mutex already locked.
func (s *Status) ComputeProgressMetrics() {
	s.Progress = s.calculateProgressMetrics()
	s.Workers = s.calculateWorkerMetrics()
}

// addRateSample adds a new sample to the rolling window, keeping only samples from the last 10 seconds.
// Must be called with the Status mutex already locked.
func (s *Status) addRateSample(sample RateSample) {
	const windowDuration = 10 * time.Second // Keep samples from last 10 seconds

	s.Workers.RecentSamples = append(s.Workers.RecentSamples, sample)

	// Prune samples older than windowDuration
	cutoffTime := sample.Timestamp.Add(-windowDuration)
	filtered := s.Workers.RecentSamples[:0] // Reuse underlying array
	for _, samp := range s.Workers.RecentSamples {
		if !samp.Timestamp.Before(cutoffTime) {
			filtered = append(filtered, samp)
		}
	}
	s.Workers.RecentSamples = filtered
}

// calculateAverageWorkers calculates the average number of active workers across samples.
func (s *Status) calculateAverageWorkers(samples []RateSample) float64 {
	if len(samples) == 0 {
		return 0
	}

	var totalWorkers int
	for _, sample := range samples {
		totalWorkers += sample.ActiveWorkers
	}

	return float64(totalWorkers) / float64(len(samples))
}

// calculateCumulativeRates calculates transfer rates from overall elapsed time.
func (s *Status) calculateCumulativeRates(metrics *WorkerMetrics) {
	if s.StartTime.IsZero() {
		return
	}

	elapsed := time.Since(s.StartTime)
	if elapsed <= 0 {
		return
	}

	totalBytes := atomic.LoadInt64(&s.TransferredBytes)
	metrics.TotalRate = float64(totalBytes) / elapsed.Seconds()

	activeWorkers := atomic.LoadInt32(&s.ActiveWorkers)
	if activeWorkers > 0 {
		metrics.PerWorkerRate = metrics.TotalRate / float64(activeWorkers)
	}
}

// calculateCumulativeWorkerMetrics calculates worker metrics from cumulative totals.
func (s *Status) calculateCumulativeWorkerMetrics(metrics *WorkerMetrics) {
	// Use cumulative totals as fallback
	totalTime := s.TotalReadTime + s.TotalWriteTime
	if totalTime > 0 {
		metrics.ReadPercent = (float64(s.TotalReadTime) / float64(totalTime)) * ProgressPercentageScale
		metrics.WritePercent = (float64(s.TotalWriteTime) / float64(totalTime)) * ProgressPercentageScale
	}

	// Calculate rate from overall progress
	s.calculateCumulativeRates(metrics)
}

// calculateProgressMetrics computes files%, bytes%, time%, and overall%.
func (s *Status) calculateProgressMetrics() ProgressMetrics {
	metrics := ProgressMetrics{}

	// Calculate files percentage
	if s.TotalFilesInSource > 0 {
		totalProcessed := s.AlreadySyncedFiles + s.ProcessedFiles
		metrics.FilesPercent = float64(totalProcessed) / float64(s.TotalFilesInSource)
	}

	// Calculate bytes percentage
	if s.TotalBytesInSource > 0 {
		totalProcessedBytes := s.AlreadySyncedBytes + atomic.LoadInt64(&s.TransferredBytes)
		metrics.BytesPercent = float64(totalProcessedBytes) / float64(s.TotalBytesInSource)
	}

	// Calculate time percentage
	//nolint:nestif // Complex time calculation logic requires nested conditions
	if !s.StartTime.IsZero() {
		elapsed := time.Since(s.StartTime)
		if s.EstimatedTimeLeft > 0 {
			totalEstimated := elapsed + s.EstimatedTimeLeft
			if totalEstimated > 0 {
				metrics.TimePercent = float64(elapsed) / float64(totalEstimated)
			}
		} else if elapsed > 0 {
			// If no ETA available but sync is running, assume we're early in progress
			// Use bytes or files progress as fallback
			if metrics.BytesPercent > 0 {
				metrics.TimePercent = metrics.BytesPercent
			} else if metrics.FilesPercent > 0 {
				metrics.TimePercent = metrics.FilesPercent
			}
		}
	}

	// Calculate overall percentage as simple average of the three
	metrics.OverallPercent = (metrics.FilesPercent + metrics.BytesPercent + metrics.TimePercent) / NumProgressDimensions

	return metrics
}

// calculateRates calculates total and per-worker transfer rates.
func (s *Status) calculateRates(metrics *WorkerMetrics, totalBytes int64, totalDuration time.Duration) {
	if totalDuration <= 0 {
		return
	}

	metrics.TotalRate = float64(totalBytes) / totalDuration.Seconds()

	// Calculate average workers across samples
	avgWorkers := s.calculateAverageWorkers(metrics.RecentSamples)
	if avgWorkers > 0 {
		metrics.PerWorkerRate = metrics.TotalRate / avgWorkers
	}
}

// calculateRollingWindowMetrics calculates worker metrics from recent samples.
func (s *Status) calculateRollingWindowMetrics(metrics *WorkerMetrics) {
	var totalBytes int64
	var totalReadTime time.Duration
	var totalWriteTime time.Duration
	var totalDuration time.Duration

	for index, sample := range metrics.RecentSamples {
		totalBytes += sample.BytesTransferred
		totalReadTime += sample.ReadTime
		totalWriteTime += sample.WriteTime

		// Calculate duration between samples
		if index > 0 {
			totalDuration += sample.Timestamp.Sub(metrics.RecentSamples[index-1].Timestamp)
		}
	}

	// Calculate read/write percentages
	totalIOTime := totalReadTime + totalWriteTime
	if totalIOTime > 0 {
		metrics.ReadPercent = (float64(totalReadTime) / float64(totalIOTime)) * ProgressPercentageScale
		metrics.WritePercent = (float64(totalWriteTime) / float64(totalIOTime)) * ProgressPercentageScale
	}

	// Calculate transfer rates
	s.calculateRates(metrics, totalBytes, totalDuration)
}

// calculateWorkerMetrics computes worker performance metrics using rolling window.
func (s *Status) calculateWorkerMetrics() WorkerMetrics {
	metrics := WorkerMetrics{}

	// Copy the recent samples for the UI
	metrics.RecentSamples = make([]RateSample, len(s.Workers.RecentSamples))
	copy(metrics.RecentSamples, s.Workers.RecentSamples)

	// If no samples yet, fall back to cumulative metrics
	if len(metrics.RecentSamples) == 0 {
		s.calculateCumulativeWorkerMetrics(&metrics)

		return metrics
	}

	// Calculate metrics from rolling window samples
	s.calculateRollingWindowMetrics(&metrics)

	return metrics
}

// unexported constants.
const (
	fileStatusComplete   = "complete"
	fileStatusCopying    = "copying"
	fileStatusError      = "error"
	fileStatusFinalizing = "finalizing" // After data transfer, before file close/chtimes
	fileStatusOpening    = "opening"    // Before first progress callback (file open/create in progress)
	// FileToSync status constants
	fileStatusPending = "pending"
	phaseComplete     = "complete"
	phaseCountingDest = "counting_dest"
	// Analysis phase constants
	phaseCountingSource = "counting_source"
)

type dirToDelete struct {
	relPath string
	depth   int
}

// countAndLogOrphanedItems counts and logs sample of orphaned items
// countOrphanedItems counts files and directories in destination that don't exist in source.
func countOrphanedItems(sourceFiles, destFiles map[string]*fileops.FileInfo) (int, int, int64) {
	filesToDelete := 0
	dirsToDelete := 0
	var bytesToDelete int64

	for relPath, dstFile := range destFiles {
		if _, exists := sourceFiles[relPath]; !exists {
			if dstFile.IsDir {
				dirsToDelete++
			} else {
				filesToDelete++
				bytesToDelete += dstFile.Size
			}
		}
	}

	return filesToDelete, dirsToDelete, bytesToDelete
}

// formatSyncReason formats the reason why a file needs to be synced.
func formatSyncReason(srcFile, dstFile *fileops.FileInfo) string {
	if srcFile.Size != dstFile.Size {
		return fmt.Sprintf("size mismatch: src=%d dst=%d", srcFile.Size, dstFile.Size)
	}

	if !srcFile.ModTime.Equal(dstFile.ModTime) {
		return fmt.Sprintf("modtime mismatch: src=%s dst=%s (diff=%s)",
			srcFile.ModTime.Format(time.RFC3339Nano),
			dstFile.ModTime.Format(time.RFC3339Nano),
			srcFile.ModTime.Sub(dstFile.ModTime))
	}

	return ""
}
