package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/calvertjadon/pongo/internal/core"
)

// TimerMsg is one timer completion. It carries the correlation the shell requested and when the host ran the timer
// command, so the model measures elapsed time from the previous completion instead of reading a clock, and tests can
// build completions.
type TimerMsg struct {
	// Token is the core timing sequence the timer belongs to.
	Token core.TimerToken
	// Issue is the shell's request number for that sequence; only the latest issue is honored.
	Issue uint64
	// Fired is when the host ran the timer command.
	Fired time.Time
}

// elapsed converts one completion into the core's elapsed event, measured from the sequence's baseline: the previous
// completion's fire time, or when the sequence (or a recovery) was started.
func (chain timerChain) elapsed(message TimerMsg) core.Event {
	return core.EventElapsed{Elapsed: coreDuration(message.Fired.Sub(chain.baseline)), Token: message.Token}
}

// timerChain is the shell's timer scheduling state: the request it issued last, a request number that tells each
// issued timer apart, the fire time elapsed is measured from, and whether a timing sequence is still owed time. A
// resumed sequence can leave two timers in flight, so only the latest issue is honored.
type timerChain struct {
	token    core.TimerToken
	delay    core.Duration
	issue    uint64
	baseline time.Time
	live     bool
}

// requested records a core timer request as the chain's latest one. A sequence that starts, or whose token changes,
// measures from this request; a continuing sequence measures from the previous completion. The sequence stays live
// until the core answers a completion without asking for another timer.
func (chain timerChain) requested(timer core.Timer) timerChain {
	if !chain.live || timer.Token != chain.token {
		chain.baseline = time.Now()
	}
	chain.token = timer.Token
	chain.delay = timer.Delay
	chain.issue++
	chain.live = true

	return chain
}

// matches reports whether one completion belongs to the chain's latest request. A chain with no live sequence matches
// nothing, and only the latest request number of the live sequence is honored, so a completion left over from a
// paused, abandoned, or undersized sequence is dropped.
func (chain timerChain) matches(message TimerMsg) bool {
	return chain.live && message.Token == chain.token && message.Issue == chain.issue
}

// stopped records that the core answered the latest completion without requesting another timer. The sequence is over,
// so recovery must not re-arm it.
func (chain timerChain) stopped() timerChain {
	chain.live = false

	return chain
}

// resumed re-arms a live sequence after an undersized terminal. The new request number makes the completion that was
// in flight while undersized arrive stale, and the fresh baseline taken now means time spent undersized is never
// caught up.
func (chain timerChain) resumed() (timerChain, tea.Cmd) {
	if !chain.live {
		return chain, nil
	}
	chain.issue++
	chain.baseline = time.Now()

	return chain, chain.tick()
}

// tick asks the host for one timer completion for the chain's latest request.
func (chain timerChain) tick() tea.Cmd {
	token, issue := chain.token, chain.issue

	return tea.Tick(hostDuration(chain.delay), func(fired time.Time) tea.Msg {
		return TimerMsg{Token: token, Issue: issue, Fired: fired}
	})
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
