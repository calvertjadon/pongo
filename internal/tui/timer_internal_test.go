package tui

import (
	"testing"
	"time"

	"github.com/calvertjadon/pongo/internal/core"
)

// at returns a synthetic host time offset after a fixed epoch; no test reads the host clock.
func at(offset time.Duration) time.Time {
	return time.Date(2026, time.October, 31, 12, 0, 0, 0, time.UTC).Add(offset)
}

// timerFor returns a sequence timer that has recorded one core request for token.
func timerFor(token core.TimerToken) sequenceTimer {
	timer, _ := sequenceTimer{}.requested(core.Timer{Token: token, Delay: core.TimerDelay})

	return timer
}

// honor delivers message and fails unless the timer honors it, returning the timer and the elapsed time it reported.
func honor(t *testing.T, timer sequenceTimer, message TimerMsg) (sequenceTimer, core.Duration) {
	t.Helper()

	timer, event, honored := timer.completed(message)
	if !honored {
		t.Fatalf("completion %+v was dropped, want it honored", message)
	}
	if event.Token != message.Token {
		t.Fatalf("elapsed event token = %d, want %d", event.Token, message.Token)
	}

	return timer, event.Elapsed
}

func TestNewSequenceMeasuresFromItsTimersArming(t *testing.T) {
	t.Parallel()

	_, elapsed := honor(t, timerFor(1), TimerMsg{Token: 1, Armed: at(0), Fired: at(75 * time.Millisecond)})
	if elapsed != core.ElapsedLimit {
		t.Fatalf("elapsed = %d, want %d (75 ms)", elapsed, core.ElapsedLimit)
	}
}

func TestContinuingSequenceMeasuresFromThePreviousFire(t *testing.T) {
	t.Parallel()

	timer, _ := honor(t, timerFor(1), TimerMsg{Token: 1, Armed: at(0), Fired: at(10 * time.Millisecond)})
	timer, _ = timer.requested(core.Timer{Token: 1, Delay: core.TimerDelay})

	// The host stalled for 90 ms before arming the next timer. Measuring from its arming would lose that time.
	_, elapsed := honor(t, timer, TimerMsg{
		Token: 1,
		Armed: at(100 * time.Millisecond),
		Fired: at(110 * time.Millisecond),
	})
	if want := core.Duration(12000); elapsed != want {
		t.Fatalf("elapsed = %d, want %d (100 ms since the previous fire)", elapsed, want)
	}
}

func TestNewTokenStartsAFreshSequence(t *testing.T) {
	t.Parallel()

	timer, _ := honor(t, timerFor(1), TimerMsg{Token: 1, Armed: at(0), Fired: at(10 * time.Millisecond)})
	timer, _ = timer.requested(core.Timer{Token: 1, Delay: core.TimerDelay})
	timer, _, _ = timer.resumed()
	timer, request := timer.requested(core.Timer{Token: 2, Delay: core.TimerDelay})
	if request.issue != 0 {
		t.Fatalf("a new sequence's first issue = %d, want 0", request.issue)
	}

	_, elapsed := honor(t, timer, TimerMsg{
		Token: 2,
		Armed: at(time.Second),
		Fired: at(time.Second + 10*time.Millisecond),
	})
	if want := core.Duration(1200); elapsed != want {
		t.Fatalf("elapsed = %d, want %d (10 ms since arming, not since the old sequence)", elapsed, want)
	}
}

func TestRecoveryMeasuresFromTheRearmedTimerWithoutCatchUp(t *testing.T) {
	t.Parallel()

	timer, _ := honor(t, timerFor(1), TimerMsg{Token: 1, Armed: at(0), Fired: at(10 * time.Millisecond)})
	timer, _ = timer.requested(core.Timer{Token: 1, Delay: core.TimerDelay})

	// The terminal stays undersized for five seconds; the model drops the completion that fires meanwhile.
	timer, request, rearmed := timer.resumed()
	if !rearmed {
		t.Fatal("recovery did not re-arm a sequence owed a completion")
	}
	if request.token != 1 || request.issue != 1 {
		t.Fatalf("re-armed request = token %d issue %d, want token 1 issue 1", request.token, request.issue)
	}
	if _, _, honored := timer.completed(TimerMsg{Token: 1, Issue: 0, Fired: at(5 * time.Second)}); honored {
		t.Fatal("the completion in flight while undersized was honored after recovery")
	}

	_, elapsed := honor(t, timer, TimerMsg{
		Token: 1,
		Issue: 1,
		Armed: at(5 * time.Second),
		Fired: at(5*time.Second + 10*time.Millisecond),
	})
	if want := core.Duration(1200); elapsed != want {
		t.Fatalf("elapsed = %d, want %d (10 ms since recovery, no undersized time)", elapsed, want)
	}
}

func TestEndedSequenceIsNotRearmedByRecovery(t *testing.T) {
	t.Parallel()

	tests := map[string]sequenceTimer{
		"never requested": {},
	}
	// The core answered the completion without asking for another timer: a pause or the winning point.
	answered, _ := honor(t, timerFor(1), TimerMsg{Token: 1, Armed: at(0), Fired: at(10 * time.Millisecond)})
	tests["answered without a request"] = answered

	for name, timer := range tests {
		if _, _, rearmed := timer.resumed(); rearmed {
			t.Errorf("%s: recovery re-armed an ended sequence", name)
		}
	}
}

func TestCompletionsOutsideTheLatestRequestAreDropped(t *testing.T) {
	t.Parallel()

	answered, _ := honor(t, timerFor(1), TimerMsg{Token: 1, Armed: at(0), Fired: at(10 * time.Millisecond)})
	recovered, _, _ := timerFor(1).resumed()
	resumedAfterPause, _ := timerFor(1).requested(core.Timer{Token: 2, Delay: core.TimerDelay})

	tests := []struct {
		name    string
		timer   sequenceTimer
		message TimerMsg
	}{
		{"never requested", sequenceTimer{}, TimerMsg{Token: 1}},
		{"foreign token", timerFor(1), TimerMsg{Token: 99}},
		{"earlier issue", recovered, TimerMsg{Token: 1, Issue: 0}},
		{"later issue", timerFor(1), TimerMsg{Token: 1, Issue: 1}},
		{"sequence the core ended", answered, TimerMsg{Token: 1}},
		{"pre-pause timer after resume", resumedAfterPause, TimerMsg{Token: 1}},
	}
	for _, test := range tests {
		message := test.message
		message.Armed, message.Fired = at(time.Second), at(time.Second+75*time.Millisecond)
		after, _, honored := test.timer.completed(message)
		if honored {
			t.Errorf("%s: completion honored, want dropped", test.name)
		}
		if after != test.timer {
			t.Errorf("%s: dropping the completion changed the timer", test.name)
		}
	}
}
