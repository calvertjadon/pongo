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
		model.width = message.Width
		model.height = message.Height
	case tea.KeyPressMsg:
		event, recognized := semanticEvent(message)
		if !recognized {
			return model, nil
		}

		var effect core.Effect
		model.application, effect = core.Update(model.application, event)
		return model, command(effect)
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

func semanticEvent(message tea.KeyPressMsg) (core.Event, bool) {
	switch message.String() {
	case "q", "Q":
		return core.EventQuit{}, true
	case "enter":
		return core.EventPrimary{}, true
	default:
		return nil, false
	}
}

func command(effect core.Effect) tea.Cmd {
	switch effect.Kind() {
	case core.EffectQuit:
		return tea.Quit
	case core.EffectNone:
		return nil
	default:
		return nil
	}
}
