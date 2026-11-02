package core_test

import (
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

func TestPauseFreezesACountdownBehindTheMenu(t *testing.T) {
	t.Parallel()

	model, _ := openingCountdown(t)
	paused, effect := core.Update(model, core.EventPauseToggle{})

	if paused.Screen() != core.ScreenMatch || paused.MatchPhase() != core.MatchPhasePaused {
		t.Fatalf("pause gave screen %v, phase %v; want a paused match",
			paused.Screen(), paused.MatchPhase())
	}
	if focus := paused.PauseFocus(); focus != core.PauseRowResume {
		t.Errorf("pause focus = %v, want Resume", focus)
	}
	if effect.Kind() != core.EffectNone {
		t.Errorf("pause requested %v, want no effect", effect.Kind())
	}
	assertFrozenState(t, model, paused)
}

func TestPauseFreezesARallyBehindTheMenu(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	model = setPaddleTop(t, model, true, 2)
	moving := advanceRally(t, model, token, 25)
	paused, effect := core.Update(moving, core.EventPauseToggle{})

	if paused.MatchPhase() != core.MatchPhasePaused {
		t.Fatalf("pause during a rally gave phase %v, want paused", paused.MatchPhase())
	}
	if effect.Kind() != core.EffectNone {
		t.Errorf("pause requested %v, want no effect", effect.Kind())
	}
	assertFrozenState(t, moving, paused)
}

func TestPauseMenuWrapsThroughEveryRow(t *testing.T) {
	t.Parallel()

	paused := pausedMatch(t)
	down := core.EventNavigate{Direction: core.DirectionDown}
	up := core.EventNavigate{Direction: core.DirectionUp}
	want := []core.PauseRow{
		core.PauseRowRestart,
		core.PauseRowReturnToTitle,
		core.PauseRowResume,
		core.PauseRowReturnToTitle,
	}

	model := paused
	for index, row := range want {
		step := down
		if index == 3 {
			step = up
		}
		var effect core.Effect
		model, effect = core.Update(model, step)
		if focus := model.PauseFocus(); focus != row {
			t.Fatalf("after %d × %+v focus = %v, want %v", index+1, step, focus, row)
		}
		if effect.Kind() != core.EffectNone {
			t.Fatalf("menu navigation produced effect %v, want none", effect.Kind())
		}
	}
}

func TestPausedIgnoresEveryEventExceptTheMenu(t *testing.T) {
	t.Parallel()

	paused := pausedMatch(t)
	assertNoOps(t, paused, []core.Event{
		core.EventAdjust{Direction: core.DirectionLeft},
		core.EventAdjust{Direction: core.DirectionRight},
		core.EventBack{},
		core.EventQuit{},
		wsStep(core.DirectionUp),
		wsStep(core.DirectionDown),
		arrowStep(core.DirectionUp),
		arrowStep(core.DirectionDown),
		core.EventElapsed{Elapsed: core.ElapsedLimit, Token: 1},
		core.EventElapsed{Elapsed: core.ElapsedLimit, Token: 2},
	})
}

// assertFrozenState checks the pause kept everything the frozen court shows.
func assertFrozenState(t *testing.T, before, after core.Model) {
	t.Helper()

	beforeLeft, beforeRight := before.Score()
	afterLeft, afterRight := after.Score()
	if beforeLeft != afterLeft || beforeRight != afterRight {
		t.Errorf("pause changed the score from %d:%d to %d:%d", beforeLeft, beforeRight, afterLeft, afterRight)
	}
	beforeLeftTop, beforeRightTop := before.PaddleTops()
	afterLeftTop, afterRightTop := after.PaddleTops()
	if beforeLeftTop != afterLeftTop || beforeRightTop != afterRightTop {
		t.Errorf("pause changed the paddle tops from %d, %d to %d, %d",
			beforeLeftTop, beforeRightTop, afterLeftTop, afterRightTop)
	}
	beforeX, beforeY := before.BallPosition()
	afterX, afterY := after.BallPosition()
	if beforeX != afterX || beforeY != afterY {
		t.Errorf("pause moved the ball from (%v, %v) to (%v, %v)", beforeX, beforeY, afterX, afterY)
	}
	if before.Feedback() != after.Feedback() {
		t.Errorf("pause changed the feedback from %v to %v", before.Feedback(), after.Feedback())
	}
	if before.MatchSettings() != after.MatchSettings() {
		t.Errorf("pause changed the locked settings from %+v to %+v",
			before.MatchSettings(), after.MatchSettings())
	}
}

// pausedMatch starts a local two-player match, spends a few rally steps, and pauses it.
func pausedMatch(t *testing.T) core.Model {
	t.Helper()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	model = advanceRally(t, model, token, 10)
	paused, effect := core.Update(model, core.EventPauseToggle{})
	if paused.MatchPhase() != core.MatchPhasePaused || effect.Kind() != core.EffectNone {
		t.Fatalf("pause gave phase %v and effect %v, want paused and none", paused.MatchPhase(), effect.Kind())
	}

	return paused
}

// TestBallVisibilityFollowsTheMatchPhase pins when the court draws the ball: never outside a match, through a rally,
// while paused, and through the resume countdown that continues a frozen rally, but not during a serve countdown or
// after the match has finished.
func TestBallVisibilityFollowsTheMatchPhase(t *testing.T) {
	t.Parallel()

	if core.NewModel().BallVisible() {
		t.Error("the ball is visible on the title, want it hidden outside a match")
	}
	setup, _ := core.Update(core.NewModel(), core.EventPrimary{})
	if setup.BallVisible() {
		t.Error("the ball is visible in Match setup, want it hidden outside a match")
	}

	countdown, _ := openingCountdown(t)
	if countdown.BallVisible() {
		t.Error("the ball is visible during a serve countdown, want it hidden until the serve starts")
	}

	rally, token := beginRally(t, core.ModeLocalTwoPlayer)
	if !rally.BallVisible() {
		t.Error("the ball is hidden during a rally, want it visible")
	}
	paused, _ := core.Update(rally, core.EventPauseToggle{})
	if !paused.BallVisible() {
		t.Error("the ball is hidden while paused, want the frozen court to keep it")
	}
	resumed, effect := core.Update(paused, core.EventPauseToggle{})
	if resumed.MatchPhase() != core.MatchPhaseCountdown {
		t.Fatalf("resume gave phase %v, want a countdown", resumed.MatchPhase())
	}
	if !resumed.BallVisible() {
		t.Error("the ball is hidden during a resume countdown, want the frozen rally's ball to stay visible")
	}
	if effect.Timer().Token == token {
		t.Error("the resume countdown reused the frozen rally's timing sequence")
	}

	if finishedMatch(t).BallVisible() {
		t.Error("the ball is visible after the match finished, want it hidden on Game over")
	}
}
