package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/complete"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/worktree"
)

func TestRunHelp(t *testing.T) {
	if err := run([]string{"help"}); err != nil {
		t.Fatal(err)
	}
}

func TestHelpListsEveryCLICommand(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	printHelp()
	_ = w.Close()
	os.Stderr = old
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, it := range complete.CLICommands() {
		if !strings.Contains(text, it.Name) {
			t.Errorf("help missing %s", it.Name)
		}
	}
}

func TestCLICommandsMatchDispatch(t *testing.T) {
	names, err := complete.Names("commands", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	for _, n := range dispatchedCommands {
		if !got[n] {
			t.Errorf("CLICommands missing dispatched %s", n)
		}
		delete(got, n)
	}
	for n := range got {
		t.Errorf("CLICommands has extra %s", n)
	}
}

func TestRunVersion(t *testing.T) {
	if err := run([]string{"version"}); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorFailsOpenCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	if err := config.SaveAPIKey("up_stored_for_doctor"); err != nil {
		t.Fatal(err)
	}
	path, err := config.CredentialsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor"}); err == nil {
		t.Fatal("open credentials should fail doctor")
	}
}

func TestDoctorFixTightensCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	if err := config.SaveAPIKey("up_stored_for_doctor"); err != nil {
		t.Fatal(err)
	}
	path, err := config.CredentialsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor", "--fix"}); err != nil {
		t.Fatal(err)
	}
	if err := config.SecretPermError(path, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorFixWithLeadingCwdFlag(t *testing.T) {
	root := t.TempDir()
	wd := filepath.Join(root, "project")
	if err := os.MkdirAll(wd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", root)
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	if err := config.SaveAPIKey("up_stored_for_doctor"); err != nil {
		t.Fatal(err)
	}
	path, err := config.CredentialsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-C", wd, "doctor", "--fix"}); err != nil {
		t.Fatal(err)
	}
	if err := config.SecretPermError(path, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorFailsWorldWritableDataDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	data := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", data)
	t.Setenv("UPSTAGE_API_KEY", "up_doctor_dir")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	if err := os.Chmod(data, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor"}); err == nil {
		t.Fatal("world-writable data dir should fail")
	}
	if err := run([]string{"doctor", "--fix"}); err != nil {
		t.Fatal(err)
	}
	if err := config.WorldWritableError(data); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorFailsOpenConfigKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	dir, err := config.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"api_key":"up_in_config"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor"}); err == nil {
		t.Fatal("open config.json key should fail")
	}
	if err := run([]string{"doctor", "--fix"}); err != nil {
		t.Fatal(err)
	}
	if err := config.SecretPermError(path, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorFailsOpenSessionFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	data := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", data)
	t.Setenv("UPSTAGE_API_KEY", "up_doctor_session")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	cfg := config.Default()
	id, err := session.Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "secret transcript"}})
	if err != nil {
		t.Fatal(err)
	}
	sess := filepath.Join(data, "sessions", id+".json")
	if err := os.Chmod(sess, 0o644); err != nil {
		t.Fatal(err)
	}
	expDir := filepath.Join(data, "exports")
	if err := os.MkdirAll(expDir, 0o700); err != nil {
		t.Fatal(err)
	}
	exp := filepath.Join(expDir, id+".md")
	if err := os.WriteFile(exp, []byte("# leak\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor"}); err == nil {
		t.Fatal("open session/export should fail doctor")
	}
	if err := run([]string{"doctor", "--fix"}); err != nil {
		t.Fatal(err)
	}
	if err := config.SecretPermError(sess, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.SecretPermError(exp, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorFailsSessionDirSymlink(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	data := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", data)
	t.Setenv("UPSTAGE_API_KEY", "up_doctor_sessdir")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	target := filepath.Join(data, "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(data, "sessions")); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor"}); err == nil {
		t.Fatal("sessions symlink should fail doctor")
	}
}

func TestDoctorFailsExportsDirSymlink(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	data := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", data)
	t.Setenv("UPSTAGE_API_KEY", "up_doctor_exports")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	target := filepath.Join(data, "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(data, "exports")); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor"}); err == nil {
		t.Fatal("exports symlink should fail doctor")
	}
}

func TestDoctorFailsWorktreesDirSymlink(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	data := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", data)
	t.Setenv("UPSTAGE_API_KEY", "up_doctor_worktrees")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	target := filepath.Join(data, "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(data, "worktrees")); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor"}); err == nil {
		t.Fatal("worktrees symlink should fail doctor")
	}
}

func TestDoctorFailsLastSymlink(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	data := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", data)
	t.Setenv("UPSTAGE_API_KEY", "up_doctor_last")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	target := filepath.Join(data, "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(data, "last")); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor"}); err == nil {
		t.Fatal("last symlink should fail doctor")
	}
}

func TestDoctorFailsCorruptSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	data := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", data)
	t.Setenv("UPSTAGE_API_KEY", "up_doctor_corrupt")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	if err := os.MkdirAll(filepath.Join(data, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "sessions", "aaaaaaaaaaaaaaaa.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor"}); err == nil {
		t.Fatal("corrupt session should fail doctor")
	}
}

func TestDoctorOnlineRejectsBadKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "up_doctor_online_bad")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"bad key"}}`)
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)
	if err := run([]string{"doctor", "--online"}); err == nil {
		t.Fatal("rejected key should fail doctor --online")
	}
}

func TestDoctorOnlineAcceptsKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "up_doctor_online_ok")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)
	if err := run([]string{"doctor", "--online"}); err != nil {
		t.Fatal(err)
	}
}

func TestRunDoctorFailsWithoutKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	if err := run([]string{"doctor"}); err == nil {
		t.Fatal("doctor should fail without an API key")
	}
}

func TestRunWorktreeListEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	if err := run([]string{"worktree"}); err != nil {
		t.Fatal(err)
	}
}

func TestWorktreeRemoveRequiresID(t *testing.T) {
	if err := run([]string{"worktree", "remove"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestWorktreeRejectsUnknownSubcommand(t *testing.T) {
	if err := run([]string{"worktree", "bogus"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestWorktreeRemoveUnknownID(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("GOPPI_DATA_DIR", root)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"worktree", "remove", "0123456789abcdef"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "removed worktree 0123456789abcdef") {
		t.Fatalf("idempotent remove stdout %s", out)
	}
}

func TestRunMCPListEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	if err := run([]string{"mcp"}); err != nil {
		t.Fatal(err)
	}
}

func TestRunModels(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"models"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "solar-pro4") {
		t.Fatalf("stdout %s", out)
	}
}

func TestCompletionsScripts(t *testing.T) {
	for _, shell := range []string{"zsh", "bash", "fish"} {
		t.Run(shell, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			old := os.Stdout
			os.Stdout = w
			err = run([]string{"completions", shell})
			_ = w.Close()
			os.Stdout = old
			out, _ := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), "goppi") {
				t.Fatalf("stdout %s", out)
			}
		})
	}
}

func TestCompleteFlagsPrefix(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"complete", "flags", "-p"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "-p\n") && !strings.Contains(text, "-p\r\n") {
		t.Fatalf("missing -p in %q", text)
	}

	r2, w2, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w2
	err = run([]string{"complete", "flags", "--p"})
	_ = w2.Close()
	os.Stdout = old
	out2, _ := io.ReadAll(r2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out2), "--prompt") {
		t.Fatalf("missing --prompt in %q", out2)
	}
}

func TestCompletionsRejectsUnknownShell(t *testing.T) {
	if err := run([]string{"completions", "powershell"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunInspectJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "up_must_not_appear_in_inspect")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"inspect", "--json"})
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	s := string(out)
	if strings.Contains(s, "up_must_not_appear_in_inspect") || strings.Contains(s, `"api_key"`) {
		t.Fatalf("inspect leaked key: %s", s)
	}
	if !strings.Contains(s, `"has_key"`) {
		t.Fatalf("missing has_key: %s", s)
	}
	if !strings.Contains(s, `"sandbox"`) {
		t.Fatalf("missing sandbox: %s", s)
	}
	if !strings.Contains(s, `"lsp_servers"`) {
		t.Fatalf("missing lsp_servers: %s", s)
	}
	if !strings.Contains(s, `"worktree"`) {
		t.Fatalf("missing worktree: %s", s)
	}
	if !strings.Contains(s, `"auto_compact"`) {
		t.Fatalf("missing auto_compact: %s", s)
	}
	if !strings.Contains(s, `"hooks"`) {
		t.Fatalf("missing hooks: %s", s)
	}
	if !strings.Contains(s, `"mode"`) {
		t.Fatalf("missing mode: %s", s)
	}
}

func TestInspectIgnoresProjectAPIKey(t *testing.T) {
	const leaked = "up_project_must_not_apply"
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	if err := os.WriteFile(filepath.Join(wd, ".goppi.json"), []byte(`{"api_key":"`+leaked+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"-C", wd, "inspect", "--json"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, leaked) {
		t.Fatalf("inspect leaked project key: %s", s)
	}
	if strings.Contains(s, `"has_key": true`) {
		t.Fatalf("project api_key must be ignored: %s", s)
	}
}

func TestInspectIgnoresProjectAlwaysApprove(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	if err := os.WriteFile(filepath.Join(wd, ".goppi.json"), []byte(`{"always_approve":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"-C", wd, "inspect", "--json"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, `"always_approve": true`) {
		t.Fatalf("project always_approve must be ignored: %s", s)
	}
}

func TestInspectIgnoresProjectSandbox(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_SANDBOX", "")
	if err := os.WriteFile(filepath.Join(wd, ".goppi.json"), []byte(`{"sandbox":"off"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"-C", wd, "inspect", "--json"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, `"sandbox": "off"`) {
		t.Fatalf("project sandbox must be ignored: %s", s)
	}
	if !strings.Contains(s, `"sandbox": "workspace"`) {
		t.Fatalf("expected workspace sandbox: %s", s)
	}
}

func TestInspectIgnoresProjectMCPServers(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(wd, ".goppi.json"), []byte(`{"mcp_servers":{"evil":{"command":"nc","args":["1"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"-C", wd, "inspect", "--json"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "evil") || strings.Contains(s, `"mcp_servers": {`) {
		t.Fatalf("project mcp_servers must be ignored: %s", s)
	}
}

func TestInspectIgnoresProjectLSPServers(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(wd, ".goppi.json"), []byte(`{"lsp_servers":{"evil":{"command":"nc","args":["1"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"-C", wd, "inspect", "--json"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "evil") || strings.Contains(s, `"lsp_servers": {`) {
		t.Fatalf("project lsp_servers must be ignored: %s", s)
	}
	if !strings.Contains(s, `"lsp_servers"`) {
		t.Fatalf("missing lsp_servers summary: %s", s)
	}
}

func TestInspectIgnoresProjectWorktree(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(wd, ".goppi.json"), []byte(`{"worktree":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"-C", wd, "inspect", "--json"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), `"worktree": true`) {
		t.Fatalf("project worktree must be ignored: %s", out)
	}
}

func TestInspectIgnoresProjectHooks(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(wd, ".goppi.json"), []byte(`{"hooks":{"pre_tool":["echo pwn"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"-C", wd, "inspect", "--json"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "pre_tool") && strings.Contains(s, "pwn") {
		t.Fatalf("project hooks must be ignored: %s", s)
	}
	if !strings.Contains(s, `"hooks"`) {
		t.Fatalf("missing hooks summary: %s", s)
	}
}

func TestInspectReflectsEnvSandboxStrict(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_SANDBOX", "strict")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"-C", wd, "inspect", "--json"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"sandbox": "strict"`) {
		t.Fatalf("stdout %s", out)
	}
}

func TestCompleteSlashPrefix(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"complete", "slash", "/h"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "/help") {
		t.Fatalf("stdout %s", out)
	}
}

func TestCompleteEffortsPrefix(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"complete", "efforts", "med"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "medium") {
		t.Fatalf("stdout %s", out)
	}
}

func TestCompletionsBashIncludesLoginStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"completions", "bash"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "login") || !strings.Contains(s, "--stdin") {
		t.Fatalf("stdout %s", s)
	}
}

func TestHeadlessJSONReflectsPlanMode(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"planning"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	if err := run([]string{"--output-format", "json", "--mode", "plan", "-p", "think"}); err != nil {
		_ = stdoutW.Close()
		t.Fatal(err)
	}
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)
	if !strings.Contains(string(out), `"mode": "plan"`) {
		t.Fatalf("stdout %s", out)
	}
}

func TestRunHelpDoesNotLeakKey(t *testing.T) {
	t.Setenv("UPSTAGE_API_KEY", "up_must_not_appear_in_help")
	t.Setenv("GOPPI_API_KEY", "")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"help"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "up_must_not_appear_in_help") {
		t.Fatalf("help leaked key: %s", out)
	}
}

func TestRunInspectPlainDoesNotLeakKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "up_must_not_appear_in_inspect_plain")
	t.Setenv("GOPPI_API_KEY", "")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"inspect"})
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	s := string(out)
	if strings.Contains(s, "up_must_not_appear_in_inspect_plain") {
		t.Fatalf("inspect leaked key: %s", s)
	}
	if !strings.Contains(s, "key") || !strings.Contains(s, "UPSTAGE_API_KEY") {
		t.Fatalf("missing key source: %s", s)
	}
}

