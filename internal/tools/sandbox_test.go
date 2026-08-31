package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSandboxWriteRootsIncludesExtra(t *testing.T) {
	wd := t.TempDir()
	extra := t.TempDir()
	got := strings.Join(sandboxWriteRoots(wd, extra), "\n")
	want := extra
	if real, err := filepath.EvalSymlinks(extra); err == nil {
		want = real
	}
	if !strings.Contains(got, want) {
		t.Fatalf("missing extra %s in %q", want, got)
	}
}

func TestApplySandboxForwardsExtra(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("workspace sandbox is darwin/linux")
	}
	extra := t.TempDir()
	want := extra
	if real, err := filepath.EvalSymlinks(extra); err == nil {
		want = real
	}
	cmd := exec.Command("true")
	if err := ApplySandbox(cmd, t.TempDir(), "workspace", extra); err != nil {
		t.Fatal(err)
	}
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(strings.Join(cmd.Args, "\n"), want) {
			t.Fatalf("args %v", cmd.Args)
		}
	case "linux":
		found := false
		for _, e := range cmd.Env {
			if strings.HasPrefix(e, "GOPPI_SANDBOX_EXTRA=") && strings.Contains(e, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("env %v", cmd.Env)
		}
	}
}

func TestSandboxPrivDenyIncludesSudo(t *testing.T) {
	got := strings.Join(sandboxPrivDeny(), "\n")
	if !strings.Contains(got, "/sudo") && !strings.Contains(got, "/usr/bin/sudo") {
		t.Fatalf("missing sudo in %q", got)
	}
	if !strings.Contains(got, "/su") {
		t.Fatalf("missing su in %q", got)
	}
}

func TestSandboxGitDenyPaths(t *testing.T) {
	wd := t.TempDir()
	if sandboxGitDeny(wd) != nil {
		t.Fatal("no .git should skip")
	}
	if err := os.MkdirAll(filepath.Join(wd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := sandboxGitDeny(wd)
	joined := strings.Join(got, "\n")
	for _, want := range []string{"/.git/hooks", "/.git/config", "/.git/objects", "/.git/HEAD", "/.git/packed-refs", "/.git/refs"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %q", want, got)
		}
	}
}

func TestBashSandboxBlocksGitHooks(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("seatbelt can deny a subpath of an allowed workdir; landlock cannot")
	}
	wd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wd, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := New(wd, nil, nil)
	hook := filepath.Join(wd, ".git", "hooks", "pre-commit")
	in, _ := json.Marshal(map[string]string{"command": "echo pwned > .git/hooks/pre-commit"})
	_, _ = reg.Run(context.Background(), "bash", in)
	if data, err := os.ReadFile(hook); err == nil && strings.Contains(string(data), "pwned") {
		t.Fatalf("workspace sandbox wrote git hook: %q", data)
	}
	cfg := filepath.Join(wd, ".git", "config")
	in, _ = json.Marshal(map[string]string{"command": "echo pwned > .git/config"})
	_, _ = reg.Run(context.Background(), "bash", in)
	if data, err := os.ReadFile(cfg); err == nil && strings.Contains(string(data), "pwned") {
		t.Fatalf("workspace sandbox wrote git config: %q", data)
	}
	head := filepath.Join(wd, ".git", "HEAD")
	if err := os.WriteFile(head, []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in, _ = json.Marshal(map[string]string{"command": "echo pwned > .git/HEAD"})
	_, _ = reg.Run(context.Background(), "bash", in)
	if data, err := os.ReadFile(head); err == nil && strings.Contains(string(data), "pwned") {
		t.Fatalf("workspace sandbox wrote git HEAD: %q", data)
	}
}
