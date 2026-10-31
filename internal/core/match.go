package core

// Playfield geometry in court cells. The court interior is 70 columns by 20 rows; borders sit outside it.
const (
	// CourtWidth is the number of interior columns of the fixed court.
	CourtWidth = 70
	// CourtHeight is the number of interior rows of the fixed court.
	CourtHeight = 20
	// PaddleHeight is the number of court rows a paddle covers.
	PaddleHeight = 4
	// PaddleLeftColumn is the court column of the left paddle.
	PaddleLeftColumn = 2
	// PaddleRightColumn is the court column of the right paddle.
	PaddleRightColumn = CourtWidth - 3
)

const (
	// paddleStep is the number of rows one accepted paddle step moves.
	paddleStep = 2
	// paddleTopLimit is the lowest top row that keeps a whole paddle inside the court.
	paddleTopLimit = CourtHeight - PaddleHeight
	// paddleStartTop is the top row that centers a paddle vertically.
	paddleStartTop = (CourtHeight - PaddleHeight) / 2
	// ballStartX and ballStartY are the court center the ball resets to before every serve.
	ballStartX = (CourtWidth - 1) / 2.0
	ballStartY = (CourtHeight - 1) / 2.0
)

// Serve is the direction pair a serve travels in.
type Serve struct {
	// Horizontal is the direction the serve travels across the court.
	Horizontal HorizontalDirection
	// Vertical is the direction the serve drifts as it travels.
	Vertical VerticalDirection
}

// chain is the rematch chain one match belongs to. It owns the opening directions the next match serves with and the
// zero-based chain position, which seeds the computer player.
type chain struct {
	position uint8
	opening  HorizontalDirection
	vertical VerticalDirection
}

// MatchPhase tags the phase of an active match for readers outside the core.
type MatchPhase uint8

const (
	// MatchPhaseCountdown waits 0.8 second before the serve starts or a frozen rally resumes.
	MatchPhaseCountdown MatchPhase = iota
	// MatchPhaseRally is a rally in play.
	MatchPhaseRally
)

// matchPhase is the tagged phase of a match.
//
//sumtype:decl
type matchPhase interface {
	isMatchPhase()
	tag() MatchPhase
}

// serveContinuation says what a countdown starts when it completes.
type serveContinuation uint8

const (
	// continuationStartServe launches the pending serve.
	continuationStartServe serveContinuation = iota
)

// countdownPhase waits a fixed number of steps before starting the serve or continuing a frozen rally. It owns the
// correlation token of its timing sequence.
type countdownPhase struct {
	remaining    Steps
	token        TimerToken
	continuation serveContinuation
}

// rallyPhase is a rally in play. Ball motion arrives in a later ticket.
type rallyPhase struct{}

func (countdownPhase) isMatchPhase() {}

func (rallyPhase) isMatchPhase() {}

func (countdownPhase) tag() MatchPhase { return MatchPhaseCountdown }

func (rallyPhase) tag() MatchPhase { return MatchPhaseRally }

// score is the points won by each side.
type score struct {
	left  uint8
	right uint8
}

// paddles are the top court rows of the two paddles.
type paddles struct {
	left  uint8
	right uint8
}

// ball is the ball's position in court cells. It is a fractional point: the renderer rounds it, the match does not.
type ball struct {
	x float64
	y float64
}

// matchState is one match in progress. It owns the settings it locked and everything it accumulates.
type matchState struct {
	settings Settings
	score    score
	paddles  paddles
	ball     ball
	elapsed  Duration
	phase    matchPhase
}

// startMatch locks the draft, begins a new rematch chain, initializes match data, and enters the opening countdown.
func startMatch(model Model) (Model, Effect) {
	token := model.issueToken()
	model.chain = chain{position: 0, opening: DirectionRight, vertical: DirectionUp}
	model.state = matchState{
		settings: model.draft,
		paddles:  paddles{left: paddleStartTop, right: paddleStartTop},
		ball:     ball{x: ballStartX, y: ballStartY},
		phase: countdownPhase{
			remaining:    CountdownSteps,
			token:        token,
			continuation: continuationStartServe,
		},
	}

	return model, scheduleTimer(token, TimerDelay)
}

