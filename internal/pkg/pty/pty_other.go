// +build !windows

package pty

import (
	"os"

	"github.com/creack/pty"
)

func newPty() (Pty, error) {
	pty, tty, err := pty.Open()
	if err != nil {
		return nil, err
	}

	return &unixPty{
		pty: pty,
		tty: tty,
	}, nil
}

type unixPty struct {
	pty, tty *os.File
}

func (p *unixPty) InPipe() *os.File {
	return p.tty
}

func (p *unixPty) OutPipe() *os.File {
	return p.pty
}

func (p *unixPty) Resize(cols uint16, rows uint16) error {
	return pty.Setsize(p.tty, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

func (p *unixPty) Close() error {
	p.pty.Close()
	p.tty.Close()
	return nil
}
