package instructions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "GOPPI.md"), []byte("use make test"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, found := Load(dir)
	if len(found) != 1 || found[0] != "GOPPI.md" {
		t.Fatalf("found = %v", found)
	}
	if text != "use make test" {
		t.Fatalf("text = %q", text)
	}
}

func TestLoadTruncates(t *testing.T) {
	dir := t.TempDir()
	body := strings.Repeat("x", maxBytes+80)
	if err := os.WriteFile(filepath.Join(dir, "GOPPI.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	text, found := Load(dir)
	if len(found) != 1 {
		t.Fatalf("found = %v", found)
	}
	if !strings.HasSuffix(text, "… truncated") {
		t.Fatalf("expected truncation, len=%d", len(text))
	}
	if len(text) > maxBytes+20 {
		t.Fatalf("still too large: %d", len(text))
	}
}
