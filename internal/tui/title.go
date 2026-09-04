package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/calvertjadon/pongo/internal/core"
)

const (
	minimumWidth  = 80
	minimumHeight = 28
	titleWidth    = 31
	title         = "PONGO\n\nClassic Pong for your terminal.\n\nQ  Quit"
)

// Render projects semantic application state into the current terminal dimensions.
func Render(application core.Model, width, height int) string {
	switch application.Screen() {
	case core.ScreenTitle:
		return renderTitle(width, height)
	default:
		return ""
	}
}

func renderTitle(width, height int) string {
	content := lipgloss.NewStyle().Width(titleWidth).Align(lipgloss.Center).Render(title)
	canvas := lipgloss.Place(minimumWidth, minimumHeight, lipgloss.Center, lipgloss.Center, content)
	if width < minimumWidth || height < minimumHeight {
		return content
	}

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, canvas)
}
