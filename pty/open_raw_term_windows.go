// +build windows

package pty

import (
	"io"
	"os"

	"github.com/fatih/color"
)

func IsTerminal() bool {
	return true
}

func OpenRawTerm() (Term, error) {
	return noopRestoreTerm{
		Reader: os.Stdin,
		Writer: color.Output,
	}, nil
}

type noopRestoreTerm struct {
	io.Reader
	io.Writer
}

func (noopRestoreTerm) Restore() error { return nil }
