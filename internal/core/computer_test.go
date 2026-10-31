package core_test

import (
	"math"
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

// computerCase is one difficulty's specified schedule, mirrored from the decision ticket so behavior tests can check
// every profile against independent numbers.
type computerCase struct {
	name       string
	difficulty core.Difficulty
	reaction   int
	reaim      int
	moveEvery  int
	errorRows  float64
}

// computerCases returns the three profiles at 120 fixed steps per second.
func computerCases() []computerCase {
	return []computerCase{
		{name: "easy", difficulty: core.DifficultyEasy, reaction: 48, reaim: 30, moveEvery: 20, errorRows: 4},
		{name: "normal", difficulty: core.DifficultyNormal, reaction: 24, reaim: 18, moveEvery: 12, errorRows: 3},
		{name: "hard", difficulty: core.DifficultyHard, reaction: 12, reaim: 12, moveEvery: 8, errorRows: 2.25},
	}
}

func TestComputerWaitsForItsFirstReactionBeforeAiming(t *testing.T) {
	t.Parallel()

	for _, test := range computerCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, token := computerRally(t, test.difficulty)
			drawn := model.ComputerError()
			if drawn < -test.errorRows || drawn > test.errorRows {
				t.Errorf("opening approach error = %v, want inside [-%v, %v]",
					drawn, test.errorRows, test.errorRows)
			}

			for step := 1; step <= test.reaction; step++ {
				model = nextComputerStep(t, model, token)
				_, aimed := model.ComputerTarget()
				if want := step == test.reaction; aimed != want {
					t.Fatalf("aimed after %d steps = %v, want %v", step, aimed, want)
				}
			}
		})
	}
}

func TestComputerReaimsOnItsProfileCadence(t *testing.T) {
	t.Parallel()

	for _, test := range computerCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, token := computerRally(t, test.difficulty)
			for range test.reaction {
				model = nextComputerStep(t, model, token)
			}
			target, aimed := model.ComputerTarget()
			if !aimed {
				t.Fatalf("no target after the %d step reaction delay", test.reaction)
			}

			for range test.reaim - 1 {
				model = nextComputerStep(t, model, token)
				if got, _ := model.ComputerTarget(); got != target {
					t.Fatalf("target moved between observations: %v then %v", target, got)
				}
			}
			model = nextComputerStep(t, model, token)
			if got, _ := model.ComputerTarget(); got == target {
				t.Errorf("target after the re-aim interval = %v, want a fresh observation", got)
			}
		})
	}
}

func TestComputerMovesTwoRowsOnlyOnItsSchedule(t *testing.T) {
	t.Parallel()

	for _, test := range computerCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			const centered = (core.CourtHeight - 1) / 2.0

			model, token := computerRally(t, test.difficulty)
			lastChange := 0
			changes := 0
			for step := 1; step <= 140; step++ {
				target, aimed := model.ComputerTarget()
				if !aimed {
					target = centered
				}
				_, before := model.PaddleTops()
				model = nextComputerStep(t, model, token)
				_, after := model.PaddleTops()
				if after == before {
					continue
				}
				assertMovedToward(t, target, before, after)
				if changes > 0 && step-lastChange < test.moveEvery {
					t.Fatalf("paddle moved %d steps after its last move, want at least %d",
						step-lastChange, test.moveEvery)
				}
				lastChange = step
				changes++
			}
			if changes == 0 {
				t.Error("the computer never moved toward its first target")
			}
		})
	}
}

func TestComputerStaysCenteredThroughTheCountdown(t *testing.T) {
	t.Parallel()

	model, schedule := startComputerMatch(t, core.DifficultyHard)
	token := schedule.Timer().Token
	for range int(core.CountdownSteps) {
		model, _ = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
		if _, top := model.PaddleTops(); top != 8 {
			t.Fatalf("paddle during the countdown = %d, want the centered 8", top)
		}
	}
	if _, aimed := model.ComputerTarget(); aimed {
		t.Error("the computer aimed during a countdown, want it centered")
	}
}

