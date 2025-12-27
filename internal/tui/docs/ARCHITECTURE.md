# TUI Architecture Redesign

## Problem
The current monolithic TUI model has high cyclomatic complexity because it handles all application states in a single Update/View function. This makes the code hard to maintain and test.

## Solution: Layered Sub-Model Architecture

### Top-Level Router Model
Manages screen transitions and routes messages to active sub-models.

```go
type AppModel struct {
    config       *config.Config
    currentScreen Screen
    inputScreen   *InputScreen
    analysisScreen *AnalysisScreen
    syncScreen    *SyncScreen
    summaryScreen *SummaryScreen
    width, height int
}

type Screen int
const (
    ScreenInput Screen = iota
    ScreenAnalysis
    ScreenSync
    ScreenSummary
    ScreenQuitting
)
```

### Sub-Screens (each implements tea.Model)

#### 1. InputScreen
**Responsibility:** Gather source and destination paths
**States:** input
**Fields:**
- sourceInput, destInput (textinput.Model)
- focusIndex, completions, completionIndex, showCompletions
- config reference

**Messages:**
- TransitionToAnalysisMsg{source, dest string}

#### 2. AnalysisScreen
**Responsibility:** Initialize engine and run analysis
**States:** initializing, analyzing
**Fields:**
- engine reference
- status reference
- spinner
- config reference

**Messages:**
- EngineInitializedMsg
- AnalysisCompleteMsg
- TransitionToSyncMsg

#### 3. SyncScreen
**Responsibility:** Run sync and handle cancellation
**States:** syncing, cancelling
**Fields:**
- engine reference
- status reference
- overallProgress, fileProgress
- spinner
- cancelled bool
- lastUpdate time.Time

**Messages:**
- SyncCompleteMsg
- TransitionToSummaryMsg{finalState string}

#### 4. SummaryScreen
**Responsibility:** Show final results
**States:** complete, cancelled, error
**Fields:**
- status reference
- err error
- finalState string

**Messages:**
- (none - terminal state)

### Message Flow

```
User Input → AppModel.Update() → Route to active screen
                ↓
        Active Screen.Update()
                ↓
        Returns (tea.Model, tea.Cmd)
                ↓
        AppModel checks for transition messages
                ↓
        Switch to new screen if needed
```

### Transition Messages

```go
// Sent by InputScreen when paths are validated
type TransitionToAnalysisMsg struct {
    SourcePath string
    DestPath   string
}

// Sent by AnalysisScreen when analysis completes
type TransitionToSyncMsg struct {
    Engine *syncengine.Engine
}

// Sent by SyncScreen when sync completes/fails/cancelled
type TransitionToSummaryMsg struct {
    FinalState string // "complete", "cancelled", "error"
    Err        error  // only if error
}
```

### Benefits

1. **Reduced Complexity:** Each screen has ~3-5 message types instead of 10+
2. **Clear Separation:** Each screen owns its own state and logic
3. **Easier Testing:** Can test each screen independently
4. **Better Maintainability:** Changes to one screen don't affect others
5. **Reusability:** Screens could be reused in other contexts

### Implementation Plan

1. Create screen interfaces and base types
2. Extract InputScreen from current model
3. Extract AnalysisScreen from current model
4. Extract SyncScreen from current model
5. Extract SummaryScreen from current model
6. Create AppModel router
7. Update main to use AppModel
8. Remove old monolithic model
9. Run tests and verify

### File Structure

```
internal/tui/
├── ARCHITECTURE.md (this file)
├── app.go          (AppModel - top-level router)
├── input.go        (InputScreen)
├── analysis.go     (AnalysisScreen)
├── sync.go         (SyncScreen)
├── summary.go      (SummaryScreen)
├── messages.go     (Shared message types)
├── styles.go       (Shared styles)
└── helpers.go      (Shared helper functions)
```

### Complexity Reduction Estimate

**Current:**
- Update(): 11 complexity (10 message types)
- View(): 9 complexity (9 states)

**After refactoring:**
- AppModel.Update(): ~5 complexity (route + transitions)
- InputScreen.Update(): ~4 complexity (KeyMsg, WindowSizeMsg, textinput updates)
- AnalysisScreen.Update(): ~4 complexity (InitMsg, EngineInitializedMsg, AnalysisCompleteMsg, spinner)
- SyncScreen.Update(): ~6 complexity (KeyMsg, WindowSizeMsg, tickMsg, SyncCompleteMsg, ErrorMsg, spinner)
- SummaryScreen.Update(): ~2 complexity (KeyMsg for quit)

All functions would be well under the cyclop threshold of 10.

