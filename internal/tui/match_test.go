package tui_test

import (
	"math"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/calvertjadon/pongo/internal/core"
	"github.com/calvertjadon/pongo/internal/tui"
)

// Match frame rows and columns, mirrored from the fixed layout under test.
const (
	frameScoreRow   = 1
	frameContextRow = 2
	frameCourtRow   = 3
	frameFeedback   = 25
	frameControls   = 26
	frameCourtLeft  = 4
	frameCenterRow  = 14
	frameCenterCol  = 40
)

func TestMatchFitsMinimumTerminal(t *testing.T) {
	t.Parallel()

	model, _ := startMatch(t, core.ModeLocalTwoPlayer)
	view := tui.Render(model, 80, 28)
	if width := lipgloss.Width(view); width != 80 {
		t.Fatalf("match width = %d, want 80", width)
	}
	if height := lipgloss.Height(view); height != 28 {
		t.Fatalf("match height = %d, want 28", height)
	}

	frame := lines(view)
	border := strings.Repeat("-", core.CourtWidth)
	if want := "+" + border + "+"; frame[frameCourtRow][frameCourtLeft:frameCourtLeft+len(want)] != want {
		t.Errorf("court top border = %q, want %q", frame[frameCourtRow], want)
	}

	for offset := range core.PaddleHeight {
		if cell := frame[frameCourtRow+1+8+offset][frameCourtLeft+1+core.PaddleLeftColumn]; cell != '#' {
			t.Errorf("left paddle row %d = %q, want #", offset, cell)
		}
		if cell := frame[frameCourtRow+1+8+offset][frameCourtLeft+1+core.PaddleRightColumn]; cell != '#' {
			t.Errorf("right paddle row %d = %q, want #", offset, cell)
		}
	}

	if digit := frame[frameCenterRow][frameCenterCol]; digit < '1' || digit > '3' {
		t.Errorf("countdown cell = %q, want a digit over the court center", digit)
	}
	if score := frame[frameScoreRow]; !strings.Contains(score, "PLAYER 1") ||
		!strings.Contains(score, "PLAYER 2") {
		t.Errorf("score row = %q, want both players", score)
	}
	if context := frame[frameContextRow]; !strings.Contains(context, "First to 7") {
		t.Errorf("context row = %q, want the winning score", context)
	}
	if controls := frame[frameControls]; !strings.Contains(controls, "W/S") {
		t.Errorf("controls row = %q, want paddle controls", controls)
	}
}

func TestMatchCentersFixedLayoutWithOddSurplus(t *testing.T) {
	t.Parallel()

	model, _ := startMatch(t, core.ModeLocalTwoPlayer)
	baseRow, baseColumn := locate(lines(tui.Render(model, 80, 28)), "PLAYER 1")
	tests := []struct{ width, height int }{
		{width: 81, height: 29},
		{width: 83, height: 31},
		{width: 101, height: 35},
	}
	for _, test := range tests {
		view := tui.Render(model, test.width, test.height)
		if lipgloss.Width(view) != test.width || lipgloss.Height(view) != test.height {
			t.Errorf("%dx%d view is %dx%d", test.width, test.height, lipgloss.Width(view), lipgloss.Height(view))
		}
		columnShift, rowShift := (test.width-80)/2, (test.height-28)/2
		row, column := locate(lines(view), "PLAYER 1")
		if row != baseRow+rowShift || column != baseColumn+columnShift {
			t.Errorf("%dx%d: score at row %d, column %d; want row %d, column %d",
				test.width, test.height, row, column, baseRow+rowShift, baseColumn+columnShift)
		}
	}
}

func TestMatchShowsTheCountdownDigitThenTheStillBall(t *testing.T) {
	t.Parallel()

	counting, schedule := startMatch(t, core.ModeLocalTwoPlayer)
	rally := finishCountdown(t, counting, schedule.Timer().Token)
	if rally.MatchPhase() != core.MatchPhaseRally {
		t.Fatalf("rally model is in phase %v, want rally in play", rally.MatchPhase())
	}

	countingFrame, rallyFrame := lines(tui.Render(counting, 80, 28)), lines(tui.Render(rally, 80, 28))
	if digit := countingFrame[frameCenterRow][frameCenterCol]; digit < '1' || digit > '3' {
		t.Errorf("countdown cell = %q, want a digit", digit)
	}
	if cell := rallyFrame[frameCenterRow][frameCenterCol]; cell != 'o' {
		t.Errorf("rally cell = %q, want the ball at the court center", cell)
	}
	if countingFrame[frameContextRow] != rallyFrame[frameContextRow] ||
		countingFrame[frameCourtRow] != rallyFrame[frameCourtRow] {
		t.Error("finishing the countdown moved the layout")
	}
}

