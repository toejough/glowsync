// Package sync provides file synchronization functionality.
package sync

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

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
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
	FailedFiles       int      // Number of files that failed to sync (excluding cancelled)
	CancelledFiles    int      // Number of files cancelled during copy
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
	TotalFilesInSource    int   // Total files found in source
	TotalBytesInSource    int64 // Total bytes in source
	AlreadySyncedFiles    int   // Files that were already up-to-date
	AlreadySyncedBytes    int64 // Bytes that were already up-to-date

	// Analysis progress
	AnalysisPhase     string   // "counting_source", "scanning_source", "counting_dest", "scanning_dest", "comparing", "deleting", "complete"
	ScannedFiles      int      // Number of files scanned/compared so far
	TotalFilesToScan  int      // Total files to scan/compare (0 if unknown/counting)
	CurrentPath       string   // Current path being analyzed
	AnalysisLog       []string // Recent analysis activities

	// Concurrency tracking
	ActiveWorkers     int      // Current number of active workers
	MaxWorkers        int      // Maximum workers reached
	AdaptiveMode      bool     // Whether adaptive concurrency is enabled

	// Performance tracking (for bottleneck detection)
	TotalReadTime     time.Duration // Total time spent reading from source
	TotalWriteTime    time.Duration // Total time spent writing to destination
	Bottleneck        string        // "source", "destination", or "balanced"

	// Cleanup/finalization status
	FinalizationPhase string // "updating_cache", "complete", or empty

	mu                sync.RWMutex
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
	Workers         int                // Number of concurrent workers (default: 4, 0 = adaptive)
	AdaptiveMode    bool               // Enable adaptive concurrency scaling
	ChangeType      config.ChangeType  // Type of changes expected (default: MonotonicCount)
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
		Workers:    4,                         // Default to 4 concurrent workers
		ChangeType: config.MonotonicCount,    // Default to monotonic count
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
	e.logToFile(fmt.Sprintf("Source: %s", e.SourcePath))
	e.logToFile(fmt.Sprintf("Destination: %s", e.DestPath))
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

