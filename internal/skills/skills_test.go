package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjectSkill(t *testing.T) {
	wd := t.TempDir()
	dir := filepath.Join(wd, ".goppi", "skills", "review")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Review\nLook at tests first."), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load(wd)
	if len(got) != 1 || got[0].Name != "review" {
		t.Fatalf("%+v", got)
	}
	if _, ok := Lookup(got, "review"); !ok {
		t.Fatal("lookup")
	}
}
