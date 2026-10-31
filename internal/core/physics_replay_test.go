package core_test

import (
	"math"
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

// contactTolerance is how close a replayed paddle contact must land to a target row before the contact may stand for
// that row. Public replay cannot put a fractional ball position exactly on a paddle row boundary, so the angle tests
// accept a small window and still assert the angle the observed row maps to.
const contactTolerance = 1e-2

func TestEndpointPaddleHitReboundsAtSixtyFiveDegrees(t *testing.T) {
	t.Parallel()

	model, contact := adaptiveContact(t, edgeTarget)
	angle := reboundAngle(model)
	if want := impactAngle(contact); math.Abs(angle-want) > 1e-6 {
		t.Errorf("endpoint rebound angle = %v degrees, want the map's %v", angle, want)
	}
	if math.Abs(math.Abs(angle)-65) > 0.5 {
		t.Errorf("endpoint rebound angle = %v degrees, want 65 within the replay window", angle)
	}
}

func TestCenterPaddleHitReboundsFlat(t *testing.T) {
	t.Parallel()

	model, contact := adaptiveContact(t, centerTarget)
	angle := reboundAngle(model)
	if want := impactAngle(contact); math.Abs(angle-want) > 1e-6 {
		t.Errorf("center rebound angle = %v degrees, want the map's %v", angle, want)
	}
	if math.Abs(angle) > 0.5 {
		t.Errorf("center rebound angle = %v degrees, want 0 within the replay window", angle)
	}
}

// contact is one replayed paddle contact: the top row of the paddle that took it and the ball's row on that paddle.
type contact struct {
	top uint8
	row float64
}

// adaptiveContact replays rallies until a paddle contact lands within contactTolerance of the row target names, and
// returns the model after that contact. It reads only public accessors and drives paddle-step and elapsed events, so
// it explores the reachable contact rows instead of hard-coding one event sequence.
func adaptiveContact(t *testing.T, target func(row float64, top uint8) float64) (core.Model, contact) {
	t.Helper()

	random := randomSource{state: 21}
	closest := math.Inf(1)
	hits := 0
	for range 200 {
		model, token := beginRally(t, core.ModeLocalTwoPlayer)
		if hit, ok := searchContact(t, model, token, &random, target, &closest, &hits); ok {
			return hit.model, hit.contact
		}
	}
	t.Fatalf("no replayed contact landed inside the target window; closest %v over %d hits", closest, hits)

	return core.Model{}, contact{}
}

// searchContact drives one rally, moving the approached paddle to a covering top before every crossing, until a
// contact lands inside the target window or the rally ends.
func searchContact(
	t *testing.T,
	model core.Model,
	token core.TimerToken,
	random *randomSource,
	target func(row float64, top uint8) float64,
	closest *float64,
	hits *int,
) (found struct {
	model   core.Model
	contact contact
}, ok bool) {
	t.Helper()

	for range 3000 {
		row, right, crossing := predictedContact(model)
		var chosen uint8
		movable := false
		isTarget := false
		if crossing {
			if top, found := closestTop(row, target); found {
				chosen, movable, isTarget = top, true, true
			} else if valid := validPaddleTops(row); len(valid) > 0 {
				chosen, movable = valid[random.next(len(valid))], true
			}
		}
		if movable {
			model = setPaddleTop(t, model, right, chosen)
			if distance := target(row, chosen); distance < *closest {
				*closest = distance
			}
			*hits++
		}

		lastVX, _ := model.BallVelocity()
		var effect core.Effect
		model, effect = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
		if effect.Kind() != core.EffectScheduleTimer {
			return found, false
		}
		if vx, _ := model.BallVelocity(); (vx > 0) != (lastVX > 0) && isTarget {
			found.model = model
			found.contact = contact{top: chosen, row: row}

			return found, true
		}
	}

	return found, false
}

// predictedContact returns the paddle contact row this step will reach, which side of the court it is on, and whether
// a contact is predicted at all. A wall contact before the paddle plane invalidates the prediction.
func predictedContact(model core.Model) (row float64, right, ok bool) {
	x, y := model.BallPosition()
	vx, vy := model.BallVelocity()
	if vx == 0 {
		return 0, false, false
	}
	plane := float64(core.PaddleRightColumn)
	right = true
	if vx < 0 {
		plane = core.PaddleLeftColumn
		right = false
	}
	time := (plane - x) / vx
	if time < 0 || time > 1.0/120 {
		return 0, right, false
	}
	if wallBefore(y, vy, time) {
		return 0, right, false
	}

	return y + vy*time, right, true
}

// wallBefore reports whether the ball reaches a wall before this step's paddle contact, which makes the predicted
// contact row wrong.
func wallBefore(y, vy, contactTime float64) bool {
	if vy < 0 {
		return (0-y)/vy < contactTime
	}
	if vy > 0 {
		return (19-y)/vy < contactTime
	}

	return false
}

// closestTop returns the paddle top whose target window holds row within contactTolerance. Paddle tops at the court
// edges are skipped so the rebound cannot meet a wall in the same step.
func closestTop(row float64, target func(row float64, top uint8) float64) (uint8, bool) {
	bestTop := uint8(0)
	best := math.Inf(1)
	for _, top := range validPaddleTops(row) {
		if top == 0 || top == 16 {
			continue
		}
		if distance := target(row, top); distance < best {
			best = distance
			bestTop = top
		}
	}
	if best > contactTolerance {
		return 0, false
	}

	return bestTop, true
}

// validPaddleTops lists the even paddle tops whose hittable span holds row.
func validPaddleTops(row float64) []uint8 {
	var tops []uint8
	for index := range 9 {
		top := uint8(2 * index)
		if row >= float64(top)-1e-9 && row <= float64(top+3)+1e-9 {
			tops = append(tops, top)
		}
	}

	return tops
}

// randomSource is a small deterministic pseudo-random source (splitmix64) so every run explores the same choices.
type randomSource struct {
	state uint64
}

func (source *randomSource) next(limit int) int {
	source.state += 0x9E3779B97F4A7C15
	mixed := source.state
	mixed = (mixed ^ (mixed >> 30)) * 0xBF58476D1CE4E5B9
	mixed = (mixed ^ (mixed >> 27)) * 0x94D049BB133111EB
	mixed ^= mixed >> 31

	return int(mixed % uint64(limit))
}

// edgeTarget measures how far a contact row is from the nearest end of the paddle's span.
func edgeTarget(row float64, top uint8) float64 {
	return math.Min(math.Abs(row-float64(top)), math.Abs(row-float64(top+3)))
}

// centerTarget measures how far a contact row is from the center of the paddle's span.
func centerTarget(row float64, top uint8) float64 {
	return math.Abs(row - (float64(top) + 1.5))
}

// reboundAngle returns the visual angle a rebound leaves a paddle at, away from the paddle, in degrees.
func reboundAngle(model core.Model) float64 {
	vx, vy := model.BallVelocity()

	return math.Atan2(2*vy, math.Abs(vx)) * 180 / math.Pi
}

// impactAngle is the angle the linear impact map gives this contact.
func impactAngle(hit contact) float64 {
	impact := (hit.row - (float64(hit.top) + 1.5)) / 1.5
	impact = math.Min(math.Max(impact, -1), 1)

	return impact * 65
}
