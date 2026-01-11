# Architecture Reset Plan

## Purpose

This document outlines a fundamental reset of the glowsync architecture to address systemic issues with testing, state management, and UX clarity.

---

## Part 1: Current State Analysis

### 1.1 Current Workflow (What Actually Happens)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. INPUT                                                                    │
│    User provides: source path, dest path, options (workers, mode, etc.)     │
│    Output: Config object                                                    │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 2. QUICK CHECK (Monotonic Count Optimization)                               │
│    - Count files in source (just count, no metadata)                        │
│    - Count files in destination                                             │
│    - If counts match: assume in sync, skip to confirmation with 0 files     │
│    - If counts differ: proceed to full scan                                 │
│                                                                             │
│    Output: source_count, dest_count, optimization_succeeded (bool)          │
│    UX shows: "Counting (quick check) → N files"                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    │ counts match?                 │
                    ▼                               ▼
              [Skip to 6]                    [Continue to 3]
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 3. FULL SCAN                                                                │
│    - Scan source: get path, size, modtime for every file                    │
│    - Scan destination: same                                                 │
│                                                                             │
│    Output: source_files map, dest_files map                                 │
│    UX shows: "Counting (full scan) → N files" (SAME LABEL AS QUICK CHECK!)  │
│                                                                             │
│    PROBLEM: UX doesn't distinguish this from quick check. User sees two     │
│    "counting" phases but doesn't understand why or what's different.        │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 4. COMPARE                                                                  │
│    - For each source file, check if dest file exists                        │
│    - If exists: compare based on mode (size+time, content hash, etc.)       │
│    - Build list of files that need syncing                                  │
│                                                                             │
│    Output: files_to_sync list, already_synced count                         │
│    UX shows: "Comparing files"                                              │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 5. DELETE DETECTION                                                         │
│    - Find files in dest that don't exist in source (orphans)                │
│    - These will be deleted during sync                                      │
│                                                                             │
│    Output: files_to_delete list, dirs_to_delete list                        │
│    UX shows: "Checking for files to delete"                                 │
│                                                                             │
│    PROBLEM: This happens BEFORE confirmation. User hasn't agreed yet.       │
│    PROBLEM: Why is delete detection separate from sync planning?            │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 6. CONFIRMATION                                                             │
│    - Show summary: N files to copy, M files to delete, X bytes total        │
│    - Wait for user confirmation (unless --yes flag)                         │
│                                                                             │
│    Output: user_confirmed (bool)                                            │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 7. SYNC                                                                     │
│    - Copy files that need syncing                                           │
│    - Delete orphaned files (happens here, after confirmation)               │
│    - Report progress                                                        │
│                                                                             │
│    Output: files_copied, files_deleted, bytes_transferred, errors           │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 8. SUMMARY                                                                  │
│    - Show what was done                                                     │
│    - Show any errors                                                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Current Phase Constants (from code)

```go
// In sync.go
phaseCountingSource = "counting_source"
phaseCountingDest   = "counting_dest"
phaseScanningSource = "scanning_source"  // Never actually used! totalCount always 0
phaseScanningDest   = "scanning_dest"    // Never actually used!
phaseComparing      = "comparing"
phaseDeleting       = "deleting"
phaseComplete       = "complete"
```

**Problem**: `scanning_source` and `scanning_dest` are defined but never triggered because the code checks `if totalCount > 0` which is always false.

### 1.3 Current Status Fields (the mess)

```go
type Status struct {
    // Progress tracking (transient, changes rapidly)
    ScannedFiles     int      // Current count during scanning - RESETS BETWEEN PHASES
    TotalFilesToScan int      // Total expected - often 0 because we don't know ahead of time
    CurrentPath      string   // Currently processing this path

    // Results (should be stable after phase completes)
    TotalFilesInSource int    // Set after counting source
    TotalFilesInDest   int    // Set after counting dest
    TotalFiles         int    // Files that need syncing (confusing name!)
    TotalBytes         int64  // Bytes that need syncing

    // Phase tracking
    AnalysisPhase    string   // Current phase name

    // More fields...
}
```

**Problems**:

1. `ScannedFiles` is transient progress that resets between phases - TUI polls and often misses final value
2. `TotalFilesInSource` set during quick check, may not be updated during full scan
3. `TotalFiles` means "files to sync" not "total files" - confusing name
4. No clear separation between "progress during phase" and "result of phase"

