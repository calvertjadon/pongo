package tui

import "testing"

func TestBallCellRoundsHalfUpAndClampsTheDrawnCell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		coordinate float64
		limit      int
		want       int
	}{
		{name: "below half rounds down", coordinate: 34.49, limit: 70, want: 34},
		{name: "exact half rounds up", coordinate: 34.5, limit: 70, want: 35},
		{name: "above half rounds up", coordinate: 34.51, limit: 70, want: 35},
		{name: "zero is the first cell", coordinate: 0, limit: 20, want: 0},
		{name: "last cell is clamped", coordinate: 19.9, limit: 20, want: 19},
		{name: "below the court clamps to the first cell", coordinate: -0.2, limit: 70, want: 0},
		{name: "beyond the court clamps to the last cell", coordinate: 69.6, limit: 70, want: 69},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := ballCell(test.coordinate, test.limit); got != test.want {
				t.Errorf("ballCell(%v, %d) = %d, want %d", test.coordinate, test.limit, got, test.want)
			}
		})
	}
}
