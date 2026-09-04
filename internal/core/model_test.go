package core_test

import (
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

	if updated != initial {
		t.Fatalf("quit changed model from %#v to %#v", initial, updated)
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
		if updated != initial {
			t.Errorf("event %v changed model from %#v to %#v", event, initial, updated)
		}
		if effect != core.EffectNone {
			t.Errorf("event %v produced effect %v, want none", event, effect)
		}
	}
}

func TestReplayProducesIdenticalStateAndEffect(t *testing.T) {
	t.Parallel()

	events := []core.Event{core.EventUnknown, core.EventPrimary, core.EventQuit}
	firstModel, firstEffect := replay(events)
	secondModel, secondEffect := replay(events)

	if firstModel != secondModel || firstEffect != secondEffect {
		t.Fatalf(
			"replay mismatch: first = (%#v, %v), second = (%#v, %v)",
			firstModel,
			firstEffect,
			secondModel,
			secondEffect,
		)
	}
}

func replay(events []core.Event) (core.Model, core.Effect) {
	model := core.NewModel()
	effect := core.EffectNone
	for _, event := range events {
		model, effect = core.Update(model, event)
	}

	return model, effect
}
