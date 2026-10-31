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

	initial := core.NewModel()
	for _, event := range []core.Event{core.EventPrimary{}} {
		updated, effect := core.Update(initial, event)
		if updated != initial {
			t.Errorf("event %#v changed the model from %+v to %+v", event, initial, updated)
		}
		if effect.Kind() != core.EffectNone {
			t.Errorf("event %#v produced effect %v, want none", event, effect.Kind())
		}
	}
}

func TestReplayProducesIdenticalStateAndEffects(t *testing.T) {
	t.Parallel()

	events := []core.Event{core.EventPrimary{}, core.EventQuit{}, core.EventPrimary{}}
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
