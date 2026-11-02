package core_test

import (
	"slices"
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

// lastToken is a token past every sequence a short match allocates, used to prove stale completions are no-ops.
const lastToken core.TimerToken = 25

// timedPhase is a match in a phase that holds a live timing sequence, with that sequence's token.
type timedPhase struct {
	model core.Model
	token core.TimerToken
}

func TestTimingSequenceRulesHoldInEveryTimedPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		arrange func(*testing.T) timedPhase
	}{
		{name: "opening countdown", arrange: openingCountdownSequence},
		{name: "rally", arrange: rallySequence},
		{name: "resume countdown", arrange: resumeCountdownSequence},
		{name: "serve countdown after a point", arrange: pointCountdownSequence},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			phase := test.arrange(t)
			assertCarriesPartialSteps(t, phase)
			assertCapsCatchUp(t, phase)
			assertNoOps(t, phase.model, staleCompletions(phase.token))
		})
	}
}

func TestPhasesWithoutATimingSequenceIgnoreEveryToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		arrange func(*testing.T) core.Model
	}{
		{name: "paused", arrange: pausedMatch},
		{name: "game over", arrange: finishedMatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assertNoOps(t, test.arrange(t), staleCompletions(0))
		})
	}
}

func TestCountdownLeftoverTimeNeverReachesTheRally(t *testing.T) {
	t.Parallel()

	opening := openingCountdownSequence(t)
	lastStep := spentSteps(t, opening, int(core.CountdownSteps)-1)

	exact, exactEffect := core.Update(lastStep, core.EventElapsed{Elapsed: core.StepDuration, Token: opening.token})
	if exact.MatchPhase() != core.MatchPhaseRally {
		t.Fatalf("final countdown step reached phase %v, want a rally", exact.MatchPhase())
	}
	leftover, leftoverEffect := core.Update(
		lastStep,
		core.EventElapsed{Elapsed: core.StepDuration + core.StepDuration/2, Token: opening.token},
	)
	if leftover != exact || leftoverEffect != exactEffect {
		t.Errorf("a final step with half a step left over gave %+v and %+v, want the exact launch %+v and %+v",
			leftover, leftoverEffect, exact, exactEffect)
	}
}

// TestNegativeElapsedCompletionAddsNothing checks that a completion reporting time that ran backwards neither spends a
// step nor corrupts the sequence: later completions land exactly as they would if it had never arrived.
func TestNegativeElapsedCompletionAddsNothing(t *testing.T) {
	t.Parallel()

	phase := rallySequence(t)

	// The magnitude would reverse ninety accumulated steps, far more than any carry a sequence holds.
	reversed, effect := core.Update(
		phase.model,
		core.EventElapsed{Elapsed: -10 * core.ElapsedLimit, Token: phase.token},
	)
	if reversed != phase.model {
		t.Fatalf("a completion of negative elapsed changed the rally from %+v to %+v", phase.model, reversed)
	}
	assertScheduled(t, effect, phase.token)

	later := []core.Event{
		core.EventElapsed{Elapsed: core.ElapsedLimit, Token: phase.token},
		core.EventElapsed{Elapsed: core.StepDuration / 2, Token: phase.token},
		core.EventElapsed{Elapsed: core.StepDuration / 2, Token: phase.token},
		core.EventElapsed{Elapsed: core.StepDuration, Token: phase.token},
	}
	afterNegative, negativeEffects := replayFrom(reversed, later)
	withoutNegative, plainEffects := replayFrom(phase.model, later)
	if afterNegative != withoutNegative {
		t.Fatalf("later completions after a negative one gave %+v, want the replay without it %+v",
			afterNegative, withoutNegative)
	}
	if !slices.Equal(negativeEffects, plainEffects) {
		t.Fatalf("later completions after a negative one requested %+v, want %+v", negativeEffects, plainEffects)
	}
}

