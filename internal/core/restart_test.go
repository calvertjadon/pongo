package core_test

import (
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

func TestRestartRestoresTheOriginalStartConditions(t *testing.T) {
	t.Parallel()

	setupModel := setupFor(t, core.ModeLocalTwoPlayer, core.WinningScore7, core.DifficultyNormal)
	draft := setupModel.Draft()
	started, schedule := core.Update(setupModel, core.EventPrimary{})
	rally, rallyToken := beginServingRally(t, started, schedule.Timer().Token)

	// Give the match a point, a moved paddle, scorer feedback, and an advanced serve sequence.
	rally = setPaddleTop(t, rally, true, 8)
	scored, pointEffect := runUntilPoint(t, rally, rallyToken, [2]uint8{1, 0})
	countdownToken := pointEffect.Timer().Token
	scored = setPaddleTop(t, scored, false, 12)
	for range 20 {
		scored, _ = core.Update(scored, core.EventElapsed{Elapsed: core.StepDuration, Token: countdownToken})
	}
	if serve := scored.NextServe(); serve.Vertical != core.DirectionDown {
		t.Fatalf("serve before restart = %+v, want the advanced downward sequence", serve)
	}

	paused, _ := core.Update(scored, core.EventPauseToggle{})
	restarted, effect := core.Update(focusPauseRow(t, paused, core.PauseRowRestart), core.EventPrimary{})
	assertRestarted(t, restarted, draft)

	if effect.Kind() != core.EffectScheduleTimer {
		t.Fatalf("restart requested %v, want the countdown's timer", effect.Kind())
	}
	token := effect.Timer().Token
	if token == rallyToken || token == countdownToken {
		t.Errorf("restart token = %d, want a new timing generation", token)
	}
	assertScheduled(t, effect, token)

	served, _ := beginServingRally(t, restarted, token)
	if vx, vy := served.BallVelocity(); vx <= 0 || vy >= 0 {
		t.Errorf("first serve after restart = (%v, %v), want toward the right player and up", vx, vy)
	}
}

// assertRestarted checks the match restarted at its original start conditions with its locked settings kept.
func assertRestarted(t *testing.T, restarted core.Model, draft core.Settings) {
	t.Helper()

	if restarted.Screen() != core.ScreenMatch || restarted.MatchPhase() != core.MatchPhaseCountdown {
		t.Fatalf("restart gave screen %v, phase %v; want a fresh countdown",
			restarted.Screen(), restarted.MatchPhase())
	}
	if locked := restarted.MatchSettings(); locked != draft {
		t.Errorf("restart changed the locked settings from %+v to %+v", draft, locked)
	}
	if left, right := restarted.Score(); left != 0 || right != 0 {
		t.Errorf("score after restart = %d:%d, want 0:0", left, right)
	}
	if left, right := restarted.PaddleTops(); left != 8 || right != 8 {
		t.Errorf("paddle tops after restart = %d, %d; want the centered 8, 8", left, right)
	}
	if x, y := restarted.BallPosition(); x != 34.5 || y != 9.5 {
		t.Errorf("ball after restart = (%v, %v), want the court center", x, y)
	}
	if vx, vy := restarted.BallVelocity(); vx != 0 || vy != 0 {
		t.Errorf("ball velocity after restart = (%v, %v), want it resting", vx, vy)
	}
	if feedback := restarted.Feedback(); feedback != core.FeedbackNone {
		t.Errorf("feedback after restart = %v, want none", feedback)
	}
	if remaining := restarted.CountdownRemaining(); remaining != core.CountdownSteps {
		t.Errorf("countdown after restart = %d steps, want the full %d", remaining, core.CountdownSteps)
	}
	if serve := restarted.NextServe(); serve != (core.Serve{Horizontal: core.DirectionRight, Vertical: core.DirectionUp}) {
		t.Errorf("serve after restart = %+v, want the original right and up", serve)
	}
}

func TestReturnToTitleFromPauseDiscardsTheMatchAndKeepsTheDraft(t *testing.T) {
	t.Parallel()

	setupModel := setupFor(t, core.ModeSinglePlayer, core.WinningScore11, core.DifficultyHard)
	draft := setupModel.Draft()
	started, schedule := core.Update(setupModel, core.EventPrimary{})
	rally, rallyToken := beginServingRally(t, started, schedule.Timer().Token)
	rally = advanceRally(t, rally, rallyToken, 10)
	paused, _ := core.Update(rally, core.EventPauseToggle{})

	title, effect := core.Update(focusPauseRow(t, paused, core.PauseRowReturnToTitle), core.EventPrimary{})
	if title.Screen() != core.ScreenTitle {
		t.Fatalf("Return to title gave screen %v, want the title", title.Screen())
	}
	if kept := title.Draft(); kept != draft {
		t.Errorf("Return to title changed the draft from %+v to %+v", draft, kept)
	}
	if effect.Kind() != core.EffectNone {
		t.Errorf("Return to title requested %v, want no effect", effect.Kind())
	}

	reentered, _ := core.Update(title, core.EventPrimary{})
	if reentered.Screen() != core.ScreenSetup || reentered.SetupFocus() != core.SetupRowMode {
		t.Fatalf("re-entry gave screen %v, focus %v; want setup focused on mode",
			reentered.Screen(), reentered.SetupFocus())
	}
	if kept := reentered.Draft(); kept != draft {
		t.Errorf("re-entry changed the draft from %+v to %+v", draft, kept)
	}
}

// focusPauseRow moves the pause menu focus down until row is selected.
func focusPauseRow(t *testing.T, model core.Model, row core.PauseRow) core.Model {
	t.Helper()

	for range 3 {
		if model.PauseFocus() == row {
			return model
		}
		model, _ = core.Update(model, core.EventNavigate{Direction: core.DirectionDown})
	}
	t.Fatalf("pause menu never focused %v", row)

	return model
}
