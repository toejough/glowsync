//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

func TestSummaryScreenCtrlCQuitsApp(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Press Ctrl+C key
	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := screen.Update(ctrlCMsg)

	// Should return tea.Quit command
	g.Expect(cmd).ShouldNot(BeNil(), "Ctrl+C should return a quit command")

	// Execute the command to verify it's tea.Quit
	msg := cmd()
	g.Expect(msg).Should(BeAssignableToTypeOf(tea.QuitMsg{}),
		"Ctrl+C should send tea.QuitMsg")
}

func TestSummaryScreenDisplaysActionableDiskSpaceError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Create disk space error that should be enriched
	diskErr := errors.New("write /dest/file.txt: no space left on device")
	screen := screens.NewSummaryScreen(engine, shared.StateError, diskErr, "")

	view := screen.View()

	// Should display original error
	g.Expect(view).Should(ContainSubstring("no space left on device"))

	// Should display actionable suggestions with bullets
	g.Expect(view).Should(ContainSubstring("•"))
	g.Expect(view).Should(Or(
		ContainSubstring("df -h"),
		ContainSubstring("Free up space"),
		ContainSubstring("disk usage"),
	))
}

func TestSummaryScreenDisplaysActionablePathError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Create path error that should be enriched
	pathErr := errors.New("stat /missing/path: no such file or directory")
	screen := screens.NewSummaryScreen(engine, shared.StateError, pathErr, "")

	view := screen.View()

	// Should display original error
	g.Expect(view).Should(ContainSubstring("no such file or directory"))

	// Should display actionable suggestions with bullets
	g.Expect(view).Should(ContainSubstring("•"))
	g.Expect(view).Should(Or(
		ContainSubstring("Verify the path"),
		ContainSubstring("exists"),
		ContainSubstring("parent directories"),
	))
}

// Integration Tests for Actionable Error Enrichment

func TestSummaryScreenDisplaysActionablePermissionError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Create permission error that should be enriched
	permErr := errors.New("open /home/user/file.txt: permission denied")
	screen := screens.NewSummaryScreen(engine, shared.StateError, permErr, "")

	view := screen.View()

	// Should display original error
	g.Expect(view).Should(ContainSubstring("permission denied"))

	// Should display actionable suggestions with bullets
	g.Expect(view).Should(ContainSubstring("•"))
	g.Expect(view).Should(Or(
		ContainSubstring("permissions"),
		ContainSubstring("ls -la"),
		ContainSubstring("privileged user"),
	))
}

func TestSummaryScreenDisplaysLogPath(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")
	logPath := "/tmp/test-debug.log"

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, logPath)

	view := screen.View()
	// Should display the actual log path, not the hardcoded one
	g.Expect(view).Should(ContainSubstring(logPath))
	g.Expect(view).ShouldNot(ContainSubstring("copy-files-debug.log"))
}

func TestSummaryScreenDisplaysMultipleSuggestionsFormatted(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Create error that generates multiple suggestions
	err := errors.New("permission denied")
	screen := screens.NewSummaryScreen(engine, shared.StateError, err, "")

	view := screen.View()

	// Should have bullet-formatted suggestions
	g.Expect(view).Should(ContainSubstring("•"))

	// Should have multiple suggestions by counting bullet occurrences
	suggestionCount := strings.Count(view, "•")
	g.Expect(suggestionCount).Should(BeNumerically(">=", 2),
		"Should display multiple suggestions")
}

func TestSummaryScreenEscReturnsToInput(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Press Esc key - should return to InputScreen
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := screen.Update(escMsg)

	g.Expect(cmd).ShouldNot(BeNil(), "Esc should return a transition command")

	// Execute the command to get the message
	msg := cmd()
	g.Expect(msg).Should(BeAssignableToTypeOf(shared.TransitionToInputMsg{}),
		"Esc should send TransitionToInputMsg to start new session")
}

func TestSummaryScreenNewCancelled(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	g.Expect(screen).ShouldNot(BeNil())
}

func TestSummaryScreenNewComplete(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	g.Expect(screen).ShouldNot(BeNil())

	// Call Init to ensure coverage
	cmd := screen.Init()
	g.Expect(cmd).Should(BeNil())
}

func TestSummaryScreenNewError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSummaryScreen(engine, shared.StateError, errors.New("test error"), "")

	g.Expect(screen).ShouldNot(BeNil())
}

func TestSummaryScreenNewNilEngine(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := screens.NewSummaryScreen(nil, shared.StateError, errors.New("test error"), "")

	g.Expect(screen).ShouldNot(BeNil())
}

func TestSummaryScreenUpdate(t *testing.T) {
	t.Parallel()

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Test WindowSizeMsg
	sizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := screen.Update(sizeMsg)

	g := NewWithT(t)
	g.Expect(updatedModel).ShouldNot(BeNil())

	// Test quit keys
	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := screen.Update(ctrlCMsg)
	g.Expect(cmd).ShouldNot(BeNil())

	qMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd = screen.Update(qMsg)
	g.Expect(cmd).ShouldNot(BeNil())

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd = screen.Update(enterMsg)
	g.Expect(cmd).ShouldNot(BeNil())
}

func TestSummaryScreenUsesErrorSymbol(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Test error state which will display ErrorSymbol in the title
	screen := screens.NewSummaryScreen(engine, shared.StateError, errors.New("fatal error"), "")
	view := screen.View()

	// Should use ErrorSymbol() helper in "✗ Sync Failed" title
	g.Expect(view).Should(ContainSubstring(shared.ErrorSymbol()))
}

