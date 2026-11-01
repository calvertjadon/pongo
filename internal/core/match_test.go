package core_test

import (
	"math"
	"slices"
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

func TestStartMatchLocksSettingsAndEntersOpeningCountdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mode       core.Mode
		score      core.WinningScore
		difficulty core.Difficulty
	}{
		{
			name:       "single player",
			mode:       core.ModeSinglePlayer,
			score:      core.WinningScore11,
			difficulty: core.DifficultyHard,
		},
		{
			name:       "local two-player",
			mode:       core.ModeLocalTwoPlayer,
			score:      core.WinningScore5,
			difficulty: core.DifficultyNormal,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			chosen := setupFor(t, test.mode, test.score, test.difficulty)
			started, effect := core.Update(chosen, core.EventPrimary{})

			assertOpeningMatch(t, started, effect, chosen.Draft())
		})
	}
}

// assertOpeningMatch checks the data a freshly started match locks and the timer its countdown requests.
func assertOpeningMatch(t *testing.T, started core.Model, effect core.Effect, draft core.Settings) {
	t.Helper()

	if started.Screen() != core.ScreenMatch || started.MatchPhase() != core.MatchPhaseCountdown {
		t.Fatalf("start match gave screen %v, phase %v; want a match in countdown",
			started.Screen(), started.MatchPhase())
	}
	if locked := started.MatchSettings(); locked != draft {
		t.Errorf("locked settings = %+v, want the draft %+v", locked, draft)
	}
	if left, right := started.Score(); left != 0 || right != 0 {
		t.Errorf("opening score = %d:%d, want 0:0", left, right)
	}
	if left, right := started.PaddleTops(); left != 8 || right != 8 {
		t.Errorf("opening paddle tops = %d, %d; want 8, 8", left, right)
	}
	if x, y := started.BallPosition(); x != 34.5 || y != 9.5 {
		t.Errorf("opening ball position = (%v, %v), want (34.5, 9.5)", x, y)
	}
	if remaining := started.CountdownRemaining(); remaining != core.CountdownSteps {
		t.Errorf("opening countdown = %d steps, want %d", remaining, core.CountdownSteps)
	}
	if serve := started.NextServe(); serve.Horizontal != core.DirectionRight || serve.Vertical != core.DirectionUp {
		t.Errorf("opening serve = %+v, want up and toward the right player", serve)
	}
	if started.Draft() != draft {
		t.Errorf("starting a match changed the session draft to %+v", started.Draft())
	}
	assertScheduled(t, effect, effect.Timer().Token)
	if effect.Timer().Delay != core.TimerDelay {
		t.Errorf("timer delay = %v, want %v", effect.Timer().Delay, core.TimerDelay)
	}
}

func TestCountdownCompletesIntoAServingRally(t *testing.T) {
	t.Parallel()

	model, schedule := openingCountdown(t)
	for range 10 {
		var effect core.Effect
		model, effect = core.Update(model, elapsed(schedule, core.ElapsedLimit))
		assertScheduled(t, effect, schedule.Timer().Token)
	}
	if remaining := model.CountdownRemaining(); remaining != 6 {
		t.Fatalf("countdown after ten full events = %d steps, want 6", remaining)
	}

	completed, effect := core.Update(model, elapsed(schedule, core.ElapsedLimit))
	if completed.MatchPhase() != core.MatchPhaseRally {
		t.Fatalf("countdown completion reached phase %v, want rally in play", completed.MatchPhase())
	}
	assertScheduled(t, effect, effect.Timer().Token)
	if token := effect.Timer().Token; token == schedule.Timer().Token {
		t.Fatalf("rally token = %d, want a new timing sequence", token)
	}
	if x, y := completed.BallPosition(); x != 34.5 || y != 9.5 {
		t.Errorf("ball after countdown = (%v, %v), want the court center (34.5, 9.5)", x, y)
	}
	vx, vy := stepVelocity(t, completed, effect.Timer().Token)
	assertClose(t, "serve vx", vx, 28*math.Cos(20*math.Pi/180))
	assertClose(t, "serve vy", vy, -28*math.Sin(20*math.Pi/180)/2)
	if feedback := completed.Feedback(); feedback != core.FeedbackNone {
		t.Errorf("feedback after a serve = %v, want none", feedback)
	}
	assertNoOps(t, completed, []core.Event{
		elapsed(schedule, core.ElapsedLimit),
		elapsed(schedule, core.StepDuration),
	})
}

