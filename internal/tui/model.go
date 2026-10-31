// Package tui adapts Pongo's deterministic core to Bubble Tea and terminal output.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/calvertjadon/pongo/internal/core"
)

// Model adapts Bubble Tea host messages to the deterministic core.
type Model struct {
	application core.Model
	width       int
	height      int
	timer       timerChain
}

// NewModel returns a Bubble Tea model at Pongo's initial state.
func NewModel() Model {
	return Model{application: core.NewModel()}
}

// Init requests no startup work.
func (Model) Init() tea.Cmd {
	return nil
}

// Update translates one host message and performs any requested effect.
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		return model.resized(message.Width, message.Height)
	case tea.KeyPressMsg:
		if message.String() == "ctrl+c" {
			return model, tea.Interrupt
		}
		if model.undersized() {
			return model, gatedKey(message)
		}

		event, recognized := model.semanticEvent(message)
		if !recognized {
			return model, nil
		}

		delivered, effect := model.deliver(event)

		return delivered.perform(effect)
	case TimerMsg:
		if model.undersized() || !model.timer.matches(message) {
			return model, nil
		}

		delivered, effect := model.deliver(message.elapsed())
		if effect.Kind() != core.EffectScheduleTimer {
			delivered.timer = delivered.timer.stopped()
		}

		return delivered.perform(effect)
	}

	return model, nil
}

// View renders the current semantic state in the alternate screen.
func (model Model) View() tea.View {
	return tea.View{
		Content:   Render(model.application, model.width, model.height),
		AltScreen: true,
	}
}

// deliver passes one semantic event to the core and returns the model with the effect it requests.
func (model Model) deliver(event core.Event) (Model, core.Effect) {
	var effect core.Effect
	model.application, effect = core.Update(model.application, event)

	return model, effect
}

// perform maps one core effect to the host work that carries it out.
func (model Model) perform(effect core.Effect) (tea.Model, tea.Cmd) {
	switch effect.Kind() {
	case core.EffectNone:
		return model, nil
	case core.EffectQuit:
		return model, tea.Quit
	case core.EffectScheduleTimer:
		model.timer = model.timer.requested(effect.Timer())

		return model, model.timer.tick()
	default:
		return model, nil
	}
}

// semanticEvent translates one key press into the semantic event it carries. Keys with no meaning in the active state
// are dropped.
func (model Model) semanticEvent(message tea.KeyPressMsg) (core.Event, bool) {
	switch message.String() {
	case "q", "Q":
		return core.EventQuit{}, true
	case "enter":
		return core.EventPrimary{}, true
	case "esc":
		return core.EventBack{}, true
	case "p", "P":
		return model.pauseToggle()
	case "w", "W":
		return paddleStep(model, core.DirectionUp, core.BindingWS)
	case "s", "S":
		return paddleStep(model, core.DirectionDown, core.BindingWS)
	case "up":
		return verticalKey(model, core.DirectionUp)
	case "down":
		return verticalKey(model, core.DirectionDown)
	case "left":
		return core.EventAdjust{Direction: core.DirectionLeft}, true
	case "right":
		return core.EventAdjust{Direction: core.DirectionRight}, true
	default:
		return nil, false
	}
}

// pauseToggle translates P, which freezes a match in play and resumes a paused one.
func (model Model) pauseToggle() (core.Event, bool) {
	if model.application.Screen() != core.ScreenMatch {
		return nil, false
	}
	if model.paddling() || model.application.MatchPhase() == core.MatchPhasePaused {
		return core.EventPauseToggle{}, true
	}

	return nil, false
}

// verticalKey translates an arrow key: a paddle step while a match is in play, otherwise menu navigation.
func verticalKey(model Model, direction core.VerticalDirection) (core.Event, bool) {
	if model.paddling() {
		return core.EventPaddleStep{Direction: direction, Group: core.BindingArrows}, true
	}

	return core.EventNavigate{Direction: direction}, true
}

// paddleStep translates W or S, which move a paddle only while a match is in play.
func paddleStep(model Model, direction core.VerticalDirection, group core.BindingGroup) (core.Event, bool) {
	if !model.paddling() {
		return nil, false
	}

	return core.EventPaddleStep{Direction: direction, Group: group}, true
}

// paddling reports whether the core is in a match phase where keys move paddles.
func (model Model) paddling() bool {
	if model.application.Screen() != core.ScreenMatch {
		return false
	}

	switch model.application.MatchPhase() {
	case core.MatchPhaseCountdown, core.MatchPhaseRally:
		return true
	case core.MatchPhasePaused, core.MatchPhaseGameOver:
		return false
	default:
		return false
	}
}