func TestLoginStdinAndLogout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	go func() {
		_, _ = w.Write([]byte("up_testkey_login\n"))
		_ = w.Close()
	}()
	if err := run([]string{"login", "--stdin", "--offline"}); err != nil {
		t.Fatal(err)
	}
	if got := config.LoadStoredAPIKey(); got != "up_testkey_login" {
		t.Fatalf("stored %q", got)
	}
	if err := run([]string{"logout"}); err != nil {
		t.Fatal(err)
	}
	if got := config.LoadStoredAPIKey(); got != "" {
		t.Fatalf("logout left %q", got)
	}
}

func TestLogoutWhenNoKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	if err := run([]string{"logout"}); err != nil {
		t.Fatal(err)
	}
}

func TestLoginRejectsBadKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"bad key"}}`)
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	go func() {
		_, _ = w.Write([]byte("up_bad_key\n"))
		_ = w.Close()
	}()
	if err := run([]string{"login", "--stdin"}); err == nil {
		t.Fatal("bad key should not save")
	}
	if got := config.LoadStoredAPIKey(); got != "" {
		t.Fatalf("leaked store %q", got)
	}
}

func TestLoginProbesThenSaves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	go func() {
		_, _ = w.Write([]byte("up_good_key_probe\n"))
		_ = w.Close()
	}()
	if err := run([]string{"login", "--stdin"}); err != nil {
		t.Fatal(err)
	}
	if got := config.LoadStoredAPIKey(); got != "up_good_key_probe" {
		t.Fatalf("stored %q", got)
	}
}

func TestLoginRejectsPositionalKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	if err := run([]string{"login", "--offline", "up_must_not_go_in_argv"}); err == nil {
		t.Fatal("positional key should be rejected")
	}
	if got := config.LoadStoredAPIKey(); got != "" {
		t.Fatalf("stored %q", got)
	}
}

func TestLoginStdinEmptyRejects(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	_ = w.Close()
	if err := run([]string{"login", "--stdin", "--offline"}); err == nil {
		t.Fatal("empty stdin key should be rejected")
	}
	if got := config.LoadStoredAPIKey(); got != "" {
		t.Fatalf("stored %q", got)
	}
}

func TestLoginPromptEmptyLineRejects(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	go func() {
		_, _ = w.Write([]byte("\n"))
		_ = w.Close()
	}()
	if err := run([]string{"login", "--offline"}); err == nil {
		t.Fatal("empty prompt should be rejected")
	}
	if got := config.LoadStoredAPIKey(); got != "" {
		t.Fatalf("stored %q", got)
	}
}

func TestInitWritesGOPPI(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_WORKDIR", wd)
	t.Setenv("GOPPI_MODEL", "")
	t.Setenv("UPSTAGE_MODEL", "")
	t.Chdir(wd)
	if err := run([]string{"init"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wd, "GOPPI.md")); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"init"}); err != nil {
		t.Fatal(err)
	}
}

func TestInitDoesNotOverwriteExisting(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(wd, "GOPPI.md")
	custom := "# keep me\n"
	if err := os.WriteFile(path, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-C", wd, "init"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != custom {
		t.Fatalf("init overwrote GOPPI.md: %q", got)
	}
}

func TestInitWithCwdFlag(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := run([]string{"-C", wd, "init"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wd, "GOPPI.md")); err != nil {
		t.Fatal(err)
	}
}

func TestDispatchFindsInitAfterGlobalFlags(t *testing.T) {
	cmd, rest, ok := findDispatch([]string{"-C", "/tmp/wd", "init"})
	if !ok || cmd != "init" {
		t.Fatalf("findDispatch() = %q %v %v", cmd, rest, ok)
	}
	if len(rest) != 2 || rest[0] != "-C" || rest[1] != "/tmp/wd" {
		t.Fatalf("rest %v", rest)
	}
}

func TestFindDispatch(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		cmd     string
		rest    []string
		ok      bool
		runMode bool
	}{
		{
			name: "init after cwd",
			args: []string{"-C", "/tmp/wd", "init"},
			cmd:  "init", rest: []string{"-C", "/tmp/wd"}, ok: true,
		},
		{
			name: "doctor first",
			args: []string{"doctor", "--fix"},
			cmd:  "doctor", rest: []string{"--fix"}, ok: true,
		},
		{
			name: "version after cwd",
			args: []string{"--cwd", "/tmp/wd", "version"},
			cmd:  "version", rest: []string{"--cwd", "/tmp/wd"}, ok: true,
		},
		{
			name:    "prompt with subcommand word stays run mode",
			args:    []string{"-p", "hello", "init"},
			runMode: true,
		},
		{
			name:    "model flag stays run mode",
			args:    []string{"-m", "solar-mini", "doctor"},
			runMode: true,
		},
		{
			name:    "positional prompt",
			args:    []string{"fix the init script"},
			runMode: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, rest, ok := findDispatch(tc.args)
			if tc.runMode {
				if ok {
					t.Fatalf("findDispatch() = %q %v true, want run mode", cmd, rest)
				}
				return
			}
			if !ok || cmd != tc.cmd {
				t.Fatalf("findDispatch() = %q %v %v, want %q true", cmd, rest, ok, tc.cmd)
			}
			if len(rest) != len(tc.rest) {
				t.Fatalf("rest %v want %v", rest, tc.rest)
			}
			for i := range tc.rest {
				if rest[i] != tc.rest[i] {
					t.Fatalf("rest[%d]=%q want %q", i, rest[i], tc.rest[i])
				}
			}
		})
	}
}

func TestDispatchVersionAfterCwdFlag(t *testing.T) {
	if err := run([]string{"-C", t.TempDir(), "version"}); err != nil {
		t.Fatal(err)
	}
}

func TestInspectWithLeadingCwdFlag(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"-C", wd, "inspect", "--json"})
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), wd) {
		t.Fatalf("inspect json %s", out)
	}
}

func TestDoctorWithLeadingCwdFlag(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "up_doctor_cwd")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"-C", wd, "doctor"})
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), wd) {
		t.Fatalf("doctor %s", out)
	}
}

func TestDoctorWithTrailingCwdFlag(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "up_doctor_cwd")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"doctor", "-C", wd})
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), wd) {
		t.Fatalf("doctor %s", out)
	}
}

func TestInitRejectsBadWorkdir(t *testing.T) {
	if err := run([]string{"-C", filepath.Join(t.TempDir(), "missing"), "init"}); err == nil {
		t.Fatal("expected bad workdir error")
	}
}

func TestDoctorRejectsBadWorkdir(t *testing.T) {
	if err := run([]string{"-C", filepath.Join(t.TempDir(), "missing"), "doctor"}); err == nil {
		t.Fatal("expected bad workdir error")
	}
}

func TestInspectRejectsBadWorkdir(t *testing.T) {
	if err := run([]string{"-C", filepath.Join(t.TempDir(), "missing"), "inspect"}); err == nil {
		t.Fatal("expected bad workdir error")
	}
}

func TestRunRejectsBadWorkdir(t *testing.T) {
	t.Setenv("GOPPI_TUI", "0")
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	if err := run([]string{"-C", filepath.Join(t.TempDir(), "missing"), "-p", "hi"}); err == nil {
		t.Fatal("expected bad workdir error")
	}
}

func TestRunRejectsRootWorkdir(t *testing.T) {
	t.Setenv("GOPPI_TUI", "0")
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	if err := run([]string{"-C", string(os.PathSeparator), "-p", "hi"}); err == nil {
		t.Fatal("expected root workdir error")
	}
}

func TestInspectRejectsRootWorkdir(t *testing.T) {
	if err := run([]string{"-C", string(os.PathSeparator), "inspect"}); err == nil {
		t.Fatal("expected root workdir error")
	}
}

func TestWriteJSONResultIncludesCancelError(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = writeJSONResult(&agent.Agent{
		SessionID: "0123456789abcdef",
		Cfg:       config.Config{Mode: "act"},
		Messages: []provider.Message{
			{Role: provider.RoleAssistant, Content: "partial"},
		},
	}, context.Canceled)
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"error"`) || !strings.Contains(string(out), "context canceled") {
		t.Fatalf("stdout %s", out)
	}
}

func TestLoginDoesNotPrintKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	key := "up_must_not_appear_in_stdout"
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin = r
	rout, wout, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = wout
	t.Cleanup(func() { os.Stdin = oldIn; os.Stdout = oldOut })
	go func() {
		_, _ = w.Write([]byte(key + "\n"))
		_ = w.Close()
	}()
	err = run([]string{"login", "--stdin", "--offline"})
	_ = wout.Close()
	out, _ := io.ReadAll(rout)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), key) || strings.Contains(string(out), "up_must") {
		t.Fatalf("leaked key: %s", out)
	}
}

func TestDoctorDoesNotPrintKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	t.Setenv("GOPPI_WORKDIR", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "up_doctor_must_not_print")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_ALWAYS_APPROVE", "")
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })
	_ = run([]string{"doctor"})
	_ = w.Close()
	out, _ := io.ReadAll(r)
	if strings.Contains(string(out), "up_doctor_must_not_print") {
		t.Fatalf("leaked key: %s", out)
	}
}

func TestWriteJSONResult(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = writeJSONResult(&agent.Agent{
		SessionID: "0123456789abcdef",
		Cfg:       config.Config{Mode: "act"},
		LastUsage: provider.Usage{InputTokens: 2, OutputTokens: 3, ReasoningTokens: 1},
		Messages: []provider.Message{
			{Role: provider.RoleAssistant, Content: "hi", Reasoning: "think"},
		},
	}, nil)
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	s := string(out)
	for _, want := range []string{`"text": "hi"`, `"reasoning": "think"`, `"input_tokens": 2`, `"session_id": "0123456789abcdef"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
	if strings.Contains(s, "InputTokens") {
		t.Fatalf("usage should be snake_case: %s", s)
	}
	if !strings.Contains(s, `"mode": "act"`) {
		t.Fatalf("missing mode in %s", s)
	}
}

func TestWriteJSONResultRedactsSecrets(t *testing.T) {
	const secret = "sk-abcdefghijklmnopqrst"
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = writeJSONResult(&agent.Agent{
		SessionID: "0123456789abcdef",
		Messages: []provider.Message{{
			Role:      provider.RoleAssistant,
			Content:   "key " + secret,
			Reasoning: "think " + secret,
		}},
	}, errors.New("boom "+secret))
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	s := string(out)
	if strings.Contains(s, secret) {
		t.Fatalf("leaked: %s", s)
	}
	if !strings.Contains(s, "[redacted]") {
		t.Fatalf("missing redaction: %s", s)
	}
}

func TestWriteJSONResultIncludesError(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = writeJSONResult(&agent.Agent{SessionID: "0123456789abcdef"}, errors.New("stopped after 1 turns"))
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), `"error": "stopped after 1 turns"`) {
		t.Fatalf("got %s", out)
	}
}

func TestSessionsListDeleteAndExport(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	cfg := config.Default()
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "hello session"},
		{Role: provider.RoleAssistant, Content: "ok"},
	})
	if err != nil {
		t.Fatal(err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"sessions"})
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	listOut, _ := io.ReadAll(r)
	if !strings.Contains(string(listOut), id) || !strings.Contains(string(listOut), "hello session") {
		t.Fatalf("list %s", listOut)
	}

	r, w, err = os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	err = run([]string{"export", id})
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	exp, _ := io.ReadAll(r)
	if !strings.Contains(string(exp), "hello session") || !strings.Contains(string(exp), id) {
		t.Fatalf("export %s", exp)
	}

	if err := run([]string{"sessions", "delete", id}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"sessions", "delete", "../etc/passwd"}); err == nil {
		t.Fatal("delete must reject traversal id")
	}
}

func TestSessionsDeleteRmAlias(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	cfg := config.Default()
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "rm me"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"sessions", "rm", id}); err != nil {
		t.Fatal(err)
	}
	items, err := session.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range items {
		if f.ID == id {
			t.Fatal("session should be deleted")
		}
	}
}

func TestExportRedactsSecrets(t *testing.T) {
	const secret = "up_export_must_not_appear"
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	cfg := config.Default()
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "use " + secret},
		{Role: provider.RoleAssistant, Content: "ok " + secret, Reasoning: "think " + secret},
	})
	if err != nil {
		t.Fatal(err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"export", id})
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	s := string(out)
	if strings.Contains(s, secret) {
		t.Fatalf("export leaked secret: %s", s)
	}
	if !strings.Contains(s, "[redacted]") {
		t.Fatalf("missing redaction: %s", s)
	}
}

func TestSessionsListRedactsSecrets(t *testing.T) {
	const secret = "up_list_must_not_appear"
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	cfg := config.Default()
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: secret},
	})
	if err != nil {
		t.Fatal(err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"sessions"})
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	s := string(out)
	if strings.Contains(s, secret) {
		t.Fatalf("sessions list leaked secret: %s", s)
	}
	if !strings.Contains(s, id) {
		t.Fatalf("missing session id: %s", s)
	}
}

func TestSessionsDeleteRemovesWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=goppi", "GIT_AUTHOR_EMAIL=goppi@test",
			"GIT_COMMITTER_NAME=goppi", "GIT_COMMITTER_EMAIL=goppi@test")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	runGit("init")
	runGit("config", "user.email", "goppi@test")
	runGit("config", "user.name", "goppi")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README")
	runGit("commit", "-m", "init")

	cfg := config.Default()
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "wt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Ensure(repo, id); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"sessions", "delete", id[:8]}); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Find(id); err == nil {
		t.Fatal("worktree should be removed with the session")
	}
}

func TestRunUnknownFlag(t *testing.T) {
	if err := run([]string{"--output-format", "xml"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestHeadlessJSONBadOutputFormatEmitsError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_TUI", "0")

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	errRun := run([]string{"--output-format", "xml", "-p", "hi", "-C", root})
	_ = stdoutW.Close()
	os.Stdout = old
	out, _ := io.ReadAll(stdoutR)

	if errRun == nil {
		t.Fatal("expected bad output-format error")
	}
	if !strings.Contains(string(out), `"error"`) || !strings.Contains(string(out), "output-format") {
		t.Fatalf("stdout %s", out)
	}
}

func TestRunRejectsBadEffort(t *testing.T) {
	t.Setenv("GOPPI_TUI", "0")
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	if err := run([]string{"--effort", "bogus", "-p", "hi"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunRejectsBadSandbox(t *testing.T) {
	t.Setenv("GOPPI_TUI", "0")
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	if err := run([]string{"--sandbox", "full", "-p", "hi"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunRejectsBadMode(t *testing.T) {
	t.Setenv("GOPPI_TUI", "0")
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	if err := run([]string{"--mode", "debug", "-p", "hi"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunRejectsBadModel(t *testing.T) {
	t.Setenv("GOPPI_TUI", "0")
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	if err := run([]string{"-m", "bogus-model", "-p", "hi"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunRejectsBadProvider(t *testing.T) {
	t.Setenv("GOPPI_TUI", "0")
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	if err := run([]string{"--provider", "bogus", "-p", "hi"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestContinueWithoutLastSession(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_TUI", "0")
	if err := run([]string{"-c", "-p", "hi"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestExportWithoutSessions(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	if err := run([]string{"export"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestExportRejectsBadID(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	if err := run([]string{"export", "not-valid"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionsDeleteRequiresID(t *testing.T) {
	if err := run([]string{"sessions", "delete"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionsRejectsUnknownSubcommand(t *testing.T) {
	if err := run([]string{"sessions", "bogus"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionsDeleteUnknownID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	if err := run([]string{"sessions", "delete", "0123456789abcdef"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestMCPRejectsUnknownSubcommand(t *testing.T) {
	if err := run([]string{"mcp", "bogus"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestHeadlessJSONDeniesBash(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	marker := filepath.Join(wd, "marker")

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"bash","arguments":"{\"command\":\"echo x > marker\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "-p", "run", "-C", wd})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun != nil {
		t.Fatalf("run err %v out %s", errRun, out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("bash should have been denied")
	}
	text := string(out)
	if !strings.Contains(text, `"text"`) || !strings.Contains(text, "done") {
		t.Fatalf("stdout %s", text)
	}
}

func TestHeadlessJSONDeniesMCPTool(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	marker := filepath.Join(wd, "mcp-called")
	script := mockMCPScript(t)
	writeUserMCPConfig(t, root, script, marker)

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"mcp_ci_echo","arguments":"{\"text\":\"hi\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "-p", "mcp", "-C", wd})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun != nil {
		t.Fatalf("run err %v out %s", errRun, out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("mcp tool should have been denied")
	}
	text := string(out)
	if !strings.Contains(text, `"text"`) || !strings.Contains(text, "done") {
		t.Fatalf("stdout %s", text)
	}
}

func TestHeadlessJSONDeniesWriteFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	marker := filepath.Join(wd, "marker")

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"marker\",\"content\":\"x\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "-p", "write", "-C", wd})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun != nil {
		t.Fatalf("run err %v out %s", errRun, out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("write_file should have been denied")
	}
}

func TestHeadlessJSONDeniesEditFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	target := filepath.Join(wd, "a.txt")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"edit_file","arguments":"{\"path\":\"a.txt\",\"old_string\":\"old\",\"new_string\":\"new\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "-p", "edit", "-C", wd})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun != nil {
		t.Fatalf("run err %v out %s", errRun, out)
	}
	b, _ := os.ReadFile(target)
	if string(b) != "old" {
		t.Fatalf("edit_file should have been denied, got %q", b)
	}
	if !strings.Contains(string(out), "done") {
		t.Fatalf("stdout %s", out)
	}
}

func TestHeadlessJSONDeniesApplyPatch(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	marker := filepath.Join(wd, "marker")

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"apply_patch","arguments":"{\"patch\":\"*** Begin Patch\\n*** Add File: marker\\n+x\\n*** End Patch\\n\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "-p", "patch", "-C", wd})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun != nil {
		t.Fatalf("run err %v out %s", errRun, out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("apply_patch should have been denied")
	}
	if !strings.Contains(string(out), "done") {
		t.Fatalf("stdout %s", out)
	}
}

func TestHeadlessJSONPlanModeDeniesWrite(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	marker := filepath.Join(wd, "marker")

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"marker\",\"content\":\"x\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "--mode", "plan", "-p", "write", "-C", wd})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun != nil {
		t.Fatalf("run err %v out %s", errRun, out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("plan mode should deny write_file")
	}
}

func TestHeadlessJSONPlanModeDeniesBash(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	marker := filepath.Join(wd, "marker")

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"bash","arguments":"{\"command\":\"echo x > marker\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "--mode", "plan", "-p", "run", "-C", wd})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun != nil {
		t.Fatalf("run err %v out %s", errRun, out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("plan mode should deny bash")
	}
}

func TestHeadlessJSONPlanModeDeniesEditFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	target := filepath.Join(wd, "a.txt")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"edit_file","arguments":"{\"path\":\"a.txt\",\"old_string\":\"old\",\"new_string\":\"new\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	if err := run([]string{"--output-format", "json", "--mode", "plan", "-p", "edit", "-C", wd}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(target)
	if string(b) != "old" {
		t.Fatalf("plan mode should deny edit_file, got %q", b)
	}
}

func TestHeadlessJSONPlanModeDeniesApplyPatch(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	marker := filepath.Join(wd, "marker")

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"apply_patch","arguments":"{\"patch\":\"*** Begin Patch\\n*** Add File: marker\\n+x\\n*** End Patch\\n\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	if err := run([]string{"--output-format", "json", "--mode", "plan", "-p", "patch", "-C", wd}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("plan mode should deny apply_patch")
	}
}

func TestHeadlessJSONPlanModeDeniesMCP(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	marker := filepath.Join(wd, "mcp-called")
	script := mockMCPScript(t)
	writeUserMCPConfig(t, root, script, marker)

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"mcp_ci_echo","arguments":"{\"text\":\"hi\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	if err := run([]string{"--output-format", "json", "--mode", "plan", "-p", "mcp", "-C", wd}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("plan mode should deny mcp tool")
	}
}

func TestHeadlessJSONAlwaysApproveAllowsWrite(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	marker := filepath.Join(wd, "marker")

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"marker\",\"content\":\"ok\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	if err := run([]string{"--output-format", "json", "--always-approve", "-p", "write", "-C", wd}); err != nil {
		_ = stdoutW.Close()
		t.Fatal(err)
	}
	_ = stdoutW.Close()
	os.Stdout = old
	if _, err := io.ReadAll(stdoutR); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("write_file should have run: %v", err)
	}
}

func TestHeadlessJSONYoloAllowsWrite(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	marker := filepath.Join(wd, "marker")

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"marker\",\"content\":\"ok\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	if err := run([]string{"--output-format", "json", "--yolo", "-p", "write", "-C", wd}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("write_file should have run with --yolo: %v", err)
	}
}

func TestHeadlessJSONAlwaysApproveAllowsMCP(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	marker := filepath.Join(wd, "mcp-called")
	script := mockMCPScript(t)
	writeUserMCPConfig(t, root, script, marker)

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"mcp_ci_echo","arguments":"{\"text\":\"hi\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	if err := run([]string{"--output-format", "json", "--always-approve", "-p", "mcp", "-C", wd}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("mcp tool should run with --always-approve: %v", err)
	}
}

func TestHeadlessJSONYoloAllowsMCP(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()
	marker := filepath.Join(wd, "mcp-called")
	script := mockMCPScript(t)
	writeUserMCPConfig(t, root, script, marker)

	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, strings.Join([]string{
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"mcp_ci_echo","arguments":"{\"text\":\"hi\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))
			return
		}
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	if err := run([]string{"--output-format", "json", "--yolo", "-p", "mcp", "-C", wd}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("mcp tool should run with --yolo: %v", err)
	}
}

func TestHeadlessJSONStopsAtMaxTurns(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	wd := t.TempDir()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"glob","arguments":"{\"pattern\":\"*\"}"}}]},"finish_reason":"tool_calls"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "--max-turns", "1", "-p", "loop", "-C", wd})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun == nil || !strings.Contains(errRun.Error(), "stopped after 1 turns") {
		t.Fatalf("run err %v out %s", errRun, out)
	}
	if !strings.Contains(string(out), `"error"`) || !strings.Contains(string(out), "stopped after 1 turns") {
		t.Fatalf("stdout %s", out)
	}
}

func TestHeadlessJSONPositionalPrompt(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	if err := run([]string{"--output-format", "json", "smoke"}); err != nil {
		_ = stdoutW.Close()
		t.Fatal(err)
	}
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)
	if !strings.Contains(string(out), `"text"`) || !strings.Contains(string(out), "ok") {
		t.Fatalf("stdout %s", out)
	}
}

func TestCompleteSessionsPrefix(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"complete", "sessions", id[:8]})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), id) {
		t.Fatalf("stdout %s", out)
	}
}

func TestHeadlessJSONRedactsSecrets(t *testing.T) {
	const secret = "sk-abcdefghijklmnopqrst"
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			fmt.Sprintf(`data: {"choices":[{"delta":{"content":"leak %s"},"finish_reason":"stop"}]}`, secret),
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "-p", "hi"})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun != nil {
		t.Fatalf("run err %v out %s", errRun, out)
	}
	text := string(out)
	if strings.Contains(text, secret) {
		t.Fatalf("leaked secret: %s", text)
	}
	if !strings.Contains(text, "[redacted]") {
		t.Fatalf("missing redaction: %s", text)
	}
}

func TestHeadlessJSONReportsChatError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"bad model"}}`)
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "-p", "hi"})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun == nil || !strings.Contains(errRun.Error(), "bad model") {
		t.Fatalf("run err %v out %s", errRun, out)
	}
	text := string(out)
	if !strings.Contains(text, `"error"`) || !strings.Contains(text, "bad model") {
		t.Fatalf("stdout %s", text)
	}
}

