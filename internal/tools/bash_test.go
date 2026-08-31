package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestScrubEnvDropsKeys(t *testing.T) {
	got := scrubEnv([]string{
		"PATH=/usr/bin",
		"HOME=/tmp",
		"UPSTAGE_API_KEY=up_secret",
		"GOPPI_API_KEY=up_other",
		"GITHUB_TOKEN=ghp_x",
		"AWS_SECRET_ACCESS_KEY=w",
		"LANG=C",
	})
	joined := strings.Join(got, "\n")
	for _, leak := range []string{"UPSTAGE_API_KEY", "GOPPI_API_KEY", "GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY"} {
		if strings.Contains(joined, leak) {
			t.Fatalf("leaked %s in %q", leak, joined)
		}
	}
	if !strings.Contains(joined, "PATH=") || !strings.Contains(joined, "HOME=") {
		t.Fatalf("kept env %q", joined)
	}
}

func TestBashDoesNotExposeAPIKey(t *testing.T) {
	t.Setenv("UPSTAGE_API_KEY", "up_must_not_appear")
	t.Setenv("GOPPI_API_KEY", "up_also_hidden")
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	in, _ := json.Marshal(map[string]string{"command": "printenv UPSTAGE_API_KEY; printenv GOPPI_API_KEY; echo ok"})
	out, err := reg.Run(context.Background(), "bash", in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "up_must_not_appear") || strings.Contains(out, "up_also_hidden") {
		t.Fatalf("key leaked: %q", out)
	}
}

func TestBashCancelReturnsQuickly(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	in, _ := json.Marshal(map[string]string{"command": "sleep 30"})
	start := time.Now()
	_, err := reg.Run(ctx, "bash", in)
	if time.Since(start) > 4*time.Second {
		t.Fatalf("cancel took %s", time.Since(start))
	}
	if err == nil {
		t.Fatal("expected cancel or kill error")
	}
}

func TestBashSandboxBlocksWriteOutside(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home")
	}
	outside := filepath.Join(home, ".goppi-sandbox-test-"+strconv.Itoa(os.Getpid()))
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outside) })
	marker := filepath.Join(outside, "pwned")
	reg := New(t.TempDir(), nil, nil)
	in, _ := json.Marshal(map[string]string{"command": fmt.Sprintf("echo pwned > %q", marker)})
	_, _ = reg.Run(context.Background(), "bash", in)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("workspace sandbox must not write outside workdir/temp/cache")
	}
}

func TestBashSandboxOffAllowsWriteOutside(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home")
	}
	outside := filepath.Join(home, ".goppi-sandbox-off-"+strconv.Itoa(os.Getpid()))
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outside) })
	marker := filepath.Join(outside, "ok")
	reg := New(t.TempDir(), nil, nil)
	reg.SetSandbox("off")
	in, _ := json.Marshal(map[string]string{"command": fmt.Sprintf("echo ok > %q", marker)})
	if _, err := reg.Run(context.Background(), "bash", in); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("sandbox=off should write outside")
	}
}

func TestBashSandboxBlocksSudo(t *testing.T) {
	if _, err := exec.LookPath("sudo"); err != nil {
		t.Skip("no sudo")
	}
	in, _ := json.Marshal(map[string]any{"command": "sudo -n true", "timeout_sec": 5})
	off := New(t.TempDir(), nil, nil)
	off.SetSandbox("off")
	out, err := off.Run(context.Background(), "bash", in)
	if err != nil || (out != "(ok, no output)" && !strings.HasPrefix(out, "(ok")) {
		t.Skip("sudo -n is not passwordless; cannot prove the sandbox blocked it")
	}
	reg := New(t.TempDir(), nil, nil)
	out, err = reg.Run(context.Background(), "bash", in)
	if err == nil && (out == "(ok, no output)" || out == "") {
		t.Fatalf("workspace sandbox allowed sudo: %q", out)
	}
}

func TestBashSandboxAllowsAdditionalDirWrite(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("workspace sandbox is darwin/linux")
	}
	extra := t.TempDir()
	reg := New(t.TempDir(), nil, nil)
	reg.SetExtraDirs([]string{extra})
	marker := filepath.Join(extra, "ok")
	in, _ := json.Marshal(map[string]string{"command": fmt.Sprintf("echo ok > %q", marker)})
	if _, err := reg.Run(context.Background(), "bash", in); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(marker)
	if err != nil || !strings.Contains(string(data), "ok") {
		t.Fatalf("extra write %q %v", data, err)
	}
}

func TestBashSandboxAllowsWorkdirWrite(t *testing.T) {
	wd := t.TempDir()
	reg := New(wd, nil, nil)
	in, _ := json.Marshal(map[string]string{"command": "echo inside > in.txt"})
	if _, err := reg.Run(context.Background(), "bash", in); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(wd, "in.txt"))
	if err != nil || !strings.Contains(string(data), "inside") {
		t.Fatalf("workdir write %q %v", data, err)
	}
}

func TestBashStrictBlocksLocalTCP(t *testing.T) {
	ln, got := localListener(t)
	defer ln.Close()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	reg := New(t.TempDir(), nil, nil)
	reg.SetSandbox("strict")
	in, _ := json.Marshal(map[string]any{
		"command":     fmt.Sprintf("echo > /dev/tcp/%s/%s", host, port),
		"timeout_sec": 2,
	})
	_, _ = reg.Run(context.Background(), "bash", in)
	select {
	case <-got:
		t.Fatal("strict sandbox must not reach the local listener")
	default:
	}
}

