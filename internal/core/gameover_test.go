package core_test

import (
	"slices"
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

// paddleStartTop is the top row that centers a paddle, which is where a new match puts both paddles. A serve leaves
// the court center at the opening speed and angle and passes above or below a centered paddle.
const paddleStartTop uint8 = 8

// lastToken is a token past every sequence a short match allocates, used to prove stale completions are no-ops.
const lastToken core.TimerToken = 25

func TestWinningPointEndsTheMatchAtTheLockedWinningScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		winner core.Side
		draft  core.Settings
		wantl  uint8
		wantr  uint8
	}{
		{
			name:   "left player wins a short match",
			winner: core.SideLeft,
			draft: core.Settings{
				Mode:         core.ModeLocalTwoPlayer,
				WinningScore: core.WinningScore5,
				Difficulty:   core.DifficultyNormal,
			},
			wantl: 5,
		},
		{
			name:   "right player wins a short match",
			winner: core.SideRight,
			draft: core.Settings{
				Mode:         core.ModeLocalTwoPlayer,
				WinningScore: core.WinningScore5,
				Difficulty:   core.DifficultyNormal,
			},
			wantr: 5,
		},
		{
			name:   "left player wins a standard match",
			winner: core.SideLeft,
			draft: core.Settings{
				Mode:         core.ModeLocalTwoPlayer,
				WinningScore: core.WinningScore7,
				Difficulty:   core.DifficultyNormal,
			},
			wantl: 7,
		},
		{
			name:   "right player wins a standard match",
			winner: core.SideRight,
			draft: core.Settings{
				Mode:         core.ModeLocalTwoPlayer,
				WinningScore: core.WinningScore7,
				Difficulty:   core.DifficultyNormal,
			},
			wantr: 7,
		},
		{
			name:   "left player wins a long match",
			winner: core.SideLeft,
			draft: core.Settings{
				Mode:         core.ModeLocalTwoPlayer,
				WinningScore: core.WinningScore11,
				Difficulty:   core.DifficultyNormal,
			},
			wantl: 11,
		},
		{
			name:   "right player wins a long match",
			winner: core.SideRight,
			draft: core.Settings{
				Mode:         core.ModeLocalTwoPlayer,
				WinningScore: core.WinningScore11,
				Difficulty:   core.DifficultyNormal,
			},
			wantr: 11,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assertFinishedMatch(t, playedMatch(t, test.draft, test.winner), test.winner, test.wantl, test.wantr)
		})
	}
}

// assertFinishedMatch checks everything the winning point leaves behind.
func assertFinishedMatch(t *testing.T, finished core.Model, winner core.Side, wantLeft, wantRight uint8) {
	t.Helper()

	if got := finished.MatchPhase(); got != core.MatchPhaseGameOver {
		t.Fatalf("phase after the winning point = %v, want Game over", got)
	}
	if got := finished.Winner(); got != winner {
		t.Errorf("winner = %v, want %v", got, winner)
	}
	left, right := finished.Score()
	if left != wantLeft || right != wantRight {
		t.Errorf("final score = %d:%d, want %d:%d", left, right, wantLeft, wantRight)
	}
	if remaining := finished.CountdownRemaining(); remaining != 0 {
		t.Errorf("countdown after the winning point = %d steps, want none", remaining)
	}
	if x, y := finished.BallPosition(); x != 34.5 || y != 9.5 {
		t.Errorf("ball after the winning point = (%v, %v), want the resting court center", x, y)
	}
	if vx, vy := finished.BallVelocity(); vx != 0 || vy != 0 {
		t.Errorf("ball velocity after the winning point = (%v, %v), want it resting", vx, vy)
	}
}

func TestGameOverIgnoresEveryStaleTimingSequence(t *testing.T) {
	t.Parallel()

	finished := playedMatch(t, core.Settings{
		Mode: core.ModeLocalTwoPlayer, WinningScore: core.WinningScore5, Difficulty: core.DifficultyNormal,
	}, core.SideLeft)

	events := make([]core.Event, 0, lastToken)
	for candidate := core.TimerToken(0); candidate <= lastToken; candidate++ {
		events = append(events, core.EventElapsed{Elapsed: core.ElapsedLimit, Token: candidate})
	}
	assertNoOps(t, finished, events)
}

