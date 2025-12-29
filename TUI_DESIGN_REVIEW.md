# TUI/CLI Design Review: copy-files

**Review Date:** 2025-12-28
**Reviewer:** cli-tui-architect agent

## Executive Summary

This is a well-architected file synchronization tool with a polished Bubble Tea TUI. The codebase demonstrates solid engineering practices with clear separation of concerns, comprehensive documentation, and thoughtful screen flow design. Below is a detailed analysis with specific recommendations.

---

## 1. TUI Architecture and Screen Flow

### Strengths

**Excellent Screen-Based Architecture**
- The layered architecture (`app.go` as router, `screens/` for individual screens, `shared/` for utilities) is exemplary
- Numbered screen files (`1_input.go`, `2_analysis.go`, etc.) clearly communicate flow order
- The `AppModel` acts as a clean router handling transitions via message types

**Well-Defined Message Protocol**
- Clear separation between transition messages (`TransitionToAnalysisMsg`) and internal messages (`EngineInitializedMsg`)
- Messages are documented and centralized in `/Users/joe/repos/personal/copy-files/internal/tui/shared/messages.go`

**Proper State Management**
- Each screen owns its state
- Engine reference is properly passed through transitions
- Status updates use callbacks with appropriate throttling (200ms)

### Areas for Improvement

1. **Missing Confirmation Screen**
   - Currently flows directly from Analysis to Sync without user confirmation
   - Consider adding a confirmation step showing what will be synced before starting

2. **No Back Navigation**
   - Once in Analysis, users cannot go back to Input to fix paths
   - The flow is strictly forward-only

3. **Context Detection Missing**
   - In `/Users/joe/repos/personal/copy-files/cmd/copy-files/main.go` (line 23), the TUI always uses `tea.WithAltScreen()`
   - No detection of whether stdout is a TTY - if piped, should provide non-interactive output

**Recommendation:**
```go
// In main.go - Add TTY detection
opts := []tea.ProgramOption{}
if term.IsTerminal(int(os.Stdout.Fd())) {
    opts = append(opts, tea.WithAltScreen())
} else {
    // Non-interactive mode - output JSON/text
}
```

---

## 2. User Experience and Interaction Patterns

### Strengths

**Rich Path Completion**
- Tab/Shift+Tab cycling through completions
- Right arrow to accept and continue
- Visual completion list with windowing for long lists
- Hidden files properly filtered unless prefix starts with `.`

**Good Progress Feedback**
- Spinners during loading operations
- Progress bars for both overall and per-file progress
- Activity logs showing current operations
- ETA and transfer rate calculations

**Keyboard Navigation**
- Consistent keybindings: Ctrl+C to quit, Enter to proceed, Tab for completion
- Up/Down arrows for field navigation
- Clear help text at bottom of screens

### Areas for Improvement

1. **Inline Error Display (TODO in code)**
   - In `/Users/joe/repos/personal/copy-files/internal/tui/screens/1_input.go` (line 181):
   ```go
   // TODO: Show error inline instead of just staying on screen
   ```
   - Currently path validation errors are silent - user just stays on screen with no feedback

2. **No Keyboard Shortcut Reference**
   - The help text at the bottom is good but cramped
   - Consider a `?` key for full help overlay

3. **Mouse Support**
   - No mouse click support on completions
   - Could enhance accessibility

4. **Missing Input Validation Feedback**
   - No visual indicator when a path is valid/invalid
   - Could show checkmarks or colors as user types

5. **Window Resize Edge Cases**
   - Progress bar width is calculated in `handleWindowSize` but there's no minimum terminal size enforcement

**Recommendation for inline errors:**
```go
// In InputScreen, add an error field
type InputScreen struct {
    // ...
    validationError string
}

// In renderInputView, display error:
if s.validationError != "" {
    content += "\n" + shared.RenderError(s.validationError) + "\n"
}
```

---

## 3. Code Organization and Structure

### Strengths

