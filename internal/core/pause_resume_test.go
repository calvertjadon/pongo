package core_test

import (
	"slices"
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

func TestResumeStartsAFullCountdownInANewTimingGeneration(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	model = advanceRally(t, model, token, 10)
	paused, _ := core.Update(model, core.EventPauseToggle{})

	resumed, effect := core.Update(paused, core.EventPauseToggle{})
	if resumed.MatchPhase() != core.MatchPhaseCountdown {
		t.Fatalf("resume gave phase %v, want a countdown", resumed.MatchPhase())
	}
	if remaining := resumed.CountdownRemaining(); remaining != core.CountdownSteps {
		t.Errorf("resume countdown = %d steps, want a full %d", remaining, core.CountdownSteps)
	}
	if effect.Kind() != core.EffectScheduleTimer {
		t.Fatalf("resume requested %v, want the countdown's timer", effect.Kind())
	}
	countdownToken := effect.Timer().Token
	if countdownToken == token {
		t.Errorf("resume countdown token = %d, want a new timing generation", countdownToken)
	}
	assertScheduled(t, effect, countdownToken)

	advanced, next := core.Update(resumed, core.EventElapsed{Elapsed: core.StepDuration, Token: countdownToken})
	if remaining := advanced.CountdownRemaining(); remaining != core.CountdownSteps-1 {
		t.Errorf("countdown after one step = %d, want %d", remaining, core.CountdownSteps-1)
	}
	assertScheduled(t, next, countdownToken)

	// The paused sequence is over: its completions cannot touch the resumed countdown.
	assertNoOps(t, advanced, []core.Event{
		core.EventElapsed{Elapsed: core.ElapsedLimit, Token: token},
		core.EventElapsed{Elapsed: core.StepDuration, Token: countdownToken + 1},
	})
}

func TestResumeDuringACountdownLaunchesThePendingServe(t *testing.T) {
	t.Parallel()

	model, schedule := openingCountdown(t)
	token := schedule.Timer().Token
	for range 20 {
		model, _ = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
	}
	paused, _ := core.Update(model, core.EventPauseToggle{})

	resumed, effect := core.Update(paused, core.EventPauseToggle{})
	if remaining := resumed.CountdownRemaining(); remaining != core.CountdownSteps {
		t.Fatalf("resume countdown = %d steps, want a full %d", remaining, core.CountdownSteps)
	}
	rally, rallyToken := beginServingRally(t, resumed, effect.Timer().Token)
	if rally.MatchPhase() != core.MatchPhaseRally {
		t.Fatalf("resume completion reached phase %v, want a rally", rally.MatchPhase())
	}
	if rallyToken == token {
		t.Errorf("rally token = %d, want a new timing generation", rallyToken)
	}
	vx, vy := stepVelocity(t, rally, rallyToken)
	assertClose(t, "resumed serve vx", vx, serveVX)
	assertClose(t, "resumed serve vy", vy, -serveVY)
	if serve := rally.NextServe(); serve != (core.Serve{Horizontal: core.DirectionRight, Vertical: core.DirectionUp}) {
		t.Errorf("resumed serve = %+v, want the opening serve", serve)
	}
}

func TestResumeContinuesTheFrozenBallExactly(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	model = advanceRally(t, model, token, 25)
	frozenX, frozenY := model.BallPosition()
	control := advanceRally(t, model, token, 1)
	paused, _ := core.Update(model, core.EventPauseToggle{})

	// Paddle moves made during the resume countdown stay in effect.
	resumed, effect := core.Update(paused, core.EventPauseToggle{})
	resumed, _ = core.Update(resumed, wsStep(core.DirectionDown))
	resumed, _ = core.Update(resumed, wsStep(core.DirectionDown))
	rally, rallyToken := beginServingRally(t, resumed, effect.Timer().Token)

	x, y := rally.BallPosition()
	if x != frozenX || y != frozenY {
		t.Errorf("resumed ball = (%v, %v), want the frozen (%v, %v)", x, y, frozenX, frozenY)
	}
	if left, right := rally.PaddleTops(); left != 12 {
		t.Errorf("left paddle after the resume countdown = %d, want the moved 12 (right %d)", left, right)
	}

	stepped := advanceRally(t, rally, rallyToken, 1)
	x, y = stepped.BallPosition()
	controlX, controlY := control.BallPosition()
	if x != controlX || y != controlY {
		t.Errorf("resumed ball after one step = (%v, %v), want the unpaused (%v, %v)",
			x, y, controlX, controlY)
	}
}

func TestPResumesRegardlessOfTheSelectedRow(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	paused, _ := core.Update(model, core.EventPauseToggle{})
	selected, _ := core.Update(paused, core.EventNavigate{Direction: core.DirectionDown})
	if focus := selected.PauseFocus(); focus != core.PauseRowRestart {
		t.Fatalf("focus = %v, want Restart", focus)
	}

	resumed, effect := core.Update(selected, core.EventPauseToggle{})
	if resumed.MatchPhase() != core.MatchPhaseCountdown {
		t.Fatalf("P gave phase %v, want the resume countdown", resumed.MatchPhase())
	}
	if effect.Kind() != core.EffectScheduleTimer || effect.Timer().Token == token {
		t.Errorf("P resume effect = %+v, want a timer for a new generation", effect)
	}
	if remaining := resumed.CountdownRemaining(); remaining != core.CountdownSteps {
		t.Errorf("P resume countdown = %d steps, want %d", remaining, core.CountdownSteps)
	}
}

func TestEachPauseEntrySelectsResume(t *testing.T) {
	t.Parallel()

	paused := pausedMatch(t)
	selected, _ := core.Update(paused, core.EventNavigate{Direction: core.DirectionDown})
	selected, _ = core.Update(selected, core.EventNavigate{Direction: core.DirectionDown})
	if focus := selected.PauseFocus(); focus != core.PauseRowReturnToTitle {
		t.Fatalf("focus = %v, want Return to title", focus)
	}

	resumed, _ := core.Update(selected, core.EventPauseToggle{})
	again, effect := core.Update(resumed, core.EventPauseToggle{})
	if focus := again.PauseFocus(); focus != core.PauseRowResume {
		t.Errorf("second pause focus = %v, want Resume", focus)
	}
	if again.MatchPhase() != core.MatchPhasePaused || effect.Kind() != core.EffectNone {
		t.Errorf("second pause gave phase %v and effect %v, want paused and none",
			again.MatchPhase(), effect.Kind())
	}
}

func TestStalePrePauseTimersCannotAlterTheResumedMatch(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	model = advanceRally(t, model, token, 12)
	paused, _ := core.Update(model, core.EventPauseToggle{})
	assertNoOps(t, paused, []core.Event{
		core.EventElapsed{Elapsed: core.ElapsedLimit, Token: token},
	})

	resumed, effect := core.Update(paused, core.EventPauseToggle{})
	assertNoOps(t, resumed, []core.Event{
		core.EventElapsed{Elapsed: core.ElapsedLimit, Token: token},
	})

	rally, _ := beginServingRally(t, resumed, effect.Timer().Token)
	assertNoOps(t, rally, []core.Event{
		core.EventElapsed{Elapsed: core.ElapsedLimit, Token: token},
		core.EventElapsed{Elapsed: core.StepDuration, Token: token},
	})
}

func TestPauseReplayReproducesStateAndEffects(t *testing.T) {
	t.Parallel()

	start, token := beginRally(t, core.ModeLocalTwoPlayer)
	events := []core.Event{
		wsStep(core.DirectionDown),
		core.EventElapsed{Elapsed: core.ElapsedLimit, Token: token},
		core.EventPauseToggle{},
		core.EventElapsed{Elapsed: core.ElapsedLimit, Token: token},
		core.EventNavigate{Direction: core.DirectionDown},
		core.EventNavigate{Direction: core.DirectionUp},
		core.EventPauseToggle{},
		wsStep(core.DirectionUp),
		core.EventElapsed{Elapsed: core.StepDuration, Token: token},
	}
	firstModel, firstEffects := replayFrom(start, events)
	secondModel, secondEffects := replayFrom(start, events)
	if firstModel != secondModel {
		t.Fatalf("replay model = %+v, want %+v", secondModel, firstModel)
	}
	if !slices.Equal(firstEffects, secondEffects) {
		t.Fatalf("replay effects = %+v, want %+v", secondEffects, firstEffects)
	}
}

func TestPauseMenuEnterResumesTheFrozenMatch(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	model = advanceRally(t, model, token, 10)
	control := advanceRally(t, model, token, 1)
	frozenX, frozenY := model.BallPosition()
	paused, _ := core.Update(model, core.EventPauseToggle{})

	resumed, effect := core.Update(paused, core.EventPrimary{})
	if resumed.MatchPhase() != core.MatchPhaseCountdown {
		t.Fatalf("Enter on Resume gave phase %v, want a countdown", resumed.MatchPhase())
	}
	if remaining := resumed.CountdownRemaining(); remaining != core.CountdownSteps {
		t.Errorf("Enter on Resume gave %d countdown steps, want %d", remaining, core.CountdownSteps)
	}
	if effect.Kind() != core.EffectScheduleTimer {
		t.Fatalf("Enter on Resume requested %v, want the countdown's timer", effect.Kind())
	}

	rally, rallyToken := beginServingRally(t, resumed, effect.Timer().Token)
	x, y := rally.BallPosition()
	if x != frozenX || y != frozenY {
		t.Errorf("resumed ball = (%v, %v), want the frozen (%v, %v)", x, y, frozenX, frozenY)
	}
	stepped := advanceRally(t, rally, rallyToken, 1)
	x, y = stepped.BallPosition()
	controlX, controlY := control.BallPosition()
	if x != controlX || y != controlY {
		t.Errorf("resumed ball after one step = (%v, %v), want the unpaused (%v, %v)",
			x, y, controlX, controlY)
	}
}

func TestPauseDuringAResumeCountdownStillContinuesTheFrozenRally(t *testing.T) {
	t.Parallel()

	model, token := beginRally(t, core.ModeLocalTwoPlayer)
	model = advanceRally(t, model, token, 15)
	control := advanceRally(t, model, token, 1)
	frozenX, frozenY := model.BallPosition()

	paused, _ := core.Update(model, core.EventPauseToggle{})
	first, firstEffect := core.Update(paused, core.EventPauseToggle{})
	again, _ := core.Update(first, core.EventPauseToggle{})
	resumed, effect := core.Update(again, core.EventPauseToggle{})
	if remaining := resumed.CountdownRemaining(); remaining != core.CountdownSteps {
		t.Fatalf("second resume countdown = %d steps, want a full %d", remaining, core.CountdownSteps)
	}
	if effect.Timer().Token == firstEffect.Timer().Token {
		t.Error("second resume reused the first resume's timing generation")
	}

	rally, rallyToken := beginServingRally(t, resumed, effect.Timer().Token)
	x, y := rally.BallPosition()
	if x != frozenX || y != frozenY {
		t.Errorf("ball after the second resume = (%v, %v), want the frozen (%v, %v)", x, y, frozenX, frozenY)
	}
	stepped := advanceRally(t, rally, rallyToken, 1)
	x, y = stepped.BallPosition()
	controlX, controlY := control.BallPosition()
	if x != controlX || y != controlY {
		t.Errorf("ball after one step of the second resume = (%v, %v), want the unpaused (%v, %v)",
			x, y, controlX, controlY)
	}
}
