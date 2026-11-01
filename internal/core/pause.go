package core

// PauseRow identifies one row of the pause menu, in display order.
type PauseRow uint8

const (
	// PauseRowResume continues the frozen match after a fresh countdown.
	PauseRowResume PauseRow = iota
	// PauseRowRestart restarts the unfinished match from its original start conditions.
	PauseRowRestart
	// PauseRowReturnToTitle discards the match and keeps the setup draft.
	PauseRowReturnToTitle
)

// pausedPhase freezes a match behind the pause menu. It keeps the suspended phase exactly as it was, so the frozen
// court shows the state the player left.
type pausedPhase struct {
	suspended matchPhase
	focus     PauseRow
}

func (pausedPhase) isMatchPhase() {}

func (pausedPhase) tag() MatchPhase { return MatchPhasePaused }

// stepped returns the pause menu row one navigation step away, wrapping between the first and last rows.
func (row PauseRow) stepped(direction VerticalDirection) PauseRow {
	return wrapped(row, PauseRowReturnToTitle+1, direction)
}

// togglePause freezes a match in play behind the pause menu, or resumes a paused one. Freezing requests no timer: the
// suspended timing sequence is over and every later timer belonging to it is a no-op.
func togglePause(model Model, match matchState) (Model, Effect) {
	if phase, paused := match.phase.(pausedPhase); paused {
		return resumeMatch(model, match, phase)
	}
	match.phase = pausedPhase{suspended: match.phase, focus: PauseRowResume}
	model.state = match

	return model, Effect{}
}

// updatePause applies one event while the match is paused. Only the menu moves: paddle, setup, back, quit, and
// elapsed-time events change nothing.
func updatePause(model Model, match matchState, paused pausedPhase, event Event) (Model, Effect) {
	switch event := event.(type) {
	case EventNavigate:
		paused.focus = paused.focus.stepped(event.Direction)
		match.phase = paused
		model.state = match

		return model, Effect{}
	case EventPrimary:
		return runPauseRow(model, match, paused)
	case EventPauseToggle:
		return resumeMatch(model, match, paused)
	case EventAdjust, EventBack, EventPaddleStep, EventElapsed, EventQuit:
		return model, Effect{}
	}

	return model, Effect{}
}

// runPauseRow performs the pause menu action the player selected.
func runPauseRow(model Model, match matchState, paused pausedPhase) (Model, Effect) {
	switch paused.focus {
	case PauseRowResume:
		return resumeMatch(model, match, paused)
	case PauseRowRestart:
		return restartMatch(model, match)
	case PauseRowReturnToTitle:
		return returnToTitle(model)
	}

	return model, Effect{}
}

// restartMatch restores the unfinished match's original start conditions and resets everything it accumulated: scores,
// paddles, ball, rally data, feedback, and timing. Locked settings survive, and the match enters its opening countdown
// under a new timing sequence.
func restartMatch(model Model, match matchState) (Model, Effect) {
	model.chain = match.origin

	return beginMatch(model, match.settings)
}

// returnToTitle discards the match and its rematch chain while keeping the session's setup draft.
func returnToTitle(model Model) (Model, Effect) {
	model.state = titleState{}
	model.chain = chain{}

	return model, Effect{}
}

// resumeMatch enters a fresh full countdown under a new timing sequence. Its continuation remembers what the pause
// interrupted, so completing it launches the pending serve or continues the frozen rally exactly.
func resumeMatch(model Model, match matchState, paused pausedPhase) (Model, Effect) {
	token := model.issueToken()
	match.elapsed = 0
	match.suspendComputer()
	match.phase = newCountdown(token, resumedContinuation(paused.suspended))
	model.state = match

	return model, scheduleTimer(token, TimerDelay)
}

// suspendComputer discards the computer's target for a resume countdown while keeping the error drawn for the inbound
// approach. The full reaction delay restarts when rally play resumes.
func (match *matchState) suspendComputer() {
	if !match.settings.singlePlayer() {
		return
	}
	match.computer.suspend()
}

// resumedContinuation returns what the countdown after a pause must do when it completes.
func resumedContinuation(suspended matchPhase) countdownContinuation {
	switch suspended := suspended.(type) {
	case countdownPhase:
		return suspended.continuation.resumed()
	case rallyPhase:
		return continuationResumeRally
	case pausedPhase:
		return resumedContinuation(suspended.suspended)
	case gameOverPhase:
		// A finished match is never paused, so this mapping is unreachable; Game over is not a timed phase.
		return continuationResumeRally
	}

	return continuationResumeRally
}

// PauseFocus returns the focused pause-menu row. It is meaningful only while MatchPhase is MatchPhasePaused.
func (model Model) PauseFocus() PauseRow {
	if match, ok := model.state.(matchState); ok {
		if phase, paused := match.phase.(pausedPhase); paused {
			return phase.focus
		}
	}

	return PauseRowResume
}

// BallVisible reports whether the court draws the ball. It stays visible during a rally, while paused, and through
// the resume countdown of a frozen rally; a serve countdown and game over hide it.
func (model Model) BallVisible() bool {
	match, ok := model.state.(matchState)
	if !ok {
		return false
	}

	switch phase := match.phase.(type) {
	case countdownPhase:
		return phase.continuation == continuationResumeRally
	case rallyPhase, pausedPhase:
		return true
	case gameOverPhase:
		return false
	}

	return false
}
