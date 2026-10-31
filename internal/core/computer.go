package core

import "math"

// The computer player controls the right paddle in Single player mode. Its schedule, error, and prediction come from a
// fixed difficulty profile, and its error draws come from a hand-written generator seeded per rematch chain, so
// identical state and events always replay identically.
const (
	// computerSeedMaterial is the fixed material every rematch chain's seeds derive from. Changing it changes every
	// replay.
	computerSeedMaterial uint64 = 0x504F4E474F207631
	// computerCenterRow is the row the computer aims at when it has no target: the court's vertical center. Paddle tops
	// are always even and the center sits inside the centered span, so centering settles exactly.
	computerCenterRow = (CourtHeight - 1) / 2.0
	// paddleLastOffset is the offset from a paddle's top row to the last row its four cells cover.
	paddleLastOffset = PaddleHeight - 1
	// splitMixStateStep is SplitMix64's state increment.
	splitMixStateStep uint64 = 0x9E3779B97F4A7C15
	// splitMixMultiplierOne and splitMixMultiplierTwo are SplitMix64's mixing multipliers.
	splitMixMultiplierOne uint64 = 0xBF58476D1CE4E5B9
	splitMixMultiplierTwo uint64 = 0x94D049BB133111EB
	// splitMixShiftOne, splitMixShiftTwo, and splitMixShiftThree are SplitMix64's mixing shifts.
	splitMixShiftOne   = 30
	splitMixShiftTwo   = 27
	splitMixShiftThree = 31
	// unitFractionScale turns the generator's top 53 bits into a fraction in [0, 1).
	unitFractionScale = 1.0 / (1 << 53)
)

// predictionKind tags how a profile predicts the ball's row where it meets the computer's paddle plane.
type predictionKind uint8

const (
	// predictionObserved uses the ball's observed row with no prediction.
	predictionObserved predictionKind = iota
	// predictionOneWall reflects the path at most once, then extends and clamps to the nearest court boundary.
	predictionOneWall
	// predictionEveryWall reflects the path through every wall it would meet.
	predictionEveryWall
)

// computerProfile is one difficulty's complete behavior: how long it waits before its first look, how often it
// re-aims, how often it may move, how far its error can stray, and how it predicts.
type computerProfile struct {
	reaction   Steps
	reaim      Steps
	moveEvery  Steps
	errorRows  float64
	prediction predictionKind
}

// profileFor returns the fixed behavior of one difficulty.
func profileFor(difficulty Difficulty) computerProfile {
	switch difficulty {
	case DifficultyEasy:
		return computerProfile{
			reaction:   48,
			reaim:      30,
			moveEvery:  20,
			errorRows:  4,
			prediction: predictionObserved,
		}
	case DifficultyNormal:
		return computerProfile{
			reaction:   24,
			reaim:      18,
			moveEvery:  12,
			errorRows:  3,
			prediction: predictionOneWall,
		}
	case DifficultyHard:
		return computerProfile{
			reaction:   12,
			reaim:      12,
			moveEvery:  8,
			errorRows:  2.25,
			prediction: predictionEveryWall,
		}
	}

	return computerProfile{}
}

// generator is the computer player's deterministic pseudo-random generator, SplitMix64 (Steele, Lea & Flood 2014).
// Its constants and mixing are core behavior: changing them changes every replay.
type generator uint64

// next advances the generator and returns its next output.
func (random *generator) next() uint64 {
	*random += generator(splitMixStateStep)
	mixed := uint64(*random)
	mixed = (mixed ^ (mixed >> splitMixShiftOne)) * splitMixMultiplierOne
	mixed = (mixed ^ (mixed >> splitMixShiftTwo)) * splitMixMultiplierTwo

	return mixed ^ (mixed >> splitMixShiftThree)
}

// offset draws one uniform row offset in [-amplitude, +amplitude).
func (random *generator) offset(amplitude float64) float64 {
	top := random.next() >> 11
	unit := float64(top) * unitFractionScale
	signed := unit + unit - 1

	return float64(signed * amplitude)
}

// seededComputer returns the generator a match at the given zero-based chain position and difficulty draws from. A
// restart rebuilds the same seed and a rematch advances the position.
func seededComputer(position uint8, difficulty Difficulty) generator {
	return generator(computerSeedMaterial ^ (uint64(position) << 8) ^ uint64(difficulty))
}

// computer is the computer player's state inside a match. It holds the generator, the error drawn for the current
// inbound approach, the latest target, and the approach's step count. Local two-player leaves it zero.
type computer struct {
	random        generator
	error         float64
	aim           float64
	aimed         bool
	approach      bool
	approachSteps Steps
}

// openedComputer returns the computer player a match starts with.
func openedComputer(settings Settings, origin chain) computer {
	if settings.Mode != ModeSinglePlayer {
		return computer{}
	}

	return computer{random: seededComputer(origin.position, settings.Difficulty)}
}

// target returns the row the paddle should cover, or the court center when no observation has set a target.
func (computer computer) target() float64 {
	if computer.aimed {
		return computer.aim
	}

	return computerCenterRow
}

