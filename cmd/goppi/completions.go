package main

import (
	"fmt"
	"os"

	"github.com/sspzoa/goppi/internal/complete"
)

func cmdCompletions(args []string) error {
	shell := "zsh"
	if len(args) > 0 {
		shell = args[0]
	}
	script, err := complete.Script(shell)
	if err != nil {
		return err
	}
	fmt.Print(script)
	return nil
}

func cmdComplete(args []string) error {
	kind := "commands"
	prefix := ""
	if len(args) > 0 {
		kind = args[0]
	}
	if len(args) > 1 {
		prefix = args[1]
	}
	names, err := complete.Names(kind, prefix)
	if err != nil {
		return err
	}
	for _, name := range names {
		fmt.Fprintln(os.Stdout, name)
	}
	return nil
}
