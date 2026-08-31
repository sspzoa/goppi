package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sspzoa/goppi/internal/provider"
)

const maxImageBytes = 4 << 20

type readImage struct {
	workdir string
	root    *fileRoot
	reg     *Registry
}

func (t readImage) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "read_image",
		Description: "Attach a workdir image (png, jpeg, gif, webp, bmp) so the model can see it. Use for screenshots and UI. Not for PDF — use document_parse.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"path":{"type":"string","description":"Image path inside workdir"}
			},
			"required":["path"]
		}`),
	}
}

func (t readImage) Run(_ context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path string `json:"path"`
	}](input)
	if err != nil {
		return "", err
	}
	img, err := LoadImage(t.workdir, args.Path, t.root.Extra()...)
	if err != nil {
		return "", err
	}
	if t.reg != nil {
		t.reg.queueImage(img)
	}
	return fmt.Sprintf("attached image %s (%s)", img.Path, img.MIME), nil
}

func (r *Registry) queueImage(img provider.Image) {
	if r == nil || img.URL == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pendingImages) >= 3 {
		return
	}
	r.pendingImages = append(r.pendingImages, img)
}

func (r *Registry) TakeImages() []provider.Image {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.pendingImages
	r.pendingImages = nil
	return out
}

func MentionedImages(workdir, text string, extraDirs ...string) []provider.Image {
	var out []provider.Image
	for _, m := range mentionRe.FindAllStringSubmatch(text, 8) {
		if len(m) < 2 || !looksLikeImage(m[1]) {
			continue
		}
		img, err := LoadImage(workdir, m[1], extraDirs...)
		if err != nil {
			continue
		}
		out = append(out, img)
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func HydrateImages(workdir string, msgs []provider.Message, extraDirs ...string) {
	for i := range msgs {
		for j := range msgs[i].Images {
			if msgs[i].Images[j].URL != "" || msgs[i].Images[j].Path == "" {
				continue
			}
			img, err := LoadImage(workdir, msgs[i].Images[j].Path, extraDirs...)
			if err != nil {
				continue
			}
			msgs[i].Images[j] = img
		}
	}
}

func ImageFromBytes(data []byte, hint string) (provider.Image, error) {
	if len(data) == 0 {
		return provider.Image{}, fmt.Errorf("empty image")
	}
	if len(data) > maxImageBytes {
		return provider.Image{}, fmt.Errorf("image too large (%d bytes, max %d)", len(data), maxImageBytes)
	}
	mime := sniffImage(data, hint)
	if mime == "" {
		return provider.Image{}, fmt.Errorf("not an image")
	}
	return provider.Image{
		Path: hint,
		MIME: mime,
		URL:  "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data),
	}, nil
}

func LoadImage(workdir, p string, extra ...string) (provider.Image, error) {
	abs, err := resolveAny(append([]string{workdir}, extra...), p)
	if err != nil {
		return provider.Image{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return provider.Image{}, err
	}
	if info.IsDir() {
		return provider.Image{}, fmt.Errorf("not a file")
	}
	if info.Size() > maxImageBytes {
		return provider.Image{}, fmt.Errorf("image too large (%d bytes, max %d)", info.Size(), maxImageBytes)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return provider.Image{}, err
	}
	mime := sniffImage(data, abs)
	if mime == "" {
		return provider.Image{}, fmt.Errorf("not an image")
	}
	rel := filepath.ToSlash(filepath.Clean(p))
	if filepath.IsAbs(p) {
		if r, err := filepath.Rel(workdir, abs); err == nil && !strings.HasPrefix(r, "..") {
			rel = filepath.ToSlash(r)
		}
	}
	return provider.Image{
		Path: rel,
		MIME: mime,
		URL:  "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data),
	}, nil
}

func looksLikeImage(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return true
	default:
		return false
	}
}

func sniffImage(data []byte, path string) string {
	mime := http.DetectContentType(data)
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp":
		return mime
	}
	if looksLikeImage(path) {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".png":
			return "image/png"
		case ".jpg", ".jpeg":
			return "image/jpeg"
		case ".gif":
			return "image/gif"
		case ".webp":
			return "image/webp"
		case ".bmp":
			return "image/bmp"
		}
	}
	return ""
}
