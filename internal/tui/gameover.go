package tui

import (
	"fmt"

	"github.com/calvertjadon/pongo/internal/core"
)

// Game-over overlay geometry from the settled layout: a 48 by 12 box centered over the fixed court, with the menu rows
// two lines apart below the winner and the final score.
const (
	gameOverBoxWidth   = 48
	gameOverBoxHeight  = 12
	gameOverMenuColumn = 13
	gameOverRowGap     = 2
)

// drawGameOverOverlay draws the result over the final court: the winner, the final score, and the menu with the
// selected row marked. The court, paddles, and score stay visible around the box, and no ball is drawn.
func drawGameOverOverlay(rows [][]rune, application core.Model) {
	top := courtRow + 1 + (core.CourtHeight-gameOverBoxHeight)/2
	left := courtColumn + 1 + (core.CourtWidth-gameOverBoxWidth)/2
	drawBox(rows, top, left, gameOverBoxWidth, gameOverBoxHeight, " MATCH COMPLETE ")

	settings := application.MatchSettings()
	writeCenteredInBox(rows[top+2], left, playerName(settings, application.Winner() == core.SideLeft)+" WINS")

	leftScore, rightScore := application.Score()
	writeCenteredInBox(rows[top+4], left, fmt.Sprintf("%d  -  %d", leftScore, rightScore))

	drawMenu(
		rows,
		top+6,
		left+gameOverMenuColumn,
		gameOverRowGap,
		int(core.GameOverRowReturnToTitle)+1,
		application.GameOverFocus(),
		gameOverRowLabel,
	)
}

// writeCenteredInBox writes text into the middle of one box row, clipping at the box edges.
func writeCenteredInBox(row []rune, left int, text string) {
	writeText(row, left+(gameOverBoxWidth-len(text))/2, text)
}

// gameOverRowLabel names one Game over menu action.
func gameOverRowLabel(row core.GameOverRow) string {
	switch row {
	case core.GameOverRowRematch:
		return "Rematch"
	case core.GameOverRowChangeSetup:
		return "Change setup"
	case core.GameOverRowReturnToTitle:
		return "Return to title"
	}

	return ""
}
