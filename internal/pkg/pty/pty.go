package pty

import (
	"os"
)

// Pty is the minimal pseudo-tty interface we require.
type Pty interface {
	InPipe() *os.File
	OutPipe() *os.File
	Resize(cols uint16, rows uint16) error
	Close() error
}

// New creates a new Pty.
func New() (Pty, error) {
	return newPty()
}
