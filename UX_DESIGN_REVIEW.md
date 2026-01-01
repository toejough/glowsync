# UX Design Review: GlowSync TUI

**Date:** 2025-12-29
**Reviewer:** cli-tui-architect agent

## Executive Summary

This is a well-structured file synchronization TUI built with Bubble Tea. The recent improvements (inline errors, NO_COLOR support, ASCII fallbacks) are solid foundations. However, several opportunities exist to enhance user experience, reduce cognitive load, and improve accessibility.

---

## 1. User Flow and Navigation

### Strengths
- **Clear linear progression**: Input → Analysis → Sync → Summary follows a logical wizard-like pattern
- **Tab completion on Input screen** is intuitive and provides immediate value
- **Screen transitions are automatic** after successful completion of each phase

### Issues and Recommendations

**Issue 1.1: No Back Navigation**

The flow is strictly forward-only. If a user realizes they entered the wrong path after seeing the analysis, there is no way to go back.

Location: `internal/tui/screens/2_analysis.go:50-77`

```go
// Current: No handling for "back" navigation
func (s AnalysisScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    // ... no case for "esc" or "backspace" to return to input
    }
}
```

**Recommendation**: Add `Esc` or `Backspace` to return to the Input screen before sync begins. Show help text: "Press Esc to change paths".

---

**Issue 1.2: Confirmation Gap Before Sync**

The transition from Analysis to Sync is immediate. Users never see a summary of what will happen before committing.

Location: `internal/tui/screens/2_analysis.go:108-115`

```go
func (s AnalysisScreen) handleAnalysisComplete() (tea.Model, tea.Cmd) {
    // Immediately transitions - no confirmation
    return s, func() tea.Msg {
        return shared.TransitionToSyncMsg{
            Engine: s.engine,
        }
    }
}
```

**Recommendation**: Add a brief "Analysis Complete" confirmation screen showing:
- Number of files to copy
- Total size
- Number of files to delete (if any)
- "Press Enter to begin sync, Esc to cancel"

---

**Issue 1.3: Enter Key Overloaded on Input Screen**

On the Input screen, pressing Enter on source moves to destination, but Enter on destination validates and proceeds. This inconsistency can be confusing.

Location: `internal/tui/screens/1_input.go:169-195`

```go
func (s InputScreen) handleEnter() (tea.Model, tea.Cmd) {
    s.showCompletions = false
    if s.focusIndex == 0 && s.sourceInput.Value() != "" {
        // Move to destination input
        return s.moveToNextField()
    } else if s.focusIndex == 1 && s.destInput.Value() != "" {
        // Validate and proceed
        // ...
    }
}
```

**Recommendation**: Consider separating navigation (Tab/Shift+Tab or Up/Down) from submission (Enter always submits if both fields valid). Or add a visible "Start Sync" button/indicator when on destination field.

---

## 2. Visual Hierarchy and Information Density

### Strengths
- **Progress bars with percentages** give clear quantitative feedback
- **Spinner indicates activity** during analysis phases
- **Boxed content** creates clear visual boundaries
- **Color semantic consistency**: green=success, red=error, yellow=warning

### Issues and Recommendations

**Issue 2.1: Sync Screen Information Overload**

The sync screen displays 6+ distinct sections simultaneously:
1. Overall Progress
2. Session Progress
3. Statistics (workers, rate, elapsed, ETA)
4. Currently Copying list
5. Recent Files list
6. Errors

Location: `internal/tui/screens/3_sync.go:422-469`

```go
func (s SyncScreen) renderSyncingView() string {
    // Overall progress
    s.renderOverallProgress(&builder)
    // Session progress
    s.renderSessionProgress(&builder)
    // Statistics
    s.renderStatistics(&builder)
    // File list
    s.renderFileList(&builder)
    // Errors
    s.renderSyncingErrors(&builder)
    // ... help text
}
```

**Recommendation**:
- Collapse "Overall Progress" and "This Session" into a single view by default, with keystroke to toggle detail view
- Use visual separators (horizontal rules) between sections
- Consider a two-column layout for statistics vs file list on wider terminals

---

**Issue 2.2: Summary Screen Dense Statistics**

The Summary screen lists many statistics in a flat list format without visual grouping.

Location: `internal/tui/screens/4_summary.go:261-298`

```go
func (s SummaryScreen) renderCompleteSummary(builder *strings.Builder, elapsed time.Duration) {
    // Long flat list of stats:
    // Total files in source: X
    // Already up-to-date: X
    // Files synced successfully: X
    // Files cancelled: X
    // Files failed: X
    // Total files to copy: X
    // Total bytes to copy: X
    // Time elapsed: X
    // Average speed: X
}
```