func TestMatchControlsFollowTheMode(t *testing.T) {
	t.Parallel()

	localModel, _ := startMatch(t, core.ModeLocalTwoPlayer)
	singleModel, _ := startMatch(t, core.ModeSinglePlayer)
	local := lines(tui.Render(localModel, 80, 28))[frameControls]
	single := lines(tui.Render(singleModel, 80, 28))[frameControls]

	if local == single {
		t.Fatalf("local and single-player controls are both %q, want mode-specific controls", local)
	}
	if !strings.Contains(local, "Player 1") || !strings.Contains(local, "Player 2") {
		t.Errorf("local controls = %q, want both players", local)
	}
	if !strings.Contains(single, "W/S") || !strings.Contains(single, "Up/Down") {
		t.Errorf("single-player controls = %q, want both binding groups", single)
	}
	if strings.Contains(single, "Player 2") || strings.Contains(single, "CPU") {
		t.Errorf("single-player controls = %q, want no controls for the computer's paddle", single)
	}
}

// beginRally starts a match in mode, spends the opening countdown, and returns the rally with its timer token.
func beginRally(t *testing.T, mode core.Mode) (core.Model, core.TimerToken) {
	t.Helper()

	model, schedule := startMatch(t, mode)
	token := schedule.Timer().Token
	for range 2*int(core.CountdownSteps) + 1 {
		var effect core.Effect
		model, effect = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
		if model.MatchPhase() == core.MatchPhaseRally {
			if effect.Kind() != core.EffectScheduleTimer {
				t.Fatalf("rally launch requested %v, want a timer", effect.Kind())
			}

			return model, effect.Timer().Token
		}
	}
	t.Fatal("countdown did not finish")

	return model, 0
}

// scoreUntilCountdown drives the rally until a point is scored and returns the new countdown and its token.
func scoreUntilCountdown(t *testing.T, model core.Model, token core.TimerToken) (core.Model, core.TimerToken) {
	t.Helper()

	for range 4000 {
		var effect core.Effect
		model, effect = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
		if model.MatchPhase() == core.MatchPhaseCountdown {
			if effect.Kind() != core.EffectScheduleTimer {
				t.Fatalf("point requested %v, want a countdown timer", effect.Kind())
			}

			return model, effect.Timer().Token
		}
	}
	t.Fatal("no point was scored")

	return model, 0
}

// startMatch replays the setup keys for mode and returns the match with the timer its countdown requested.
func startMatch(t *testing.T, mode core.Mode) (core.Model, core.Effect) {
	t.Helper()

	model, _ := core.Update(core.NewModel(), core.EventPrimary{})
	if mode == core.ModeLocalTwoPlayer {
		model, _ = core.Update(model, core.EventAdjust{Direction: core.DirectionRight})
	}
	started, schedule := core.Update(focusMatchStart(t, model), core.EventPrimary{})
	if started.Screen() != core.ScreenMatch {
		t.Fatalf("start match left screen %v, want a match", started.Screen())
	}

	return started, schedule
}

// focusMatchStart navigates up through setup's wrapping rows until Start match is focused.
func focusMatchStart(t *testing.T, model core.Model) core.Model {
	t.Helper()

	for range 4 {
		if model.SetupFocus() == core.SetupRowStart {
			return model
		}
		model, _ = core.Update(model, core.EventNavigate{Direction: core.DirectionUp})
	}
	t.Fatalf("setup never focused Start match")

	return model
}

// finishCountdown spends the model's countdown and returns the model once the rally begins.
func finishCountdown(t *testing.T, model core.Model, token core.TimerToken) core.Model {
	t.Helper()

	for range 2 * int(core.CountdownSteps) {
		if model.MatchPhase() != core.MatchPhaseCountdown {
			return model
		}
		model, _ = core.Update(model, core.EventElapsed{Elapsed: core.ElapsedLimit, Token: token})
	}
	t.Fatalf("countdown did not finish")

	return model
}

