# TUI Package - Layered Architecture

## Directory Structure

```
internal/tui/
│
├── app.go                    # 🎯 Top-level router - manages screen transitions
│
├── screens/                  # 📱 Individual screens (numbered by flow order)
│   ├── 1_input.go           # ⌨️  Input Screen - gather source/dest paths
│   ├── 2_analysis.go        # 🔍 Analysis Screen - initialize & analyze
│   ├── 3_sync.go            # 🔄 Sync Screen - run sync & show progress
│   └── 4_summary.go         # ✅ Summary Screen - show final results
│
├── shared/                   # 🔧 Shared utilities used by multiple screens
│   ├── messages.go          # 📨 Message types (transitions & internal)
│   ├── styles.go            # 🎨 Lipgloss styles
│   └── helpers.go           # 🛠️  Helper functions (formatBytes, etc.)
│
└── docs/                     # 📚 Documentation
    ├── ARCHITECTURE.md
    ├── COMPLEXITY_COMPARISON.md
    └── REFACTORING_PLAN.md
```

## Application Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                          AppModel                               │
│                      (Top-level Router)                         │
│                                                                 │
│  Responsibilities:                                              │
│  • Route messages to active screen                              │
│  • Handle screen transitions                                    │
│  • Manage shared state (engine, status, config)                │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
        ┌────────────────────────────────────────────┐
        │         Which screen is active?            │
        └────────────────────────────────────────────┘
                 │        │        │        │
        ┌────────┘        │        │        └────────┐
        ▼                 ▼        ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ InputScreen  │  │AnalysisScreen│  │  SyncScreen  │  │SummaryScreen │
│              │  │              │  │              │  │              │
│ Gather paths │→ │ Initialize & │→ │ Run sync &   │→ │ Show final   │
│ from user    │  │ analyze      │  │ show progress│  │ results      │
└──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘
       │                 │                 │                 │
       │                 │                 │                 │
       ▼                 ▼                 ▼                 ▼
  Transition        Transition        Transition         (Terminal
  ToAnalysis        ToSync            ToSummary           State)
```

## Screen Responsibilities

### 1️⃣ InputScreen (`screens/1_input.go`)
- **Purpose:** Gather source and destination paths from user
- **UI Elements:** Text inputs, path completions
- **Exit Condition:** User presses Enter with valid paths
- **Next Screen:** AnalysisScreen

### 2️⃣ AnalysisScreen (`screens/2_analysis.go`)
- **Purpose:** Initialize sync engine and analyze files
- **UI Elements:** Spinner, progress messages
- **Exit Condition:** Analysis completes or errors
- **Next Screen:** SyncScreen (success) or SummaryScreen (error)

### 3️⃣ SyncScreen (`screens/3_sync.go`)
- **Purpose:** Run file sync and show detailed progress
- **UI Elements:** Progress bars, file lists, statistics, spinner
- **Exit Condition:** Sync completes, errors, or user cancels
- **Next Screen:** SummaryScreen

### 4️⃣ SummaryScreen (`screens/4_summary.go`)
- **Purpose:** Show final results and statistics
- **UI Elements:** Summary stats, error list, completion message
- **Exit Condition:** User presses Enter or Ctrl+C to quit
- **Next Screen:** None (application exits)

## Message Flow

### Transition Messages (between screens)
```
TransitionToAnalysisMsg  →  InputScreen → AnalysisScreen
TransitionToSyncMsg      →  AnalysisScreen → SyncScreen
TransitionToSummaryMsg   →  SyncScreen → SummaryScreen
```

### Internal Messages (within screens)
```
EngineInitializedMsg  →  Used by AnalysisScreen
AnalysisCompleteMsg   →  Used by AnalysisScreen
SyncCompleteMsg       →  Used by SyncScreen
ErrorMsg              →  Used by AnalysisScreen, SyncScreen
```

## Complexity Metrics

| Component | Cyclomatic Complexity | Status |
|-----------|----------------------|--------|
| AppModel.Update() | 7 | ✅ Under threshold (10) |
| AppModel.View() | 6 | ✅ Under threshold (10) |
| InputScreen.Update() | 4 | ✅ Under threshold (10) |
| AnalysisScreen.Update() | 5 | ✅ Under threshold (10) |
| SyncScreen.Update() | 7 | ✅ Under threshold (10) |
| SummaryScreen.Update() | 2 | ✅ Under threshold (10) |

**All functions are under the cyclop threshold of 10!** 🎉

## Usage

```go
// In cmd/glowsync/main.go
model := tui.NewAppModel(cfg)
program := tea.NewProgram(model)
if _, err := program.Run(); err != nil {
    log.Fatal(err)
}
```

## Testing

Each screen can be tested independently:

```go
// Test InputScreen
screen := NewInputScreen(cfg)
screen, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
// Assert transition message is emitted

// Test AnalysisScreen
screen := NewAnalysisScreen(cfg)
screen, cmd := screen.Update(EngineInitializedMsg{engine})
// Assert analysis starts

// etc.
```

## Benefits

✅ **Low Complexity** - All functions under cyclop threshold  
✅ **Self-Documenting** - Directory structure shows architecture  
✅ **Easy Navigation** - Numbered files show flow order  
✅ **Testable** - Each screen can be tested independently  
✅ **Maintainable** - Changes to one screen don't affect others  
✅ **Scalable** - Easy to add new screens or features  

