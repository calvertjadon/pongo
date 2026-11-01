package core

// Duration is a span of time in core time units. One unit is 1/120000 second, a fraction small enough that every rule
// lands on whole units: a fixed simulation step is 1000 units, the 75 ms catch-up limit is 9000 units, and the 0.8 s
// countdown is 96000 units. The shell converts measured host time into these units, so the core never reads a clock.
type Duration int64

// DurationUnitsPerSecond is the number of Duration units in one second.
const DurationUnitsPerSecond Duration = 120000

// StepDuration is the time one fixed simulation step advances the match.
const StepDuration Duration = 1000

// ElapsedLimit is the most time one elapsed event may contribute: 75 ms, or nine fixed steps. A delayed timer never
// makes the match jump further, and the excess is discarded rather than accumulated.
const ElapsedLimit Duration = 9000

// Steps counts fixed simulation steps at 120 per second.
type Steps uint16

// CountdownSteps is the length of a serve or resume countdown: 0.8 second, or 96 fixed steps.
const CountdownSteps Steps = 96

// TimerDelay is the delay the core asks the shell for between timer completions: one fixed step. The shell measures
// real elapsed time, so the delay only sets how finely the shell reports time.
const TimerDelay Duration = StepDuration

// TimerToken identifies one core timing sequence. Tokens are never zero and never repeat within a session, so a
// completion from an abandoned sequence can never match a later one.
type TimerToken uint64

// firstTimerToken is the token of the session's first timing sequence. Zero never identifies a sequence.
const firstTimerToken TimerToken = 1

// Timer asks the shell to report elapsed time after Delay, correlated by Token.
type Timer struct {
	Token TimerToken
	Delay Duration
}

// timingSequence is one live timing sequence: the token that identifies it and the elapsed time it has accepted but
// not yet spent on a whole fixed step. A phase that holds one keeps asking the shell for time; replacing it with a
// phase that holds none ends the sequence, so every later completion carrying its token is stale.
type timingSequence struct {
	token TimerToken
	carry Duration
}

// beginSequence starts a new timing sequence under the session's next unused token and returns it with its first
// timer request. The new sequence carries no time from any earlier one.
func (model *Model) beginSequence() (timingSequence, Effect) {
	sequence := timingSequence{token: model.nextToken}
	model.nextToken++

	return sequence, sequence.request()
}

// admit accepts one completion of this sequence and returns the whole fixed steps it completes. The completion
// contributes at most ElapsedLimit, and time short of a whole step carries to the next completion. A completion of
// any other sequence is rejected and changes nothing.
func (sequence timingSequence) admit(elapsed EventElapsed) (timingSequence, Steps, bool) {
	if elapsed.Token != sequence.token {
		return sequence, 0, false
	}
	sequence.carry += min(max(elapsed.Elapsed, 0), ElapsedLimit)
	steps := Steps(sequence.carry / StepDuration)
	sequence.carry -= Duration(steps) * StepDuration

	return sequence, steps, true
}

// request asks the shell for the next completion of this sequence.
func (sequence timingSequence) request() Effect {
	return Effect{kind: EffectScheduleTimer, timer: Timer{Token: sequence.token, Delay: TimerDelay}}
}