func TestComputerKeepsOneErrorPerApproach(t *testing.T) {
	t.Parallel()

	model, token := computerRally(t, core.DifficultyHard)
	first := model.ComputerError()
	for step := 1; step <= 40; step++ {
		model = nextComputerStep(t, model, token)
		if got := model.ComputerError(); got != first {
			t.Fatalf("error after %d steps = %v, want the approach's %v", step, got, first)
		}
	}

	lastVX, _ := model.BallVelocity()
	for range 4000 {
		if lastVX < 0 {
			model = steerLeftPaddle(t, model)
		}
		next, effect := core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
		if effect.Kind() != core.EffectScheduleTimer {
			t.Fatalf("the rally ended before the computer's return")
		}
		vx, _ := next.BallVelocity()
		if lastVX < 0 && vx > 0 {
			if got := next.ComputerError(); got == first {
				t.Fatalf("new approach error = %v, want a fresh draw", got)
			}

			return
		}
		lastVX = vx
		model = next
	}
	t.Fatal("the left paddle never returned the computer's shot")
}

func TestPauseDiscardsTheTargetAndRestartsTheReactionDelay(t *testing.T) {
	t.Parallel()

	model, token := computerRally(t, core.DifficultyHard)
	model = advanceRally(t, model, token, 40)
	if _, aimed := model.ComputerTarget(); !aimed {
		t.Fatal("no target before the pause")
	}
	drawn := model.ComputerError()

	paused, _ := core.Update(model, core.EventPauseToggle{})
	if _, top := paused.PaddleTops(); top == 8 {
		t.Fatal("the paddle was still centered before the pause; the test needs a displaced paddle")
	}
	beforeTarget, _ := model.ComputerTarget()
	pausedTarget, _ := paused.ComputerTarget()
	if pausedTarget != beforeTarget {
		t.Error("pause discarded the target, want the computer frozen exactly")
	}
	if got := paused.ComputerError(); got != drawn {
		t.Errorf("paused error = %v, want the drawn %v", got, drawn)
	}

	resumed, effect := core.Update(paused, core.EventPauseToggle{})
	if _, aimed := resumed.ComputerTarget(); aimed {
		t.Error("resume kept the old target, want it discarded until the next observation")
	}
	if got := resumed.ComputerError(); got != drawn {
		t.Errorf("resumed error = %v, want the drawn %v", got, drawn)
	}

	counting, countdownToken := beginServingRally(t, resumed, effect.Timer().Token)
	if _, top := counting.PaddleTops(); top != 8 {
		t.Errorf("paddle after the resume countdown = %d, want it centered again", top)
	}
	if got := counting.ComputerError(); got != drawn {
		t.Errorf("error after the resume countdown = %v, want the drawn %v", got, drawn)
	}

	for step := 1; step < 12; step++ {
		counting = nextComputerStep(t, counting, countdownToken)
		if _, aimed := counting.ComputerTarget(); aimed {
			t.Fatalf("aimed %d steps after an approach restarted, want the full reaction delay", step)
		}
	}
	counting = nextComputerStep(t, counting, countdownToken)
	if _, aimed := counting.ComputerTarget(); !aimed {
		t.Error("no target after the restarted 12 step reaction delay")
	}
}

