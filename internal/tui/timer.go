package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/calvertjadon/pongo/internal/core"
)

// TimerMsg is one timer completion. It carries the correlation the shell requested and the host times the timer
// command recorded, so the model measures elapsed time without reading a clock and tests can build completions.
type TimerMsg struct {
	// Token is the core timing sequence the timer belongs to.
	Token core.TimerToken
	// Issue counts the recoveries from an undersized terminal the timing sequence has been through: 0 for a new
	// sequence, then one more per recovery. Only the latest issue is honored.
	Issue uint64
	// Armed is when the host started the timer.
	Armed time.Time
	// Fired is when the timer fired.
	Fired time.Time
}

// timerRequest is one host timer the shell asks for: the timing sequence it belongs to, its issue, and its delay.
type timerRequest struct {
	token core.TimerToken
	issue uint64
	delay core.Duration
}

// sequenceTimer is the shell's timer for the live timing sequence. It decides which completions count, converts them
// into the core's elapsed time, and re-arms the sequence after an undersized terminal recovers. A new sequence or a
// recovery measures a completion from when its timer was armed; a continuing sequence measures from the previous
// completion's fire time, so time the host spent between completions is never lost.
type sequenceTimer struct {
	request timerRequest
	// baseline is the previous honored completion's fire time, or zero when the next completion measures from its
	// own arming.
	baseline time.Time
	// armed reports whether a timer for the latest request is owed a completion. Honoring a completion ends the
	// sequence until the core asks for another timer.
	armed bool
}

// requested records a core timer request and returns the host timer to arm. A request under a new token starts a new
// timing sequence; a request under the current token continues it from the completion the core just answered.
func (timer sequenceTimer) requested(asked core.Timer) (sequenceTimer, timerRequest) {
	if asked.Token != timer.request.token {
		timer.request = timerRequest{token: asked.Token}
		timer.baseline = time.Time{}
	}
	timer.request.delay = asked.Delay
	timer.armed = true

	return timer, timer.request
}

// completed honors one completion of the latest request and returns the elapsed event it carries. A completion of
// another timing sequence, an earlier issue, or a sequence that is not owed one is dropped and changes nothing.
func (timer sequenceTimer) completed(message TimerMsg) (sequenceTimer, core.EventElapsed, bool) {
	if !timer.armed || message.Token != timer.request.token || message.Issue != timer.request.issue {
		return timer, core.EventElapsed{}, false
	}

	from := timer.baseline
	if from.IsZero() {
		from = message.Armed
	}
	timer.baseline = message.Fired
	timer.armed = false

	return timer, core.EventElapsed{Elapsed: coreDuration(message.Fired.Sub(from)), Token: message.Token}, true
}

// resumed re-arms a sequence still owed a completion after an undersized terminal recovers. The new issue makes the
// completion in flight while undersized arrive stale, and measuring from the new timer's arming means time spent
// undersized is never caught up. A sequence the core ended stays ended.
func (timer sequenceTimer) resumed() (sequenceTimer, timerRequest, bool) {
	if !timer.armed {
		return timer, timerRequest{}, false
	}
	timer.request.issue++
	timer.baseline = time.Time{}

	return timer, timer.request, true
}

// arm asks the host for one completion of request, stamped with when the timer started and when it fired.
func arm(request timerRequest) tea.Cmd {
	return func() tea.Msg {
		armed := time.Now()
		fired := <-time.After(hostDuration(request.delay))

		return TimerMsg{Token: request.token, Issue: request.issue, Armed: armed, Fired: fired}
	}
}

// hostDuration converts core time units into a host duration.
func hostDuration(duration core.Duration) time.Duration {
	return time.Duration(duration) * time.Second / time.Duration(core.DurationUnitsPerSecond)
}

// coreDuration converts measured host time into core time units. One unit is 25000/3 nanoseconds, so nanoseconds are
// multiplied before dividing; a measurement truncates by less than one unit.
func coreDuration(elapsed time.Duration) core.Duration {
	return core.Duration(elapsed.Nanoseconds() * 3 / 25000)
}
