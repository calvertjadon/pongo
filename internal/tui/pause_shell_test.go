package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/calvertjadon/pongo/internal/core"
	"github.com/calvertjadon/pongo/internal/tui"
)

func TestPauseKeyFreezesTheMatchInTheShell(t *testing.T) {
	t.Parallel()

	model, _ := startMatchInShell(t)
	application := startedMatchInShell(t)
	model, _, first := probeTimer(t, model, 1, newTimerTape().next(tickPeriod))
	application, _ = core.Update(
		application,
		core.EventElapsed{Elapsed: core.ElapsedLimit, Token: 1},
	)

	paused, cmd := model.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if cmd != nil {
		t.Error("the pause key produced a command, want none")
	}

	frozen, _ := core.Update(application, core.EventPauseToggle{})
	if content := paused.View().Content; content != tui.Render(frozen, 80, 28) {
		t.Error("the pause key does not show the frozen match behind the overlay")
	}
	if !strings.Contains(paused.View().Content, "PAUSED") {
		t.Error("the paused view omits the pause overlay")
	}

	// The countdown timer the shell already requested changes nothing while paused.
	after, afterCmd := paused.Update(
		tui.TimerMsg{Token: 1, Issue: first.Issue + 1, Fired: time.Now().Add(tickPeriod)},
	)
	if afterCmd != nil {
		t.Error("a timer completion while paused requested another timer")
	}
	if after.View().Content != paused.View().Content {
		t.Error("a timer completion while paused changed the frozen view")
	}
}

func TestPausedShellMovesTheMenuAndDropsPaddleKeys(t *testing.T) {
	t.Parallel()

	model, _ := startMatchInShell(t)
	paused, _ := model.Update(tea.KeyPressMsg{Code: 'p', Text: "P", Mod: tea.ModShift})

	for _, key := range []tea.KeyPressMsg{
		{Code: 'w', Text: "w"},
		{Code: 's', Text: "s"},
		{Code: tea.KeyEscape},
	} {
		updated, cmd := paused.Update(key)
		if cmd != nil {
			t.Errorf("key %q while paused produced a command, want none", key.String())
		}
		if updated.View().Content != paused.View().Content {
			t.Errorf("key %q while paused changed the frozen view", key.String())
		}
	}

	selected, cmd := paused.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Error("menu navigation while paused produced a command")
	}
	if selected.View().Content == paused.View().Content {
		t.Error("Down while paused did not move the pause menu selection")
	}
	if !strings.Contains(selected.View().Content, "> Restart match") {
		t.Error("Down while paused did not select Restart match")
	}

	resumed, cmd := selected.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if cmd == nil {
		t.Error("P resume requested no timer, want the new countdown's timer")
	}
	if strings.Contains(resumed.View().Content, "PAUSED") {
		t.Errorf("P resume left the pause overlay visible")
	}
}

func TestResumeRearmsTheTimerChainAndDropsThePrePauseTimer(t *testing.T) {
	t.Parallel()

	model, _ := startMatchInShell(t)
	paused, _ := model.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	resumed, cmd := paused.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if cmd == nil {
		t.Fatal("resume did not request the countdown's timer")
	}

	stale, staleCmd := resumed.Update(tui.TimerMsg{Token: 1, Fired: time.Now().Add(tickPeriod)})
	if staleCmd != nil {
		t.Error("the pre-pause timer requested another timer after resume")
	}
	if stale.View().Content != resumed.View().Content {
		t.Error("the pre-pause timer changed the resumed view")
	}

	fresh, freshCmd, message := probeTimer(t, resumed, 2, newTimerTape().next(tickPeriod))
	if freshCmd == nil {
		t.Error("the resume countdown timer requested no next timer")
	}
	if duplicate, duplicateCmd := fresh.Update(message); duplicateCmd != nil ||
		duplicate.View().Content != fresh.View().Content {
		t.Error("a duplicate resume timer changed the match")
	}
}

func TestRecoveryWhilePausedDoesNotRearmAStoppedChain(t *testing.T) {
	t.Parallel()

	model, _ := startMatchInShell(t)
	model, _, first := probeTimer(t, model, 1, newTimerTape().next(tickPeriod))
	paused, _ := model.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	answered, cmd := paused.Update(
		tui.TimerMsg{Token: 1, Issue: first.Issue + 1, Fired: time.Now().Add(tickPeriod)},
	)
	if cmd != nil {
		t.Fatal("the paused core answered a completion with a timer, want the chain stopped")
	}

	shrunk, _ := answered.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	recovered, cmd := shrunk.Update(tea.WindowSizeMsg{Width: 80, Height: 28})
	if cmd != nil {
		t.Error("recovery while paused re-armed a stopped timing chain")
	}
	if recovered.View().Content != answered.View().Content {
		t.Error("recovery while paused changed the frozen view")
	}
}

func TestPauseKeyDoesNothingOutsideAMatch(t *testing.T) {
	t.Parallel()

	setupModel := sized(tui.NewModel())
	setupModel, _ = setupModel.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	for _, model := range []tea.Model{setupModel} {
		updated, cmd := model.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
		if cmd != nil || updated.View().Content != model.View().Content {
			t.Error("the pause key changed a non-match screen")
		}
	}

	matchModel, _ := startMatchInShell(t)
	undersized, _ := matchModel.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	updated, cmd := undersized.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if cmd != nil || updated.View().Content != undersized.View().Content {
		t.Error("the pause key changed an undersized match")
	}
}
