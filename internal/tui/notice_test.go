package tui_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/calvertjadon/pongo/internal/core"
	"github.com/calvertjadon/pongo/internal/tui"
)

func TestUndersizedNoticeClipsToTheTerminal(t *testing.T) {
	t.Parallel()

	view := tui.Render(core.NewModel(), 20, 3)
	want := strings.Join([]string{
		"       PONGO        ",
		"  Terminal: 20 x 3  ",
		" Required: 80 x 28  ",
	}, "\n")

	if view != want {
		t.Fatalf("notice = %q, want %q", view, want)
	}
	if tiny := tui.Render(core.NewModel(), 1, 1); tiny != "P" {
		t.Errorf("one-by-one notice = %q, want %q", tiny, "P")
	}
	for _, size := range []struct{ width, height int }{{width: 0, height: 20}, {width: 20, height: 0}, {width: -4, height: -3}} {
		if view := tui.Render(core.NewModel(), size.width, size.height); view != "" {
			t.Errorf("%dx%d notice = %q, want nothing", size.width, size.height, view)
		}
	}
}

func TestUndersizedNoticeIsCenteredAndComplete(t *testing.T) {
	t.Parallel()

	const (
		width  = 79
		height = 27
	)
	rows := lines(tui.Render(core.NewModel(), width, height))
	if len(rows) != height {
		t.Fatalf("notice has %d rows, want %d", len(rows), height)
	}

	contents := []string{"PONGO", "Terminal: 79 x 27", "Required: 80 x 28", "Resize the terminal to continue."}
	top := (height - len(contents)) / 2
	for row, content := range contents {
		line := rows[top+row]
		if !strings.Contains(line, content) {
			t.Fatalf("notice row %d = %q, want %q", top+row, line, content)
		}
		if left := strings.Index(line, content); left != (width-len(content))/2 || len(line) != width {
			t.Errorf("notice row %d = %q, want %q centered in %d columns", top+row, line, content, width)
		}
	}
	for row, line := range rows {
		if row < top || row >= top+len(contents) {
			if strings.TrimSpace(line) != "" {
				t.Errorf("notice row %d = %q, want a blank row", row, line)
			}
		}
	}
}

func TestUndersizedNoticeNeverExceedsTheTerminal(t *testing.T) {
	t.Parallel()

	for width := range 80 {
		for height := range 28 {
			assertNoticeFits(t, width, height)
		}
	}
}

// assertNoticeFits checks that one undersized render never writes outside its terminal.
func assertNoticeFits(t *testing.T, width, height int) {
	t.Helper()

	view := tui.Render(core.NewModel(), width, height)
	if width == 0 || height == 0 {
		if view != "" {
			t.Fatalf("%dx%d notice = %q, want nothing", width, height, view)
		}

		return
	}
	if got := lipgloss.Height(view); got != height {
		t.Fatalf("%dx%d notice height = %d", width, height, got)
	}
	for row, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("%dx%d notice row %d width = %d: %q", width, height, row, got, line)
		}
	}
}

func TestUndersizedNoticeReplacesEveryScreen(t *testing.T) {
	t.Parallel()

	notice := tui.Render(core.NewModel(), 60, 20)
	if !strings.Contains(notice, "Required: 80 x 28") {
		t.Fatalf("notice = %q, want the required dimensions", notice)
	}

	setupModel, _ := core.Update(core.NewModel(), core.EventPrimary{})
	matchModel, _ := startMatch(t, core.ModeLocalTwoPlayer)
	for _, application := range []core.Model{setupModel, matchModel} {
		if view := tui.Render(application, 60, 20); view != notice {
			t.Errorf("screen %v shows %q when undersized, want the notice", application.Screen(), view)
		}
	}
}

func TestMinimumTerminalIsInclusive(t *testing.T) {
	t.Parallel()

	if view := tui.Render(core.NewModel(), 80, 28); strings.Contains(view, "Required:") {
		t.Error("80 by 28 shows the size notice, want the title")
	}
	for _, size := range []struct{ width, height int }{{width: 79, height: 28}, {width: 80, height: 27}} {
		view := tui.Render(core.NewModel(), size.width, size.height)
		if !strings.Contains(view, "Required: 80 x 28") {
			t.Errorf("%dx%d does not show the size notice", size.width, size.height)
		}
	}
}