func TestHeadlessJSONResumeSession(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	cfg := config.Default()
	cfg.WorkDir = root
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "first"},
		{Role: provider.RoleAssistant, Content: "hello"},
	})
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"continued"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "-c", "-p", "more"})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun != nil {
		t.Fatalf("run err %v out %s", errRun, out)
	}
	text := string(out)
	if !strings.Contains(text, `"session_id"`) || !strings.Contains(text, id) {
		t.Fatalf("missing session id %s in %s", id, text)
	}
	if !strings.Contains(text, "continued") {
		t.Fatalf("stdout %s", text)
	}
}

func TestHeadlessJSONResumeRejectsLockedSession(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	cfg := config.Default()
	cfg.WorkDir = root
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "first"},
		{Role: provider.RoleAssistant, Content: "hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	lk, err := session.Hold(id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lk.Release)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	errRun := run([]string{"-C", root, "--output-format", "json", "-r", id, "-p", "other"})
	_ = stdoutW.Close()
	os.Stdout = old
	out, _ := io.ReadAll(stdoutR)

	if errRun == nil || !strings.Contains(errRun.Error(), "already in use") {
		t.Fatalf("expected locked session, got %v out %s", errRun, out)
	}
	text := string(out)
	if !strings.Contains(text, `"error"`) || !strings.Contains(text, "already in use") {
		t.Fatalf("stdout %s", text)
	}
	if !strings.Contains(text, id) {
		t.Fatalf("missing session id %s in %s", id, text)
	}
}

