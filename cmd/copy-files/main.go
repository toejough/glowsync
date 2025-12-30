// Package main is the entry point for the copy-files application.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joe/copy-files/internal/config"
	"github.com/joe/copy-files/internal/tui"
	"golang.org/x/term" //nolint:depguard // Required for TTY detection
)

func main() {
	// Parse configuration
	cfg, err := config.ParseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create and run TUI
	model := tui.NewAppModel(cfg)

	// Only use alt screen if stdout is a TTY
	var opts []tea.ProgramOption
	if term.IsTerminal(int(os.Stdout.Fd())) {
		opts = append(opts, tea.WithAltScreen())
	}

	p := tea.NewProgram(model, opts...)

	_, err = p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
