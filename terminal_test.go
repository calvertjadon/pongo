//go:build linux || darwin

package main_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// exitTimeout bounds every wait on the executable so that a game started by mistake fails the test instead of hanging.
const exitTimeout = 10 * time.Second

func TestGameRefusesToStartWithoutTerminal(t *testing.T) {
	t.Parallel()

	pongo := buildPongo(t)
	t.Run("no terminal", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		command := exec.Command(pongo)
		command.Stdout = &stdout
		command.Stderr = &stderr
		// A new session has no controlling terminal, so nothing can fall back to /dev/tty.
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		assertStartupRejected(t, command, &stderr)
	})
	t.Run("output not a terminal", func(t *testing.T) {
		t.Parallel()

		terminal := newPseudoTerminal(t, 80, 28)
		before := terminal.state(t)
		var stdout, stderr bytes.Buffer
		command := exec.Command(pongo)
		command.Stdin = terminal.terminal
		command.Stdout = &stdout
		command.Stderr = &stderr

		assertStartupRejected(t, command, &stderr)
		if after := terminal.state(t); after != before {
			t.Fatalf("terminal state changed:\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})
	t.Run("input not a terminal", func(t *testing.T) {
		t.Parallel()

		terminal := newPseudoTerminal(t, 80, 28)
		var stderr bytes.Buffer
		command := exec.Command(pongo)
		command.Stdout = terminal.terminal
		command.Stderr = &stderr
		// The pseudo-terminal becomes the controlling terminal, so /dev/tty would reach it.
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 1}

		assertStartupRejected(t, command, &stderr)
	})
}

func TestSessionExitsRestoreTerminal(t *testing.T) {
	t.Parallel()

	pongo := buildPongo(t)
	for _, exit := range sessionExits() {
		t.Run(exit.name, func(t *testing.T) {
			t.Parallel()

			assertSessionExit(t, pongo, exit)
		})
	}
}

// sessionExit describes one way to end a game session and the documented exit status it reports.
type sessionExit struct {
	name   string
	status int
	end    func(*pseudoTerminal, *os.Process) error
}

// sessionExits lists every catchable way to leave a running game.
func sessionExits() []sessionExit {
	return []sessionExit{
		{name: "Q on the title", status: 0, end: typing("q")},
		{name: "Ctrl+C", status: 130, end: typing("\x03")},
		{name: "SIGINT", status: 130, end: signaling(syscall.SIGINT)},
		{name: "SIGHUP", status: 129, end: signaling(syscall.SIGHUP)},
		{name: "SIGTERM", status: 143, end: signaling(syscall.SIGTERM)},
	}
}

// assertSessionExit runs a game on a fresh pseudo-terminal, ends it the named way, and checks the status and terminal.
func assertSessionExit(t *testing.T, executable string, exit sessionExit) {
	t.Helper()

	terminal := newPseudoTerminal(t, 80, 28)
	before := terminal.state(t)
	command := terminal.startGame(t, executable, t.TempDir())
	if err := exit.end(terminal, command.Process); err != nil {
		t.Fatalf("end session: %v", err)
	}

	if status := waitForExit(t, command); status != exit.status {
		t.Fatalf("pongo exited %d after %s, want %d", status, exit.name, exit.status)
	}
	if after := terminal.state(t); after != before {
		t.Fatalf("terminal not restored after %s:\nbefore:\n%s\nafter:\n%s", exit.name, before, after)
	}
}

func TestSessionCreatesNoFiles(t *testing.T) {
	t.Parallel()

	pongo := buildPongo(t)
	directories := map[string]string{
		"working directory": t.TempDir(),
		"HOME":              t.TempDir(),
		"XDG_CONFIG_HOME":   t.TempDir(),
		"XDG_CACHE_HOME":    t.TempDir(),
		"XDG_STATE_HOME":    t.TempDir(),
		"XDG_DATA_HOME":     t.TempDir(),
		"TMPDIR":            t.TempDir(),
	}
	overrides := []string{
		// Bubble Tea writes a trace log when TEA_TRACE names a file and a panic log when TEA_DEBUG is set.
		"TEA_TRACE=" + filepath.Join(directories["working directory"], "trace.log"),
		"TEA_DEBUG=true",
	}
	for name, directory := range directories {
		if name != "working directory" {
			overrides = append(overrides, name+"="+directory)
		}
	}

	terminal := newPseudoTerminal(t, 80, 28)
	command := terminal.startGame(t, pongo, directories["working directory"], overrides...)
	if _, err := terminal.controller.WriteString("q"); err != nil {
		t.Fatalf("type Q: %v", err)
	}
	if status := waitForExit(t, command); status != 0 {
		t.Fatalf("pongo exited %d after Q, want 0", status)
	}

	for name, directory := range directories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, entry := range entries {
			t.Errorf("pongo created %s in %s", entry.Name(), name)
		}
	}
}

func typing(keys string) func(*pseudoTerminal, *os.Process) error {
	return func(terminal *pseudoTerminal, _ *os.Process) error {
		_, err := terminal.controller.WriteString(keys)
		return err
	}
}

func signaling(signal syscall.Signal) func(*pseudoTerminal, *os.Process) error {
	return func(_ *pseudoTerminal, process *os.Process) error {
		return process.Signal(signal)
	}
}

// assertStartupRejected runs command and requires a contextual error with exit status 1.
func assertStartupRejected(t *testing.T, command *exec.Cmd, stderr *bytes.Buffer) {
	t.Helper()

	if err := command.Start(); err != nil {
		t.Fatalf("start pongo: %v", err)
	}
	if status := waitForExit(t, command); status != 1 {
		t.Fatalf("pongo exited %d without a terminal, want 1; output: %s", status, stderr)
	}
	if stderr.Len() == 0 {
		t.Fatal("pongo printed no error without a terminal")
	}
}

// waitForExit waits for a started command and returns its exit status, killing it after exitTimeout.
func waitForExit(t *testing.T, command *exec.Cmd) int {
	t.Helper()

	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()

	select {
	case err := <-exited:
		var exitError *exec.ExitError
		if err != nil && !errors.As(err, &exitError) {
			t.Fatalf("wait for pongo: %v", err)
		}
		if !command.ProcessState.Exited() {
			t.Fatalf("pongo did not exit normally: %v", command.ProcessState)
		}
		return command.ProcessState.ExitCode()
	case <-time.After(exitTimeout):
		if err := command.Process.Kill(); err != nil {
			t.Errorf("kill pongo: %v", err)
		}
		<-exited
		t.Fatalf("pongo still running after %v", exitTimeout)
		return -1
	}
}

// pseudoTerminal is a pseudo-terminal whose output is captured from its controller side.
type pseudoTerminal struct {
	controller *os.File
	terminal   *os.File

	mu     sync.Mutex
	output bytes.Buffer
}

func newPseudoTerminal(t *testing.T, columns, rows uint16) *pseudoTerminal {
	t.Helper()

	controller, terminal, err := openPseudoTerminal()
	if err != nil {
		t.Fatalf("open pseudo-terminal: %v", err)
	}
	// The kernel ends controller reads once every terminal-side descriptor is closed.
	t.Cleanup(func() {
		if err := terminal.Close(); err != nil {
			t.Errorf("close pseudo-terminal: %v", err)
		}
		if err := controller.Close(); err != nil {
			t.Errorf("close pseudo-terminal controller: %v", err)
		}
	})

	size := struct{ rows, columns, xPixels, yPixels uint16 }{rows: rows, columns: columns}
	if err := ioctl(terminal, syscall.TIOCSWINSZ, unsafe.Pointer(&size)); err != nil {
		t.Fatalf("set pseudo-terminal size: %v", err)
	}

	pty := &pseudoTerminal{controller: controller, terminal: terminal}
	go pty.capture()

	return pty
}

func (pty *pseudoTerminal) capture() {
	buffer := make([]byte, 4096)
	for {
		count, err := pty.controller.Read(buffer)
		pty.mu.Lock()
		pty.output.Write(buffer[:count])
		pty.mu.Unlock()
		if err != nil {
			return
		}
	}
}

// startGame runs pongo on this terminal and waits until it has taken over the terminal. Pongo inherits the test
// environment, with color disabled and overrides applied last. It does not lead a session on the terminal, because on
// macOS a session leader's exit revokes its controlling terminal and would hide the state the tests inspect afterward.
func (pty *pseudoTerminal) startGame(t *testing.T, executable, directory string, overrides ...string) *exec.Cmd {
	t.Helper()

	before := pty.state(t)
	command := exec.Command(executable)
	command.Dir = directory
	command.Env = append(append(os.Environ(), "TERM=xterm-256color", "NO_COLOR=1"), overrides...)
	command.Stdin = pty.terminal
	command.Stdout = pty.terminal
	command.Stderr = pty.terminal
	if err := command.Start(); err != nil {
		t.Fatalf("start pongo: %v", err)
	}
	t.Cleanup(func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})

	deadline := time.Now().Add(exitTimeout)
	for pty.state(t) == before {
		if time.Now().After(deadline) {
			t.Fatalf("pongo never took over the terminal; output: %q", pty.written())
		}
		time.Sleep(10 * time.Millisecond)
	}

	return command
}

