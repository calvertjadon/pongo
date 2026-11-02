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

func TestSteppedBallTakesThePaddleBeforeALaterWall(t *testing.T) {
	t.Parallel()

	// The ball reaches the left paddle plane 6 ms into the step and the top wall at 8 ms: the paddle contact is
	// earlier and must rebound the ball before the wall is reached.
	current := ball{x: PaddleLeftColumn + 0.06, y: 0.08, vx: -10, vy: -10, speed: serveSpeed}
	got := steppedBall(current, paddles{left: 0, right: paddleStartTop})

	if got.speed != serveSpeed+hitSpeedGain {
		t.Errorf("speed after the paddle-then-wall step = %v, want one hit's increment %v",
			got.speed, serveSpeed+hitSpeedGain)
	}
	if got.vx <= 0 {
		t.Errorf("vx after the paddle-then-wall step = %v, want away from the left paddle", got.vx)
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

func TestPaddleSpanCoversExactlyTheFourRowsItsTopStarts(t *testing.T) {
	t.Parallel()

	const top = uint8(8)
	tests := []struct {
		name      string
		row       float64
		wantHit   bool
		wantAngle float64
	}{
		{name: "just above the top row misses", row: float64(top) - 0.01},
		{name: "top row hits at the steepest angle", row: float64(top), wantHit: true, wantAngle: -65},
		{name: "bottom row hits at the steepest angle", row: float64(top + 3), wantHit: true, wantAngle: 65},
		{name: "just below the bottom row misses", row: float64(top+3) + 0.01},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// The ball sits just right of the left paddle plane and travels straight left for 5 ms, so it reaches the
			// plane at exactly the test row.
			current := ball{x: PaddleLeftColumn + 0.05, y: test.row, vx: -10, speed: serveSpeed}
			got := steppedBall(current, paddles{left: top, right: paddleStartTop})
			if !test.wantHit {
				if got.speed != serveSpeed || got.vx != -10 || got.y != test.row {
					t.Errorf("ball at row %v was returned: speed %v, vx %v, y %v",
						test.row, got.speed, got.vx, got.y)
				}

				return
			}
			if got.speed != serveSpeed+hitSpeedGain {
				t.Errorf("speed after the hit at row %v = %v, want one hit's increment", test.row, got.speed)
			}
			angle := math.Atan2(2*got.vy, got.vx) * 180 / math.Pi
			if math.Abs(angle-test.wantAngle) > 1e-9 {
				t.Errorf("rebound angle at row %v = %v degrees, want %v", test.row, angle, test.wantAngle)
			}
		})
	}
}

// TestHitsPaddleJudgesTheRowAtTheContactTime pins that the span is tested at the row the ball reaches when it crosses
// the paddle plane, and that a plane already crossed within the tolerance is judged at the row the ball holds now.
func TestHitsPaddleJudgesTheRowAtTheContactTime(t *testing.T) {
	t.Parallel()

	const top = uint8(4)
	tests := []struct {
		name string
		ball ball
		time float64
		want bool
	}{
		{name: "reaches the span at the contact time", ball: ball{y: 9, vy: -1000}, time: 0.005, want: true},
		{name: "leaves the span by the contact time", ball: ball{y: 3, vy: 1000}, time: 0.005, want: false},
		{
			name: "a plane crossed within the tolerance is judged where the ball is",
			ball: ball{y: 7, vy: -22},
			time: -contactEpsilon,
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := hitsPaddle(test.ball, test.time, top); got != test.want {
				t.Errorf("hitsPaddle(%+v, %v, %d) = %v, want %v", test.ball, test.time, top, got, test.want)
			}
		})
	}
}

// TestHitsPaddleCountsRowsOnTheToleranceEdge pins that the span's tolerance is inclusive at both ends and that a row
// beyond it misses.
func TestHitsPaddleCountsRowsOnTheToleranceEdge(t *testing.T) {
	t.Parallel()

	const top = uint8(4)
	tests := []struct {
		name string
		row  float64
		want bool
	}{
		{name: "first row inside the tolerance", row: float64(top) - contactEpsilon, want: true},
		{name: "last row inside the tolerance", row: float64(top+paddleLastOffset) + contactEpsilon, want: true},
		{name: "above the tolerance", row: float64(top) - 2*contactEpsilon},
		{name: "below the tolerance", row: float64(top+paddleLastOffset) + 2*contactEpsilon},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := hitsPaddle(ball{y: test.row}, 0, top); got != test.want {
				t.Errorf("hitsPaddle at row %v = %v, want %v", test.row, got, test.want)
			}
		})
	}
}

