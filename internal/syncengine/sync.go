// Package syncengine provides file synchronization functionality.
package syncengine

//go:generate impgen syncengine.Engine.Cancel
//go:generate impgen syncengine.Engine.EnableFileLogging
//go:generate impgen syncengine.Engine.CloseLog
//go:generate impgen syncengine.FormatBytes

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/pkg/fileops"
)

const (
	// MaxErrorsBeforeAbort is the maximum number of errors before we stop the sync
	MaxErrorsBeforeAbort = 10
)

// FormatBytes formats bytes into human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
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
	TotalBytesInSource int64 // Total bytes in source
	AlreadySyncedFiles int   // Files that were already up-to-date
	AlreadySyncedBytes int64 // Bytes that were already up-to-date

	// Analysis progress
	AnalysisPhase    string   // "counting_source", "scanning_source", "counting_dest", "scanning_dest", "comparing", "deleting", "complete"
	ScannedFiles     int      // Number of files scanned/compared so far
	TotalFilesToScan int      // Total files to scan/compare (0 if unknown/counting)
	CurrentPath      string   // Current path being analyzed
	AnalysisLog      []string // Recent analysis activities

	// Concurrency tracking
	ActiveWorkers int  // Current number of active workers
	MaxWorkers    int  // Maximum workers reached
	AdaptiveMode  bool // Whether adaptive concurrency is enabled

	// Performance tracking (for bottleneck detection)
	TotalReadTime  time.Duration // Total time spent reading from source
	TotalWriteTime time.Duration // Total time spent writing to destination
	Bottleneck     string        // "source", "destination", or "balanced"

	// Cleanup/finalization status
	FinalizationPhase string // "updating_cache", "complete", or empty

	mu sync.RWMutex
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

// Engine handles the synchronization process
type Engine struct {
	SourcePath      string
	DestPath        string
	Status          *Status
	Workers         int               // Number of concurrent workers (default: 4, 0 = adaptive)
	AdaptiveMode    bool              // Enable adaptive concurrency scaling
	ChangeType      config.ChangeType // Type of changes expected (default: MonotonicCount)
	FileOps         *fileops.FileOps  // File operations (for dependency injection)
	statusCallbacks []func(*Status)
	mu              sync.RWMutex
	cancelChan      chan struct{} // Channel to signal cancellation
	cancelOnce      sync.Once     // Ensure Cancel() is only called once
	logFile         *os.File      // Optional log file for debugging
	logMu           sync.Mutex    // Mutex for log file writes
}

// NewEngine creates a new sync engine
func NewEngine(source, dest string) *Engine {
	return &Engine{
		SourcePath: source,
		DestPath:   dest,
		Workers:    4,                        // Default to 4 concurrent workers
		ChangeType: config.MonotonicCount,    // Default to monotonic count
		FileOps:    fileops.NewRealFileOps(), // Use real filesystem by default
		Status: &Status{
			StartTime: time.Now(),
		},
		statusCallbacks: make([]func(*Status), 0),
		cancelChan:      make(chan struct{}),
	}
}