**Recommendation**: Group related stats visually:
```
+-- Source ------------------+
| Total files: 1,234 (2.5 GB)|
+-- This Session ------------+
| Synced: 45    Failed: 2    |
| Skipped: 1,189 (up-to-date)|
+-- Performance -------------+
| Duration: 2m 30s           |
| Speed: 15.2 MB/s           |
+----------------------------+
```

---

**Issue 2.3: Empty State Handling**

When there are no files to sync (already up-to-date), the UX could be more celebratory.

Location: Summary screen does not have special handling for "nothing to do" scenario.

**Recommendation**: Add a distinct "Already in sync!" message with a check mark when `status.TotalFiles == 0 && status.FailedFiles == 0`.

---

## 3. Feedback and Responsiveness

### Strengths
- **200ms status update throttle** prevents UI thrashing
- **Spinner animation** shows activity during analysis
- **Progress bars update in real-time** during sync
- **Phase text changes** to indicate analysis stage

### Issues and Recommendations

**Issue 3.1: No Feedback During Path Validation**

When user presses Enter to validate paths, there is a synchronous filesystem stat call with no visual feedback if the disk is slow.

Location: `internal/tui/screens/1_input.go:174-192`

```go
// Validate paths (synchronous, potentially slow)
err := s.config.ValidatePaths()
if err != nil {
    s.validationError = err.Error()
    return s, nil
}
```

**Recommendation**: For large/remote filesystems, consider:
- Brief spinner during validation
- Or async validation with immediate "Checking paths..." feedback

---

**Issue 3.2: Finalization Phase Unclear**

The finalization phase shows "Updating destination cache..." but users may not understand why this is necessary or how long it takes.

Location: `internal/tui/screens/3_sync.go:426-432`

```go
if s.status != nil && s.status.FinalizationPhase == "updating_cache" {
    builder.WriteString(shared.RenderTitle("Finalizing..."))
    builder.WriteString("\n\n")
    builder.WriteString(shared.RenderLabel("Updating destination cache..."))
    builder.WriteString(shared.RenderDim("(This helps the next sync run faster)"))
}
```

**Recommendation**:
- Show a progress indicator if the cache update is lengthy
- Rephrase to: "Saving state for incremental sync..."
- Consider showing file count being cached

---

**Issue 3.3: Cancel Feedback Delay**

After pressing Ctrl+C or q, the message "Cancelling... waiting for workers to finish" appears, but there is no progress indication for how long this might take.

Location: `internal/tui/screens/3_sync.go:463-466`

```go
if s.cancelled {
    builder.WriteString(shared.RenderDim("Cancelling... waiting for workers to finish"))
} else {
    builder.WriteString(shared.RenderDim("Press Ctrl+C or q to cancel"))
}
```

**Recommendation**:
- Show active worker count: "Cancelling... waiting for 3 workers to finish"
- Add timeout or force-quit option: "Press Ctrl+C again to force quit"

---

## 4. Error Handling and Recovery

### Strengths
- **Inline validation errors** on Input screen clear on navigation
- **Error count shown during sync** with expandable list
- **Error summary on completion** with file paths and messages
- **Truncated error messages** prevent overflow

### Issues and Recommendations

**Issue 4.1: Error Messages Not Actionable**

Validation errors like "source path does not exist" are shown, but users cannot quickly fix them.

Location: `internal/tui/screens/1_input.go:370-373`

```go
if s.validationError != "" {
    content += "\n" + shared.RenderError("Error: "+s.validationError) + "\n"
}
```

**Recommendation**:
- Return focus to the problematic field automatically
- Add suggestions: "Source path does not exist. Did you mean: ~/Documents/backup?"
- For permission errors: "Try running with sudo or check directory permissions"

---

**Issue 4.2: Sync Errors Lack Retry Option**

When files fail during sync, there is no way to retry just the failed files.

Location: Summary screen shows errors but offers no remediation.

**Recommendation**:
- Add "Press r to retry failed files" on Summary screen when `FailedFiles > 0`
- Or suggest CLI command: "Run again with: glowsync --source ... --retry-failed"

---

**Issue 4.3: Debug Log Path Hardcoded in Display**

The debug log path message always says "glowsync-debug.log" but the actual path is from environment or temp directory.

Location: `internal/tui/screens/4_summary.go:216,335`

```go
builder.WriteString(shared.RenderDim("Debug log saved to: glowsync-debug.log"))
```

But the log path is dynamically determined in analysis screen:
```go
logPath := os.Getenv("COPY_FILES_LOG")
if logPath == "" {
    logPath = filepath.Join(os.TempDir(), "glowsync-debug.log")
}
```

**Recommendation**: Pass the actual log path to the Summary screen and display it accurately.

---

## 5. Cognitive Load

### Strengths
- **Clear phase labels** ("Counting files...", "Scanning source...")
- **Help text at bottom** of each screen
- **Familiar keyboard shortcuts** (Ctrl+C, Tab, Enter)

