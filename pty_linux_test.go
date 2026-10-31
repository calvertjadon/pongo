package main_test

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

// openPseudoTerminal returns the controller side of a new pseudo-terminal and its unlocked terminal side.
func openPseudoTerminal() (*os.File, *os.File, error) {
	controller, err := openBlocking("/dev/ptmx")
	if err != nil {
		return nil, nil, err
	}

	var unlock int32
	if err := ioctl(controller, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock)); err != nil {
		return nil, nil, closeAfter(controller, fmt.Errorf("unlock pseudo-terminal: %w", err))
	}
	var number uint32
	if err := ioctl(controller, syscall.TIOCGPTN, unsafe.Pointer(&number)); err != nil {
		return nil, nil, closeAfter(controller, fmt.Errorf("name pseudo-terminal: %w", err))
	}

	terminal, err := openBlocking("/dev/pts/" + strconv.FormatUint(uint64(number), 10))
	if err != nil {
		return nil, nil, closeAfter(controller, err)
	}

	return controller, terminal, nil
}
