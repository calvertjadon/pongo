package core

// Mode selects who controls the right paddle.
type Mode uint8

const (
	// ModeSinglePlayer pits the player against the computer.
	ModeSinglePlayer Mode = iota
	// ModeLocalTwoPlayer lets two people share one keyboard.
	ModeLocalTwoPlayer
)

func (mode Mode) toggled() Mode {
	switch mode {
	case ModeSinglePlayer:
		return ModeLocalTwoPlayer
	case ModeLocalTwoPlayer:
		return ModeSinglePlayer
	}

	return mode
}

// WinningScore is the number of points that wins a match. Its value is the point total.
type WinningScore uint8

const (
	// WinningScore5 plays a short match.
	WinningScore5 WinningScore = 5
	// WinningScore7 plays a standard match.
	WinningScore7 WinningScore = 7
	// WinningScore11 plays a long match.
	WinningScore11 WinningScore = 11
)

// cycled returns the neighboring winning score in direction, wrapping at both ends.
func (score WinningScore) cycled(direction HorizontalDirection) WinningScore {
	switch score {
	case WinningScore5:
		return choose(direction, WinningScore11, WinningScore7)
	case WinningScore7:
		return choose(direction, WinningScore5, WinningScore11)
	case WinningScore11:
		return choose(direction, WinningScore7, WinningScore5)
	}

	return score
}

// Difficulty selects the computer player's profile.
type Difficulty uint8

const (
	// DifficultyEasy is the most forgiving computer player.
	DifficultyEasy Difficulty = iota
	// DifficultyNormal is the middle computer player.
	DifficultyNormal
	// DifficultyHard is the toughest computer player.
	DifficultyHard
)

// cycled returns the neighboring difficulty in direction, wrapping at both ends.
func (difficulty Difficulty) cycled(direction HorizontalDirection) Difficulty {
	switch difficulty {
	case DifficultyEasy:
		return choose(direction, DifficultyHard, DifficultyNormal)
	case DifficultyNormal:
		return choose(direction, DifficultyEasy, DifficultyHard)
	case DifficultyHard:
		return choose(direction, DifficultyNormal, DifficultyEasy)
	}

	return difficulty
}

// Settings are the choices that configure a match. Difficulty is kept in Local two-player even though it does not
// apply there.
type Settings struct {
	Mode         Mode
	WinningScore WinningScore
	Difficulty   Difficulty
}

func defaultSettings() Settings {
	return Settings{Mode: ModeSinglePlayer, WinningScore: WinningScore7, Difficulty: DifficultyNormal}
}

// choose returns leftward for DirectionLeft and rightward for DirectionRight.
func choose[T any](direction HorizontalDirection, leftward, rightward T) T {
	switch direction {
	case DirectionLeft:
		return leftward
	case DirectionRight:
		return rightward
	}

	return rightward
}
