package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChangePreviewWriteNew(t *testing.T) {
	dir := t.TempDir()
	got := ChangePreview(dir, "write_file", []byte(`{"path":"a.go","contents":"package a\n"}`))
	if !strings.Contains(got, "new file") || !strings.Contains(got, "+ package a") {
		t.Fatalf("%q", got)
	}
}

func TestChangePreviewWriteOverwrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ChangePreview(dir, "write_file", []byte(`{"path":"a.go","contents":"new\n"}`))
	if !strings.Contains(got, "~ 1 → 1") || !strings.Contains(got, "- old") || !strings.Contains(got, "+ new") {
		t.Fatalf("%q", got)
	}
}

func TestChangePreviewEdit(t *testing.T) {
	got := ChangePreview("", "edit_file", []byte(`{"path":"a.go","old_string":"foo","new_string":"bar"}`))
	if !strings.Contains(got, "- foo") || !strings.Contains(got, "+ bar") {
		t.Fatalf("%q", got)
	}
}

func TestChangePreviewPatch(t *testing.T) {
	got := ChangePreview("", "apply_patch", []byte(`{"patch":"*** Begin Patch\n*** Update File: a.go\n@@\n-old\n+new\n*** End Patch\n"}`))
	if strings.Contains(got, "Begin Patch") || !strings.Contains(got, "Update File: a.go") || !strings.Contains(got, "+new") {
		t.Fatalf("%q", got)
	}
}

func TestAskDetailIncludesPathAndPreview(t *testing.T) {
	got := AskDetail(t.TempDir(), "write_file", []byte(`{"path":"a.go","contents":"x"}`))
	if !strings.Contains(got, "a.go") || !strings.Contains(got, "new file") {
		t.Fatalf("%q", got)
	}
}

func TestAskDetailRedacts(t *testing.T) {
	got := AskDetail(t.TempDir(), "bash", []byte(`{"command":"echo sk-abcdefghijklmnopqrst"}`))
	if strings.Contains(got, "sk-abcdefghijklmnopqrst") {
		t.Fatalf("%q", got)
	}
	if !strings.Contains(got, "[redacted]") {
		t.Fatalf("%q", got)
	}
}

func TestChangePreviewRedacts(t *testing.T) {
	got := ChangePreview(t.TempDir(), "write_file", []byte(`{"path":".env","contents":"k=sk-abcdefghijklmnopqrst"}`))
	if strings.Contains(got, "sk-abcdefghijklmnopqrst") {
		t.Fatalf("%q", got)
	}
}
