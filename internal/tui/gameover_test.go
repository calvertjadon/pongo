package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/calvertjadon/pongo/internal/core"
	"github.com/calvertjadon/pongo/internal/tui"
)

// Game-over overlay frame rows and columns, mirrored from the fixed layout under test.
const (
	frameGameOverTop    = 8
	frameGameOverBottom = 19
	frameGameOverLeft   = 16
	frameGameOverRight  = 63
)

func TestGameOverOverlayShowsTheWinnerAndFinalScore(t *testing.T) {
	t.Parallel()

	finished := playedToGameOver(t)
	if left, right := finished.Score(); left != 7 || right != 0 {
		t.Fatalf("final score = %d:%d, want 7:0", left, right)
	}

	view := tui.Render(finished, 80, 28)
	if width, height := lipgloss.Width(view), lipgloss.Height(view); width != 80 || height != 28 {
		t.Fatalf("game-over view is %dx%d, want 80x28", width, height)
	}

	frame := lines(view)
	overlay := frame[frameGameOverTop : frameGameOverBottom+1]
	if !strings.Contains(strings.Join(overlay, "\n"), "PLAYER 1 WINS") {
		t.Errorf("overlay = %q, want the winner named", overlay)
	}
	scoreRow, _ := rowWithin(overlay, "7")
	if scoreRow < 0 || !strings.Contains(overlay[scoreRow], "0") ||
		!strings.Contains(overlay[scoreRow], "-") {
		t.Errorf("overlay score row = %q, want the final score", overlay)
	}
	for _, row := range []string{"Rematch", "Change setup", "Return to title"} {
		if !strings.Contains(strings.Join(overlay, "\n"), row) {
			t.Errorf("overlay = %q, want the %q choice", overlay, row)
		}
	}
	if !strings.Contains(strings.Join(overlay, "\n"), "> Rematch") {
		t.Errorf("overlay = %q, want Rematch selected", overlay)
	}
}

func TestGameOverHidesTheBallAndKeepsTheFinalCourt(t *testing.T) {
	t.Parallel()

	finished := playedToGameOver(t)
	if finished.BallVisible() {
		t.Error("the ball is visible after the winning point, want it hidden")
	}

	frame := lines(tui.Render(finished, 80, 28))
	assertNoBallInCourt(t, frame)
	for _, column := range []int{frameGameOverLeft, frameGameOverRight} {
		if cell := frame[frameGameOverTop][column]; cell != '+' {
			t.Errorf("overlay corner at column %d = %q, want +", column, cell)
		}
	}
	left, right := finished.PaddleTops()
	assertPaddlesDrawn(t, frame, left, right)
	border := "+" + strings.Repeat("-", core.CourtWidth) + "+"
	if frame[frameCourtRow][frameCourtLeft:frameCourtLeft+len(border)] != border {
		t.Errorf("court top border = %q, want it visible above the overlay", frame[frameCourtRow])
	}
	if score := frame[frameScoreRow]; !strings.Contains(score, "PLAYER 1") ||
		!strings.Contains(score, "PLAYER 2") || !strings.Contains(score, "7") {
		t.Errorf("score row = %q, want the final score visible above the overlay", score)
	}
	if feedback := frame[frameFeedback]; strings.TrimSpace(feedback) != "" {
		t.Errorf("feedback row = %q, want it empty after the winning point", feedback)
	}
}

func TestGameOverMenuSelectionMovesWithoutMovingTheCourt(t *testing.T) {
	t.Parallel()

	finished := playedToGameOver(t)
	selected := finished
	for selected.GameOverFocus() != core.GameOverRowChangeSetup {
		selected, _ = core.Update(selected, core.EventNavigate{Direction: core.DirectionDown})
	}

	before, after := lines(tui.Render(finished, 80, 28)), lines(tui.Render(selected, 80, 28))
	overlay := strings.Join(after[frameGameOverTop:frameGameOverBottom+1], "\n")
	if !strings.Contains(overlay, "> Change setup") {
		t.Errorf("overlay = %q, want Change setup selected", overlay)
	}
	if strings.Contains(overlay, "> Rematch") {
		t.Error("the overlay still marks Rematch as selected")
	}
	for _, row := range []int{frameScoreRow, frameContextRow, frameCourtRow, frameCourtRow + core.CourtHeight + 1} {
		if before[row] != after[row] {
			t.Errorf("row %d moved from %q to %q when the selection changed", row, before[row], after[row])
		}
	}
}

