# Screens Directory

This directory contains the individual screen implementations for the TUI application. Each screen is a self-contained Bubble Tea model that handles a specific phase of the application flow.

## Screen Flow

Screens are numbered to indicate the expected flow order:

```
1_input.go → 2_analysis.go → 3_sync.go → 4_summary.go
```

However, the flow is not strictly linear:
- **2_analysis.go** can skip to **4_summary.go** on error
- **3_sync.go** can transition to **4_summary.go** on completion, error, or cancellation

## Screen Interface

Each screen implements the Bubble Tea `tea.Model` interface:

```go
type Model interface {
    Init() tea.Cmd
    Update(tea.Msg) (tea.Model, tea.Cmd)
    View() string
}
```

## Screen Responsibilities

### 1_input.go - InputScreen
**Purpose:** Gather source and destination paths from the user

**Owns:**
- Text input fields (source, destination)
- Path completion logic
- Input validation

**Transitions to:** AnalysisScreen (via `TransitionToAnalysisMsg`)

**Key principle:** This screen should ONLY handle user input. No engine initialization, no file operations.

---

### 2_analysis.go - AnalysisScreen
**Purpose:** Initialize the sync engine and analyze files

**Owns:**
- Engine initialization
- Analysis progress display
- Spinner for visual feedback

**Transitions to:** 
- SyncScreen (via `TransitionToSyncMsg`) on success
- SummaryScreen (via `TransitionToSummaryMsg`) on error

**Key principle:** This screen bridges input and sync. It creates the engine and runs analysis, but doesn't perform any actual file operations.

---

### 3_sync.go - SyncScreen
**Purpose:** Run the file sync operation and display detailed progress

**Owns:**
- Sync execution
- Unified progress bar (file count as primary metric)
- Per-file progress bars in file list
- File list display
- Statistics display (rate, ETA, workers)
- Cancellation handling

**Transitions to:** SummaryScreen (via `TransitionToSummaryMsg`)

**Key principle:** This is the most complex screen. It handles real-time updates, user cancellation, and detailed progress display. The unified progress bar shows file count progress with bytes/rate/ETA as secondary metrics, maximizing vertical space for the file list. Keep the Update() method focused on message routing; extract complex rendering logic into helper methods.

---

### 4_summary.go - SummaryScreen
**Purpose:** Display final results (success, error, or cancellation)

**Owns:**
- Final statistics display
- Error message display (if applicable)
- Completion message

**Transitions to:** None (terminal state - user quits the application)

**Key principle:** This is a read-only screen. It displays information but doesn't perform any operations. The only user action is to quit.

## Communication Between Screens

Screens do NOT communicate directly with each other. Instead:

1. **Screens emit transition messages** (defined in `shared/messages.go`)
2. **AppModel catches these messages** and performs the transition
3. **AppModel creates the next screen** with necessary data

Example flow:
```
InputScreen validates paths
    ↓
InputScreen returns TransitionToAnalysisMsg{source, dest}
    ↓
AppModel.Update() catches TransitionToAnalysisMsg
    ↓
AppModel creates AnalysisScreen with source/dest
    ↓
AppModel switches to AnalysisScreen
```

## Shared State

Screens should be **stateless** with respect to other screens. They receive all necessary data through:

1. **Constructor parameters** (e.g., `NewAnalysisScreen(config)`)
2. **Transition messages** (e.g., `TransitionToSyncMsg{Engine}`)

Screens should NOT:
- Access global variables
- Share mutable state
- Depend on other screens directly

## Testing Screens

Each screen can be tested independently:

```go
// Example: Test InputScreen transition
screen := NewInputScreen(cfg)
screen.sourceInput.SetValue("/valid/source")
screen.destInput.SetValue("/valid/dest")

model, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyEnter})

// Assert that TransitionToAnalysisMsg is emitted
msg := cmd()
assert.IsType(t, TransitionToAnalysisMsg{}, msg)
```

## Adding a New Screen

To add a new screen between existing screens:

1. **Create the file** with appropriate number (e.g., `2.5_preparation.go` or renumber existing)
2. **Implement tea.Model interface** (Init, Update, View)
3. **Define transition messages** in `shared/messages.go`
4. **Update AppModel** in `app.go` to handle the new screen
5. **Update this README** to document the new screen

## Complexity Guidelines

Each screen's `Update()` method should have cyclomatic complexity < 10:

- Keep switch statements simple (single-line case bodies)
- Extract multi-line logic into helper methods
- Use early returns to reduce nesting
- Delegate complex rendering to helper methods

Current complexity targets:
- InputScreen.Update(): ~4
- AnalysisScreen.Update(): ~5
- SyncScreen.Update(): ~7
- SummaryScreen.Update(): ~2