func TestBashWorkspaceAllowsLocalTCP(t *testing.T) {
	ln, got := localListener(t)
	defer ln.Close()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	reg := New(t.TempDir(), nil, nil)
	in, _ := json.Marshal(map[string]any{
		"command":     fmt.Sprintf("echo > /dev/tcp/%s/%s", host, port),
		"timeout_sec": 2,
	})
	_, _ = reg.Run(context.Background(), "bash", in)
	select {
	case <-got:
	case <-time.After(3 * time.Second):
		t.Fatal("workspace sandbox should still reach localhost")
	}
}

func localListener(t *testing.T) (net.Listener, <-chan struct{}) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan struct{}, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		_ = c.Close()
		got <- struct{}{}
	}()
	return ln, got
}

func TestBashBackgroundEcho(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	t.Cleanup(reg.Close)
	in, _ := json.Marshal(map[string]any{"command": "echo hello-bg", "background": true})
	out, err := reg.Run(context.Background(), "bash", in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "started job 1") {
		t.Fatalf("%q", out)
	}
	poll, _ := json.Marshal(map[string]any{"id": 1, "wait_sec": 2})
	got, err := reg.Run(context.Background(), "bash_poll", poll)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "hello-bg") || !strings.Contains(got, "exited") {
		t.Fatalf("%q", got)
	}
}

func TestBashBackgroundKill(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	t.Cleanup(reg.Close)
	in, _ := json.Marshal(map[string]any{"command": "sleep 30", "background": true})
	out, err := reg.Run(context.Background(), "bash", in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "started job") {
		t.Fatalf("%q", out)
	}
	kill, _ := json.Marshal(map[string]int{"id": 1})
	got, err := reg.Run(context.Background(), "bash_kill", kill)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "exited") && !strings.Contains(got, "signal") && !strings.Contains(got, "killed") {
		t.Fatalf("%q", got)
	}
}

func TestBashBackgroundMaxJobs(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	t.Cleanup(reg.Close)
	for i := 0; i < maxBgJobs; i++ {
		in, _ := json.Marshal(map[string]any{"command": "sleep 30", "background": true})
		if _, err := reg.Run(context.Background(), "bash", in); err != nil {
			t.Fatal(err)
		}
	}
	in, _ := json.Marshal(map[string]any{"command": "sleep 30", "background": true})
	_, err := reg.Run(context.Background(), "bash", in)
	if err == nil || !strings.Contains(err.Error(), "background jobs") {
		t.Fatalf("got %v", err)
	}
}

func TestResetRuntimeKillsBackground(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	t.Cleanup(reg.Close)
	in, _ := json.Marshal(map[string]any{"command": "sleep 30", "background": true})
	if _, err := reg.Run(context.Background(), "bash", in); err != nil {
		t.Fatal(err)
	}
	run, _ := reg.JobCounts()
	if run != 1 {
		t.Fatalf("running %d", run)
	}
	reg.ResetRuntime()
	run, total := reg.JobCounts()
	if run != 0 || total != 0 {
		t.Fatalf("after reset running=%d total=%d", run, total)
	}
	_, err := reg.Run(context.Background(), "bash_poll", json.RawMessage(`{"id":1}`))
	if err == nil || !strings.Contains(err.Error(), "unknown job") {
		t.Fatalf("got %v", err)
	}
}

func TestBashCloseKillsBackground(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	in, _ := json.Marshal(map[string]any{"command": "sleep 30", "background": true})
	if _, err := reg.Run(context.Background(), "bash", in); err != nil {
		t.Fatal(err)
	}
	reg.Close()
	_, err := reg.Run(context.Background(), "bash_poll", json.RawMessage(`{"id":1}`))
	if err == nil || !strings.Contains(err.Error(), "unknown job") {
		t.Fatalf("got %v", err)
	}
}

func TestJobSummaryOmitsOutput(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	t.Cleanup(reg.Close)
	in, _ := json.Marshal(map[string]any{"command": "echo secret-output", "background": true})
	if _, err := reg.Run(context.Background(), "bash", in); err != nil {
		t.Fatal(err)
	}
	poll, _ := json.Marshal(map[string]any{"id": 1, "wait_sec": 2})
	got, err := reg.Run(context.Background(), "bash_poll", poll)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "secret-output") {
		t.Fatalf("poll should keep output: %q", got)
	}
	sum := reg.JobSummary()
	if strings.Contains(sum, "secret-output") {
		t.Fatalf("summary leaked output: %q", sum)
	}
	if !strings.Contains(sum, "job 1") {
		t.Fatalf("%q", sum)
	}
	run, total := reg.JobCounts()
	if total != 1 || run != 0 {
		t.Fatalf("counts %d/%d", run, total)
	}
}

func TestPlanModeDeniesBashKill(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	reg.SetMode("plan")
	_, err := reg.Run(context.Background(), "bash_kill", json.RawMessage(`{"id":1}`))
	if err == nil || !strings.Contains(err.Error(), "plan mode") {
		t.Fatalf("got %v", err)
	}
}

func TestBashTimeoutCapped(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	in, _ := json.Marshal(map[string]any{"command": "echo hi", "timeout_sec": 9999})
	out, err := reg.Run(context.Background(), "bash", in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hi") {
		t.Fatalf("got %q", out)
	}
}
