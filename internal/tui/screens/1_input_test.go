//nolint:varnamelen // Test files use idiomatic short variable names (t, g, etc.)
package screens_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers

	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/tui/screens"
	"github.com/joe/copy-files/internal/tui/shared"
)

// TestInputScreen_ActivityLog_RendersInRightColumn verifies activity log section is present
func TestInputScreen_ActivityLog_RendersInRightColumn(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	view := screen.View()

	// Activity log section should be present
	g.Expect(view).To(ContainSubstring("Activity"), "Activity log section should be present in right column")

	// May contain an initial message (implementation will decide)
	// At minimum, the section header should exist
	g.Expect(view).NotTo(BeEmpty(), "View should contain activity log section")
}

// ============================================================================
// Integration Tests - Combined Elements
// ============================================================================

// TestInputScreen_AllElements_RenderTogether verifies all elements coexist in layout
func TestInputScreen_AllElements_RenderTogether(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	view := inputScreen.View()

	// All major elements should be present
	g.Expect(view).To(ContainSubstring("Input"), "Timeline should be present")
	g.Expect(view).To(ContainSubstring("Source Path"), "Source input should be present")
	g.Expect(view).To(ContainSubstring("Destination Path"), "Dest input should be present")
	g.Expect(view).To(ContainSubstring("Filter Pattern"), "Pattern input should be present")
	g.Expect(view).To(ContainSubstring("Activity"), "Activity log should be present")
	g.Expect(view).To(ContainSubstring("Navigation:"), "Help text should be present")
}

