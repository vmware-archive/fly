package main

import (
	"fmt"
	"net"
	"os"

	"github.com/concourse/fly/commands"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/concourse/go-concourse/concourse"
	"github.com/jessevdk/go-flags"
)

func main() {
	parser := flags.NewParser(&commands.Fly, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		if err == concourse.ErrUnauthorized {
			fmt.Fprintln(ui.Stderr, "not authorized. run the following to log in:")
			fmt.Fprintln(ui.Stderr, "")
			fmt.Fprintln(ui.Stderr, "    "+ui.Embolden("fly -t %s login", commands.Fly.Target))
			fmt.Fprintln(ui.Stderr, "")
		} else if err == rc.ErrNoTargetSpecified {
			fmt.Fprintln(ui.Stderr, "no target specified. specify the target with "+ui.Embolden("-t")+" or log in like so:")
			fmt.Fprintln(ui.Stderr, "")
			fmt.Fprintln(ui.Stderr, "    "+ui.Embolden("fly -t (alias) login -c (concourse url)"))
			fmt.Fprintln(ui.Stderr, "")
		} else if versionErr, ok := err.(rc.ErrVersionMismatch); ok {
			fmt.Fprintln(ui.Stderr, versionErr.Error())
			fmt.Fprintln(ui.Stderr, ui.WarningColor("cowardly refusing to run due to significant version discrepancy"))
		} else if netErr, ok := err.(net.Error); ok {
			fmt.Fprintf(ui.Stderr, "could not reach the Concourse server called %s:\n", ui.Embolden("%s", commands.Fly.Target))

			fmt.Fprintln(ui.Stderr, "")
			fmt.Fprintln(ui.Stderr, "    "+ui.Embolden("%s", netErr))
			fmt.Fprintln(ui.Stderr, "")
			fmt.Fprintln(ui.Stderr, "is the targeted Concourse running?")
		} else if err == commands.ErrShowHelpMessage {
			helpParser := flags.NewParser(&commands.Fly, flags.HelpFlag)
			helpParser.NamespaceDelimiter = "-"
			helpParser.ParseArgs([]string{"-h"})
			helpParser.WriteHelp(os.Stdout)
			os.Exit(0)
		} else {
			fmt.Fprintf(ui.Stderr, "error: %s\n", err)
		}

		os.Exit(1)
	}
}
