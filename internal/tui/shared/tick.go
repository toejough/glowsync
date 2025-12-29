package shared

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TickMsg is a message sent on each tick interval
type TickMsg time.Time

// TickCmd returns a command that sends tick messages at regular intervals
func TickCmd() tea.Cmd {
	return tea.Tick(TickIntervalMs*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}