func TestRestartRepeatsTheComputerSeed(t *testing.T) {
	t.Parallel()

	started, schedule := startComputerMatch(t, core.DifficultyHard)
	served, rallyToken := beginServingRally(t, started, schedule.Timer().Token)
	drawn := served.ComputerError()
	played := advanceRally(t, served, rallyToken, 30)

	paused, _ := core.Update(played, core.EventPauseToggle{})
	restarted, effect := core.Update(focusPauseRow(t, paused, core.PauseRowRestart), core.EventPrimary{})
	replayed, replayedToken := beginServingRally(t, restarted, effect.Timer().Token)
	if got := replayed.ComputerError(); got != drawn {
		t.Fatalf("restarted approach error = %v, want the original %v", got, drawn)
	}

	original, originalToken := served, rallyToken
	for range 140 {
		original = nextComputerStep(t, original, originalToken)
		replayed = nextComputerStep(t, replayed, replayedToken)
		_, originalTop := original.PaddleTops()
		_, replayedTop := replayed.PaddleTops()
		if originalTop != replayedTop {
			t.Fatalf("restarted paddle top = %d, want the original %d", replayedTop, originalTop)
		}
		firstTarget, firstAimed := original.ComputerTarget()
		secondTarget, secondAimed := replayed.ComputerTarget()
		if firstTarget != secondTarget || firstAimed != secondAimed {
			t.Fatalf("restarted target = %v (%v), want %v (%v)",
				secondTarget, secondAimed, firstTarget, firstAimed)
		}
		if first, second := original.ComputerError(), replayed.ComputerError(); first != second {
			t.Fatalf("restarted error = %v, want %v", second, first)
		}
	}
}

func TestEachRematchGetsADifferentComputerSequence(t *testing.T) {
	t.Parallel()

	model, schedule := startComputerMatch(t, core.DifficultyHard)
	opening, token := beginServingRally(t, model, schedule.Timer().Token)
	errors := []float64{opening.ComputerError()}

	finished := playToGameOver(t, opening, token)
	for position := 1; position <= 2; position++ {
		rematched, effect := core.Update(finished, core.EventPrimary{})
		if got := rematched.ChainPosition(); got != uint8(position) {
			t.Fatalf("chain position after rematch %d = %d", position, got)
		}
		next, nextToken := beginServingRally(t, rematched, effect.Timer().Token)
		errors = append(errors, next.ComputerError())
		finished = playToGameOver(t, next, nextToken)
	}

	for first := range errors {
		for second := first + 1; second < len(errors); second++ {
			if errors[first] == errors[second] {
				t.Errorf("match %d and match %d drew the same error %v", first, second, errors[first])
			}
		}
	}
}