### 1.4 TUI Polling Model (the root cause of bugs)

```
Engine (runs continuously)              TUI (polls every ~100ms)
──────────────────────────              ────────────────────────
Status.ScannedFiles = 100
Status.ScannedFiles = 500
Status.ScannedFiles = 1000              poll() → sees 1000, lastCount = 1000
Status.ScannedFiles = 5000
Status.ScannedFiles = 12108
Status.AnalysisPhase = "counting_dest"
Status.ScannedFiles = 0                 poll() → sees phase changed!
                                              records "counting_source" = 1000 (WRONG!)
                                              should be 12108
```

The TUI tracks "highest value seen" but if it doesn't poll at the right moment, it misses the final count. The fallback to `TotalFilesInSource` only triggers if count is exactly 0.

### 1.5 Testing Model (why bugs slip through)

**What tests exist**:

- Unit tests for engine methods with mocked filesystem
- Unit tests for TUI screens with pre-constructed status objects
- Unit tests for fileops with mocked filesystem

**What tests DON'T exist**:

- Integration tests: real engine + real filesystem + real TUI
- Timing tests: verify TUI captures correct values under various polling scenarios
- Contract tests: verify "when engine says 12108, TUI displays 12108"
- End-to-end tests: run app, create files, verify correct behavior

**The fundamental gap**: We test components in isolation with mocks, but never test that they work together correctly.

---

## Part 2: What's Intentional vs Accidental

### Intentional Design Decisions

1. **Quick check optimization**: If file counts match, assume in sync. This is a deliberate optimization for the common case where nothing changed.

2. **Separate source/dest scanning**: Scan each independently, then compare. Allows for different filesystems (local vs SFTP).

3. **Confirmation before sync**: User should approve before files are copied/deleted.

4. **Adaptive worker scaling**: Adjust concurrency based on throughput.

### Accidental/Organic Decisions

1. **Polling-based TUI updates**: Not a deliberate architecture decision, just "how it ended up." Race conditions are a side effect.

2. **Multiple overlapping status fields**: Grew organically as features were added. No unified design.

3. **Phase naming inconsistency**: `counting_source` vs `scanning_source` distinction exists in code but not in practice.

4. **Delete detection as separate phase**: Unclear if this was intentional or just how it was implemented.

5. **"Counting (quick check)" vs "Counting (full scan)" labels**: Added to explain why counting happens twice, but doesn't explain WHAT'S DIFFERENT about what we learn.

---

## Part 3: Proposed Improvements

### 3.1 Simplified Workflow (FINAL)

**UX Phases**: input → scan → compare → confirm → sync → summary

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. INPUT                                                                    │
│    User provides source, dest, options                                      │
│    UX: Input form/prompts                                                   │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 2. SCAN                                                                     │
│    Internally: quick check (count only) → full scan if needed (metadata)   │
│    User sees: "Scanning..." with progress toward result                     │
│                                                                             │
│    Progress display:                                                        │
│      "Scanning source... 5,000 files"                                       │
│      "Scanning source... 10,000 files"                                      │
│      "Scanning destination... 3,000 files"                                  │
│                                                                             │
│    Result:                                                                  │
│      "Found 12,108 files in source"                                         │
│      "Found 8,500 files in destination"                                     │
│                                                                             │
│    Output: source_files, dest_files (maps with metadata if full scan ran)   │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 3. COMPARE                                                                  │
│    Determine what needs syncing, what needs deleting                        │
│    User sees: "Comparing..." with progress toward result                    │
│                                                                             │
│    Progress display:                                                        │
│      "Comparing files... 2,000 / 12,108"                                    │
│                                                                             │
│    Result:                                                                  │
│      "3,608 files to copy (15.2 GB)"                                        │
│      "0 files to delete"                                                    │
│                                                                             │
│    Output: SyncPlan { files_to_copy, files_to_delete, bytes_to_copy }       │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 4. CONFIRM                                                                  │
│    Show plan summary, get user approval (skip if -y flag)                   │
│    UX: "Copy 3,608 files (15.2 GB). Delete 0 files. Proceed? [Y/n]"         │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 5. SYNC                                                                     │
│    Execute: copy files, delete orphans                                      │
│    User sees: progress toward completion                                    │
│                                                                             │
│    Progress display:                                                        │
│      "Syncing... 1,234 / 3,608 files (5.2 GB / 15.2 GB)"                    │
│      Current file: "videos/2024/vacation.mov"                               │
│                                                                             │
│    Output: files_copied, files_deleted, bytes_transferred, errors           │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 6. SUMMARY                                                                  │
│    What was done, any errors                                                │
│    UX: "Copied 3,608 files (15.2 GB) in 45 minutes. 0 errors."              │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Key principle**: Show progress during long operations, but frame it as progress toward a result, not as separate sub-phases. Quick check vs full scan is an internal optimization - user just sees "Scanning..."

