# Complexity Comparison: Monolithic vs Layered Architecture

## Current Monolithic Architecture

### Model.Update() - Complexity: 11
```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Routes ALL messages for ALL states
    switch msg := msg.(type) {
    case InitializeEngineMsg:        // 1
    case EngineInitializedMsg:       // 2
    case tea.KeyMsg:                 // 3 (+ routing logic inside)
    case tea.WindowSizeMsg:          // 4 (+ routing logic inside)
    case tickMsg:                    // 5
    case AnalysisStartedMsg:         // 6
    case AnalysisCompleteMsg:        // 7
    case SyncCompleteMsg:            // 8
    case ErrorMsg:                   // 9
    case spinner.TickMsg:            // 10
    }
    return m, nil                    // +1 base
}
```

### Model.View() - Complexity: 9
```go
func (m Model) View() string {
    switch m.state {
    case "quitting":      // 1
    case "input":         // 2
    case "initializing":  // 3
    case "analyzing":     // 4
    case "syncing":       // 5
    case "cancelling":    // 6
    case "complete":      // 7
    case "cancelled":     // 8
    case "error":         // 9
    }
    return ""
}
```

**Total Complexity: 20**

---

## Proposed Layered Architecture

### AppModel.Update() - Complexity: 7
```go
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:          // 1 (global)
    case TransitionToAnalysisMsg:    // 2
    case TransitionToSyncMsg:        // 3
    case TransitionToSummaryMsg:     // 4
    }
    
    // Route to active screen
    switch m.screen {
    case ScreenInput:     // 5
    case ScreenAnalysis:  // 6
    case ScreenSync:      // 7
    case ScreenSummary:   // 8
    case ScreenQuitting:  // 9
    }
    return m, cmd
}
```

### AppModel.View() - Complexity: 6
```go
func (m AppModel) View() string {
    switch m.screen {
    case ScreenInput:     // 1
    case ScreenAnalysis:  // 2
    case ScreenSync:      // 3
    case ScreenSummary:   // 4
    case ScreenQuitting:  // 5
    }
    return ""             // +1 base
}
```

### InputScreen.Update() - Complexity: 4
```go
func (s InputScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:  // 1
    case tea.KeyMsg:         // 2 (handles all key logic internally)
    }
    // Update focused input  // +1
    return s, cmd
}
```

### AnalysisScreen.Update() - Complexity: 5
```go
func (s AnalysisScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case EngineInitializedMsg:  // 1
    case AnalysisCompleteMsg:   // 2
    case ErrorMsg:              // 3
    case spinner.TickMsg:       // 4
    }
    return s, nil               // +1 base
}
```

### SyncScreen.Update() - Complexity: 7
```go
func (s SyncScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:         // 1 (cancel/quit)
    case tea.WindowSizeMsg:  // 2
    case tickMsg:            // 3
    case SyncCompleteMsg:    // 4
    case ErrorMsg:           // 5
    case spinner.TickMsg:    // 6
    }
    return s, nil            // +1 base
}
```

### SummaryScreen.Update() - Complexity: 2
```go
func (s SummaryScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:  // 1 (quit only)
    }
    return s, nil     // +1 base
}
```

**Total Complexity: 31** (but distributed across 6 functions instead of 2)

---

## Key Improvements

### 1. Maximum Function Complexity
- **Before:** 11 (Update), 9 (View) - both OVER threshold of 10
- **After:** 7 (AppModel.Update), 6 (AppModel.View) - both UNDER threshold

### 2. Cognitive Load
- **Before:** Must understand all states and messages in one function
- **After:** Each screen only understands its own messages

### 3. Testability
- **Before:** Must mock entire application state to test one feature
- **After:** Can test each screen independently

### 4. Maintainability
- **Before:** Changes to input handling affect sync handling (same file)
- **After:** Changes to InputScreen don't affect SyncScreen

### 5. Lines of Code per File
- **Before:** model.go (127 lines), update.go (539 lines), view.go (1156 lines)
- **After:** Each screen ~150-300 lines, app.go ~100 lines

---

## Migration Path

1. Create new files (app.go, input.go, analysis.go, sync.go, summary.go, messages.go)
2. Extract InputScreen logic from current model
3. Extract AnalysisScreen logic from current model
4. Extract SyncScreen logic from current model
5. Extract SummaryScreen logic from current model
6. Create AppModel router
7. Update cmd/glowsync/main.go to use AppModel
8. Delete old model.go, update.go, view.go
9. Run tests and verify

Estimated effort: 2-3 hours
Risk: Low (can be done incrementally, tests will catch issues)