func TestPaddleStepsDriveTheBindingGroupPaddleInLocalTwoPlayer(t *testing.T) {
	t.Parallel()

	started, _ := openingLocalMatch(t)
	tests := []struct {
		name      string
		step      core.EventPaddleStep
		wantLeft  uint8
		wantRight uint8
	}{
		{name: "W moves the left paddle up", step: wsStep(core.DirectionUp), wantLeft: 6, wantRight: 8},
		{name: "S moves the left paddle down", step: wsStep(core.DirectionDown), wantLeft: 10, wantRight: 8},
		{name: "Up moves the right paddle up", step: arrowStep(core.DirectionUp), wantLeft: 8, wantRight: 6},
		{name: "Down moves the right paddle down", step: arrowStep(core.DirectionDown), wantLeft: 8, wantRight: 10},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stepped, effect := core.Update(started, test.step)
			if left, right := stepped.PaddleTops(); left != test.wantLeft || right != test.wantRight {
				t.Errorf("paddle tops = %d, %d; want %d, %d", left, right, test.wantLeft, test.wantRight)
			}
			if effect.Kind() != core.EffectNone {
				t.Errorf("paddle step requested %v, want no effect", effect.Kind())
			}
		})
	}
}

func TestBothBindingGroupsDriveTheLeftPaddleInSinglePlayer(t *testing.T) {
	t.Parallel()

	started, _ := startSinglePlayerMatch(t)
	for _, step := range []core.EventPaddleStep{wsStep(core.DirectionDown), arrowStep(core.DirectionDown)} {
		stepped, _ := core.Update(started, step)
		left, right := stepped.PaddleTops()
		if left != 10 || right != 8 {
			t.Errorf("paddle tops after %+v = %d, %d; want the left paddle to move to 10", step, left, right)
		}
	}
}

func TestPaddleStepsClampAtBothCourtEdges(t *testing.T) {
	t.Parallel()

	started, _ := openingLocalMatch(t)
	tests := []struct {
		name      string
		step      core.EventPaddleStep
		wantLeft  uint8
		wantRight uint8
	}{
		{name: "left paddle at the top", step: wsStep(core.DirectionUp), wantLeft: 0, wantRight: 8},
		{name: "left paddle at the bottom", step: wsStep(core.DirectionDown), wantLeft: 16, wantRight: 8},
		{name: "right paddle at the top", step: arrowStep(core.DirectionUp), wantLeft: 8, wantRight: 0},
		{name: "right paddle at the bottom", step: arrowStep(core.DirectionDown), wantLeft: 8, wantRight: 16},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model := started
			for range 16 {
				model, _ = core.Update(model, test.step)
			}
			if left, right := model.PaddleTops(); left != test.wantLeft || right != test.wantRight {
				t.Errorf("paddle tops = %d, %d; want %d, %d", left, right, test.wantLeft, test.wantRight)
			}
		})
	}
}

func TestPaddleStepsWorkDuringTheCountdownAndTheRally(t *testing.T) {
	t.Parallel()

	model, schedule := openingLocalMatch(t)
	model, _ = core.Update(model, wsStep(core.DirectionDown))
	if left, _ := model.PaddleTops(); left != 10 {
		t.Fatalf("left paddle during countdown = %d, want 10", left)
	}

	model = finishCountdown(t, model, schedule.Timer().Token)
	model, _ = core.Update(model, arrowStep(core.DirectionDown))
	left, right := model.PaddleTops()
	if left != 10 || right != 10 {
		t.Errorf("paddle tops after a rally step = %d, %d; want 10, 10", left, right)
	}
}

func TestMatchIgnoresMenuAndQuitEvents(t *testing.T) {
	t.Parallel()

	started, _ := openingLocalMatch(t)
	assertNoOps(t, started, []core.Event{
		core.EventPrimary{},
		core.EventNavigate{Direction: core.DirectionUp},
		core.EventNavigate{Direction: core.DirectionDown},
		core.EventAdjust{Direction: core.DirectionLeft},
		core.EventAdjust{Direction: core.DirectionRight},
		core.EventBack{},
		core.EventQuit{},
	})
}

func TestMatchReplayReproducesStateAndEffects(t *testing.T) {
	t.Parallel()

	started, schedule := openingLocalMatch(t)
	events := []core.Event{
		wsStep(core.DirectionUp),
		arrowStep(core.DirectionDown),
		elapsed(schedule, core.ElapsedLimit),
		wsStep(core.DirectionDown),
		elapsed(schedule, core.ElapsedLimit, schedule.Timer().Token+1),
		elapsed(schedule, 3*core.StepDuration),
		core.EventPrimary{},
		arrowStep(core.DirectionUp),
	}
	firstModel, firstEffects := replayFrom(started, events)
	secondModel, secondEffects := replayFrom(started, events)

	if firstModel != secondModel {
		t.Fatalf("replay model = %+v, want %+v", secondModel, firstModel)
	}
	if !slices.Equal(firstEffects, secondEffects) {
		t.Fatalf("replay effects = %+v, want %+v", secondEffects, firstEffects)
	}
}