### Issues and Recommendations

**Issue 5.1: Help Text Too Dense**

The Input screen help text is one long line:

Location: `internal/tui/screens/1_input.go:375-376`

```go
shared.RenderSubtitle("Tab/Shift+Tab to cycle • → to accept & continue • ↑↓ to switch fields • Enter to continue • Ctrl+C to quit")
```

**Recommendation**:
- Use multi-line format for clearer scanning
- Show context-sensitive help (only show Tab hints when completions are visible)
```
Navigation: Tab/Shift+Tab cycle completions | Up/Down switch fields
Actions:    Enter confirm | Right accept path | Ctrl+C quit
```

---

**Issue 5.2: Two Progress Bars Confusing**

"Overall Progress (All Files)" vs "This Session" distinction requires mental model building.

Location: `internal/tui/screens/3_sync.go:264-347`

**Recommendation**:
- Lead with session progress (what's actively happening)
- Make overall progress secondary or collapsed
- Use clearer labels: "Copying Now: 45/200 files" vs "After Sync: 1,234/1,234 files"

---

**Issue 5.3: Bottleneck Indicators Unclear**

The bottleneck emojis (source-limited, dest-limited, balanced) appear without explanation.

Location: `internal/tui/screens/3_sync.go:106-121`

```go
func (s SyncScreen) getBottleneckInfo() string {
    switch s.status.Bottleneck {
    case shared.StateSource:
        return " source-limited"
    case shared.StateDestination:
        return " dest-limited"
    case shared.StateBalanced:
        return " balanced"
    }
}
```

**Recommendation**:
- Show on first appearance with tooltip-style explanation
- Or use "source slow" / "dest slow" / "optimal" for immediate comprehension
- Consider only showing this in verbose/debug mode

---

## 6. Accessibility Considerations

### Strengths
- **NO_COLOR support** implemented via `colorsDisabled` check
- **ASCII fallbacks** for Unicode symbols (`[OK]` vs `✓`)
- **TERM=dumb detection** for limited terminals

### Issues and Recommendations

**Issue 6.1: Color-Only Status Differentiation**

File item status uses color as primary differentiator:

Location: `internal/tui/shared/styles.go:145-170`

```go
func FileItemCompleteStyle() lipgloss.Style {
    return lipgloss.NewStyle().Foreground(SuccessColor())
}
func FileItemCopyingStyle() lipgloss.Style {
    return lipgloss.NewStyle().Foreground(WarningColor())
}
func FileItemErrorStyle() lipgloss.Style {
    return lipgloss.NewStyle().Foreground(ErrorColor())
}
```

Location: `internal/tui/screens/3_sync.go:299-321` - symbols are used but inconsistently

**Recommendation**: Always pair color with icon/prefix:
```
✓ path/to/file.txt   (green, complete)
○ path/to/file.txt   (yellow, copying)
✗ path/to/file.txt   (red, error)
```

The `SuccessSymbol()`, `ErrorSymbol()`, `PendingSymbol()` exist but should be used consistently everywhere file status is shown.

---

**Issue 6.2: Progress Bar Accessibility**

Progress bars use color gradients but no text alternative when colors are disabled.

Location: Uses Bubble Tea's `progress.New(progress.WithDefaultGradient())`

**Recommendation**: When `colorsDisabled`, show ASCII progress bar: `[=========>          ] 45%`

---

**Issue 6.3: Missing Screen Reader Labels**

There are no ARIA-style announcements for screen changes or status updates.

**Recommendation**: For accessibility compliance, consider:
- Logging major events to stderr when `TERM=dumb`
- Printing status line updates for screen reader parsing

---

## 7. Consistency and Patterns

### Strengths
- **Consistent use of shared styles** through `shared.RenderLabel()`, `shared.RenderError()`, etc.
- **Consistent key bindings**: Ctrl+C works everywhere
- **Consistent box styling** for all screens

### Issues and Recommendations

**Issue 7.1: Inconsistent Exit Key Handling**

Different screens accept different exit keys:

Location: Summary screen (`internal/tui/screens/4_summary.go:46-49`):
```go
case shared.KeyCtrlC, "q", "enter":
    return s, tea.Quit
```

Location: Input screen (`internal/tui/screens/1_input.go:199-200`):
```go
case shared.KeyCtrlC, "esc":
    return s, tea.Quit
```

Location: Sync screen (`internal/tui/screens/3_sync.go:144-145`):
```go
case shared.KeyCtrlC, "q":
    // Cancel but don't quit
```

**Recommendation**:
- Standardize: `Ctrl+C` always quits immediately, `q` quits when safe, `Esc` goes back or cancels
- Document this in help text consistently

---

**Issue 7.2: Inconsistent Error Display Limits**

Different screens show different numbers of errors:

- Sync screen: 5 errors max (line 393)
- Complete summary: 10 errors max (line 232)
- Error view: 5 errors max (line 373)
- Cancelled view: 5 errors max (line 122)

**Recommendation**: Extract to a shared constant and use consistently, or scale based on terminal height.

---

**Issue 7.3: Inconsistent Path Truncation**

Some screens calculate `maxWidth` from terminal width, others use hardcoded values or don't truncate at all.

Location: Summary screen stores width but uses it inconsistently.

**Recommendation**: Use `shared.CalculateMaxPathWidth()` consistently everywhere paths are displayed.

---

## 8. Delight Factors and Polish

### Opportunities for Enhancement

**8.1: Add Completion Sound/Bell**

Terminal bell on sync completion for long-running operations:
```go
// When sync completes successfully
fmt.Print("\a") // Terminal bell
```

---

**8.2: Add Time Estimation During Analysis**

The analysis phase shows count but no ETA. For large directories, this creates uncertainty.

**Recommendation**: After initial counting, show "Analyzing ~1,234 files, about 30 seconds remaining..."

---

**8.3: Add Keyboard Shortcut Hints**

Show shortcuts as you type, similar to vim's showcmd:
```
[Tab: completions] [Enter: confirm] [Ctrl+C: quit]
```
Highlight the relevant one based on context.

---

**8.4: Add Summary Sparkline**

On completion, show a tiny visualization of the sync:
```
Transfer Rate: [..||||||...] 15.2 MB/s avg
               ^slow   ^fast
```

---

**8.5: Celebrate Success Better**

When sync completes with no errors, the current `✓ Sync Complete!` is functional but not delightful.

**Recommendation**:
- Use a larger celebration for big syncs: "Successfully synced 1,234 files (2.5 GB) in 2m 30s"
- Consider: `All files synchronized successfully.` (more professional tone)

---

## Priority Summary

| Priority | Issue | Impact | Effort |
|----------|-------|--------|--------|
| High | Add confirmation before sync starts | Prevents accidental syncs | Medium |
| High | Back navigation from Analysis | Prevents restart on mistakes | Medium |
| High | Consistent color + icon for accessibility | Accessibility compliance | Low |
| High | Fix hardcoded debug log path | User confusion | Low |
| Medium | Simplify dual progress bars | Cognitive load | Medium |
| Medium | Actionable error messages | User efficiency | Medium |
| Medium | Standardize exit key behavior | Consistency | Low |
| Medium | Cancel progress indicator | User anxiety | Low |
| Low | Multi-line help text | Scannability | Low |
| Low | Grouped summary statistics | Visual hierarchy | Medium |
| Low | Terminal bell on completion | Delight | Low |

---

## Implementation Plan

**Total Issues:** 25

### Phase 1: High Priority (4 issues)
1. Issue 4.3 - Fix hardcoded debug log path (Low effort)
2. Issue 6.1 - Consistent color + icon for accessibility (Low effort)
3. Issue 1.1 - Back navigation from Analysis (Medium effort)
4. Issue 1.2 - Confirmation before sync starts (Medium effort)

### Phase 2: Medium Priority (4 issues)
5. Issue 7.1 - Standardize exit key behavior (Low effort)
6. Issue 3.3 - Cancel progress indicator (Low effort)
7. Issue 4.1 - Actionable error messages (Medium effort)
8. Issue 5.2 - Simplify dual progress bars (Medium effort)

### Phase 3: Low Priority Quick Wins (6 issues)
9. Issue 5.1 - Multi-line help text (Low effort)
10. Issue 7.2 - Inconsistent error display limits (Low effort)
11. Issue 7.3 - Inconsistent path truncation (Low effort)
12. Issue 2.3 - Empty state handling (Low effort)
13. Issue 8.1 - Add completion sound/bell (Low effort)
14. Issue 8.5 - Celebrate success better (Low effort)

### Phase 4: Remaining Features (11 issues)
15. Issue 1.3 - Enter key overloaded
16. Issue 2.1 - Sync screen information overload
17. Issue 2.2 - Summary screen dense statistics
18. Issue 3.1 - No feedback during path validation
19. Issue 3.2 - Finalization phase unclear
20. Issue 4.2 - Sync errors lack retry option
21. Issue 5.3 - Bottleneck indicators unclear
22. Issue 6.2 - Progress bar accessibility
23. Issue 6.3 - Missing screen reader labels
24. Issue 8.2 - Add time estimation during analysis
25. Issue 8.3 - Add keyboard shortcut hints
26. Issue 8.4 - Add summary sparkline

---

## Conclusion

This TUI has a solid foundation with good architectural separation between screens and shared components. The recent accessibility improvements (NO_COLOR, ASCII fallbacks) are excellent additions. Focusing on the high-priority items—particularly the confirmation flow and accessibility consistency—would significantly improve user experience and confidence in the tool.
