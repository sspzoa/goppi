package tools

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/sspzoa/goppi/internal/provider"
)

const (
	maxMentionBytes = 64 << 10
	maxMentionFiles = 4
)

var mentionRe = regexp.MustCompile(`@([^\s]+)`)

func ExpandMentions(workdir, text string, extraDirs ...string) (string, []provider.Image) {
	imgs := MentionedImages(workdir, text, extraDirs...)
	extra := mentionedTexts(workdir, text, extraDirs...)
	if extra == "" {
		return text, imgs
	}
	return text + "\n\n" + extra, imgs
}

func mentionedTexts(workdir, text string, extraDirs ...string) string {
	seen := map[string]bool{}
	var parts []string
	for _, m := range mentionRe.FindAllStringSubmatch(text, 8) {
		if len(parts) >= maxMentionFiles {
			break
		}
		if len(m) < 2 || looksLikeImage(m[1]) {
			continue
		}
		abs, err := resolveAny(append([]string{workdir}, extraDirs...), m[1])
		if err != nil {
			continue
		}
		if seen[abs] {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			continue
		}
		body, trunc, err := readMention(abs)
		if err != nil {
			continue
		}
		seen[abs] = true
		rel := filepath.ToSlash(filepath.Clean(m[1]))
		if filepath.IsAbs(m[1]) {
			if r, err := filepath.Rel(workdir, abs); err == nil && !strings.HasPrefix(r, "..") {
				rel = filepath.ToSlash(r)
			}
		}
		block := fmt.Sprintf("--- @%s ---\n%s", rel, body)
		if trunc {
			block += "\n…(truncated)"
		}
		parts = append(parts, block)
	}
	return strings.Join(parts, "\n\n")
}

func readMention(abs string) (string, bool, error) {
	f, err := os.Open(abs)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	buf := make([]byte, maxMentionBytes+1)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", false, err
	}
	trunc := n > maxMentionBytes
	if trunc {
		n = maxMentionBytes
	}
	data := buf[:n]
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return "", false, fmt.Errorf("not text")
	}
	return RedactSecrets(string(data)), trunc, nil
}