func updateMatch(model Model, match matchState, event Event) (Model, Effect) {
	switch event := event.(type) {
	case EventPaddleStep:
		match.paddles = moved(match.paddles, match.settings.Mode, event)
		model.state = match
		return model, Effect{}
	case EventElapsed:
		return advanceMatch(model, match, event)
	case EventNavigate, EventAdjust, EventPrimary, EventBack, EventQuit:
		return model, Effect{}
	}

	return model, Effect{}
}

// advanceMatch applies one elapsed event to the match's active phase.
func advanceMatch(model Model, match matchState, elapsed EventElapsed) (Model, Effect) {
	var effect Effect
	switch phase := match.phase.(type) {
	case countdownPhase:
		match, effect = advanceCountdown(match, phase, elapsed)
	case rallyPhase:
		// A rally has no motion yet, so elapsed time changes nothing until ball motion arrives.
	}
	model.state = match

	return model, effect
}

// advanceCountdown spends elapsed time on the countdown, which requests its next timer while steps remain and enters
// the rally when they run out. A completion from another timing sequence is a strict no-op.
func advanceCountdown(match matchState, countdown countdownPhase, elapsed EventElapsed) (matchState, Effect) {
	if elapsed.Token != countdown.token {
		return match, Effect{}
	}

	var steps Steps
	match, steps = match.acceptedTime(elapsed.Elapsed)
	if steps >= countdown.remaining {
		match.elapsed = 0
		match.phase = rallyPhase{}

		return match, Effect{}
	}

	countdown.remaining -= steps
	match.phase = countdown

	return match, scheduleTimer(countdown.token, TimerDelay)
}

// moved applies one paddle step to the paddle its binding group controls. In Local two-player each group drives one
// paddle; in Single player both groups drive the human's left paddle.
func moved(paddles paddles, mode Mode, step EventPaddleStep) paddles {
	if step.Group == BindingArrows && mode == ModeLocalTwoPlayer {
		paddles.right = stepped(paddles.right, step.Direction)

		return paddles
	}

	paddles.left = stepped(paddles.left, step.Direction)

	return paddles
}

// stepped moves one paddle top by a step and clamps it inside the court.
func stepped(top uint8, direction VerticalDirection) uint8 {
	next := int(top)
	switch direction {
	case DirectionUp:
		next -= paddleStep
	case DirectionDown:
		next += paddleStep
	}

	return uint8(min(max(next, 0), paddleTopLimit))
}

// MatchSettings returns the settings locked when the active match started. It is meaningful only while Screen is
// ScreenMatch.
func (model Model) MatchSettings() Settings {
	if match, ok := model.state.(matchState); ok {
		return match.settings
	}

	return model.draft
}

// MatchPhase returns the phase of the active match. It is meaningful only while Screen is ScreenMatch.
func (model Model) MatchPhase() MatchPhase {
	if match, ok := model.state.(matchState); ok {
		return match.phase.tag()
	}

	return MatchPhaseCountdown
}

// Score returns the points won by the left and right players. It is meaningful only while Screen is ScreenMatch.
func (model Model) Score() (left, right uint8) {
	if match, ok := model.state.(matchState); ok {
		return match.score.left, match.score.right
	}

	return 0, 0
}

// PaddleTops returns the top court row of the left and right paddles; each covers PaddleHeight rows from there. It is
// meaningful only while Screen is ScreenMatch.
func (model Model) PaddleTops() (left, right uint8) {
	if match, ok := model.state.(matchState); ok {
		return match.paddles.left, match.paddles.right
	}

	return 0, 0
}

// BallPosition returns the ball's court position as a fractional point. It is meaningful only while Screen is
// ScreenMatch.
func (model Model) BallPosition() (x, y float64) {
	if match, ok := model.state.(matchState); ok {
		return match.ball.x, match.ball.y
	}

	return 0, 0
}

// CountdownRemaining returns the fixed steps left in the active countdown, at most CountdownSteps. It is meaningful
// only while MatchPhase is MatchPhaseCountdown.
func (model Model) CountdownRemaining() Steps {
	if match, ok := model.state.(matchState); ok {
		if countdown, counting := match.phase.(countdownPhase); counting {
			return countdown.remaining
		}
	}

	return 0
}

// NextServe returns the directions the next serve will travel in. It is meaningful only while Screen is ScreenMatch.
func (model Model) NextServe() Serve {
	return Serve{Horizontal: model.chain.opening, Vertical: model.chain.vertical}
}
