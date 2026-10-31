package core_test

import (
	"slices"
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
}

// computerCases returns the three profiles at 120 fixed steps per second.
func computerCases() []computerCase {
	return []computerCase{
		{name: "easy", difficulty: core.DifficultyEasy, reaction: 48, reaim: 30, moveEvery: 20},
		{name: "normal", difficulty: core.DifficultyNormal, reaction: 24, reaim: 18, moveEvery: 12},
		{name: "hard", difficulty: core.DifficultyHard, reaction: 12, reaim: 12, moveEvery: 8},
	}
}

// computerTrajectory is one replayed step's player-visible state: the ball's court position, both paddle tops, and the
// score. Comparing whole trajectories is how these tests observe the computer's hidden aim and error draws.
type computerTrajectory struct {
	x, y        float64
	left, right uint8
	leftScore   uint8
	rightScore  uint8
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
}

func TestComputerWaitsForItsReactionDelayBeforeMoving(t *testing.T) {
	t.Parallel()

	for _, test := range computerCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, token := computerRally(t, test.difficulty)
			for step := 1; step <= test.reaction; step++ {
				model = nextComputerStep(t, model, token)
				if _, top := model.PaddleTops(); top != 8 {
					t.Fatalf("paddle moved to %d within the %d step reaction delay", top, test.reaction)
				}
			}
			if test.difficulty == core.DifficultyEasy {
				// Easy observes the row the ball has reached, which can fall inside its centered span, so this seed's
				// first approach may need no movement at all.
				return
			}
			for range test.moveEvery {
				model = nextComputerStep(t, model, token)
				if _, top := model.PaddleTops(); top != 8 {
					return
				}
			}
			t.Fatal("the paddle never moved toward the observed approach")
		})
	}
}

func TestComputerMovesTwoRowsOnlyOnItsSchedule(t *testing.T) {
	t.Parallel()

	// Easy observes a row that can already lie inside its centered span, so its first approach may stay still.
	for _, test := range computerCases()[1:] {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, token := computerRally(t, test.difficulty)
			_, previous := model.PaddleTops()
			lastMove := 0
			changes := 0
			for step := 1; step <= 140; step++ {
				model = nextComputerStep(t, model, token)
				_, top := model.PaddleTops()
				if top == previous {
					continue
				}
				assertComputerMoved(t, previous, top)
				if changes > 0 && step-lastMove < test.moveEvery {
					t.Fatalf("paddle moved %d steps after its last move, want at least %d",
						step-lastMove, test.moveEvery)
				}
				lastMove, previous = step, top
				changes++
			}
			if changes == 0 {
				t.Error("the computer never moved toward its first targets")
			}
		})
	}
}

// assertComputerMoved checks one computer movement was exactly two rows inside the court.
func assertComputerMoved(t *testing.T, before, after uint8) {
	t.Helper()

	if after%2 != 0 || after > 16 {
		t.Fatalf("paddle top after a move = %d, want an even row inside the court", after)
	}
	if difference := int(after) - int(before); difference != 2 && difference != -2 {
		t.Fatalf("paddle moved from %d to %d, want exactly two rows", before, after)
	}
}

func TestComputerSettlesOnOneRowWithinAnApproach(t *testing.T) {
	t.Parallel()

	test := computerCases()[2] // Hard, whose first target always lies outside the centered span.
	model, token := computerRally(t, test.difficulty)
	lastTop := uint8(8)
	lastChange := 0
	for step := 1; step <= 400; step++ {
		beforeX, _ := model.BallPosition()
		model = nextComputerStep(t, model, token)
		afterX, _ := model.BallPosition()
		_, top := model.PaddleTops()
		if top != lastTop {
			lastTop = top
			lastChange = step
		}

		// The paddle returns the serve when the ball's horizontal travel reverses.
		if afterX >= beforeX {
			continue
		}
		if step-lastChange < test.moveEvery {
			t.Fatalf("paddle last moved %d steps before the return, want it settled at least %d",
				step-lastChange, test.moveEvery)
		}
		if left, right := model.Score(); left != 0 || right != 0 {
			t.Fatalf("score after the return = %d:%d, want the serve returned", left, right)
		}

		return
	}
	t.Fatal("the ball never returned from the computer's paddle")
}

