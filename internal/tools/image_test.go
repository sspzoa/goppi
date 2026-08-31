package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePNG(t *testing.T, dir, name string) string {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImageFromBytes(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	img, err := ImageFromBytes(buf.Bytes(), "")
	if err != nil {
		t.Fatal(err)
	}
	if img.MIME != "image/png" || !strings.HasPrefix(img.URL, "data:image/png;base64,") {
		t.Fatalf("%+v", img)
	}
	if _, err := ImageFromBytes([]byte("not-an-image"), ""); err == nil {
		t.Fatal("expected reject")
	}
}

func TestLoadImage(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, dir, "a.png")
	img, err := LoadImage(dir, "a.png")
	if err != nil {
		t.Fatal(err)
	}
	if img.MIME != "image/png" || !strings.HasPrefix(img.URL, "data:image/png;base64,") {
		t.Fatalf("%+v", img)
	}
	if img.Path != "a.png" {
		t.Fatalf("path %q", img.Path)
	}
}

func TestLoadImageRejectsOutside(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadImage(dir, "../secret.png")
	if err == nil {
		t.Fatal("expected reject")
	}
}

func TestLoadImageRejectsText(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadImage(dir, "a.txt")
	if err == nil {
		t.Fatal("expected reject")
	}
}

func TestMentionedImages(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, dir, "shot.png")
	got := MentionedImages(dir, "look at @shot.png and README.md")
	if len(got) != 1 || got[0].Path != "shot.png" {
		t.Fatalf("%+v", got)
	}
}

func TestReadImageQueues(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, dir, "a.png")
	reg := New(dir, nil, nil)
	out, err := reg.Run(context.Background(), "read_image", json.RawMessage(`{"path":"a.png"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "attached image") {
		t.Fatalf("%q", out)
	}
	imgs := reg.TakeImages()
	if len(imgs) != 1 || imgs[0].MIME != "image/png" {
		t.Fatalf("%+v", imgs)
	}
	if len(reg.TakeImages()) != 0 {
		t.Fatal("queue should drain")
	}
}
