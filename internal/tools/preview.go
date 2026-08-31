package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

const (
	previewLines = 8
	previewRunes = 80
)

func AskDetail(workdir, name string, input json.RawMessage, extraDirs ...string) string {
	base := Detail(name, input)
	extra := ChangePreview(workdir, name, input, extraDirs...)
	switch {
	case extra == "":
		return RedactSecrets(base)
	case base == "":
		return RedactSecrets(extra)
	default:
		return RedactSecrets(base + "\n" + extra)
	}
}

func ChangePreview(workdir, name string, input json.RawMessage, extraDirs ...string) string {
	var out string
	switch name {
	case "write_file":
		out = previewWrite(workdir, input, extraDirs...)
	case "edit_file":
		out = previewEdit(input)
	case "apply_patch":
		out = previewPatch(input)
	}
	return RedactSecrets(out)
}

func previewWrite(workdir string, input json.RawMessage, extra ...string) string {
	args, err := decode[struct {
		Path     string `json:"path"`
		Contents string `json:"contents"`
	}](input)
	if err != nil || args.Contents == "" {
		return ""
	}
	neu := args.Contents
	if abs, err := resolveAny(append([]string{workdir}, extra...), args.Path); err == nil {
		if data, err := os.ReadFile(abs); err == nil {
			old := string(data)
			if old == neu {
				return "unchanged"
			}
			return fmt.Sprintf("~ %d → %d lines\n%s", countLines(old), countLines(neu), plusMinus(old, neu))
		}
	}
	return "new file\n" + prefixLines("+ ", neu)
}

func previewEdit(input json.RawMessage) string {
	args, err := decode[struct {
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}](input)
	if err != nil {
		return ""
	}
	if args.OldString == "" && args.NewString == "" {
		return ""
	}
	return strings.TrimRight(prefixLines("- ", args.OldString)+prefixLines("+ ", args.NewString), "\n")
}

func previewPatch(input json.RawMessage) string {
	var raw struct {
		Patch string `json:"patch"`
	}
	if json.Unmarshal(input, &raw) != nil || strings.TrimSpace(raw.Patch) == "" {
		return ""
	}
	var keep []string
	for _, line := range strings.Split(raw.Patch, "\n") {
		s := strings.TrimRight(line, "\r")
		if strings.HasPrefix(s, "*** Begin") || strings.HasPrefix(s, "*** End") {
			continue
		}
		if s == "" && len(keep) == 0 {
			continue
		}
		keep = append(keep, clipPreviewLine(s))
		if len(keep) >= previewLines {
			break
		}
	}
	return strings.Join(keep, "\n")
}

func plusMinus(old, neu string) string {
	ol := splitPreviewLines(old)
	nl := splitPreviewLines(neu)
	i := 0
	for i < len(ol) && i < len(nl) && ol[i] == nl[i] {
		i++
	}
	var b strings.Builder
	for _, line := range ol[i:] {
		if strings.Count(b.String(), "\n") >= previewLines/2 {
			break
		}
		b.WriteString("- ")
		b.WriteString(clipPreviewLine(line))
		b.WriteByte('\n')
	}
	for _, line := range nl[i:] {
		if strings.Count(b.String(), "\n") >= previewLines {
			break
		}
		b.WriteString("+ ")
		b.WriteString(clipPreviewLine(line))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func prefixLines(prefix, s string) string {
	var b strings.Builder
	n := 0
	for _, line := range splitPreviewLines(s) {
		b.WriteString(prefix)
		b.WriteString(clipPreviewLine(line))
		b.WriteByte('\n')
		n++
		if n >= previewLines {
			if countLines(s) > previewLines {
				b.WriteString("…\n")
			}
			break
		}
	}
	return b.String()
}

func splitPreviewLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func countLines(s string) int {
	s = strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func clipPreviewLine(s string) string {
	if utf8.RuneCountInString(s) <= previewRunes {
		return s
	}
	return string([]rune(s)[:previewRunes]) + "…"
}
