package core

import (
	"math"
	"testing"
)

// Public replay cannot reach the geometries in this file. A fixed step moves the ball at most about 0.37 cells across
// and 0.18 cells down at the speed cap, so a paddle plane and a court wall meet only at an exact corner, and even a
// crafted rally would need an exact float coincidence to land on one. The same bound fits at most one paddle contact
// and one wall contact into a step, so the four-contact limit is unreachable too. These tests drive the per-step
// resolver with explicit geometry instead.

func TestSteppedBallReflectsBothAxesOnceOnAnEqualTimeCorner(t *testing.T) {
	t.Parallel()

	// The ball reaches the left paddle's top row and the top wall at the same instant: both contacts take 4 ms.
	corner := ball{x: PaddleLeftColumn + 0.04, y: 0.04, vx: -10, vy: -10, speed: serveSpeed}
	got := steppedBall(corner, paddles{left: 0, right: paddleStartTop})

	if got.speed != serveSpeed+hitSpeedGain {
		t.Errorf("speed after the corner = %v, want one hit's increment %v", got.speed, serveSpeed+hitSpeedGain)
	}
	if got.vx <= 0 || got.vy <= 0 {
		t.Errorf(
			"velocity after the corner = (%v, %v), want away from the paddle and down into the court",
			got.vx,
			got.vy,
		)
	}
	angle := math.Atan2(2*got.vy, got.vx) * 180 / math.Pi
	if math.Abs(angle-65) > 1e-9 {
		t.Errorf("rebound angle after the corner = %v degrees, want 65", angle)
	}

	leftOver := stepSeconds - 0.004
	wantX := PaddleLeftColumn + got.speed*math.Cos(maxReboundAngle)*leftOver
	wantY := got.speed * math.Sin(maxReboundAngle) / 2 * leftOver
	if math.Abs(got.x-wantX) > 1e-9 || math.Abs(got.y-wantY) > 1e-9 {
		t.Errorf(
			"ball after the corner = (%v, %v), want (%v, %v); the remaining step was spent once",
			got.x,
			got.y,
			wantX,
			wantY,
		)
	}
}

func TestSteppedBallResolvesACompoundContactInOrder(t *testing.T) {
	t.Parallel()

	// The ball hits the top wall 2 ms before the left paddle in the same step. The wall reflects it down, the paddle
	// sends it back up at the end row, and the wall reflects it once more, so it ends moving away and down.
	compound := ball{x: PaddleLeftColumn + 0.06, y: 0.04, vx: -10, vy: -10, speed: serveSpeed}
	got := steppedBall(compound, paddles{left: 0, right: paddleStartTop})

	if got.vx <= 0 || got.vy <= 0 {
		t.Errorf("velocity after the compound contact = (%v, %v), want away from the paddle and down", got.vx, got.vy)
	}
	if got.y < 0 || got.y > CourtHeight-1 {
		t.Errorf("ball after the compound contact is at y = %v, outside the court", got.y)
	}
	if got.speed != serveSpeed+hitSpeedGain {
		t.Errorf("speed after the compound contact = %v, want one hit's increment", got.speed)
	}
}

func TestSteppedBallStopsAfterFourContacts(t *testing.T) {
	t.Parallel()

	// A constructed ball fast enough to cross the whole court several times in one fixed step. The resolver must stop
	// at the fourth contact, keep that contact's position, and discard the movement left in the step.
	fast := ball{x: ballStartX, y: ballStartY, vy: 1e6, speed: serveSpeed}
	got := steppedBall(fast, paddles{left: paddleStartTop, right: paddleStartTop})

	if math.Abs(got.x-ballStartX) > 1e-9 {
		t.Errorf("ball x after four contacts = %v, want the start %v", got.x, ballStartX)
	}
	if math.Abs(got.y) > 1e-9 {
		t.Errorf("ball y after four contacts = %v, want the fourth contact row 0", got.y)
	}
	if got.vy <= 0 {
		t.Errorf("ball vy after the fourth contact = %v, want the top wall's reflection", got.vy)
	}
}
