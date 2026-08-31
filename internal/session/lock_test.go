package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHoldRejectsInvalidID(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	if _, err := Hold("../etc/passwd"); err == nil {
		t.Fatal("expected invalid id")
	}
}

func TestHoldRejectsSymlink(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	id := NewID()
	path, err := lockPath(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(filepath.Dir(path), "other.lock")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := Hold(id); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("got %v", err)
	}
}