func TestWinningPointWinsWithoutATwoPointMargin(t *testing.T) {
	t.Parallel()

	draft := core.Settings{
		Mode:         core.ModeLocalTwoPlayer,
		WinningScore: core.WinningScore5,
		Difficulty:   core.DifficultyNormal,
	}
	model, schedule := startMatchWith(t, draft)
	token := schedule.Timer().Token

	// Alternate scorers so both sides reach four points; a two-point margin would keep the match alive.
	for point := range 8 {
		scorer := core.SideLeft
		if point%2 == 1 {
			scorer = core.SideRight
		}
		model, token = playPoint(t, model, token, scorer)
	}

	if left, right := model.Score(); left != 4 || right != 4 {
		t.Fatalf("score after eight points = %d:%d, want 4:4", left, right)
	}
	if model.MatchPhase() != core.MatchPhaseCountdown {
		t.Fatalf("phase at 4:4 = %v, want the next serve's countdown", model.MatchPhase())
	}

	finished, _ := playPoint(t, model, token, core.SideLeft)
	if left, right := finished.Score(); left != 5 || right != 4 {
		t.Fatalf("final score = %d:%d, want 5:4 with no two-point margin", left, right)
	}
	if finished.MatchPhase() != core.MatchPhaseGameOver || finished.Winner() != core.SideLeft {
		t.Fatalf("phase %v and winner %v after 5:4, want Game over and the left player",
			finished.MatchPhase(), finished.Winner())
	}
}

func TestGameOverMenuWrapsAndEveryOtherEventIsANoOp(t *testing.T) {
	t.Parallel()

	finished := playedMatch(t, core.Settings{
		Mode: core.ModeLocalTwoPlayer, WinningScore: core.WinningScore5, Difficulty: core.DifficultyNormal,
	}, core.SideLeft)
	if finished.GameOverFocus() != core.GameOverRowRematch {
		t.Fatalf("game-over focus = %v, want Rematch", finished.GameOverFocus())
	}

	up := core.EventNavigate{Direction: core.DirectionUp}
	down := core.EventNavigate{Direction: core.DirectionDown}
	for index, row := range []core.GameOverRow{
		core.GameOverRowReturnToTitle,
		core.GameOverRowChangeSetup,
		core.GameOverRowRematch,
	} {
		var effect core.Effect
		finished, effect = core.Update(finished, up)
		if focus := finished.GameOverFocus(); focus != row {
			t.Fatalf("after %d × up focus = %v, want %v", index+1, focus, row)
		}
		if effect.Kind() != core.EffectNone {
			t.Fatalf("navigation produced effect %v, want none", effect.Kind())
		}
		if finished.MatchPhase() != core.MatchPhaseGameOver || finished.Screen() != core.ScreenMatch {
			t.Fatalf("navigation left phase %v on screen %v", finished.MatchPhase(), finished.Screen())
		}
	}
	for index, row := range []core.GameOverRow{
		core.GameOverRowChangeSetup,
		core.GameOverRowReturnToTitle,
		core.GameOverRowRematch,
	} {
		var effect core.Effect
		finished, effect = core.Update(finished, down)
		if focus := finished.GameOverFocus(); focus != row {
			t.Fatalf("after %d × down focus = %v, want %v", index+1, focus, row)
		}
		if effect.Kind() != core.EffectNone {
			t.Fatalf("navigation produced effect %v, want none", effect.Kind())
		}
	}

	assertNoOps(t, finished, []core.Event{
		core.EventPauseToggle{},
		wsStep(core.DirectionUp),
		wsStep(core.DirectionDown),
		arrowStep(core.DirectionUp),
		arrowStep(core.DirectionDown),
		core.EventBack{},
		core.EventQuit{},
		core.EventAdjust{Direction: core.DirectionLeft},
		core.EventAdjust{Direction: core.DirectionRight},
	})
}

