# UX Elements Reference

This document describes all UI elements, their data sources, and display rules.
Use this to plan UX changes by moving, reordering, or changing trigger conditions.

## How to Use This Document

- **Move elements** between phases by cutting/pasting their entry
- **Change triggers** by editing the `show_when` / `hide_when` fields
- **Reorder** elements within a phase by changing their position in the list
- **Add notes** in the `proposed_changes` field for discussion

---

## Current Architecture: Multi-Screen

```
PHASE: input → analysis → confirmation → sync → summary
SCREENS: 1_input.go → 2_analysis.go → 2.5_confirmation.go → 3_sync.go → 4_summary.go
```

Each phase = separate screen. Transitions destroy previous screen, create new one.

---

## Element Catalog

### ELEMENT: timeline_header

```yaml
id: timeline_header
description: Shows workflow progress (Input → Scan → Compare → Sync → Done)
current_phases: [analysis, confirmation, sync, summary]
missing_from: [input]

data_source: Current phase name
render_function: shared.RenderTimeline(phase)

show_when: Screen is not input
hide_when: Never (once shown)

display:
  format: "✓ Input ── ◉ Scan ── ○ Compare ── ○ Sync ── ○ Done"
  symbols:
    completed: ✓ (green)
    active: ◉ (primary)
    pending: ○ (dim)
    error: ✗ (red)
    skipped: ⊘ (dim)

proposed_changes: |
```

---

### ELEMENT: source_dest_context

```yaml
id: source_dest_context
description: Shows source path, dest path, and filter pattern for context
current_phases: [input, analysis, confirmation, sync, summary]

data_source:
  - config.SourcePath
  - config.DestPath
  - config.FilePattern (optional)

show_when: Always
hide_when: Never

display:
  input_phase: Editable text inputs with completions
  other_phases: Read-only boxes showing paths

proposed_changes: |
```

---

### ELEMENT: source_path_input

```yaml
id: source_path_input
description: Text input for source path with filesystem completions
current_phases: [input]

data_source: User input + os.ReadDir() for completions
render_function: textinput.Model

show_when: input phase
hide_when: Leave input phase

display:
  format: "Source Path:\n▶ [text input with cursor]"
  completions_shown_when: focused AND showCompletions=true
  max_completions_visible: 8 (windowed with "...")

controls:
  - TAB: Cycle completions
  - →: Accept completion
  - ESC: Clear field

proposed_changes: |
```

---

### ELEMENT: dest_path_input

```yaml
id: dest_path_input
description: Text input for destination path with filesystem completions
current_phases: [input]

data_source: User input + os.ReadDir() for completions
render_function: textinput.Model

show_when: input phase
hide_when: Leave input phase

display:
  format: "Destination Path:\n  [text input]"
  completions_shown_when: focused AND showCompletions=true

controls:
  - TAB: Cycle completions
  - →: Accept completion
  - ESC: Clear field

proposed_changes: |
```

---

### ELEMENT: filter_pattern_input

```yaml
id: filter_pattern_input
description: Text input for optional file filter pattern (e.g., *.mov)
current_phases: [input]

data_source: User input (no completions)
render_function: textinput.Model

show_when: input phase
hide_when: Leave input phase

display:
  format: "Filter Pattern (optional):\n  [text input]"
  placeholder: "*.mov"

proposed_changes: |
```

---

### ELEMENT: validation_error

```yaml
id: validation_error
description: Shows path validation errors
current_phases: [input]

data_source: config.ValidatePaths() error

show_when: User presses ENTER AND validation fails
hide_when: User types or navigates

display:
  format: Red error text below inputs
  style: ErrorStyle()

proposed_changes: |
```

---

### ELEMENT: spinner

```yaml
id: spinner
description: Animated spinner indicating activity
current_phases: [analysis, sync]

data_source: spinner.Model (ticks every 80ms)
render_function: spinner.View()

show_when:
  - analysis: Always during analysis
  - sync: Only during cancellation
hide_when: Phase completes

display:
  format: "⠋ [status text]"
  animation: Braille dots cycle

proposed_changes: |
```

---

### ELEMENT: phase_status_text

```yaml
id: phase_status_text
description: Current operation description
current_phases: [analysis]

data_source: status.AnalysisPhase

show_when: analysis phase
hide_when: Leave analysis phase

display:
  values:
    CountingSource: "Counting files in source..."
    ScanningSource: "Scanning source directory..."
    CountingDest: "Counting files in destination..."
    ScanningDest: "Scanning destination directory..."
    Comparing: "Comparing files to determine sync plan..."
    Deleting: "Checking for files to delete..."
    Complete: "Analysis complete!"

proposed_changes: |
```

