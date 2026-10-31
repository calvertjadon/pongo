package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/calvertjadon/pongo/internal/core"
)

// setupRowWidth fits the active marker, the widest label column, and the widest editable value.
const setupRowWidth = 2 + 22 + len("< Local two-player >")

func renderSetup(draft core.Settings, focus core.SetupRow) string {
	rows := []string{
		setupRow(focus == core.SetupRowMode, "Mode", editable(modeName(draft.Mode))),
		"",
		setupRow(focus == core.SetupRowWinningScore, "Winning score", editable(strconv.Itoa(int(draft.WinningScore)))),
		"",
		setupRow(focus == core.SetupRowDifficulty, "Computer difficulty", difficultyValue(draft)),
		"",
		setupRow(focus == core.SetupRowStart, "Start match", ""),
	}
	form := lipgloss.NewStyle().Border(lipgloss.ASCIIBorder()).Padding(1, 3).Render(strings.Join(rows, "\n"))
	controls := "Up/Down move   Left/Right change   Enter accept   Esc back"

	return lipgloss.JoinVertical(lipgloss.Center, "MATCH SETUP", "", form, "", controls)
}

func setupRow(active bool, label, value string) string {
	marker := "  "
	if active {
		marker = "> "
	}

	return fmt.Sprintf("%-*s", setupRowWidth, fmt.Sprintf("%s%-22s%s", marker, label, value))
}

func editable(value string) string {
	return "< " + value + " >"
}

func difficultyValue(draft core.Settings) string {
	switch draft.Mode {
	case core.ModeLocalTwoPlayer:
		return "--"
	case core.ModeSinglePlayer:
		return editable(difficultyName(draft.Difficulty))
	default:
		return editable(difficultyName(draft.Difficulty))
	}
}

func modeName(mode core.Mode) string {
	switch mode {
	case core.ModeSinglePlayer:
		return "Single player"
	case core.ModeLocalTwoPlayer:
		return "Local two-player"
	default:
		return ""
	}
}

func difficultyName(difficulty core.Difficulty) string {
	switch difficulty {
	case core.DifficultyEasy:
		return "Easy"
	case core.DifficultyNormal:
		return "Normal"
	case core.DifficultyHard:
		return "Hard"
	default:
		return ""
	}
}
