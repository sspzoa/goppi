package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sspzoa/goppi/internal/provider"
)

type applyPatch struct {
	workdir string
	root    *fileRoot
	snaps   *snapStack
}

func (applyPatch) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "apply_patch",
		Description: "Apply a multi-file patch. Prefer this over repeated edit_file when changing several places. Format:\n*** Begin Patch\n*** Add File: path\n+line\n*** Update File: path\n@@\n context\n-old\n+new\n*** Delete File: path\n*** End Patch",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"patch":{"type":"string","description":"*** Begin Patch … *** End Patch"}
			},
			"required":["patch"]
		}`),
	}
}

func (t applyPatch) Run(_ context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Patch string `json:"patch"`
	}](input)
	if err != nil {
		return "", err
	}
	if len(args.Patch) > maxWriteBytes {
		return "", fmt.Errorf("patch exceeds %d bytes", maxWriteBytes)
	}
	ops, err := parsePatch(args.Patch)
	if err != nil {
		return "", err
	}
	var done []string
	for _, op := range ops {
		path, err := scopedResolveWrite(t.workdir, t.root, op.path)
		if err != nil {
			return "", err
		}
		t.snaps.remember(path)
		msg, err := applyOp(path, op)
		if err != nil {
			return "", err
		}
		done = append(done, msg)
	}
	return strings.Join(done, "\n"), nil
}

type patchOp struct {
	kind  string // add, update, delete
	path  string
	hunks [][]patchLine
}

type patchLine struct {
	kind byte // ' ', '-', '+'
	text string
}

func parsePatch(s string) ([]patchOp, error) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "*** Begin Patch") || !strings.Contains(s, "*** End Patch") {
		return nil, fmt.Errorf("patch must contain *** Begin Patch and *** End Patch")
	}
	raw := strings.Split(s, "\n")
	var ops []patchOp
	var cur *patchOp
	begun := false
	for _, line := range raw {
		switch {
		case line == "*** Begin Patch":
			begun = true
		case line == "*** End Patch":
			if cur != nil {
				ops = append(ops, *cur)
				cur = nil
			}
			begun = false
		case !begun:
			continue
		case strings.HasPrefix(line, "*** Add File:"):
			if cur != nil {
				ops = append(ops, *cur)
			}
			cur = &patchOp{kind: "add", path: strings.TrimSpace(strings.TrimPrefix(line, "*** Add File:"))}
		case strings.HasPrefix(line, "*** Update File:"):
			if cur != nil {
				ops = append(ops, *cur)
			}
			cur = &patchOp{kind: "update", path: strings.TrimSpace(strings.TrimPrefix(line, "*** Update File:"))}
		case strings.HasPrefix(line, "*** Delete File:"):
			if cur != nil {
				ops = append(ops, *cur)
			}
			cur = &patchOp{kind: "delete", path: strings.TrimSpace(strings.TrimPrefix(line, "*** Delete File:"))}
		case cur == nil:
			return nil, fmt.Errorf("patch line outside a file hunk: %q", clipLine(line))
		case line == "@@" || strings.HasPrefix(line, "@@ "):
			if cur != nil {
				cur.hunks = append(cur.hunks, nil)
			}
		case cur.kind == "add":
			text := line
			if strings.HasPrefix(line, "+") {
				text = line[1:]
			}
			appendPatchLine(cur, patchLine{kind: '+', text: text})
		default:
			pl := patchLine{kind: ' ', text: line}
			if line != "" {
				switch line[0] {
				case '+', '-', ' ':
					pl = patchLine{kind: line[0], text: line[1:]}
				}
			} else {
				pl = patchLine{kind: ' ', text: ""}
			}
			appendPatchLine(cur, pl)
		}
	}
	if cur != nil {
		ops = append(ops, *cur)
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("patch has no file operations")
	}
	for _, op := range ops {
		if op.path == "" {
			return nil, fmt.Errorf("%s missing path", op.kind)
		}
	}
	return ops, nil
}

func appendPatchLine(op *patchOp, ln patchLine) {
	if len(op.hunks) == 0 {
		op.hunks = append(op.hunks, nil)
	}
	op.hunks[len(op.hunks)-1] = append(op.hunks[len(op.hunks)-1], ln)
}

func applyOp(path string, op patchOp) (string, error) {
	switch op.kind {
	case "add":
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("add %s: already exists", path)
		}
		var b strings.Builder
		for _, hunk := range op.hunks {
			for _, ln := range hunk {
				if ln.kind == '-' {
					return "", fmt.Errorf("add %s: unexpected removal line", path)
				}
				b.WriteString(ln.text)
				b.WriteByte('\n')
			}
		}
		if err := writeAtomic(path, []byte(b.String()), 0o644); err != nil {
			return "", err
		}
		return "added " + path, nil
	case "delete":
		if err := os.Remove(path); err != nil {
			return "", err
		}
		return "deleted " + path, nil
	case "update":
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		next, err := applyHunks(string(raw), op.hunks)
		if err != nil {
			return "", fmt.Errorf("update %s: %w", path, err)
		}
		if len(next) > maxWriteBytes {
			return "", fmt.Errorf("result would exceed %d bytes", maxWriteBytes)
		}
		mode := os.FileMode(0o644)
		if st, err := os.Stat(path); err == nil {
			mode = st.Mode().Perm()
		}
		if err := writeAtomic(path, []byte(next), mode); err != nil {
			return "", err
		}
		return "updated " + path, nil
	default:
		return "", fmt.Errorf("unknown patch op %q", op.kind)
	}
}

func applyHunks(src string, hunks [][]patchLine) (string, error) {
	cur := src
	for _, lines := range hunks {
		if len(lines) == 0 {
			continue
		}
		file := splitLines([]byte(cur))
		old, neu := hunkSides(lines)
		if len(old) == 0 && len(neu) == 0 {
			continue
		}
		start, err := findUnique(file, old)
		if err != nil {
			return "", err
		}
		out := make([]string, 0, len(file)-len(old)+len(neu))
		out = append(out, file[:start]...)
		out = append(out, neu...)
		out = append(out, file[start+len(old):]...)
		if strings.HasSuffix(cur, "\n") || cur == "" {
			cur = strings.Join(out, "\n") + "\n"
		} else {
			cur = strings.Join(out, "\n")
		}
	}
	return cur, nil
}

func hunkSides(lines []patchLine) (old, neu []string) {
	for _, ln := range lines {
		switch ln.kind {
		case ' ', '-':
			old = append(old, ln.text)
		}
		switch ln.kind {
		case ' ', '+':
			neu = append(neu, ln.text)
		}
	}
	return old, neu
}

func findUnique(file, old []string) (int, error) {
	if len(old) == 0 {
		return 0, fmt.Errorf("hunk removes nothing")
	}
	found := -1
	for i := 0; i+len(old) <= len(file); i++ {
		ok := true
		for j := range old {
			if file[i+j] != old[j] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if found >= 0 {
			return 0, fmt.Errorf("hunk matches more than once")
		}
		found = i
	}
	if found < 0 {
		return 0, fmt.Errorf("hunk not found")
	}
	return found, nil
}

func clipLine(s string) string {
	if len(s) > 60 {
		return s[:60] + "…"
	}
	return s
}

func firstPatchPath(patch string) string {
	for _, line := range strings.Split(patch, "\n") {
		for _, p := range []string{"*** Add File:", "*** Update File:", "*** Delete File:"} {
			if strings.HasPrefix(line, p) {
				return strings.TrimSpace(strings.TrimPrefix(line, p))
			}
		}
	}
	return "patch"
}
