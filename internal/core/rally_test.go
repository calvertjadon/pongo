package core_test

import (
	"math"
	"slices"
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

// Rally worked values. A serve leaves the court center at 28 cells/s on a 20 degree visual angle; court cells are
// twice as tall as they are wide, so a visual angle θ has the velocity (speed·cos θ, speed·sin θ / 2).
const (
	serveVX = 26.311393382005434
	serveVY = 4.788282006559362
)

// rallyTops returns a reachable paddle sequence: the right paddle takes the serve near its center, the left returns
// it, and the right then takes it on the paddle's top row with a steep angle that sends the ball off the top and
// bottom walls before the rally reaches the speed cap.
func rallyTops() []uint8 {
	return []uint8{2, 4, 10, 10, 16, 14, 2}
}

func TestRallyServesAtTheOpeningSpeedAndAngle(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	if x, y := model.BallPosition(); x != 34.5 || y != 9.5 {
		t.Errorf("served ball = (%v, %v), want the court center (34.5, 9.5)", x, y)
	}
	vx, vy := stepVelocity(t, model, token)
	assertClose(t, "serve vx", vx, serveVX)
	assertClose(t, "serve vy", vy, -serveVY)
	assertClose(t, "serve speed", math.Hypot(vx, 2*vy), 28)
	if serve := model.NextServe(); serve != (core.Serve{Horizontal: core.DirectionRight, Vertical: core.DirectionUp}) {
		t.Errorf("opening serve = %+v, want up and toward the right player", serve)
	}
}

func TestBallMovesOneFixedStepOfItsVelocity(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	afterOne := advanceRally(t, model, token, 1)
	assertBallStep(t, afterOne, 1)
	afterNine := advanceRally(t, afterOne, token, 8)
	afterTen := advanceRally(t, afterNine, token, 1)
	assertBallStep(t, afterTen, 10)
	x9, y9 := afterNine.BallPosition()
	x10, y10 := afterTen.BallPosition()
	vx, vy := (x10-x9)*stepsPerSecond, (y10-y9)*stepsPerSecond
	assertClose(t, "free-flight vx", vx, serveVX)
	assertClose(t, "free-flight vy", vy, -serveVY)
	if math.Abs(math.Hypot(vx, 2*vy)-28) > 1e-9 {
		t.Errorf("free flight changed the speed to %v, want 28", math.Hypot(vx, 2*vy))
	}
}

func TestElapsedBeyondTheCatchUpLimitAdvancesNineStepsAtMost(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	capped, effect := core.Update(model, core.EventElapsed{Elapsed: 10 * core.ElapsedLimit, Token: token})
	assertScheduled(t, effect, token)
	assertBallStep(t, capped, 9)

	next := advanceRally(t, capped, token, 1)
	assertBallStep(t, next, 10)
}

func TestPaddleReboundsMapTheImpactRowToTheAngle(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	_, speeds, velocities := driveHits(t, model, token, rallyTops()[:3])
	if want := []float64{31, 34, 37}; !almostEqualSlice(speeds, want) {
		t.Errorf("speeds after the first three hits = %v, want %v", speeds, want)
	}

	// A center hit returns the ball almost flat and away from the paddle.
	vx, vy := velocities[2][0], velocities[2][1]
	assertClose(t, "end-row vx", vx, -24.552058173591295)
	assertClose(t, "end-row vy", vy, -13.840126800725027)
	angle := math.Atan2(2*vy, -vx) * 180 / math.Pi
	wantAngle := (10.382445702757567 - 11.5) / 1.5 * 65
	if difference := math.Abs(angle - wantAngle); difference > 1e-6 {
		t.Errorf("end-row rebound angle = %v degrees, want the linear map's %v", angle, wantAngle)
	}
}

func TestPaddleHitsAccelerateAndCapAtFortyFour(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	_, speeds, _ := driveHits(t, model, token, rallyTops())
	want := []float64{31, 34, 37, 40, 43, 44, 44}
	if !almostEqualSlice(speeds, want) {
		t.Errorf("hit speeds = %v, want %v", speeds, want)
	}
}

func TestBallReboundsOffWallsWithoutChangingSpeed(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	afterHits, _, velocities := driveHits(t, model, token, rallyTops()[:3])
	vx, vy := velocities[2][0], velocities[2][1]
	speed := math.Hypot(vx, 2*vy)
	if vy >= 0 {
		t.Fatalf("ball after the steep hit moves down, want toward the top wall")
	}

	// The vertical step reverses at the top wall; two equal steps after that give the reflected velocity.
	bounced, bouncedVX, bouncedVY := wallBounce(t, afterHits, token)
	assertClose(t, "reflected vx", bouncedVX, vx)
	assertClose(t, "reflected vy", bouncedVY, -vy)
	if x, y := bounced.BallPosition(); x < 0 || x > core.CourtWidth-1 || y < 0 || y > core.CourtHeight-1 {
		t.Errorf("ball after the wall rebound is at (%v, %v), outside the court", x, y)
	}
	if got := math.Hypot(bouncedVX, 2*bouncedVY); math.Abs(got-speed) > 1e-9 {
		t.Errorf("wall rebound changed the speed to %v, want %v", got, speed)
	}
}

func TestScoringAwardsThePointAndServesTowardTheConceder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		rightTop     uint8
		wantScore    [2]uint8
		wantFeedback core.Feedback
		wantServe    core.Serve
	}{
		{
			name:         "left player scores on the right edge",
			rightTop:     8,
			wantScore:    [2]uint8{1, 0},
			wantFeedback: core.FeedbackLeftScored,
			wantServe:    core.Serve{Horizontal: core.DirectionRight, Vertical: core.DirectionDown},
		},
		{
			name:         "right player scores on the left edge",
			rightTop:     2,
			wantScore:    [2]uint8{0, 1},
			wantFeedback: core.FeedbackRightScored,
			wantServe:    core.Serve{Horizontal: core.DirectionLeft, Vertical: core.DirectionDown},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, token := beginRally(t, core.ModeLocalTwoPlayer)
			model = setPaddleTop(t, model, true, test.rightTop)
			scored, effect := runUntilPoint(t, model, token, test.wantScore)

			assertScoredPoint(
				t,
				scored,
				effect,
				token,
				test.wantScore,
				test.wantFeedback,
				test.wantServe,
				test.rightTop,
			)
		})
	}
}

