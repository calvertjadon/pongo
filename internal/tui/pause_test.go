package tui_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/calvertjadon/pongo/internal/core"
	"github.com/calvertjadon/pongo/internal/tui"
)

// Pause overlay geometry from the settled layout: a 34 by 10 box centered over the 70 by 20 court.
const (
	pauseOverlayWidth  = 34
	pauseOverlayHeight = 10
	pauseFrameTop      = 9
	pauseFrameLeft     = 23
	pauseFirstRow      = pauseFrameTop + 2
	pauseRowGap        = 2
	pauseMenuColumn    = pauseFrameLeft + 8
)

func TestPauseOverlayIsCenteredOverTheFrozenCourt(t *testing.T) {
	t.Parallel()

	frame := lines(tui.Render(pausedModel(t), 80, 28))
	top := frameCourtRow + 1 + (core.CourtHeight-pauseOverlayHeight)/2
	left := frameCourtLeft + 1 + (core.CourtWidth-pauseOverlayWidth)/2
	if top != pauseFrameTop || left != pauseFrameLeft {
		t.Fatalf("overlay origin = (%d, %d), want the layout's (%d, %d)", top, left, pauseFrameTop, pauseFrameLeft)
	}

	for _, corner := range [][2]int{
		{top, left},
		{top, left + pauseOverlayWidth - 1},
		{top + pauseOverlayHeight - 1, left},
		{top + pauseOverlayHeight - 1, left + pauseOverlayWidth - 1},
	} {
		if cell := frame[corner[0]][corner[1]]; cell != '+' {
			t.Errorf("overlay corner at row %d, column %d = %q, want +", corner[0], corner[1], cell)
		}
	}
	if strip := frame[top][left:]; !strings.HasPrefix(strip, "+- PAUSED ") {
		t.Errorf("overlay top row = %q, want the PAUSED title in the border", frame[top])
	}
	for _, row := range []int{top + 1, top + 5} {
		if side := frame[row][left]; side != '|' {
			t.Errorf("overlay side at row %d = %q, want |", row, side)
		}
	}
}

func TestPauseMenuMarksTheActiveRowWithoutColor(t *testing.T) {
	t.Parallel()

	view := tui.Render(pausedModel(t), 80, 28)
	frame := lines(view)
	rows := []struct {
		name   string
		line   string
		marker string
	}{
		{name: "Resume", line: frame[pauseFirstRow], marker: "> Resume"},
		{name: "Restart match", line: frame[pauseFirstRow+pauseRowGap], marker: "  Restart match"},
		{name: "Return to title", line: frame[pauseFirstRow+2*pauseRowGap], marker: "  Return to title"},
	}
	for _, row := range rows {
		if !strings.Contains(row.line, row.marker) {
			t.Errorf("%s row = %q, want %q", row.name, row.line, row.marker)
		}
	}
	if strings.Contains(view, "\x1b[") {
		t.Error("pause view carries color escapes, want plain text")
	}
	if hint := frame[pauseFirstRow+3*pauseRowGap]; !strings.Contains(hint, "P resume") ||
		!strings.Contains(hint, "Enter") {
		t.Errorf("pause hint row = %q, want the Enter and P actions", hint)
	}
}

func TestPauseMenuMovesTheActiveMarker(t *testing.T) {
	t.Parallel()

	paused := pausedModel(t)
	selected, _ := core.Update(paused, core.EventNavigate{Direction: core.DirectionDown})
	frame := lines(tui.Render(selected, 80, 28))
	if row := frame[pauseFirstRow]; !strings.Contains(row, "  Resume") || strings.Contains(row, "> Resume") {
		t.Errorf("Resume row after moving = %q, want it unselected", row)
	}
	if row := frame[pauseFirstRow+pauseRowGap]; !strings.Contains(row, "> Restart match") {
		t.Errorf("Restart row after moving = %q, want it selected", row)
	}
}

func TestPauseKeepsTheScoreCourtBallAndPaddlesVisible(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	// Move the ball past the overlay's right edge so the frozen court shows it beside the menu.
	for range 80 {
		model, _ = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
	}
	paused, _ := core.Update(model, core.EventPauseToggle{})
	frame := lines(tui.Render(paused, 80, 28))

	if score := frame[frameScoreRow]; !strings.Contains(score, "PLAYER 1") || !strings.Contains(score, "PLAYER 2") {
		t.Errorf("score row while paused = %q, want both players", score)
	}
	if border := frame[frameCourtRow]; !strings.Contains(border, "+---") {
		t.Errorf("court border while paused = %q, want the court visible", border)
	}
	x, y := paused.BallPosition()
	row, column := int(y+0.5), int(x+0.5)
	frameColumn := frameCourtLeft + 1 + column
	if frameColumn >= pauseFrameLeft && frameColumn < pauseFrameLeft+pauseOverlayWidth {
		t.Fatalf("frozen ball column %d lies under the overlay, want it in the visible court", frameColumn)
	}
	if cell := frame[frameCourtRow+1+row][frameColumn]; cell != 'o' {
		t.Errorf("frozen ball cell = %q, want o at row %d, column %d", cell, row, column)
	}
	left, right := paused.PaddleTops()
	if cell := frame[frameCourtRow+1+int(left)][frameCourtLeft+1+core.PaddleLeftColumn]; cell != '#' {
		t.Errorf("left paddle cell while paused = %q, want #", cell)
	}
	if cell := frame[frameCourtRow+1+int(right)][frameCourtLeft+1+core.PaddleRightColumn]; cell != '#' {
		t.Errorf("right paddle cell while paused = %q, want #", cell)
	}
}

