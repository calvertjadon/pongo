package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/calvertjadon/pongo/internal/core"
)

const (
	minimumWidth  = 80
	minimumHeight = 28
)

// Render projects semantic application state into the current terminal dimensions.
func Render(application core.Model, width, height int) string {
	switch application.Screen() {
	case core.ScreenTitle:
		return placeFixed(renderTitle(), width, height)
	case core.ScreenSetup:
		return placeFixed(renderSetup(application.Draft(), application.SetupFocus()), width, height)
	default:
		return ""
	}
}

func renderTitle() string {
	prompts := lipgloss.JoinVertical(lipgloss.Left, "Enter  Match setup", "Q      Quit")

	return lipgloss.JoinVertical(lipgloss.Center, "PONGO", "", "Classic Pong for your terminal.", "", prompts)
}

// placeFixed centers content in the fixed 80 by 28 layout, then centers that layout in the terminal. Odd surplus
// goes to the right and below.
func placeFixed(content string, width, height int) string {
	if width < minimumWidth || height < minimumHeight {
		return content
	}
	canvas := lipgloss.Place(minimumWidth, minimumHeight, lipgloss.Center, lipgloss.Center, content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, canvas)
}
