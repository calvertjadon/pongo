package core

import "testing"

func TestSplitMix64KnownAnswer(t *testing.T) {
	t.Parallel()

	var random generator
	if got := random.next(); got != 0xE220A8397B1DCDAF {
		t.Fatalf("first SplitMix64 output from state 0 = %#x, want the published 0xE220A8397B1DCDAF", got)
	}
}

func TestPredictionDiffersByProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		vy     float64
		easy   float64
		normal float64
		hard   float64
	}{
		{name: "straight to the paddle", vy: 3, easy: 9.5, normal: 14.375, hard: 14.375},
		{name: "one wall", vy: -10, easy: 9.5, normal: 6.75, hard: 6.75},
		{name: "more than one wall", vy: -100, easy: 9.5, normal: 19, hard: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			current := ball{x: 34.5, y: 9.5, vx: 20, vy: test.vy}
			if got := predictedRow(profileFor(DifficultyEasy), current); got != test.easy {
				t.Errorf("Easy prediction = %v, want the observed row %v", got, test.easy)
			}
			if got := predictedRow(profileFor(DifficultyNormal), current); got != test.normal {
				t.Errorf("Normal prediction = %v, want %v", got, test.normal)
			}
			if got := predictedRow(profileFor(DifficultyHard), current); got != test.hard {
				t.Errorf("Hard prediction = %v, want %v", got, test.hard)
			}
		})
	}
}

func TestErrorOffsetSpansTheProfileAmplitude(t *testing.T) {
	t.Parallel()

	for _, difficulty := range []Difficulty{DifficultyEasy, DifficultyNormal, DifficultyHard} {
		t.Run(difficultyName(difficulty), func(t *testing.T) {
			t.Parallel()

			profile := profileFor(difficulty)
			random := seededComputer(0, difficulty)
			low, high := 0.0, 0.0
			for range 1000 {
				drawn := random.offset(profile.errorRows)
				if drawn < -profile.errorRows || drawn >= profile.errorRows {
					t.Fatalf("error %v outside [-%v, %v)", drawn, profile.errorRows, profile.errorRows)
				}
				low = min(low, drawn)
				high = max(high, drawn)
			}
			if low > -0.99*profile.errorRows || high < 0.99*profile.errorRows {
				t.Errorf("1000 draws span [%v, %v], want close to ±%v", low, high, profile.errorRows)
			}
		})
	}
}

func TestSeedVariesByPositionAndDifficulty(t *testing.T) {
	t.Parallel()

	seen := make(map[generator]bool)
	for _, difficulty := range []Difficulty{DifficultyEasy, DifficultyNormal, DifficultyHard} {
		for position := range uint32(4) {
			seed := seededComputer(position, difficulty)
			if seen[seed] {
				t.Fatalf("seed for position %d and difficulty %v repeats", position, difficulty)
			}
			seen[seed] = true
		}
	}
}

// TestTargetWithoutAnObservationIsTheCourtCenter pins the fallback row moveComputer reads when the computer has not
// sampled the ball yet: the court's vertical center.
func TestTargetWithoutAnObservationIsTheCourtCenter(t *testing.T) {
	t.Parallel()

	if got := (computer{}).target(); got != 9.5 {
		t.Errorf("target without an observation = %v, want the court's vertical center 9.5", got)
	}
}

// TestEndingOrSuspendingAnApproachDropsTheTarget pins that no path back to a centered computer leaves the last aim in
// place.
func TestEndingOrSuspendingAnApproachDropsTheTarget(t *testing.T) {
	t.Parallel()

	profile := profileFor(DifficultyHard)
	tests := []struct {
		name    string
		restart func(*computer)
	}{
		{name: "begin", restart: func(current *computer) { current.beginApproach(profile) }},
		{name: "suspend", restart: func(current *computer) { current.suspend() }},
		{name: "resume inbound", restart: func(current *computer) { current.resume(true) }},
		{name: "resume outbound", restart: func(current *computer) { current.resume(false) }},
		{name: "end", restart: func(current *computer) { current.endApproach() }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			aimed := computer{aim: 14, aimed: true, approach: true, approachSteps: 6, error: 2}
			test.restart(&aimed)
			if got := aimed.target(); got != computerCenterRow {
				t.Errorf("target after %s = %v, want the court center %v", test.name, got, computerCenterRow)
			}
		})
	}
}

// TestARestartedApproachSpendsTheFullReactionDelay pins that beginning, suspending, or resuming an approach always
// starts its observation schedule over instead of continuing a stale step count.
func TestARestartedApproachSpendsTheFullReactionDelay(t *testing.T) {
	t.Parallel()

	profile := profileFor(DifficultyHard)
	tests := []struct {
		name    string
		restart func(*computer)
	}{
		{name: "begin", restart: func(current *computer) { current.beginApproach(profile) }},
		{name: "suspend", restart: func(current *computer) { current.suspend() }},
		{name: "resume inbound", restart: func(current *computer) { current.resume(true) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stale := computer{approach: true, approachSteps: profile.reaction}
			test.restart(&stale)
			if stale.due(profile) {
				t.Errorf("a %s approach reports a scheduled observation before its reaction delay", test.name)
			}
		})
	}
}