// assertCarriesPartialSteps checks that two half-step completions advance the phase exactly as one whole step does and
// that each re-arms the same sequence.
func assertCarriesPartialSteps(t *testing.T, phase timedPhase) {
	t.Helper()

	half := core.EventElapsed{Elapsed: core.StepDuration / 2, Token: phase.token}
	model, first := core.Update(phase.model, half)
	assertScheduled(t, first, phase.token)
	model, second := core.Update(model, half)
	assertScheduled(t, second, phase.token)

	if want := spentSteps(t, phase, 1); model != want {
		t.Errorf("two half steps gave %+v, want one whole step %+v", model, want)
	}
}

// assertCapsCatchUp checks that a late completion advances the phase exactly as the nine steps of ElapsedLimit do,
// discarding the excess rather than carrying it, and re-arms the same sequence.
func assertCapsCatchUp(t *testing.T, phase timedPhase) {
	t.Helper()

	capped, effect := core.Update(phase.model, core.EventElapsed{Elapsed: 10 * core.ElapsedLimit, Token: phase.token})
	assertScheduled(t, effect, phase.token)

	if want := spentSteps(t, phase, int(core.ElapsedLimit/core.StepDuration)); capped != want {
		t.Errorf("a completion of ten catch-up limits gave %+v, want nine steps %+v", capped, want)
	}
}

// staleCompletions returns a catch-up-limit completion for every token up to lastToken except live.
func staleCompletions(live core.TimerToken) []core.Event {
	events := make([]core.Event, 0, lastToken+1)
	for candidate := core.TimerToken(0); candidate <= lastToken; candidate++ {
		if candidate != live {
			events = append(events, core.EventElapsed{Elapsed: core.ElapsedLimit, Token: candidate})
		}
	}

	return events
}

// spentSteps spends steps one-step completions of the phase's sequence and checks that each re-arms it.
func spentSteps(t *testing.T, phase timedPhase, steps int) core.Model {
	t.Helper()

	model := phase.model
	for range steps {
		var effect core.Effect
		model, effect = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: phase.token})
		assertScheduled(t, effect, phase.token)
	}

	return model
}

// requestedToken checks that effect requests a timer and returns its token.
func requestedToken(t *testing.T, effect core.Effect) core.TimerToken {
	t.Helper()

	if effect.Kind() != core.EffectScheduleTimer {
		t.Fatalf("effect = %v, want a timer request", effect.Kind())
	}

	return effect.Timer().Token
}

func openingCountdownSequence(t *testing.T) timedPhase {
	t.Helper()

	model, schedule := openingCountdown(t)

	return timedPhase{model: model, token: requestedToken(t, schedule)}
}

func rallySequence(t *testing.T) timedPhase {
	t.Helper()

	opening := openingCountdownSequence(t)
	model, token := beginServingRally(t, opening.model, opening.token)

	return timedPhase{model: model, token: token}
}

func resumeCountdownSequence(t *testing.T) timedPhase {
	t.Helper()

	paused, _ := core.Update(spentSteps(t, rallySequence(t), 10), core.EventPauseToggle{})
	resumed, effect := core.Update(paused, core.EventPauseToggle{})
	if resumed.MatchPhase() != core.MatchPhaseCountdown {
		t.Fatalf("resume reached phase %v, want a countdown", resumed.MatchPhase())
	}

	return timedPhase{model: resumed, token: requestedToken(t, effect)}
}

func pointCountdownSequence(t *testing.T) timedPhase {
	t.Helper()

	rally := rallySequence(t)
	scored, effect := runUntilPoint(t, rally.model, rally.token, [2]uint8{1, 0})
	if scored.MatchPhase() != core.MatchPhaseCountdown {
		t.Fatalf("the point reached phase %v, want a countdown", scored.MatchPhase())
	}

	return timedPhase{model: scored, token: requestedToken(t, effect)}
}

func finishedMatch(t *testing.T) core.Model {
	t.Helper()

	return playedMatch(t, core.Settings{
		Mode: core.ModeLocalTwoPlayer, WinningScore: core.WinningScore5, Difficulty: core.DifficultyNormal,
	}, core.SideLeft)
}
