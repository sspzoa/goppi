package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "main.ts", false},
		{"**/*.go", "internal/tools/fs.go", true},
		{"**/*.go", "fs.go", true},
		{"internal/**/*.go", "internal/tools/fs.go", true},
		{"internal/**/*.go", "cmd/goppi/main.go", false},
		{"**/*", "a/b/c", true},
	}
	for _, tc := range cases {
		got, err := matchGlob(tc.pattern, tc.name)
		if err != nil {
			t.Fatalf("%s: %v", tc.pattern, err)
		}
		if got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

func TestResolveStaysInWorkdir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolve(root, "ok.txt")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "ok.txt" {
		t.Fatalf("got %s", got)
	}

	inside, err := resolve(root, "./nested/../ok.txt")
	if err != nil {
		t.Fatal(err)
	}
	if inside != got {
		t.Fatalf("cleaned path %s != %s", inside, got)
	}

	for _, p := range []string{"../secret", "../../etc/passwd", "/etc/passwd", filepath.Join(root, "..", "outside")} {
		if _, err := resolve(root, p); err == nil {
			t.Fatalf("expected reject %q", p)
		} else if !strings.Contains(err.Error(), "outside workdir") && p != "" {
			t.Fatalf("%q: %v", p, err)
		}
	}
}

func TestAllowSessionSkipsSecondAsk(t *testing.T) {
	n := 0
	reg := New(t.TempDir(), nil, func(string, string) Verdict {
		n++
		return AllowedSession
	})
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"x","contents":"y"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"x","contents":"z"}`)); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("asked %d times", n)
	}
	reg.ClearSessionAllows()
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"x","contents":"w"}`)); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("after clear asked %d", n)
	}
}

func TestAllowOnceAsksAgain(t *testing.T) {
	n := 0
	reg := New(t.TempDir(), nil, func(string, string) Verdict {
		n++
		return Allowed
	})
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"x","contents":"a"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"x","contents":"b"}`)); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("once should re-ask, got %d", n)
	}
}

func TestPermissionDenied(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, AlwaysDeny)
	_, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"x","contents":"y"}`))
	if err == nil || err.Error() != "permission denied: write_file" {
		t.Fatalf("got %v", err)
	}
}

func TestWriteFileRejectsGitHooks(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := New(dir, nil, nil)
	_, err := reg.Run(context.Background(), "write_file", []byte(`{"path":".git/hooks/pre-commit","contents":"#!/bin/sh\n"}`))
	if err == nil || !strings.Contains(err.Error(), "refusing to write") {
		t.Fatalf("got %v", err)
	}
	_, err = reg.Run(context.Background(), "write_file", []byte(`{"path":".git/config","contents":"[core]\n"}`))
	if err == nil || !strings.Contains(err.Error(), "refusing to write") {
		t.Fatalf("config: %v", err)
	}
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"ok.go","contents":"package ok\n"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestWriteFileRejectsGitHEADAndRefs(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "refs", "heads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "nested", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := New(dir, nil, nil)
	_, err := reg.Run(context.Background(), "write_file", []byte(`{"path":".git/HEAD","contents":"ref: refs/heads/pwned\n"}`))
	if err == nil || !strings.Contains(err.Error(), "refusing to write") {
		t.Fatalf("HEAD: %v", err)
	}
	_, err = reg.Run(context.Background(), "write_file", []byte(`{"path":".git/refs/heads/main","contents":"deadbeef\n"}`))
	if err == nil || !strings.Contains(err.Error(), "refusing to write") {
		t.Fatalf("refs: %v", err)
	}
	_, err = reg.Run(context.Background(), "write_file", []byte(`{"path":"nested/.git/config","contents":"[core]\n"}`))
	if err == nil || !strings.Contains(err.Error(), "refusing to write") {
		t.Fatalf("nested: %v", err)
	}
}

func TestWriteFileRejectsHuge(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	huge := strings.Repeat("a", maxWriteBytes+1)
	_, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"big","contents":"`+huge+`"}`))
	if err == nil || !strings.Contains(err.Error(), "exceed") {
		t.Fatalf("got %v", err)
	}
}

func TestUndoRestoresWrite(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"a.txt","contents":"one"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"a.txt","contents":"two"}`)); err != nil {
		t.Fatal(err)
	}
	msg, err := reg.UndoLast()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "restored") {
		t.Fatalf("%q", msg)
	}
	out, err := reg.Run(context.Background(), "read_file", []byte(`{"path":"a.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "one") || strings.Contains(out, "two") {
		t.Fatalf("after undo: %q", out)
	}
}

func TestWriteThenReadInsideWorkdir(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"a.txt","contents":"hello"}`)); err != nil {
		t.Fatal(err)
	}
	out, err := reg.Run(context.Background(), "read_file", []byte(`{"path":"a.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("read %q", out)
	}
}

func TestResolveRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "leak")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	if _, err := resolve(root, "leak"); err == nil {
		t.Fatal("symlink escape should fail")
	}
}

func TestReadFileAllowsAdditionalDir(t *testing.T) {
	wd := t.TempDir()
	extra := t.TempDir()
	if err := os.WriteFile(filepath.Join(extra, "lib.go"), []byte("package lib\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(wd, nil, nil)
	reg.SetExtraDirs([]string{extra})
	path, _ := json.Marshal(filepath.Join(extra, "lib.go"))
	out, err := reg.Run(context.Background(), "read_file", []byte(`{"path":`+string(path)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "package lib") {
		t.Fatalf("%q", out)
	}
	if _, err := reg.Run(context.Background(), "read_file", []byte(`{"path":"/etc/passwd"}`)); err == nil {
		t.Fatal("outside extra roots should fail")
	}
}

func TestGlobAndGrepSearchAdditionalDir(t *testing.T) {
	wd := t.TempDir()
	extra := t.TempDir()
	if err := os.WriteFile(filepath.Join(extra, "lib.go"), []byte("package extraLib\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(wd, nil, nil)
	reg.SetExtraDirs([]string{extra})
	glob, err := reg.Run(context.Background(), "glob", []byte(`{"pattern":"**/*.go"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(glob, "lib.go") {
		t.Fatalf("glob missed extra: %q", glob)
	}
	grep, err := reg.Run(context.Background(), "grep", []byte(`{"pattern":"extraLib"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(grep, "extraLib") {
		t.Fatalf("grep missed extra: %q", grep)
	}
}
