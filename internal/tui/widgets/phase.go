package widgets

// NewPhaseWidget creates a widget that displays the current phase message.
// Returns a closure that returns the appropriate message for the given phase.
func NewPhaseWidget(phase string) func() string {
	return func() string {
		switch phase {
		case "analyzing":
			return "Analyzing files..."
		case "confirmation":
			return "Preparing sync..."
		case "syncing":
			return "Syncing files..."
		default:
			return ""
		}
	}
}
