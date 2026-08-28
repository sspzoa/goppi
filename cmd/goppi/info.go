package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/instructions"
	"github.com/sspzoa/goppi/internal/ui"
	"github.com/sspzoa/goppi/internal/upstage"
)

func cmdModels() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	for _, m := range upstage.ChatModels {
		ui.ModelRow(m.ID == cfg.Model, m.ID, m.Summary)
	}
	return nil
}

func cmdDoctor() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	key := cfg.ResolveAPIKey()
	src := cfg.KeySource()
	ok := func(ok bool, label, detail string) {
		mark := "✗"
		color := ui.ErrC()
		if ok {
			mark = "✓"
			color = ui.OK()
		}
		fmt.Fprintf(os.Stdout, "  %s %s  %s\n", ui.Paint(color, mark), label, ui.Paint(ui.Mute(), detail))
	}
	ok(key != "", "api key", map[bool]string{true: src + "  " + maskKey(key), false: "missing — goppi login"}[key != ""])
	ok(cfg.Model != "", "model", cfg.Model)
	ok(cfg.WorkDir != "", "workdir", cfg.WorkDir)
	if _, err := os.Stat(cfg.WorkDir); err != nil {
		ok(false, "workdir readable", err.Error())
	} else {
		ok(true, "workdir readable", "ok")
	}
	dir, err := config.UserDataDir()
	ok(err == nil, "session dir", dir)
	_, found := instructions.Load(cfg.WorkDir)
	if len(found) == 0 {
		ok(false, "instructions", "no GOPPI.md / AGENTS.md — goppi init")
	} else {
		ok(true, "instructions", fmt.Sprintf("%v", found))
	}
	return nil
}

func cmdInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	_, found := instructions.Load(cfg.WorkDir)
	info := map[string]any{
		"version":          config.Version,
		"model":            cfg.Model,
		"reasoning_effort": cfg.ReasoningEffort,
		"base_url":         cfg.BaseURL,
		"workdir":          cfg.WorkDir,
		"max_turns":        cfg.MaxTurns,
		"max_tokens":       cfg.MaxTokens,
		"key_source":       cfg.KeySource(),
		"has_key":          cfg.ResolveAPIKey() != "",
		"instructions":     found,
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}
	fmt.Printf("  version   %s\n", config.Version)
	fmt.Printf("  model     %s\n", cfg.Model)
	fmt.Printf("  effort    %s\n", cfg.ReasoningEffort)
	fmt.Printf("  base_url  %s\n", cfg.BaseURL)
	fmt.Printf("  workdir   %s\n", cfg.WorkDir)
	fmt.Printf("  key       %s\n", map[bool]string{true: cfg.KeySource(), false: "missing"}[cfg.ResolveAPIKey() != ""])
	if len(found) == 0 {
		fmt.Printf("  rules     (none)\n")
	} else {
		fmt.Printf("  rules     %s\n", found)
	}
	return nil
}
