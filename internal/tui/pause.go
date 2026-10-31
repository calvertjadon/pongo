package tui

import "github.com/calvertjadon/pongo/internal/core"

// Pause overlay geometry from the settled layout: a 34 by 10 box with its menu rows two lines apart, centered over
// the fixed court.
const (
	pauseBoxWidth  = 34
	pauseBoxHeight = 10
	pauseRowGap    = 2
)

// drawPauseOverlay draws the pause menu over the frozen court. It is called after the court, so the box hides whatever
// lies beneath it while the score rows and court borders stay visible.
func drawPauseOverlay(rows [][]rune, application core.Model) {
	top := courtRow + 1 + (core.CourtHeight-pauseBoxHeight)/2
	left := courtColumn + 1 + (core.CourtWidth-pauseBoxWidth)/2
	drawBox(rows, top, left, pauseBoxWidth, pauseBoxHeight, " PAUSED ")

	focus := application.PauseFocus()
	for row := core.PauseRow(0); row <= core.PauseRowReturnToTitle; row++ {
		marker := "  "
		if row == focus {
			marker = "> "
		}
		writeText(rows[top+2+pauseRowGap*int(row)], left+8, marker+pauseRowLabel(row))
	}
	writeText(rows[top+8], left+5, "Enter select | P resume")
}

// drawBox draws a bordered box with title set into the top border.
func drawBox(rows [][]rune, top, left, width, height int, title string) {
	for row := top; row < top+height; row++ {
		for column := left; column < left+width; column++ {
			rows[row][column] = ' '
		}
		rows[row][left] = '|'
		rows[row][left+width-1] = '|'
	}
	drawBorder(rows[top], left, width)
	drawBorder(rows[top+height-1], left, width)
	writeText(rows[top], left+2, title)
}

// drawBorder draws one horizontal box border.
func drawBorder(row []rune, left, width int) {
	for column := left; column < left+width; column++ {
		row[column] = '-'
	}
	row[left] = '+'
	row[left+width-1] = '+'
}

// pauseRowLabel names one pause-menu action.
func pauseRowLabel(row core.PauseRow) string {
	switch row {
	case core.PauseRowResume:
		return "Resume"
	case core.PauseRowRestart:
		return "Restart match"
	case core.PauseRowReturnToTitle:
		return "Return to title"
	}

	return ""
}

// writeText writes ASCII text into a frame row starting at column, clipping at the row's edges.
func writeText(row []rune, column int, text string) {
	for offset, character := range []rune(text) {
		if at := column + offset; at >= 0 && at < len(row) {
			row[at] = character
		}
	}
}