### 3.2 Event-Based Communication (not polling)

**Current model** (polling):

```go
// Engine updates status continuously
engine.Status.ScannedFiles = count

// TUI polls periodically
func (s *Screen) Update(msg tea.Msg) {
    case tickMsg:
        s.status = s.engine.GetStatus()  // Hope we catch the right value!
}
```

**Proposed model** (events):

```go
// Engine emits events when significant things happen
type AnalysisEvent interface{}

type SourceCountComplete struct {
    Count int
}

type DestCountComplete struct {
    Count int
}

type ScanProgress struct {
    Phase    string
    Current  int
    Total    int  // 0 if unknown
}

type AnalysisComplete struct {
    Plan SyncPlan
}

// TUI subscribes to events
func (s *Screen) Update(msg tea.Msg) {
    case SourceCountComplete:
        s.sourceCount = msg.Count  // Guaranteed correct, no race
    case AnalysisComplete:
        s.plan = msg.Plan
}
```

**Benefits**:

- No race conditions - events are delivered, not polled
- Clear contract - engine specifies exactly what events it emits
- Testable - can verify specific events are emitted

### 3.3 Cleaner State Model

**Current** (overlapping fields):

```go
type Status struct {
    ScannedFiles       int    // Transient, resets
    TotalFilesInSource int    // Result, but when is it set?
    TotalFilesToScan   int    // What does this even mean?
    TotalFiles         int    // Actually means "files to sync"
    // ... more confusion
}
```

**Proposed** (clear phases and results):

```go
type AnalysisState struct {
    Phase Phase

    // Results (set once when phase completes, never changes)
    SourceCount    *int      // nil until source counting done
    DestCount      *int      // nil until dest counting done
    QuickCheckPass *bool     // nil until quick check done, true if counts match
    Plan           *SyncPlan // nil until analysis complete

    // Progress (only valid during certain phases)
    Progress *Progress
}

type Progress struct {
    Current int
    Total   int  // 0 if unknown
    Path    string
}

type SyncPlan struct {
    FilesToCopy   []FileInfo
    FilesToDelete []FileInfo
    BytesToCopy   int64
}
```

### 3.4 Testing Strategy (FINAL)

**Target**: 80% coverage, primarily through imptest

**Testing pyramid**:

```
                    ┌─────────────────┐
                    │  Integration    │  ← Minimal, behind -integration flag
                    │  (real I/O)     │     Validates overall flow works
                    └────────┬────────┘
                             │
              ┌──────────────┴──────────────┐
              │       Behavior Tests        │  ← Primary coverage via imptest
              │   (mock impure I/O)         │     "Given input X, calls dep Y with Z,
              │                             │      when dep returns W, function does V"
              └──────────────┬──────────────┘
                             │
         ┌───────────────────┴───────────────────┐
         │            Unit Tests                 │  ← Edge cases, pure functions
         │   (imptest where applicable)          │
         └───────────────────────────────────────┘
```

**Behavior test pattern (imptest)**:

```go
// Test: When Analyze() is called, it scans source, emits event with correct count
func TestAnalyze_ScansSource_EmitsCountEvent(t *testing.T) {
    t.Parallel()

    // Create mocks
    fsMock := MockFileSystem(t)
    scannerMock := MockFileScanner(t)
    eventSink := MockEventSink(t)

    engine := NewEngine(fsMock.Mock, eventSink.Mock)

    go func() {
        // Expect: engine calls fs.Scan("/source")
        fsMock.Method.Scan.ExpectCalledWithExactly("/source").
            InjectReturnValues(scannerMock.Mock)

        // Expect: engine iterates scanner
        scannerMock.Method.Next.ExpectCalled().InjectReturnValues(FileInfo{...}, true)
        scannerMock.Method.Next.ExpectCalled().InjectReturnValues(FileInfo{...}, true)
        scannerMock.Method.Next.ExpectCalled().InjectReturnValues(FileInfo{}, false)
        scannerMock.Method.Err.ExpectCalled().InjectReturnValues(nil)

        // Expect: engine emits event with count 2
        eventSink.Method.Emit.ExpectCalledWithExactly(SourceScanComplete{Count: 2})
    }()

    err := engine.Analyze()
    g.Expect(err).ShouldNot(HaveOccurred())
}
```

