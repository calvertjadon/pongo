// Package main provides the Pongo executable.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/calvertjadon/pongo/internal/tui"
)

func main() {
	os.Exit(run())
}

func run() int {
	_, err := tea.NewProgram(tui.NewModel()).Run()
	if err == nil {
		return 0
	}

	if _, writeErr := fmt.Fprintf(os.Stderr, "pongo: %v\n", err); writeErr != nil {
		return 1
	}

	return 1
}
