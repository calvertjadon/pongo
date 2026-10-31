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
	// MatchPhasePaused freezes the match behind the pause menu and keeps the suspended phase.
	MatchPhasePaused
	// MatchPhaseGameOver is a finished match waiting for the player's choice.
	MatchPhaseGameOver
)

// matchPhase is the tagged phase of a match.
//
//sumtype:decl
type matchPhase interface {
	isMatchPhase()
	tag() MatchPhase
}

// countdownContinuation says what a countdown does when it completes.
type countdownContinuation uint8

const (
	// continuationStartServe launches the pending serve.
	continuationStartServe countdownContinuation = iota
	// continuationResumeServe launches the pending serve after a pause interrupted the same countdown.
	continuationResumeServe
	// continuationResumeRally continues the frozen rally from its exact position and velocity.
	continuationResumeRally
)

// startsServe reports whether completing the countdown launches the pending serve.
func (continuation countdownContinuation) startsServe() bool {
	switch continuation {
	case continuationStartServe, continuationResumeServe:
		return true
	case continuationResumeRally:
		return false
	}

	return false
}

// resumed returns the continuation that resumes this countdown after a pause. A serve countdown still starts its
// serve, and a frozen rally stays frozen.
func (continuation countdownContinuation) resumed() countdownContinuation {
	switch continuation {
	case continuationStartServe, continuationResumeServe:
		return continuationResumeServe
	case continuationResumeRally:
		return continuationResumeRally
	}

	return continuation
}

// countdownPhase waits a fixed number of steps before starting the serve or continuing a frozen rally. It owns the
// correlation token of its timing sequence.
type countdownPhase struct {
	remaining    Steps
	token        TimerToken
	continuation countdownContinuation
}

// rallyPhase is a rally in play. It owns the correlation token of its timing sequence.
type rallyPhase struct {
	token TimerToken
}

// Feedback identifies the scorer feedback shown during the countdown that follows a point. It clears when the rally
// begins.
type Feedback uint8

const (
	// FeedbackNone means no point has been scored since the last serve.
	FeedbackNone Feedback = iota
	// FeedbackLeftScored reports that the left player scored the point that ended the last rally.
	FeedbackLeftScored
	// FeedbackRightScored reports that the right player scored the point that ended the last rally.
	FeedbackRightScored
)

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

// ball is the ball's position in court cells and its velocity in cells per second. Position is a fractional point:
// the renderer rounds it, the match does not. Speed is the visual speed a paddle hit raises; the velocity carries the
// cell aspect correction.
type ball struct {
	x     float64
	y     float64
	vx    float64
	vy    float64
	speed float64
}

// matchState is one match in progress. It owns the settings it locked and everything it accumulates.
type matchState struct {
	settings  Settings
	origin    chain
	score     score
	paddles   paddles
	ball      ball
	serve     Serve
	feedback  Feedback
	elapsed   Duration
	stepCount uint32
	computer  computer
	phase     matchPhase
}

// startMatch locks the draft, begins a new rematch chain, initializes match data, and enters the opening countdown.
func startMatch(model Model) (Model, Effect) {
	model.chain = chain{position: 0, opening: DirectionRight, vertical: DirectionUp}

	return beginMatch(model, model.draft)
}

// beginMatch starts a match at the chain's current opening directions with the given locked settings. Starting and
// rematching share it; both build on openedMatch so the initial data has one definition.
func beginMatch(model Model, settings Settings) (Model, Effect) {
	token := model.issueToken()
	model.state = openedMatch(settings, model.chain, token)

	return model, scheduleTimer(token, TimerDelay)
}

// openedMatch returns a match at its original start conditions: centered paddles, the resting ball, no score or
// feedback, the chain's opening serve, and the opening countdown under token. Starting and restarting share it so the
// reset rules live in one place; later reset paths extend this function.
func openedMatch(settings Settings, origin chain, token TimerToken) matchState {
	return matchState{
		settings: settings,
		origin:   origin,
		paddles:  paddles{left: paddleStartTop, right: paddleStartTop},
		ball:     resetBall(),
		serve:    Serve{Horizontal: origin.opening, Vertical: origin.vertical},
		computer: openedComputer(settings, origin),
		phase: countdownPhase{
			remaining:    CountdownSteps,
			token:        token,
			continuation: continuationStartServe,
		},
	}
}

func updateMatch(model Model, match matchState, event Event) (Model, Effect) {
	if phase, paused := match.phase.(pausedPhase); paused {
		return updatePause(model, match, phase, event)
	}
	if over, finished := match.phase.(gameOverPhase); finished {
		return updateGameOver(model, match, over, event)
	}

	switch event := event.(type) {
	case EventPauseToggle:
		return togglePause(model, match)
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
		match, effect = advanceCountdown(&model, match, phase, elapsed)
	case rallyPhase:
		match, effect = advanceRally(&model, match, phase, elapsed)
	case pausedPhase, gameOverPhase:
		return model, Effect{}
	}
	model.state = match

	return model, effect
}

