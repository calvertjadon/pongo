package core

import (
	"math"
	"testing"
)

// The court's own angles stay below maxReboundAngle, but sinAngle and cosAngle are complete argument-reducing
// implementations of math's sine and cosine: the octant selection, the sign corrections, and the pi/4 reduction are
// all part of them. These tests evaluate the functions as sine and cosine, with an explicit angle, because no reachable
// rally turns strongly enough to exercise the upper octants.

// TestAngleFunctionsMatchMathAcrossThePrincipalRange checks both functions against math's at angles covering every
// octant and both signs.
func TestAngleFunctionsMatchMathAcrossThePrincipalRange(t *testing.T) {
	t.Parallel()

	angles := []float64{
		-math.Pi,
		-5 * math.Pi / 6,
		-3 * math.Pi / 4,
		-2 * math.Pi / 3,
		-math.Pi / 2,
		-math.Pi / 3,
		-math.Pi / 4,
		-math.Pi / 6,
		0,
		math.Pi / 6,
		math.Pi / 4,
		math.Pi / 3,
		math.Pi / 2,
		2 * math.Pi / 3,
		3 * math.Pi / 4,
		5 * math.Pi / 6,
		math.Pi,
		5 * math.Pi / 4,
		3 * math.Pi / 2,
		13 * math.Pi / 6,
	}
	for _, angle := range angles {
		if got, want := sinAngle(angle), math.Sin(angle); math.Abs(got-want) > 1e-12 {
			t.Errorf("sinAngle(%v) = %v, want %v", angle, got, want)
		}
		if got, want := cosAngle(angle), math.Cos(angle); math.Abs(got-want) > 1e-12 {
			t.Errorf("cosAngle(%v) = %v, want %v", angle, got, want)
		}
	}
}

// TestAngleFunctionsAreOddEvenAndPeriodic checks the identities that make the reduction correct: sine is odd, cosine
// is even, and both repeat after a full turn, which exercises the octant wrap past a half turn.
func TestAngleFunctionsAreOddEvenAndPeriodic(t *testing.T) {
	t.Parallel()

	for _, angle := range []float64{0.1, math.Pi / 5, 1.1} {
		if got, want := sinAngle(-angle), -sinAngle(angle); got != want {
			t.Errorf("sinAngle(%v) = %v, want the odd reflection %v", -angle, got, want)
		}
		if got, want := cosAngle(-angle), cosAngle(angle); got != want {
			t.Errorf("cosAngle(%v) = %v, want the even reflection %v", -angle, got, want)
		}
	}

	fullTurn := 2 * math.Pi
	for _, angle := range []float64{math.Pi / 6, math.Pi / 3, 2 * math.Pi / 3} {
		if got, want := sinAngle(angle+fullTurn), sinAngle(angle); math.Abs(got-want) > 1e-9 {
			t.Errorf("sinAngle(%v + 2π) = %v, want %v", angle, got, want)
		}
		if got, want := cosAngle(angle+fullTurn), cosAngle(angle); math.Abs(got-want) > 1e-9 {
			t.Errorf("cosAngle(%v + 2π) = %v, want %v", angle, got, want)
		}
	}
}

// TestAngleFunctionsMatchTheirRecordedBits pins the results as IEEE-754 bit patterns. The functions exist so that
// angles evaluate identically on every architecture, and math.Sin and math.Cos cannot be the oracle for that: the
// arm64 compiler fuses their polynomial multiply-adds and changes their last bits, while these explicit-rounding
// versions must not. The goldens are therefore the amd64 results and every architecture must reproduce them.
func TestAngleFunctionsMatchTheirRecordedBits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		angle  float64
		sine   uint64
		cosine uint64
	}{
		{angle: -5 * math.Pi / 6, sine: 13826050856027422719, cosine: 13829347719771606187},
		{angle: -math.Pi / 6, sine: 13826050856027422720, cosine: 4605975682916830378},
		{angle: 0, sine: 0, cosine: 4607182418800017408},
		{angle: math.Pi / 6, sine: 4602678819172646912, cosine: 4605975682916830378},
		{angle: math.Pi / 4, sine: 4604544271217802188, cosine: 4604544271217802189},
		{angle: math.Pi / 3, sine: 4605975682916830379, cosine: 4602678819172646911},
		{angle: math.Pi / 2, sine: 4607182418800017408, cosine: 4364452196894661632},
		{angle: 5 * math.Pi / 6, sine: 4602678819172646911, cosine: 13829347719771606187},
		{angle: math.Pi, sine: 4368955796522032128, cosine: 13830554455654793216},
		{angle: 5 * math.Pi / 4, sine: 13827916308072577996, cosine: 13827916308072577998},
		{angle: 13 * math.Pi / 6, sine: 4602678819172646912, cosine: 4605975682916830378},
	}
	for _, test := range tests {
		if got := math.Float64bits(sinAngle(test.angle)); got != test.sine {
			t.Errorf("sinAngle(%v) bits = %d, want %d", test.angle, got, test.sine)
		}
		if got := math.Float64bits(cosAngle(test.angle)); got != test.cosine {
			t.Errorf("cosAngle(%v) bits = %d, want %d", test.angle, got, test.cosine)
		}
	}
}