// TestTheObservationScheduleIsTheReactionDelayThenEveryReaim pins when the computer samples the ball: never before the
// reaction delay, then exactly at the reaction delay and at each later re-aim interval. It uses Easy because its
// reaction delay and re-aim interval differ, so the first sample cannot be confused with a multiple of the interval.
func TestTheObservationScheduleIsTheReactionDelayThenEveryReaim(t *testing.T) {
	t.Parallel()

	easy := profileFor(DifficultyEasy)
	hard := profileFor(DifficultyHard)
	tests := []struct {
		name    string
		steps   Steps
		profile computerProfile
		want    bool
	}{
		{name: "easy just before the reaction delay", steps: 47, profile: easy},
		{name: "easy at the reaction delay", steps: 48, profile: easy, want: true},
		{name: "easy one step past the reaction delay", steps: 49, profile: easy},
		{name: "easy just before the first re-aim", steps: 77, profile: easy},
		{name: "easy at the first re-aim", steps: 78, profile: easy, want: true},
		{name: "easy at the second re-aim", steps: 108, profile: easy, want: true},
		{name: "hard before the reaction delay", steps: 11, profile: hard},
		{name: "hard at the reaction delay", steps: 12, profile: hard, want: true},
		{name: "hard at the next re-aim", steps: 24, profile: hard, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			observer := computer{approachSteps: test.steps}
			if got := observer.due(test.profile); got != test.want {
				t.Errorf("due at step %d = %v, want %v", test.steps, got, test.want)
			}
		})
	}
}

// TestReflectOnceReflectsAtMostOneWallAndClamps pins the one-wall prediction: a row reflects off the wall it crossed,
// then clamps to the nearest court row when the reflected line would leave the court again.
func TestReflectOnceReflectsAtMostOneWallAndClamps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		row  float64
		want float64
	}{
		{name: "above the top wall reflects onto it", row: -3, want: 3},
		{name: "on the top row is unchanged", row: 0, want: 0},
		{name: "on the bottom row is unchanged", row: 19, want: 19},
		{name: "one row past the bottom wall reflects back", row: 20, want: 18},
		{name: "past the bottom wall reflects once", row: 25, want: 13},
		{name: "needing a second reflection clamps to the top", row: 100, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := reflectOnce(test.row); got != test.want {
				t.Errorf("reflectOnce(%v) = %v, want %v", test.row, got, test.want)
			}
		})
	}
}

// TestReflectEveryFoldsThroughEveryWall pins the full-fold prediction: rows repeat every twice the court height minus
// one, and each half-period mirrors the other.
func TestReflectEveryFoldsThroughEveryWall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		row  float64
		want float64
	}{
		{name: "above the top wall folds down", row: -1, want: 1},
		{name: "on the top row is unchanged", row: 0, want: 0},
		{name: "on the bottom row is unchanged", row: 19, want: 19},
		{name: "one row past the bottom wall folds back", row: 20, want: 18},
		{name: "one full period returns to the top row", row: 38, want: 0},
		{name: "one period and a row returns to row one", row: 39, want: 1},
		{name: "one period above the top wall folds down", row: -39, want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := reflectEvery(test.row); got != test.want {
				t.Errorf("reflectEvery(%v) = %v, want %v", test.row, got, test.want)
			}
		})
	}
}

// TestMoveComputerStepsOnlyForTargetsOutsideThePaddleSpan pins the movement rule: a target above the paddle's top row
// steps up, a target below its last row steps down, and a target inside the four covered rows leaves it alone.
func TestMoveComputerStepsOnlyForTargetsOutsideThePaddleSpan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target float64
		want   uint8
	}{
		{name: "above the top row steps up", target: 6.5, want: 6},
		{name: "exactly the top row stays", target: 8, want: 8},
		{name: "inside the span stays", target: 10.9, want: 8},
		{name: "exactly the last row stays", target: 11, want: 8},
		{name: "past the last row steps down", target: 11.1, want: 10},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			match := matchState{
				settings: Settings{Mode: ModeSinglePlayer, Difficulty: DifficultyHard},
				paddles:  paddles{right: 8},
			}
			match.moveComputer(test.target)
			if _, got := (Model{state: match}).PaddleTops(); got != test.want {
				t.Errorf("paddle top after a target at %v = %d, want %d", test.target, got, test.want)
			}
		})
	}
}