// assertScoredPoint checks every part of a point that ended a rally.
func assertScoredPoint(
	t *testing.T,
	scored core.Model,
	effect core.Effect,
	rallyToken core.TimerToken,
	wantScore [2]uint8,
	wantFeedback core.Feedback,
	wantServe core.Serve,
	rightTop uint8,
) {
	t.Helper()

	left, right := scored.Score()
	if [2]uint8{left, right} != wantScore {
		t.Errorf("score = %d:%d, want %d:%d", left, right, wantScore[0], wantScore[1])
	}
	if feedback := scored.Feedback(); feedback != wantFeedback {
		t.Errorf("feedback = %v, want %v", feedback, wantFeedback)
	}
	if serve := scored.NextServe(); serve != wantServe {
		t.Errorf("next serve = %+v, want %+v", serve, wantServe)
	}
	if x, y := scored.BallPosition(); x != 34.5 || y != 9.5 {
		t.Errorf("ball after the point = (%v, %v), want the court center", x, y)
	}
	if leftTop, gotRight := scored.PaddleTops(); leftTop != 8 || gotRight != rightTop {
		t.Errorf("paddle tops after the point = %d, %d; want them kept", leftTop, gotRight)
	}
	if scored.MatchPhase() != core.MatchPhaseCountdown {
		t.Errorf("phase after the point = %v, want a new start-serve countdown", scored.MatchPhase())
	}
	if remaining := scored.CountdownRemaining(); remaining != core.CountdownSteps {
		t.Errorf("countdown after the point = %d steps, want %d", remaining, core.CountdownSteps)
	}
	if effect.Kind() != core.EffectScheduleTimer || effect.Timer().Token == rallyToken {
		t.Errorf("point effect = %+v, want a timer for a new timing sequence", effect)
	}

	// The ball stays resting at the center through the new countdown's steps.
	resting, _ := core.Update(
		scored,
		core.EventElapsed{Elapsed: core.StepDuration, Token: effect.Timer().Token},
	)
	if x, y := resting.BallPosition(); x != 34.5 || y != 9.5 {
		t.Errorf("ball one countdown step after the point = (%v, %v), want it resting", x, y)
	}
}

