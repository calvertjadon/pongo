package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/calvertjadon/pongo/internal/tui"
)

func TestCtrlCRequestsHostInterrupt(t *testing.T) {
	t.Parallel()

	_, command := tui.NewModel().Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if command == nil {
		t.Fatal("Ctrl+C returned no command, want an interrupt")
	}
	if _, interrupted := command().(tea.InterruptMsg); !interrupted {
		t.Fatal("Ctrl+C command does not interrupt the program")
	}
}

func TestQuitKeyRequestsNormalExit(t *testing.T) {
	t.Parallel()

	_, command := tui.NewModel().Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if command == nil {
		t.Fatal("Q on the title returned no command, want a quit")
	}
	if _, quit := command().(tea.QuitMsg); !quit {
		t.Fatal("Q on the title does not quit the program normally")
	}
}
