package tui_test

import (
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
