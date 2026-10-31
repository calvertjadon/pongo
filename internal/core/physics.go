package core

import "math"

// Ball physics in court cells and seconds. A rally starts at serveSpeed on a serveAngle visual path; each paddle hit
// adds hitSpeedGain up to maxSpeed and sets the visual rebound angle by where the ball touched the paddle, from flat
// at the center to maxReboundAngle at either end. Court cells are twice as tall as they are wide, so vertical velocity
// carries the half factor that makes visual angles look right on screen.
const (
	serveSpeed      = 28.0
	hitSpeedGain    = 3.0
	maxSpeed        = 44.0
	serveAngle      = 20 * math.Pi / 180
	maxReboundAngle = 65 * math.Pi / 180
	// paddleHalfSpan is half the vertical extent of a paddle's hittable span, which covers the four rows its top
	// starts: rows [top, top+3], so the span center is 1.5 rows below the top. The top and bottom rows map to the
	// steepest rebound, and the span center to a flat one.
	paddleHalfSpan = (PaddleHeight - 1) / 2.0
	// contactEpsilon is the time tolerance that makes two contact times count as the same instant.
	contactEpsilon = 1e-7
	// maxContacts is the most contacts one fixed step resolves. The ball holds its last valid position after that.
	maxContacts = 4
)

// stepSeconds is the time one fixed step advances the ball, in seconds.
const stepSeconds = float64(StepDuration) / float64(DurationUnitsPerSecond)

// sign returns the court direction of a horizontal direction as a velocity multiplier.
func (direction HorizontalDirection) sign() float64 {
	switch direction {
	case DirectionLeft:
		return -1
	case DirectionRight:
		return 1
	}

	return 1
}

// sign returns the court direction of a vertical direction as a velocity multiplier. Court rows grow downward.
func (direction VerticalDirection) sign() float64 {
	switch direction {
	case DirectionUp:
		return -1
	case DirectionDown:
		return 1
	}

	return 1
}

// opposite returns the other vertical direction.
func (direction VerticalDirection) opposite() VerticalDirection {
	switch direction {
	case DirectionUp:
		return DirectionDown
	case DirectionDown:
		return DirectionUp
	}

	return direction
}

// opposite returns the other horizontal direction.
func (direction HorizontalDirection) opposite() HorizontalDirection {
	switch direction {
	case DirectionLeft:
		return DirectionRight
	case DirectionRight:
		return DirectionLeft
	}

	return direction
}

// contactKind tags what the ball meets on its path.
type contactKind uint8

const (
	// contactTopWall is the row-zero wall.
	contactTopWall contactKind = iota
	// contactBottomWall is the last-row wall.
	contactBottomWall
	// contactLeftPaddle is the left paddle's plane.
	contactLeftPaddle
	// contactRightPaddle is the right paddle's plane.
	contactRightPaddle
)

// isWall reports whether the contact is a court wall rather than a paddle.
func (kind contactKind) isWall() bool {
	switch kind {
	case contactTopWall, contactBottomWall:
		return true
	case contactLeftPaddle, contactRightPaddle:
		return false
	}

	return false
}

// contact is one surface the ball reaches after time seconds. Paddle contacts carry the paddle's top row, and a
// wallPaddle contact also reflects the wall at the same instant.
type contact struct {
	kind       contactKind
	time       float64
	top        uint8
	wallPaddle bool
}

// servedBall returns the ball at the court center, launched along serve at the opening serve speed and angle. The
// vertical direction only chooses which way the serve drifts.
func servedBall(serve Serve) ball {
	return ball{
		x:     ballStartX,
		y:     ballStartY,
		vx:    serve.Horizontal.sign() * serveSpeed * math.Cos(serveAngle),
		vy:    serve.Vertical.sign() * serveSpeed * math.Sin(serveAngle) / 2,
		speed: serveSpeed,
	}
}

// resetBall returns the ball resting at the court center before a serve.
func resetBall() ball {
	return ball{x: ballStartX, y: ballStartY, speed: serveSpeed}
}

