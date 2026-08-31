package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/tools"
	"github.com/sspzoa/goppi/internal/ui"
	"github.com/sspzoa/goppi/internal/worktree"
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
		f, err := session.Resolve(args[1])
		if err != nil {
			return err
		}
		if cfg, err := config.Load(); err == nil {
			_ = tools.FireSessionEnd(context.Background(), cfg, f.ID, "delete")
		}
		if err := session.Delete(f.ID); err != nil {
			return err
		}
		if err := worktree.Remove(f.ID); err != nil {
			ui.Warn("worktree: %s", err)
		}
		ui.Info("deleted %s", f.ID)
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
		f, err = session.Resolve(args[0])
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
	s = session.SafeTitle(s)
	if s == "" {
		return "-"
	}
	return s
}