func TestNextServeAlternatesVerticalAndFacesTheConceder(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	first, pointEffect := runUntilPoint(t, model, token, [2]uint8{1, 0})
	countdownToken := pointEffect.Timer().Token
	if serve := first.NextServe(); serve != (core.Serve{Horizontal: core.DirectionRight, Vertical: core.DirectionDown}) {
		t.Fatalf("serve after the first point = %+v, want right and down", serve)
	}

	// The next rally serves toward the right again, so a miss concedes another point to the left player and the
	// vertical direction alternates back to up.
	second, secondToken := beginServingRally(t, first, countdownToken)
	scored, _ := runUntilPoint(t, second, secondToken, [2]uint8{2, 0})
	if serve := scored.NextServe(); serve != (core.Serve{Horizontal: core.DirectionRight, Vertical: core.DirectionUp}) {
		t.Errorf("serve after the second point = %+v, want right and up", serve)
	}
	if feedback := scored.Feedback(); feedback != core.FeedbackLeftScored {
		t.Errorf("feedback after the second point = %v, want the left player", feedback)
	}
}

func TestLaterServesLaunchTowardTheConceder(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	model = setPaddleTop(t, model, true, 2)
	scored, pointEffect := runUntilPoint(t, model, token, [2]uint8{0, 1})

	served, servedToken := beginServingRally(t, scored, pointEffect.Timer().Token)
	vx, vy := stepVelocity(t, served, servedToken)
	if vx >= 0 {
		t.Errorf("serve vx after the right player scored = %v, want toward the left player", vx)
	}
	if vy <= 0 {
		t.Errorf("serve vy after the point = %v, want the alternated downward serve", vy)
	}
	assertClose(t, "serve speed", math.Hypot(vx, 2*vy), 28)
	if serve := served.NextServe(); serve != (core.Serve{Horizontal: core.DirectionLeft, Vertical: core.DirectionDown}) {
		t.Errorf("serve after the right player scored = %+v, want left and down", serve)
	}
}

func TestFeedbackClearsWhenTheNextRallyBegins(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	scored, pointEffect := runUntilPoint(t, model, token, [2]uint8{1, 0})
	if scored.Feedback() != core.FeedbackLeftScored {
		t.Fatalf("feedback after the point = %v, want the left player", scored.Feedback())
	}

	rally, _ := beginServingRally(t, scored, pointEffect.Timer().Token)
	if feedback := rally.Feedback(); feedback != core.FeedbackNone {
		t.Errorf("feedback when the rally begins = %v, want none", feedback)
	}
	if x, y := rally.BallPosition(); x != 34.5 || y != 9.5 {
		t.Errorf("ball at the next serve = (%v, %v), want the court center", x, y)
	}
}

func TestRallyReplayReproducesStateAndEffects(t *testing.T) {
	t.Parallel()

	start, token := beginRally(t, core.ModeLocalTwoPlayer)
	_, pointEffect := runUntilPoint(t, start, token, [2]uint8{1, 0})
	countdownToken := pointEffect.Timer().Token

	events := []core.Event{
		wsStep(core.DirectionDown),
		core.EventElapsed{Elapsed: core.ElapsedLimit, Token: token},
		arrowStep(core.DirectionUp),
		core.EventElapsed{Elapsed: core.StepDuration, Token: token},
	}
	for range 20 {
		events = append(events, core.EventElapsed{Elapsed: core.ElapsedLimit, Token: token})
	}
	events = append(events,
		core.EventElapsed{Elapsed: core.StepDuration, Token: countdownToken},
		wsStep(core.DirectionUp),
		core.EventPrimary{},
	)

	firstModel, firstEffects := replayFrom(start, events)
	secondModel, secondEffects := replayFrom(start, events)
	if firstModel != secondModel {
		t.Fatalf("replay model = %+v, want %+v", secondModel, firstModel)
	}
	if !slices.Equal(firstEffects, secondEffects) {
		t.Fatalf("replay effects = %+v, want %+v", secondEffects, firstEffects)
	}
}