// advanceCountdown spends elapsed time on the countdown, which requests its next timer while steps remain and enters
// the rally when they run out. The rally starts with its own timing sequence, launches the pending serve, and clears
// the scorer feedback. A completion from another timing sequence is a strict no-op.
func advanceCountdown(
	model *Model,
	match matchState,
	countdown countdownPhase,
	elapsed EventElapsed,
) (matchState, Effect) {
	if elapsed.Token != countdown.token {
		return match, Effect{}
	}

	var steps Steps
	match, steps = match.acceptedTime(elapsed.Elapsed)
	for range steps {
		if countdown.remaining == 0 {
			break
		}
		match.moveComputer(computerCenterRow)
		countdown.remaining--
	}
	if countdown.remaining == 0 {
		token := model.issueToken()
		match.elapsed = 0
		if countdown.continuation.startsServe() {
			match.ball = servedBall(match.serve)
			match.startServeApproach()
		} else {
			match.resumeComputer()
		}
		match.feedback = FeedbackNone
		match.phase = rallyPhase{token: token}

		return match, scheduleTimer(token, TimerDelay)
	}

	match.phase = countdown

	return match, scheduleTimer(countdown.token, TimerDelay)
}

// advanceRally spends elapsed time on the rally's fixed steps. Each step moves the ball and resolves its contacts;
// the moment the ball crosses a scoring boundary the point is awarded and the match moves to the next serve's
// countdown. The rest of that event's movement is discarded. A completion from another timing sequence is a strict
// no-op.
func advanceRally(model *Model, match matchState, rally rallyPhase, elapsed EventElapsed) (matchState, Effect) {
	if elapsed.Token != rally.token {
		return match, Effect{}
	}

	var steps Steps
	match, steps = match.acceptedTime(elapsed.Elapsed)
	for range steps {
		match.moveComputer(match.computer.target())
		match.ball = steppedBall(match.ball, match.paddles)
		if scorer, scored := match.ball.scorer(); scored {
			return scoredPoint(model, match, scorer)
		}
		match.observeComputer()
	}

	return match, scheduleTimer(rally.token, TimerDelay)
}

// scorer reports which player won the point when the ball has crossed a scoring boundary: the ball leaving the right
// edge concedes to the left player, and leaving the left edge concedes to the right player.
func (current ball) scorer() (Feedback, bool) {
	switch {
	case current.x > CourtWidth-1:
		return FeedbackLeftScored, true
	case current.x < 0:
		return FeedbackRightScored, true
	}

	return FeedbackNone, false
}

// scoredPoint atomically records the point, resets the ball to the court center, keeps both paddles, alternates the
// serve's vertical direction, and sends the next serve toward whoever conceded. A winning point instead enters Game
// over and requests no timer; any other point enters the new start-serve countdown.
func scoredPoint(model *Model, match matchState, scorer Feedback) (matchState, Effect) {
	serve := Serve{Horizontal: DirectionLeft, Vertical: model.chain.vertical.opposite()}
	switch scorer {
	case FeedbackLeftScored:
		match.score.left++
		serve.Horizontal = DirectionRight
	case FeedbackRightScored:
		match.score.right++
	case FeedbackNone:
	}

	model.chain.vertical = serve.Vertical
	match.serve = serve
	match.ball = resetBall()
	match.elapsed = 0
	match.computer.endApproach()
	if winner, won := match.winner(); won {
		match.feedback = FeedbackNone
		match.phase = gameOverPhase{winner: winner, focus: GameOverRowRematch}

		return match, Effect{}
	}

	token := model.issueToken()
	match.feedback = scorer
	match.phase = countdownPhase{
		remaining:    CountdownSteps,
		token:        token,
		continuation: continuationStartServe,
	}

	return match, scheduleTimer(token, TimerDelay)
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

// BallVelocity returns the ball's court velocity in cells per second. The pair carries the cell aspect correction, so
// the visual speed is the length of (vx, 2*vy). It is meaningful only while Screen is ScreenMatch.
func (model Model) BallVelocity() (vx, vy float64) {
	if match, ok := model.state.(matchState); ok {
		return match.ball.vx, match.ball.vy
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

// NextServe returns the directions the next serve will travel in: the serve the pending countdown will launch, or the
// serve that is in flight during a rally. It is meaningful only while Screen is ScreenMatch.
func (model Model) NextServe() Serve {
	if match, ok := model.state.(matchState); ok {
		return match.serve
	}

	return Serve{Horizontal: model.chain.opening, Vertical: model.chain.vertical}
}

// Feedback returns the scorer feedback the countdown after a point shows. It is FeedbackNone once the rally begins.
// It is meaningful only while Screen is ScreenMatch.
func (model Model) Feedback() Feedback {
	if match, ok := model.state.(matchState); ok {
		return match.feedback
	}

	return FeedbackNone
}
