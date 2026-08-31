package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandMentionsTextAndImage(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, dir, "shot.png")
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte("package foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, imgs := ExpandMentions(dir, "look at @foo.go and @shot.png")
	if !strings.Contains(text, "package foo") || !strings.Contains(text, "--- @foo.go ---") {
		t.Fatalf("%q", text)
	}
	if len(imgs) != 1 || imgs[0].Path != "shot.png" {
		t.Fatalf("%+v", imgs)
	}
	if strings.Contains(text, "PNG") || strings.Contains(text, "--- @shot.png") {
		t.Fatalf("image must not be inlined: %q", text)
	}
}

func TestExpandMentionsRejectsEscapeAndBinary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin.dat"), []byte{'a', 0, 'b'}, 0o644); err != nil {
		t.Fatal(err)
	}
	text, _ := ExpandMentions(dir, "x @../passwd @bin.dat @ok.txt")
	if strings.Contains(text, "--- @../passwd") || strings.Contains(text, "--- @bin.dat") {
		t.Fatalf("%q", text)
	}
	if !strings.Contains(text, "--- @ok.txt ---") || !strings.Contains(text, "ok") {
		t.Fatalf("%q", text)
	}
}

func TestExpandMentionsRedactsSecrets(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "key.env"), []byte("k=sk-abcdefghijklmnopqrst\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	text, _ := ExpandMentions(dir, "see @key.env")
	if strings.Contains(text, "sk-abcdefghijklmnopqrst") {
		t.Fatalf("%q", text)
	}
	if !strings.Contains(text, "[redacted]") {
		t.Fatalf("%q", text)
	}
}

func TestExpandMentionsTruncates(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("a", maxMentionBytes+32)
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	text, _ := ExpandMentions(dir, "@big.txt")
	if !strings.Contains(text, "truncated") {
		t.Fatalf("%q", text)
	}
	if strings.Contains(text, strings.Repeat("a", maxMentionBytes+1)) {
		t.Fatal("body not truncated")
	}
}
