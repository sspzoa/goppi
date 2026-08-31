package tools

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxGitGuardFile  = 256 * 1024
	maxGitGuardFiles = 64
	maxGitGuardRefs  = 4096
	maxGitGuardObjs  = 100_000
)

type gitFileSnap struct {
	mode os.FileMode
	data []byte
}

type gitFileWatch struct {
	path  string
	label string
	snap  *gitFileSnap
	ok    bool
}

type gitTreeWatch struct {
	dir     string
	prefix  string
	snap    map[string]gitFileSnap
	existed bool
	ok      bool
}

type gitGuard struct {
	hooksDir     string
	hooks        map[string]gitFileSnap
	hooksExisted bool
	configPath   string
	config       *gitFileSnap
	wtConfigPath string
	wtConfig     *gitFileSnap
	objectsDir   string
	objects      map[string]struct{}
	objectsOK    bool
	files        []gitFileWatch
	trees        []gitTreeWatch
}

func snapshotGitGuards(dirs []string) []*gitGuard {
	var out []*gitGuard
	seen := map[string]bool{}
	for _, dir := range dirs {
		g := snapshotGitGuard(dir)
		if g == nil {
			continue
		}
		key := g.hooksDir + "\x00" + g.configPath
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, g)
	}
	return out
}

func revertGitGuards(gs []*gitGuard) error {
	var first error
	for _, g := range gs {
		if err := revertGitWrites(g); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func snapshotGitGuard(workdir string) *gitGuard {
	common := gitCommonDir(workdir)
	if common == "" {
		return nil
	}
	g := &gitGuard{
		hooksDir:   filepath.Join(common, "hooks"),
		hooks:      map[string]gitFileSnap{},
		configPath: filepath.Join(common, "config"),
		objectsDir: filepath.Join(common, "objects"),
		objects:    map[string]struct{}{},
	}
	if dir := resolveGitDir(workdir); dir != "" && dir != common {
		g.wtConfigPath = filepath.Join(dir, "config")
		g.wtConfig = readGitFile(g.wtConfigPath)
	}
	if st, err := os.Lstat(g.hooksDir); err == nil && st.IsDir() && st.Mode()&os.ModeSymlink == 0 {
		g.hooksExisted = true
		g.hooks = snapshotGitTree(g.hooksDir, maxGitGuardFiles)
	}
	g.config = readGitFile(g.configPath)
	if st, err := os.Lstat(g.objectsDir); err == nil && st.IsDir() && st.Mode()&os.ModeSymlink == 0 {
		g.objectsOK = true
		g.objects = listGitTree(g.objectsDir, maxGitGuardObjs)
	}
	dir := resolveGitDir(workdir)
	if dir == "" {
		dir = common
	}
	g.files = append(g.files, watchGitFile(filepath.Join(dir, "HEAD"), ".git/HEAD"))
	g.files = append(g.files, watchGitFile(filepath.Join(common, "packed-refs"), ".git/packed-refs"))
	g.trees = append(g.trees, watchGitTree(filepath.Join(common, "refs"), ".git/refs", maxGitGuardRefs))
	if dir != common {
		g.files = append(g.files, watchGitFile(filepath.Join(common, "HEAD"), ".git/HEAD"))
		g.files = append(g.files, watchGitFile(filepath.Join(dir, "packed-refs"), ".git/packed-refs"))
		g.trees = append(g.trees, watchGitTree(filepath.Join(dir, "refs"), ".git/refs", maxGitGuardRefs))
	}
	return g
}

func revertGitWrites(g *gitGuard) error {
	if g == nil {
		return nil
	}
	var reverted []string
	reverted = append(reverted, g.restoreHooks()...)
	reverted = append(reverted, restoreGitFile(g.configPath, g.config, ".git/config")...)
	if g.wtConfigPath != "" {
		reverted = append(reverted, restoreGitFile(g.wtConfigPath, g.wtConfig, ".git/config")...)
	}
	reverted = append(reverted, g.restoreObjects()...)
	for _, w := range g.files {
		reverted = append(reverted, w.restore()...)
	}
	for _, tr := range g.trees {
		if !tr.ok {
			continue
		}
		reverted = append(reverted, restoreGitDirTree(tr.dir, tr.snap, tr.existed, tr.prefix)...)
	}
	if len(reverted) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(reverted))
	for _, p := range reverted {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return fmt.Errorf("refusing to persist writes to %s", strings.Join(out, ", "))
}

func (g *gitGuard) restoreHooks() []string {
	return restoreGitDirTree(g.hooksDir, g.hooks, g.hooksExisted, ".git/hooks")
}

func restoreGitDirTree(dir string, snap map[string]gitFileSnap, existed bool, prefix string) []string {
	var reverted []string
	if !existed {
		if _, err := os.Lstat(dir); err == nil {
			if err := os.RemoveAll(dir); err == nil {
				reverted = append(reverted, prefix)
			}
		}
		return reverted
	}
	if st, err := os.Lstat(dir); err == nil && st.Mode()&os.ModeSymlink != 0 {
		_ = os.Remove(dir)
		reverted = append(reverted, prefix)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return reverted
	}
	current := map[string]struct{}{}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		current[rel] = struct{}{}
		was, ok := snap[rel]
		if !ok {
			if os.Remove(path) == nil {
				reverted = append(reverted, prefix+"/"+rel)
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			_ = os.Remove(path)
			if writeGitFile(path, was) == nil {
				reverted = append(reverted, prefix+"/"+rel)
			}
			return nil
		}
		data, err := os.ReadFile(path)
		info, _ := d.Info()
		if err != nil || !bytes.Equal(data, was.data) || (info != nil && info.Mode().Perm() != was.mode.Perm()) {
			if writeGitFile(path, was) == nil {
				reverted = append(reverted, prefix+"/"+rel)
			}
		}
		return nil
	})
	for rel, was := range snap {
		if _, ok := current[rel]; ok {
			continue
		}
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if writeGitFile(path, was) == nil {
			reverted = append(reverted, prefix+"/"+rel)
		}
	}
	return reverted
}

func watchGitFile(path, label string) gitFileWatch {
	w := gitFileWatch{path: path, label: label}
	st, err := os.Lstat(path)
	if err != nil {
		w.ok = true
		return w
	}
	if !st.Mode().IsRegular() || st.Size() > maxGitGuardFile {
		return w
	}
	w.snap = readGitFile(path)
	w.ok = w.snap != nil
	return w
}

func (w gitFileWatch) restore() []string {
	if !w.ok {
		return nil
	}
	return restoreGitFile(w.path, w.snap, w.label)
}

func watchGitTree(dir, prefix string, limit int) gitTreeWatch {
	w := gitTreeWatch{dir: dir, prefix: prefix}
	st, err := os.Lstat(dir)
	if err != nil {
		w.ok = true
		return w
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return w
	}
	w.existed = true
	w.snap = snapshotGitTree(dir, limit)
	w.ok = len(w.snap) < limit
	return w
}

func (g *gitGuard) restoreObjects() []string {
	if !g.objectsOK {
		return nil
	}
	if st, err := os.Lstat(g.objectsDir); err != nil {
		return nil
	} else if st.Mode()&os.ModeSymlink != 0 {
		_ = os.Remove(g.objectsDir)
		_ = os.MkdirAll(g.objectsDir, 0o755)
		return []string{".git/objects"}
	}
	var reverted []string
	_ = filepath.WalkDir(g.objectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(g.objectsDir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if _, ok := g.objects[rel]; ok {
			return nil
		}
		if os.Remove(path) == nil {
			reverted = append(reverted, ".git/objects/"+rel)
		}
		return nil
	})
	return reverted
}

func restoreGitFile(path string, snap *gitFileSnap, label string) []string {
	if snap == nil {
		if _, err := os.Lstat(path); err == nil {
			if os.Remove(path) == nil {
				return []string{label}
			}
		}
		return nil
	}
	if st, err := os.Lstat(path); err == nil && st.Mode()&os.ModeSymlink != 0 {
		_ = os.Remove(path)
		if writeGitFile(path, *snap) == nil {
			return []string{label}
		}
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, snap.data) {
		if writeGitFile(path, *snap) == nil {
			return []string{label}
		}
	}
	return nil
}

func readGitFile(path string) *gitFileSnap {
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() {
		return nil
	}
	if st.Size() > maxGitGuardFile {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > maxGitGuardFile {
		return nil
	}
	return &gitFileSnap{mode: st.Mode(), data: data}
}

func writeGitFile(path string, snap gitFileSnap) error {
	mode := snap.mode.Perm()
	if mode == 0 {
		mode = 0o644
	}
	return writeAtomic(path, snap.data, mode)
}

func snapshotGitTree(root string, limit int) map[string]gitFileSnap {
	out := map[string]gitFileSnap{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || len(out) >= limit {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if snap := readGitFile(path); snap != nil {
			out[filepath.ToSlash(rel)] = *snap
		}
		return nil
	})
	return out
}

func listGitTree(root string, limit int) map[string]struct{} {
	out := map[string]struct{}{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || len(out) >= limit {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		out[filepath.ToSlash(rel)] = struct{}{}
		return nil
	})
	return out
}

func gitCommonDir(workdir string) string {
	dir := resolveGitDir(workdir)
	if dir == "" {
		return ""
	}
	if data, err := os.ReadFile(filepath.Join(dir, "commondir")); err == nil {
		p := strings.TrimSpace(string(data))
		if p != "" {
			if !filepath.IsAbs(p) {
				p = filepath.Join(dir, p)
			}
			if abs, err := filepath.Abs(p); err == nil {
				if real, err := filepath.EvalSymlinks(abs); err == nil {
					abs = real
				}
				if st, err := os.Stat(abs); err == nil && st.IsDir() {
					return abs
				}
			}
		}
	}
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		return real
	}
	return dir
}

func resolveGitDir(workdir string) string {
	abs, err := filepath.Abs(workdir)
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	git := filepath.Join(abs, ".git")
	st, err := os.Lstat(git)
	if err != nil {
		return ""
	}
	if st.Mode()&os.ModeSymlink != 0 {
		real, err := filepath.EvalSymlinks(git)
		if err != nil {
			return ""
		}
		info, err := os.Stat(real)
		if err != nil || !info.IsDir() {
			return ""
		}
		return real
	}
	if st.IsDir() {
		return git
	}
	data, err := os.ReadFile(git)
	if err != nil || len(data) > 512 {
		return ""
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(strings.ToLower(line), "gitdir:") {
		return ""
	}
	p := strings.TrimSpace(line[len("gitdir:"):])
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(abs, p)
	}
	p, err = filepath.Abs(p)
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		p = real
	}
	info, err := os.Stat(p)
	if err != nil || !info.IsDir() {
		return ""
	}
	return p
}
