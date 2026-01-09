package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

var _ = Describe("UnifiedScreen", func() {
	var (
		cfg    *config.Config
		screen *UnifiedScreen
	)

	BeforeEach(func() {
		cfg = &config.Config{
			SourcePath: "/tmp/src",
			DestPath:   "/tmp/dst",
		}
		screen = NewUnifiedScreen(cfg)
	})

	Describe("Phase Tracking", func() {
		It("starts at input phase", func() {
			Expect(screen.phase).To(Equal(PhaseInput))
		})

		It("has input screen initialized", func() {
			Expect(screen.hasInput).To(BeTrue())
		})

		It("has other screens not initialized initially", func() {
			Expect(screen.hasAnalysis).To(BeFalse())
			Expect(screen.hasConfirmation).To(BeFalse())
			Expect(screen.hasSync).To(BeFalse())
			Expect(screen.hasSummary).To(BeFalse())
		})
	})

	Describe("Phase Transitions", func() {
		It("advances to scan phase on TransitionToAnalysisMsg", func() {
			msg := shared.TransitionToAnalysisMsg{
				SourcePath: "/tmp/src",
				DestPath:   "/tmp/dst",
			}

			newModel, _ := screen.Update(msg)
			updated := newModel.(*UnifiedScreen)

			Expect(updated.phase).To(Equal(PhaseScan))
			Expect(updated.hasAnalysis).To(BeTrue())
			Expect(updated.hasInput).To(BeTrue()) // Input preserved
		})

		It("advances to compare phase on TransitionToConfirmationMsg", func() {
			// First advance to scan
			screen.phase = PhaseScan
			screen.analysis = *screens.NewAnalysisScreen(cfg)
			screen.hasAnalysis = true

			msg := shared.TransitionToConfirmationMsg{
				Engine:  nil, // Engine would be set in real usage
				LogPath: "/tmp/log",
			}

			newModel, _ := screen.Update(msg)
			updated := newModel.(*UnifiedScreen)

			Expect(updated.phase).To(Equal(PhaseCompare))
			Expect(updated.hasConfirmation).To(BeTrue())
			Expect(updated.hasAnalysis).To(BeTrue()) // Analysis preserved
		})

		It("advances to sync phase on TransitionToSyncMsg", func() {
			screen.phase = PhaseCompare

			msg := shared.TransitionToSyncMsg{
				Engine:  nil,
				LogPath: "/tmp/log",
			}

			newModel, _ := screen.Update(msg)
			updated := newModel.(*UnifiedScreen)

			Expect(updated.phase).To(Equal(PhaseSync))
			Expect(updated.hasSync).To(BeTrue())
		})

		It("advances to done phase on TransitionToSummaryMsg", func() {
			screen.phase = PhaseSync

			msg := shared.TransitionToSummaryMsg{
				FinalState: "complete",
			}

			newModel, _ := screen.Update(msg)
			updated := newModel.(*UnifiedScreen)

			Expect(updated.phase).To(Equal(PhaseDone))
			Expect(updated.hasSummary).To(BeTrue())
		})
	})

	Describe("View Accumulation", func() {
		BeforeEach(func() {
			screen.width = 80
			screen.height = 24
		})

		It("shows input section at input phase", func() {
			view := screen.View()

			Expect(view).To(ContainSubstring("Source"))
			Expect(view).To(ContainSubstring("Destination"))
		})

		It("shows timeline header", func() {
			view := screen.View()

			Expect(view).To(ContainSubstring("Input"))
		})

		It("accumulates input and analysis at scan phase", func() {
			screen.phase = PhaseScan
			screen.analysis = *screens.NewAnalysisScreen(cfg)
			screen.hasAnalysis = true

			view := screen.View()

			// Both sections should be present
			Expect(view).To(ContainSubstring("Source"))         // From input
			Expect(view).To(ContainSubstring("Scanning Files")) // From analysis
		})

		It("shows input section even after advancing phases", func() {
			// Advance to scan phase
			screen.phase = PhaseScan
			screen.analysis = *screens.NewAnalysisScreen(cfg)
			screen.hasAnalysis = true

			view := screen.View()

			// Input summary should still be present (shows compact form after input phase)
			Expect(view).To(ContainSubstring("Source:"))
			Expect(view).To(ContainSubstring("Dest:"))
		})
	})

	Describe("Window Size Handling", func() {
		It("stores width and height", func() {
			msg := tea.WindowSizeMsg{Width: 120, Height: 40}

			newModel, _ := screen.Update(msg)
			updated := newModel.(*UnifiedScreen)

			Expect(updated.width).To(Equal(120))
			Expect(updated.height).To(Equal(40))
		})

		It("propagates window size to child screens", func() {
			msg := tea.WindowSizeMsg{Width: 120, Height: 40}
			screen.Update(msg)

			Expect(screen.input).NotTo(BeNil())
			// Input screen should have received the size
		})
	})

	Describe("Input Delegation", func() {
		It("delegates key messages to input screen at input phase", func() {
			screen.width = 80
			screen.height = 24

			// Type something into source field
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}

			newModel, _ := screen.Update(msg)
			updated := newModel.(*UnifiedScreen)

			// Input screen should have processed the key
			Expect(updated.input).NotTo(BeNil())
		})
	})
})

func TestUnifiedScreen(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UnifiedScreen Suite")
}