// TestAnEndedApproachDrawsAFreshErrorWhenTheBallReturns pins that a ball moving away ends the approach, so the next
// inbound approach draws its own error instead of sampling with the cleared one.
func TestAnEndedApproachDrawsAFreshErrorWhenTheBallReturns(t *testing.T) {
	t.Parallel()

	profile := profileFor(DifficultyHard)
	random := seededComputer(0, DifficultyHard)
	random.offset(profile.errorRows) // the first approach's draw
	fresh := random.offset(profile.errorRows)

	match := inboundComputerMatch(DifficultyHard)
	match.observeComputer()
	match.ball.vx = -20
	match.observeComputer()
	match.ball.vx = 20
	for range 2 * int(profile.reaction) {
		match.observeComputer()
	}

	if got, want := match.computer.target(), 5+fresh; got != want {
		t.Errorf("target after the ball returned = %v, want the fresh error's target %v", got, want)
	}
}

// TestAResumedInboundApproachSamplesWithItsDrawnError pins that the error drawn when an approach started survives the
// pause and the reaction delay restart.
func TestAResumedInboundApproachSamplesWithItsDrawnError(t *testing.T) {
	t.Parallel()

	profile := profileFor(DifficultyHard)
	random := seededComputer(0, DifficultyHard)
	first := random.offset(profile.errorRows)

	match := inboundComputerMatch(DifficultyHard)
	match.observeComputer()
	match.computer.suspend()
	match.computer.resume(true)
	for range 2 * int(profile.reaction) {
		match.observeComputer()
	}

	if got, want := match.computer.target(), 5+first; got != want {
		t.Errorf("target after resuming = %v, want the approach's error target %v", got, want)
	}
}

// TestStartServeApproachFollowsTheServeDirection pins that a serve toward the computer begins the approach at once,
// while a serve away leaves the computer centered.
func TestStartServeApproachFollowsTheServeDirection(t *testing.T) {
	t.Parallel()

	profile := profileFor(DifficultyHard)
	random := seededComputer(0, DifficultyHard)
	first := random.offset(profile.errorRows)

	t.Run("toward the computer begins the approach", func(t *testing.T) {
		t.Parallel()

		match := inboundComputerMatch(DifficultyHard)
		match.serve.Horizontal = DirectionRight
		match.startServeApproach()
		for range int(profile.reaction) {
			match.observeComputer()
		}
		if got, want := match.computer.target(), 5+first; got != want {
			t.Errorf("target at the reaction delay = %v, want the serve's error target %v", got, want)
		}
	})
	t.Run("away leaves the computer centered", func(t *testing.T) {
		t.Parallel()

		match := inboundComputerMatch(DifficultyHard)
		match.computer = computer{aim: 14, aimed: true, approach: true, approachSteps: 6}
		match.serve.Horizontal = DirectionLeft
		match.startServeApproach()
		if got := match.computer.target(); got != computerCenterRow {
			t.Errorf("target after a serve away = %v, want the court center %v", got, computerCenterRow)
		}
	})
}

// TestResumeComputerClearsTheTargetOnResuming pins that resuming a paused rally drops the aim: the reaction delay
// restarts before the computer samples the ball again.
func TestResumeComputerClearsTheTargetOnResuming(t *testing.T) {
	t.Parallel()

	match := inboundComputerMatch(DifficultyHard)
	match.computer = computer{aim: 14, aimed: true, approach: true, approachSteps: 6}
	match.resumeComputer()
	if got := match.computer.target(); got != computerCenterRow {
		t.Errorf("target after resuming = %v, want the court center %v", got, computerCenterRow)
	}
}

// TestResumeComputerEndsTheApproachForAnOutboundBall pins that resuming after the ball turned away clears the approach,
// so the next inbound approach draws a fresh error.
func TestResumeComputerEndsTheApproachForAnOutboundBall(t *testing.T) {
	t.Parallel()

	profile := profileFor(DifficultyHard)
	random := seededComputer(0, DifficultyHard)
	random.offset(profile.errorRows) // the first approach's draw
	fresh := random.offset(profile.errorRows)

	match := inboundComputerMatch(DifficultyHard)
	match.observeComputer()
	match.ball.vx = -20
	match.resumeComputer()
	match.ball.vx = 20
	for range 2 * int(profile.reaction) {
		match.observeComputer()
	}

	if got, want := match.computer.target(), 5+fresh; got != want {
		t.Errorf("target after the ball returned = %v, want the fresh error's target %v", got, want)
	}
}

// inboundComputerMatch returns a Single player match whose ball sits on row 5 and travels toward the computer, so every
// prediction for the match is row 5.
func inboundComputerMatch(difficulty Difficulty) matchState {
	return matchState{
		settings: Settings{Mode: ModeSinglePlayer, Difficulty: difficulty},
		ball:     ball{x: 30, y: 5, vx: 20, speed: serveSpeed},
		computer: computer{random: seededComputer(0, difficulty)},
	}
}

// difficultyName names a difficulty for a subtest.
func difficultyName(difficulty Difficulty) string {
	switch difficulty {
	case DifficultyEasy:
		return "easy"
	case DifficultyNormal:
		return "normal"
	case DifficultyHard:
		return "hard"
	}

	return "unknown"
}
