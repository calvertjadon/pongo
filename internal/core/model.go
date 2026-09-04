// Package core contains Pongo's deterministic application state and transitions.
package core

// Screen identifies the active application screen.
type Screen uint8

const (
	// ScreenTitle is the initial Pongo screen.
	ScreenTitle Screen = iota
)

// Event is a semantic input understood by the deterministic core.
type Event uint8

const (
	// EventUnknown represents an input with no meaning in the current application state.
	EventUnknown Event = iota
	// EventPrimary requests the active screen's primary action.
	EventPrimary
	// EventQuit requests a normal host exit.
	EventQuit
)

// Effect requests work from the host shell.
type Effect uint8

const (
	// EffectNone requests no host work.
	EffectNone Effect = iota
	// EffectQuit asks the host shell to exit normally.
	EffectQuit
)

// Model holds semantic application state.
type Model struct {
	screen Screen
}

// NewModel returns Pongo's initial semantic state.
func NewModel() Model {
	return Model{screen: ScreenTitle}
}

// Screen returns the active application screen.
func (model Model) Screen() Screen {
	return model.screen
}

// Update applies one semantic event and returns the resulting state and host effect.
func Update(model Model, event Event) (Model, Effect) {
	if model.screen == ScreenTitle && event == EventQuit {
		return model, EffectQuit
	}

	return model, EffectNone
}