func TestGameOverCentersFixedLayoutWithOddSurplus(t *testing.T) {
	t.Parallel()

	finished := playedToGameOver(t)
	baseRow, baseColumn := locate(lines(tui.Render(finished, 80, 28)), "PLAYER 1 WINS")
	if baseRow < 0 {
		t.Fatal("the overlay does not name the winner")
	}
	for _, test := range []struct{ width, height int }{
		{width: 81, height: 29},
		{width: 83, height: 31},
		{width: 101, height: 35},
	} {
		view := tui.Render(finished, test.width, test.height)
		if lipgloss.Width(view) != test.width || lipgloss.Height(view) != test.height {
			t.Errorf("%dx%d view is %dx%d", test.width, test.height, lipgloss.Width(view), lipgloss.Height(view))
		}
		row, column := locate(lines(view), "PLAYER 1 WINS")
		rowShift, columnShift := (test.height-28)/2, (test.width-80)/2
		if row != baseRow+rowShift || column != baseColumn+columnShift {
			t.Errorf("%dx%d: winner at row %d, column %d; want row %d, column %d",
				test.width, test.height, row, column, baseRow+rowShift, baseColumn+columnShift)
		}
	}
}

func TestGameOverKeysDriveTheMenu(t *testing.T) {
	t.Parallel()

	model, application := shellGameOver(t)

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if cmd != nil {
		t.Errorf("menu navigation produced a command, want none")
	}
	application, _ = core.Update(application, core.EventNavigate{Direction: core.DirectionUp})
	if updated.View().Content != tui.Render(application, 80, 28) {
		t.Error("Up does not reach the Game over menu")
	}

	inert := []tea.KeyPressMsg{
		{Code: 'w', Text: "w"},
		{Code: 's', Text: "s"},
		{Code: 'p', Text: "p"},
		{Code: 'P', Text: "P", Mod: tea.ModShift},
		{Code: tea.KeyEscape},
	}
	for _, key := range inert {
		same, keyCmd := updated.Update(key)
		if keyCmd != nil || same.View().Content != updated.View().Content {
			t.Errorf("key %q changed the game-over view", key.String())
		}
	}

	// Down wraps back to Rematch, where Enter starts the next match through a new timer.
	selected, _ := updated.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	application, _ = core.Update(application, core.EventNavigate{Direction: core.DirectionDown})
	if selected.View().Content != tui.Render(application, 80, 28) {
		t.Fatal("Down does not reach the Game over menu")
	}
	rematch, cmd := selected.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Error("Rematch scheduled no timer")
	}
	application, _ = core.Update(application, core.EventPrimary{})
	if rematch.View().Content != tui.Render(application, 80, 28) {
		t.Error("Rematch does not start the next match")
	}
	if strings.Contains(rematch.View().Content, "WINS") {
		t.Error("the next match still shows the finished result")
	}
}

func TestGameOverRecoveryDoesNotRearmAStoppedTimerChain(t *testing.T) {
	t.Parallel()

	model, application := shellGameOver(t)

	stale, cmd := model.Update(tui.TimerMsg{Token: 1, Fired: time.Now().Add(tickPeriod)})
	if cmd != nil {
		t.Error("a completion after the winning point scheduled another timer")
	}
	if stale.View().Content != tui.Render(application, 80, 28) {
		t.Error("a completion after the winning point changed the view")
	}

	small, cmd := stale.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if cmd != nil {
		t.Error("shrinking the terminal scheduled a timer")
	}
	recovered, cmd := small.Update(tea.WindowSizeMsg{Width: 80, Height: 28})
	if cmd != nil {
		t.Error("recovery re-armed the stopped timer chain of a finished match")
	}
	if recovered.View().Content != tui.Render(application, 80, 28) {
		t.Error("recovery changed the game-over view")
	}
}

