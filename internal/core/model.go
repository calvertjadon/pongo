// Package core contains Pongo's deterministic application state and transitions.
package core

// Screen tags the active top-level application state for readers outside the core.
type Screen uint8

const (
	// ScreenTitle is the initial Pongo screen.
	ScreenTitle Screen = iota
)

// Event is a semantic input understood by the deterministic core. Its variants are closed to this package.
//
//sumtype:decl
type Event interface {
	isEvent()
}

// EventPrimary requests the active state's primary action.
type EventPrimary struct{}

// EventQuit requests a normal host exit.
type EventQuit struct{}

func (EventPrimary) isEvent() {}
func (EventQuit) isEvent()    {}

// EffectKind identifies which variant of host work an Effect requests.
type EffectKind uint8

const (
	// EffectNone requests no host work.
	EffectNone EffectKind = iota
	// EffectQuit asks the host shell to exit normally.
	EffectQuit
)

// Effect is a closed request for host work. Only the core constructs effects; the zero Effect requests nothing.
type Effect struct {
	kind EffectKind
}

// Kind returns the requested variant of host work.
func (effect Effect) Kind() EffectKind {
	return effect.kind
}

// state is the tagged top-level application state. Each variant owns its own data.
//
//sumtype:decl
type state interface {
	isState()
}

type titleState struct{}

func (titleState) isState() {}

// Model holds semantic application state. Use NewModel; the zero Model behaves as Title.
type Model struct {
	state state
}

// NewModel returns Pongo's initial semantic state.
func NewModel() Model {
	return Model{state: titleState{}}
}

// Screen returns the tag of the active top-level application state.
func (model Model) Screen() Screen {
	switch model.state.(type) {
	case titleState:
		return ScreenTitle
	case nil:
		return ScreenTitle
	}

	return ScreenTitle
}

// Update applies one semantic event and returns the resulting state and host effect.
// An event that does not apply to the active state preserves the model and requests no effect.
func Update(model Model, event Event) (Model, Effect) {
	switch model.state.(type) {
	case titleState:
		return updateTitle(model, event)
	case nil:
		return updateTitle(model, event)
	}

	return model, Effect{}
}

func updateTitle(model Model, event Event) (Model, Effect) {
	switch event.(type) {
	case EventQuit:
		return model, Effect{kind: EffectQuit}
	case EventPrimary:
		return model, Effect{}
	}

	return model, Effect{}
}