**Integration test pattern** (behind flag):

```go
//go:build integration

func TestEndToEnd_ScanShowsCorrectCounts(t *testing.T) {
    // Setup: real temp directories with real files
    sourceDir := t.TempDir()
    createFiles(t, sourceDir, 100)

    destDir := t.TempDir()
    createFiles(t, destDir, 50)

    // Run real engine, collect real events
    engine, _ := NewEngine(sourceDir, destDir)
    events := collectEvents(engine)

    err := engine.Analyze()
    require.NoError(t, err)

    // Verify the numbers the user would see
    require.Equal(t, 100, findEvent[SourceScanComplete](events).Count)
    require.Equal(t, 50, findEvent[DestScanComplete](events).Count)
}
```

Run with: `go test -tags=integration ./...`

---

## Part 4: Questions for Discussion

### Workflow Questions

1. **Quick check optimization**: Is this valuable? It saves time when nothing changed, but adds complexity and the UX is confusing. Should we:
   - Keep it but explain better in UX?
   - Make it optional (flag to skip)?
   - Remove it entirely?

2. **Delete behavior**: Currently we detect deletes during analysis but execute during sync. Should we:
   - Keep as-is but make UX clearer?
   - Make delete a separate confirmation? ("These files will be DELETED: ... OK?")
   - Make delete opt-in (flag required)?

3. **Phase granularity**: How much detail should user see?
   - Minimal: "Analyzing... → Syncing... → Done"
   - Current: Show each sub-phase
   - Detailed: Show every operation

### Technical Questions

4. **Event vs polling**: Event-based is cleaner but requires refactoring. Worth it?

5. **Backwards compatibility**: Should new architecture maintain same CLI flags and behavior?

6. **Testing investment**: How much test coverage is "enough"? 80%? 95%? Integration tests only?

---

### Answers

1. keep it, but explain better in UX
2. keep as-is but make UX Clearer
3. show the same phases everywhere: input, scan, compare, confirm, sync, summary
4. event-based is cleaner and worth the refactoring
5. we need input, output, filter, and yes flags.
6. 80%, mostly through imptest, if possible. Mock the Impure IO. Validate your behavior. When a function gets called
   with an input, what dependencies are called with what args? When those dependencies return, what does the function
   do next? what does it return? Add minimal integration tests to validate the overall flow, and require a special test
   flag to actually run them. Finally, any other unit tests that are needed to cover edge cases (also with imptest where
   possible).

## Part 5: Implementation Plan

### Development Process: TDD with Check-ins

**Every feature follows this cycle:**

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. RED: Write failing test (imptest)                                        │
│    - Test describes expected behavior                                       │
│    - Test fails because implementation doesn't exist                        │
│    - git commit -m "test(scope): describe what behavior is being tested"    │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 2. GREEN: Implement minimally to pass                                       │
│    - Write just enough code to make the test pass                           │
│    - Don't over-engineer, don't add extras                                  │
│    - git commit -m "feat(scope): implement behavior"                        │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 3. REFACTOR: Clean up for readability + linter                              │
│    - Improve naming, structure, comments                                    │
│    - Fix any linter warnings                                                │
│    - git commit -m "refactor(scope): clean up implementation"               │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 4. CHECK-IN: Pause and sync with Joe                                        │
│    - Show what was done                                                     │
│    - Get feedback before proceeding                                         │
│    - Adjust course if needed                                                │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
                            [Next feature]
```

**Commit message format:**
```
<type>(scope): description

AI-Used: [claude]
```

Types: `test` (red), `feat` (green), `refactor` (refactor)

**Check-in template:**
```
Completed TDD cycle for: [feature name]

RED: [test file] - tests that [behavior]
GREEN: [impl file] - implements [behavior]
REFACTOR: [changes made]

Ready for feedback before proceeding to [next feature].
```

### Phase A: Define Event Contract

**A1. Define event types** (`internal/syncengine/events.go`)

```go
type Event interface{ isEvent() }

