package main_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// normalizeState hides the PENDIN flag, which the macOS terminal driver sets on its own when input is pending and
// canonical mode resumes. It is not a mode a program sets or restores.
func normalizeState(state string) string {
	return strings.ReplaceAll(state, " pendin", " -pendin")
}

// openPseudoTerminal returns the controller side of a new pseudo-terminal and its unlocked terminal side.
func openPseudoTerminal() (*os.File, *os.File, error) {
	controller, err := openBlocking("/dev/ptmx")
	if err != nil {
		return nil, nil, err
	}

	if err := ioctl(controller, syscall.TIOCPTYGRANT, nil); err != nil {
		return nil, nil, closeAfter(controller, fmt.Errorf("grant pseudo-terminal: %w", err))
	}
	if err := ioctl(controller, syscall.TIOCPTYUNLK, nil); err != nil {
		return nil, nil, closeAfter(controller, fmt.Errorf("unlock pseudo-terminal: %w", err))
	}
	// TIOCPTYGNAME writes a NUL-terminated path into a 128-byte buffer.
	name := make([]byte, 128)
	if err := ioctl(controller, syscall.TIOCPTYGNAME, unsafe.Pointer(&name[0])); err != nil {
		return nil, nil, closeAfter(controller, fmt.Errorf("name pseudo-terminal: %w", err))
	}
	path, _, _ := bytes.Cut(name, []byte{0})

	terminal, err := openBlocking(string(path))
	if err != nil {
		return nil, nil, closeAfter(controller, err)
	}

	return controller, terminal, nil
}
