package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/instructions"
	"github.com/sspzoa/goppi/internal/mcp"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/tools"
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

func cmdDoctor(args []string) error {
	cfg, args, err := loadConfigWithWorkdirArgs(args)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fix := fs.Bool("fix", false, "chmod open secret files back to 0600")
	online := fs.Bool("online", false, "probe the stored API key")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	key := cfg.ResolveAPIKey()
	src := cfg.KeySource()
	var failed int
	check := func(good, fatal bool, label, detail string) {
		mark := "✗"
		color := ui.ErrC()
		if good {
			mark = "✓"
			color = ui.OK()
		} else if fatal {
			failed++
		}
		fmt.Fprintf(os.Stdout, "  %s %s  %s\n", ui.Paint(color, mark), label, ui.Paint(ui.Mute(), detail))
	}
	check(key != "", true, "api key", map[bool]string{true: src, false: "missing — goppi login"}[key != ""])
	if *online && key != "" {
		api := upstage.New(key, cfg.BaseURL)
		api.UserAgent = "goppi/" + config.Version
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err := api.ProbeKey(ctx, cfg.Model)
		cancel()
		if err != nil {
			check(false, true, "api key live", err.Error())
		} else {
			check(true, false, "api key live", "ok")
		}
	}
	if cred, err := config.CredentialsPath(); err == nil {
		if _, err := os.Lstat(cred); err == nil {
			if err := config.SecretPermError(cred, 0o600); err != nil {
				if *fix {
					if e := config.TightenSecret(cred, 0o600); e != nil {
						check(false, true, "credentials mode", e.Error())
					} else {
						check(true, false, "credentials mode", "was open, chmod 0600")
					}
				} else {
					check(false, true, "credentials mode", err.Error()+" — goppi doctor --fix")
				}
			} else {
				check(true, false, "credentials mode", "0600")
			}
		}
	}
	if path, err := config.UserConfigPath(); err == nil && config.FileHasAPIKey(path) {
		if err := config.SecretPermError(path, 0o600); err != nil {
			if *fix {
				if e := config.TightenSecret(path, 0o600); e != nil {
					check(false, true, "config.json mode", e.Error())
				} else {
					check(true, false, "config.json mode", "was open, chmod 0600")
				}
			} else {
				check(false, true, "config.json mode", err.Error()+" — goppi doctor --fix")
			}
		} else {
			check(true, false, "config.json mode", "0600")
		}
	}
	if dir, err := config.UserConfigDir(); err == nil {
		doctorDirMode(check, *fix, "config dir mode", dir)
	}
	if dir, err := config.UserDataDir(); err == nil {
		doctorDirMode(check, *fix, "data dir mode", dir)
	}
	check(cfg.Model != "", true, "model", cfg.Model)
	check(cfg.WorkDir != "", true, "workdir", cfg.WorkDir)
	if _, err := os.Stat(cfg.WorkDir); err != nil {
		check(false, true, "workdir readable", err.Error())
	} else {
		check(true, false, "workdir readable", "ok")
		if err := config.ProbeWritable(cfg.WorkDir); err != nil {
			check(false, true, "workdir writable", err.Error())
		} else {
			check(true, false, "workdir writable", "ok")
		}
	}
	dir, err := config.UserDataDir()
	check(err == nil, true, "session dir", dir)
	if err == nil {
		if err := config.ProbeWritable(dir); err != nil {
			check(false, true, "session dir writable", err.Error())
		} else {
			check(true, false, "session dir writable", "ok")
		}
		doctorDataSubdir(check, *fix, "sessions dir", filepath.Join(dir, "sessions"))
		doctorSecretFiles(check, *fix, "session files", filepath.Join(dir, "sessions"), ".json")
		if probs := session.Problems(); len(probs) > 0 {
			check(false, true, "session JSON", strings.Join(probs, "; "))
		} else {
			check(true, false, "session JSON", "ok")
		}
		doctorDataSubdir(check, *fix, "exports dir", filepath.Join(dir, "exports"))
		doctorSecretFiles(check, *fix, "export files", filepath.Join(dir, "exports"), ".md")
		doctorDataSubdir(check, *fix, "worktrees dir", filepath.Join(dir, "worktrees"))
		if err := config.SecretPermError(filepath.Join(dir, "last"), 0o600); err != nil {
			if *fix {
				if e := config.TightenSecret(filepath.Join(dir, "last"), 0o600); e != nil {
					check(false, true, "last pointer", e.Error())
				} else {
					check(true, false, "last pointer", "was open, chmod 0600")
				}
			} else {
				check(false, true, "last pointer", err.Error()+" — goppi doctor --fix")
			}
		}
	}
	_, found := instructions.Load(cfg.WorkDir)
	if len(found) == 0 {
		check(false, false, "instructions", "no GOPPI.md / AGENTS.md — goppi init")
	} else {
		check(true, false, "instructions", fmt.Sprintf("%v", found))
	}
	if cfg.AlwaysApprove {
		check(false, false, "always_approve", "on — write/bash/MCP will not ask")
	} else {
		check(true, false, "always_approve", "off")
	}
	switch cfg.Sandbox {
	case "off":
		check(false, false, "sandbox", "off — bash has this account's write and network privileges")
	case "strict":
		check(true, false, "sandbox", "strict — bash writes stay in workdir/temp/cache and network is blocked")
	default:
		check(true, false, "sandbox", cfg.Sandbox+" — bash writes stay in workdir/temp/cache")
	}
	if names := cfg.MCPNames(); len(names) == 0 {
		check(true, false, "mcp", "none")
	} else {
		check(true, false, "mcp", strings.Join(names, ", "))
	}
	if names := cfg.LSPNames(); len(names) == 0 {
		check(true, false, "lsp", "auto (gopls if this is a Go module)")
	} else {
		check(true, false, "lsp", strings.Join(names, ", "))
	}
	if failed > 0 {
		return fmt.Errorf("doctor: %d check(s) failed", failed)
	}
	return nil
}