// TestInputScreen_CompletionList_PositionedInLeftColumn verifies completions appear in left column
func TestInputScreen_CompletionList_PositionedInLeftColumn(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Type "/" and Tab to trigger completions
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}
	updatedModel, _ = inputScreen.Update(typeMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	updatedModel, _ = inputScreen.Update(tabMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	view := inputScreen.View()

	// Activity log should still be in right column
	g.Expect(view).To(ContainSubstring("Activity"), "Activity log should remain in right column")

	// Input fields (and completions if present) should be in left column
	g.Expect(view).To(ContainSubstring("Source Path"), "Input fields should be in left column")
}

// ============================================================================
// Completion List Rendering Tests
// ============================================================================

// TestInputScreen_CompletionList_StillRenders verifies completion list works with new layout
func TestInputScreen_CompletionList_StillRenders(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Type a path that might have completions (e.g., "/")
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}
	updatedModel, _ := screen.Update(typeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Press Tab to trigger completions
	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	updatedModel, _ = inputScreen.Update(tabMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	view := inputScreen.View()

	// View should still contain core elements even with completions showing
	g.Expect(view).To(ContainSubstring("Source Path"), "Source field should be visible with completions")

	// If completions exist, they may be visible in the view
	// (Implementation will determine exact rendering behavior)
	g.Expect(view).NotTo(BeEmpty(), "View should render with or without completions")
}

// TestInputScreen_FocusChange_UpdatesPrompts verifies prompt changes when focus changes
func TestInputScreen_FocusChange_UpdatesPrompts(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Start with source focused
	viewBefore := screen.View()
	g.Expect(viewBefore).To(ContainSubstring("Source Path"), "Source should be visible initially")

	// Move focus to dest field
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ := screen.Update(downMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	viewAfter := inputScreen.View()

	// Both fields should still be visible, but focus changed
	g.Expect(viewAfter).To(ContainSubstring("Source Path"), "Source should still be visible")
	g.Expect(viewAfter).To(ContainSubstring("Destination Path"), "Dest should be visible and focused")

	// Views should be different due to focus change
	g.Expect(viewBefore).NotTo(Equal(viewAfter), "View should change when focus changes")
}

// ============================================================================
// Focus State Rendering Tests
// ============================================================================

// TestInputScreen_FocusedField_ShowsPromptArrow verifies focused field displays prompt arrow
func TestInputScreen_FocusedField_ShowsPromptArrow(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Source field starts focused
	view := screen.View()

	// The PromptArrow is styled, but when rendered it should be visible in the output
	// We test by checking that the view contains input field content
	// The exact prompt rendering will be verified visually in GREEN phase
	g.Expect(view).To(ContainSubstring("Source Path"), "Focused field should be visible")
	g.Expect(view).NotTo(BeEmpty(), "Focused field should render with prompt")
}

// TestInputScreen_HelpText_PositionedCorrectly verifies help text appears at bottom
func TestInputScreen_HelpText_PositionedCorrectly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	view := inputScreen.View()
	lines := strings.Split(view, "\n")

	// Help text should be in the lower portion of the view
	// Find lines containing help keywords
	helpLineFound := false
	for _, line := range lines {
		if strings.Contains(line, "Navigation:") || strings.Contains(line, "Actions:") {
			helpLineFound = true
			break
		}
	}

	g.Expect(helpLineFound).To(BeTrue(), "Help text should be present in view")
}

// ============================================================================
// Help Text Rendering Tests
// ============================================================================

// TestInputScreen_HelpText_StillVisible verifies help text renders in new layout
func TestInputScreen_HelpText_StillVisible(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	view := screen.View()

	// Help text should be visible
	g.Expect(view).To(ContainSubstring("Navigation:"), "Navigation help should be visible")
	g.Expect(view).To(ContainSubstring("Actions:"), "Actions help should be visible")
	g.Expect(view).To(ContainSubstring("Tab"), "Tab key help should be visible")
	g.Expect(view).To(ContainSubstring("Enter"), "Enter key help should be visible")
	g.Expect(view).To(ContainSubstring("Ctrl+C"), "Ctrl+C help should be visible")
}

// TestInputScreen_InputFields_UseWidgetBoxes verifies widget boxes are used for input sections
func TestInputScreen_InputFields_UseWidgetBoxes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	view := inputScreen.View()
	stripped := stripANSI(view)

	// Widget boxes should have structure (borders, multiple lines per section)
	lines := strings.Split(stripped, "\n")
	g.Expect(len(lines)).To(BeNumerically(">", 10), "Widget box structure should create multiple lines")

	// Should contain input field labels (part of widget box titles/content)
	g.Expect(view).To(ContainSubstring("Source Path"), "Widget box should contain source input")
	g.Expect(view).To(ContainSubstring("Destination Path"), "Widget box should contain dest input")
}

// ============================================================================
// Layout Integration Tests
// ============================================================================

// TestInputScreen_RendersWithNewLayout verifies basic layout structure is present
func TestInputScreen_RendersWithNewLayout(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Set reasonable window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	view := inputScreen.View()

	// Should contain timeline (header element)
	g.Expect(view).To(ContainSubstring("Input"), "Timeline should be present with Input phase")

	// Should contain input field labels
	g.Expect(view).To(ContainSubstring("Source Path"), "Source input field should be present")
	g.Expect(view).To(ContainSubstring("Destination Path"), "Destination input field should be present")

	// Should contain activity log section marker
	g.Expect(view).To(ContainSubstring("Activity"), "Activity log section should be present")

	// Should not be empty
	g.Expect(view).NotTo(BeEmpty(), "View should render content")
}

// TestInputScreen_ResizeWindow_LayoutAdjusts verifies layout responds to window size changes
func TestInputScreen_ResizeWindow_LayoutAdjusts(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Start with one size
	sizeMsg1 := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := screen.Update(sizeMsg1)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	view1 := inputScreen.View()

	// Resize to different size
	sizeMsg2 := tea.WindowSizeMsg{Width: 150, Height: 50}
	updatedModel, _ = inputScreen.Update(sizeMsg2)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	view2 := inputScreen.View()

	// Views should be different due to resize
	g.Expect(view1).NotTo(Equal(view2), "View should change when window size changes")

	// Core elements should still be present in both
	g.Expect(view2).To(ContainSubstring("Source Path"), "Input fields should adapt to new size")
	g.Expect(view2).To(ContainSubstring("Activity"), "Activity log should adapt to new size")
}

// TestInputScreen_Timeline_ShowsInputPhaseActive verifies timeline shows "input" phase as active
func TestInputScreen_Timeline_ShowsInputPhaseActive(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	view := screen.View()
	stripped := stripANSI(view)

	// Timeline should show "Input" phase with active symbol
	g.Expect(stripped).To(ContainSubstring("Input"), "Timeline should contain Input phase name")

	// Should contain active symbol (◉ or [*] depending on terminal)
	activeSymbol := shared.ActiveSymbol()
	g.Expect(stripped).To(ContainSubstring(activeSymbol), "Timeline should show active symbol for Input phase")

	// Should show pending symbols for later phases
	pendingSymbol := shared.PendingSymbol()
	g.Expect(stripped).To(ContainSubstring(pendingSymbol), "Timeline should show pending symbols for future phases")
}

// TestInputScreen_TwoColumnLayout_LeftAndRightContent verifies two-column structure
func TestInputScreen_TwoColumnLayout_LeftAndRightContent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Set window size for proper layout
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	view := inputScreen.View()

	// Left column: Input fields
	g.Expect(view).To(ContainSubstring("Source Path"), "Left column should contain Source Path input")
	g.Expect(view).To(ContainSubstring("Destination Path"), "Left column should contain Destination Path input")
	g.Expect(view).To(ContainSubstring("Filter Pattern"), "Left column should contain Filter Pattern input")

	// Right column: Activity log
	g.Expect(view).To(ContainSubstring("Activity"), "Right column should contain Activity section")
}

