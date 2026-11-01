# Charm toolkit capabilities for Pongo

Research snapshot: 2026-09-04. This note answers [Verify Bubble Tea and Charm toolkit capabilities](https://github.com/calvertjadon/pongo/issues/2) for Pongo's Linux and macOS terminal target. It covers the current stable v2 modules and the terminal behavior that later game, layout, resize, release, and verification decisions must account for.

## Stable modules and versions

GitHub's first-party `latest` release metadata reported non-draft, non-prerelease releases on the snapshot date:

| Module | Recommended version | Primary evidence |
| --- | --- | --- |
| `charm.land/bubbletea/v2` | `v2.0.9` | [latest release metadata](https://api.github.com/repos/charmbracelet/bubbletea/releases/latest), [v2.0.9 release](https://github.com/charmbracelet/bubbletea/releases/tag/v2.0.9), [tagged `go.mod`](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/go.mod) |
| `charm.land/lipgloss/v2` | `v2.0.6` | [latest release metadata](https://api.github.com/repos/charmbracelet/lipgloss/releases/latest), [v2.0.6 release](https://github.com/charmbracelet/lipgloss/releases/tag/v2.0.6), [tagged `go.mod`](https://github.com/charmbracelet/lipgloss/blob/v2.0.6/go.mod) |

Both recommended modules declare `go 1.25.0`. Their v2 module paths use the `charm.land` namespace. The Bubble Tea v2 upgrade guide also records the import-path change from the old GitHub paths. Use these exact imports in later build tickets:

```go
import tea "charm.land/bubbletea/v2"
import "charm.land/lipgloss/v2"
```

Sources: [Bubble Tea v2 `go.mod`](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/go.mod), [Lip Gloss v2 `go.mod`](https://github.com/charmbracelet/lipgloss/blob/v2.0.6/go.mod), [Bubble Tea v2 upgrade guide](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/UPGRADE_GUIDE_V2.md#import-paths).

## Bubble Tea's Elm Architecture boundary

Bubble Tea v2 supplies the `Model` interface with `Init`, `Update`, and `View`. `Msg` is an event, and `Cmd` produces an event after I/O or other work. `View` returns a `tea.View`, which carries content and terminal features. Bubble Tea renders the view after updates, so Pongo can keep game rules in a deterministic model and use the Bubble Tea model as a thin event shell. Sources: [v2 README tutorial](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/README.md#the-model), [core model and view types](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L31-L121), [v2 upgrade guide](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/UPGRADE_GUIDE_V2.md#view-returns-a-teaview-now).

`tea.NewProgram` runs the event loops and returns the final model and an error. The default panic handler restores terminal state. `WithoutCatchPanics` disables that protection and can leave the terminal unusable after a panic. Sources: [program lifecycle](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L999-L1182), [program options](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/options.go#L69-L82).

## Timing and commands

Bubble Tea exposes two one-shot timer commands:

- `tea.Tick(duration, fn)` starts a timer for the requested duration and sends one message containing the callback's timestamp. It does not repeat by itself. Return another `Tick` command from `Update` after handling the tick to schedule the next event.
- `tea.Every(duration, fn)` sends one message aligned to the system clock. It can also be returned again for a repeating schedule, but the first delay may be shorter than the requested duration because it waits for the next clock boundary.

`tea.Batch` runs commands concurrently with no ordering guarantee. `tea.Sequence` runs commands in order. Sources: [command API and timer semantics](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/commands.go).

For Pongo, use `tea.Tick` for a game-update command and schedule the next tick only while the match is active. Do not use the renderer's frame rate as the simulation clock. `tea.WithFPS` sets the renderer's maximum rate, which defaults to 60 FPS and caps at 120 FPS. It does not make `Update` run at that rate and does not replace a game command. Source: [`WithFPS`](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/options.go#L129-L139), [renderer limits](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/renderer.go#L9-L16).

A command runs in its own goroutine, and Bubble Tea cannot cancel a command that is already running. A pause or mode change must therefore stop returning future ticks and ignore any late tick message that a command already produced. Source: [command handling and shutdown context](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L747-L790).

## Keyboard input and local two-player controls

The v2 input API sends `tea.KeyPressMsg` for presses and `tea.KeyReleaseMsg` for releases. A `tea.Key` carries printable `Text`, a special-key or rune `Code`, modifier flags, and optional `ShiftedCode`, `BaseCode`, and `IsRepeat` fields. `String()` gives a readable key name such as `"up"` or `"ctrl+c"`; `Keystroke()` retains modifier information. Sources: [key message types](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/key.go#L194-L260), [key fields and matching](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/key.go#L262-L377).

Bubble Tea does not expose a simultaneous-key snapshot, a maximum number of held keys, or a combined `KeysDown` message. Its event model represents one key event with modifier flags. A local two-player match must treat input as an event stream. The model can move a paddle on each press, or track presses and releases itself. The latter needs `KeyReleaseMsg` support, but Bubble Tea only receives repeat and release events when the terminal supports the requested keyboard enhancement. Sources: [key event representation](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/key.go#L262-L377), [keyboard-enhancement request fields](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L178-L229), [enhancement capability checks](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/keyboard.go).

For predictable controls across terminals, later decisions should define a press-based fallback. If the game wants held-key state, request `ReportEventTypes`, handle release messages when available, and retain a fallback that does not require releases. Do not assume that two held keys arrive in one message or that every Linux or macOS terminal reports releases. Source: [keyboard enhancements in the v2 view](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L163-L229).

Pongo does not need mouse input, text editing, forms, or generic list navigation. It can match its small fixed control set directly with `KeyPressMsg`. Bubbles v2.2.1 does provide reusable key bindings, help, timer, list, and other components, but its module also requires Bubble Tea, Lip Gloss, and other packages. Add it only if a later approved interaction decision chooses a component that earns that dependency. Sources: [Bubbles v2.2.1 release metadata](https://api.github.com/repos/charmbracelet/bubbles/releases/latest), [Bubbles component scope](https://github.com/charmbracelet/bubbles/blob/v2.2.1/README.md), [Bubbles v2.2.1 module requirements](https://github.com/charmbracelet/bubbles/blob/v2.2.1/go.mod), [Bubbles key binding package](https://github.com/charmbracelet/bubbles/blob/v2.2.1/key/key.go).

## Alternate screen and cleanup

Bubble Tea v2 moved alternate-screen control into the declarative view. Set `view.AltScreen = true` in every full-screen view. Bubble Tea enters or exits the alternate buffer when that field changes, and the view documentation states that it exits the alternate screen automatically when the program quits. Do not use the removed v1 `WithAltScreen` option or imperative enter/exit commands. Sources: [view's `AltScreen` field](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L141-L161), [v2 migration guidance](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/UPGRADE_GUIDE_V2.md#the-big-idea-declarative-views), [renderer screen transitions](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/cursed_renderer.go#L353-L363).

A normal `tea.Quit` path renders the final model, then shutdown restores the terminal. `Program.Kill` skips the final render but still restores terminal state. The renderer cleanup exits the alternate screen, restores the cursor, and resets terminal modes such as keyboard enhancements, bracketed paste, focus reporting, and mouse tracking. Keep Bubble Tea's default panic recovery enabled. Sources: [quit and kill lifecycle](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L1148-L1274), [renderer cleanup](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/cursed_renderer.go#L176-L224).

`ReleaseTerminal` and `RestoreTerminal` exist for handing the terminal to another process or suspending and resuming a program. They are not needed for ordinary menu or match transitions. Source: [terminal release and restore](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L1326-L1377).

## Resize handling and layout

Bubble Tea sends `tea.WindowSizeMsg` once at startup and again when the terminal changes size. On Unix, the resize listener receives `SIGWINCH`, queries the output TTY, and sends the new width and height in terminal cells. `RequestWindowSize` is available, but the normal startup and resize messages make an explicit request unnecessary. Sources: [window-size message](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/screen.go#L5-L13), [automatic startup and resize delivery](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L1040-L1062), [Unix `SIGWINCH` listener](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/signals_unix.go), [manual request command](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/commands.go#L101-L106).

Resize delivery depends on a TTY output. Bubble Tea does not start the resize listener when output is not a terminal. A later layout decision must store the latest cell dimensions in model state, derive the arena and HUD from those dimensions, and define behavior for a terminal that is too small. The game must not keep stale coordinates after a resize. Sources: [resize handler TTY check](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L687-L700), [resize query](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tty.go#L88-L112).

## Rendering, width, and color

`tea.View.Content` is the styled string Bubble Tea renders. Bubble Tea's renderer is cell-based and handles redraws, cursor state, alternate-screen state, terminal modes, and terminal color profile integration. Sources: [Bubble Tea renderer description](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/README.md#bubble-tea), [view content](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L54-L77), [cursed renderer frame path](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/cursed_renderer.go#L285-L343).

Lip Gloss provides immutable style values and `Style.Render`, with borders, padding, margins, alignment, width, height, and wrapping. Its `Width`, `Height`, and `Size` functions measure terminal cells, ignore ANSI sequences, and account for wide characters and emoji. `Width` returns the widest line, while `Height` counts newline-separated lines. Use `lipgloss.Width` rather than `len` or rune count when calculating HUD or layout widths. Sources: [Lip Gloss style API](https://github.com/charmbracelet/lipgloss/blob/v2.0.6/style.go#L112-L262), [cell-size functions](https://github.com/charmbracelet/lipgloss/blob/v2.0.6/size.go), [joining utilities](https://github.com/charmbracelet/lipgloss/blob/v2.0.6/join.go).

Lip Gloss v2.0.6 also uses terminal Unicode grapheme support when possible, with release notes calling out improved Devanagari rendering. Pongo's playfield should still use simple one-cell glyphs for the paddles, ball, borders, and score separators. Text that later becomes localized must use cell-width measurement rather than byte or rune count. Source: [Lip Gloss v2.0.6 release notes](https://github.com/charmbracelet/lipgloss/releases/tag/v2.0.6).

Bubble Tea detects the terminal color profile from environment variables and terminfo, sends a `ColorProfileMsg` at startup, and lets a caller override the profile with `WithColorProfile`. Lip Gloss supports ANSI 16-color, ANSI 256-color, truecolor, and no-color output. In a Bubble Tea program, Lip Gloss styles are downsampled to the detected profile automatically. Sources: [Bubble Tea color profile message](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/profile.go), [profile option](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/options.go#L112-L127), [Lip Gloss color profiles and downsampling](https://github.com/charmbracelet/lipgloss/blob/v2.0.6/README.md#automatically-downsampling-colors), [Lip Gloss color implementation](https://github.com/charmbracelet/lipgloss/blob/v2.0.6/color.go#L178-L280).

Pongo should use color to distinguish information, not to carry game state by itself. A no-color or low-color terminal must remain playable. `tea.RequestCapability("RGB")` and `tea.RequestCapability("Tc")` can query truecolor terminfo capabilities, but Bubble Tea warns that some terminals, including Apple's Terminal.app, can answer incorrectly and break output. Do not add capability queries unless a later decision shows a real need. Sources: [capability request and Apple Terminal warning](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/termcap.go), [Lip Gloss `NoColor`](https://github.com/charmbracelet/lipgloss/blob/v2.0.6/color.go#L20-L47).

## Linux and macOS constraints

The Bubble Tea Unix implementation has build tags for both `linux` and `darwin`. It enters raw mode only when the input is a terminal, identifies terminal output separately, and supports Unix process suspension. Its Unix resize implementation uses `SIGWINCH`. These facts support the requested Linux and macOS target, but they do not make a non-interactive pipe behave like a terminal. Sources: [Unix TTY build tags and raw-mode checks](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tty_unix.go), [Unix termios support](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/termios_unix.go), [Unix resize signal handling](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/signals_unix.go).

When standard input is not a TTY, Bubble Tea attempts to open the running TTY before starting interactive input. A release and verification procedure must run the game in a real interactive terminal or an explicitly provisioned pseudo-terminal. Redirected output also removes automatic resize notifications. Sources: [input selection in `Program.Run`](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L1010-L1029), [TTY opening and input reader](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tty.go#L43-L86).

## Dependency decision

Use only these direct dependencies for the initial Pongo shell:

```text
charm.land/bubbletea/v2 v2.0.9
charm.land/lipgloss/v2 v2.0.6
```

Do not add direct dependencies on Bubble Tea's transitive `ultraviolet`, `colorprofile`, or `charmbracelet/x` packages. Bubble Tea and Lip Gloss already own those integrations. Do not add Bubbles, Harmonica, Huh, or Glamour now:

- Bubbles is useful when Pongo needs a reusable list, text input, help view, timer widget, or other component. Pongo's fixed controls, title screen, mode selection, and match HUD do not require one yet. If a later decision chooses a Bubbles component, use `charm.land/bubbles/v2 v2.2.1` and review its dependency graph first. Sources: [Bubbles release metadata](https://api.github.com/repos/charmbracelet/bubbles/releases/latest), [Bubbles module](https://github.com/charmbracelet/bubbles/blob/v2.2.1/go.mod), [Bubbles components](https://github.com/charmbracelet/bubbles/blob/v2.2.1/README.md).
- Harmonica provides spring and projectile animation simulators, not a fixed-step Pong rules engine. It has no need in the initial dependency set. Source: [Harmonica v0.2.0 release](https://github.com/charmbracelet/harmonica/releases/tag/v0.2.0), [Harmonica API](https://github.com/charmbracelet/harmonica/blob/v0.2.0/harmonica.go).

The Go toolchain for the two recommended v2 modules must satisfy Go 1.25.0. Later release work must publish Linux and macOS binaries or source instructions that make this requirement explicit. Sources: [Bubble Tea module metadata](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/go.mod), [Lip Gloss module metadata](https://github.com/charmbracelet/lipgloss/blob/v2.0.6/go.mod).

## Facts for later decisions

- **Architecture:** Keep game rules and collision math independent of Bubble Tea. `Update` consumes timer and key events, changes model state, and returns the next command. `View` projects that state into `tea.View`.
- **Timing:** Use one recurring `tea.Tick` schedule for active play. Treat `WithFPS` as a renderer limit, not simulation time. Stop scheduling on pause, scoring, and game over, and ignore late ticks.
- **Controls:** Model controls as individual key events. Do not depend on a simultaneous-key message or universal key-release support. Define a press-based fallback before promising held-key behavior.
- **Alternate screen and release:** Set `AltScreen` declaratively. Let Bubble Tea quit and panic cleanup restore the cursor, screen buffer, keyboard modes, paste mode, and other terminal state. Verify both normal quit and forced termination paths in a TTY.
- **Layout and resize:** Store `WindowSizeMsg` dimensions in cells. Recompute all playfield and HUD geometry after every resize. Define a too-small-terminal view before implementation.
- **Width:** Use Lip Gloss cell measurements for styled or localized text. Avoid `len`, rune count, and assumptions that every displayed glyph occupies one cell.
- **Color:** Keep the game usable with ANSI 16-color, ANSI 256-color, truecolor, and no-color profiles. Avoid truecolor capability queries unless needed because of the Apple Terminal.app warning.
- **Platform and verification:** Test on Linux and macOS in real TTYs or controlled pseudo-terminals with at least one low-color or no-color environment. Exercise resize, normal quit, `Ctrl+C`, alternate-screen restoration, and the agreed control fallback. The source supports these Unix targets; terminal emulator behavior still belongs in the acceptance procedure.
