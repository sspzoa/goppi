package main

import (
	"fmt"
	"os"

	"github.com/sspzoa/goppi/internal/ui"
	"github.com/sspzoa/goppi/internal/worktree"
)

func cmdWorktree(args []string) error {
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list":
		items, err := worktree.List()
		if err != nil {
			return err
		}
		if len(items) == 0 {
			ui.Info("worktree 없음. goppi --worktree")
			return nil
		}
		for _, wt := range items {
			fmt.Fprintf(os.Stdout, "  %s  %s  %s\n", wt.ID, wt.Branch, wt.Path)
		}
		return nil
	case "remove", "rm":
		if len(args) < 2 {
			return fmt.Errorf("usage: goppi worktree remove <id>")
		}
		if err := worktree.Remove(args[1]); err != nil {
			return err
		}
		ui.Info("removed worktree %s", args[1])
		return nil
	default:
		return fmt.Errorf("usage: goppi worktree [list|remove <id>]")
	}
}
