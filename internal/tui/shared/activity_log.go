package shared

import (
	"strings"
)

// RenderActivityLog renders a chronological activity log with optional title.
// Entries are displayed in chronological order (oldest to newest).
// If maxEntries > 0, limits display to the most recent N entries.
func RenderActivityLog(title string, entries []string, maxEntries int) string {
	var builder strings.Builder

	// Handle title rendering
	// Using strings.TrimSpace to treat whitespace-only titles as empty
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle != "" {
		// Use RenderLabel for bold + colored styling (pattern from tests)
		builder.WriteString(RenderLabel(trimmedTitle))
		builder.WriteString("\n")

		// Add blank line after title section (wsl_v5 pattern from reference files)
		if len(entries) > 0 {
			builder.WriteString("\n")
		}
	}

	// Handle nil/empty entries gracefully
	if len(entries) == 0 {
		return builder.String()
	}

	// Calculate which entries to display
	// Preallocating start index variable to avoid recalculation
	startIdx := 0

	// maxEntries <= 0 means show all entries
	// maxEntries > 0 means show most recent N entries
	if maxEntries > 0 && maxEntries < len(entries) {
		// Calculate start index to show most recent N entries
		// Example: 5 entries, maxEntries=3 → start at index 2 (entries[2:5])
		startIdx = len(entries) - maxEntries
	}

	// Render entries in chronological order (oldest first)
	// Using 2-space indentation for entries (consistent with TUI patterns)
	for i := startIdx; i < len(entries); i++ {
		builder.WriteString("  ")
		builder.WriteString(entries[i])

		// Add newline after each entry except the last
		// This keeps output clean without trailing newline
		if i < len(entries)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}