// TestPaddleContactFindsOnlyAnApproachingBallInTheSpan pins that a paddle contact needs the ball moving toward that
// paddle faster than the contact tolerance, meeting the span, and that a plane already crossed within the tolerance is
// a contact now rather than in the past.
func TestPaddleContactFindsOnlyAnApproachingBallInTheSpan(t *testing.T) {
	t.Parallel()

	both := paddles{left: 4, right: 8}
	tests := []struct {
		name      string
		ball      ball
		wantFound bool
		wantKind  contactKind
	}{
		{
			name:      "left plane reached inside the step",
			ball:      ball{x: PaddleLeftColumn + 0.05, y: 5, vx: -10},
			wantFound: true,
			wantKind:  contactLeftPaddle,
		},
		{
			name:      "right plane reached inside the step",
			ball:      ball{x: PaddleRightColumn - 0.05, y: 9, vx: 10},
			wantFound: true,
			wantKind:  contactRightPaddle,
		},
		{
			name:      "left plane already crossed within the tolerance",
			ball:      ball{x: PaddleLeftColumn - 1e-8, y: 5, vx: -10},
			wantFound: true,
			wantKind:  contactLeftPaddle,
		},
		{
			name:      "right plane already crossed within the tolerance",
			ball:      ball{x: PaddleRightColumn + 1e-8, y: 9, vx: 10},
			wantFound: true,
			wantKind:  contactRightPaddle,
		},
		{
			name: "left speed exactly at the tolerance is not approach",
			ball: ball{x: PaddleLeftColumn, y: 5, vx: -contactEpsilon},
		},
		{
			name: "left drift below the tolerance is not approach",
			ball: ball{x: PaddleLeftColumn, y: 5, vx: contactEpsilon / 10},
		},
		{
			name: "right speed exactly at the tolerance is not approach",
			ball: ball{x: PaddleRightColumn, y: 9, vx: contactEpsilon},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			met, found := paddleContact(test.ball, both, stepSeconds)
			if found != test.wantFound {
				t.Fatalf("paddleContact(%+v) found = %v, want %v", test.ball, found, test.wantFound)
			}
			if !found {
				return
			}
			if met.kind != test.wantKind {
				t.Errorf("contact kind = %v, want %v", met.kind, test.wantKind)
			}
			if met.time < 0 {
				t.Errorf("contact time = %v, want it not in the past", met.time)
			}
			if met.time > stepSeconds {
				t.Errorf("contact time = %v, want it inside the step", met.time)
			}
		})
	}
}

// TestContactTimeIsReachableWithinTheStepTolerance pins the window a contact time must fall in: from the step start
// within one tolerance to the step end within one tolerance.
func TestContactTimeIsReachableWithinTheStepTolerance(t *testing.T) {
	t.Parallel()

	const remaining = 0.005
	tests := []struct {
		name string
		time float64
		want bool
	}{
		{name: "a contact now", time: 0, want: true},
		{name: "a contact inside the step", time: 0.004, want: true},
		{name: "exactly at the end tolerance", time: remaining + contactEpsilon, want: true},
		{name: "within the end tolerance", time: remaining + contactEpsilon/2, want: true},
		{name: "beyond the end tolerance", time: remaining + 2*contactEpsilon},
		{name: "exactly at the start tolerance", time: -contactEpsilon, want: true},
		{name: "beyond the start tolerance", time: -2 * contactEpsilon},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := reachable(test.time, remaining); got != test.want {
				t.Errorf("reachable(%v, %v) = %v, want %v", test.time, remaining, got, test.want)
			}
		})
	}
}

// TestContactsShareAnInstantOnlyWithinTheTolerance pins that two contact times count as the same instant when they are
// within the tolerance either way round, and that a distinctly earlier contact does not share one.
func TestContactsShareAnInstantOnlyWithinTheTolerance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		one   float64
		other float64
		want  bool
	}{
		{name: "equal times", one: 0.01, other: 0.01, want: true},
		{name: "the later contact within the tolerance", one: 0.01 + contactEpsilon/2, other: 0.01, want: true},
		{name: "the earlier contact within the tolerance", one: 0.01, other: 0.01 + contactEpsilon/2, want: true},
		{name: "exactly the tolerance apart", one: 0, other: contactEpsilon, want: true},
		{name: "twice the tolerance apart", one: 0, other: 2 * contactEpsilon},
		{name: "a distinctly earlier contact", one: 0.01, other: 0.02},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := sameInstant(contact{time: test.one}, contact{time: test.other})
			if got != test.want {
				t.Errorf("sameInstant(%v, %v) = %v, want %v", test.one, test.other, got, test.want)
			}
		})
	}
}

// TestPaddleReboundSaturatesAtTheSpanEnds pins that a contact row outside the hittable span rebounds exactly as the
// nearest end row does, so no rebound angle exceeds the steepest one.
func TestPaddleReboundSaturatesAtTheSpanEnds(t *testing.T) {
	t.Parallel()

	const top = uint8(8)
	rebound := func(row float64) ball {
		return rebounded(ball{y: row, speed: serveSpeed}, contact{kind: contactLeftPaddle, top: top})
	}
	tests := []struct {
		name   string
		end    float64
		beyond float64
	}{
		{name: "above the first row", end: float64(top), beyond: float64(top) - 10},
		{name: "below the last row", end: float64(top + paddleLastOffset), beyond: float64(top+3) + 10},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			want := rebound(test.end)
			got := rebound(test.beyond)
			if got.vx != want.vx || got.vy != want.vy || got.speed != want.speed {
				t.Errorf(
					"rebound from row %v = (%v, %v) at speed %v, want the end row's (%v, %v) at speed %v",
					test.beyond, got.vx, got.vy, got.speed, want.vx, want.vy, want.speed,
				)
			}
		})
	}
}

