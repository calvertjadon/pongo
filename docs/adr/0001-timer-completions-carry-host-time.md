# Timer completions carry their own host time

The Shell never reads a clock. Its timer command stamps each completion with when the timer was armed and when it fired, and the Shell measures a timing sequence's elapsed time from those stamps alone: a new sequence or a recovery measures fired minus armed, and a continuing sequence measures from the previous completion's fire time. We chose this over Bubble Tea's `tea.Tick` with `time.Now()` because a clock read inside the Shell made the timing baseline untestable: the test meant to prove that recovery from an undersized terminal never catches up passed even with the fresh baseline deleted.

## Considered Options

- **`tea.Tick` plus `time.Now()` in the Shell.** The obvious Bubble Tea path. Rejected: tests cannot control the baseline, so they guess request numbers and lean on wall-clock slack.
- **Inject a clock into the Shell model.** Testable, but keeps the clock inside the model and widens its constructor for every caller; completions already cross the host seam, so they can carry the time.

## Consequences

The timer starts when Bubble Tea runs the command rather than inside `Update`, microseconds later. Shell tests build completions with synthetic `Armed` and `Fired` times and never depend on the host clock.