// shellGameOver drives the shell and its core replay with host timer completions until the left player wins.
func shellGameOver(t *testing.T) (tea.Model, core.Model) {
	t.Helper()

	model, cmd := startMatchInShell(t)
	if cmd == nil {
		t.Fatal("starting a match scheduled no timer")
	}
	application := startedMatchInShell(t)

	// The countdown's first request is discovered from the shell's response; every honored completion then asks for
	// the next request number.
	token := core.TimerToken(1)
	tape := newTimerTape()
	model, next, message := probeTimer(t, model, token, tape.next(tickPeriod))
	if next == nil {
		t.Fatal("the first completion requested no next timer")
	}
	application, _ = core.Update(application, core.EventElapsed{Elapsed: core.ElapsedLimit, Token: token})
	issue := message.Issue + 1

	for range 5000 {
		updated, next := model.Update(tui.TimerMsg{Token: token, Issue: issue, Fired: tape.next(tickPeriod)})
		model = updated
		var effect core.Effect
		application, effect = core.Update(application, core.EventElapsed{
			Elapsed: core.ElapsedLimit,
			Token:   token,
		})

		if application.MatchPhase() == core.MatchPhaseGameOver {
			if next != nil {
				t.Error("the winning point scheduled another timer")
			}

			return updated, application
		}
		if next == nil {
			t.Fatalf("timer issue %d requested no next timer before Game over", issue)
		}
		if effect.Kind() != core.EffectScheduleTimer {
			t.Fatalf("a live match answered a completion with %v", effect.Kind())
		}

		token, issue = effect.Timer().Token, issue+1
	}
	t.Fatal("the match never ended")

	return model, application
}

// playedToGameOver plays a Local two-player match with one-step elapsed events until the left player wins.
func playedToGameOver(t *testing.T) core.Model {
	t.Helper()

	model, schedule := startMatch(t, core.ModeLocalTwoPlayer)
	token := schedule.Timer().Token
	for range 4 * int(core.WinningScore11) * int(core.CountdownSteps) {
		if model.MatchPhase() == core.MatchPhaseGameOver {
			return model
		}
		var effect core.Effect
		model, effect = core.Update(model, core.EventElapsed{Elapsed: core.ElapsedLimit, Token: token})
		if effect.Kind() == core.EffectScheduleTimer {
			token = effect.Timer().Token
		}
	}
	t.Fatal("the match never ended")

	return model
}

// assertNoBallInCourt checks that no court cell outside the overlay draws the ball.
func assertNoBallInCourt(t *testing.T, frame []string) {
	t.Helper()

	for y := range core.CourtHeight {
		for x := range core.CourtWidth {
			if x >= frameGameOverLeft-frameCourtLeft-1 && x < frameGameOverRight-frameCourtLeft {
				continue // the overlay text occupies the middle of the court
			}
			if cell := frame[frameCourtRow+1+y][frameCourtLeft+1+x]; cell == 'o' {
				t.Fatalf("court cell (%d, %d) draws a ball after the winning point", x, y)
			}
		}
	}
}

// assertPaddlesDrawn checks both paddles are drawn at the given tops.
func assertPaddlesDrawn(t *testing.T, frame []string, left, right uint8) {
	t.Helper()

	for offset := range core.PaddleHeight {
		if cell := frame[frameCourtRow+1+int(left)+offset][frameCourtLeft+1+core.PaddleLeftColumn]; cell != '#' {
			t.Errorf("left paddle row %d = %q, want #", offset, cell)
		}
		if cell := frame[frameCourtRow+1+int(right)+offset][frameCourtLeft+1+core.PaddleRightColumn]; cell != '#' {
			t.Errorf("right paddle row %d = %q, want #", offset, cell)
		}
	}
}

// rowWithin returns the first row in rows containing text, or -1.
func rowWithin(rows []string, text string) (int, int) {
	for row, line := range rows {
		if column := strings.Index(line, text); column >= 0 {
			return row, column
		}
	}

	return -1, -1
}
