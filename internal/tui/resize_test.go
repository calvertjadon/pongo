package tui_test

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/calvertjadon/pongo/internal/core"
	"github.com/calvertjadon/pongo/internal/tui"
)

// minimumSize is the inclusive smallest terminal that shows the fixed layout.
func minimumSize() tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: 80, Height: 28}
}

// undersizedSize is a terminal below the inclusive minimum in both dimensions.
func undersizedSize() tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: 60, Height: 20}
}

// gatedKeys are the keys an undersized terminal must drop before the core sees them.
func gatedKeys() []tea.KeyPressMsg {
	return []tea.KeyPressMsg{
		{Code: 'w', Text: "w"},
		{Code: 's', Text: "s", IsRepeat: true},
		{Code: tea.KeyUp},
		{Code: tea.KeyDown},
		{Code: tea.KeyEnter},
		{Code: tea.KeyEscape},
		{Code: tea.KeyLeft},
		{Code: tea.KeyRight},
		{Code: 'p', Text: "p"},
	}
}

// timerMessage builds the countdown completion one live sequence reports after tickPeriod.
func timerMessage(issue uint64, scheduled time.Time) tui.TimerMsg {
	return tui.TimerMsg{Token: 1, Issue: issue, Scheduled: scheduled, Fired: scheduled.Add(tickPeriod)}
}

func TestUndersizedTerminalFreezesTheMatch(t *testing.T) {
	t.Parallel()

	frozen, _ := startMatchInShell(t)
	prior := frozen.View().Content
	shrunk, cmd := frozen.Update(undersizedSize())
	if cmd != nil {
		t.Fatal("entering the undersized condition requested work")
	}
	notice := tui.Render(core.NewModel(), 60, 20)
	if got := shrunk.View().Content; got != notice {
		t.Fatalf("undersized match does not show the size notice")
	}

	for _, key := range gatedKeys() {
		updated, cmd := shrunk.Update(key)
		if cmd != nil {
			t.Errorf("undersized key %q produced a command, want none", key.String())
		}
		if updated.View().Content != notice {
			t.Errorf("undersized key %q changed the screen", key.String())
		}
	}

	clock := time.Unix(1_700_000_000, 0)
	for _, issue := range []uint64{1, 2} {
		message := timerMessage(issue, clock)
		updated, cmd := shrunk.Update(message)
		if cmd != nil {
			t.Errorf("undersized timer issue %d produced a command, want none", issue)
		}
		if updated.View().Content != notice {
			t.Errorf("undersized timer issue %d changed the screen", issue)
		}
	}

	recovered, cmd := shrunk.Update(minimumSize())
	if cmd == nil {
		t.Fatal("recovering from the undersized condition did not re-arm the countdown")
	}
	if got := recovered.View().Content; got != prior {
		t.Fatal("recovery does not show the exact preserved view")
	}
}

func TestRecoveredMatchContinuesWithoutCatchUp(t *testing.T) {
	t.Parallel()

	model, _ := startMatchInShell(t)
	clock := time.Unix(1_700_000_000, 0)
	model, issue, clock := tickCountdown(t, model, 1, clock)
	honored := 1

	shrunk, _ := model.Update(undersizedSize())
	for range 5 {
		updated, cmd := shrunk.Update(timerMessage(issue, clock))
		if cmd != nil {
			t.Fatal("an undersized timer completion requested the next one")
		}
		shrunk = updated
	}

	recovered, rearmed := shrunk.Update(minimumSize())
	if rearmed == nil {
		t.Fatal("recovery did not re-arm the countdown")
	}
	issue++ // recovery issues a fresh request number for the same sequence.

	delayed, cmd := recovered.Update(timerMessage(issue-1, clock))
	if cmd != nil {
		t.Fatal("the completion left in flight before recovery was honored")
	}
	if delayed.View().Content != recovered.View().Content {
		t.Fatal("the delayed completion changed the recovered view")
	}

	stepsPerTick := int(core.ElapsedLimit / core.StepDuration)
	want := (int(core.CountdownSteps) + stepsPerTick - 1) / stepsPerTick
	for {
		model, issue, clock = tickCountdown(t, recovered, issue, clock)
		recovered = model
		honored++
		if completed(recovered) {
			break
		}
		if honored > want {
			t.Fatalf("countdown took more than %d completions", want)
		}
	}
	if honored != want {
		t.Fatalf("countdown took %d completions, want %d", honored, want)
	}
}

