package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	minimumWidth  = 80
	minimumHeight = 28
)

var titleLines = []string{
	"PONGO",
	"",
	"Terminal Pong with state-driven design.",
	"",
	"Q  Quit",
}

// RenderTitle projects the title into the current terminal dimensions.
func RenderTitle(width, height int) string {
	title := strings.Join(titleLines, "\n")
	canvas := lipgloss.Place(minimumWidth, minimumHeight, lipgloss.Center, lipgloss.Center, title)
	if width < minimumWidth || height < minimumHeight {
		return title
	}

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, canvas)
}