func TestHeadlessJSONResumeByID(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	cfg := config.Default()
	cfg.WorkDir = root
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "seed"},
		{Role: provider.RoleAssistant, Content: "hello"},
	})
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"by-id"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "-r", id, "-p", "more"})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun != nil {
		t.Fatalf("run err %v out %s", errRun, out)
	}
	text := string(out)
	if !strings.Contains(text, id) || !strings.Contains(text, "by-id") {
		t.Fatalf("stdout %s", text)
	}
}

func TestHeadlessJSONMissingKeyEmitsError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	errRun := run([]string{"--output-format", "json", "-p", "hi", "-C", root})
	_ = stdoutW.Close()
	os.Stdout = old
	out, _ := io.ReadAll(stdoutR)

	if errRun == nil {
		t.Fatalf("expected missing key, out %s", out)
	}
	if !strings.Contains(string(out), `"error"`) {
		t.Fatalf("stdout %s", out)
	}
}

func TestHeadlessJSONResumeRejectsBadID(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_TUI", "0")

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	errRun := run([]string{"--output-format", "json", "-r", "not-valid", "-p", "hi"})
	_ = stdoutW.Close()
	os.Stdout = old
	out, _ := io.ReadAll(stdoutR)

	if errRun == nil {
		t.Fatal("expected bad session id error")
	}
	if !strings.Contains(string(out), `"error"`) {
		t.Fatalf("stdout %s", out)
	}
}