---

### ELEMENT: counting_progress

```yaml
id: counting_progress
description: File count and scan rate during counting phases
current_phases: [analysis]

data_source:
  - status.ScannedFiles
  - status.TotalFilesToScan
  - Calculated scan rate (files/sec)

show_when: status.AnalysisPhase in [CountingSource, CountingDest]
hide_when: Enter scanning/comparing phase

display:
  format: "Found: 1,234 items (456 files/sec)"

proposed_changes: |
```

---

### ELEMENT: analysis_progress_bar

```yaml
id: analysis_progress_bar
description: Overall analysis progress bar
current_phases: [analysis]

data_source: status.CalculateAnalysisProgress()
render_function: shared.RenderProgress()

show_when: status.AnalysisPhase in [ScanningSource, ScanningDest, Comparing, Deleting]
hide_when: Counting phases OR analysis complete

display:
  format: "[════════════>              ] 45%"

proposed_changes: |
```

---

### ELEMENT: analysis_metrics

```yaml
id: analysis_metrics
description: Files/Bytes/Time breakdown during analysis
current_phases: [analysis]

data_source:
  - status.ProcessedFiles / status.TotalFiles
  - status.ProcessedBytes / status.TotalBytes
  - Elapsed / Estimated time

show_when: status.AnalysisPhase in [ScanningSource, ScanningDest, Comparing]
hide_when: Counting phases

display:
  format: |
    Files: 123 / 456 (27.0%)
    Bytes: 1.2 MB / 4.5 MB (26.7%)
    Time: 00:15 / 00:56 (26.8%)

proposed_changes: |
```

---

### ELEMENT: current_path

```yaml
id: current_path
description: File currently being scanned/processed
current_phases: [analysis]

data_source: status.CurrentPath

show_when: status.CurrentPath is not empty
hide_when: status.CurrentPath is empty

display:
  format: "Current: /truncated/path/to/file..."
  truncation: Middle-truncate to fit terminal width

proposed_changes: |
```

---

### ELEMENT: activity_log

```yaml
id: activity_log
description: Rolling log of recent operations
current_phases: [analysis]

data_source: status.AnalysisLog (last 10 entries)

show_when: analysis phase
hide_when: Leave analysis phase

display:
  format: |
    Activity Log:
      Scanning: /source/folder1
      Scanning: /source/folder2
      [...]
  max_entries: 10

proposed_changes: |
```

---

### ELEMENT: sync_plan_summary

```yaml
id: sync_plan_summary
description: Summary of files to sync after analysis
current_phases: [confirmation]

data_source:
  - status.TotalFiles
  - status.TotalBytes

show_when: confirmation phase
hide_when: Leave confirmation phase

display:
  format: |
    Files to sync: 42
    Total size: 1.2 GB

proposed_changes: |
```

---

### ELEMENT: filter_indicator

```yaml
id: filter_indicator
description: Shows active file filter pattern
current_phases: [confirmation]

data_source: engine.FilePattern

show_when: FilePattern is not empty
hide_when: FilePattern is empty

display:
  format: "Filtering by: *.mov"

proposed_changes: |
```

---

### ELEMENT: no_files_message

```yaml
id: no_files_message
description: Message when no files need syncing
current_phases: [confirmation]

data_source: status.TotalFiles == 0

show_when: TotalFiles == 0
hide_when: TotalFiles > 0

display:
  with_filter: "No files match your filter"
  without_filter: "All files already synced"

proposed_changes: |
```

---

### ELEMENT: sync_progress_bar

```yaml
id: sync_progress_bar
description: Overall sync progress bar
current_phases: [sync]

data_source: Average of (FilesPercent, BytesPercent, TimePercent)
render_function: shared.RenderProgress()

show_when: sync phase AND not cancelled
hide_when: Leave sync phase OR cancelled

display:
  format: "[════════════════════>              ]"

proposed_changes: |
```

---

### ELEMENT: sync_metrics

