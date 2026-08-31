package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGitGuardRestoresHookAndConfig(t *testing.T) {
	wd := t.TempDir()
	hooks := filepath.Join(wd, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wd, ".git", "objects", "aa"), 0o755); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(hooks, "pre-commit")
	if err := os.WriteFile(hook, []byte("safe\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(wd, ".git", "config")
	if err := os.WriteFile(cfg, []byte("[core]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	obj := filepath.Join(wd, ".git", "objects", "aa", "bb")
	if err := os.WriteFile(obj, []byte("obj"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := snapshotGitGuard(wd)
	if g == nil {
		t.Fatal("expected guard")
	}
	if err := os.WriteFile(hook, []byte("pwned\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("pwned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wd, ".git", "objects", "aa", "cc"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := revertGitWrites(g); err == nil {
		t.Fatal("expected revert error")
	}
	if data, err := os.ReadFile(hook); err != nil || string(data) != "safe\n" {
		t.Fatalf("hook %q %v", data, err)
	}
	if data, err := os.ReadFile(cfg); err != nil || string(data) != "[core]\n" {
		t.Fatalf("config %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(wd, ".git", "objects", "aa", "cc")); err == nil {
		t.Fatal("new object should be removed")
	}
	if _, err := os.Stat(obj); err != nil {
		t.Fatal("existing object should stay")
	}
}

func TestGitGuardRestoresHEADAndRefs(t *testing.T) {
	wd := t.TempDir()
	git := filepath.Join(wd, ".git")
	refs := filepath.Join(git, "refs", "heads")
	if err := os.MkdirAll(refs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(git, "objects", "aa"), 0o755); err != nil {
		t.Fatal(err)
	}
	head := filepath.Join(git, "HEAD")
	if err := os.WriteFile(head, []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	branch := filepath.Join(refs, "main")
	if err := os.WriteFile(branch, []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	packed := filepath.Join(git, "packed-refs")
	if err := os.WriteFile(packed, []byte("# pack\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	obj := filepath.Join(git, "objects", "aa", "bb")
	if err := os.WriteFile(obj, []byte("obj"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := snapshotGitGuard(wd)
	if g == nil {
		t.Fatal("expected guard")
	}
	if err := os.WriteFile(head, []byte("ref: refs/heads/pwned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(branch, []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refs, "pwned"), []byte("cccccccccccccccccccccccccccccccccccccccc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packed, []byte("pwned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(git, "objects", "aa", "cc"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := revertGitWrites(g)
	if err == nil {
		t.Fatal("expected revert error")
	}
	if !strings.Contains(err.Error(), ".git/HEAD") || !strings.Contains(err.Error(), ".git/refs") {
		t.Fatalf("error %v", err)
	}
	if data, readErr := os.ReadFile(head); readErr != nil || string(data) != "ref: refs/heads/main\n" {
		t.Fatalf("HEAD %q %v", data, readErr)
	}
	if data, readErr := os.ReadFile(branch); readErr != nil || !strings.HasPrefix(string(data), "aaaaaaaa") {
		t.Fatalf("ref %q %v", data, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(refs, "pwned")); statErr == nil {
		t.Fatal("new ref should be removed")
	}
	if data, readErr := os.ReadFile(packed); readErr != nil || string(data) != "# pack\n" {
		t.Fatalf("packed-refs %q %v", data, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(git, "objects", "aa", "cc")); statErr == nil {
		t.Fatal("new object should be removed")
	}
}

func TestGitGuardLeavesHugePackedRefs(t *testing.T) {
	wd := t.TempDir()
	git := filepath.Join(wd, ".git")
	if err := os.MkdirAll(git, 0o755); err != nil {
		t.Fatal(err)
	}
	big := bytes.Repeat([]byte("a"), maxGitGuardFile+8)
	packed := filepath.Join(git, "packed-refs")
	if err := os.WriteFile(packed, big, 0o644); err != nil {
		t.Fatal(err)
	}
	g := snapshotGitGuard(wd)
	if err := os.WriteFile(packed, append(big, 'x'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := revertGitWrites(g); err != nil {
		t.Fatalf("huge packed-refs should be skipped: %v", err)
	}
	if _, err := os.Stat(packed); err != nil {
		t.Fatal("huge packed-refs must stay")
	}
}

func TestGitGuardSkipsMissingGit(t *testing.T) {
	wd := t.TempDir()
	if g := snapshotGitGuard(wd); g != nil {
		t.Fatal("no .git should skip")
	}
}

func TestBashAllowsGitInitWithoutRepo(t *testing.T) {
	wd := t.TempDir()
	reg := New(wd, nil, nil)
	reg.SetSandbox("off")
	in, _ := json.Marshal(map[string]string{"command": "mkdir -p .git/hooks && echo init > .git/hooks/pre-commit && echo ok > .git/config"})
	if _, err := reg.Run(context.Background(), "bash", in); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wd, ".git", "hooks", "pre-commit")); err != nil {
		t.Fatal("git init in a non-repo must keep hooks")
	}
}

func TestGitGuardWorktreeCommonDir(t *testing.T) {
	wd := t.TempDir()
	common := filepath.Join(wd, "common.git")
	wt := filepath.Join(wd, "wt.git")
	src := filepath.Join(wd, "src")
	if err := os.MkdirAll(filepath.Join(common, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(common, "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("safe\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "commondir"), []byte("../common.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ".git"), []byte("gitdir: "+wt+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wtHead := filepath.Join(wt, "HEAD")
	if err := os.WriteFile(wtHead, []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := snapshotGitGuard(src)
	if g == nil {
		t.Fatal("expected worktree guard")
	}
	if err := os.WriteFile(hook, []byte("pwned\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wtHead, []byte("ref: refs/heads/pwned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := revertGitWrites(g); err == nil {
		t.Fatal("expected revert")
	}
	if data, err := os.ReadFile(hook); err != nil || string(data) != "safe\n" {
		t.Fatalf("common hook %q %v", data, err)
	}
	if data, err := os.ReadFile(wtHead); err != nil || string(data) != "ref: refs/heads/main\n" {
		t.Fatalf("worktree HEAD %q %v", data, err)
	}
}

func TestBashRevertsGitHooksSandboxOff(t *testing.T) {
	wd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wd, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(wd, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("safe\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := New(wd, nil, nil)
	reg.SetSandbox("off")
	in, _ := json.Marshal(map[string]string{"command": "echo pwned > .git/hooks/pre-commit"})
	_, err := reg.Run(context.Background(), "bash", in)
	if err == nil {
		t.Fatal("expected revert error")
	}
	if !strings.Contains(err.Error(), ".git/hooks") {
		t.Fatalf("error %v", err)
	}
	data, readErr := os.ReadFile(hook)
	if readErr != nil || string(data) != "safe\n" {
		t.Fatalf("hook persisted %q %v", data, readErr)
	}
}

func TestBashRevertsGitHEAD(t *testing.T) {
	wd := t.TempDir()
	git := filepath.Join(wd, ".git")
	if err := os.MkdirAll(filepath.Join(git, "refs", "heads"), 0o755); err != nil {
		t.Fatal(err)
	}
	head := filepath.Join(git, "HEAD")
	if err := os.WriteFile(head, []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	branch := filepath.Join(git, "refs", "heads", "main")
	if err := os.WriteFile(branch, []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(wd, nil, nil)
	reg.SetSandbox("off")
	in, _ := json.Marshal(map[string]string{"command": "echo pwned > .git/HEAD && echo bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb > .git/refs/heads/main"})
	_, err := reg.Run(context.Background(), "bash", in)
	if err == nil {
		t.Fatal("expected revert error")
	}
	if !strings.Contains(err.Error(), ".git/HEAD") && !strings.Contains(err.Error(), ".git/refs") {
		t.Fatalf("error %v", err)
	}
	if data, readErr := os.ReadFile(head); readErr != nil || string(data) != "ref: refs/heads/main\n" {
		t.Fatalf("HEAD persisted %q %v", data, readErr)
	}
	if data, readErr := os.ReadFile(branch); readErr != nil || !strings.HasPrefix(string(data), "aaaaaaaa") {
		t.Fatalf("ref persisted %q %v", data, readErr)
	}
}

func TestBashBackgroundRevertsGitHooks(t *testing.T) {
	wd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wd, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(wd, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("safe\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := New(wd, nil, nil)
	reg.SetSandbox("off")
	in, _ := json.Marshal(map[string]any{"command": "echo pwned > .git/hooks/pre-commit", "background": true})
	if _, err := reg.Run(context.Background(), "bash", in); err != nil {
		t.Fatal(err)
	}
	poll, _ := json.Marshal(map[string]int{"id": 1, "wait_sec": 5})
	deadline := time.Now().Add(5 * time.Second)
	for {
		status, _ := reg.Run(context.Background(), "bash_poll", poll)
		if strings.Contains(status, "refusing") || strings.Contains(status, "exited") || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	data, readErr := os.ReadFile(hook)
	if readErr != nil || string(data) != "safe\n" {
		t.Fatalf("background hook persisted %q %v", data, readErr)
	}
}