**Excellent Documentation**
- Multiple README.md files explaining architecture
- Clear dependency rules documented
- Cyclomatic complexity targets documented and met

**Clean Package Structure**
```
internal/tui/
├── app.go          # Router
├── screens/        # Individual screens
├── shared/         # Shared utilities
└── docs/           # Documentation
```

**Proper Separation of Concerns**
- Sync engine (`internal/syncengine/`) is separate from TUI
- File operations (`pkg/fileops/`) are injectable
- Formatters (`pkg/formatters/`) are reusable

**Test Coverage**
- Tests exist for all major components
- Using Gomega matchers for expressive assertions
- Internal tests with `_internal_test.go` suffix

### Areas for Improvement

1. **Magic Numbers in Shared Package**
   - `/Users/joe/repos/personal/copy-files/internal/tui/shared/styles.go` has constants like `ProgressHalfDivisor = 2` which seem overly abstracted
   - Constants like `ProgressLogThreshold = 20` are used for multiple unrelated purposes (path display calculations AND log entry limits)

2. **Duplicate Code Patterns**
   - `truncatePath` is defined as a method on both `SyncScreen` and `SummaryScreen`, both just calling `shared.TruncatePath`
   - Consider removing the wrapper methods

3. **tickCmd Defined in analysis.go but used in sync.go**
   - The `tickMsg` type and `tickCmd` function are in `2_analysis.go` but also used by `3_sync.go`
   - Should be moved to shared/

4. **Long Lines with nolint Comments**
   - Many `//nolint:lll` comments suggest the code structure could be improved rather than suppressed

**Recommendation:**
Move tick-related code to shared:
```go
// In shared/tick.go
type TickMsg time.Time

func TickCmd() tea.Cmd {
    return tea.Tick(TickIntervalMs*time.Millisecond, func(t time.Time) tea.Msg {
        return TickMsg(t)
    })
}
```

---

## 4. Visual Design and Styling

### Strengths

**Consistent Color Palette**
- Semantic colors defined in `/Users/joe/repos/personal/copy-files/internal/tui/shared/styles.go`:
  - Primary (205 - pink/purple)
  - Success (42 - green)
  - Error (196 - red)
  - Warning (226 - yellow)

**Good Visual Hierarchy**
- Titles, subtitles, labels, and body text have distinct styles
- Box borders with rounded corners
- Appropriate padding and margins

**Status-Appropriate Styling**
- File items have different styles for pending, copying, complete, error states
- Progress bars use default gradient

### Areas for Improvement

1. **No Dark/Light Theme Detection**
   - Uses hardcoded ANSI color codes
   - No adaptation to terminal color scheme
   - Could use `lipgloss.HasDarkBackground()` for detection

2. **No NO_COLOR Support**
   - The `NO_COLOR` environment variable is not respected
   - Standard CLI convention to disable colors

3. **Unicode Symbols Without Fallback**
   - Uses Unicode characters like `▶`, `✓`, `✗`, `→` without ASCII fallback
   - Could fail on terminals with limited Unicode support

4. **Emoji in Production UI**
   - Several screens use emoji (e.g., `renderInputView` line 347: `"🚀 File Sync Tool"`)
   - Emoji rendering is inconsistent across terminals

**Recommendation:**
```go
// Add NO_COLOR detection in styles.go
func init() {
    if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
        // Disable all colors
    }
}

// Use ASCII fallback for symbols
func SuccessSymbol() string {
    if runtime.GOOS == "windows" || !supportsUnicode() {
        return "[OK]"
    }
    return "✓"
}
```

---

## 5. Error Handling and User Feedback

### Strengths

**Comprehensive Error Tracking**
- `FileError` struct captures both path and error
- `Status.Errors` slice tracks all errors
- Separate tracking for cancelled vs failed files

**Error Limits**
- `MaxErrorsBeforeAbort = 10` prevents runaway failures
- Clear error messages in summary screen

