package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/joe/copy-files/internal/syncengine"
)

func TestSyncScreen_DebugLogging(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a temporary log file
	logFile := filepath.Join(t.TempDir(), "debug.log")

	// Create test engine with verbose mode enabled
	engine := mustNewEngine(t, "/tmp/test-source", "/tmp/test-dest")
	engine.Verbose = true
	err := engine.EnableFileLogging(logFile)
	g.Expect(err).ShouldNot(HaveOccurred())
	defer engine.CloseLog()

	// Create sync screen with engine and status
	screen := &SyncScreen{
		engine: engine,
		status: &syncengine.Status{
			ActiveWorkers: 5,
			CurrentFiles:  []string{"file1.txt", "file2.txt", "file3.txt"},
			FilesToSync: []*syncengine.FileToSync{
				{RelativePath: "file1.txt", Status: "copying", Size: 1024, Transferred: 512},
				{RelativePath: "file2.txt", Status: "opening", Size: 2048, Transferred: 0},
				{RelativePath: "file3.txt", Status: "finalizing", Size: 512, Transferred: 512},
				{RelativePath: "file4.txt", Status: "pending", Size: 1024, Transferred: 0},
			},
		},
		height: 50,
	}

	// Trigger render which should log debug info
	_ = screen.View()

	// Close the log to flush
	engine.CloseLog()

	// Read the log file
	logContent, err := os.ReadFile(logFile)
	g.Expect(err).ShouldNot(HaveOccurred())

	logStr := string(logContent)

	// Verify debug logging was written
	g.Expect(logStr).Should(ContainSubstring("[DISPLAY]"))
	g.Expect(logStr).Should(ContainSubstring("Workers: 5"))
	g.Expect(logStr).Should(ContainSubstring("Files to display: 4"))
	g.Expect(logStr).Should(ContainSubstring("copying:1"))
	g.Expect(logStr).Should(ContainSubstring("opening:1"))
	g.Expect(logStr).Should(ContainSubstring("finalizing:1"))
	g.Expect(logStr).Should(ContainSubstring("other:1")) // pending file
	g.Expect(logStr).Should(ContainSubstring("CurrentFiles: 3"))
}

func TestSyncScreen_DebugLogging_NoLogWhenNotVerbose(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a temporary log file
	logFile := filepath.Join(t.TempDir(), "debug.log")

	// Create test engine with verbose mode DISABLED
	engine := mustNewEngine(t, "/tmp/test-source", "/tmp/test-dest")
	engine.Verbose = false
	err := engine.EnableFileLogging(logFile)
	g.Expect(err).ShouldNot(HaveOccurred())
	defer engine.CloseLog()

	// Create sync screen with engine and status
	screen := &SyncScreen{
		engine: engine,
		status: &syncengine.Status{
			ActiveWorkers: 3,
			FilesToSync: []*syncengine.FileToSync{
				{RelativePath: "file1.txt", Status: "copying", Size: 1024, Transferred: 512},
			},
		},
		height: 50,
	}

	// Trigger render
	_ = screen.View()

	// Close the log to flush
	engine.CloseLog()

	// Read the log file
	logContent, err := os.ReadFile(logFile)
	if err != nil {
		// Log file might not exist if nothing was logged, which is fine
		if !strings.Contains(err.Error(), "no such file") {
			g.Expect(err).ShouldNot(HaveOccurred())
		}
		return
	}

	logStr := string(logContent)

	// Verify NO debug logging was written when verbose is disabled
	g.Expect(logStr).ShouldNot(ContainSubstring("[DISPLAY]"))
}