```yaml
id: sync_metrics
description: Files/Bytes/Time progress during sync
current_phases: [sync]

data_source:
  - status.ProcessedFiles / status.TotalFiles
  - status.TransferredBytes / status.TotalBytes
  - Elapsed / Estimated time

show_when: sync phase
hide_when: Leave sync phase

display:
  format: |
    Files: 42 / 100 (42.0%)
    Bytes: 1.2 GB / 2.8 GB (42.8%)
    Time: 02:15 / 05:20 (42.3%)

proposed_changes: |
```

---

### ELEMENT: worker_stats

```yaml
id: worker_stats
description: Active worker count and bottleneck indicator
current_phases: [sync, summary]

data_source:
  - status.ActiveWorkers
  - status.Bottleneck (adaptive mode only)

show_when: sync phase OR summary phase
hide_when: Never (carries through to summary)

display:
  format: "Workers: 8 🟢 optimal"
  bottleneck_indicators:
    source: "🔴 source slow"
    destination: "🟡 dest slow"
    balanced: "🟢 optimal"
  bottleneck_shown_when: Adaptive mode enabled

proposed_changes: |
```

---

### ELEMENT: transfer_speed_stats

```yaml
id: transfer_speed_stats
description: Read/Write percentages and transfer speeds
current_phases: [sync, summary]

data_source:
  - status.Workers.ReadPercent
  - status.Workers.WritePercent
  - status.Workers.TotalRate
  - status.Workers.PerWorkerRate

show_when: sync phase OR summary phase
hide_when: Never

display:
  sync_format: |
    R:60% / W:40%
    Speed: 45.2 MB/worker • 361.6 MB total
  summary_format: |
    Read speed: 600 MB/s • Write speed: 500 MB/s

proposed_changes: |
```

---

### ELEMENT: active_file_list

```yaml
id: active_file_list
description: Currently copying files with per-file progress bars
current_phases: [sync]

data_source:
  - status.CurrentFiles (list of active file paths)
  - status.FilesToSync (file details with progress)

show_when: len(CurrentFiles) > 0
hide_when: len(CurrentFiles) == 0

display:
  header: "Currently Copying (N):"
  per_file_format: "[████░░░] 45.3% video1.mov (copying)"
  status_labels:
    opening: "(waiting for dest)"
    copying: "(copying)"
    finalizing: "(finalizing)"
  max_files: Calculated from terminal height
  overflow: "... and X more files"

proposed_changes: |
```

---

### ELEMENT: recent_file_list

```yaml
id: recent_file_list
description: Recently completed files (shown when no active transfers)
current_phases: [sync]

data_source: status.FilesToSync (filtered to complete/error status)

show_when: len(CurrentFiles) == 0
hide_when: len(CurrentFiles) > 0

display:
  header: "Recent Files:"
  per_file_format: "✓ video1.mov"
  status_icons:
    complete: ✓ (green)
    error: ✗ (red)
    pending: ○ (dim)
    active: ⠋ (spinner)
  max_files: 5

proposed_changes: |
```

---

### ELEMENT: error_list

```yaml
id: error_list
description: List of file errors with context-sensitive limits
current_phases: [confirmation, sync, summary]

data_source: status.Errors

show_when: len(Errors) > 0
hide_when: len(Errors) == 0

display:
  format: |
    ✗ /path/to/file
      Permission denied
      Suggestion: Check file permissions...

  max_errors_by_context:
    in_progress: 3 # sync, confirmation
    complete: 10 # summary after success
    cancelled: 5 # summary after cancel
    error: 5 # summary after error

  overflow_message:
    in_progress: "... and X more (see summary)"
    complete: "... and X more error(s)"

proposed_changes: |
```

---

### ELEMENT: cancellation_view

```yaml
id: cancellation_view
description: Shown when user cancels sync
current_phases: [sync]

data_source:
  - status.ActiveWorkers
  - status.CurrentFiles

show_when: User pressed ESC/q during sync
hide_when: All workers finished

display:
  format: |
    🚫 Cancelling Sync

    ⠋ Waiting for workers to finish

    Active workers: 3

    Files being finalized:
      • video1.mov
      • photo2.jpg
      ... and 1 more files

proposed_changes: |
```

---

### ELEMENT: summary_header

```yaml
id: summary_header
description: Success/cancelled/error header with key stats
current_phases: [summary]

data_source:
  - finalState: "complete" | "cancelled" | "error"
  - status.ProcessedFiles
  - status.TransferredBytes
  - Elapsed time

show_when: summary phase
hide_when: Never

display:
  complete: "✓ Successfully synchronized 42 files (1.2 GB) in 2m 30s"
  cancelled: "⚠ Sync Cancelled"
  error: "✗ Sync Failed"

proposed_changes: |

```