**Graceful Cancellation**
- Cancel channel properly signals workers
- Mid-copy cancellation supported
- Cancelled files tracked separately from failed files

### Areas for Improvement

1. **Silent Validation Failures**
   - As noted above, path validation in InputScreen provides no user feedback

2. **Error Messages Not Actionable**
   - Errors are displayed but don't suggest remediation
   - Example: "failed to delete directory" - could suggest checking permissions

3. **Log File Path Hardcoded**
   - `/Users/joe/repos/personal/copy-files/internal/tui/screens/2_analysis.go` line 137:
   ```go
   logPath := "copy-files-debug.log"
   ```
   - Should be configurable or use temp directory

4. **Error Display Truncation**
   - Errors are truncated to `maxWidth-3` characters which may hide critical information
   - Consider expandable error details

5. **No Retry Mechanism**
   - Failed files cannot be retried without restarting the entire sync

**Recommendation:**
```go
// Make log path configurable
logPath := os.Getenv("COPY_FILES_LOG")
if logPath == "" {
    logPath = filepath.Join(os.TempDir(), "copy-files-debug.log")
}
```

---

## 6. CLI Best Practices Assessment

### What's Done Well
- Version command (`--version`)
- Help text on flags
- Default values in config
- Subcommand-style `ChangeType` with aliases

### What's Missing

1. **No Shell Completion Support**
   - No fish/zsh/bash completion scripts
   - Could use `cobra` or generate completions

2. **No Man Page**
   - Complex tools benefit from `man copy-files`

3. **No JSON Output Mode**
   - For scripting: `--format=json` flag
   - Would enable parsing of sync results

4. **No Dry-Run Mode**
   - `--dry-run` to show what would be synced without doing it
   - Critical for safe preview

5. **No Quiet/Verbose Flags**
   - Standard `-q`/`-v` flags expected
   - Would be useful for scripted usage

6. **Exit Codes Not Documented**
   - Only 0 and 1 used
   - Should have specific codes for different failure types

**Recommendation for exit codes:**
```go
const (
    ExitSuccess       = 0
    ExitGeneralError  = 1
    ExitUsageError    = 2
    ExitPartialSync   = 3  // Some files failed
    ExitCancelled     = 4
    ExitPathError     = 5  // Source/dest issues
)
```

---

## 7. Performance Considerations

### Strengths
- Throttled status updates (200ms)
- Atomic operations for frequently updated fields (`TransferredBytes`)
- Worker pool with adaptive scaling
- Buffered channels for job distribution

### Areas for Improvement

1. **Status Copy Overhead**
   - `GetStatus()` in `/Users/joe/repos/personal/copy-files/internal/syncengine/sync.go` copies significant data on every call
   - Consider using a snapshot pattern with less frequent full copies

2. **Memory for Large File Lists**
   - `FilesToSync` slice could be very large
   - The optimization to only copy recent files (line 300-310) is good but could be more aggressive

---

## Summary of Key Recommendations

### High Priority
1. Add inline error display for path validation in InputScreen
2. Add NO_COLOR and TTY detection for proper CLI behavior
3. Add `--dry-run` mode for safe preview
4. Move `tickCmd` to shared package to eliminate duplication
5. Fix hardcoded log path

### Medium Priority
6. Add confirmation screen before sync starts
7. Add back navigation from Analysis to Input
8. Implement `--format=json` for scripted usage
9. Add ASCII fallbacks for Unicode symbols
10. Document exit codes

### Low Priority
11. Add shell completion support
12. Add man page generation
13. Add theme detection (dark/light)
14. Consider mouse support
15. Add retry mechanism for failed files

---

## Conclusion

This is a high-quality TUI implementation that follows modern Bubble Tea patterns well. The architecture is clean, well-documented, and maintainable. The main areas for improvement are around CLI best practices (TTY detection, NO_COLOR, output formats) and user feedback (inline errors, confirmation screens). The codebase would benefit from these enhancements but is already production-quality for its current feature set.