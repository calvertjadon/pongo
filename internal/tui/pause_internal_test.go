package tui

import "testing"

// TestWriteTextClipsAtBothRowEdges pins the box-drawing helper's clipping: text that starts left of the row or runs
// past its end keeps only the cells that fit instead of writing outside the frame.
func TestWriteTextClipsAtBothRowEdges(t *testing.T) {
	t.Parallel()

	row := blankRow(6)
	writeText(row, -2, "abcdefghij")
	if got, want := string(row), "cdefgh"; got != want {
		t.Errorf("text starting before the row = %q, want %q", got, want)
	}

	row = blankRow(6)
	writeText(row, 4, "xyz")
	if got, want := string(row), "    xy"; got != want {
		t.Errorf("text running past the row = %q, want %q", got, want)
	}
}
