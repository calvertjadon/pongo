package core

// GameOverRow identifies one row of the Game over menu, in display order.
type GameOverRow uint8

const (
	// GameOverRowRematch starts another match with the same settings.
	GameOverRowRematch GameOverRow = iota
	// GameOverRowChangeSetup opens Match setup holding the finished match's settings.
	GameOverRowChangeSetup
	// GameOverRowReturnToTitle leaves the match for the title.
	GameOverRowReturnToTitle
)

// Side names one half of the court.
type Side uint8

const (
	// SideLeft is the left player's side.
	SideLeft Side = iota
	// SideRight is the right player's side, defended by the second local player or the computer.
	SideRight
)

// gameOverPhase is a finished match. It records which side won the final point and the selected menu row.
type gameOverPhase struct {
	winner Side
	focus  GameOverRow
}

func (gameOverPhase) isMatchPhase() {}

func (gameOverPhase) tag() MatchPhase { return MatchPhaseGameOver }

// winner returns the side that has reached the locked winning score, if either has. Matches end at the score itself,
// so a winning point wins without a two-point margin.
func (match matchState) winner() (Side, bool) {
	winning := uint8(match.settings.WinningScore)

	switch {
	case match.score.left >= winning:
		return SideLeft, true
	case match.score.right >= winning:
		return SideRight, true
	}

	return SideLeft, false
}

// updateGameOver handles the Game over menu. A finished match requests no timers and ignores every event but menu
// navigation and the primary action.
func updateGameOver(model Model, match matchState, over gameOverPhase, event Event) (Model, Effect) {
	switch event := event.(type) {
	case EventNavigate:
		over.focus = over.focus.stepped(event.Direction)
		match.phase = over
		model.state = match

		return model, Effect{}
	case EventPrimary:
		switch over.focus {
		case GameOverRowRematch:
			return gameOverRematch(model, match)
		case GameOverRowChangeSetup:
			return gameOverChangeSetup(model, match)
		case GameOverRowReturnToTitle:
			return returnToTitle(model)
		}

		return model, Effect{}
	case EventPauseToggle, EventAdjust, EventBack, EventQuit, EventPaddleStep, EventElapsed:
		return model, Effect{}
	}

	return model, Effect{}
}

// stepped moves one menu row in direction, wrapping between the first and last rows.
func (row GameOverRow) stepped(direction VerticalDirection) GameOverRow {
	return wrapped(row, GameOverRowReturnToTitle+1, direction)
}

// gameOverRematch continues the chain and starts another match with the settings the finished one locked: the chain
// advances, its opening direction flips, and its vertical serve direction carries on from the final rally.
func gameOverRematch(model Model, finished matchState) (Model, Effect) {
	model.chain.position++
	model.chain.opening = model.chain.opening.opposite()

	return beginMatch(model, finished.settings)
}

// gameOverChangeSetup ends the chain, copies the finished match's locked settings into the session draft, and opens
// Match setup focused on Mode.
func gameOverChangeSetup(model Model, finished matchState) (Model, Effect) {
	model.draft = finished.settings
	model.chain = chain{}
	model.state = setupState{focus: SetupRowMode}

	return model, Effect{}
}

// GameOverFocus returns the focused Game over menu row. It is meaningful only while MatchPhase is MatchPhaseGameOver.
func (model Model) GameOverFocus() GameOverRow {
	if match, ok := model.state.(matchState); ok {
		if over, finished := match.phase.(gameOverPhase); finished {
			return over.focus
		}
	}

	return GameOverRowRematch
}

// Winner returns the side that won the finished match. It is meaningful only while MatchPhase is MatchPhaseGameOver.
func (model Model) Winner() Side {
	if match, ok := model.state.(matchState); ok {
		if over, finished := match.phase.(gameOverPhase); finished {
			return over.winner
		}
	}

	return SideLeft
}