func TestRematchKeepsSettingsAndContinuesTheChain(t *testing.T) {
	t.Parallel()

	draft := core.Settings{
		Mode:         core.ModeLocalTwoPlayer,
		WinningScore: core.WinningScore5,
		Difficulty:   core.DifficultyNormal,
	}
	finished := playedMatch(t, draft, core.SideLeft)

	rematch, effect := core.Update(finished, core.EventPrimary{})
	if rematch.Screen() != core.ScreenMatch || rematch.MatchPhase() != core.MatchPhaseCountdown {
		t.Fatalf("rematch gave screen %v and phase %v, want a match countdown",
			rematch.Screen(), rematch.MatchPhase())
	}
	if effect.Kind() != core.EffectScheduleTimer {
		t.Fatalf("rematch requested %v, want a timer", effect.Kind())
	}
	if effect.Timer().Token == 0 || rematch.ChainPosition() != 1 {
		t.Errorf("rematch token %d and chain position %d, want a new timer and position 1",
			effect.Timer().Token, rematch.ChainPosition())
	}
	if got := rematch.MatchSettings(); got != draft {
		t.Errorf("rematch settings = %+v, want the locked %+v", got, draft)
	}
	if left, right := rematch.Score(); left != 0 || right != 0 {
		t.Errorf("rematch score = %d:%d, want 0:0", left, right)
	}
	if leftTop, rightTop := rematch.PaddleTops(); leftTop != paddleStartTop || rightTop != paddleStartTop {
		t.Errorf("rematch paddle tops = %d, %d; want both centered", leftTop, rightTop)
	}
	if remaining := rematch.CountdownRemaining(); remaining != core.CountdownSteps {
		t.Errorf("rematch countdown = %d steps, want a full one", remaining)
	}
	if got := rematch.Feedback(); got != core.FeedbackNone {
		t.Errorf("rematch feedback = %v, want none", got)
	}

	// Five points flip the vertical serve five times, so the finished match leaves the chain pointing down, and the
	// rematch's opening direction flips away from the finished match's.
	wantServe := core.Serve{Horizontal: core.DirectionLeft, Vertical: core.DirectionDown}
	if got := rematch.NextServe(); got != wantServe {
		t.Errorf("rematch opening serve = %+v, want %+v", got, wantServe)
	}
	rally, _ := beginServingRally(t, rematch, effect.Timer().Token)
	if vx, vy := rally.BallVelocity(); vx >= 0 || vy <= 0 {
		t.Errorf("rematch serve velocity = (%v, %v), want left and down", vx, vy)
	}

	second := playedMatchFrom(t, rematch, effect.Timer().Token, core.SideLeft)
	if focus := second.GameOverFocus(); focus != core.GameOverRowRematch {
		t.Errorf("focus after the second match = %v, want Rematch selected again", focus)
	}
	again, effect := core.Update(second, core.EventPrimary{})
	if again.ChainPosition() != 2 {
		t.Errorf("chain position after the second rematch = %d, want 2", again.ChainPosition())
	}
	if got := again.NextServe(); got.Horizontal != core.DirectionRight {
		t.Errorf("third match opening = %+v, want it flipped back to the right", got)
	}
	if effect.Kind() != core.EffectScheduleTimer {
		t.Errorf("second rematch requested %v, want a timer", effect.Kind())
	}
}

func TestChangeSetupSeedsTheDraftAndEndsTheChain(t *testing.T) {
	t.Parallel()

	draft := core.Settings{
		Mode:         core.ModeSinglePlayer,
		WinningScore: core.WinningScore11,
		Difficulty:   core.DifficultyHard,
	}
	finished := playedMatch(t, draft, core.SideLeft)

	chosen := finished
	for chosen.GameOverFocus() != core.GameOverRowChangeSetup {
		chosen, _ = core.Update(chosen, core.EventNavigate{Direction: core.DirectionDown})
	}

	setup, effect := core.Update(chosen, core.EventPrimary{})
	if setup.Screen() != core.ScreenSetup || setup.SetupFocus() != core.SetupRowMode {
		t.Fatalf("Change setup gave screen %v and focus %v, want Match setup focused on Mode",
			setup.Screen(), setup.SetupFocus())
	}
	if got := setup.Draft(); got != draft {
		t.Errorf("draft after Change setup = %+v, want the finished match's %+v", got, draft)
	}
	if effect.Kind() != core.EffectNone {
		t.Errorf("Change setup requested %v, want no effect", effect.Kind())
	}
	if position := setup.ChainPosition(); position != 0 {
		t.Errorf("chain position after Change setup = %d, want the chain ended", position)
	}

	started, schedule := core.Update(focusRow(t, setup, core.SetupRowStart), core.EventPrimary{})
	if started.Screen() != core.ScreenMatch || schedule.Kind() != core.EffectScheduleTimer {
		t.Fatalf("Start match after Change setup gave screen %v and effect %v, want a match countdown",
			started.Screen(), schedule.Kind())
	}
	if got := started.NextServe(); got != (core.Serve{Horizontal: core.DirectionRight, Vertical: core.DirectionUp}) {
		t.Errorf("new chain opening serve = %+v, want right and up", got)
	}
}