// steppedBall advances the ball one fixed step, resolving contacts chronologically: the earliest contact is reached,
// the ball rebounds there, and the leftover time is spent on the next contact. A wall and paddle contact at the same
// instant reflect both affected axes once. After maxContacts the ball holds its last valid position and the movement
// left in the step is discarded.
func steppedBall(current ball, paddles paddles) ball {
	remaining := stepSeconds
	for range maxContacts {
		if remaining <= 0 {
			break
		}

		wall, hasWall := wallContact(current, remaining)
		paddle, hasPaddle := paddleContact(current, paddles, remaining)
		var met contact
		switch {
		case !hasWall && !hasPaddle:
			return advanced(current, remaining)
		case hasWall && !hasPaddle:
			met = wall
		case !hasWall && hasPaddle:
			met = paddle
		case sameInstant(wall, paddle):
			met = paddle
			met.wallPaddle = true
			met.time = min(wall.time, paddle.time)
		case wall.time < paddle.time:
			met = wall
		default:
			met = paddle
		}

		current = advanced(current, met.time)
		current = rebounded(current, met)
		remaining -= met.time
	}

	return current
}

// wallContact returns the earliest court wall the ball reaches within remaining time.
func wallContact(current ball, remaining float64) (contact, bool) {
	if current.vy < -contactEpsilon {
		time := (0 - current.y) / current.vy
		if reachable(time, remaining) {
			return contact{kind: contactTopWall, time: max(0, time)}, true
		}
	}
	if current.vy > contactEpsilon {
		time := (CourtHeight - 1 - current.y) / current.vy
		if reachable(time, remaining) {
			return contact{kind: contactBottomWall, time: max(0, time)}, true
		}
	}

	return contact{}, false
}

// paddleContact returns the earliest paddle plane the ball reaches within remaining time, but only while the ball
// moves toward that paddle and meets it inside the paddle's hittable span.
func paddleContact(current ball, paddles paddles, remaining float64) (contact, bool) {
	if current.vx < -contactEpsilon {
		time := (PaddleLeftColumn - current.x) / current.vx
		if reachable(time, remaining) && hitsPaddle(current, time, paddles.left) {
			return contact{kind: contactLeftPaddle, time: max(0, time), top: paddles.left}, true
		}
	}
	if current.vx > contactEpsilon {
		time := (PaddleRightColumn - current.x) / current.vx
		if reachable(time, remaining) && hitsPaddle(current, time, paddles.right) {
			return contact{kind: contactRightPaddle, time: max(0, time), top: paddles.right}, true
		}
	}

	return contact{}, false
}

// reachable reports whether a contact time falls inside the time left in the step, allowing for float tolerances.
func reachable(time, remaining float64) bool {
	return time >= -contactEpsilon && time <= remaining+contactEpsilon
}

// hitsPaddle reports whether the ball's path crosses a paddle plane inside that paddle's hittable span.
func hitsPaddle(current ball, time float64, top uint8) bool {
	y := current.y + current.vy*max(0, time)
	low := float64(top) - contactEpsilon
	high := float64(top) + PaddleHeight + contactEpsilon

	return y >= low && y <= high
}

// sameInstant reports whether two contacts happen at the same instant.
func sameInstant(one, other contact) bool {
	return math.Abs(one.time-other.time) <= contactEpsilon
}

// advanced moves the ball along its velocity for time.
func advanced(current ball, time float64) ball {
	current.x += current.vx * time
	current.y += current.vy * time

	return current
}

// rebounded applies one contact's rebound. A wall reflects the vertical velocity. A paddle sets the new speed and the
// rebound angle from the impact row, sending the ball away from the paddle. A wall-paddle contact applies the paddle
// rebound and then the wall's reflection, so both affected axes change exactly once.
func rebounded(current ball, met contact) ball {
	if met.kind.isWall() {
		current.vy = -current.vy

		return current
	}

	impact := (current.y - paddleCenter(met.top)) / paddleHalfSpan
	impact = min(max(impact, -1), 1)
	speed := min(current.speed+hitSpeedGain, maxSpeed)
	angle := impact * maxReboundAngle

	current.vx = paddleAway(met.kind) * speed * math.Cos(angle)
	current.vy = speed * math.Sin(angle) / 2
	current.speed = speed
	if met.wallPaddle {
		current.vy = -current.vy
	}

	return current
}

// paddleCenter returns the vertical center of a paddle's hittable span.
func paddleCenter(top uint8) float64 {
	return float64(top) + paddleHalfSpan
}

// paddleAway returns the horizontal direction a rebound leaves a paddle in.
func paddleAway(kind contactKind) float64 {
	switch kind {
	case contactLeftPaddle:
		return 1
	case contactRightPaddle:
		return -1
	case contactTopWall, contactBottomWall:
		return 0
	}

	return 0
}
