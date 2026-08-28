package instructions

import (
	"os"
	"path/filepath"
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