// Analyze scans source and destination to determine what needs to be synced
func (e *Engine) Analyze() error {
	e.logAnalysis("Starting analysis...")

	// Check for cancellation at the start
	select {
	case <-e.cancelChan:
		return fmt.Errorf("analysis cancelled")
	default:
	}

	// For monotonic-count mode, first check if file counts match
	// If they do, we can skip detailed scanning (optimization)
	if e.ChangeType == config.MonotonicCount {
		e.logAnalysis("Monotonic-count mode: checking file counts...")

		// Count source files
		e.Status.mu.Lock()
		e.Status.AnalysisPhase = "counting_source"
		e.Status.mu.Unlock()

		sourceCount, err := fileops.CountFilesWithProgress(e.SourcePath, func(path string, count int) {
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
			return fmt.Errorf("failed to count source files: %w", err)
		}

		e.logAnalysis(fmt.Sprintf("Source file count: %d", sourceCount))

		// Count destination files
		e.Status.mu.Lock()
		e.Status.AnalysisPhase = "counting_dest"
		e.Status.ScannedFiles = 0
		e.Status.mu.Unlock()

		destCount, err := fileops.CountFilesWithProgress(e.DestPath, func(path string, count int) {
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
			return fmt.Errorf("failed to count destination files: %w", err)
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
			return nil
		}

		e.logAnalysis(fmt.Sprintf("✗ File counts differ (%d vs %d) - proceeding with full scan", sourceCount, destCount))
	}

	// Scan source directory
	e.logAnalysis(fmt.Sprintf("Scanning source: %s", e.SourcePath))

	// Update analysis phase
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = "counting_source"
	e.Status.ScannedFiles = 0
	e.Status.TotalFilesToScan = 0
	e.Status.mu.Unlock()

	// Scan source directory with progress
	sourceFiles, err := fileops.ScanDirectoryWithProgress(e.SourcePath, func(path string, scannedCount int, totalCount int) {
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
		return fmt.Errorf("failed to scan source: %w", err)
	}

	e.logAnalysis(fmt.Sprintf("Source scan complete: %d items found", len(sourceFiles)))

	// Check for cancellation after source scan
	select {
	case <-e.cancelChan:
		return fmt.Errorf("analysis cancelled")
	default:
	}

	// Scan destination directory
	e.logAnalysis(fmt.Sprintf("Scanning destination: %s", e.DestPath))

	// Update analysis phase
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = "counting_dest"
	e.Status.ScannedFiles = 0
	e.Status.TotalFilesToScan = 0
	e.Status.mu.Unlock()

	// Scan destination directory with progress
	destFiles, err := fileops.ScanDirectoryWithProgress(e.DestPath, func(path string, scannedCount int, totalCount int) {
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
		return fmt.Errorf("failed to scan destination: %w", err)
	}
	if destFiles == nil {
		destFiles = make(map[string]*fileops.FileInfo)
		e.logAnalysis("Destination directory does not exist (will be created)")
	} else {
		e.logAnalysis(fmt.Sprintf("Destination scan complete: %d items found", len(destFiles)))
	}

	// Check for cancellation after destination scan
	select {
	case <-e.cancelChan:
		return fmt.Errorf("analysis cancelled")
	default:
	}

	e.Status.mu.Lock()
	e.Status.AnalysisPhase = "comparing"
	e.Status.ScannedFiles = 0
	e.Status.TotalFilesToScan = len(sourceFiles)
	e.Status.mu.Unlock()

	e.logAnalysis(fmt.Sprintf("Comparing files to determine sync plan (%d files to compare)...", len(sourceFiles)))

	// Log sample of source and dest paths for debugging
	sourceCount := 0
	e.logAnalysis("Sample source paths:")
	for relPath := range sourceFiles {
		if sourceCount < 5 {
			e.logAnalysis(fmt.Sprintf("  Source: %s", relPath))
			sourceCount++
		} else {
			break
		}
	}

	destCount := 0
	e.logAnalysis("Sample destination paths:")
	for relPath := range destFiles {
		if destCount < 5 {
			e.logAnalysis(fmt.Sprintf("  Dest: %s", relPath))
			destCount++
		} else {
			break
		}
	}

	// Determine files to sync
	e.Status.mu.Lock()
	e.Status.FilesToSync = make([]*FileToSync, 0)
	e.Status.TotalBytes = 0
	e.Status.TotalFilesInSource = 0
	e.Status.TotalBytesInSource = 0
	e.Status.AlreadySyncedFiles = 0
	e.Status.AlreadySyncedBytes = 0
	e.Status.mu.Unlock()

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
		var needsSync bool
		switch e.ChangeType {
		case config.MonotonicCount, config.FluctuatingCount:
			// For count-based modes, only check if file exists (path comparison)
			needsSync = (dstFile == nil)
		default:
			// For other modes, use full comparison (size + modtime)
			needsSync = fileops.FilesNeedSync(srcFile, dstFile)
		}

		// Prepare log messages outside the lock
		var logMsg string

		if needsSync {
			needSyncCount++

			// Log first 10 files that need sync for debugging
			if needSyncCount <= 10 {
				if dstFile == nil {
					logMsg = fmt.Sprintf("  → Need sync: %s (destination missing)", relPath)
				} else {
					reason := ""
					if srcFile.Size != dstFile.Size {
						reason = fmt.Sprintf("size mismatch: src=%d dst=%d", srcFile.Size, dstFile.Size)
					} else if !srcFile.ModTime.Equal(dstFile.ModTime) {
						reason = fmt.Sprintf("modtime mismatch: src=%s dst=%s (diff=%s)",
							srcFile.ModTime.Format(time.RFC3339Nano),
							dstFile.ModTime.Format(time.RFC3339Nano),
							srcFile.ModTime.Sub(dstFile.ModTime))
					}
					logMsg = fmt.Sprintf("  → Need sync: %s (%s)", relPath, reason)
				}
			}
		} else {
			alreadySyncedCount++

			// Log first 10 already-synced files for debugging
			if alreadySyncedCount <= 10 {
				logMsg = fmt.Sprintf("  ✓ Already synced: %s (size=%d, modtime=%s)",
					relPath, srcFile.Size, srcFile.ModTime.Format(time.RFC3339Nano))
			}
		}

		// Now update status with lock
		e.Status.mu.Lock()
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
		comparedCount++
		e.Status.ScannedFiles = comparedCount
		e.Status.CurrentPath = relPath
		e.Status.mu.Unlock()

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

	e.Status.mu.Lock()
	totalFiles := len(e.Status.FilesToSync)
	totalBytes := e.Status.TotalBytes
	alreadySynced := e.Status.AlreadySyncedFiles
	e.Status.mu.Unlock()

	e.logAnalysis(fmt.Sprintf("Found %d files to sync (%s total)", totalFiles, formatBytes(totalBytes)))
	if alreadySynced > 0 {
		e.logAnalysis(fmt.Sprintf("%d files already up-to-date", alreadySynced))
	}

	// Log diagnostic info about the comparison
	e.logAnalysis(fmt.Sprintf("Comparison summary: %d source files, %d dest files, %d need sync, %d already synced",
		len(sourceFiles), len(destFiles), totalFiles, alreadySynced))

	// Find files and directories to delete (exist in dest but not in source)
	e.Status.mu.Lock()
	e.Status.AnalysisPhase = "deleting"
	e.Status.ScannedFiles = 0
	e.Status.TotalFilesToScan = len(destFiles)
	e.Status.mu.Unlock()

	deletedCount := 0
	deleteErrorCount := 0
	checkedCount := 0
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
			dstPath := filepath.Join(e.DestPath, relPath)

			// Log first 10 deletions for debugging
			if deletedCount < 10 {
				e.logAnalysis(fmt.Sprintf("  → Deleting: %s (not in source)", relPath))
			}

			if err := os.Remove(dstPath); err != nil {
				// Track error instead of failing
				e.Status.mu.Lock()
				e.Status.Errors = append(e.Status.Errors, FileError{
					FilePath: relPath,
					Error:    fmt.Errorf("failed to delete: %w", err),
				})
				deleteErrorCount++
				errorCount := len(e.Status.Errors)
				e.Status.mu.Unlock()

				e.logAnalysis(fmt.Sprintf("✗ Error deleting %s: %v", relPath, err))

				// Check if we've hit the error limit
				if errorCount >= MaxErrorsBeforeAbort {
					return fmt.Errorf("too many errors (%d), aborting sync", errorCount)
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
	if deletedCount > 0 || deleteErrorCount > 0 {
		if deleteErrorCount == 0 {
			e.logAnalysis(fmt.Sprintf("✓ Deleted %d files from destination", deletedCount))
		} else if deletedCount == 0 {
			e.logAnalysis(fmt.Sprintf("✗ Failed to delete all %d files (see errors below)", deleteErrorCount))
		} else {
			e.logAnalysis(fmt.Sprintf("Deleted %d files, failed to delete %d files (see errors below)",
				deletedCount, deleteErrorCount))
		}
	}

	// Now delete directories (in reverse depth order, deepest first)
	deletedDirCount := 0
	deleteDirErrorCount := 0

	if dirsToDelete > 0 {
		e.logAnalysis(fmt.Sprintf("Deleting %d orphaned directories...", dirsToDelete))

		// Collect directories to delete and sort by depth (deepest first)
		type dirToDelete struct {
			relPath string
			depth   int
		}
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

		// Delete directories
		for _, dir := range dirsToRemove {
			// Check for cancellation
			select {
			case <-e.cancelChan:
				return fmt.Errorf("analysis cancelled")
			default:
			}

			dstPath := filepath.Join(e.DestPath, dir.relPath)

			// Log first 10 directory deletions for debugging
			if deletedDirCount < 10 {
				e.logAnalysis(fmt.Sprintf("  → Deleting directory: %s (not in source)", dir.relPath))
			}

			if err := os.Remove(dstPath); err != nil {
				// Track error instead of failing
				e.Status.mu.Lock()
				e.Status.Errors = append(e.Status.Errors, FileError{
					FilePath: dir.relPath,
					Error:    fmt.Errorf("failed to delete directory: %w", err),
				})
				deleteDirErrorCount++
				errorCount := len(e.Status.Errors)
				e.Status.mu.Unlock()

				e.logAnalysis(fmt.Sprintf("✗ Error deleting directory %s: %v", dir.relPath, err))

				// Check if we've hit the error limit
				if errorCount >= MaxErrorsBeforeAbort {
					return fmt.Errorf("too many errors (%d), aborting sync", errorCount)
				}
			} else {
				deletedDirCount++
			}
		}

		// Summary of directory deletion
		if deletedDirCount > 0 || deleteDirErrorCount > 0 {
			if deleteDirErrorCount == 0 {
				e.logAnalysis(fmt.Sprintf("✓ Deleted %d directories from destination", deletedDirCount))
			} else if deletedDirCount == 0 {
				e.logAnalysis(fmt.Sprintf("✗ Failed to delete all %d directories (see errors below)", deleteDirErrorCount))
			} else {
				e.logAnalysis(fmt.Sprintf("Deleted %d directories, failed to delete %d directories (see errors below)",
					deletedDirCount, deleteDirErrorCount))
			}
		}
	}

	e.Status.mu.Lock()
	e.Status.TotalFiles = len(e.Status.FilesToSync)
	e.Status.AnalysisPhase = "complete"
	e.Status.mu.Unlock()

	e.logAnalysis("Analysis complete!")
	e.notifyStatusUpdate()

	return nil
}

// Sync performs the actual synchronization using parallel workers
func (e *Engine) Sync() error {
	if e.AdaptiveMode {
		return e.syncAdaptive()
	}
	return e.syncFixed()
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
	if numWorkers > len(e.Status.FilesToSync) {
		numWorkers = len(e.Status.FilesToSync)
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	e.Status.mu.Lock()
	e.Status.ActiveWorkers = numWorkers
	e.Status.MaxWorkers = numWorkers
	e.Status.mu.Unlock()

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
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
					// Just send error to channel for counting
					e.notifyStatusUpdate()
					errors <- err
				}
			}
		}()
	}

	// Send all files to the job queue (with cancellation check)
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

	// Collect errors concurrently to avoid blocking workers
	var allErrors []error
	var errorsMu sync.Mutex
	var errorsWg sync.WaitGroup
	errorsWg.Add(1)
	go func() {
		defer errorsWg.Done()
		for err := range errors {
			errorsMu.Lock()
			allErrors = append(allErrors, err)
			errorsMu.Unlock()
		}
	}()

	// Wait for all workers to complete
	wg.Wait()
	close(errors)

	// Wait for error collector to finish
	errorsWg.Wait()

	// Record completion time
	e.Status.mu.Lock()
	e.Status.EndTime = time.Now()
	processedFiles := e.Status.ProcessedFiles
	totalFiles := e.Status.TotalFiles
	e.Status.mu.Unlock()

	e.logToFile(fmt.Sprintf("Sync phase complete: %d / %d files copied", processedFiles, totalFiles))

	// Mark finalization as complete
	e.Status.mu.Lock()
	e.Status.FinalizationPhase = "complete"
	e.Status.mu.Unlock()
	e.notifyStatusUpdate()

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

	for i := 0; i < startWorkers; i++ {
		wg.Add(1)
		activeWorkers++
		go e.worker(&wg, jobs, errors)
	}

	e.Status.mu.Lock()
	e.Status.ActiveWorkers = activeWorkers
	e.Status.MaxWorkers = activeWorkers
	e.Status.mu.Unlock()
	e.notifyStatusUpdate()

	// Adaptive scaling goroutine
	go func() {
		// Use different algorithms for adaptive vs fixed mode
		if !e.AdaptiveMode {
			// Fixed mode - no scaling
			return
		}

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		// Files-per-worker based scaling algorithm - continuously dynamic
		var lastProcessedFiles int
		var lastPerWorkerSpeed float64
		var filesAtLastCheck int
		var lastCheckTime time.Time
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
				filesSinceLastCheck := currentProcessedFiles - filesAtLastCheck

				// Check if we've processed enough files to evaluate (targetFilesPerWorker files per worker)
				if filesSinceLastCheck >= currentWorkers*targetFilesPerWorker {
					now := time.Now()

					// Calculate current per-worker speed (bytes per second per worker)
					if lastProcessedFiles > 0 && !lastCheckTime.IsZero() {
						filesProcessed := currentProcessedFiles - lastProcessedFiles
						elapsed := now.Sub(lastCheckTime).Seconds()

						if filesProcessed > 0 && elapsed > 0 {
							// Calculate per-worker throughput
							bytesPerFile := float64(currentBytes) / float64(currentProcessedFiles)
							filesPerSecond := float64(filesProcessed) / elapsed
							currentPerWorkerSpeed := (bytesPerFile * filesPerSecond) / float64(currentWorkers)

							e.logToFile(fmt.Sprintf("Adaptive: Evaluated %d workers over %d files in %.1fs - per-worker: %.2f MB/s (prev: %.2f MB/s)",
								currentWorkers, filesProcessed, elapsed, currentPerWorkerSpeed/1024/1024, lastPerWorkerSpeed/1024/1024))

							// Decision logic: continuously adjust based on per-worker speed
							if lastPerWorkerSpeed > 0 {
								speedRatio := currentPerWorkerSpeed / lastPerWorkerSpeed

								if speedRatio >= 0.98 {
									// Per-worker speed maintained or improved - add a worker
									if currentWorkers < maxWorkers {
										workerControl <- true
										if speedRatio >= 1.02 {
											e.logToFile(fmt.Sprintf("Adaptive: ↑ Per-worker speed improved (+%.1f%%), adding worker (%d -> %d)",
												(speedRatio-1)*100, currentWorkers, currentWorkers+1))
										} else {
											e.logToFile(fmt.Sprintf("Adaptive: → Per-worker speed stable, adding worker to test (%d -> %d)",
												currentWorkers, currentWorkers+1))
										}
									}
								} else {
									// Per-worker speed decreased - don't add workers
									// Workers will naturally finish and count will drop
									// Then we'll re-evaluate with fewer workers
									e.logToFile(fmt.Sprintf("Adaptive: ↓ Per-worker speed decreased (-%.1f%%), not adding workers - will re-evaluate at %d workers",
										(1-speedRatio)*100, currentWorkers))
								}
							} else {
								// First measurement - add a worker to test
								if currentWorkers < maxWorkers {
									workerControl <- true
									e.logToFile(fmt.Sprintf("Adaptive: First measurement complete, adding worker (%d -> %d)",
										currentWorkers, currentWorkers+1))
								}
							}

							// Update tracking variables
							lastPerWorkerSpeed = currentPerWorkerSpeed
							lastProcessedFiles = currentProcessedFiles
							filesAtLastCheck = currentProcessedFiles
							lastCheckTime = now
						}
					} else {
						// First check - just record baseline
						lastProcessedFiles = currentProcessedFiles
						filesAtLastCheck = currentProcessedFiles
						lastCheckTime = now

						// Add a worker to start scaling
						if currentWorkers < maxWorkers {
							workerControl <- true
							e.logToFile(fmt.Sprintf("Adaptive: Baseline established after %d files, adding worker (%d -> %d)",
								filesSinceLastCheck, currentWorkers, currentWorkers+1))
						}
					}
				}
			}
		}
	}()

	// Worker control goroutine
	go func() {
		for add := range workerControl {
			if add {
				wg.Add(1)
				go e.worker(&wg, jobs, errors)

				e.Status.mu.Lock()
				e.Status.ActiveWorkers++
				if e.Status.ActiveWorkers > e.Status.MaxWorkers {
					e.Status.MaxWorkers = e.Status.ActiveWorkers
				}
				activeWorkers = e.Status.ActiveWorkers
				e.Status.mu.Unlock()
				e.notifyStatusUpdate()
			}
		}
	}()

	// Send all files to the job queue (with cancellation check)
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

	// Collect errors concurrently to avoid blocking workers
	var allErrors []error
	var errorsMu sync.Mutex
	var errorsWg sync.WaitGroup
	errorsWg.Add(1)
	go func() {
		defer errorsWg.Done()
		for err := range errors {
			errorsMu.Lock()
			allErrors = append(allErrors, err)
			errorsMu.Unlock()
		}
	}()

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
func (e *Engine) syncFile(fileToSync *FileToSync) error {
	srcPath := filepath.Join(e.SourcePath, fileToSync.RelativePath)
	dstPath := filepath.Join(e.DestPath, fileToSync.RelativePath)

	e.Status.mu.Lock()
	e.Status.CurrentFile = fileToSync.RelativePath
	e.Status.CurrentFiles = append(e.Status.CurrentFiles, fileToSync.RelativePath)
	fileToSync.Status = "copying"
	e.Status.mu.Unlock()
	e.notifyStatusUpdate()

	// Track this file's previous transferred bytes for delta calculation
	var previousBytes int64 = 0
	var lastNotifyTime time.Time

	// Progress callback for this file
	// Optimized to reduce lock contention - only notify UI every 100ms
	progressCallback := func(bytesTransferred, _ int64, _ string) {
		// Calculate delta without lock
		delta := bytesTransferred - previousBytes
		previousBytes = bytesTransferred

		// Use atomic add for the most frequently updated field
		atomic.AddInt64(&e.Status.TransferredBytes, delta)

		// Update per-file progress without full lock
		fileToSync.Transferred = bytesTransferred

		// Only acquire lock every 100ms to reduce contention
		// Don't notify UI here - let the TUI tick handle display updates
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

	// Copy the file with timing stats (pass cancel channel for mid-copy cancellation)
	stats, err := fileops.CopyFileWithStats(srcPath, dstPath, progressCallback, e.cancelChan)

	e.Status.mu.Lock()

	// Track read/write times for bottleneck detection (only if stats is not nil)
	if stats != nil {
		e.Status.TotalReadTime += stats.ReadTime
		e.Status.TotalWriteTime += stats.WriteTime

		// Determine bottleneck based on cumulative times
		totalTime := e.Status.TotalReadTime + e.Status.TotalWriteTime
		if totalTime > 0 {
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
	}

	// Remove from currently copying files
	for i, f := range e.Status.CurrentFiles {
		if f == fileToSync.RelativePath {
			e.Status.CurrentFiles = append(e.Status.CurrentFiles[:i], e.Status.CurrentFiles[i+1:]...)
			break
		}
	}

	if err != nil {
		// Check if this was a cancellation vs an actual error
		if err.Error() == "copy cancelled" {
			fileToSync.Status = "cancelled"
			e.Status.CancelledFiles++
			e.Status.CancelledCopies = append(e.Status.CancelledCopies, fileToSync.RelativePath)
		} else {
			fileToSync.Status = "error"
			fileToSync.Error = err
			e.Status.FailedFiles++
			e.Status.Errors = append(e.Status.Errors, FileError{
				FilePath: fileToSync.RelativePath,
				Error:    err,
			})
		}
		e.Status.mu.Unlock()
		return fmt.Errorf("failed to copy %s: %w", fileToSync.RelativePath, err)
	}

	fileToSync.Status = "complete"
	e.Status.ProcessedFiles++

	// Add to recently completed (keep last 10)
	e.Status.RecentlyCompleted = append(e.Status.RecentlyCompleted, fileToSync.RelativePath)
	if len(e.Status.RecentlyCompleted) > 10 {
		e.Status.RecentlyCompleted = e.Status.RecentlyCompleted[len(e.Status.RecentlyCompleted)-10:]
	}

	// Log first 10 completed files
	if e.Status.ProcessedFiles <= 10 {
		e.logToFile(fmt.Sprintf("  ✓ Copied: %s (%s)", fileToSync.RelativePath, formatBytes(fileToSync.Size)))
	}

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
		TotalFiles:        e.Status.TotalFiles,
		ProcessedFiles:    e.Status.ProcessedFiles,
		FailedFiles:       e.Status.FailedFiles,
		CancelledFiles:    e.Status.CancelledFiles,
		TotalBytes:        e.Status.TotalBytes,
		TransferredBytes:  atomic.LoadInt64(&e.Status.TransferredBytes),
		CurrentFile:       e.Status.CurrentFile,
		CurrentFileBytes:  e.Status.CurrentFileBytes,
		CurrentFileTotal:  e.Status.CurrentFileTotal,
		StartTime:         e.Status.StartTime,
		EndTime:           e.Status.EndTime,
		BytesPerSecond:    e.Status.BytesPerSecond,
		EstimatedTimeLeft: e.Status.EstimatedTimeLeft,
		CompletionTime:    e.Status.CompletionTime,
		TotalFilesInSource:    e.Status.TotalFilesInSource,
		TotalBytesInSource:    e.Status.TotalBytesInSource,
		AlreadySyncedFiles:    e.Status.AlreadySyncedFiles,
		AlreadySyncedBytes:    e.Status.AlreadySyncedBytes,
		AnalysisPhase:     e.Status.AnalysisPhase,
		ScannedFiles:      e.Status.ScannedFiles,
		TotalFilesToScan:  e.Status.TotalFilesToScan,
		CurrentPath:       e.Status.CurrentPath,
		ActiveWorkers:     e.Status.ActiveWorkers,
		MaxWorkers:        e.Status.MaxWorkers,
		AdaptiveMode:      e.Status.AdaptiveMode,
		TotalReadTime:     e.Status.TotalReadTime,
		TotalWriteTime:    e.Status.TotalWriteTime,
		Bottleneck:        e.Status.Bottleneck,
		FinalizationPhase: e.Status.FinalizationPhase,
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

