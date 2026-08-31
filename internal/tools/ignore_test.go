package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIgnorePatterns(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\nsecret/\n/build\n!keep.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ign := loadIgnore(dir)
	if !ign.ignored("a.log", false) {
		t.Fatal("*.log")
	}
	if ign.ignored("keep.log", false) {
		t.Fatal("negation")
	}
	if !ign.ignored("secret", true) || !ign.ignored("secret/x", false) {
		t.Fatal("secret/")
	}
	if !ign.ignored("build", true) || ign.ignored("src/build", true) {
		t.Fatal("/build anchored")
	}
	if ign.ignored("ok.txt", false) {
		t.Fatal("ok.txt")
	}
}

func TestGlobHonorsGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hide.log"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(dir, nil, nil)
	out, err := reg.Run(context.Background(), "glob", []byte(`{"pattern":"**/*"}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "hide.log") {
		t.Fatalf("%q", out)
	}
	if !strings.Contains(out, "ok.txt") {
		t.Fatalf("%q", out)
	}
}

func TestGrepHonorsGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("secret.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("needle"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pub.txt"), []byte("needle"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(dir, nil, nil)
	out, err := reg.Run(context.Background(), "grep", []byte(`{"pattern":"needle"}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "secret.txt") {
		t.Fatalf("%q", out)
	}
	if !strings.Contains(out, "pub.txt") {
		t.Fatalf("%q", out)
	}
	forced, err := reg.Run(context.Background(), "grep", []byte(`{"pattern":"needle","path":"secret.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(forced, "secret.txt") {
		t.Fatalf("explicit path: %q", forced)
	}
}

func TestNestedGitignore(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "pkg")
	if err := os.Mkdir(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, ".gitignore"), []byte("hidden.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "hidden.txt"), []byte("secret needle"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "visible.txt"), []byte("secret needle"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "root.txt"), []byte("secret needle"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(dir, nil, nil)
	out, err := reg.Run(context.Background(), "glob", []byte(`{"pattern":"**/*.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "hidden.txt") {
		t.Fatalf("glob %q", out)
	}
	if !strings.Contains(out, "visible.txt") || !strings.Contains(out, "root.txt") {
		t.Fatalf("glob %q", out)
	}
	hits, err := reg.Run(context.Background(), "grep", []byte(`{"pattern":"needle"}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hits, "hidden.txt") {
		t.Fatalf("grep %q", hits)
	}
	if !strings.Contains(hits, "visible.txt") || !strings.Contains(hits, "root.txt") {
		t.Fatalf("grep %q", hits)
	}
}

func TestNestedGitignoreNegation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "keep")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, ".gitignore"), []byte("!keep.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "keep.tmp"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "drop.tmp"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	ign := loadIgnore(dir)
	if !ign.ignored("drop.tmp", false) {
		t.Fatal("root *.tmp")
	}
	if ign.ignored("keep/keep.tmp", false) {
		t.Fatal("nested negation should win")
	}
}

func TestGlobDoesNotSkipWorkdirNamedBin(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(dir, nil, nil)
	out, err := reg.Run(context.Background(), "glob", []byte(`{"pattern":"**/*.go"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ok.go") {
		t.Fatalf("%q", out)
	}
}
