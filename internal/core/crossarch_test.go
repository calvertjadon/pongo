package core_test

import (
	"math"
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

// determinismSteps is the length of the cross-architecture replay: 2400 fixed steps, or 20 seconds of match time.
const determinismSteps = 2400

// replayGolden is one replay's exact observable end state. The float fields are the IEEE 754 bit patterns of the final
// ball position, so the comparison is exact rather than tolerant.
type replayGolden struct {
	x, y                uint64
	leftTop, rightTop   uint8
	leftScore, rightScr uint8
}

func TestReplayIsBitIdenticalAcrossArchitectures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings core.Settings
		want     replayGolden
	}{
		{
			name: "local two-player match",
			settings: core.Settings{
				Mode:         core.ModeLocalTwoPlayer,
				WinningScore: core.WinningScore11,
				Difficulty:   core.DifficultyNormal,
			},
			want: replayGolden{
				x:         4633570110106485064,
				y:         4624098427588193472,
				leftTop:   0,
				rightTop:  0,
				leftScore: 9,
				rightScr:  0,
			},
		},
		{
			name: "single-player hard match",
			settings: core.Settings{
				Mode:         core.ModeSinglePlayer,
				WinningScore: core.WinningScore11,
				Difficulty:   core.DifficultyHard,
			},
			want: replayGolden{
				x:         4630052260657758208,
				y:         4621537642612260864,
				leftTop:   0,
				rightTop:  8,
				leftScore: 0,
				rightScr:  5,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model := replayDeterminismMatch(t, test.settings)
			x, y := model.BallPosition()
			leftTop, rightTop := model.PaddleTops()
			leftScore, rightScore := model.Score()
			got := replayGolden{
				x:         math.Float64bits(x),
				y:         math.Float64bits(y),
				leftTop:   leftTop,
				rightTop:  rightTop,
				leftScore: leftScore,
				rightScr:  rightScore,
			}
			if got != test.want {
				t.Fatalf("replay end state = %+v, want %+v; if this change is intended, recompute the golden on "+
					"amd64 and explain it in the commit", got, test.want)
			}
		})
	}
}

// replayDeterminismMatch starts a match, spends the countdown, and replays a fixed sequence of fixed steps. Paddle
// steps follow a schedule that depends only on the step number, so every architecture replays the same semantic
// events and only the core's arithmetic decides the state. This is what the macOS arm64 CI job compares with the
// amd64 values in the goldens: any fused multiply-add, or a math function that fuses internally, changes them.
func replayDeterminismMatch(t *testing.T, settings core.Settings) core.Model {
	t.Helper()

	model, schedule := startMatchWith(t, settings)
	model, token := beginServingRally(t, model, schedule.Timer().Token)
	for step := range determinismSteps {
		if step%9 == 0 {
			model, _ = core.Update(model, wsStep(determinismDirection(step/9)))
		}
		if step%14 == 0 {
			model, _ = core.Update(model, arrowStep(determinismDirection(step/14+2)))
		}

		var effect core.Effect
		model, effect = core.Update(model, core.EventElapsed{Elapsed: core.StepDuration, Token: token})
		if effect.Kind() == core.EffectScheduleTimer {
			token = effect.Timer().Token
		}
	}

	return model
}

// determinismDirection alternates a paddle direction every 40 calls of the fixed schedule.
func determinismDirection(phase int) core.VerticalDirection {
	if (phase/40)%2 == 0 {
		return core.DirectionUp
	}

	return core.DirectionDown
}
