// Package core contains Pongo's deterministic application state and transitions.
package core

// Screen tags the active top-level application state for readers outside the core.
type Screen uint8

const (
	// ScreenTitle is the initial Pongo screen.
	ScreenTitle Screen = iota
	// ScreenSetup is the Match setup form.
	ScreenSetup
)

// Event is a semantic input understood by the deterministic core. Its variants are closed to this package.
//
//sumtype:decl
type Event interface {
	isEvent()
}

// EventPrimary requests the active state's primary action: on Title it opens Match setup, and in Match setup it
// accepts the focused row.
type EventPrimary struct{}

// EventQuit requests a normal host exit.
type EventQuit struct{}

// VerticalDirection is up or down.
type VerticalDirection uint8

const (
	// DirectionUp points toward the top of the screen.
	DirectionUp VerticalDirection = iota
	// DirectionDown points toward the bottom of the screen.
	DirectionDown
)

// EventNavigate moves a menu's focus one available row.
type EventNavigate struct {
	Direction VerticalDirection
}

// HorizontalDirection is left or right.
type HorizontalDirection uint8

const (
	// DirectionLeft points toward the left of the screen.
	DirectionLeft HorizontalDirection = iota
	// DirectionRight points toward the right of the screen.
	DirectionRight
)

// EventAdjust changes the focused value one step.
type EventAdjust struct {
	Direction HorizontalDirection
}

// EventBack leaves the active screen for the one before it.
type EventBack struct{}

func (EventPrimary) isEvent()  {}
func (EventQuit) isEvent()     {}
func (EventNavigate) isEvent() {}
func (EventAdjust) isEvent()   {}
func (EventBack) isEvent()     {}

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

// setupState is the Match setup form. The draft it edits belongs to the session, not to this state.
type setupState struct {
	focus SetupRow
}

func (titleState) isState() {}
func (setupState) isState() {}

// Model holds semantic application state. Use NewModel; the zero Model is not a valid state.
type Model struct {
	state state
	draft Settings
}

// NewModel returns Pongo's initial semantic state.
func NewModel() Model {
	return Model{state: titleState{}, draft: defaultSettings()}
}

// Draft returns the session's setup draft: the settings the next match will start with.
func (model Model) Draft() Settings {
	return model.draft
}

// SetupFocus returns the focused Match setup row. It is meaningful only while Screen is ScreenSetup.
func (model Model) SetupFocus() SetupRow {
	if setup, ok := model.state.(setupState); ok {
		return setup.focus
	}

	return SetupRowMode
}

// Screen returns the tag of the active top-level application state.
func (model Model) Screen() Screen {
	switch model.state.(type) {
	case titleState:
		return ScreenTitle
	case setupState:
		return ScreenSetup
	}

	return ScreenTitle
}

// Update applies one semantic event and returns the resulting state and host effect.
// An event that does not apply to the active state preserves the model and requests no effect.
func Update(model Model, event Event) (Model, Effect) {
	switch current := model.state.(type) {
	case titleState:
		return updateTitle(model, event)
	case setupState:
		return updateSetup(model, current, event)
	}

	return model, Effect{}
}

func updateTitle(model Model, event Event) (Model, Effect) {
	switch event.(type) {
	case EventQuit:
		return model, Effect{kind: EffectQuit}
	case EventPrimary:
		model.state = setupState{focus: SetupRowMode}
		return model, Effect{}
	case EventNavigate, EventAdjust, EventBack:
		return model, Effect{}
	}

	return model, Effect{}
}