// Cancel stops the sync operation gracefully
func (e *Engine) Cancel() {
	e.cancelOnce.Do(func() {
		close(e.cancelChan)
	})
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

// CloseLog closes the log file if open
func (e *Engine) CloseLog() {
	if e.logFile != nil {
		e.logToFile(fmt.Sprintf("\n=== Sync Log Ended: %s ===", time.Now().Format(time.RFC3339)))
		_ = e.logFile.Close()
		e.logFile = nil
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

// RegisterStatusCallback registers a callback for status updates
func (e *Engine) RegisterStatusCallback(callback func(*Status)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.statusCallbacks = append(e.statusCallbacks, callback)
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

// tryMonotonicCountOptimization checks if file counts match in monotonic-count mode.
// Returns true if optimization succeeded (counts match), false if full scan is needed.
func (e *Engine) tryMonotonicCountOptimization() (bool, error) {
	if e.ChangeType != config.MonotonicCount {
		return false, nil
	}

	e.logAnalysis("Monotonic-count mode: checking file counts...")

	// Count source files
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = "counting_source"
	e.Status.mu.Unlock()

	sourceCount, err := e.FileOps.CountFilesWithProgress(e.SourcePath, func(path string, count int) {
		e.Status.mu.Lock()
		e.Status.ScannedFiles = count
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

	// Count destination files
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = "counting_dest"
	e.Status.ScannedFiles = 0
	e.Status.mu.Unlock()

	destCount, err := e.FileOps.CountFilesWithProgress(e.DestPath, func(path string, count int) {
		e.Status.mu.Lock()
		e.Status.ScannedFiles = count
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

	// Update status with counts
	e.Status.mu.Lock()
	e.Status.TotalFilesInSource = sourceCount
	e.Status.mu.Unlock()

	// If counts match, assume everything is fine (monotonic-count optimization)
	if sourceCount == destCount {
		e.logAnalysis("✓ File counts match - assuming directories are in sync (monotonic-count mode)")
		e.Status.mu.Lock()
		e.Status.AnalysisPhase = "complete"
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

// scanSourceDirectory scans the source directory and returns file information.
func (e *Engine) scanSourceDirectory() (map[string]*fileops.FileInfo, error) {
	e.logAnalysis("Scanning source: " + e.SourcePath)

	// Update analysis phase
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = "counting_source"
	e.Status.ScannedFiles = 0
	e.Status.TotalFilesToScan = 0
	e.Status.mu.Unlock()

	// Scan source directory with progress
	sourceFiles, err := e.FileOps.ScanDirectoryWithProgress(e.SourcePath, func(path string, scannedCount int, totalCount int) {
		e.Status.mu.Lock()
		e.Status.CurrentPath = path
		e.Status.ScannedFiles = scannedCount
		e.Status.TotalFilesToScan = totalCount

		// Update phase when we transition from counting to scanning
		if totalCount > 0 && e.Status.AnalysisPhase == "counting_source" {
			e.Status.AnalysisPhase = "scanning_source"
		}
		e.Status.mu.Unlock()

		// Log every 10 files to avoid spam
		if scannedCount%10 == 0 {
			if totalCount > 0 {
				e.logAnalysis(fmt.Sprintf("Scanning %d / %d files from source...", scannedCount, totalCount))
			} else {
				e.logAnalysis(fmt.Sprintf("Counting files in source: %d so far...", scannedCount))
			}
		}
		e.notifyStatusUpdate()
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan source: %w", err)
	}

	e.logAnalysis(fmt.Sprintf("Source scan complete: %d items found", len(sourceFiles)))

	return sourceFiles, nil
}

// scanDestinationDirectory scans the destination directory and returns file information.
func (e *Engine) scanDestinationDirectory() (map[string]*fileops.FileInfo, error) {
	e.logAnalysis("Scanning destination: " + e.DestPath)

	// Update analysis phase
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = "counting_dest"
	e.Status.ScannedFiles = 0
	e.Status.TotalFilesToScan = 0
	e.Status.mu.Unlock()

	// Scan destination directory with progress
	destFiles, err := e.FileOps.ScanDirectoryWithProgress(e.DestPath, func(path string, scannedCount int, totalCount int) {
		e.Status.mu.Lock()
		e.Status.CurrentPath = path
		e.Status.ScannedFiles = scannedCount
		e.Status.TotalFilesToScan = totalCount

		// Update phase when we transition from counting to scanning
		if totalCount > 0 && e.Status.AnalysisPhase == "counting_dest" {
			e.Status.AnalysisPhase = "scanning_dest"
		}
		e.Status.mu.Unlock()

		// Log every 10 files to avoid spam
		if scannedCount%10 == 0 {
			if totalCount > 0 {
				e.logAnalysis(fmt.Sprintf("Scanning %d / %d files from destination...", scannedCount, totalCount))
			} else {
				e.logAnalysis(fmt.Sprintf("Counting files in destination: %d so far...", scannedCount))
			}
		}
		e.notifyStatusUpdate()
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to scan destination: %w", err)
	}
	if destFiles == nil {
		destFiles = make(map[string]*fileops.FileInfo)
		e.logAnalysis("Destination directory does not exist (will be created)")
	} else {
		e.logAnalysis(fmt.Sprintf("Destination scan complete: %d items found", len(destFiles)))
	}

	return destFiles, nil
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
	if comparedCount < 5 {
		if needsSync {
			e.logAnalysis(fmt.Sprintf("  → Hash mismatch: %s (src=%s... dst=%s...)",
				relPath, srcHash[:8], dstHash[:8]))
		} else {
			e.logAnalysis(fmt.Sprintf("  ✓ Hash match: %s (%s...)", relPath, srcHash[:8]))
		}
	}

	return needsSync
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
	if comparedCount < 5 {
		if needsSync {
			e.logAnalysis("  → Bytes differ: " + relPath)
		} else {
			e.logAnalysis("  ✓ Bytes match: " + relPath)
		}
	}

	return needsSync
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
		if sourceCount < 5 {
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
		if destCount < 5 {
			e.logAnalysis("  Dest: " + relPath)
			destCount++
		} else {
			break
		}
	}
}

// prepareComparisonLogMessage prepares a log message for file comparison results
func (e *Engine) prepareComparisonLogMessage(needsSync bool, needSyncCount, alreadySyncedCount int, relPath string, srcFile, dstFile *fileops.FileInfo) string {
	if needsSync {
		// Log first 10 files that need sync for debugging
		if needSyncCount <= 10 {
			if dstFile == nil {
				return fmt.Sprintf("  → Need sync: %s (destination missing)", relPath)
			}

			reason := ""
			if srcFile.Size != dstFile.Size {
				reason = fmt.Sprintf("size mismatch: src=%d dst=%d", srcFile.Size, dstFile.Size)
			} else if !srcFile.ModTime.Equal(dstFile.ModTime) {
				reason = fmt.Sprintf("modtime mismatch: src=%s dst=%s (diff=%s)",
					srcFile.ModTime.Format(time.RFC3339Nano),
					dstFile.ModTime.Format(time.RFC3339Nano),
					srcFile.ModTime.Sub(dstFile.ModTime))
			}
			return fmt.Sprintf("  → Need sync: %s (%s)", relPath, reason)
		}
	} else {
		// Log first 10 already-synced files for debugging
		if alreadySyncedCount <= 10 {
			return fmt.Sprintf("  ✓ Already synced: %s (size=%d, modtime=%s)",
				relPath, srcFile.Size, srcFile.ModTime.Format(time.RFC3339Nano))
		}
	}

	return ""
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

func (e *Engine) logComparisonSummary(sourceFiles, destFiles map[string]*fileops.FileInfo) {
	e.Status.mu.Lock()
	totalFiles := len(e.Status.FilesToSync)
	totalBytes := e.Status.TotalBytes
	alreadySynced := e.Status.AlreadySyncedFiles
	e.Status.mu.Unlock()

	e.logAnalysis(fmt.Sprintf("Found %d files to sync (%s total)", totalFiles, FormatBytes(totalBytes)))
	if alreadySynced > 0 {
		e.logAnalysis(fmt.Sprintf("%d files already up-to-date", alreadySynced))
	}

	// Log diagnostic info about the comparison
	e.logAnalysis(fmt.Sprintf("Comparison summary: %d source files, %d dest files, %d need sync, %d already synced",
		len(sourceFiles), len(destFiles), totalFiles, alreadySynced))
}

func (e *Engine) compareAndPlanSync(sourceFiles, destFiles map[string]*fileops.FileInfo) error {
	e.initializeComparisonStatus()

	comparedCount := 0
	needSyncCount := 0
	alreadySyncedCount := 0

	for relPath, srcFile := range sourceFiles {
		// Check for cancellation periodically (every 100 files)
		if comparedCount%100 == 0 {
			select {
			case <-e.cancelChan:
				return fmt.Errorf("analysis cancelled")
			default:
			}
		}

		if srcFile.IsDir {
			continue // Skip directories
		}

		dstFile := destFiles[relPath]

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

	return nil
}

// deleteOrphanedItems deletes files and directories from destination that don't exist in source
func (e *Engine) deleteOrphanedItems(sourceFiles, destFiles map[string]*fileops.FileInfo) error {
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = "deleting"
	e.Status.ScannedFiles = 0
	e.Status.TotalFilesToScan = len(destFiles)
	e.Status.mu.Unlock()

	// Count and log orphaned items
	filesToDelete, dirsToDelete := e.countAndLogOrphanedItems(sourceFiles, destFiles)

	// Delete files first (before directories)
	if err := e.deleteOrphanedFiles(sourceFiles, destFiles, filesToDelete); err != nil {
		return err
	}

	// Delete directories (in reverse depth order, deepest first)
	if err := e.deleteOrphanedDirectories(sourceFiles, destFiles, dirsToDelete); err != nil {
		return err
	}

	return nil
}

// countAndLogOrphanedItems counts and logs sample of orphaned items
func (e *Engine) countAndLogOrphanedItems(sourceFiles, destFiles map[string]*fileops.FileInfo) (int, int) {
	filesToDelete := 0
	dirsToDelete := 0

	// First pass: count files and directories to delete
	for relPath, dstFile := range destFiles {
		if _, exists := sourceFiles[relPath]; !exists {
			if dstFile.IsDir {
				dirsToDelete++
			} else {
				filesToDelete++
			}
		}
	}

	if filesToDelete > 0 || dirsToDelete > 0 {
		e.logAnalysis(fmt.Sprintf("Found %d files and %d directories in destination that don't exist in source", filesToDelete, dirsToDelete))

		// Log sample of destination files/dirs that don't exist in source
		loggedDeletes := 0
		e.logAnalysis("Sample destination items not in source:")
		for relPath, dstFile := range destFiles {
			if _, exists := sourceFiles[relPath]; !exists {
				if loggedDeletes < 5 {
					itemType := "file"
					if dstFile.IsDir {
						itemType = "dir"
					}
					e.logAnalysis(fmt.Sprintf("  Dest only (%s): %s", itemType, relPath))
					loggedDeletes++
				} else {
					break
				}
			}
		}
	}

	return filesToDelete, dirsToDelete
}

// deleteOrphanedFiles deletes files from destination that don't exist in source
func (e *Engine) deleteFile(relPath string, deletedCount int) error {
	dstPath := filepath.Join(e.DestPath, relPath)

	// Log first 10 deletions for debugging
	if deletedCount < 10 {
		e.logAnalysis(fmt.Sprintf("  → Deleting: %s (not in source)", relPath))
	}

	if err := e.FileOps.Remove(dstPath); err != nil {
		// Track error instead of failing
		e.Status.mu.Lock()
		e.Status.Errors = append(e.Status.Errors, FileError{
			FilePath: relPath,
			Error:    fmt.Errorf("failed to delete: %w", err),
		})
		errorCount := len(e.Status.Errors)
		e.Status.mu.Unlock()

		e.logAnalysis(fmt.Sprintf("✗ Error deleting %s: %v", relPath, err))

		// Check if we've hit the error limit
		if errorCount >= MaxErrorsBeforeAbort {
			return fmt.Errorf("too many errors (%d), aborting sync", errorCount)
		}
		return fmt.Errorf("delete failed") // Signal error but continue
	}

	return nil
}

func (e *Engine) logFileDeletionSummary(deletedCount, deleteErrorCount int) {
	if deletedCount == 0 && deleteErrorCount == 0 {
		return
	}

	if deleteErrorCount == 0 {
		e.logAnalysis(fmt.Sprintf("✓ Deleted %d files from destination", deletedCount))
	} else if deletedCount == 0 {
		e.logAnalysis(fmt.Sprintf("✗ Failed to delete all %d files (see errors below)", deleteErrorCount))
	} else {
		e.logAnalysis(fmt.Sprintf("Deleted %d files, failed to delete %d files (see errors below)",
			deletedCount, deleteErrorCount))
	}
}

func (e *Engine) deleteOrphanedFiles(sourceFiles, destFiles map[string]*fileops.FileInfo, filesToDelete int) error {
	if filesToDelete == 0 {
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
				return fmt.Errorf("analysis cancelled")
			default:
			}
		}

		if dstFile.IsDir {
			continue
		}

		checkedCount++
		e.Status.mu.Lock()
		e.Status.ScannedFiles = checkedCount
		e.Status.CurrentPath = relPath
		e.Status.mu.Unlock()

		if _, exists := sourceFiles[relPath]; !exists {
			// Delete this file from destination
			if err := e.deleteFile(relPath, deletedCount); err != nil {
				if err.Error() == "delete failed" {
					deleteErrorCount++
				} else {
					return err // Error limit reached
				}
			} else {
				deletedCount++
			}
		}

		// Notify every 100 files
		if checkedCount%100 == 0 {
			e.notifyStatusUpdate()
		}
	}

	// Summary of file deletion phase
	e.logFileDeletionSummary(deletedCount, deleteErrorCount)

	return nil
}

type dirToDelete struct {
	relPath string
	depth   int
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

func (e *Engine) deleteDirectory(relPath string, deletedCount int) error {
	dstPath := filepath.Join(e.DestPath, relPath)

	// Log first 10 directory deletions for debugging
	if deletedCount < 10 {
		e.logAnalysis(fmt.Sprintf("  → Deleting directory: %s (not in source)", relPath))
	}

	if err := e.FileOps.Remove(dstPath); err != nil {
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
			return fmt.Errorf("too many errors (%d), aborting sync", errorCount)
		}
		return fmt.Errorf("delete failed") // Signal error but continue
	}

	return nil
}

func (e *Engine) logDirectoryDeletionSummary(deletedCount, errorCount int) {
	if deletedCount == 0 && errorCount == 0 {
		return
	}

	if errorCount == 0 {
		e.logAnalysis(fmt.Sprintf("✓ Deleted %d directories from destination", deletedCount))
	} else if deletedCount == 0 {
		e.logAnalysis(fmt.Sprintf("✗ Failed to delete all %d directories (see errors below)", errorCount))
	} else {
		e.logAnalysis(fmt.Sprintf("Deleted %d directories, failed to delete %d directories (see errors below)",
			deletedCount, errorCount))
	}
}

// deleteOrphanedDirectories deletes directories from destination that don't exist in source
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
			return fmt.Errorf("analysis cancelled")
		default:
		}

		if err := e.deleteDirectory(dir.relPath, deletedDirCount); err != nil {
			if err.Error() == "delete failed" {
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

func (e *Engine) checkCancellation() error {
	select {
	case <-e.cancelChan:
		return fmt.Errorf("analysis cancelled")
	default:
		return nil
	}
}

func (e *Engine) finalizeAnalysis() {
	e.Status.mu.Lock()
	e.Status.TotalFiles = len(e.Status.FilesToSync)
	e.Status.AnalysisPhase = "complete"
	e.Status.mu.Unlock()

	e.logAnalysis("Analysis complete!")
	e.notifyStatusUpdate()
}

// Analyze scans source and destination to determine what needs to be synced
func (e *Engine) Analyze() error {
	e.logAnalysis("Starting analysis...")

	if err := e.checkCancellation(); err != nil {
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

	// Scan source directory
	sourceFiles, err := e.scanSourceDirectory()
	if err != nil {
		return err
	}

	if err := e.checkCancellation(); err != nil {
		return err
	}

	// Scan destination directory
	destFiles, err := e.scanDestinationDirectory()
	if err != nil {
		return err
	}

	if err := e.checkCancellation(); err != nil {
		return err
	}

	e.logSamplePaths(sourceFiles, destFiles)

	// Compare files and determine which need sync
	if err := e.compareAndPlanSync(sourceFiles, destFiles); err != nil {
		return err
	}

	// Delete orphaned files and directories from destination
	if err := e.deleteOrphanedItems(sourceFiles, destFiles); err != nil {
		return err
	}

	e.finalizeAnalysis()

	return nil
}

// Sync performs the actual synchronization using parallel workers
func (e *Engine) Sync() error {
	if e.AdaptiveMode {
		return e.syncAdaptive()
	}
	return e.syncFixed()
}

func (e *Engine) startFixedWorkers(numWorkers int, jobs chan *FileToSync, errors chan error) *sync.WaitGroup {
	var wg sync.WaitGroup
	for range numWorkers {
		wg.Go(func() {
			for fileToSync := range jobs {
				// Check for cancellation
				select {
				case <-e.cancelChan:
					return
				default:
				}

				if err := e.syncFile(fileToSync); err != nil {
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

func (e *Engine) collectErrors(errors chan error) ([]error, *sync.WaitGroup) {
	var allErrors []error
	var errorsMu sync.Mutex
	var errorsWg sync.WaitGroup
	errorsWg.Go(func() {
		for err := range errors {
			errorsMu.Lock()
			allErrors = append(allErrors, err)
			errorsMu.Unlock()
		}
	})
	return allErrors, &errorsWg
}

func (e *Engine) finalizeSyncPhase() {
	e.Status.mu.Lock()
	e.Status.EndTime = time.Now()
	processedFiles := e.Status.ProcessedFiles
	totalFiles := e.Status.TotalFiles
	e.Status.mu.Unlock()

	e.logToFile(fmt.Sprintf("Sync phase complete: %d / %d files copied", processedFiles, totalFiles))

	e.Status.mu.Lock()
	e.Status.FinalizationPhase = "complete"
	e.Status.mu.Unlock()
	e.notifyStatusUpdate()
}

// syncFixed uses a fixed number of workers
func (e *Engine) syncFixed() error {
	e.logToFile("Starting sync phase...")
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
	e.Status.ActiveWorkers = numWorkers
	e.Status.MaxWorkers = numWorkers
	e.Status.mu.Unlock()

	// Start worker pool
	wg := e.startFixedWorkers(numWorkers, jobs, errors)

	// Send all files to the job queue
	e.enqueueFilesForSync(jobs)

	// Collect errors concurrently
	allErrors, errorsWg := e.collectErrors(errors)

	// Wait for all workers to complete
	wg.Wait()
	close(errors)

	// Wait for error collector to finish
	errorsWg.Wait()

	// Finalize sync phase
	e.finalizeSyncPhase()

	// Return combined error if any failures occurred
	if len(allErrors) > 0 {
		return fmt.Errorf("%d file(s) failed to sync (first error: %w)", len(allErrors), allErrors[0])
	}

	return nil
}

// syncAdaptive uses adaptive concurrency that scales based on throughput
func (e *Engine) syncAdaptive() error {
	e.logToFile("Starting sync phase (adaptive mode)...")
	e.logToFile(fmt.Sprintf("Files to sync: %d", len(e.Status.FilesToSync)))

	e.Status.mu.Lock()
	e.Status.StartTime = time.Now()
	e.Status.AdaptiveMode = true
	e.Status.mu.Unlock()

	// Create channels for work distribution
	jobs := make(chan *FileToSync, 100) // Buffered channel for pending work
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
	var wg sync.WaitGroup
	workerControl := make(chan bool, 100) // true = add worker, false = remove worker
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
	e.Status.ActiveWorkers = activeWorkers
	e.Status.MaxWorkers = activeWorkers
	e.Status.mu.Unlock()
	e.notifyStatusUpdate()

	// Start background goroutines for adaptive scaling, worker control, and job distribution
	e.startAdaptiveScaling(done, jobs, workerControl)
	e.startWorkerControl(&wg, jobs, errors, workerControl)
	e.distributeJobs(jobs)

	// Collect errors concurrently to avoid blocking workers
	var allErrors []error
	var errorsMu sync.Mutex
	var errorsWg sync.WaitGroup
	errorsWg.Go(func() {
		for err := range errors {
			errorsMu.Lock()
			allErrors = append(allErrors, err)
			errorsMu.Unlock()
		}
	})

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
		return fmt.Errorf("sync aborted: too many errors (%d errors, limit is %d)", errorCount, MaxErrorsBeforeAbort)
	}

	// Mark finalization as complete
	e.Status.mu.Lock()
	e.Status.FinalizationPhase = "complete"
	e.Status.mu.Unlock()
	e.notifyStatusUpdate()

	// Return combined error if any failures occurred (but didn't hit limit)
	if len(allErrors) > 0 {
		return fmt.Errorf("%d file(s) failed to sync (see error details in completion screen)", len(allErrors))
	}

	return nil
}

// adaptiveScalingState holds the state for adaptive scaling algorithm
type adaptiveScalingState struct {
	lastProcessedFiles int
	lastPerWorkerSpeed float64
	filesAtLastCheck   int
	lastCheckTime      time.Time
}

// evaluateAndScale evaluates current performance and decides whether to add workers
func (e *Engine) evaluateAndScale(state *adaptiveScalingState, currentProcessedFiles, currentWorkers int, currentBytes int64, maxWorkers int, workerControl chan bool) {
	now := time.Now()

	// Calculate current per-worker speed (bytes per second per worker)
	if state.lastProcessedFiles > 0 && !state.lastCheckTime.IsZero() {
		filesProcessed := currentProcessedFiles - state.lastProcessedFiles
		elapsed := now.Sub(state.lastCheckTime).Seconds()

		if filesProcessed > 0 && elapsed > 0 {
			// Calculate per-worker throughput
			bytesPerFile := float64(currentBytes) / float64(currentProcessedFiles)
			filesPerSecond := float64(filesProcessed) / elapsed
			currentPerWorkerSpeed := (bytesPerFile * filesPerSecond) / float64(currentWorkers)

			e.logToFile(fmt.Sprintf("Adaptive: Evaluated %d workers over %d files in %.1fs - per-worker: %.2f MB/s (prev: %.2f MB/s)",
				currentWorkers, filesProcessed, elapsed, currentPerWorkerSpeed/1024/1024, state.lastPerWorkerSpeed/1024/1024))

			// Make scaling decision based on per-worker speed
			e.makeScalingDecision(state.lastPerWorkerSpeed, currentPerWorkerSpeed, currentWorkers, maxWorkers, workerControl)

			// Update tracking variables
			state.lastPerWorkerSpeed = currentPerWorkerSpeed
			state.lastProcessedFiles = currentProcessedFiles
			state.filesAtLastCheck = currentProcessedFiles
			state.lastCheckTime = now
		}
	} else {
		// First check - just record baseline
		state.lastProcessedFiles = currentProcessedFiles
		state.filesAtLastCheck = currentProcessedFiles
		state.lastCheckTime = now

		// Add a worker to start scaling
		if currentWorkers < maxWorkers {
			workerControl <- true
			filesSinceLastCheck := currentProcessedFiles - state.filesAtLastCheck
			e.logToFile(fmt.Sprintf("Adaptive: Baseline established after %d files, adding worker (%d -> %d)",
				filesSinceLastCheck, currentWorkers, currentWorkers+1))
		}
	}
}

// makeScalingDecision decides whether to add workers based on per-worker speed comparison
func (e *Engine) makeScalingDecision(lastPerWorkerSpeed, currentPerWorkerSpeed float64, currentWorkers, maxWorkers int, workerControl chan bool) {
	// First measurement - add a worker to test
	if lastPerWorkerSpeed == 0 {
		if currentWorkers < maxWorkers {
			workerControl <- true
			e.logToFile(fmt.Sprintf("Adaptive: First measurement complete, adding worker (%d -> %d)",
				currentWorkers, currentWorkers+1))
		}
		return
	}

	speedRatio := currentPerWorkerSpeed / lastPerWorkerSpeed

	// Per-worker speed decreased - don't add workers
	if speedRatio < 0.98 {
		e.logToFile(fmt.Sprintf("Adaptive: ↓ Per-worker speed decreased (-%.1f%%), not adding workers - will re-evaluate at %d workers",
			(1-speedRatio)*100, currentWorkers))
		return
	}

	// Per-worker speed maintained or improved - add a worker
	if currentWorkers >= maxWorkers {
		return
	}

	workerControl <- true
	if speedRatio >= 1.02 {
		e.logToFile(fmt.Sprintf("Adaptive: ↑ Per-worker speed improved (+%.1f%%), adding worker (%d -> %d)",
			(speedRatio-1)*100, currentWorkers, currentWorkers+1))
	} else {
		e.logToFile(fmt.Sprintf("Adaptive: → Per-worker speed stable, adding worker to test (%d -> %d)",
			currentWorkers, currentWorkers+1))
	}
}

// startAdaptiveScaling starts a goroutine that monitors performance and adjusts worker count
func (e *Engine) startAdaptiveScaling(done chan struct{}, jobs chan *FileToSync, workerControl chan bool) {
	// Use different algorithms for adaptive vs fixed mode
	if !e.AdaptiveMode {
		// Fixed mode - no scaling
		return
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		// Files-per-worker based scaling algorithm - continuously dynamic
		state := &adaptiveScalingState{}
		targetFilesPerWorker := 5
		maxWorkers := len(e.Status.FilesToSync) // Cap at total files

		e.logToFile("Adaptive: Starting with 1 worker, will continuously adjust based on per-worker efficiency")

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				e.Status.mu.RLock()
				currentProcessedFiles := e.Status.ProcessedFiles
				currentWorkers := e.Status.ActiveWorkers
				currentBytes := e.Status.TransferredBytes
				e.Status.mu.RUnlock()

				// Only scale if we have pending work
				pendingWork := len(jobs)
				if pendingWork == 0 {
					continue
				}

				// Calculate how many files have been processed since last check
				filesSinceLastCheck := currentProcessedFiles - state.filesAtLastCheck

				// Check if we've processed enough files to evaluate (targetFilesPerWorker files per worker)
				if filesSinceLastCheck >= currentWorkers*targetFilesPerWorker {
					e.evaluateAndScale(state, currentProcessedFiles, currentWorkers, currentBytes, maxWorkers, workerControl)
				}
			}
		}
	}()
}

// startWorkerControl starts a goroutine that manages adding workers dynamically
func (e *Engine) startWorkerControl(wg *sync.WaitGroup, jobs <-chan *FileToSync, errors chan<- error, workerControl chan bool) {
	go func() {
		for add := range workerControl {
			if add {
				wg.Add(1)
				go e.worker(wg, jobs, errors)

				e.Status.mu.Lock()
				e.Status.ActiveWorkers++
				if e.Status.ActiveWorkers > e.Status.MaxWorkers {
					e.Status.MaxWorkers = e.Status.ActiveWorkers
				}
				e.Status.mu.Unlock()
				e.notifyStatusUpdate()
			}
		}
	}()
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

		if err := e.syncFile(fileToSync); err != nil {
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
	}
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
	if _, err := e.FileOps.Stat(dstPath); err != nil {
		return false, nil // Destination doesn't exist, need to copy
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
	if err := e.FileOps.Chtimes(dstPath, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		return false, fmt.Errorf("failed to update modtime: %w", err)
	}

	// Mark file as complete without copying
	e.Status.mu.Lock()
	fileToSync.Status = "complete"
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

	return true, nil
}

// createProgressCallback creates a progress callback for file copying with throttling
func (e *Engine) createProgressCallback(fileToSync *FileToSync) func(int64, int64, string) {
	var previousBytes int64
	var lastNotifyTime time.Time

	return func(bytesTransferred, _ int64, _ string) {
		// Calculate delta without lock
		delta := bytesTransferred - previousBytes
		previousBytes = bytesTransferred

		// Use atomic add for the most frequently updated field
		atomic.AddInt64(&e.Status.TransferredBytes, delta)

		// Update per-file progress without full lock
		fileToSync.Transferred = bytesTransferred

		// Only acquire lock every 100ms to reduce contention
		now := time.Now()
		if now.Sub(lastNotifyTime) < 100*time.Millisecond {
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

func (e *Engine) syncFile(fileToSync *FileToSync) error {
	srcPath := filepath.Join(e.SourcePath, fileToSync.RelativePath)
	dstPath := filepath.Join(e.DestPath, fileToSync.RelativePath)

	e.Status.mu.Lock()
	e.Status.CurrentFile = fileToSync.RelativePath
	e.Status.CurrentFiles = append(e.Status.CurrentFiles, fileToSync.RelativePath)
	fileToSync.Status = "copying"
	e.Status.mu.Unlock()
	e.notifyStatusUpdate()

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
	stats, err := e.FileOps.CopyFileWithStats(srcPath, dstPath, progressCallback, e.cancelChan)

	// Update bottleneck detection and handle copy result
	return e.handleCopyResult(fileToSync, stats, err)
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
	if readPercent > 0.60 {
		e.Status.Bottleneck = "source"
	} else if writePercent > 0.60 {
		e.Status.Bottleneck = "destination"
	} else {
		e.Status.Bottleneck = "balanced"
	}
}

func (e *Engine) removeFromCurrentFiles(relativePath string) {
	for i, f := range e.Status.CurrentFiles {
		if f == relativePath {
			e.Status.CurrentFiles = append(e.Status.CurrentFiles[:i], e.Status.CurrentFiles[i+1:]...)
			break
		}
	}
}

func (e *Engine) handleCopyError(fileToSync *FileToSync, copyErr error) error {
	// Check if this was a cancellation vs an actual error
	if copyErr.Error() == "copy cancelled" {
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

func (e *Engine) handleCopySuccess(fileToSync *FileToSync) {
	fileToSync.Status = "complete"
	e.Status.ProcessedFiles++

	// Add to recently completed (keep last 10)
	e.Status.RecentlyCompleted = append(e.Status.RecentlyCompleted, fileToSync.RelativePath)
	if len(e.Status.RecentlyCompleted) > 10 {
		e.Status.RecentlyCompleted = e.Status.RecentlyCompleted[len(e.Status.RecentlyCompleted)-10:]
	}

	// Log first 10 completed files
	if e.Status.ProcessedFiles <= 10 {
		e.logToFile(fmt.Sprintf("  ✓ Copied: %s (%s)", fileToSync.RelativePath, FormatBytes(fileToSync.Size)))
	}
}

func (e *Engine) handleCopyResult(fileToSync *FileToSync, stats *fileops.CopyStats, copyErr error) error {
	e.Status.mu.Lock()

	// Track read/write times for bottleneck detection
	e.updateBottleneckDetection(stats)

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

// GetStatus returns a copy of the current status
func (e *Engine) GetStatus() *Status {
	e.Status.mu.RLock()
	defer e.Status.mu.RUnlock()

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
		TotalBytesInSource: e.Status.TotalBytesInSource,
		AlreadySyncedFiles: e.Status.AlreadySyncedFiles,
		AlreadySyncedBytes: e.Status.AlreadySyncedBytes,
		AnalysisPhase:      e.Status.AnalysisPhase,
		ScannedFiles:       e.Status.ScannedFiles,
		TotalFilesToScan:   e.Status.TotalFilesToScan,
		CurrentPath:        e.Status.CurrentPath,
		ActiveWorkers:      e.Status.ActiveWorkers,
		MaxWorkers:         e.Status.MaxWorkers,
		AdaptiveMode:       e.Status.AdaptiveMode,
		TotalReadTime:      e.Status.TotalReadTime,
		TotalWriteTime:     e.Status.TotalWriteTime,
		Bottleneck:         e.Status.Bottleneck,
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

	// Only copy recently active files from FilesToSync to reduce lock time
	// UI only needs to see files that are copying or recently completed
	// This prevents holding the lock for milliseconds copying 1000+ file pointers
	status.FilesToSync = make([]*FileToSync, 0, 20)
	recentCount := 0
	maxRecent := 20 // Only copy last 20 files (enough for UI display)
	for i := len(e.Status.FilesToSync) - 1; i >= 0 && recentCount < maxRecent; i-- {
		file := e.Status.FilesToSync[i]
		if file.Status == "copying" || file.Status == "complete" || file.Status == "error" {
			status.FilesToSync = append([]*FileToSync{file}, status.FilesToSync...)
			recentCount++
		}
	}

	return status
}