func TestMatchFeedbackShowsInTheReservedFooterWithoutMovingTheCourt(t *testing.T) {
	t.Parallel()

	rally, token := beginRally(t, core.ModeLocalTwoPlayer)
	scored, _ := scoreUntilCountdown(t, rally, token)
	rallyFrame := lines(tui.Render(rally, 80, 28))
	scoredFrame := lines(tui.Render(scored, 80, 28))

	feedback := scoredFrame[frameFeedback]
	if !strings.Contains(feedback, "PLAYER 1 scores") {
		t.Errorf("feedback row = %q, want the left player's point", feedback)
	}
	if !strings.Contains(feedback, "Serve to PLAYER 2") {
		t.Errorf("feedback row = %q, want the next serve toward the conceding player", feedback)
	}
	if blank := strings.Repeat(" ", 80); rallyFrame[frameFeedback] != blank {
		t.Errorf("feedback row during a rally = %q, want blank", rallyFrame[frameFeedback])
	}
	for _, row := range []int{frameContextRow, frameCourtRow, frameControls} {
		if scoredFrame[row] != rallyFrame[row] {
			t.Errorf("row %d changed after the point: %q, want %q", row, scoredFrame[row], rallyFrame[row])
		}
	}
}

func TestMatchFeedbackClearsWhenTheNextRallyBegins(t *testing.T) {
	t.Parallel()

	rally, token := beginRally(t, core.ModeLocalTwoPlayer)
	scored, countdownToken := scoreUntilCountdown(t, rally, token)
	served := finishCountdown(t, scored, countdownToken)

	if row := lines(tui.Render(served, 80, 28))[frameFeedback]; row != strings.Repeat(" ", 80) {
		t.Errorf("feedback row when the rally begins = %q, want blank", row)
	}
}

func TestMatchDrawsTheBallInTheNearestCell(t *testing.T) {
	t.Parallel()

	rally, token := beginRally(t, core.ModeLocalTwoPlayer)
	frame := lines(tui.Render(rally, 80, 28))
	// The serve starts exactly at (34.5, 9.5); an exact half rounds to the higher cell.
	if cell := frame[frameCenterRow][frameCenterCol]; cell != 'o' {
		t.Fatalf("ball cell at the serve = %q, want o at the higher half cell", cell)
	}

	stepped, _ := core.Update(rally, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
	x, y := stepped.BallPosition()
	if x == math.Trunc(x) || y == math.Trunc(y) {
		t.Fatalf("physics position (%v, %v) rounded, want the unrounded point", x, y)
	}
	frame = lines(tui.Render(stepped, 80, 28))
	row, column := int(math.Floor(y+0.5)), int(math.Floor(x+0.5))
	if cell := frame[frameCourtRow+1+row][frameCourtLeft+1+column]; cell != 'o' {
		t.Errorf("ball cell after one step = %q, want o at row %d, column %d", cell, row, column)
	}
	if cell := frame[frameCourtRow+1+row][frameCourtLeft+1+column-1]; cell == 'o' {
		t.Errorf("ball drawn one cell left of the nearest cell")
	}
	if cell := frame[frameCenterRow][frameCenterCol]; cell == 'o' {
		t.Errorf("ball is still drawn at its previous cell")
	}
}

func TestMatchCountdownDigitFallsAsTheCountdownRuns(t *testing.T) {
	t.Parallel()

	model, schedule := startMatch(t, core.ModeLocalTwoPlayer)
	token := schedule.Timer().Token
	tests := []struct {
		name   string
		spent  core.Steps
		expect byte
	}{
		{name: "start", spent: 0, expect: '3'},
		{name: "second third", spent: 32, expect: '2'},
		{name: "last third", spent: 64, expect: '1'},
	}
	for _, test := range tests {
		counting := model
		for range test.spent {
			counting, _ = core.Update(counting, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
		}
		frame := lines(tui.Render(counting, 80, 28))
		if digit := frame[frameCenterRow][frameCenterCol]; digit != test.expect {
			t.Errorf("%s: countdown cell = %q, want %q", test.name, digit, test.expect)
		}
	}
}