// beginRally starts a match in mode, spends the opening countdown, and returns the rally with its timer token.
func beginRally(t *testing.T, mode core.Mode) (core.Model, core.TimerToken) {
	t.Helper()

	started, schedule := openingCountdownFor(t, mode)

	return beginServingRally(t, started, schedule.Timer().Token)
}

// openingCountdownFor starts a match in mode and returns the model with the opening countdown's timer request.
func openingCountdownFor(t *testing.T, mode core.Mode) (core.Model, core.Effect) {
	t.Helper()

	setupModel := setupFor(t, mode, core.WinningScore7, core.DifficultyNormal)
	started, schedule := core.Update(setupModel, core.EventPrimary{})
	if started.Screen() != core.ScreenMatch || schedule.Kind() != core.EffectScheduleTimer {
		t.Fatalf("start match gave screen %v and effect %v, want a match countdown", started.Screen(), schedule.Kind())
	}

	return started, schedule
}

// beginServingRally spends the active countdown and returns the rally with the token its launch requested.
func beginServingRally(t *testing.T, model core.Model, token core.TimerToken) (core.Model, core.TimerToken) {
	t.Helper()

	if model.MatchPhase() != core.MatchPhaseCountdown {
		t.Fatalf("model phase is %v, want a countdown", model.MatchPhase())
	}
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

// advanceRally spends steps one-step elapsed events on the rally and checks that each requests the next timer.
func advanceRally(t *testing.T, model core.Model, token core.TimerToken, steps int) core.Model {
	t.Helper()

	for range steps {
		var effect core.Effect
		model, effect = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
		assertScheduled(t, effect, token)
	}

	return model
}

// runUntilPoint spends one-step elapsed events until the score reaches want and returns the new countdown.
func runUntilPoint(t *testing.T, model core.Model, token core.TimerToken, want [2]uint8) (core.Model, core.Effect) {
	t.Helper()

	for range 4000 {
		var effect core.Effect
		model, effect = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
		if left, right := model.Score(); [2]uint8{left, right} == want {
			return model, effect
		}
		if model.MatchPhase() != core.MatchPhaseRally {
			t.Fatalf("rally ended without reaching the score %v", want)
		}
		assertScheduled(t, effect, token)
	}
	t.Fatalf("score never reached %v", want)

	return model, core.Effect{}
}

// stepsPerSecond is the rate of fixed simulation steps: one step is 1/120 second.
const stepsPerSecond = 120

// stepBall spends one fixed rally step and returns the model with the ball's position change.
func stepBall(t *testing.T, model core.Model, token core.TimerToken) (core.Model, float64, float64) {
	t.Helper()

	beforeX, beforeY := model.BallPosition()
	var effect core.Effect
	model, effect = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
	if model.MatchPhase() != core.MatchPhaseRally || effect.Kind() != core.EffectScheduleTimer {
		t.Fatalf("the rally ended after phase %v with effect %v", model.MatchPhase(), effect.Kind())
	}
	afterX, afterY := model.BallPosition()

	return model, afterX - beforeX, afterY - beforeY
}

// stepVelocity spends one fixed step and returns the velocity the ball's position change shows, in cells per second.
func stepVelocity(t *testing.T, model core.Model, token core.TimerToken) (float64, float64) {
	t.Helper()

	_, dx, dy := stepBall(t, model, token)

	return dx * stepsPerSecond, dy * stepsPerSecond
}

// freeVelocity spends steps until two consecutive steps move the ball equally, which means both were free flight, and
// returns the model after the second of them with the velocity those steps carried.
func freeVelocity(t *testing.T, model core.Model, token core.TimerToken) (core.Model, float64, float64) {
	t.Helper()

	var previousDX, previousDY float64
	primed := false
	for range 60 {
		var dx, dy float64
		model, dx, dy = stepBall(t, model, token)
		if primed && math.Abs(dx-previousDX) < 1e-9 && math.Abs(dy-previousDY) < 1e-9 {
			return model, dx * stepsPerSecond, dy * stepsPerSecond
		}
		previousDX, previousDY, primed = dx, dy, true
	}
	t.Fatal("the ball never flew two consecutive equal steps")

	return model, 0, 0
}

// wallBounce spends steps until the ball's vertical travel reverses at a wall, then returns the model with the
// reflected velocity from the free flight that follows.
func wallBounce(t *testing.T, model core.Model, token core.TimerToken) (core.Model, float64, float64) {
	t.Helper()

	for range 400 {
		var dy float64
		model, _, dy = stepBall(t, model, token)
		if dy <= 0 {
			continue
		}

		return freeVelocity(t, model, token)
	}
	t.Fatal("the ball never bounced off a wall")

	return model, 0, 0
}

// driveHits spends steps until the paddle sequence tops have each taken a hit, and returns the model after the last
// hit with the speed and velocity that followed each hit. The approached paddle is moved into place after each hit.
func driveHits(
	t *testing.T,
	model core.Model,
	token core.TimerToken,
	tops []uint8,
) (core.Model, []float64, [][2]float64) {
	t.Helper()

	model = setPaddleTop(t, model, true, tops[0])
	speeds := make([]float64, 0, len(tops))
	velocities := make([][2]float64, 0, len(tops))
	var lastDirection float64
	hit := 0
	for hit < len(tops) {
		var dx float64
		model, dx, _ = stepBall(t, model, token)
		direction := math.Copysign(1, dx)
		if lastDirection == 0 || direction == lastDirection {
			lastDirection = direction

			continue
		}

		// The horizontal travel reversed, so the previous step sent the ball off a paddle. The free flight that
		// follows shows the new velocity.
		var vx, vy float64
		model, vx, vy = freeVelocity(t, model, token)
		speeds = append(speeds, math.Hypot(vx, 2*vy))
		velocities = append(velocities, [2]float64{vx, vy})
		hit++
		if hit < len(tops) {
			model = setPaddleTop(t, model, vx > 0, tops[hit])
		}
		lastDirection = 0
	}

	return model, speeds, velocities
}

// setPaddleTop moves one paddle with its binding group until it reaches top.
func setPaddleTop(t *testing.T, model core.Model, right bool, top uint8) core.Model {
	t.Helper()

	left, rightTop := model.PaddleTops()
	current := left
	group := core.BindingWS
	if right {
		current = rightTop
		group = core.BindingArrows
	}
	for current != top {
		direction := core.DirectionUp
		if top > current {
			direction = core.DirectionDown
		}

		var effect core.Effect
		model, effect = core.Update(model, core.EventPaddleStep{Direction: direction, Group: group})
		if effect.Kind() != core.EffectNone {
			t.Fatalf("paddle step requested %v, want no effect", effect.Kind())
		}
		left, rightTop = model.PaddleTops()
		if right {
			current = rightTop
		} else {
			current = left
		}
	}

	return model
}

// assertBallStep checks the ball is exactly the served velocity times the given number of fixed steps from center.
func assertBallStep(t *testing.T, model core.Model, steps int) {
	t.Helper()

	x, y := model.BallPosition()
	assertClose(t, "ball x", x, 34.5+float64(steps)*serveVX/120)
	assertClose(t, "ball y", y, 9.5-float64(steps)*serveVY/120)
}

func assertClose(t *testing.T, name string, got, want float64) {
	t.Helper()

	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func almostEqualSlice(got, want []float64) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if math.Abs(got[index]-want[index]) > 1e-9 {
			return false
		}
	}

	return true
}
