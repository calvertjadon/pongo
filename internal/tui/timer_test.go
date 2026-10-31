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

// timerTape supplies completion fire times on a synthetic clock that advances one period per completion. Anchoring it
// once keeps the reported elapsed time at the requested period however fast or slow the test runs.
type timerTape struct {
	fired time.Time
}

// newTimerTape starts a tape at the host clock.
func newTimerTape() *timerTape {
	return &timerTape{fired: time.Now()}
}

// next returns the tape's next fire time.
func (tape *timerTape) next(period time.Duration) time.Time {
	tape.fired = tape.fired.Add(period)

	return tape.fired
}

// probeTimer delivers one completion for the shell's live timer request at the given fire time. It discovers the
// request number from the shell's response instead of assuming the private numbering, and returns the updated model,
// the shell's next timer request, and the completion it honored.
func probeTimer(
	t *testing.T,
	model tea.Model,
	token core.TimerToken,
	fired time.Time,
) (tea.Model, tea.Cmd, tui.TimerMsg) {
	t.Helper()

	for issue := uint64(0); issue < 2048; issue++ {
		message := tui.TimerMsg{Token: token, Issue: issue, Fired: fired}
		updated, cmd := model.Update(message)
		if cmd != nil {
			return updated, cmd, message
		}
	}
	t.Fatalf("no completion matched the shell's live timer request for token %d", token)

	return model, nil, tui.TimerMsg{}
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
		updated, next, _ := probeTimer(t, model, 1, tape.next(tickPeriod))
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
		model, next, _ = probeTimer(t, model, countdownToken, tape.next(tickPeriod))
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
	updated, cmd, _ := probeTimer(t, model, countdownToken, tape.next(tickPeriod))
	if cmd == nil {
		t.Error("a rally completion requested no next timer")
	}
	if updated.View().Content == before {
		t.Error("a rally completion did not move the ball")
	}
}

func TestShellDropsObsoleteTimerCompletions(t *testing.T) {
	t.Parallel()

	model, _ := startMatchInShell(t)
	once, cmd, message := probeTimer(t, model, 1, newTimerTape().next(tickPeriod))
	if cmd == nil {
		t.Fatal("the first completion requested no next timer")
	}

	stale, staleCmd := once.Update(message)
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

	foreign, cmd := model.Update(tui.TimerMsg{Token: 99, Fired: time.Now().Add(tickPeriod)})
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
	for _, message := range []tui.TimerMsg{
		{Token: 1, Issue: 0, Fired: time.Now().Add(tickPeriod)},
		{Token: 1, Issue: 1, Fired: time.Now().Add(tickPeriod)},
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

func TestCompletionReportsTheGapSinceThePreviousCompletion(t *testing.T) {
	t.Parallel()

	model, _ := startMatchInShell(t)
	application := startedMatchInShell(t)
	var countdownToken core.TimerToken = 1

	// The first completion fired 100 ms before the loop handled it, so it contributes no time; the second fires now,
	// 100 ms after the first. Measuring from the previous fire reports the whole gap, while measuring from when the
	// next timer was requested would lose the stalled 100 ms.
	fired := time.Now().Add(-100 * time.Millisecond)
	model, next, _ := probeTimer(t, model, countdownToken, fired)
	if next == nil {
		t.Fatal("the first completion requested no next timer")
	}
	application, _ = core.Update(application, core.EventElapsed{Token: countdownToken})

	model, next, _ = probeTimer(t, model, countdownToken, fired.Add(100*time.Millisecond))
	if next == nil {
		t.Fatal("the delayed completion requested no next timer")
	}
	application, _ = core.Update(
		application,
		core.EventElapsed{Elapsed: 12000, Token: countdownToken}, // 100 milliseconds in core units
	)
	if got, want := model.View().Content, tui.Render(application, 80, 28); got != want {
		t.Fatal("the shell did not report the stalled completion's full elapsed time")
	}
}
