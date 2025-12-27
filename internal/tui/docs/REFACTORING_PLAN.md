# TUI Refactoring Plan: Layered Architecture

## New Directory Structure

```
internal/tui/
├── app.go                          # Top-level router (AppModel)
│
├── screens/                        # Sub-models (numbered by flow order)
│   ├── 1_input.go                  # InputScreen - path gathering
│   ├── 2_analysis.go               # AnalysisScreen - initializing & analyzing
│   ├── 3_sync.go                   # SyncScreen - syncing & cancelling
│   └── 4_summary.go                # SummaryScreen - complete/cancelled/error
│
├── shared/                         # Shared code used by multiple screens
│   ├── messages.go                 # Message types (transitions & internal)
│   ├── styles.go                   # Lipgloss styles
│   └── helpers.go                  # Shared helper functions (formatBytes, etc.)
│
├── ARCHITECTURE.md                 # Architecture documentation
├── COMPLEXITY_COMPARISON.md        # Before/after complexity analysis
│
└── [OLD FILES - to be deleted after migration]
    ├── model.go
    ├── update.go
    └── view.go
```

## File Responsibilities

### `app.go` - Top-Level Router
- **Type:** `AppModel`
- **Responsibility:** Route messages to active screen, handle screen transitions
- **Complexity:** ~7 (Update), ~6 (View)
- **Size:** ~150 lines

### `screens/1_input.go` - Input Screen
- **Type:** `InputScreen`
- **Responsibility:** Gather source and destination paths from user
- **States:** input
- **Messages Handled:** KeyMsg, WindowSizeMsg
- **Messages Emitted:** TransitionToAnalysisMsg
- **Complexity:** ~4
- **Size:** ~250 lines

### `screens/2_analysis.go` - Analysis Screen
- **Type:** `AnalysisScreen`
- **Responsibility:** Initialize engine and run analysis
- **States:** initializing, analyzing
- **Messages Handled:** EngineInitializedMsg, AnalysisCompleteMsg, ErrorMsg, spinner.TickMsg
- **Messages Emitted:** TransitionToSyncMsg, TransitionToSummaryMsg (on error)
- **Complexity:** ~5
- **Size:** ~200 lines

### `screens/3_sync.go` - Sync Screen
- **Type:** `SyncScreen`
- **Responsibility:** Run sync, handle cancellation, show progress
- **States:** syncing, cancelling
- **Messages Handled:** KeyMsg, WindowSizeMsg, tickMsg, SyncCompleteMsg, ErrorMsg, spinner.TickMsg
- **Messages Emitted:** TransitionToSummaryMsg
- **Complexity:** ~7
- **Size:** ~400 lines (includes progress rendering)

### `screens/4_summary.go` - Summary Screen
- **Type:** `SummaryScreen`
- **Responsibility:** Show final results (complete/cancelled/error)
- **States:** complete, cancelled, error
- **Messages Handled:** KeyMsg (quit only)
- **Messages Emitted:** None (terminal state)
- **Complexity:** ~2
- **Size:** ~300 lines (includes detailed summary rendering)

### `shared/messages.go` - Message Types
- Transition messages (TransitionToAnalysisMsg, etc.)
- Internal messages (EngineInitializedMsg, etc.)
- **Size:** ~50 lines

### `shared/styles.go` - Lipgloss Styles
- All lipgloss style definitions
- Moved from view.go
- **Size:** ~50 lines

### `shared/helpers.go` - Helper Functions
- formatBytes()
- formatDuration()
- Path completion helpers
- **Size:** ~150 lines

## Migration Steps

### Phase 1: Setup Structure
1. ✅ Create `screens/` directory
2. ✅ Create `shared/` directory
3. ✅ Create placeholder files

### Phase 2: Extract Shared Code
4. Create `shared/messages.go` with all message types
5. Create `shared/styles.go` with all lipgloss styles
6. Create `shared/helpers.go` with helper functions

### Phase 3: Extract Screens (one at a time)
7. Create `screens/1_input.go` - extract input logic
8. Create `screens/2_analysis.go` - extract analysis logic
9. Create `screens/3_sync.go` - extract sync logic
10. Create `screens/4_summary.go` - extract summary logic

### Phase 4: Create Router
11. Create `app.go` - top-level router

### Phase 5: Integration
12. Update `cmd/copy-files/main.go` to use AppModel
13. Run tests, verify everything works
14. Delete old files (model.go, update.go, view.go)

### Phase 6: Cleanup
15. Remove prototype files (PROTOTYPE_*.go.example)
16. Update documentation

## Benefits of This Structure

### 1. Self-Documenting
- Directory names explain purpose (`screens/`, `shared/`)
- File numbers show flow order (1→2→3→4)
- File names describe responsibility

### 2. Easy Navigation
- Want to change input handling? → `screens/1_input.go`
- Want to change sync display? → `screens/3_sync.go`
- Want to add a message type? → `shared/messages.go`

### 3. Clear Dependencies
- `app.go` depends on `screens/*` and `shared/*`
- `screens/*` depend on `shared/*`
- `screens/*` are independent of each other

### 4. Scalability
- Easy to add new screens (e.g., `5_settings.go`)
- Easy to add shared utilities
- Clear where new code should go

### 5. Testing
- Can test each screen independently
- Can test shared helpers in isolation
- Can test app routing separately

## Estimated Effort

- **Phase 1-2 (Setup + Shared):** 30 minutes
- **Phase 3 (Extract Screens):** 2 hours
- **Phase 4-5 (Router + Integration):** 1 hour
- **Phase 6 (Cleanup):** 15 minutes

**Total:** ~3.5 hours

## Risk Assessment

- **Risk Level:** Low
- **Mitigation:** Incremental approach, tests verify correctness
- **Rollback:** Keep old files until fully verified

## Next Steps

Ready to proceed? I'll start with:
1. Create directory structure
2. Extract shared code
3. Build screens one by one
4. Create router
5. Integrate and test