// beginApproach starts an inbound approach and draws the one error offset that applies to every target in it.
func (computer *computer) beginApproach(profile computerProfile) {
	computer.approach = true
	computer.approachSteps = 0
	computer.aimed = false
	computer.error = computer.random.offset(profile.errorRows)
}

// endApproach clears the approach and everything drawn for it.
func (computer *computer) endApproach() {
	computer.approach = false
	computer.approachSteps = 0
	computer.aim = 0
	computer.aimed = false
	computer.error = 0
}

// due reports whether this approach step is a scheduled observation: the first reaction delay after the approach
// starts, then every re-aim interval.
func (computer computer) due(profile computerProfile) bool {
	if computer.approachSteps < profile.reaction {
		return false
	}

	return (computer.approachSteps-profile.reaction)%profile.reaim == 0
}

// interceptRow returns the ball's row where it meets the computer's paddle plane if its current velocity carried it
// straight there.
func interceptRow(current ball) float64 {
	time := (PaddleRightColumn - current.x) / current.vx
	travel := float64(current.vy * time)

	return current.y + travel
}

// reflectOnce reflects a straight-line row off at most one wall. A row that would need another reflection keeps the
// once-reflected line and clamps to the nearest court boundary.
func reflectOnce(row float64) float64 {
	switch {
	case row < 0:
		row = -row
	case row > CourtHeight-1:
		row = 2*(CourtHeight-1) - row
	}

	return min(max(row, 0), float64(CourtHeight-1))
}

// reflectEvery folds a straight-line row through every wall between the ball and the paddle plane. The walls are the
// first and last court rows, so the path repeats every twice the court height minus one.
func reflectEvery(row float64) float64 {
	const period = 2 * (CourtHeight - 1)
	folded := math.Mod(row, period)
	if folded < 0 {
		folded += period
	}
	if folded > CourtHeight-1 {
		folded = period - folded
	}

	return folded
}

// predictedRow returns the row a profile predicts at the computer's paddle plane for the ball's current public state.
func predictedRow(profile computerProfile, current ball) float64 {
	switch profile.prediction {
	case predictionObserved:
		return current.y
	case predictionOneWall:
		return reflectOnce(interceptRow(current))
	case predictionEveryWall:
		return reflectEvery(interceptRow(current))
	}

	return current.y
}

// moveComputer spends one fixed match step of the computer's schedule. On a movement opportunity it takes at most one
// two-row step toward target, and it does not move when the target already lies inside the paddle's rows.
func (match *matchState) moveComputer(target float64) {
	match.stepCount++
	if match.settings.Mode != ModeSinglePlayer {
		return
	}

	profile := profileFor(match.settings.Difficulty)
	if match.stepCount%uint32(profile.moveEvery) != 0 {
		return
	}

	top := match.paddles.right
	switch {
	case target < float64(top):
		match.paddles.right = stepped(top, DirectionUp)
	case target > float64(top+paddleLastOffset):
		match.paddles.right = stepped(top, DirectionDown)
	}
}

// observeComputer updates the approach after one fixed step moved the ball. A ball now moving away ends the approach;
// a ball moving toward the computer without one starts it and draws its error; otherwise the approach counts the step
// and samples the public match state at its scheduled moments.
func (match *matchState) observeComputer() {
	if match.settings.Mode != ModeSinglePlayer {
		return
	}

	computer := &match.computer
	profile := profileFor(match.settings.Difficulty)
	switch {
	case match.ball.vx < 0:
		computer.endApproach()
	case !computer.approach:
		computer.beginApproach(profile)
	default:
		computer.approachSteps++
		if computer.due(profile) {
			computer.aim = float64(predictedRow(profile, match.ball) + computer.error)
			computer.aimed = true
		}
	}
}

// startServeApproach begins the approach when a launched serve travels toward the computer; a serve traveling away
// leaves the computer centered.
func (match *matchState) startServeApproach() {
	if match.settings.Mode != ModeSinglePlayer {
		return
	}
	if match.serve.Horizontal == DirectionRight {
		match.computer.beginApproach(profileFor(match.settings.Difficulty))

		return
	}
	match.computer.endApproach()
}

// resumeComputer restarts the computer's reaction delay when a paused inbound approach resumes. The error drawn for
// the approach survives the pause; the target does not.
func (match *matchState) resumeComputer() {
	if match.settings.Mode != ModeSinglePlayer {
		return
	}
	if match.ball.vx <= 0 {
		match.computer.endApproach()

		return
	}
	match.computer.approach = true
	match.computer.approachSteps = 0
	match.computer.aimed = false
}

// ComputerTarget returns the row the computer player aims its paddle span at and whether it currently has a target.
// Without a target it moves toward the court center. It is meaningful only while Screen is ScreenMatch in Single
// player mode.
func (model Model) ComputerTarget() (row float64, aimed bool) {
	if match, ok := model.state.(matchState); ok {
		return match.computer.aim, match.computer.aimed
	}

	return 0, false
}

// ComputerError returns the row offset drawn for the computer player's current inbound approach, or zero when no
// approach is active. The same offset applies to every target until the approach ends. It is meaningful only while
// Screen is ScreenMatch in Single player mode.
func (model Model) ComputerError() float64 {
	if match, ok := model.state.(matchState); ok {
		return match.computer.error
	}

	return 0
}