func doctorSecretFiles(check func(good, fatal bool, label, detail string), fix bool, label, dir, suffix string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			check(true, false, label, "none")
			return
		}
		check(false, true, label, err.Error())
		return
	}
	var open int
	var fixed int
	for _, e := range ents {
		if e.IsDir() || (suffix != "" && !strings.HasSuffix(e.Name(), suffix)) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if err := config.SecretPermError(path, 0o600); err == nil {
			continue
		}
		open++
		if fix {
			if err := config.TightenSecret(path, 0o600); err == nil {
				fixed++
			}
		}
	}
	if open == 0 {
		check(true, false, label, "ok")
		return
	}
	if fix && fixed == open {
		check(true, false, label, fmt.Sprintf("was open, chmod 0600 (%d)", fixed))
		return
	}
	check(false, true, label, fmt.Sprintf("%d file(s) group/other readable — goppi doctor --fix", open))
}

func doctorDataSubdir(check func(good, fatal bool, label, detail string), fix bool, label, dir string) {
	if _, err := os.Lstat(dir); err != nil {
		if os.IsNotExist(err) {
			return
		}
		check(false, true, label, err.Error())
		return
	}
	if err := config.SecretLinkError(dir); err != nil {
		check(false, true, label, err.Error())
		return
	}
	doctorDirMode(check, fix, label, dir)
}

func doctorDirMode(check func(good, fatal bool, label, detail string), fix bool, label, dir string) {
	if err := config.WorldWritableError(dir); err != nil {
		if fix {
			if e := os.Chmod(dir, 0o700); e != nil {
				check(false, true, label, e.Error())
			} else {
				check(true, false, label, "was world-writable, chmod 0700")
			}
		} else {
			check(false, true, label, err.Error()+" — goppi doctor --fix")
		}
		return
	}
	check(true, false, label, "ok")
}

