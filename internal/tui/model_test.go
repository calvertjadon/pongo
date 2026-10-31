package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

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

func TestTitleIgnoresEnterAndUnrecognizedKeys(t *testing.T) {
	t.Parallel()

	keys := []tea.KeyPressMsg{
		{Code: tea.KeyEnter},
		{Code: 'x', Text: "x"},
		{Code: tea.KeyUp},
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

func sized(model tui.Model) tea.Model {
	resized, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 28})

	return resized
}
