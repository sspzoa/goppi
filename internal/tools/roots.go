package tools

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

type fileRoot struct {
	mu      sync.Mutex
	primary string
	extra   []string
}

func (f *fileRoot) All() []string {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, 1+len(f.extra))
	out = append(out, f.primary)
	return append(out, f.extra...)
}

func (f *fileRoot) Extra() []string {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.extra) == 0 {
		return nil
	}
	return append([]string{}, f.extra...)
}

func (f *fileRoot) setExtra(dirs []string) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.extra = append([]string{}, dirs...)
}

func (f *fileRoot) Resolve(p string) (string, error) {
	if f == nil {
		return "", fmt.Errorf("empty path")
	}
	return resolveAny(f.All(), p)
}

func (f *fileRoot) ResolveWrite(p string) (string, error) {
	abs, err := f.Resolve(p)
	if err != nil {
		return "", err
	}
	for _, root := range f.All() {
		if err := denyGitWrite(root, abs); err != nil {
			return "", err
		}
	}
	return abs, nil
}

func ResolveInRoots(workdir string, extra []string, p string) (string, error) {
	roots := make([]string, 0, 1+len(extra))
	roots = append(roots, workdir)
	return resolveAny(append(roots, extra...), p)
}

func (r *Registry) SetExtraDirs(dirs []string) {
	if r == nil || r.root == nil {
		return
	}
	r.root.setExtra(dirs)
}

func searchRoots(workdir string, root *fileRoot) []string {
	if root != nil {
		if all := root.All(); len(all) > 0 {
			return all
		}
	}
	return []string{workdir}
}

func displayRel(primary, abs string) string {
	if r, err := filepath.Rel(primary, abs); err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return r
	}
	return abs
}

func scopedResolve(workdir string, root *fileRoot, p string) (string, error) {
	if root != nil {
		return root.Resolve(p)
	}
	return resolve(workdir, p)
}

func scopedResolveWrite(workdir string, root *fileRoot, p string) (string, error) {
	if root != nil {
		return root.ResolveWrite(p)
	}
	return resolveWrite(workdir, p)
}