func TestComputerReplayReproducesStateAndEffects(t *testing.T) {
	t.Parallel()

	model, token := computerRally(t, core.DifficultyHard)
	current := model
	events := make([]core.Event, 0, 260)
	for range 100 {
		events = append(events, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
		current, _ = core.Update(current, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
	}
	events = append(events, core.EventPauseToggle{})
	current, _ = core.Update(current, core.EventPauseToggle{})
	events = append(events, core.EventPauseToggle{})
	_, resumed := core.Update(current, core.EventPauseToggle{})
	for range 150 {
		events = append(events, core.EventElapsed{Elapsed: core.StepDuration, Token: resumed.Timer().Token})
	}

	firstModel, firstEffects := replayFrom(model, events)
	secondModel, secondEffects := replayFrom(model, events)
	if firstModel != secondModel {
		t.Fatalf("replay model = %+v, want %+v", secondModel, firstModel)
	}
	if len(firstEffects) != len(secondEffects) {
		t.Fatalf("replay effects = %d, want %d", len(secondEffects), len(firstEffects))
	}
	for index := range firstEffects {
		if firstEffects[index] != secondEffects[index] {
			t.Fatalf("effect %d = %+v, want %+v", index, secondEffects[index], firstEffects[index])
		}
	}
}

func TestLocalTwoPlayerLeavesTheRightPaddleAlone(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	for range 100 {
		model = nextComputerStep(t, model, token)
	}
	if _, top := model.PaddleTops(); top != 8 {
		t.Errorf("right paddle in Local two-player = %d, want the centered 8", top)
	}
	if _, aimed := model.ComputerTarget(); aimed {
		t.Error("Local two-player produced a computer target")
	}
	if drawn := model.ComputerError(); drawn != 0 {
		t.Errorf("Local two-player drew a computer error %v, want zero", drawn)
	}
}

// assertMovedToward checks one computer movement was a single two-row step toward target. A move with the target
// already inside the paddle's four rows is a failure: the computer skips that step.
func assertMovedToward(t *testing.T, target float64, before, after uint8) {
	t.Helper()

	lastRow := float64(before) + core.PaddleHeight - 1
	switch {
	case target < float64(before):
		if after != before-2 {
			t.Fatalf("paddle moved from %d to %d toward target %v", before, after, target)
		}
	case target > lastRow:
		if after != before+2 {
			t.Fatalf("paddle moved from %d to %d toward target %v", before, after, target)
		}
	default:
		t.Fatalf("paddle moved with target %v already inside its span %d..%v", target, before, lastRow)
	}
}

// startComputerMatch starts a Single player match at difficulty and returns the opening countdown's timer request.
func startComputerMatch(t *testing.T, difficulty core.Difficulty) (core.Model, core.Effect) {
	t.Helper()

	setupModel := setupFor(t, core.ModeSinglePlayer, core.WinningScore7, difficulty)
	started, schedule := core.Update(setupModel, core.EventPrimary{})
	if started.Screen() != core.ScreenMatch || schedule.Kind() != core.EffectScheduleTimer {
		t.Fatalf("start match gave screen %v and effect %v, want a match countdown",
			started.Screen(), schedule.Kind())
	}

	return started, schedule
}

// computerRally starts a Single player match and spends the opening countdown, leaving the serve traveling toward the
// computer.
func computerRally(t *testing.T, difficulty core.Difficulty) (core.Model, core.TimerToken) {
	t.Helper()

	started, schedule := startComputerMatch(t, difficulty)

	return beginServingRally(t, started, schedule.Timer().Token)
}

// nextComputerStep spends one fixed rally step and fails when the rally ends.
func nextComputerStep(t *testing.T, model core.Model, token core.TimerToken) core.Model {
	t.Helper()

	next, effect := core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
	if next.MatchPhase() != core.MatchPhaseRally || effect.Kind() != core.EffectScheduleTimer {
		t.Fatalf("the rally ended after phase %v with effect %v", next.MatchPhase(), effect.Kind())
	}

	return next
}

// steerLeftPaddle moves the left paddle so its span covers the ball where the current velocity would cross the left
// paddle plane, reflecting through every wall as needed.
func steerLeftPaddle(t *testing.T, model core.Model) core.Model {
	t.Helper()

	x, y := model.BallPosition()
	vx, vy := model.BallVelocity()
	if vx >= 0 {
		return model
	}
	row := foldRow(y + float64(vy*((core.PaddleLeftColumn-x)/vx)))

	return setPaddleTop(t, model, false, evenTopCovering(row))
}

// foldRow folds an unbounded row into the court through every wall.
func foldRow(row float64) float64 {
	period := 2 * float64(core.CourtHeight-1)
	folded := math.Mod(row, period)
	if folded < 0 {
		folded += period
	}
	if folded > float64(core.CourtHeight-1) {
		folded = period - folded
	}

	return folded
}

// evenTopCovering returns the even paddle top whose four rows contain row.
func evenTopCovering(row float64) uint8 {
	limit := core.CourtHeight - core.PaddleHeight
	top := min(max(int(math.Floor(row/2))*2, 0), limit)
	if row > float64(top+core.PaddleHeight-1) && top+2 <= limit {
		top += 2
	}

	return uint8(top)
}

// playToGameOver spends fixed steps until the computer wins the match, keeping the human paddle still.
func playToGameOver(t *testing.T, model core.Model, token core.TimerToken) core.Model {
	t.Helper()

	for range 20000 {
		if model.MatchPhase() == core.MatchPhaseGameOver {
			return model
		}
		var effect core.Effect
		model, effect = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
		if effect.Kind() == core.EffectScheduleTimer {
			token = effect.Timer().Token
		}
	}
	t.Fatal("the computer match never ended")

	return model
}
