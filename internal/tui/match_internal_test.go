package tui

import (
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

// TestBallCellRoundsToTheNearestCourtCell pins the ball's cell mapping: the nearest cell wins, an exact half rounds to
// the higher cell, and a coordinate outside the court clamps to its edge so the ball is never drawn off the court.
func TestBallCellRoundsToTheNearestCourtCell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		coordinate float64
		limit      int
		want       int
	}{
		{name: "below the court clamps to the first cell", coordinate: -0.6, limit: core.CourtWidth, want: 0},
		{name: "just below zero rounds up to the first cell", coordinate: -0.1, limit: core.CourtWidth, want: 0},
		{name: "whole coordinate", coordinate: 12, limit: core.CourtWidth, want: 12},
		{name: "below half rounds down", coordinate: 12.4, limit: core.CourtWidth, want: 12},
		{name: "exact half rounds up", coordinate: 12.5, limit: core.CourtWidth, want: 13},
		{name: "last cell", coordinate: 69, limit: core.CourtWidth, want: 69},
		{name: "beyond the court clamps to the last cell", coordinate: 70.4, limit: core.CourtWidth, want: 69},
	}
	for _, test := range tests {
		if got := ballCell(test.coordinate, test.limit); got != test.want {
			t.Errorf("%s: ballCell(%v, %d) = %d, want %d", test.name, test.coordinate, test.limit, got, test.want)
		}
	}
}

// TestCountdownDigitNeverShowsAnythingButOneTwoOrThree pins the countdown mapping's clamp: whatever step count it is
// handed, the court shows only the three, two, or one that the countdown contract names.
func TestCountdownDigitNeverShowsAnythingButOneTwoOrThree(t *testing.T) {
	t.Parallel()

	for remaining := core.Steps(0); remaining <= 2*core.CountdownSteps; remaining++ {
		if digit := countdownDigit(remaining); digit < '1' || digit > '3' {
			t.Fatalf("countdownDigit(%d) = %q, want a digit from 1 to 3", remaining, digit)
		}
	}
}

// TestWriteCenteredClipsTextWiderThanTheRow pins the centering helper's clipping: text that does not fit loses the
// cells that fall outside the row instead of writing outside the frame.
func TestWriteCenteredClipsTextWiderThanTheRow(t *testing.T) {
	t.Parallel()

	row := blankRow(4)
	writeCentered(row, "abcdefgh")
	if got, want := string(row), "cdef"; got != want {
		t.Errorf("centered wide text = %q, want %q", got, want)
	}
}
