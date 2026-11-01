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

// timerTape builds timer completions on a synthetic clock: each completion is armed when the previous one fired and
// fires one period later, so every completion reports exactly one period whether it starts or continues a sequence.
type timerTape struct {
	fired time.Time
}

// newTimerTape starts a tape at a fixed synthetic time.
func newTimerTape() *timerTape {
	return &timerTape{fired: time.Date(2026, time.October, 31, 12, 0, 0, 0, time.UTC)}
}

// completion returns the next completion for token's timing sequence. No recovery has happened, so its issue is 0.
func (tape *timerTape) completion(token core.TimerToken) tui.TimerMsg {
	armed := tape.fired
	tape.fired = tape.fired.Add(tickPeriod)

	return tui.TimerMsg{Token: token, Armed: armed, Fired: tape.fired}
}

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

	tape := newTimerTape()
	for range 100 {
		updated, next := model.Update(tape.completion(1))
		model = updated

		if completed(model) {
			if next == nil {
				t.Error("countdown completion requested no timer, want the rally's timing sequence")
			}

			return
		}
		if next == nil {
			t.Fatal("a countdown completion advanced the countdown but requested no next timer")
		}
	}
	t.Fatal("countdown never completed through shell timers")
}

func TestRallyTimersMoveTheBallThroughTheShell(t *testing.T) {
	t.Parallel()

	model, _ := startMatchInShell(t)
	tape := newTimerTape()
	var countdownToken core.TimerToken = 1
	for range 100 {
		var next tea.Cmd
		model, next = model.Update(tape.completion(countdownToken))
		if completed(model) {
			if next == nil {
				t.Fatal("the countdown completion requested no rally timer")
			}
			countdownToken = 2 // the rally's own sequence

			break
		}
		if next == nil {
			t.Fatal("a countdown completion advanced the countdown but requested no next timer")
		}
	}

	before := model.View().Content
	updated, cmd := model.Update(tape.completion(countdownToken))
	if cmd == nil {
		t.Error("a rally completion requested no next timer")
	}
	if updated.View().Content == before {
		t.Error("a rally completion did not move the ball")
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