func TestPauseAndResumeRestartTheReactionDelay(t *testing.T) {
	t.Parallel()

	test := computerCases()[2]
	model, token := computerRally(t, test.difficulty)
	model = advanceRally(t, model, token, 40)
	paused, _ := core.Update(model, core.EventPauseToggle{})
	resumed, effect := core.Update(paused, core.EventPauseToggle{})
	counting, countdownToken := beginServingRally(t, resumed, effect.Timer().Token)
	if _, top := counting.PaddleTops(); top != 8 {
		t.Fatalf("paddle after the resume countdown = %d, want it centered again", top)
	}

	for step := 1; step <= test.reaction; step++ {
		counting = nextComputerStep(t, counting, countdownToken)
		if _, top := counting.PaddleTops(); top != 8 {
			t.Fatalf("paddle moved to %d within the restarted %d step reaction delay", top, test.reaction)
		}
	}
	for range test.moveEvery {
		counting = nextComputerStep(t, counting, countdownToken)
		if _, top := counting.PaddleTops(); top != 8 {
			return
		}
	}
	t.Fatal("the resumed approach never re-aimed")
}

func TestRestartReplaysTheComputerTrajectory(t *testing.T) {
	t.Parallel()

	started, schedule := startComputerMatch(t, core.DifficultyHard)
	served, rallyToken := beginServingRally(t, started, schedule.Timer().Token)
	played := advanceRally(t, served, rallyToken, 30)

	paused, _ := core.Update(played, core.EventPauseToggle{})
	restarted, effect := core.Update(focusPauseRow(t, paused, core.PauseRowRestart), core.EventPrimary{})
	replayed, replayedToken := beginServingRally(t, restarted, effect.Timer().Token)

	originalTrajectory := matchTrajectory(t, served, rallyToken, 140)
	restartedTrajectory := matchTrajectory(t, replayed, replayedToken, 140)
	if !slices.Equal(originalTrajectory, restartedTrajectory) {
		t.Fatalf("restarted trajectory differs from the original at step %d",
			firstDifference(originalTrajectory, restartedTrajectory))
	}
}

func TestRematchesGetADifferentButReproducibleComputerSequence(t *testing.T) {
	t.Parallel()

	openingA := computerSequence(t, 0)
	openingB := computerSequence(t, 0)
	if !slices.Equal(openingA, openingB) {
		t.Fatal("the same match replayed a different trajectory")
	}

	rematchA := computerSequence(t, 1)
	rematchB := computerSequence(t, 1)
	if !slices.Equal(rematchA, rematchB) {
		t.Fatal("the same rematch replayed a different trajectory")
	}
	if slices.Equal(openingA, rematchA) {
		t.Fatal("the first rematch replayed the opening match's trajectory")
	}

	secondRematch := computerSequence(t, 2)
	if slices.Equal(rematchA, secondRematch) {
		t.Fatal("the second rematch replayed the first rematch's trajectory")
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
}

// computerSequence returns the first steps of a Hard match's trajectory after the given number of rematches. It builds
// the chain from a fresh match each time, so two calls replay the same seeds.
func computerSequence(t *testing.T, rematches int) []computerTrajectory {
	t.Helper()

	model, schedule := startComputerMatch(t, core.DifficultyHard)
	model, token := beginServingRally(t, model, schedule.Timer().Token)
	for range rematches {
		var effect core.Effect
		model = playToGameOver(t, model, token)
		model, effect = core.Update(model, core.EventPrimary{})
		if effect.Kind() != core.EffectScheduleTimer {
			t.Fatalf("rematch requested %v, want the countdown's timer", effect.Kind())
		}
		model, token = beginServingRally(t, model, effect.Timer().Token)
	}

	return matchTrajectory(t, model, token, 300)
}

// matchTrajectory spends fixed steps, following the timing token the match issues, and records what the player sees.
func matchTrajectory(t *testing.T, model core.Model, token core.TimerToken, steps int) []computerTrajectory {
	t.Helper()

	trajectory := make([]computerTrajectory, 0, steps+1)
	for range steps + 1 {
		leftScore, rightScore := model.Score()
		x, y := model.BallPosition()
		leftTop, rightTop := model.PaddleTops()
		trajectory = append(trajectory, computerTrajectory{
			x: x, y: y, left: leftTop, right: rightTop, leftScore: leftScore, rightScore: rightScore,
		})
		var effect core.Effect
		model, effect = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
		if effect.Kind() == core.EffectScheduleTimer {
			token = effect.Timer().Token
		}
	}

	return trajectory
}

// firstDifference returns the index of the first trajectory step that differs, or -1.
func firstDifference(one, other []computerTrajectory) int {
	for index := range one {
		if one[index] != other[index] {
			return index
		}
	}

	return -1
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