// setupFor returns a model in setup whose Start match row is focused, holding the requested draft.
func setupFor(t *testing.T, mode core.Mode, score core.WinningScore, difficulty core.Difficulty) core.Model {
	t.Helper()

	model, _ := replay([]core.Event{core.EventPrimary{}})
	if mode == core.ModeLocalTwoPlayer {
		model, _ = core.Update(model, core.EventAdjust{Direction: core.DirectionRight})
	}
	model = focusRow(t, model, core.SetupRowWinningScore)
	for model.Draft().WinningScore != score {
		model, _ = core.Update(model, core.EventAdjust{Direction: core.DirectionRight})
	}
	if mode == core.ModeSinglePlayer {
		model = focusRow(t, model, core.SetupRowDifficulty)
		for model.Draft().Difficulty != difficulty {
			model, _ = core.Update(model, core.EventAdjust{Direction: core.DirectionRight})
		}
	}

	return focusRow(t, model, core.SetupRowStart)
}

// openingLocalMatch starts a Local two-player match with a 7 point winning score.
func openingLocalMatch(t *testing.T) (core.Model, core.Effect) {
	t.Helper()

	setupModel := setupFor(t, core.ModeLocalTwoPlayer, core.WinningScore7, core.DifficultyNormal)
	started, schedule := core.Update(setupModel, core.EventPrimary{})
	if started.Screen() != core.ScreenMatch {
		t.Fatalf("start match left screen %v, want a match", started.Screen())
	}

	return started, schedule
}

// openingCountdown starts a local two-player match and returns the timer its countdown requested.
func openingCountdown(t *testing.T) (core.Model, core.Effect) {
	t.Helper()

	started, schedule := openingLocalMatch(t)
	if schedule.Kind() != core.EffectScheduleTimer {
		t.Fatalf("opening countdown requested %v, want a timer", schedule.Kind())
	}

	return started, schedule
}

// startSinglePlayerMatch starts a Single player match with the default draft.
func startSinglePlayerMatch(t *testing.T) (core.Model, core.Effect) {
	t.Helper()

	setupModel := setupFor(t, core.ModeSinglePlayer, core.WinningScore7, core.DifficultyNormal)

	return core.Update(setupModel, core.EventPrimary{})
}

// finishCountdown spends the countdown one step at a time until the rally begins.
func finishCountdown(t *testing.T, model core.Model, token core.TimerToken) core.Model {
	t.Helper()

	for range 2 * int(core.CountdownSteps) {
		if model.MatchPhase() != core.MatchPhaseCountdown {
			return model
		}
		model, _ = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
	}
	t.Fatalf("countdown did not finish within %d steps", 2*int(core.CountdownSteps))

	return model
}

// focusRow navigates down through setup until row is focused.
func focusRow(t *testing.T, model core.Model, row core.SetupRow) core.Model {
	t.Helper()

	for range 4 {
		if model.SetupFocus() == row {
			return model
		}
		model, _ = core.Update(model, core.EventNavigate{Direction: core.DirectionDown})
	}
	t.Fatalf("setup never focused %v", row)

	return model
}

// elapsed builds an elapsed event for the countdown's timing sequence, or for another token when given.
func elapsed(schedule core.Effect, amount core.Duration, other ...core.TimerToken) core.Event {
	token := schedule.Timer().Token
	if len(other) > 0 {
		token = other[0]
	}

	return core.EventElapsed{Elapsed: amount, Token: token}
}

func wsStep(direction core.VerticalDirection) core.EventPaddleStep {
	return core.EventPaddleStep{Direction: direction, Group: core.BindingWS}
}

func arrowStep(direction core.VerticalDirection) core.EventPaddleStep {
	return core.EventPaddleStep{Direction: direction, Group: core.BindingArrows}
}

// assertScheduled checks that effect requests a timer correlated with token.
func assertScheduled(t *testing.T, effect core.Effect, token core.TimerToken) {
	t.Helper()

	if effect.Kind() != core.EffectScheduleTimer {
		t.Fatalf("effect = %v, want a timer request", effect.Kind())
	}
	if got := effect.Timer().Token; got != token {
		t.Fatalf("timer token = %d, want %d", got, token)
	}
}

// replayFrom applies events to a model and returns the final state and every effect.
func replayFrom(model core.Model, events []core.Event) (core.Model, []core.Effect) {
	effects := make([]core.Effect, 0, len(events))
	for _, event := range events {
		var effect core.Effect
		model, effect = core.Update(model, event)
		effects = append(effects, effect)
	}

	return model, effects
}
