package tui_test

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/calvertjadon/pongo/internal/core"
	"github.com/calvertjadon/pongo/internal/tui"
)

// tickPeriod is the host time one shell timer reports: 75 ms, or nine fixed steps.
const tickPeriod = 75 * time.Millisecond

func TestStartingAMatchSchedulesATimerForTheCountdown(t *testing.T) {
	t.Parallel()

	_, cmd := startMatchInShell(t)
	if cmd == nil {
		t.Fatal("starting a match scheduled no timer")
	}
}

func TestCountdownCompletesThroughShellTimers(t *testing.T) {
	t.Parallel()

	model, cmd := startMatchInShell(t)
	if cmd == nil {
		t.Fatal("starting a match scheduled no timer")
	}

	clock := time.Unix(1_700_000_000, 0)
	for issue := uint64(1); ; issue++ {
		fired := clock.Add(tickPeriod)
		updated, next := model.Update(tui.TimerMsg{Token: 1, Issue: issue, Scheduled: clock, Fired: fired})
		model, clock = updated, fired

		if completed(model) {
			if next != nil {
				t.Error("countdown completion requested another timer, want none")
			}

			return
		}
		if next == nil {
			t.Fatalf("timer issue %d advanced the countdown but requested no next timer", issue)
		}
		if issue == 100 {
			t.Fatal("countdown never completed through shell timers")
		}
	}
}

func TestShellDropsObsoleteTimerCompletions(t *testing.T) {
	t.Parallel()

	model, _ := startMatchInShell(t)
	clock := time.Unix(1_700_000_000, 0)
	once, cmd := model.Update(tui.TimerMsg{Token: 1, Issue: 1, Scheduled: clock, Fired: clock.Add(tickPeriod)})
	if cmd == nil {
		t.Fatal("the first completion requested no next timer")
	}

	stale, staleCmd := once.Update(tui.TimerMsg{Token: 1, Issue: 1, Scheduled: clock, Fired: clock.Add(tickPeriod)})
	if staleCmd != nil {
		t.Error("an obsolete completion requested another timer")
	}
	if stale.View().Content != once.View().Content {
		t.Error("an obsolete completion changed the view")
	}
}

func TestShellReportsForeignTokensThatTheCoreIgnores(t *testing.T) {
	t.Parallel()

	model, _ := startMatchInShell(t)
	clock := time.Unix(1_700_000_000, 0)

	foreign, cmd := model.Update(tui.TimerMsg{Token: 99, Issue: 1, Scheduled: clock, Fired: clock.Add(tickPeriod)})
	if cmd != nil {
		t.Error("a foreign token requested the next timer")
	}
	if foreign.View().Content != model.View().Content {
		t.Error("a foreign token changed the view")
	}
}

func TestMatchKeysTranslateToPaddleStepsForTheirBindingGroup(t *testing.T) {
	t.Parallel()

	keys := []tea.KeyPressMsg{
		{Code: 'w', Text: "w"},
		{Code: 's', Text: "s", IsRepeat: true},
		{Code: 'W', Text: "W", Mod: tea.ModShift},
		{Code: tea.KeyUp},
		{Code: tea.KeyDown, IsRepeat: true},
	}
	events := []core.Event{
		core.EventPaddleStep{Direction: core.DirectionUp, Group: core.BindingWS},
		core.EventPaddleStep{Direction: core.DirectionDown, Group: core.BindingWS},
		core.EventPaddleStep{Direction: core.DirectionUp, Group: core.BindingWS},
		core.EventPaddleStep{Direction: core.DirectionUp, Group: core.BindingArrows},
		core.EventPaddleStep{Direction: core.DirectionDown, Group: core.BindingArrows},
	}

	model, _ := startMatchInShell(t)
	application := startedMatchInShell(t)
	for index, key := range keys {
		updated, cmd := model.Update(key)
		application, _ = core.Update(application, events[index])

		if cmd != nil {
			t.Fatalf("key %q produced a command, want none", key.String())
		}
		if updated.View().Content != tui.Render(application, 80, 28) {
			t.Fatalf("key %q does not move the paddle for %#v", key.String(), events[index])
		}
		model = updated
	}
}

func TestMovementKeysOutsideAMatchDoNothing(t *testing.T) {
	t.Parallel()

	model := setupInShell()
	for _, key := range []tea.KeyPressMsg{
		{Code: 'w', Text: "w"},
		{Code: 's', Text: "s"},
		{Code: 'W', Text: "W", Mod: tea.ModShift},
	} {
		updated, cmd := model.Update(key)
		if cmd != nil {
			t.Errorf("key %q produced a command, want none", key.String())
		}
		if updated.View().Content != model.View().Content {
			t.Errorf("key %q changed the setup view", key.String())
		}
	}
}

// startMatchInShell drives the shell keys that start a Local two-player match and returns the model and the command
// the starting key requested.
func startMatchInShell(t *testing.T) (tea.Model, tea.Cmd) {
	t.Helper()

	model := setupInShell()
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyUp})

	return model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// setupInShell opens Match setup in the shell and toggles it to Local two-player.
func setupInShell() tea.Model {
	model := sized(tui.NewModel())
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyLeft})

	return model
}

// startedMatchInShell returns the core state the shell reaches by starting a Local two-player match.
func startedMatchInShell(t *testing.T) core.Model {
	t.Helper()

	model, _ := core.Update(core.NewModel(), core.EventPrimary{})
	model, _ = core.Update(model, core.EventAdjust{Direction: core.DirectionLeft})
	model, _ = core.Update(model, core.EventNavigate{Direction: core.DirectionUp})
	started, _ := core.Update(model, core.EventPrimary{})
	if started.Screen() != core.ScreenMatch {
		t.Fatalf("core replay left screen %v, want a match", started.Screen())
	}

	return started
}

// completed reports whether the shell's view shows the ball over the court center, which is where the countdown ends.
func completed(model tea.Model) bool {
	return lines(model.View().Content)[frameCenterRow][frameCenterCol] == 'o'
}

func TestShellIgnoresTimersItNeverRequested(t *testing.T) {
	t.Parallel()

	model := sized(tui.NewModel())
	clock := time.Unix(1_700_000_000, 0)
	for _, message := range []tui.TimerMsg{
		{Token: 1, Issue: 0, Scheduled: clock, Fired: clock.Add(tickPeriod)},
		{Token: 1, Issue: 1, Scheduled: clock, Fired: clock.Add(tickPeriod)},
	} {
		updated, cmd := model.Update(message)
		if cmd != nil {
			t.Errorf("message %+v produced a command, want none", message)
		}
		if updated.View().Content != model.View().Content {
			t.Errorf("message %+v changed the title view", message)
		}
	}
}
