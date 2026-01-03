package widgets

import "strings"

const maxActivityEntries = 10

// NewActivityLogWidget creates a widget that displays recent activity entries.
// Returns a closure that formats the activity log from the activities list.
func NewActivityLogWidget(getActivities func() []string) func() string {
	return func() string {
		activities := getActivities()
		if len(activities) == 0 {
			return ""
		}

		// Limit to last N entries
		startIdx := 0
		if len(activities) > maxActivityEntries {
			startIdx = len(activities) - maxActivityEntries
		}

		var builder strings.Builder
		for i := startIdx; i < len(activities); i++ {
			builder.WriteString(activities[i])
			if i < len(activities)-1 {
				builder.WriteString("\n")
			}
		}

		return builder.String()
	}
}
