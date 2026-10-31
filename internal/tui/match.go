package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/calvertjadon/pongo/internal/core"
)

// Frame rows and columns of the fixed 80 by 28 match layout. The row between the court and the controls is reserved
// for scorer feedback.
const (
	scoreRow    = 1
	contextRow  = 2
	courtRow    = 3
	feedbackRow = courtRow + core.CourtHeight + 2
	controlsRow = courtRow + core.CourtHeight + 3
	courtColumn = 4
)

// renderMatch draws the court-first match layout: the score, the context line, the fixed court with both paddles, the
// reserved feedback row, and the controls that apply to the current mode.
func renderMatch(application core.Model) string {
	rows := make([][]rune, minimumHeight)
	for row := range rows {
		rows[row] = blankRow(minimumWidth)
	}

	settings := application.MatchSettings()
	writeCentered(rows[scoreRow], scoreLine(application))
	writeCentered(rows[contextRow], contextLine(settings))
	drawCourt(rows)
	drawPaddles(rows, application)
	drawPlay(rows, application)
	if application.MatchPhase() == core.MatchPhasePaused {
		drawPauseOverlay(rows, application)
	}
	writeCentered(rows[feedbackRow], feedbackLine(application))
	writeCentered(rows[controlsRow], controlsLine(settings.Mode))

	return joinRows(rows)
}

// drawCourt draws the court border and the center marks.
func drawCourt(rows [][]rune) {
	drawCourtBorder(rows, courtRow)
	drawCourtBorder(rows, courtRow+core.CourtHeight+1)

	for y := range core.CourtHeight {
		rows[interiorRow(y)][courtColumn] = '|'
		rows[interiorRow(y)][courtColumn+core.CourtWidth+1] = '|'
	}
	for y := 0; y < core.CourtHeight; y += 2 {
		rows[interiorRow(y)][interiorColumn(core.CourtWidth/2)] = ':'
	}
}

// drawCourtBorder draws one horizontal court border.
func drawCourtBorder(rows [][]rune, row int) {
	for column := courtColumn; column <= courtColumn+core.CourtWidth+1; column++ {
		rows[row][column] = '-'
	}
	rows[row][courtColumn] = '+'
	rows[row][courtColumn+core.CourtWidth+1] = '+'
}

// drawPaddles draws both four-row paddles at their court columns.
func drawPaddles(rows [][]rune, application core.Model) {
	left, right := application.PaddleTops()
	for offset := range core.PaddleHeight {
		rows[interiorRow(int(left)+offset)][interiorColumn(core.PaddleLeftColumn)] = '#'
		rows[interiorRow(int(right)+offset)][interiorColumn(core.PaddleRightColumn)] = '#'
	}
}

// drawPlay draws the countdown digit and the ball over the court. The digit shows while a countdown runs, and the ball
// shows whenever it is visible, so a frozen rally stays on the court through a pause and its resume.
func drawPlay(rows [][]rune, application core.Model) {
	if application.MatchPhase() == core.MatchPhaseCountdown {
		rows[interiorRow(core.CourtHeight/2)][interiorColumn(core.CourtWidth/2)] =
			countdownDigit(application.CountdownRemaining())
	}
	if application.BallVisible() {
		x, y := application.BallPosition()
		rows[interiorRow(ballCell(y, core.CourtHeight))][interiorColumn(ballCell(x, core.CourtWidth))] = 'o'
	}
}

// scoreLine names both players and their points.
func scoreLine(application core.Model) string {
	left, right := application.Score()
	rightName := "CPU"
	if application.MatchSettings().Mode == core.ModeLocalTwoPlayer {
		rightName = "PLAYER 2"
	}

	return fmt.Sprintf("PLAYER 1    %d     :     %d   %s", left, right, rightName)
}

// contextLine states what the match is played to and who plays it.
func contextLine(settings core.Settings) string {
	return fmt.Sprintf("First to %d | %s", settings.WinningScore, modeName(settings.Mode))
}

// feedbackLine names the player who scored the last point and where the next serve goes. It is empty when the rally
// that follows has already cleared the feedback.
func feedbackLine(application core.Model) string {
	scorer := application.Feedback()
	if scorer == core.FeedbackNone {
		return ""
	}

	settings := application.MatchSettings()

	return fmt.Sprintf(
		"%s scores  |  Serve to %s",
		playerName(settings, scorer == core.FeedbackLeftScored),
		playerName(settings, application.NextServe().Horizontal == core.DirectionLeft),
	)
}

// playerName names the left player or, on the right, the second local player or the computer.
func playerName(settings core.Settings, left bool) string {
	if left {
		return "PLAYER 1"
	}
	if settings.Mode == core.ModeLocalTwoPlayer {
		return "PLAYER 2"
	}

	return "CPU"
}

// controlsLine lists the movement keys that apply in the current mode, plus the pause key.
func controlsLine(mode core.Mode) string {
	switch mode {
	case core.ModeLocalTwoPlayer:
		return "W/S Player 1  |  Up/Down Player 2  |  P pause"
	case core.ModeSinglePlayer:
		return "W/S or Up/Down move  |  P pause"
	default:
		return "W/S or Up/Down move  |  P pause"
	}
}

// countdownDigit maps the steps left in a countdown to the digit drawn over the court: three, two, or one. The
// countdown ends as it reaches zero, so the digit never shows zero.
func countdownDigit(remaining core.Steps) rune {
	thirds := (3*int(remaining) + int(core.CountdownSteps) - 1) / int(core.CountdownSteps)

	return rune('0' + min(max(thirds, 1), 3))
}

// ballCell maps a fractional court coordinate to the cell the ball is drawn in: the nearest cell, with an exact half
// rounding to the higher index, clamped to the court.
func ballCell(coordinate float64, limit int) int {
	return min(max(int(math.Floor(coordinate+0.5)), 0), limit-1)
}

// interiorRow and interiorColumn map a court coordinate to its cell in the fixed frame.
func interiorRow(y int) int { return courtRow + 1 + y }

func interiorColumn(x int) int { return courtColumn + 1 + x }

// blankRow returns one full frame row of spaces.
func blankRow(width int) []rune {
	row := make([]rune, width)
	for index := range row {
		row[index] = ' '
	}

	return row
}

// writeCentered writes ASCII text into the middle of a frame row, clipping text that does not fit.
func writeCentered(row []rune, text string) {
	content := []rune(text)
	start := (len(row) - len(content)) / 2
	for offset, character := range content {
		if column := start + offset; column >= 0 && column < len(row) {
			row[column] = character
		}
	}
}

// joinRows turns the frame rows into one newline-separated block.
func joinRows(rows [][]rune) string {
	lines := make([]string, len(rows))
	for index, row := range rows {
		lines[index] = string(row)
	}

	return strings.Join(lines, "\n")
}
