package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/calvertjadon/pongo/internal/core"
	"github.com/calvertjadon/pongo/internal/tui"
)

func TestTitleQuitKeysRequestHostQuit(t *testing.T) {
	t.Parallel()

	for _, key := range []tea.KeyPressMsg{{Code: 'q', Text: "q"}, {Code: 'q', Text: "Q", Mod: tea.ModShift}} {
		initial := sized(tui.NewModel())
		updated, cmd := initial.Update(key)

		if cmd == nil {
			t.Fatalf("key %q produced no command, want host quit", key.String())
		}
		if _, quit := cmd().(tea.QuitMsg); !quit {
			t.Errorf("key %q command does not quit the host", key.String())
		}
		if updated.View().Content != initial.View().Content {
			t.Errorf("key %q changed the title view", key.String())
		}
	}
}

func TestTitleIgnoresKeysWithoutTitleMeaning(t *testing.T) {
	t.Parallel()

	keys := []tea.KeyPressMsg{
		{Code: 'x', Text: "x"},
		{Code: tea.KeyUp},
		{Code: tea.KeyLeft},
		{Code: tea.KeyEscape},
		{Code: 'p', Text: "p"},
	}
	for _, key := range keys {
		initial := sized(tui.NewModel())
		updated, cmd := initial.Update(key)

		if cmd != nil {
			t.Errorf("key %q produced a command, want none", key.String())
		}
		if updated.View().Content != initial.View().Content {
			t.Errorf("key %q changed the title view", key.String())
		}
	}
}

func TestSetupKeysDriveCoreSetupEvents(t *testing.T) {
	t.Parallel()

	keys := []tea.KeyPressMsg{
		{Code: tea.KeyEnter},
		{Code: tea.KeyRight},
		{Code: tea.KeyDown},
		{Code: tea.KeyLeft},
		{Code: tea.KeyUp},
		{Code: 'q', Text: "q"},
		{Code: tea.KeyEnter},
		{Code: tea.KeyEscape},
		{Code: tea.KeyEnter},
		{Code: tea.KeyDown},
	}
	events := []core.Event{
		core.EventPrimary{},
		core.EventAdjust{Direction: core.DirectionRight},
		core.EventNavigate{Direction: core.DirectionDown},
		core.EventAdjust{Direction: core.DirectionLeft},
		core.EventNavigate{Direction: core.DirectionUp},
		core.EventQuit{},
		core.EventPrimary{},
		core.EventBack{},
		core.EventPrimary{},
		core.EventNavigate{Direction: core.DirectionDown},
	}

	model := sized(tui.NewModel())
	application := core.NewModel()
	for index, key := range keys {
		var cmd tea.Cmd
		model, cmd = model.Update(key)
		application, _ = core.Update(application, events[index])

		if cmd != nil {
			t.Fatalf("key %q produced a command, want none", key.String())
		}
		if model.View().Content != tui.Render(application, 80, 28) {
			t.Fatalf("key %q does not show the core state after %#v", key.String(), events[index])
		}
	}
}

func sized(model tui.Model) tea.Model {
	resized, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 28})

	return resized
}
