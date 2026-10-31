package core

// SetupRow identifies one row of the Match setup form, in display order.
type SetupRow uint8

const (
	// SetupRowMode selects the Mode.
	SetupRowMode SetupRow = iota
	// SetupRowWinningScore selects the Winning score.
	SetupRowWinningScore
	// SetupRowDifficulty selects the Computer difficulty. It is unavailable in Local two-player.
	SetupRowDifficulty
	// SetupRowStart is the Start match action.
	SetupRowStart
)

func updateSetup(model Model, setup setupState, event Event) (Model, Effect) {
	switch event := event.(type) {
	case EventNavigate:
		model.state = setupState{focus: nextAvailableRow(setup.focus, event.Direction, model.draft.Mode)}
		return model, Effect{}
	case EventAdjust:
		model.draft = adjustDraft(model.draft, setup.focus, event.Direction)
		return model, Effect{}
	case EventPrimary:
		if setup.focus == SetupRowStart {
			return startMatch(model)
		}

		model.state = setupState{focus: nextAvailableRow(setup.focus, DirectionDown, model.draft.Mode)}
		return model, Effect{}
	case EventBack:
		model.state = titleState{}
		return model, Effect{}
	case EventQuit, EventPaddleStep, EventElapsed:
		return model, Effect{}
	}

	return model, Effect{}
}

// nextAvailableRow moves one available row in direction and wraps. Computer difficulty is unavailable in Local
// two-player.
func nextAvailableRow(row SetupRow, direction VerticalDirection, mode Mode) SetupRow {
	next := stepRow(row, direction)
	if next == SetupRowDifficulty && mode == ModeLocalTwoPlayer {
		next = stepRow(next, direction)
	}

	return next
}

// stepRow moves one row in direction, wrapping between the first and last rows.
func stepRow(row SetupRow, direction VerticalDirection) SetupRow {
	const rows = SetupRowStart + 1
	switch direction {
	case DirectionUp:
		return (row + rows - 1) % rows
	case DirectionDown:
		return (row + 1) % rows
	}

	return row
}

// adjustDraft changes only the field on the focused row. Start match has no value.
func adjustDraft(draft Settings, focus SetupRow, direction HorizontalDirection) Settings {
	switch focus {
	case SetupRowMode:
		draft.Mode = draft.Mode.toggled()
	case SetupRowWinningScore:
		draft.WinningScore = draft.WinningScore.cycled(direction)
	case SetupRowDifficulty:
		draft.Difficulty = draft.Difficulty.cycled(direction)
	case SetupRowStart:
	}

	return draft
}