// written returns everything programs have written to the terminal so far.
func (pty *pseudoTerminal) written() string {
	pty.mu.Lock()
	defer pty.mu.Unlock()

	return pty.output.String()
}

// state returns the terminal's settings as reported by stty.
func (pty *pseudoTerminal) state(t *testing.T) string {
	t.Helper()

	command := exec.Command("stty", "-a")
	command.Stdin = pty.terminal
	output, err := command.Output()
	if err != nil {
		t.Fatalf("stty -a: %v", err)
	}

	return string(output)
}

func openBlocking(path string) (*os.File, error) {
	descriptor, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	return os.NewFile(uintptr(descriptor), path), nil
}

func ioctl(file *os.File, request uintptr, argument unsafe.Pointer) error {
	connection, err := file.SyscallConn()
	if err != nil {
		return fmt.Errorf("access %s: %w", file.Name(), err)
	}

	var callErr error
	if err := connection.Control(func(descriptor uintptr) {
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, descriptor, request, uintptr(argument)); errno != 0 {
			callErr = errno
		}
	}); err != nil {
		return fmt.Errorf("control %s: %w", file.Name(), err)
	}

	return callErr
}

// closeAfter closes file after a failure and returns the failure.
func closeAfter(file *os.File, failure error) error {
	if err := file.Close(); err != nil {
		return errors.Join(failure, err)
	}

	return failure
}
