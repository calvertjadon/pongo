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

func TestTitleQuitRequestsHostExitWithoutChangingState(t *testing.T) {
	t.Parallel()

	initial := core.NewModel()
	updated, effect := core.Update(initial, core.EventQuit)

	if updated.Screen() != initial.Screen() {
		t.Fatalf("quit changed screen from %v to %v", initial.Screen(), updated.Screen())
	}
	if effect != core.EffectQuit {
		t.Fatalf("quit effect = %v, want host quit", effect)
	}
}

func TestTitleIgnoresInactiveEvents(t *testing.T) {
	t.Parallel()

	initial := core.NewModel()
	for _, event := range []core.Event{core.EventUnknown, core.EventPrimary} {
		updated, effect := core.Update(initial, event)
		if updated.Screen() != initial.Screen() {
			t.Errorf("event %v changed screen from %v to %v", event, initial.Screen(), updated.Screen())
		}
		if effect != core.EffectNone {
			t.Errorf("event %v produced effect %v, want none", event, effect)
		}
	}
}

func TestReplayProducesIdenticalStateAndEffects(t *testing.T) {
	t.Parallel()

	events := []core.Event{core.EventUnknown, core.EventPrimary, core.EventQuit}
	firstModel, firstEffects := replay(events)
	secondModel, secondEffects := replay(events)

	if firstModel.Screen() != secondModel.Screen() {
		t.Fatalf("replay screen = %v, want %v", secondModel.Screen(), firstModel.Screen())
	}
	if !slices.Equal(firstEffects, secondEffects) {
		t.Fatalf("replay effects = %v, want %v", secondEffects, firstEffects)
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