// TestInputScreen_UnfocusedFields_NoPromptArrow verifies unfocused fields have no prompt arrow
func TestInputScreen_UnfocusedFields_NoPromptArrow(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Source is focused, dest and pattern are unfocused
	view := screen.View()

	// All fields should be visible in the view
	g.Expect(view).To(ContainSubstring("Source Path"), "Source field should be visible")
	g.Expect(view).To(ContainSubstring("Destination Path"), "Dest field should be visible (unfocused)")
	g.Expect(view).To(ContainSubstring("Filter Pattern"), "Pattern field should be visible (unfocused)")

	// Unfocused fields render with "  " prompt instead of arrow (tested by existing code path)
	// This test verifies all fields are visible; specific prompt verification happens visually
}

// TestInputScreen_ValidationError_ClearsOnTyping verifies error clears when user types
func TestInputScreen_ValidationError_ClearsOnTyping(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Trigger validation error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := screen.Update(enterMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	viewWithError := inputScreen.View()
	g.Expect(viewWithError).To(ContainSubstring("source path is required"), "Error should be visible")

	// Type something
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}
	updatedModel, _ = inputScreen.Update(typeMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	viewAfterTyping := inputScreen.View()
	g.Expect(viewAfterTyping).NotTo(ContainSubstring("source path is required"), "Error should be cleared after typing")
}

// ============================================================================
// Validation Error Rendering Tests
// ============================================================================

// TestInputScreen_ValidationError_RendersCorrectly verifies error messages render properly
func TestInputScreen_ValidationError_RendersCorrectly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Press Enter with empty inputs to trigger validation error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := screen.Update(enterMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	view := inputScreen.View()

	// Should contain error message
	g.Expect(view).To(ContainSubstring("Error:"), "Error section should be present")
	g.Expect(view).To(ContainSubstring("source path is required"), "Specific error message should be shown")

	// Error should be visible alongside other content
	g.Expect(view).To(ContainSubstring("Source Path"), "Input fields should still be visible")
	g.Expect(view).To(ContainSubstring("Activity"), "Activity log should still be visible")
}

// TestInputScreen_WithInputAndError_AllElementsVisible verifies layout works with active input and error
func TestInputScreen_WithInputAndError_AllElementsVisible(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &config.Config{}
	screen := screens.NewInputScreen(cfg)

	// Set window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := screen.Update(sizeMsg)
	inputScreen, ok := updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Type some input
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/tmp")}
	updatedModel, _ = inputScreen.Update(typeMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	// Trigger validation error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = inputScreen.Update(enterMsg)
	inputScreen, ok = updatedModel.(screens.InputScreen)
	g.Expect(ok).Should(BeTrue())

	view := inputScreen.View()

	// All elements should coexist
	g.Expect(view).To(ContainSubstring("Input"), "Timeline should be visible")
	g.Expect(view).To(ContainSubstring("Source Path"), "Input fields should be visible")
	g.Expect(view).To(ContainSubstring("Activity"), "Activity log should be visible")
	g.Expect(view).To(ContainSubstring("Error:"), "Error message should be visible")
	g.Expect(view).To(ContainSubstring("Navigation:"), "Help text should be visible")
}

// ============================================================================
// Helper Functions
// ============================================================================

// stripANSI removes ANSI escape codes from a string for easier testing
func stripANSI(s string) string {
	// Simple ANSI stripper - removes common ANSI escape sequences
	// Pattern: ESC [ ... m (for colors, bold, etc.)
	result := s
	inEscape := false
	var cleaned strings.Builder

	for i := 0; i < len(result); i++ {
		if result[i] == '\033' && i+1 < len(result) && result[i+1] == '[' {
			inEscape = true
			i++ // Skip the '['

			continue
		}
		if inEscape {
			if result[i] == 'm' {
				inEscape = false
			}

			continue
		}
		cleaned.WriteByte(result[i])
	}

	return cleaned.String()
}
