package tui

import (
	"fmt"
	"strings"
)

// undersized reports whether a terminal of these dimensions cannot show the fixed 80 by 28 layout. It is a condition
// of the host, not an application state.
func undersized(width, height int) bool {
	return width < minimumWidth || height < minimumHeight
}

// renderNotice replaces the whole screen while the terminal is undersized: the game's name, the current dimensions,
// the required ones, and what to do about it. The notice is centered, and every line and row is clipped to the
// terminal, so the output never wraps, never exceeds the terminal, and never panics, down to one by one.
func renderNotice(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	contents := []string{
		"PONGO",
		fmt.Sprintf("Terminal: %d x %d", width, height),
		fmt.Sprintf("Required: %d x %d", minimumWidth, minimumHeight),
		"Resize the terminal to continue.",
	}
	top := max((height-len(contents))/2, 0)
	rows := make([]string, height)
	for row := range rows {
		rows[row] = strings.Repeat(" ", width)
		if offset := row - top; offset >= 0 && offset < len(contents) {
			rows[row] = centeredLine(contents[offset], width)
		}
	}

	return strings.Join(rows, "\n")
}

// centeredLine centers one line of notice text in width cells and clips what does not fit. Notice text is ASCII, so
// its bytes are its cells.
func centeredLine(text string, width int) string {
	content := text[:min(len(text), width)]
	left := (width - len(content)) / 2

	return strings.Repeat(" ", left) + content + strings.Repeat(" ", width-left-len(content))
}
