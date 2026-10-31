package core_test

import (
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

func TestSetupNavigationVisitsEveryRowAndWrapsInSinglePlayer(t *testing.T) {
	t.Parallel()

	assertFocusSequence(t, core.EventNavigate{Direction: core.DirectionDown}, []core.SetupRow{
		core.SetupRowWinningScore, core.SetupRowDifficulty, core.SetupRowStart, core.SetupRowMode,
	})
	assertFocusSequence(t, core.EventNavigate{Direction: core.DirectionUp}, []core.SetupRow{
		core.SetupRowStart, core.SetupRowDifficulty, core.SetupRowWinningScore, core.SetupRowMode,
	})
}

func TestSetupNavigationSkipsDifficultyInLocalTwoPlayer(t *testing.T) {
	t.Parallel()

	local := core.EventAdjust{Direction: core.DirectionRight}
	assertFocusSequence(t, core.EventNavigate{Direction: core.DirectionDown}, []core.SetupRow{
		core.SetupRowWinningScore, core.SetupRowStart, core.SetupRowMode, core.SetupRowWinningScore,
	}, local)
	assertFocusSequence(t, core.EventNavigate{Direction: core.DirectionUp}, []core.SetupRow{
		core.SetupRowStart, core.SetupRowWinningScore, core.SetupRowMode, core.SetupRowStart,
	}, local)
}

func TestSetupModeTogglesFromEitherHorizontalKey(t *testing.T) {
	t.Parallel()

	for _, direction := range []core.HorizontalDirection{core.DirectionLeft, core.DirectionRight} {
		adjust := core.EventAdjust{Direction: direction}
		assertDraftSequence(t, adjust, []core.Settings{
			{Mode: core.ModeLocalTwoPlayer, WinningScore: core.WinningScore7, Difficulty: core.DifficultyNormal},
			{Mode: core.ModeSinglePlayer, WinningScore: core.WinningScore7, Difficulty: core.DifficultyNormal},
		})
	}
}

func TestSetupWinningScoreCyclesAndWrapsBothWays(t *testing.T) {
	t.Parallel()

	toScore := core.EventNavigate{Direction: core.DirectionDown}
	settings := func(score core.WinningScore) core.Settings {
		return core.Settings{Mode: core.ModeSinglePlayer, WinningScore: score, Difficulty: core.DifficultyNormal}
	}
	assertDraftSequence(t, core.EventAdjust{Direction: core.DirectionRight}, []core.Settings{
		settings(core.WinningScore11), settings(core.WinningScore5), settings(core.WinningScore7),
	}, toScore)
	assertDraftSequence(t, core.EventAdjust{Direction: core.DirectionLeft}, []core.Settings{
		settings(core.WinningScore5), settings(core.WinningScore11), settings(core.WinningScore7),
	}, toScore)
}

func TestSetupDifficultyCyclesAndWrapsBothWays(t *testing.T) {
	t.Parallel()

	down := core.EventNavigate{Direction: core.DirectionDown}
	settings := func(difficulty core.Difficulty) core.Settings {
		return core.Settings{Mode: core.ModeSinglePlayer, WinningScore: core.WinningScore7, Difficulty: difficulty}
	}
	assertDraftSequence(t, core.EventAdjust{Direction: core.DirectionRight}, []core.Settings{
		settings(core.DifficultyHard), settings(core.DifficultyEasy), settings(core.DifficultyNormal),
	}, down, down)
	assertDraftSequence(t, core.EventAdjust{Direction: core.DirectionLeft}, []core.Settings{
		settings(core.DifficultyEasy), settings(core.DifficultyHard), settings(core.DifficultyNormal),
	}, down, down)
}

func TestSetupModeSwitchKeepsDifficultyDraft(t *testing.T) {
	t.Parallel()

	down := core.EventNavigate{Direction: core.DirectionDown}
	up := core.EventNavigate{Direction: core.DirectionUp}
	toggle := core.EventAdjust{Direction: core.DirectionRight}
	hard := core.EventAdjust{Direction: core.DirectionRight}
	model, _ := replay([]core.Event{core.EventPrimary{}, down, down, hard, up, up, toggle})
	if draft := model.Draft(); draft.Mode != core.ModeLocalTwoPlayer || draft.Difficulty != core.DifficultyHard {
		t.Fatalf("local two-player draft = %+v, want local mode keeping hard difficulty", draft)
	}

	model, _ = core.Update(model, toggle)
	want := core.Settings{
		Mode:         core.ModeSinglePlayer,
		WinningScore: core.WinningScore7,
		Difficulty:   core.DifficultyHard,
	}
	if draft := model.Draft(); draft != want {
		t.Fatalf("draft after switching back = %+v, want %+v", draft, want)
	}
	model, _ = core.Update(model, up)
	model, _ = core.Update(model, up)
	if focus := model.SetupFocus(); focus != core.SetupRowDifficulty {
		t.Fatalf("focus after switching back and moving up twice = %v, want difficulty", focus)
	}
}

func TestSetupPrimaryAdvancesToNextAvailableRow(t *testing.T) {
	t.Parallel()

	enter := core.EventPrimary{}
	local := core.EventAdjust{Direction: core.DirectionRight}
	tests := []struct {
		name   string
		prefix []core.Event
		want   []core.SetupRow
	}{
		{
			name: "single player",
			want: []core.SetupRow{core.SetupRowWinningScore, core.SetupRowDifficulty, core.SetupRowStart},
		},
		{
			name:   "local two-player",
			prefix: []core.Event{local},
			want:   []core.SetupRow{core.SetupRowWinningScore, core.SetupRowStart},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model, _ := replay(append([]core.Event{enter}, test.prefix...))
			draft := model.Draft()
			for index, row := range test.want {
				var effect core.Effect
				model, effect = core.Update(model, enter)
				if focus := model.SetupFocus(); focus != row {
					t.Fatalf("after %d × Enter focus = %v, want %v", index+1, focus, row)
				}
				if model.Draft() != draft || effect.Kind() != core.EffectNone {
					t.Fatalf("Enter changed the draft to %+v or requested %v", model.Draft(), effect.Kind())
				}
			}
		})
	}
}