func TestReturnToTitleKeepsTheDraftAndANewMatchStartsAFreshChain(t *testing.T) {
	t.Parallel()

	draft := core.Settings{
		Mode:         core.ModeLocalTwoPlayer,
		WinningScore: core.WinningScore5,
		Difficulty:   core.DifficultyNormal,
	}
	model, schedule := startMatchWith(t, draft)
	model = playedMatchFrom(t, model, schedule.Timer().Token, core.SideLeft)

	// A rematch first, so the chain this test ends is a continuing one rather than the opening chain.
	rematch, effect := core.Update(model, core.EventPrimary{})
	if rematch.ChainPosition() != 1 {
		t.Fatalf("chain position after the rematch = %d, want 1", rematch.ChainPosition())
	}
	second := playedMatchFrom(t, rematch, effect.Timer().Token, core.SideLeft)

	chosen := second
	for chosen.GameOverFocus() != core.GameOverRowReturnToTitle {
		chosen, _ = core.Update(chosen, core.EventNavigate{Direction: core.DirectionDown})
	}
	title, effect := core.Update(chosen, core.EventPrimary{})
	if title.Screen() != core.ScreenTitle {
		t.Fatalf("Return to title gave screen %v, want the title", title.Screen())
	}
	if got := title.Draft(); got != draft {
		t.Errorf("draft after Return to title = %+v, want %+v", got, draft)
	}
	if effect.Kind() != core.EffectNone {
		t.Errorf("Return to title requested %v, want no effect", effect.Kind())
	}
	if position := title.ChainPosition(); position != 0 {
		t.Errorf("chain position after Return to title = %d, want the chain ended", position)
	}

	setup, _ := core.Update(title, core.EventPrimary{})
	if setup.Screen() != core.ScreenSetup || setup.SetupFocus() != core.SetupRowMode {
		t.Fatalf("re-entry gave screen %v and focus %v, want setup on Mode", setup.Screen(), setup.SetupFocus())
	}
	started, schedule := core.Update(focusRow(t, setup, core.SetupRowStart), core.EventPrimary{})
	if started.Screen() != core.ScreenMatch || schedule.Kind() != core.EffectScheduleTimer {
		t.Fatalf("new match gave screen %v and effect %v", started.Screen(), schedule.Kind())
	}
	if got := started.NextServe(); got != (core.Serve{Horizontal: core.DirectionRight, Vertical: core.DirectionUp}) {
		t.Errorf("new chain opening serve = %+v, want up toward the right player", got)
	}
}

func TestGameOverReplayReproducesStateAndEffects(t *testing.T) {
	t.Parallel()

	model, schedule := startMatchWith(t, core.Settings{
		Mode: core.ModeLocalTwoPlayer, WinningScore: core.WinningScore5, Difficulty: core.DifficultyNormal,
	})
	finished := playedMatchFrom(t, model, schedule.Timer().Token, core.SideLeft)
	events := []core.Event{
		core.EventNavigate{Direction: core.DirectionUp},
		core.EventNavigate{Direction: core.DirectionDown},
		wsStep(core.DirectionUp),
		core.EventElapsed{Elapsed: core.ElapsedLimit, Token: schedule.Timer().Token},
		core.EventBack{},
	}

	firstModel, firstEffects := replayFrom(finished, events)
	secondModel, secondEffects := replayFrom(finished, events)
	if firstModel != secondModel {
		t.Fatalf("replay model = %+v, want %+v", secondModel, firstModel)
	}
	if !slices.Equal(firstEffects, secondEffects) {
		t.Fatalf("replay effects = %+v, want %+v", secondEffects, firstEffects)
	}
}