func TestResumeCountdownKeepsTheFrozenBallVisible(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	for range 20 {
		model, _ = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
	}
	frozenX, frozenY := model.BallPosition()
	paused, _ := core.Update(model, core.EventPauseToggle{})
	resumed, _ := core.Update(paused, core.EventPauseToggle{})
	frame := lines(tui.Render(resumed, 80, 28))

	if digit := frame[frameCenterRow][frameCenterCol]; digit != '3' {
		t.Errorf("resume countdown cell = %q, want the full countdown digit", digit)
	}
	row, column := int(frozenY+0.5), int(frozenX+0.5)
	if cell := frame[frameCourtRow+1+row][frameCourtLeft+1+column]; cell != 'o' {
		t.Errorf("frozen ball during the resume countdown = %q, want o at row %d, column %d", cell, row, column)
	}
	if width := lipgloss.Width(tui.Render(resumed, 80, 28)); width != 80 {
		t.Errorf("resume view width = %d, want 80", width)
	}
}

// TestPauseBoxKeepsItsBordersAndHidesTheCourtItCovers pins the pause box: an intact border exactly its own size, an
// opaque interior, and no drawing outside it.
func TestPauseBoxKeepsItsBordersAndHidesTheCourtItCovers(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	for range 20 {
		model, _ = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
	}
	court := lines(tui.Render(model, 80, 28))
	paused, _ := core.Update(model, core.EventPauseToggle{})
	frame := lines(tui.Render(paused, 80, 28))

	top, left := pauseFrameTop, pauseFrameLeft
	border := "+" + strings.Repeat("-", pauseOverlayWidth-2) + "+"
	for _, row := range []int{top, top + pauseOverlayHeight - 1} {
		if frame[row][left] != '+' || frame[row][left+pauseOverlayWidth-1] != '+' {
			t.Errorf("pause box corners at row %d = %q and %q, want +",
				row, frame[row][left], frame[row][left+pauseOverlayWidth-1])
		}
		if frame[row][left-1] != court[row][left-1] ||
			frame[row][left+pauseOverlayWidth] != court[row][left+pauseOverlayWidth] {
			t.Errorf("pause box border at row %d is drawn outside its %d columns", row, pauseOverlayWidth)
		}
	}
	if got := frame[top+pauseOverlayHeight-1][left : left+pauseOverlayWidth]; got != border {
		t.Errorf("pause box bottom border = %q, want %q", got, border)
	}
	for row := top + 1; row < top+pauseOverlayHeight-1; row++ {
		if cell := frame[row][left]; cell != '|' {
			t.Errorf("pause box left wall at row %d = %q, want |", row, cell)
		}
		if cell := frame[row][left+pauseOverlayWidth-1]; cell != '|' {
			t.Errorf("pause box right wall at row %d = %q, want |", row, cell)
		}
	}
	// The box hides the court's dashed center line where it covers it.
	for _, row := range []int{top + 1, top + 3, top + 5, top + 7} {
		if cell := frame[row][frameCenterCol]; cell != ' ' {
			t.Errorf("court cell under the pause box at row %d = %q, want blank", row, cell)
		}
	}
	for _, row := range []int{top - 1, top + pauseOverlayHeight} {
		if frame[row] != court[row] {
			t.Errorf("row %d outside the pause box changed from %q to %q", row, court[row], frame[row])
		}
	}
}

func TestPauseOverlayCentersWithOddSurplus(t *testing.T) {
	t.Parallel()

	paused := pausedModel(t)
	baseRow, baseColumn := locate(lines(tui.Render(paused, 80, 28)), "PAUSED")
	for _, size := range []struct{ width, height int }{{width: 83, height: 31}, {width: 101, height: 35}} {
		frame := lines(tui.Render(paused, size.width, size.height))
		row, column := locate(frame, "PAUSED")
		rowShift, columnShift := (size.height-28)/2, (size.width-80)/2
		if row != baseRow+rowShift {
			t.Errorf("%dx%d: PAUSED at row %d, want %d", size.width, size.height, row, baseRow+rowShift)
		}
		if want := baseColumn + columnShift; column != want {
			t.Errorf("%dx%d: PAUSED at column %d, want %d", size.width, size.height, column, want)
		}
	}
}

// pausedModel starts a local two-player rally, moves the ball away from the center, and pauses it.
func pausedModel(t *testing.T) core.Model {
	t.Helper()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	for range 20 {
		model, _ = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
	}
	paused, _ := core.Update(model, core.EventPauseToggle{})
	if paused.MatchPhase() != core.MatchPhasePaused {
		t.Fatalf("pause gave phase %v, want paused", paused.MatchPhase())
	}

	return paused
}

func TestMatchFooterAdvertisesPause(t *testing.T) {
	t.Parallel()

	frame := lines(tui.Render(pausedModel(t), 80, 28))
	if controls := frame[frameControls]; !strings.Contains(controls, "P pause") {
		t.Errorf("controls row = %q, want the pause key", controls)
	}
}