func cmdInspect(args []string) error {
	cfg, args, err := loadConfigWithWorkdirArgs(args)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	_, found := instructions.Load(cfg.WorkDir)
	info := map[string]any{
		"version":          config.Version,
		"provider":         cfg.Provider,
		"mode":             cfg.Mode,
		"model":            cfg.Model,
		"reasoning_effort": cfg.ReasoningEffort,
		"base_url":         cfg.BaseURL,
		"workdir":          cfg.WorkDir,
		"max_turns":        cfg.MaxTurns,
		"max_tokens":       cfg.MaxTokens,
		"auto_compact":     cfg.AutoCompact,
		"compact_at":       cfg.CompactAt,
		"key_source":       cfg.KeySource(),
		"has_key":          cfg.ResolveAPIKey() != "",
		"always_approve":   cfg.AlwaysApprove,
		"sandbox":          cfg.Sandbox,
		"instructions":     found,
		"mcp_servers":      cfg.MCPNames(),
		"lsp_servers":      cfg.LSPNames(),
		"worktree":         cfg.Worktree,
		"hooks":            cfg.HookCounts(),
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}
	fmt.Printf("  version   %s\n", config.Version)
	fmt.Printf("  provider  %s\n", cfg.Provider)
	fmt.Printf("  mode      %s\n", cfg.Mode)
	fmt.Printf("  model     %s\n", cfg.Model)
	fmt.Printf("  effort    %s\n", cfg.ReasoningEffort)
	fmt.Printf("  base_url  %s\n", cfg.BaseURL)
	fmt.Printf("  workdir   %s\n", cfg.WorkDir)
	fmt.Printf("  key       %s\n", map[bool]string{true: cfg.KeySource(), false: "missing"}[cfg.ResolveAPIKey() != ""])
	fmt.Printf("  yolo      %v\n", cfg.AlwaysApprove)
	fmt.Printf("  sandbox   %s\n", cfg.Sandbox)
	fmt.Printf("  worktree  %v\n", cfg.Worktree)
	fmt.Printf("  compact   %v @ %d\n", cfg.AutoCompact, cfg.CompactAt)
	h := cfg.HookCounts()
	fmt.Printf("  hooks     pre %d  post %d  start %d  end %d\n", h["pre_tool"], h["post_tool"], h["session_start"], h["session_end"])
	if len(found) == 0 {
		fmt.Printf("  rules     (none)\n")
	} else {
		fmt.Printf("  rules     %s\n", found)
	}
	if names := cfg.MCPNames(); len(names) == 0 {
		fmt.Printf("  mcp       (none)\n")
	} else {
		fmt.Printf("  mcp       %s\n", strings.Join(names, ", "))
	}
	if names := cfg.LSPNames(); len(names) == 0 {
		fmt.Printf("  lsp       auto\n")
	} else {
		fmt.Printf("  lsp       %s\n", strings.Join(names, ", "))
	}
	return nil
}

func cmdMCP(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list":
		names := cfg.MCPNames()
		if len(names) == 0 {
			ui.Info("no mcp servers in ~/.config/goppi/config.json")
			return nil
		}
		for _, name := range names {
			s := cfg.MCPServers[name]
			fmt.Printf("  %s  %s %s\n", name, s.Command, strings.Join(s.Args, " "))
		}
		return nil
	case "tools":
		sessions, errs := mcp.StartAll(context.Background(), cfg.MCPServers, cfg.WorkDir, config.Version, func(cmd *exec.Cmd) error {
			return tools.ApplySandbox(cmd, cfg.WorkDir, cfg.Sandbox, cfg.ExtraDirs...)
		})
		defer func() {
			for _, s := range sessions {
				s.Close()
			}
		}()
		for _, e := range errs {
			ui.Warn("%s", e)
		}
		if len(sessions) == 0 {
			if len(errs) == 0 {
				ui.Info("no mcp servers in ~/.config/goppi/config.json")
			}
			return nil
		}
		for _, s := range sessions {
			for _, t := range s.Tools {
				fmt.Printf("  %s  %s\n", mcp.ToolName(s.Name, t.Name), t.Description)
			}
		}
		return nil
	default:
		return fmt.Errorf("usage: goppi mcp [list|tools]")
	}
}
