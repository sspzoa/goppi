package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sspzoa/goppi/internal/config"
)

func TestMatchHook(t *testing.T) {
	if !matchHook("", "bash") || !matchHook("*", "bash") || !matchHook("bash", "bash") {
		t.Fatal("exact/all")
	}
	if matchHook("bash", "write_file") {
		t.Fatal("other")
	}
	if !matchHook("mcp_*", "mcp_fs_read") || matchHook("mcp_*", "bash") {
		t.Fatal("prefix")
	}
}

func TestPreToolHookDenies(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, AlwaysAllow)
	reg.SetHooks(config.Hooks{PreTool: []config.Hook{{
		Matcher: "bash",
		Command: "echo blocked; exit 2",
	}}})
	in, _ := json.Marshal(map[string]string{"command": "echo hi"})
	_, err := reg.Run(context.Background(), "bash", in)
	if err == nil || !strings.Contains(err.Error(), "hook denied") {
		t.Fatalf("got %v", err)
	}
}

func TestPreToolHookMatcherSkips(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	reg.SetHooks(config.Hooks{PreTool: []config.Hook{{
		Matcher: "bash",
		Command: "exit 2",
	}}})
	in, _ := json.Marshal(map[string]any{"todos": []map[string]string{
		{"id": "1", "content": "x", "status": "pending"},
	}})
	if _, err := reg.Run(context.Background(), "todo_write", in); err != nil {
		t.Fatal(err)
	}
}

func TestPostToolHookAppends(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(dir, nil, nil)
	reg.SetHooks(config.Hooks{PostTool: []config.Hook{{
		Command: "echo HOOKED",
	}}})
	in, _ := json.Marshal(map[string]string{"path": "a.txt"})
	out, err := reg.Run(context.Background(), "read_file", in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "HOOKED") {
		t.Fatalf("%q", out)
	}
}

func TestSessionStartHook(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "started")
	cfg := config.Default()
	cfg.WorkDir = dir
	cfg.Hooks.SessionStart = []config.Hook{{Command: "touch started"}}
	if err := FireSessionStart(context.Background(), cfg, "abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(err)
	}
}

func TestSessionEndHook(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ended")
	cfg := config.Default()
	cfg.WorkDir = dir
	cfg.Hooks.SessionEnd = []config.Hook{{Command: "touch ended"}}
	if err := FireSessionEnd(context.Background(), cfg, "abc", "close"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(err)
	}
}

func TestPreToolHookDenialRedactsSecrets(t *testing.T) {
	t.Setenv("UPSTAGE_API_KEY", "up_hook_secret_value")
	dir := t.TempDir()
	reg := New(dir, nil, AlwaysAllow)
	reg.SetHooks(config.Hooks{PreTool: []config.Hook{{
		Command: "echo up_hook_secret_value; echo sk-abcdefghijklmnopqrstuvwxyz; exit 2",
	}}})
	in, _ := json.Marshal(map[string]string{"command": "echo hi"})
	_, err := reg.Run(context.Background(), "bash", in)
	if err == nil {
		t.Fatal("expected deny")
	}
	msg := err.Error()
	if strings.Contains(msg, "up_hook_secret_value") || strings.Contains(msg, "sk-abcdefgh") {
		t.Fatalf("leaked: %q", msg)
	}
	if !strings.Contains(msg, "[redacted]") {
		t.Fatalf("expected redaction: %q", msg)
	}
}

func TestHookTimeoutKillsChildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups are unix")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "late")
	cfg := config.Default()
	cfg.WorkDir = dir
	cfg.Hooks.SessionStart = []config.Hook{{
		Command: "(sleep 2; echo late > late) & sleep 60",
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	err := FireSessionStart(ctx, cfg, "abc")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("got %v", err)
	}
	time.Sleep(2500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("child survived hook cancel")
	}
}

func TestHookRevertsGitHEAD(t *testing.T) {
	wd := t.TempDir()
	git := filepath.Join(wd, ".git")
	if err := os.MkdirAll(filepath.Join(git, "refs", "heads"), 0o755); err != nil {
		t.Fatal(err)
	}
	head := filepath.Join(git, "HEAD")
	if err := os.WriteFile(head, []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.WorkDir = wd
	cfg.Sandbox = "off"
	cfg.Hooks.SessionStart = []config.Hook{{Command: "echo pwned > .git/HEAD"}}
	err := FireSessionStart(context.Background(), cfg, "abc")
	if err == nil || !strings.Contains(err.Error(), ".git/HEAD") {
		t.Fatalf("got %v", err)
	}
	if data, readErr := os.ReadFile(head); readErr != nil || string(data) != "ref: refs/heads/main\n" {
		t.Fatalf("HEAD persisted %q %v", data, readErr)
	}
}

func TestHookSandboxBlocksOutsideWrite(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("workspace sandbox is darwin/linux")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home")
	}
	outside := filepath.Join(home, ".goppi-hook-sandbox-test-"+strconv.Itoa(os.Getpid()))
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outside) })
	marker := filepath.Join(outside, "pwned")
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	cfg.Sandbox = "workspace"
	cfg.Hooks.SessionStart = []config.Hook{{Command: fmt.Sprintf("echo pwned > %q", marker)}}
	_ = FireSessionStart(context.Background(), cfg, "abc")
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("hook wrote outside workdir")
	}
}

func TestHookSandboxAllowsAdditionalDirWrite(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("workspace sandbox is darwin/linux")
	}
	extra := t.TempDir()
	marker := filepath.Join(extra, "ok")
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	cfg.Sandbox = "workspace"
	cfg.ExtraDirs = []string{extra}
	cfg.Hooks.SessionStart = []config.Hook{{Command: fmt.Sprintf("echo ok > %q", marker)}}
	if err := FireSessionStart(context.Background(), cfg, "abc"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(marker)
	if err != nil || !strings.Contains(string(data), "ok") {
		t.Fatalf("extra write %q %v", data, err)
	}
}

func TestHookStdinRedactsSecrets(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "hook.json")
	reg := New(dir, nil, AlwaysAllow)
	reg.SetHooks(config.Hooks{PostTool: []config.Hook{{
		Command: fmt.Sprintf("cat > %q", out),
	}}})
	in, _ := json.Marshal(map[string]string{"command": "echo sk-abcdefghijklmnopqrst"})
	if _, err := reg.Run(context.Background(), "bash", in); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sk-abcdefghijklmnopqrst") {
		t.Fatalf("hook stdin leaked: %s", data)
	}
	if !strings.Contains(string(data), "[redacted]") {
		t.Fatalf("hook stdin missing redaction: %s", data)
	}
}

func TestHookScrubsSecrets(t *testing.T) {
	t.Setenv("UPSTAGE_API_KEY", "up_must_not_appear")
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	reg.SetHooks(config.Hooks{PostTool: []config.Hook{{
		Command: "printenv UPSTAGE_API_KEY; echo ok",
	}}})
	in, _ := json.Marshal(map[string]any{"todos": []map[string]string{
		{"id": "1", "content": "x", "status": "pending"},
	}})
	out, err := reg.Run(context.Background(), "todo_write", in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "up_must_not_appear") {
		t.Fatalf("leaked: %q", out)
	}
}
