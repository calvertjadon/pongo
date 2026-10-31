package tui_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/calvertjadon/pongo/internal/core"
	"github.com/calvertjadon/pongo/internal/tui"
)

func setupLabels() [4]string {
	return [...]string{"Mode", "Winning score", "Computer difficulty", "Start match"}
}

func TestSetupFormFitsMinimumTerminal(t *testing.T) {
	t.Parallel()

	view := tui.Render(setup(), 80, 28)
	if width := lipgloss.Width(view); width != 80 {
		t.Fatalf("setup width = %d, want 80", width)
	}
	if height := lipgloss.Height(view); height != 28 {
		t.Fatalf("setup height = %d, want 28", height)
	}
	for _, label := range setupLabels() {
		if count := strings.Count(view, label); count != 1 {
			t.Errorf("label %q appears %d times, want once on one unwrapped line", label, count)
		}
	}
}

func TestSetupMarksActiveRowAndEditableValues(t *testing.T) {
	t.Parallel()

	view := lines(tui.Render(setup(), 80, 28))
	want := map[string]string{
		"Mode":                "< Single player >",
		"Winning score":       "< 7 >",
		"Computer difficulty": "< Normal >",
	}
	for label, value := range want {
		if row := rowOf(t, view, label); !strings.Contains(row, value) {
			t.Errorf("row %q = %q, want editable value %q", label, row, value)
		}
	}
	if row := rowOf(t, view, "Start match"); strings.ContainsAny(row, "<>") {
		t.Errorf("Start match row %q has markers, want none", row)
	}
	assertActiveRow(t, view, "Mode")

	down := core.EventNavigate{Direction: core.DirectionDown}
	assertActiveRow(t, lines(tui.Render(setup(down), 80, 28)), "Winning score")
	assertActiveRow(t, lines(tui.Render(setup(down, down, down), 80, 28)), "Start match")
}

func TestSetupShowsUnavailableDifficultyInLocalTwoPlayer(t *testing.T) {
	t.Parallel()

	view := lines(tui.Render(setup(core.EventAdjust{Direction: core.DirectionRight}), 80, 28))
	if row := rowOf(t, view, "Mode"); !strings.Contains(row, "< Local two-player >") {
		t.Errorf("Mode row = %q, want local two-player", row)
	}
	row := rowOf(t, view, "Computer difficulty")
	if !strings.Contains(row, "--") || strings.ContainsAny(row, "<>") || strings.Contains(row, "Normal") {
		t.Errorf("difficulty row = %q, want -- without editable markers or the kept value", row)
	}
}

func TestSetupCentersFixedLayoutWithOddSurplus(t *testing.T) {
	t.Parallel()

	model := setup()
	baseRow, baseColumn := locate(lines(tui.Render(model, 80, 28)), "Mode")
	tests := []struct{ width, height int }{
		{width: 81, height: 29},
		{width: 82, height: 30},
		{width: 83, height: 31},
		{width: 101, height: 35},
	}
	for _, test := range tests {
		view := tui.Render(model, test.width, test.height)
		if lipgloss.Width(view) != test.width || lipgloss.Height(view) != test.height {
			t.Errorf("%dx%d view is %dx%d", test.width, test.height, lipgloss.Width(view), lipgloss.Height(view))
		}
		columnShift, rowShift := (test.width-80)/2, (test.height-28)/2
		row, column := locate(lines(view), "Mode")
		if row != baseRow+rowShift || column != baseColumn+columnShift {
			t.Errorf("%dx%d: Mode at row %d, column %d; want row %d, column %d",
				test.width, test.height, row, column, baseRow+rowShift, baseColumn+columnShift)
		}
	}
}

func setup(events ...core.Event) core.Model {
	model, _ := core.Update(core.NewModel(), core.EventPrimary{})
	for _, event := range events {
		model, _ = core.Update(model, event)
	}

	return model
}

func rowOf(t *testing.T, view []string, label string) string {
	t.Helper()

	row, _ := locate(view, label)
	if row < 0 {
		t.Fatalf("view has no %q row", label)
	}

	return view[row]
}

func assertActiveRow(t *testing.T, view []string, active string) {
	t.Helper()

	for _, label := range setupLabels() {
		marked := strings.Contains(rowOf(t, view, label), "> "+label)
		if marked != (label == active) {
			t.Errorf("row %q active marker = %t, want %t", label, marked, label == active)
		}
	}
}