// startMatchWith starts a match from a setup draft holding settings.
func startMatchWith(t *testing.T, settings core.Settings) (core.Model, core.Effect) {
	t.Helper()

	setupModel := setupFor(t, settings.Mode, settings.WinningScore, settings.Difficulty)
	started, schedule := core.Update(setupModel, core.EventPrimary{})
	if started.Screen() != core.ScreenMatch || schedule.Kind() != core.EffectScheduleTimer {
		t.Fatalf("start match gave screen %v and effect %v, want a match countdown",
			started.Screen(), schedule.Kind())
	}
	if got := started.MatchSettings(); got != settings {
		t.Fatalf("locked settings = %+v, want %+v", got, settings)
	}

	return started, schedule
}

// playedMatch starts a match with draft and plays it until winner wins it.
func playedMatch(t *testing.T, draft core.Settings, winner core.Side) core.Model {
	t.Helper()

	started, schedule := startMatchWith(t, draft)

	return playedMatchFrom(t, started, schedule.Timer().Token, winner)
}

// playedMatchFrom plays an open match until winner wins it.
func playedMatchFrom(t *testing.T, model core.Model, token core.TimerToken, winner core.Side) core.Model {
	t.Helper()

	for range 4 * int(core.WinningScore11) {
		if model.MatchPhase() == core.MatchPhaseGameOver {
			return model
		}
		model, token = playPoint(t, model, token, winner)
	}
	t.Fatal("the match never ended")

	return model
}

// playPoint spends the pending countdown and rally so that scorer wins the point, and returns the model with the token
// of the next countdown, or zero when the point ended the match.
func playPoint(
	t *testing.T,
	model core.Model,
	countdownToken core.TimerToken,
	scorer core.Side,
) (core.Model, core.TimerToken) {
	t.Helper()

	rally, rallyToken := beginServingRally(t, model, countdownToken)
	rally = arrangePoint(t, rally, scorer)
	left, right := rally.Score()
	want := [2]uint8{left, right}
	if scorer == core.SideLeft {
		want[0]++
	} else {
		want[1]++
	}
	scored, effect := runUntilPoint(t, rally, rallyToken, want)

	if scored.MatchPhase() == core.MatchPhaseGameOver {
		if effect.Kind() != core.EffectNone {
			t.Fatalf("the winning point requested %v, want no effect", effect.Kind())
		}

		return scored, 0
	}
	if effect.Kind() != core.EffectScheduleTimer {
		t.Fatalf("a point requested %v, want a countdown timer", effect.Kind())
	}

	return scored, effect.Timer().Token
}

// arrangePoint positions both paddles so scorer wins the pending serve. The receiving paddle stays centered, which
// leaves it out of the opening serve's path; when the receiving side must score, it returns the serve from the middle
// of its span, and the other centered paddle misses that flat return.
func arrangePoint(t *testing.T, model core.Model, scorer core.Side) core.Model {
	t.Helper()

	serve := model.NextServe()
	defending := core.SideRight
	if serve.Horizontal == core.DirectionLeft {
		defending = core.SideLeft
	}

	model = setPaddleTop(t, model, false, paddleStartTop)
	model = setPaddleTop(t, model, true, paddleStartTop)
	if defending != scorer {
		return model
	}

	return setPaddleTop(t, model, defending == core.SideRight, hitRow(serve.Vertical))
}

// hitRow is the paddle top whose four rows contain the impact row of a serve: the ball leaves the center at 20
// degrees and reaches row 3.6 when it drifts up or row 15.4 when it drifts down.
func hitRow(vertical core.VerticalDirection) uint8 {
	switch vertical {
	case core.DirectionUp:
		return 2
	case core.DirectionDown:
		return 14
	}

	return paddleStartTop
}
