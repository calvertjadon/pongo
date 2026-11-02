package core

import "testing"

// Public replay cannot land on a scoring boundary exactly: the ball starts at a fractional court position and moves by
// fractional velocities, so the scoring comparison is decided one step on either side of the last column. This test
// drives the boundary with explicit ball positions instead.

// TestScorerAwardsThePointOnlyPastTheLastColumn checks that the ball scores when it leaves the court past the last
// interior column and not while it is still on an edge column: the ball leaving the right edge concedes to the left
// player and the ball leaving the left edge concedes to the right player.
func TestScorerAwardsThePointOnlyPastTheLastColumn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		x      float64
		scorer Feedback
		scored bool
	}{
		{name: "at the last interior column", x: CourtWidth - 1},
		{name: "past the last interior column", x: CourtWidth - 0.5, scorer: FeedbackLeftScored, scored: true},
		{name: "at the left edge", x: 0},
		{name: "past the left edge", x: -0.5, scorer: FeedbackRightScored, scored: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			scorer, scored := ball{x: test.x}.scorer()
			if scored != test.scored {
				t.Fatalf("ball at x = %v scored = %v, want %v", test.x, scored, test.scored)
			}
			if scorer != test.scorer {
				t.Errorf("ball at x = %v scored for %v, want %v", test.x, scorer, test.scorer)
			}
		})
	}
}
