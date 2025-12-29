package screens

import (
	"testing"
	"time"

	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/syncengine"
)

func TestAnalysisHandleTick(t *testing.T) {
	t.Parallel()
	gomega := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := &AnalysisScreen{
		config:     cfg,
		lastUpdate: time.Now().Add(-1 * time.Second),
	}

	// Initialize with engine first
	engine := syncengine.NewEngine("/source", "/dest")
	screen.engine = engine
	screen.status = engine.GetStatus()

	// Send tick message
	updatedModel, cmd := screen.handleTick()

	gomega.Expect(updatedModel).ShouldNot(BeNil())
	gomega.Expect(cmd).ShouldNot(BeNil())
}

func TestAnalysisHandleTickWithoutEngine(t *testing.T) {
	t.Parallel()
	gomega := NewWithT(t)

	cfg := &config.Config{
		SourcePath: "/source",
		DestPath:   "/dest",
	}
	screen := &AnalysisScreen{
		config:     cfg,
		lastUpdate: time.Now(),
	}

	// Send tick message without engine
	updatedModel, cmd := screen.handleTick()

	gomega.Expect(updatedModel).ShouldNot(BeNil())
	gomega.Expect(cmd).ShouldNot(BeNil())
}

func TestSyncHandleTick(t *testing.T) {
	t.Parallel()
	gomega := NewWithT(t)

	engine := syncengine.NewEngine("/source", "/dest")
	screen := &SyncScreen{
		engine:     engine,
		lastUpdate: time.Now().Add(-1 * time.Second),
		status:     engine.GetStatus(),
	}

	// Send tick message
	updatedModel, cmd := screen.handleTick()

	gomega.Expect(updatedModel).ShouldNot(BeNil())
	gomega.Expect(cmd).ShouldNot(BeNil())
}

func TestSyncHandleTickWithoutEngine(t *testing.T) {
	t.Parallel()
	gomega := NewWithT(t)

	screen := &SyncScreen{
		lastUpdate: time.Now(),
	}

	// Send tick message without engine
	updatedModel, cmd := screen.handleTick()

	gomega.Expect(updatedModel).ShouldNot(BeNil())
	gomega.Expect(cmd).ShouldNot(BeNil())
}
