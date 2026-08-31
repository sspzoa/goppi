package tools

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxDiffFiles     = 20
	maxDiffLines     = 400
	maxDiffFileLines = 2000
	diffContext      = 2
)

func (s *snapStack) originals() []snapshot {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]snapshot{}
	var order []string
	for _, it := range s.items {
		if _, ok := seen[it.path]; ok {
			continue
		}
		seen[it.path] = it
		order = append(order, it.path)
	}
	out := make([]snapshot, 0, len(order))
	for _, p := range order {
		out = append(out, seen[p])
	}
	return out
}

func (s *snapStack) clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.items = nil
	s.mu.Unlock()
}

func (r *Registry) ClearEdits() {
	if r != nil && r.snaps != nil {
		r.snaps.clear()
	}
}

func (r *Registry) SessionDiff() string {
	if r == nil || r.snaps == nil {
		return "(no edits)"
	}
	origs := r.snaps.originals()
	if len(origs) == 0 {
		return "(no edits)"
	}
	var b strings.Builder
	shown := 0
	lines := 0
	for _, snap := range origs {
		if shown >= maxDiffFiles {
			fmt.Fprintf(&b, "... %d more files\n", len(origs)-shown)
			break
		}
		cur, err := os.ReadFile(snap.path)
		existedNow := err == nil
		if !existedNow && !snap.existed {
			continue
		}
		var old []byte
		if snap.existed {
			old = snap.data
		}
		var neu []byte
		if existedNow {
			neu = cur
		}
		if bytes.Equal(old, neu) {
			continue
		}
		rel := relWork(r.workdir, snap.path)
		chunk := unifiedDiff(rel, old, neu, snap.existed, existedNow)
		if chunk == "" {
			continue
		}
		n := strings.Count(chunk, "\n")
		if lines+n > maxDiffLines {
			remain := maxDiffLines - lines
			if remain > 0 {
				b.WriteString(clipLines(chunk, remain))
			}
			b.WriteString("... diff truncated\n")
			break
		}
		b.WriteString(chunk)
		if !strings.HasSuffix(chunk, "\n") {
			b.WriteByte('\n')
		}
		shown++
		lines += n
	}
	if b.Len() == 0 {
		return "(no edits)"
	}
	return strings.TrimRight(b.String(), "\n")
}

func relWork(workdir, path string) string {
	root := workdir
	if real, err := filepath.EvalSymlinks(workdir); err == nil {
		root = real
	}
	p := path
	if real, err := filepath.EvalSymlinks(path); err == nil {
		p = real
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(filepath.Base(path))
	}
	return filepath.ToSlash(rel)
}

func clipLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	parts := strings.Split(s, "\n")
	if len(parts) <= n {
		return s
	}
	return strings.Join(parts[:n], "\n") + "\n"
}

func unifiedDiff(rel string, old, neu []byte, oldExisted, newExisted bool) string {
	a := splitLines(old)
	b := splitLines(neu)
	if len(a) > maxDiffFileLines || len(b) > maxDiffFileLines {
		return fmt.Sprintf("%s  (%d → %d lines, too large to diff)\n", rel, len(a), len(b))
	}
	oldName, newName := "a/"+rel, "b/"+rel
	if !oldExisted {
		oldName = "/dev/null"
	}
	if !newExisted {
		newName = "/dev/null"
	}
	ops := lineEdits(a, b)
	var body strings.Builder
	i, j := 0, 0
	for k := 0; k < len(ops); {
		for k < len(ops) && ops[k] == ' ' {
			i++
			j++
			k++
		}
		if k >= len(ops) {
			break
		}
		startK := k
		startI, startJ := i, j
		for k < len(ops) && ops[k] != ' ' {
			switch ops[k] {
			case '-':
				i++
			case '+':
				j++
			}
			k++
		}
		ctxBefore := diffContext
		if startI < ctxBefore {
			ctxBefore = startI
		}
		hunkI := startI - ctxBefore
		hunkJ := startJ - ctxBefore
		endK := k
		ctxAfter := 0
		for ctxAfter < diffContext && endK < len(ops) && ops[endK] == ' ' {
			endK++
			ctxAfter++
		}
		oldCount := (startI - hunkI) + (i - startI) + ctxAfter
		newCount := (startJ - hunkJ) + (j - startJ) + ctxAfter
		fmt.Fprintf(&body, "@@ -%d,%d +%d,%d @@\n", hunkStart(hunkI, oldCount), oldCount, hunkStart(hunkJ, newCount), newCount)
		ai, aj := hunkI, hunkJ
		for t := startK - ctxBefore; t < endK; t++ {
			switch ops[t] {
			case ' ':
				body.WriteByte(' ')
				body.WriteString(a[ai])
				body.WriteByte('\n')
				ai++
				aj++
			case '-':
				body.WriteByte('-')
				body.WriteString(a[ai])
				body.WriteByte('\n')
				ai++
			case '+':
				body.WriteByte('+')
				body.WriteString(b[aj])
				body.WriteByte('\n')
				aj++
			}
		}
		i += ctxAfter
		j += ctxAfter
		k = endK
	}
	if body.Len() == 0 {
		return ""
	}
	return fmt.Sprintf("--- %s\n+++ %s\n%s", oldName, newName, body.String())
}

func hunkStart(idx, count int) int {
	if count == 0 {
		return idx
	}
	return idx + 1
}

func splitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	s := strings.TrimSuffix(string(b), "\n")
	return strings.Split(s, "\n")
}

func lineEdits(a, b []string) []byte {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	var rev []byte
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			rev = append(rev, ' ')
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			rev = append(rev, '+')
			j--
		} else {
			rev = append(rev, '-')
			i--
		}
	}
	for l, r := 0, len(rev)-1; l < r; l, r = l+1, r-1 {
		rev[l], rev[r] = rev[r], rev[l]
	}
	return rev
}
