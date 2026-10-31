package tui_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/calvertjadon/pongo/internal/core"
	"github.com/calvertjadon/pongo/internal/tui"
)

func TestTitleFitsMinimumTerminal(t *testing.T) {
	t.Parallel()

	view := tui.Render(core.NewModel(), 80, 28)
	if width := lipgloss.Width(view); width != 80 {
		t.Fatalf("title width = %d, want 80", width)
	}
	if height := lipgloss.Height(view); height != 28 {
		t.Fatalf("title height = %d, want 28", height)
	}
	if !strings.Contains(view, "PONGO") {
		t.Fatal("title omits its name")
	}
	if row, _ := locate(lines(view), "Enter"); row < 0 || !strings.Contains(lines(view)[row], "setup") {
		t.Error("title omits the Enter prompt for match setup")
	}
	if row, _ := locate(lines(view), "Q "); row < 0 || !strings.Contains(lines(view)[row], "Quit") {
		t.Error("title omits the Q prompt for quit")
	}
}

func TestTitleCentersFixedLayoutWithOddSurplus(t *testing.T) {
	t.Parallel()

	view := tui.Render(core.NewModel(), 83, 31)
	lines := strings.Split(view, "\n")
	row, column := locate(lines, "PONGO")

	if row != 12 || column != 38 {
		t.Fatalf("PONGO starts at row %d, column %d; want row 12, column 38", row, column)
	}
	if width := lipgloss.Width(view); width != 83 {
		t.Fatalf("title width = %d, want 83", width)
	}
	if height := lipgloss.Height(view); height != 31 {
		t.Fatalf("title height = %d, want 31", height)
	}
}

func locate(lines []string, text string) (int, int) {
	for row, line := range lines {
		if column := strings.Index(line, text); column >= 0 {
			return row, column
		}
	}

	return -1, -1
}

func lines(view string) []string {
	return strings.Split(view, "\n")
}
