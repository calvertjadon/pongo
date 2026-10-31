package core

import "testing"

func TestSplitMix64KnownAnswer(t *testing.T) {
	t.Parallel()

	var random generator
	if got := random.next(); got != 0xE220A8397B1DCDAF {
		t.Fatalf("first SplitMix64 output from state 0 = %#x, want the published 0xE220A8397B1DCDAF", got)
	}
}

func TestDifficultyProfilesMatchTheSpecifiedSchedule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		difficulty Difficulty
		want       computerProfile
	}{
		{
			difficulty: DifficultyEasy,
			want: computerProfile{
				reaction:   48,
				reaim:      30,
				moveEvery:  20,
				errorRows:  4,
				prediction: predictionObserved,
			},
		},
		{
			difficulty: DifficultyNormal,
			want: computerProfile{
				reaction:   24,
				reaim:      18,
				moveEvery:  12,
				errorRows:  3,
				prediction: predictionOneWall,
			},
		},
		{
			difficulty: DifficultyHard,
			want: computerProfile{
				reaction:   12,
				reaim:      12,
				moveEvery:  8,
				errorRows:  2.25,
				prediction: predictionEveryWall,
			},
		},
	}
	for _, test := range tests {
		t.Run(difficultyName(test.difficulty), func(t *testing.T) {
			t.Parallel()

			if got := profileFor(test.difficulty); got != test.want {
				t.Errorf("profile = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestPredictionDiffersByProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		vy     float64
		easy   float64
		normal float64
		hard   float64
	}{
		{name: "straight to the paddle", vy: 3, easy: 9.5, normal: 14.375, hard: 14.375},
		{name: "one wall", vy: -10, easy: 9.5, normal: 6.75, hard: 6.75},
		{name: "more than one wall", vy: -100, easy: 9.5, normal: 19, hard: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			current := ball{x: 34.5, y: 9.5, vx: 20, vy: test.vy}
			if got := predictedRow(profileFor(DifficultyEasy), current); got != test.easy {
				t.Errorf("Easy prediction = %v, want the observed row %v", got, test.easy)
			}
			if got := predictedRow(profileFor(DifficultyNormal), current); got != test.normal {
				t.Errorf("Normal prediction = %v, want %v", got, test.normal)
			}
			if got := predictedRow(profileFor(DifficultyHard), current); got != test.hard {
				t.Errorf("Hard prediction = %v, want %v", got, test.hard)
			}
		})
	}
}

func TestErrorOffsetSpansTheProfileAmplitude(t *testing.T) {
	t.Parallel()

	for _, difficulty := range []Difficulty{DifficultyEasy, DifficultyNormal, DifficultyHard} {
		t.Run(difficultyName(difficulty), func(t *testing.T) {
			t.Parallel()

			profile := profileFor(difficulty)
			random := seededComputer(0, difficulty)
			low, high := 0.0, 0.0
			for range 1000 {
				drawn := random.offset(profile.errorRows)
				if drawn < -profile.errorRows || drawn >= profile.errorRows {
					t.Fatalf("error %v outside [-%v, %v)", drawn, profile.errorRows, profile.errorRows)
				}
				low = min(low, drawn)
				high = max(high, drawn)
			}
			if low > -0.99*profile.errorRows || high < 0.99*profile.errorRows {
				t.Errorf("1000 draws span [%v, %v], want close to ±%v", low, high, profile.errorRows)
			}
		})
	}
}

func TestSeedVariesByPositionAndDifficulty(t *testing.T) {
	t.Parallel()

	seen := make(map[generator]bool)
	for _, difficulty := range []Difficulty{DifficultyEasy, DifficultyNormal, DifficultyHard} {
		for position := range uint8(4) {
			seed := seededComputer(position, difficulty)
			if seen[seed] {
				t.Fatalf("seed for position %d and difficulty %v repeats", position, difficulty)
			}
			seen[seed] = true
		}
	}
}

// difficultyName names a difficulty for a subtest.
func difficultyName(difficulty Difficulty) string {
	switch difficulty {
	case DifficultyEasy:
		return "easy"
	case DifficultyNormal:
		return "normal"
	case DifficultyHard:
		return "hard"
	}

	return "unknown"
}