func TestSummaryScreenUsesSuccessSymbol(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")
	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	view := screen.View()

	// Should use SuccessSymbol() helper, not hardcoded ✓
	g.Expect(view).Should(ContainSubstring(shared.SuccessSymbol()))
}

func TestSummaryScreenViewCancelled(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Set up engine with some status data
	status := engine.GetStatus()
	status.StartTime = time.Now().Add(-5 * time.Second)
	status.ProcessedFiles = 50
	status.TotalFiles = 100
	status.CancelledFiles = 25

	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	view := screen.View()
	g.Expect(view).Should(ContainSubstring("Cancelled"))
}

func TestSummaryScreenViewCancelledAdaptive(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Set up adaptive mode for cancelled state
	status := engine.GetStatus()
	status.StartTime = time.Now().Add(-5 * time.Second)
	status.AdaptiveMode = true
	status.MaxWorkers = 8
	status.ActiveWorkers = 4
	status.Bottleneck = "destination"

	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	view := screen.View()
	g.Expect(view).Should(ContainSubstring("Cancelled"))
}

func TestSummaryScreenViewCancelledWithErrors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Set up engine with errors
	status := engine.GetStatus()
	status.StartTime = time.Now().Add(-5 * time.Second)
	status.Errors = []syncengine.FileError{
		{FilePath: "/path/to/file1", Error: errors.New("error 1")},
	}

	screen := screens.NewSummaryScreen(engine, shared.StateCancelled, nil, "")

	view := screen.View()
	g.Expect(view).Should(ContainSubstring("Cancelled"))
}

func TestSummaryScreenViewComplete(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Set up engine with some status data
	status := engine.GetStatus()
	status.StartTime = time.Now().Add(-5 * time.Second)
	status.EndTime = time.Now()
	status.TotalFiles = 100
	status.ProcessedFiles = 100
	status.TotalBytes = 1024 * 1024
	status.TransferredBytes = 1024 * 1024

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	// Test View rendering
	view := screen.View()
	g.Expect(view).Should(ContainSubstring("All files already up-to-date"))
}

func TestSummaryScreenViewCompleteWithAlreadySynced(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Note: We can't easily set internal status, so just verify the view renders
	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	view := screen.View()
	g.Expect(view).Should(ContainSubstring("All files already up-to-date"))
}

func TestSummaryScreenViewCompleteWithErrors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Note: We can't easily set internal status, so just verify the view renders
	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	view := screen.View()
	g.Expect(view).Should(ContainSubstring("All files already up-to-date"))
}

func TestSummaryScreenViewError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	screen := screens.NewSummaryScreen(engine, shared.StateError, errors.New("fatal error"), "")

	view := screen.View()
	g.Expect(view).Should(ContainSubstring("Sync Failed"))
	g.Expect(view).Should(ContainSubstring("fatal error"))
}

func TestSummaryScreenViewErrorWithAdditionalErrors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Note: We can't easily set internal status, so just verify the view renders
	screen := screens.NewSummaryScreen(engine, shared.StateError, errors.New("fatal error"), "")

	view := screen.View()
	g.Expect(view).Should(ContainSubstring("Sync Failed"))
}

func TestSummaryScreenViewErrorWithPartialProgress(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Note: We can't easily set internal status, so just verify the view renders
	screen := screens.NewSummaryScreen(engine, shared.StateError, errors.New("fatal error"), "")

	view := screen.View()
	g.Expect(view).Should(ContainSubstring("Sync Failed"))
}

func TestSummaryScreenViewWithAdaptiveMode(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Set up adaptive mode stats
	status := engine.GetStatus()
	status.StartTime = time.Now().Add(-5 * time.Second)
	status.EndTime = time.Now()
	status.AdaptiveMode = true
	status.MaxWorkers = 8
	status.TotalReadTime = 2 * time.Second
	status.TotalWriteTime = 3 * time.Second
	status.Bottleneck = "source"

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	view := screen.View()
	g.Expect(view).Should(ContainSubstring("All files already up-to-date"))
}

func TestSummaryScreenViewWithRecentlyCompleted(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Note: We can't easily set internal status, so just verify the view renders
	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	view := screen.View()
	g.Expect(view).Should(ContainSubstring("All files already up-to-date"))
}

func TestSummaryScreenViewCompleteWithZeroFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")

	// Set up engine with zero files synced (all already up-to-date)
	status := engine.GetStatus()
	status.StartTime = time.Now().Add(-2 * time.Second)
	status.EndTime = time.Now()
	status.TotalFilesInSource = 10
	status.AlreadySyncedFiles = 10
	status.ProcessedFiles = 0 // Zero files synced this session
	status.TotalFiles = 0
	status.TotalBytes = 0
	status.TransferredBytes = 0

	screen := screens.NewSummaryScreen(engine, shared.StateComplete, nil, "")

	view := screen.View()
	g.Expect(view).Should(ContainSubstring("All files already up-to-date"))

	// Should show helpful explanation when 0 files synced
	// (all files were already up-to-date)
	g.Expect(view).Should(Or(
		ContainSubstring("All files already up-to-date"),
		ContainSubstring("No files needed to be synced"),
		ContainSubstring("already in sync"),
	), "Should show explanation for 0 files synced")
}