func TestSetupStartMatchIgnoresAdjustment(t *testing.T) {
	t.Parallel()

	up := core.EventNavigate{Direction: core.DirectionUp}
	initial, _ := replay([]core.Event{core.EventPrimary{}, up})
	if initial.SetupFocus() != core.SetupRowStart {
		t.Fatalf("focus = %v, want start match", initial.SetupFocus())
	}
	assertNoOps(t, initial, []core.Event{
		core.EventAdjust{Direction: core.DirectionLeft},
		core.EventAdjust{Direction: core.DirectionRight},
	})
}

// assertNoOps checks that each event leaves the whole model unchanged and requests no effect.
func assertNoOps(t *testing.T, initial core.Model, events []core.Event) {
	t.Helper()

	for _, event := range events {
		updated, effect := core.Update(initial, event)
		if updated != initial {
			t.Errorf("event %#v changed the model from %+v to %+v", event, initial, updated)
		}
		if effect.Kind() != core.EffectNone {
			t.Errorf("event %#v produced effect %v, want none", event, effect.Kind())
		}
	}
}

func TestSetupBackReturnsToTitleKeepingEditsAndReentryFocusesMode(t *testing.T) {
	t.Parallel()

	down := core.EventNavigate{Direction: core.DirectionDown}
	right := core.EventAdjust{Direction: core.DirectionRight}
	edited, _ := replay([]core.Event{core.EventPrimary{}, right, down, right})
	want := core.Settings{
		Mode:         core.ModeLocalTwoPlayer,
		WinningScore: core.WinningScore11,
		Difficulty:   core.DifficultyNormal,
	}
	if edited.Draft() != want {
		t.Fatalf("edited draft = %+v, want %+v", edited.Draft(), want)
	}

	title, effect := core.Update(edited, core.EventBack{})
	if title.Screen() != core.ScreenTitle || title.Draft() != want || effect.Kind() != core.EffectNone {
		t.Fatalf("Esc gave screen %v, draft %+v, effect %v; want title, kept draft, none",
			title.Screen(), title.Draft(), effect.Kind())
	}

	reentered, effect := core.Update(title, core.EventPrimary{})
	if reentered.Screen() != core.ScreenSetup || reentered.SetupFocus() != core.SetupRowMode {
		t.Fatalf("re-entry gave screen %v, focus %v; want setup focused on mode",
			reentered.Screen(), reentered.SetupFocus())
	}
	if reentered.Draft() != want || effect.Kind() != core.EffectNone {
		t.Fatalf("re-entry gave draft %+v, effect %v; want %+v, none", reentered.Draft(), effect.Kind(), want)
	}
}

func TestSetupIgnoresQuit(t *testing.T) {
	t.Parallel()

	down := core.EventNavigate{Direction: core.DirectionDown}
	for _, prefix := range [][]core.Event{{}, {down}, {down, down}, {down, down, down}} {
		initial, _ := replay(append([]core.Event{core.EventPrimary{}}, prefix...))
		assertNoOps(t, initial, []core.Event{core.EventQuit{}})
	}
}

// assertFocusSequence enters setup with the given prefix events and checks the focus after each navigation step.
func assertFocusSequence(t *testing.T, step core.EventNavigate, want []core.SetupRow, prefix ...core.Event) {
	t.Helper()

	model, _ := replay(append([]core.Event{core.EventPrimary{}}, prefix...))
	for index, row := range want {
		var effect core.Effect
		model, effect = core.Update(model, step)
		if focus := model.SetupFocus(); focus != row {
			t.Fatalf("after %d × %+v focus = %v, want %v", index+1, step, focus, row)
		}
		if effect.Kind() != core.EffectNone {
			t.Fatalf("navigation produced effect %v, want none", effect.Kind())
		}
	}
}

// assertDraftSequence enters setup with the given prefix events and checks the draft after each adjustment, which
// must keep the focused row and request no effect.
func assertDraftSequence(t *testing.T, step core.EventAdjust, want []core.Settings, prefix ...core.Event) {
	t.Helper()

	model, _ := replay(append([]core.Event{core.EventPrimary{}}, prefix...))
	focus := model.SetupFocus()
	for index, settings := range want {
		var effect core.Effect
		model, effect = core.Update(model, step)
		if draft := model.Draft(); draft != settings {
			t.Fatalf("after %d × %+v draft = %+v, want %+v", index+1, step, draft, settings)
		}
		if model.SetupFocus() != focus {
			t.Fatalf("adjustment moved focus from %v to %v", focus, model.SetupFocus())
		}
		if effect.Kind() != core.EffectNone {
			t.Fatalf("adjustment produced effect %v, want none", effect.Kind())
		}
	}
}
