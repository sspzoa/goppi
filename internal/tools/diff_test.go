package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionDiffNewFile(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"a.txt","contents":"hello\n"}`)); err != nil {
		t.Fatal(err)
	}
	got := reg.SessionDiff()
	if !strings.Contains(got, "+++ b/a.txt") || !strings.Contains(got, "+hello") {
		t.Fatalf("%s", got)
	}
	if !strings.Contains(got, "--- /dev/null") {
		t.Fatalf("new file should be vs /dev/null:\n%s", got)
	}
}

func TestSessionDiffEdit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(dir, nil, nil)
	if _, err := reg.Run(context.Background(), "edit_file", []byte(`{"path":"a.txt","old_string":"one","new_string":"two"}`)); err != nil {
		t.Fatal(err)
	}
	got := reg.SessionDiff()
	if !strings.Contains(got, "-one") || !strings.Contains(got, "+two") {
		t.Fatalf("%s", got)
	}
}

func TestSessionDiffEmpty(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	if got := reg.SessionDiff(); got != "(no edits)" {
		t.Fatalf("%q", got)
	}
}

func TestSessionDiffUsesOriginal(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"a.txt","contents":"v1\n"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"a.txt","contents":"v2\n"}`)); err != nil {
		t.Fatal(err)
	}
	got := reg.SessionDiff()
	if strings.Contains(got, "+v1") {
		t.Fatalf("should diff vs first snapshot:\n%s", got)
	}
	if !strings.Contains(got, "+v2") {
		t.Fatalf("%s", got)
	}
}

func TestClearEdits(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	if _, err := reg.Run(context.Background(), "write_file", []byte(`{"path":"a.txt","contents":"x"}`)); err != nil {
		t.Fatal(err)
	}
	reg.ClearEdits()
	if got := reg.SessionDiff(); got != "(no edits)" {
		t.Fatalf("%q", got)
	}
}

func TestUnifiedEqualIsEmpty(t *testing.T) {
	if got := unifiedDiff("x", []byte("a\n"), []byte("a\n"), true, true); got != "" {
		t.Fatalf("%q", got)
	}
}