func TestForeignTokenDoesNotEndTheLiveSequence(t *testing.T) {
	t.Parallel()

	model, _ := startMatchInShell(t)
	clock := time.Unix(1_700_000_000, 0)
	foreign, cmd := model.Update(tui.TimerMsg{Token: 99, Issue: 1, Scheduled: clock, Fired: clock.Add(tickPeriod)})
	if cmd != nil {
		t.Fatal("a foreign token requested the next timer")
	}

	updated, cmd := foreign.Update(timerMessage(1, clock))
	if cmd == nil {
		t.Fatal("the genuine completion after a foreign token requested no next timer")
	}
	if completed(updated) {
		t.Fatal("one completion after a foreign token ended the countdown")
	}
}

func TestUndersizedTerminalKeepsTheHostExitKeys(t *testing.T) {
	t.Parallel()

	match, _ := startMatchInShell(t)
	for _, model := range []tea.Model{sized(tui.NewModel()), match} {
		shrunk, _ := model.Update(undersizedSize())
		notice := shrunk.View().Content

		for _, key := range []tea.KeyPressMsg{{Code: 'q', Text: "q"}, {Code: 'q', Text: "Q", Mod: tea.ModShift}} {
			updated, cmd := shrunk.Update(key)
			if cmd == nil {
				t.Fatalf("undersized key %q produced no command, want host quit", key.String())
			}
			if _, quit := cmd().(tea.QuitMsg); !quit {
				t.Errorf("undersized key %q command does not quit the host", key.String())
			}
			if updated.View().Content != notice {
				t.Errorf("undersized key %q changed the screen", key.String())
			}
		}

		interrupted, cmd := shrunk.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		if cmd == nil {
			t.Fatal("undersized Ctrl+C produced no command, want an interrupt")
		}
		if _, interrupt := cmd().(tea.InterruptMsg); !interrupt {
			t.Error("undersized Ctrl+C command does not interrupt the host")
		}
		if interrupted.View().Content != notice {
			t.Error("undersized Ctrl+C changed the screen")
		}
	}
}

func TestAdequateResizeOnlyRecentersTheLayout(t *testing.T) {
	t.Parallel()

	application := startedMatchInShell(t)
	model, _ := startMatchInShell(t)
	prior := model.View().Content

	for _, size := range []tea.WindowSizeMsg{{Width: 81, Height: 29}, {Width: 83, Height: 31}, {Width: 101, Height: 35}} {
		updated, cmd := model.Update(size)
		if cmd != nil {
			t.Fatalf("%dx%d resize requested work", size.Width, size.Height)
		}
		view := updated.View().Content
		if view != tui.Render(application, size.Width, size.Height) {
			t.Fatalf("%dx%d resize does not show the unchanged match", size.Width, size.Height)
		}
		for _, line := range lines(view) {
			if lipgloss.Width(line) != size.Width {
				t.Fatalf("%dx%d resize left a row of width %d", size.Width, size.Height, lipgloss.Width(line))
			}
		}
		if lipgloss.Height(view) != size.Height {
			t.Fatalf("%dx%d resize left %d rows", size.Width, size.Height, lipgloss.Height(view))
		}
		model = updated
	}

	restored, _ := model.Update(minimumSize())
	if restored.View().Content != prior {
		t.Fatal("resizing away from the minimum changed the match state")
	}

	stepsPerTick := int(core.ElapsedLimit / core.StepDuration)
	want := (int(core.CountdownSteps) + stepsPerTick - 1) / stepsPerTick
	honored := 0
	clock := time.Unix(1_700_000_000, 0)
	for issue := uint64(1); !completed(restored); issue++ {
		restored, _, clock = tickCountdown(t, restored, issue, clock)
		honored++
		if honored > want {
			t.Fatalf("the resized countdown took more than %d completions", want)
		}
	}
	if honored != want {
		t.Fatalf("the resized countdown took %d completions, want %d", honored, want)
	}
}