// TestWallContactFindsOnlyAnIncomingBallAtEachWall pins that a wall contact comes from a ball moving toward that wall,
// that a plane already crossed within the tolerance is a contact now rather than in the past, and that a vertical
// speed at the tolerance is treated as stationary.
func TestWallContactFindsOnlyAnIncomingBallAtEachWall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ball      ball
		wantFound bool
		wantKind  contactKind
		wantNow   bool
	}{
		{
			name:      "top wall reached inside the step",
			ball:      ball{y: 0.05, vy: -10},
			wantFound: true,
			wantKind:  contactTopWall,
		},
		{
			name:      "bottom wall reached inside the step",
			ball:      ball{y: CourtHeight - 1 - 0.05, vy: 10},
			wantFound: true,
			wantKind:  contactBottomWall,
		},
		{
			name:      "top plane crossed within the tolerance",
			ball:      ball{y: -contactEpsilon / 2, vy: -10},
			wantFound: true,
			wantKind:  contactTopWall,
			wantNow:   true,
		},
		{
			name:      "bottom plane crossed within the tolerance",
			ball:      ball{y: CourtHeight - 1 + contactEpsilon/2, vy: 10},
			wantFound: true,
			wantKind:  contactBottomWall,
			wantNow:   true,
		},
		{name: "top speed exactly at the tolerance is stationary", ball: ball{y: 0, vy: -contactEpsilon}},
		{
			name: "bottom speed exactly at the tolerance is stationary",
			ball: ball{y: CourtHeight - 1, vy: contactEpsilon},
		},
		{name: "a drift down at the top wall is not incoming", ball: ball{y: 0, vy: contactEpsilon / 10}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			met, found := wallContact(test.ball, stepSeconds)
			if found != test.wantFound {
				t.Fatalf("wallContact(%+v) found = %v, want %v", test.ball, found, test.wantFound)
			}
			if !found {
				return
			}
			if met.kind != test.wantKind {
				t.Errorf("contact kind = %v, want %v", met.kind, test.wantKind)
			}
			if met.time < 0 {
				t.Errorf("contact time = %v, want it not in the past", met.time)
			}
			if test.wantNow && met.time != 0 {
				t.Errorf("contact time = %v, want the clamped instant 0", met.time)
			}
		})
	}
}

// TestSteppedBallResolvesASimultaneousContactAtTheEarlierInstant pins that a wall and a paddle contact within the
// tolerance of one another advance the ball to the earlier instant only, so it stops on the surface reached first.
func TestSteppedBallResolvesASimultaneousContactAtTheEarlierInstant(t *testing.T) {
	t.Parallel()

	// The ball reaches the wall exactly as the step ends and the paddle plane one tolerance inside it, or the other
	// way round. Both contacts are genuine, so the step must pick the earlier instant.
	const slack = 1e-8

	t.Run("wall earlier", func(t *testing.T) {
		t.Parallel()

		current := ball{
			x:     PaddleLeftColumn + 10*stepSeconds + 10*slack,
			y:     4.8 * stepSeconds,
			vx:    -10,
			vy:    -4.8,
			speed: serveSpeed,
		}
		got := steppedBall(current, paddles{left: 0, right: paddleStartTop})

		if math.Abs(got.y) > 1e-12 {
			t.Errorf("ball y after the corner = %v, want the wall row 0 the earlier instant reached", got.y)
		}
		if got.x <= PaddleLeftColumn {
			t.Errorf("ball x after the corner = %v, want it short of the paddle plane %v", got.x, PaddleLeftColumn)
		}
	})

	t.Run("paddle earlier", func(t *testing.T) {
		t.Parallel()

		current := ball{
			x:     PaddleLeftColumn + 10*stepSeconds,
			y:     4.8*stepSeconds + 4.8*slack,
			vx:    -10,
			vy:    -4.8,
			speed: serveSpeed,
		}
		got := steppedBall(current, paddles{left: 0, right: paddleStartTop})

		if math.Abs(got.x-PaddleLeftColumn) > 1e-12 {
			t.Errorf(
				"ball x after the corner = %v, want the paddle plane %v the earlier instant reached",
				got.x,
				PaddleLeftColumn,
			)
		}
	})
}

// TestOnlyTheTwoWallKindsAreWalls pins the contact classification: the two wall kinds are walls and paddle or unknown
// kinds are not, so a rebound off an unrecognised kind still takes the paddle branch.
func TestOnlyTheTwoWallKindsAreWalls(t *testing.T) {
	t.Parallel()

	for _, kind := range []contactKind{contactLeftPaddle, contactRightPaddle, contactKind(255)} {
		if kind.isWall() {
			t.Errorf("contact kind %d is a wall, want it not to be", kind)
		}
	}
	for _, kind := range []contactKind{contactTopWall, contactBottomWall} {
		if !kind.isWall() {
			t.Errorf("contact kind %d is not a wall, want it to be", kind)
		}
	}
}
