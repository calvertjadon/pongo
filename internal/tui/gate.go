package tui

import tea "charm.land/bubbletea/v2"

// undersized reports whether the terminal cannot show the fixed layout. While it cannot, the shell keeps the latest
// dimensions, sends the core no gameplay, navigation, or elapsed-time events, and ignores timer completions. Resizing,
// Q, and Ctrl+C stay active.
func (model Model) undersized() bool {
	return undersized(model.width, model.height)
}

// resized stores the latest terminal dimensions and re-arms the live timing sequence when the terminal recovers from
// the undersized condition. Resizing never changes application state: an adequate resize only moves the layout origin,
// and recovery shows the preserved state with a fresh timing baseline and no catch-up.
func (model Model) resized(width, height int) (Model, tea.Cmd) {
	recovered := model.undersized() && !undersized(width, height)
	model.width = width
	model.height = height

	if !recovered {
		return model, nil
	}

	var request timerRequest
	var rearmed bool
	model.timer, request, rearmed = model.timer.resumed()
	if !rearmed {
		return model, nil
	}

	return model, arm(request)
}

// gatedKey performs the only key actions an undersized terminal keeps: Q exits the host immediately and Ctrl+C
// interrupts it, both without the core. Every other key is dropped.
func gatedKey(message tea.KeyPressMsg) tea.Cmd {
	switch message.String() {
	case "q", "Q":
		return tea.Quit
	default:
		return nil
	}
}