func TestAdequateResizePreservesTheRally(t *testing.T) {
	t.Parallel()

	application, _ := startMatchInShell(t)
	clock := time.Unix(1_700_000_000, 0)
	issue := uint64(1)
	for !completed(application) {
		application, issue, clock = tickCountdown(t, application, issue, clock)
	}
	rally := finishCountdown(t, startedMatchInShell(t), 1)

	for _, size := range []tea.WindowSizeMsg{{Width: 101, Height: 35}, {Width: 81, Height: 29}} {
		updated, cmd := application.Update(size)
		if cmd != nil {
			t.Fatalf("%dx%d rally resize requested work", size.Width, size.Height)
		}
		if view := updated.View().Content; view != tui.Render(rally, size.Width, size.Height) {
			t.Fatalf("%dx%d rally resize does not show the unchanged rally", size.Width, size.Height)
		}
		application = updated
	}

	moved, _ := core.Update(rally, core.EventPaddleStep{Direction: core.DirectionUp, Group: core.BindingWS})
	stepped, cmd := application.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if cmd != nil {
		t.Fatal("a rally key produced a command, want none")
	}
	if view := stepped.View().Content; view != tui.Render(moved, 81, 29) {
		t.Fatal("a rally key after resizing did not reach the core")
	}
}

func TestRepeatedResizesUseTheLatestDimensions(t *testing.T) {
	t.Parallel()

	application := startedMatchInShell(t)
	model, _ := startMatchInShell(t)
	sizes := []tea.WindowSizeMsg{
		{Width: 79, Height: 28},
		{Width: 80, Height: 27},
		{Width: 60, Height: 20},
		{Width: 200, Height: 10},
		{Width: 100, Height: 40},
		{Width: 80, Height: 28},
	}
	recoveries := 0
	for _, size := range sizes {
		updated, cmd := model.Update(size)
		if cmd != nil {
			recoveries++
			if recoveries > 1 {
				t.Fatalf("%dx%d resize re-armed the countdown twice", size.Width, size.Height)
			}
		}
		model = updated

		want := tui.Render(core.NewModel(), size.Width, size.Height)
		if size.Width >= 80 && size.Height >= 28 {
			want = tui.Render(application, size.Width, size.Height)
		}
		if view := model.View().Content; view != want {
			t.Fatalf("%dx%d resize shows the wrong screen", size.Width, size.Height)
		}
	}
	if recoveries != 1 {
		t.Fatalf("the resizes recovered %d times, want once", recoveries)
	}
}

func TestUndersizedTitleAndSetupIgnoreInput(t *testing.T) {
	t.Parallel()

	title := sized(tui.NewModel())
	shrunkTitle, _ := title.Update(undersizedSize())
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyDown}, {Code: tea.KeyRight}} {
		updated, cmd := shrunkTitle.Update(key)
		if cmd != nil {
			t.Errorf("undersized title key %q produced a command, want none", key.String())
		}
		if updated.View().Content != shrunkTitle.View().Content {
			t.Errorf("undersized title key %q changed the screen", key.String())
		}
	}
	restoredTitle, cmd := shrunkTitle.Update(minimumSize())
	if cmd != nil {
		t.Error("title recovery requested a timer")
	}
	if restoredTitle.View().Content != tui.Render(core.NewModel(), 80, 28) {
		t.Fatal("undersized title input reached the core")
	}

	application, _ := core.Update(core.NewModel(), core.EventPrimary{})
	application, _ = core.Update(application, core.EventAdjust{Direction: core.DirectionLeft})
	shrunkSetup, _ := setupInShell().Update(tea.WindowSizeMsg{Width: 79, Height: 27})
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyDown}, {Code: tea.KeyLeft}, {Code: tea.KeyEnter}, {Code: tea.KeyEscape}} {
		updated, cmd := shrunkSetup.Update(key)
		if cmd != nil {
			t.Errorf("undersized setup key %q produced a command, want none", key.String())
		}
		if updated.View().Content != shrunkSetup.View().Content {
			t.Errorf("undersized setup key %q changed the screen", key.String())
		}
	}
	restoredSetup, cmd := shrunkSetup.Update(minimumSize())
	if cmd != nil {
		t.Error("setup recovery requested a timer")
	}
	if restoredSetup.View().Content != tui.Render(application, 80, 28) {
		t.Fatal("undersized setup input reached the core")
	}
}

// tickCountdown honors one countdown completion through the shell and returns the model, the next request number, and
// the clock it leaves behind.
func tickCountdown(t *testing.T, model tea.Model, issue uint64, clock time.Time) (tea.Model, uint64, time.Time) {
	t.Helper()

	fired := clock.Add(tickPeriod)
	updated, cmd := model.Update(tui.TimerMsg{Token: 1, Issue: issue, Scheduled: clock, Fired: fired})
	if cmd == nil && !completed(updated) {
		t.Fatalf("countdown completion issue %d did not request the next timer", issue)
	}

	return updated, issue + 1, fired
}