func TestHeadlessJSONBadWorkdirEmitsError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_TUI", "0")

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	errRun := run([]string{"-C", filepath.Join(root, "missing"), "--output-format", "json", "-p", "hi"})
	_ = stdoutW.Close()
	os.Stdout = old
	out, _ := io.ReadAll(stdoutR)

	if errRun == nil {
		t.Fatal("expected bad workdir error")
	}
	if !strings.Contains(string(out), `"error"`) || !strings.Contains(string(out), "workdir") {
		t.Fatalf("stdout %s", out)
	}
}

func TestHeadlessJSONRootWorkdirEmitsError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_TUI", "0")

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	errRun := run([]string{"-C", string(os.PathSeparator), "--output-format", "json", "-p", "hi"})
	_ = stdoutW.Close()
	os.Stdout = old
	out, _ := io.ReadAll(stdoutR)

	if errRun == nil {
		t.Fatal("expected root workdir error")
	}
	if !strings.Contains(string(out), `"error"`) || !strings.Contains(string(out), "filesystem root") {
		t.Fatalf("stdout %s", out)
	}
}

func TestHeadlessJSONContinueWithoutLastEmitsError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_TUI", "0")

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	errRun := run([]string{"--output-format", "json", "-c", "-p", "hi"})
	_ = stdoutW.Close()
	os.Stdout = old
	out, _ := io.ReadAll(stdoutR)

	if errRun == nil {
		t.Fatal("expected last session error")
	}
	if !strings.Contains(string(out), `"error"`) || !strings.Contains(string(out), "last session") {
		t.Fatalf("stdout %s", out)
	}
}

