package core_test

import (
	"slices"
	"testing"

	"github.com/calvertjadon/pongo/internal/core"
)

func TestNewModelStartsAtTitle(t *testing.T) {
	t.Parallel()

	model := core.NewModel()
	if model.Screen() != core.ScreenTitle {
		t.Fatalf("initial screen = %v, want title", model.Screen())
	}
}

func TestNewModelHasDefaultSetupDraft(t *testing.T) {
	t.Parallel()

	want := core.Settings{
		Mode:         core.ModeSinglePlayer,
		WinningScore: core.WinningScore7,
		Difficulty:   core.DifficultyNormal,
	}
	if draft := core.NewModel().Draft(); draft != want {
		t.Fatalf("initial draft = %+v, want %+v", draft, want)
	}
}

func TestTitlePrimaryOpensSetupFocusedOnModeWithDraft(t *testing.T) {
	t.Parallel()

	initial := core.NewModel()
	updated, effect := core.Update(initial, core.EventPrimary{})

	if updated.Screen() != core.ScreenSetup {
		t.Fatalf("screen after primary = %v, want setup", updated.Screen())
	}
	if focus := updated.SetupFocus(); focus != core.SetupRowMode {
		t.Errorf("setup focus = %v, want mode", focus)
	}
	if updated.Draft() != initial.Draft() {
		t.Errorf("entering setup changed the draft from %+v to %+v", initial.Draft(), updated.Draft())
	}
	if effect.Kind() != core.EffectNone {
		t.Errorf("entering setup produced effect %v, want none", effect.Kind())
	}
}

func TestTitleQuitRequestsHostExitWithoutChangingState(t *testing.T) {
	t.Parallel()

	initial := core.NewModel()
	updated, effect := core.Update(initial, core.EventQuit{})

	if updated != initial {
		t.Fatalf("quit changed the model from %+v to %+v", initial, updated)
	}
	if effect.Kind() != core.EffectQuit {
		t.Fatalf("quit effect = %v, want host quit", effect.Kind())
	}
}

func TestTitleIgnoresInactiveEvents(t *testing.T) {
	t.Parallel()

	assertNoOps(t, core.NewModel(), []core.Event{
		core.EventNavigate{Direction: core.DirectionUp},
		core.EventNavigate{Direction: core.DirectionDown},
		core.EventAdjust{Direction: core.DirectionLeft},
		core.EventAdjust{Direction: core.DirectionRight},
		core.EventBack{},
	})
}

func TestReplayProducesIdenticalStateAndEffects(t *testing.T) {
	t.Parallel()

	events := []core.Event{
		core.EventPrimary{},
		core.EventAdjust{Direction: core.DirectionRight},
		core.EventNavigate{Direction: core.DirectionDown},
		core.EventAdjust{Direction: core.DirectionLeft},
		core.EventPrimary{},
		core.EventQuit{},
		core.EventBack{},
		core.EventQuit{},
		core.EventPrimary{},
		core.EventNavigate{Direction: core.DirectionUp},
	}
	firstModel, firstEffects := replay(events)
	secondModel, secondEffects := replay(events)

	if firstModel != secondModel {
		t.Fatalf("replay model = %+v, want %+v", secondModel, firstModel)
	}
	if !slices.Equal(firstEffects, secondEffects) {
		t.Fatalf("replay effects = %+v, want %+v", secondEffects, firstEffects)
	}
}

func replay(events []core.Event) (core.Model, []core.Effect) {
	model := core.NewModel()
	effects := make([]core.Effect, 0, len(events))
	for _, event := range events {
		var effect core.Effect
		model, effect = core.Update(model, event)
		effects = append(effects, effect)
	}

	return model, effects
}
