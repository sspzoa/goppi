package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sspzoa/goppi/internal/config"
)

func TestMain(m *testing.M) {
	if os.Getenv("GOPPI_FAKE_LSP") == "1" {
		fakeLSP()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeLSP() {
	_, _ = os.Stderr.WriteString("lsp-stderr-line\n")
	conn := newConn(os.Stdin, os.Stdout)
	conn.reply = func(string) any { return nil }
	conn.onNotify = func(method string, params json.RawMessage) {
		if method != "textDocument/didOpen" {
			return
		}
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if json.Unmarshal(params, &p) != nil || p.TextDocument.URI == "" {
			return
		}
		_ = conn.notify("textDocument/publishDiagnostics", map[string]any{
			"uri": p.TextDocument.URI,
			"diagnostics": []map[string]any{{
				"range": map[string]any{
					"start": map[string]int{"line": 2, "character": 0},
					"end":   map[string]int{"line": 2, "character": 1},
				},
				"severity": 1,
				"source":   "fake",
				"message":  "undefined: Foo",
			}},
		})
	}
	select {}
}

func TestStartFakeDiagnostics(t *testing.T) {
	t.Setenv("GOPPI_FAKE_LSP", "1")
	t.Setenv("GOPPI_LSP", "off")
	wd := t.TempDir()
	src := filepath.Join(wd, "main.go")
	if err := os.WriteFile(src, []byte("package main\n\nvar x Foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	s, err := Start(ctx, "fake", config.LSPServer{Command: self, Language: "go"}, wd, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	hub := &Hub{}
	hub.add(s)
	out, err := hub.Query(ctx, wd, src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "main.go:3:1") || !strings.Contains(out, "undefined: Foo") {
		t.Fatalf("got %q", out)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if strings.Contains(s.Stderr(), "lsp-stderr-line") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stderr %q", s.Stderr())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestProjectLooksLikeGo(t *testing.T) {
	wd := t.TempDir()
	if looksLikeGo(wd) {
		t.Fatal("empty dir")
	}
	if err := os.WriteFile(filepath.Join(wd, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !looksLikeGo(wd) {
		t.Fatal("go.mod")
	}
}

func TestFormatEmpty(t *testing.T) {
	if formatDiags("/tmp", nil) != "(no diagnostics)" {
		t.Fatal("empty")
	}
}

func TestStartAllOff(t *testing.T) {
	t.Setenv("GOPPI_LSP", "off")
	hub, errs := StartAll(context.Background(), map[string]config.LSPServer{
		"x": {Command: "definitely-missing-lsp"},
	}, t.TempDir(), "test", nil)
	if len(errs) != 0 {
		t.Fatalf("errs %v", errs)
	}
	if len(hub.Names()) != 0 {
		t.Fatalf("names %v", hub.Names())
	}
}

func TestStartHookError(t *testing.T) {
	_, err := Start(context.Background(), "x", config.LSPServer{Command: os.Args[0]}, t.TempDir(), "test", func(*exec.Cmd) error {
		return fmt.Errorf("blocked")
	})
	if err == nil || !strings.Contains(err.Error(), "sandbox") || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("got %v", err)
	}
}
