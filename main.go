// Package main provides the Pongo executable.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/calvertjadon/pongo/internal/tui"
)

const usage = `Usage: pongo [--help | --version]

Pongo is classic Pong in the terminal. Run it without arguments in an
interactive terminal of at least 80 by 28 cells.

Options:
  -h, --help     Print this help and exit.
  --version      Print the version and exit.
`

// Process exit statuses.
const (
	exitSuccess   = 0
	exitFailure   = 1
	exitUsage     = 2
	exitHangup    = 128 + 1
	exitInterrupt = 128 + 2
	exitTerminate = 128 + 15
)

// developmentVersion identifies a build without module version information, such as one built with -buildvcs=false.
const developmentVersion = "v0.0.0-devel"

type command uint8

const (
	commandPlay command = iota
	commandHelp
	commandVersion
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(arguments []string) int {
	selected, err := parseCommand(arguments)
	if err != nil {
		report("%v\nRun 'pongo --help' for usage.", err)
		return exitUsage
	}

	switch selected {
	case commandHelp:
		return write(os.Stdout, usage)
	case commandVersion:
		return write(os.Stdout, version()+"\n")
	case commandPlay:
		return play()
	}

	return exitFailure
}

// parseCommand accepts no arguments, or exactly one of the documented options.
func parseCommand(arguments []string) (command, error) {
	switch {
	case len(arguments) == 0:
		return commandPlay, nil
	case len(arguments) > 1:
		return commandPlay, fmt.Errorf("unsupported arguments %q", arguments)
	}

	switch arguments[0] {
	case "-h", "--help":
		return commandHelp, nil
	case "--version":
		return commandVersion, nil
	default:
		return commandPlay, fmt.Errorf("unsupported argument %q", arguments[0])
	}
}

// version reports the main module version the Go toolchain embedded in this executable. Go stamps the exact tag for
// a clean tagged checkout or `go install module@version`, and a pseudo-version for other version-controlled builds.
func version() string {
	info, available := debug.ReadBuildInfo()
	if !available || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return developmentVersion
	}

	return info.Main.Version
}

// play runs the game in the terminal until it quits, fails, or a catchable host signal ends it. Bubble Tea restores
// the cursor, input modes, and alternate screen on every one of those paths, including a recovered panic.
func play() int {
	if err := requireTerminal(); err != nil {
		report("%v", err)
		return exitFailure
	}
	if err := disableFrameworkLogs(); err != nil {
		report("%v", err)
		return exitFailure
	}

	// Pongo handles signals itself because Bubble Tea treats SIGTERM as a normal quit and ignores SIGHUP.
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		select {
		case received := <-signals:
			cancel(signalError{signal: received})
		case <-ctx.Done():
		}
	}()

	program := tea.NewProgram(
		tui.NewModel(),
		tea.WithContext(ctx),
		tea.WithoutSignalHandler(),
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	)
	_, err := program.Run()

	return exitStatus(err, context.Cause(ctx))
}

// exitStatus maps how the game ended to the documented process exit status.
func exitStatus(runErr, cause error) int {
	var caught signalError
	switch {
	case errors.As(cause, &caught):
		return caught.status()
	case errors.Is(runErr, tea.ErrInterrupted):
		return exitInterrupt
	case runErr != nil:
		report("%v", runErr)
		return exitFailure
	default:
		return exitSuccess
	}
}

// signalError records the host signal that ended the game.
type signalError struct {
	signal os.Signal
}

func (err signalError) Error() string {
	return "received " + err.signal.String()
}

// status follows the shell convention of 128 plus the signal number.
func (err signalError) status() int {
	switch err.signal {
	case syscall.SIGHUP:
		return exitHangup
	case syscall.SIGINT:
		return exitInterrupt
	case syscall.SIGTERM:
		return exitTerminate
	default:
		return exitFailure
	}
}

// requireTerminal ensures the game reads keys from and draws to an interactive terminal.
func requireTerminal() error {
	if !term.IsTerminal(os.Stdin.Fd()) {
		return errors.New("an interactive terminal is required: standard input is not a terminal")
	}
	if !term.IsTerminal(os.Stdout.Fd()) {
		return errors.New("an interactive terminal is required: standard output is not a terminal")
	}

	return nil
}

// disableFrameworkLogs keeps Bubble Tea from writing trace or panic log files, since Pongo creates no files.
func disableFrameworkLogs() error {
	for _, variable := range []string{"TEA_TRACE", "TEA_DEBUG"} {
		if err := os.Unsetenv(variable); err != nil {
			return fmt.Errorf("unset %s: %w", variable, err)
		}
	}

	return nil
}

// write prints text to standard output and reports a failed write as a runtime error.
func write(output io.Writer, text string) int {
	if _, err := io.WriteString(output, text); err != nil {
		report("write output: %v", err)
		return exitFailure
	}

	return exitSuccess
}

// report prints a contextual error to standard error. A failed write leaves only the exit status to report it.
func report(format string, arguments ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "pongo: "+format+"\n", arguments...)
}