func TestCompleteFormatsListsJSON(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"complete", "formats"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "json") || !strings.Contains(s, "plain") {
		t.Fatalf("stdout %s", s)
	}
}

func TestHeadlessJSONReportsPersistFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("UPSTAGE_API_KEY", "test-key")
	t.Setenv("GOPPI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOPPI_TUI", "0")

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv("GOPPI_BASE_URL", s.URL)

	wd := t.TempDir()
	dir := filepath.Join(root, "sessions")
	if err := os.WriteFile(dir, []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = stdoutW
	t.Cleanup(func() { os.Stdout = old })

	errRun := run([]string{"--output-format", "json", "-p", "hi", "-C", wd})
	_ = stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)

	if errRun == nil || !strings.Contains(errRun.Error(), "session save") {
		t.Fatalf("run err %v", errRun)
	}
	text := string(out)
	if !strings.Contains(text, `"error"`) || !strings.Contains(text, "session save") {
		t.Fatalf("stdout %s", text)
	}
}

func TestHeadlessJSONCancelOnSIGTERM(t *testing.T) {
	testHeadlessJSONCancelOnSignal(t, syscall.SIGTERM)
}

func TestHeadlessJSONCancelOnSIGINT(t *testing.T) {
	testHeadlessJSONCancelOnSignal(t, syscall.SIGINT)
}

func testHeadlessJSONCancelOnSignal(t *testing.T, sig os.Signal) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("signal cancel smoke is unix-only")
	}

	started := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flush", 500)
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `data: {"choices":[{"delta":{"content":"partial"}}]}`+"\n\n")
		fl.Flush()
		close(started)
		<-r.Context().Done()
	}))
	t.Cleanup(s.Close)

	root := t.TempDir()
	bin := filepath.Join(t.TempDir(), "goppi")
	build := exec.Command("go", "build", "-o", bin, "github.com/sspzoa/goppi/cmd/goppi")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}

	cmd := exec.Command(bin, "--output-format", "json", "-p", "slow")
	cmd.Env = append(os.Environ(),
		"GOPPI_DATA_DIR="+root,
		"HOME="+root,
		"UPSTAGE_API_KEY=test-key",
		"GOPPI_API_KEY=",
		"OPENAI_API_KEY=",
		"GOPPI_TUI=0",
		"GOPPI_BASE_URL="+s.URL,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("mock chat did not start")
	}
	time.Sleep(50 * time.Millisecond)
	if err := cmd.Process.Signal(sig); err != nil {
		t.Fatal(err)
	}

	out, _ := io.ReadAll(stdout)
	_ = cmd.Wait()
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("stdout %s: %v", out, err)
	}
	errMsg, _ := payload["error"].(string)
	if errMsg == "" {
		t.Fatalf("missing error: %s", out)
	}
	if !strings.Contains(errMsg, "cancel") && !strings.Contains(errMsg, "termin") && !strings.Contains(errMsg, "signal") {
		t.Fatalf("unexpected error %q in %s", errMsg, out)
	}
	if _, ok := payload["session_id"].(string); !ok || payload["session_id"] == "" {
		t.Fatalf("missing session_id: %s", out)
	}
}

func mockMCPScript(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir := wd; ; {
		path := filepath.Join(dir, "scripts", "ci-mock-mcp.py")
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("scripts/ci-mock-mcp.py not found")
		}
		dir = parent
	}
}

func writeUserMCPConfig(t *testing.T, home, script, marker string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "goppi")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	raw := fmt.Sprintf(`{
  "mcp_servers": {
    "ci": {
      "command": "python3",
      "args": [%q],
      "env": {"GOPPI_MCP_MARKER": %q}
    }
  }
}`, script, marker)
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
}
