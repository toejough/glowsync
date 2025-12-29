package screens

import (
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/syncengine"
)

func TestCalculateMaxFilesToShow(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		height: 50,
	}

	maxFiles := screen.calculateMaxFilesToShow()
	g.Expect(maxFiles).Should(BeNumerically(">=", 1))
}

func TestGetBottleneckInfo(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			AdaptiveMode: true,
		},
	}

	// Test source bottleneck
	screen.status.Bottleneck = "source"
	result := screen.getBottleneckInfo()
	g.Expect(result).Should(ContainSubstring("source"))

	// Test destination bottleneck
	screen.status.Bottleneck = "destination"
	result = screen.getBottleneckInfo()
	g.Expect(result).Should(ContainSubstring("dest"))

	// Test balanced
	screen.status.Bottleneck = "balanced"
	result = screen.getBottleneckInfo()
	g.Expect(result).Should(ContainSubstring("balanced"))

	// Test no adaptive mode
	screen.status.AdaptiveMode = false
	result = screen.getBottleneckInfo()
	g.Expect(result).Should(BeEmpty())
}

func TestGetMaxPathWidth(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		width: 100,
	}

	width := screen.getMaxPathWidth()
	g.Expect(width).Should(BeNumerically(">", 0))
}

func TestTruncatePath(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{}

	// Test short path (no truncation)
	result := screen.truncatePath("/short/path.txt", 100)
	g.Expect(result).Should(Equal("/short/path.txt"))

	// Test long path (truncation)
	longPath := "/very/long/path/to/some/file/that/needs/truncation/file.txt"
	result = screen.truncatePath(longPath, 20)
	g.Expect(result).Should(ContainSubstring("..."))
	g.Expect(len(result)).Should(BeNumerically("<=", 20))
}

func TestRenderOverallProgress(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			TotalBytesInSource: 1024 * 1024,
			AlreadySyncedBytes: 512 * 1024,
			TransferredBytes:   256 * 1024,
			TotalFilesInSource: 100,
			AlreadySyncedFiles: 50,
			ProcessedFiles:     25,
		},
	}

	var builder strings.Builder
	screen.renderOverallProgress(&builder)
	result := builder.String()
	g.Expect(result).Should(ContainSubstring("Overall Progress"))
}

func TestRenderSessionProgress(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			TotalBytes:       1024 * 1024,
			TransferredBytes: 512 * 1024,
			TotalFiles:       100,
			ProcessedFiles:   50,
			FailedFiles:      5,
		},
	}

	var builder strings.Builder
	screen.renderSessionProgress(&builder)
	result := builder.String()
	g.Expect(result).Should(ContainSubstring("This Session"))
	g.Expect(result).Should(ContainSubstring("failed"))
}

func TestRenderStatistics(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			StartTime:        time.Now().Add(-10 * time.Second),
			TransferredBytes: 1024 * 1024,
			TotalBytes:       2 * 1024 * 1024,
			ProcessedFiles:   50,
			TotalFiles:       100,
			ActiveWorkers:    4,
		},
	}

	var builder strings.Builder
	screen.renderStatistics(&builder)
	result := builder.String()
	g.Expect(result).Should(ContainSubstring("Workers"))
	g.Expect(result).Should(ContainSubstring("Rate"))
}

func TestRenderFileList(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			CurrentFiles: []string{"file1.txt", "file2.txt"},
			FilesToSync: []*syncengine.FileToSync{
				{RelativePath: "file1.txt", Status: "copying", Size: 1024, Transferred: 512},
			},
		},
		height: 50,
	}

	var builder strings.Builder
	screen.renderFileList(&builder)
	result := builder.String()
	g.Expect(result).ShouldNot(BeEmpty())
}

func TestRenderCurrentlyCopying(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			CurrentFiles: []string{"file1.txt"},
			FilesToSync: []*syncengine.FileToSync{
				{RelativePath: "file1.txt", Status: "copying", Size: 1024, Transferred: 512},
			},
		},
	}

	var builder strings.Builder
	screen.renderCurrentlyCopying(&builder, 5)
	result := builder.String()
	g.Expect(result).Should(ContainSubstring("Currently Copying"))
}

func TestRenderRecentFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	screen := &SyncScreen{
		status: &syncengine.Status{
			FilesToSync: []*syncengine.FileToSync{
				{RelativePath: "file1.txt", Status: "complete"},
				{RelativePath: "file2.txt", Status: "complete"},
			},
		},
	}

	var builder strings.Builder
	screen.renderRecentFiles(&builder, 5)
	result := builder.String()
	g.Expect(result).Should(ContainSubstring("Recent Files"))
}

func TestRenderSyncingErrors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	testErr := fmt.Errorf("test error")

	screen := &SyncScreen{
		status: &syncengine.Status{
			Errors: []syncengine.FileError{
				{FilePath: "/path/to/file1.txt", Error: testErr},
			},
		},
		width: 100,
	}

	var builder strings.Builder
	screen.renderSyncingErrors(&builder)
	result := builder.String()
	g.Expect(result).Should(ContainSubstring("Errors"))

	// Test with no errors
	screen.status.Errors = []syncengine.FileError{}

	builder.Reset()
	screen.renderSyncingErrors(&builder)
	result = builder.String()
	g.Expect(result).Should(BeEmpty())
}
