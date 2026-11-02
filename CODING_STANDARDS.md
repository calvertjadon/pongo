# Pongo coding standards

These rules apply to every Pongo build ticket.

## Architecture

- Keep all player-visible state and transitions in a deterministic core.
- The core owns semantic events, fixed-step timing, seeded variation, and explicit effect requests.
- The core must not import Bubble Tea, Lip Gloss, terminal, clock, operating-system, or ambient-randomness APIs.
- The Bubble Tea shell translates host messages, measures elapsed time, performs effects, and owns terminal lifecycle.
- Rendering must be pure. It reads semantic state and current terminal dimensions and cannot mutate the core.
- Keep dependencies directed inward. The shell and renderer can depend on the core. The core cannot depend on them.
- Do not duplicate game or navigation state outside the core.

## Go

- Keep exports narrow. Prefer concrete types.
- Define an interface at its consuming boundary only when a real alternate implementation exists.
- Do not use mutable package globals or `init` side effects.
- Do not panic for user input, host errors, or ordinary state transitions.
- Handle returned errors. Add useful context at host and I/O boundaries, preserve causes with `%w`, and inspect wrapped errors with `errors.Is` or `errors.As`.
- Add a dependency only when the build ticket names the need and explains why the standard library or an approved Charm dependency cannot meet it.
- Keep functions at or below 80 lines, 40 statements, and a cognitive complexity of 20.
- A `nolint` directive must name one linter, give a reason, and use the smallest possible scope. Reviewer approval is required. A directive cannot suppress an architecture, determinism, specification, or test failure.

## Formatting and static analysis

Use the golangci-lint version pinned in `tools/go.mod` with `.golangci.yml`. The configuration enables `goimports` and `golines` with a 120-column target. It enables Staticcheck `SA*` checks, `errcheck`, `errorlint`, `exhaustive`, `gochecksumtype`, `gocognit`, `gocritic`, `godoclint`, `funlen`, `ineffassign`, `nilerr`, `nolintlint`, `predeclared`, `revive`, `unused`, and `whitespace`.

Run `./scripts/gate.sh` from the repository root. It runs the format check, the tidy checks, `go vet`, the lint pass, race tests with coverage, and the build, and exits 0 only when every step passes and the formatter prints no diff.

## Deterministic tests

- Use the standard `testing` package. Do not add assertion, mock, or snapshot libraries.
- Test the core through explicit starting state, semantic events, elapsed durations, and seeds.
- Assert observable semantic state and effect requests. Do not assert private field copies, terminal escape strings, or incidental wording.
- Core tests must not start Bubble Tea, read a clock, use ambient randomness, sleep, or depend on timing tolerances.
- Replaying identical inputs must produce identical state and effects.
- Cover boundaries, invariants, transitions, invalid and stale events, effect correlation, collision ordering, reset rules, and ticket-specific failure cases when they become reachable.
- Do not gate on a coverage percentage. Record coverage output and judge whether tests catch the plausible failures named by the ticket.

## Interactive verification

After the automated gate, build and run the game at the candidate commit in a supported Linux or macOS TTY at 80 by 28 cells or larger.

Always verify startup, every changed acceptance criterion, the neighboring flow, return to the title when available, `Q`, process exit, and terminal restoration. Record the commit SHA, operating system, terminal, terminal dimensions, steps, and observations on the build ticket.

## Review and approval

Resolve `main`, then list `git log <merge-base>..HEAD --oneline` and review `git diff <merge-base>...HEAD`.

Run Standards and Spec as separate reviews. Standards receives this file, repository agent instructions, the fixed smell baseline, the commit list, and the exact diff command. Spec receives the full build ticket and comments, linked decisions, the commit list, and the same diff.

A review blocks only on a problem introduced by the branch or a pre-existing problem that the branch materially worsens. Classify a concrete breach as Blocking. Classify a supported but uncertain concern as Question. Each Question must become Blocking or the reviewer must withdraw it with a reason.

When the branch deletes or rewrites tests, name, for each removed or weakened assertion, the test at the candidate commit that fails on the same breakage. Statement coverage does not show this; a covered line can lose its only assertion. Classify a removed assertion that no remaining test replaces as Blocking. Classify one whose replacement you cannot confirm from the diff and the tests as Question.

The learner fixes every Blocking finding. The agent does not patch the learner's branch. After a fix, rerun the complete automated gate and both reviews from `main` to the new candidate commit. Repeat interactive steps affected by the fix, or the complete checklist if behavior changed.

Approval binds to one commit SHA. Fast-forward `main` to that SHA only when:

1. Every automated command passed at that SHA.
2. The interactive record covers the required checklist at that SHA.
3. Standards reports Pass.
4. Spec reports Pass.
5. No Blocking finding or unresolved Question remains.
6. The candidate still descends directly from current `main`.

Any code or configuration change invalidates the approval evidence. If `main` moves, rebase the branch and repeat the complete gate. Integrate with a fast-forward only.