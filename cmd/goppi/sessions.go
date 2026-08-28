package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/ui"
)

func cmdSessions(args []string) error {
	if len(args) == 0 {
		return sessionsList()
	}
	switch args[0] {
	case "list":
		return sessionsList()
	case "delete", "rm":
		if len(args) < 2 {
			return fmt.Errorf("usage: goppi sessions delete <id>")
		}
		if err := session.Delete(args[1]); err != nil {
			return err
		}
		ui.Info("deleted %s", args[1])
		return nil
	default:
		return fmt.Errorf("usage: goppi sessions [list|delete <id>]")
	}
}

func sessionsList() error {
	items, err := session.List()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		ui.Info("세션이 없습니다.")
		return nil
	}
	fmt.Fprintf(os.Stdout, "  %s  %s  %s\n", pad("ID", 16), pad("UPDATED", 16), "TITLE")
	for _, f := range items {
		fmt.Fprintf(os.Stdout, "  %s  %s  %s\n",
			pad(f.ID, 16),
			pad(f.UpdatedAt.Local().Format("01-02 15:04"), 16),
			dash(f.Title),
		)
	}
	return nil
}

func cmdExport(args []string) error {
	var f session.File
	var err error
	if len(args) == 0 {
		f, err = session.LoadLast()
	} else {
		f, err = session.Load(args[0])
	}
	if err != nil {
		return err
	}
	fmt.Print(session.ExportMarkdown(f))
	return nil
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
