# Shared Directory

This directory contains code that is shared across multiple screens and the top-level AppModel. Code should only be placed here if it's used by **2 or more** components.

## Files

### messages.go - Message Type Definitions

**Purpose:** Define all message types used for communication between screens and within screens.

**Contains:**
1. **Transition Messages** - Trigger screen changes (handled by AppModel)
   - `TransitionToAnalysisMsg`
   - `TransitionToSyncMsg`
   - `TransitionToSummaryMsg`

2. **Internal Messages** - Used within screens (handled by individual screens)
   - `EngineInitializedMsg`
   - `AnalysisCompleteMsg`
   - `SyncCompleteMsg`
   - `ErrorMsg`
   - `tickMsg` (internal timer)

**Key principle:** All message types should be defined here, even if only used by one screen. This provides a single source of truth for the application's message protocol.

**Naming convention:**
- Transition messages: `TransitionTo<Screen>Msg`
- Internal messages: `<Event>Msg` (e.g., `EngineInitializedMsg`)

---

### styles.go - Lipgloss Style Definitions

**Purpose:** Define all Lipgloss styles used for rendering the TUI.

**Contains:**
- Color definitions
- Text styles (bold, italic, etc.)
- Box styles (borders, padding, margins)
- Layout styles

**Key principle:** Centralize all styling to ensure visual consistency across screens. If a style is used in only one place, it can still live here for consistency.

**Organization:**
```go
var (
    // Colors
    primaryColor   = lipgloss.Color("#7D56F4")
    successColor   = lipgloss.Color("#04B575")
    errorColor     = lipgloss.Color("#FF0000")
    
    // Text styles
    titleStyle     = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
    errorStyle     = lipgloss.NewStyle().Foreground(errorColor)
    
    // Box styles
    boxStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
)
```

**Usage in screens:**
```go
import "github.com/joe/copy-files/internal/tui/shared"

func (s InputScreen) View() string {
    return shared.TitleStyle.Render("Enter Paths")
}
```

---

### helpers.go - Shared Helper Functions

**Purpose:** Utility functions used by multiple screens.

**Contains:**
- `formatBytes(bytes int64) string` - Format byte counts (e.g., "1.5 MB")
- `formatDuration(d time.Duration) string` - Format durations (e.g., "2m 30s")
- `formatRate(bytesPerSec float64) string` - Format transfer rates (e.g., "5.2 MB/s")
- Path completion helpers (if used by multiple screens)
- Other formatting/utility functions

**Key principle:** Only add functions here if they're used by 2+ screens. Screen-specific helpers should stay in the screen file.

**Example:**
```go
// Used by SyncScreen and SummaryScreen
func FormatBytes(bytes int64) string {
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
```

## Dependency Rules

### ✅ Allowed Dependencies

**Screens can depend on shared:**
```go
// In screens/1_input.go
import "github.com/joe/copy-files/internal/tui/shared"

func (s InputScreen) View() string {
    return shared.TitleStyle.Render("Input")
}
```

**AppModel can depend on shared:**
```go
// In app.go
import "github.com/joe/copy-files/internal/tui/shared"

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case shared.TransitionToAnalysisMsg:
        // ...
    }
}
```

### ❌ Forbidden Dependencies

**Shared CANNOT depend on screens:**
```go
// ❌ WRONG - shared/helpers.go cannot import screens
import "github.com/joe/copy-files/internal/tui/screens"
```

**Shared CANNOT depend on app:**
```go
// ❌ WRONG - shared/messages.go cannot import app
import "github.com/joe/copy-files/internal/tui"
```

**Screens CANNOT depend on other screens:**
```go
// ❌ WRONG - screens/1_input.go cannot import screens/2_analysis.go
import "github.com/joe/copy-files/internal/tui/screens"
```

**Screens CANNOT depend on app:**
```go
// ❌ WRONG - screens/1_input.go cannot import app
import "github.com/joe/copy-files/internal/tui"
```

## Dependency Graph

```
┌─────────────────────────────────────────────┐
│                   app.go                    │
│              (Top-level router)             │
└─────────────────────────────────────────────┘
         │                           │
         ▼                           ▼
┌─────────────────┐         ┌─────────────────┐
│    screens/     │         │     shared/     │
│  (all screens)  │────────▶│  (utilities)    │
└─────────────────┘         └─────────────────┘
```

**Key principle:** Dependencies flow downward and inward. Shared code is at the bottom and cannot depend on anything above it.

## When to Add Code Here

**Add to shared/ when:**
- ✅ Code is used by 2+ screens
- ✅ Code is used by AppModel and screens
- ✅ Code defines the communication protocol (messages)
- ✅ Code defines visual styling (consistency)

**Keep in screen file when:**
- ❌ Code is only used by one screen
- ❌ Code is screen-specific logic
- ❌ Code is a helper for one screen's rendering

**Example decision:**
- `formatBytes()` → **shared/** (used by SyncScreen and SummaryScreen)
- `formatCompletionList()` → **screens/1_input.go** (only used by InputScreen)