// Scan phase events
type ScanStarted struct{ Target string }           // "source" or "dest"
type ScanProgress struct{ Target string; Count int }
type ScanComplete struct{ Target string; Count int }

// Compare phase events
type CompareStarted struct{}
type CompareProgress struct{ Compared, Total int }
type CompareComplete struct{ Plan *SyncPlan }

// Sync phase events
type SyncStarted struct{}
type SyncProgress struct{ FilesCopied, FilesTotal int; BytesCopied, BytesTotal int64 }
type SyncFileStarted struct{ Path string; Size int64 }
type SyncFileComplete struct{ Path string }
type SyncComplete struct{ Result *SyncResult }

// Error events
type ErrorOccurred struct{ Phase string; Err error }
```

**A2. Define event emitter interface**

```go
type EventEmitter interface {
    Emit(event Event)
}
```

### Phase B: Add Event Emission to Engine

**B1. Inject EventEmitter dependency**
- Add `emitter EventEmitter` field to Engine
- Make it optional (nil = no events, for backwards compat)

**B2. Emit events at key points**
- Scan start/progress/complete for source and dest
- Compare start/progress/complete
- Sync start/progress/complete
- Write behavior tests (imptest) for each emission point

**B3. Write integration tests** (behind `-tags=integration`)
- Test: correct events emitted for quick-check-pass scenario
- Test: correct events emitted for full-scan scenario
- Test: event counts match actual file counts

### Phase C: Refactor TUI to Consume Events

**C1. Create channel-based event bridge**
- Bubble Tea uses `tea.Msg` for updates
- Create adapter: `EventEmitter` → `chan tea.Msg`
- TUI subscribes to channel

**C2. Update TUI screens to use events**
- Replace polling-based status reads with event handling
- Store results from completion events (guaranteed correct)
- Use progress events for live updates

**C3. Remove polling code**
- Delete `tickMsg` handling
- Delete `lastCount` tracking logic
- Delete fallback logic (no longer needed)

### Phase D: Unify UX Phases

**D1. Update phase display**
- input → scan → compare → confirm → sync → summary
- Remove internal phase names from UX
- Show progress as "toward result" not "sub-phases"

**D2. Update TUI screen structure**
- Consider: one screen per UX phase, or unified view?
- Ensure consistent phase display across all screens

### Phase E: Clean Up

**E1. Remove deprecated code**
- Old Status fields no longer used
- Old phase constants
- Polling infrastructure

**E2. Update documentation**
- README reflects new architecture
- Code comments explain event contract

**E3. Verify coverage**
- Run `go test -cover ./...`
- Target: 80% via imptest behavior tests

---

## Part 6: Success Criteria

### Must Have (Definition of Done)

- [ ] Event types defined and documented
- [ ] Engine emits events at all key points
- [ ] TUI consumes events (no polling for results)
- [ ] TUI displays correct file counts (the original bug is fixed)
- [ ] 80% test coverage via imptest behavior tests
- [ ] Integration tests exist (behind `-tags=integration`) and pass
- [ ] UX shows unified phases: input → scan → compare → confirm → sync → summary

### Verification

1. Run existing sync scenario that showed wrong counts
2. Verify TUI shows correct counts matching log output
3. Run `go test ./...` - all pass
4. Run `go test -tags=integration ./...` - all pass
5. Run `go test -cover ./...` - 80%+ coverage

---

## Appendix: File Locations

### New Files

```
internal/syncengine/
  events.go              - Event type definitions
  events_test.go         - Behavior tests for event emission

internal/tui/
  event_bridge.go        - Adapter: EventEmitter → tea.Msg channel

tests/integration/
  scan_test.go           - Integration tests for scan phase
  sync_test.go           - Integration tests for full workflow
  helpers.go             - Test utilities (file creation, event collection)
```

### Modified Files

```
internal/syncengine/
  sync.go                - Add EventEmitter injection, emit events
  sync_test.go           - Update tests to use imptest for behavior verification

internal/tui/
  screens/2_analysis.go  - Replace polling with event consumption
  screens/3_sync.go      - Replace polling with event consumption
  app.go                 - Wire up event bridge

pkg/fileops/
  fileops_di.go          - No changes needed (already correct)
```

### Files to Eventually Remove/Deprecate

```
internal/tui/
  screens/2_analysis.go  - Remove: lastCount tracking, fallback logic
                         - Remove: tickMsg handling for status polling
```