---

### ELEMENT: source_totals

```yaml
id: source_totals
description: Total files/bytes in source directory
current_phases: [summary]

data_source:
  - status.TotalFilesInSource
  - status.TotalBytesInSource

show_when: summary phase
hide_when: Never

display:
  format: "Total files in source: 100 (2.8 GB)"

proposed_changes: |
```

---

### ELEMENT: already_synced_count

```yaml
id: already_synced_count
description: Files that were already up-to-date
current_phases: [summary]

data_source: status.AlreadySyncedFiles

show_when: summary phase
hide_when: Never

display:
  format: "Already up-to-date: 58 files (1.6 GB)"

proposed_changes: |
```

---

### ELEMENT: session_stats

```yaml
id: session_stats
description: This session's sync statistics
current_phases: [summary]

data_source:
  - status.ProcessedFiles
  - status.FailedFiles
  - status.CancelledFiles
  - status.TotalFiles
  - status.TotalBytes
  - Elapsed time
  - Calculated average speed

show_when: summary phase
hide_when: Never

display:
  format: |
    This Session:
    Files synced successfully: 42
    Files failed: 0
    Total files to copy: 42
    Total bytes to copy: 1.2 GB
    Time elapsed: 2m 30s
    Average speed: 487.5 MB/s

proposed_changes: |
```

---

### ELEMENT: recently_completed_list

```yaml
id: recently_completed_list
description: Last few completed files in summary
current_phases: [summary]

data_source: status.RecentlyCompleted

show_when: summary phase AND len(RecentlyCompleted) > 0
hide_when: len(RecentlyCompleted) == 0

display:
  header: "Recently Completed:"
  format: |
    ✓ video1.mov
    ✓ photo2.jpg
    ✓ document.pdf

proposed_changes: |
```

---

### ELEMENT: help_text

```yaml
id: help_text
description: Keyboard shortcuts and available actions
current_phases: [input, analysis, confirmation, sync, summary]

data_source: Static text per phase

show_when: Always
hide_when: Never

display:
  input: |
    Navigation: Tab/Shift+Tab • ↑↓
    Actions: → to accept • Enter to submit
    Other: Esc to clear • Ctrl+C to exit

  analysis: "Press Esc to change paths • Ctrl+C to exit"

  confirmation: "Press Enter to begin sync • Esc to cancel • Ctrl+C to exit"

  sync: "Press Esc or q to cancel • Ctrl+C to exit immediately"

  summary: |
    Press Enter or q to exit • Esc to start new sync
    Debug log: [clickable path]

proposed_changes: |
```

---

## Phase Transition Rules

```yaml
input_to_analysis:
  trigger: User presses ENTER AND validation passes
  message: TransitionToAnalysisMsg
  data_passed: SourcePath, DestPath

analysis_to_confirmation:
  trigger: AnalysisCompleteMsg AND NOT config.SkipConfirmation
  message: TransitionToConfirmationMsg
  data_passed: Engine, LogPath

analysis_to_sync:
  trigger: AnalysisCompleteMsg AND config.SkipConfirmation
  message: TransitionToSyncMsg
  data_passed: Engine, LogPath

confirmation_to_sync:
  trigger: User presses ENTER
  message: ConfirmSyncMsg → TransitionToSyncMsg
  data_passed: Engine, LogPath

sync_to_summary:
  trigger: SyncCompleteMsg OR ErrorMsg OR cancel complete
  message: TransitionToSummaryMsg
  data_passed: FinalState, Error

any_to_input:
  trigger: User presses ESC (analysis, confirmation, summary)
  message: TransitionToInputMsg
  data_passed: None (config preserved)

any_to_summary_error:
  trigger: Unrecoverable error
  message: TransitionToSummaryMsg
  data_passed: FinalState="error", Error
```

---

## Notes for Single-Screen Redesign

When converting to single-screen, consider:

1. **Element lifecycle** - Instead of "current_phases", use "appears_at" and "disappears_at" (or "persists")
2. **Animation triggers** - Add fields for slide-in, fade-in, collapse animations
3. **Stacking order** - Elements that appear later should stack below earlier ones
4. **State preservation** - Elements like source_dest_context should remain visible always
5. **Progressive disclosure** - New elements animate in as workflow progresses
