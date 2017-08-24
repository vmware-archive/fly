// +build windows

package pty

import (
	"io"
	"os"

	colorable "github.com/mattn/go-colorable"
)

func IsTerminal() bool {
	return true
}

func OpenRawTerm() (Term, error) {
	return noopRestoreTerm{
		Reader: os.Stdin,
		Writer: colorable.NewColorableStdout(),
	}, nil
}

type noopRestoreTerm struct {
	io.Reader
	io.Writer
}

func (noopRestoreTerm) Restore() error { return nil }
