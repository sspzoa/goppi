package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ApplySandbox(cmd *exec.Cmd, workdir, mode string, extra ...string) error {
	if cmd == nil {
		return nil
	}
	return wrapSandbox(cmd, workdir, normalizeSandbox(mode), extra...)
}

func sandboxOn(mode string) bool {
	return mode != "off"
}

func sandboxNetOff(mode string) bool {
	return mode == "strict"
}

func sandboxWriteRoots(workdir string, extra ...string) []string {
	var roots []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return
		}
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			abs = real
		}
		roots = append(roots, abs)
	}
	add(workdir)
	for _, e := range extra {
		add(e)
	}
	add("/tmp")
	add("/private/tmp")
	add("/var/tmp")
	add(os.TempDir())
	add(os.Getenv("TMPDIR"))
	add(os.Getenv("GOTMPDIR"))
	add(os.Getenv("GOCACHE"))
	add(os.Getenv("GOMODCACHE"))
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".cache"))
		add(filepath.Join(home, ".npm"))
		add(filepath.Join(home, "Library", "Caches"))
		add(filepath.Join(home, "go", "pkg", "mod"))
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(roots))
	for _, p := range roots {
		if seen[p] {
			continue
		}
		seen[p] = true
		if _, err := os.Stat(p); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}

func sandboxPrivDeny() []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, p := range []string{
		"/usr/bin/sudo", "/usr/sbin/sudo", "/opt/homebrew/bin/sudo", "/usr/local/bin/sudo",
		"/usr/bin/su", "/bin/su",
		"/usr/bin/doas", "/opt/homebrew/bin/doas", "/usr/local/bin/doas",
	} {
		add(p)
		if real, err := filepath.EvalSymlinks(p); err == nil {
			add(real)
		}
	}
	return out
}

func sandboxGitDeny(workdir string, extra ...string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	addGit := func(root string) {
		if root == "" {
			return
		}
		add(filepath.Join(root, "hooks"))
		add(filepath.Join(root, "config"))
		add(filepath.Join(root, "objects"))
		add(filepath.Join(root, "HEAD"))
		add(filepath.Join(root, "packed-refs"))
		add(filepath.Join(root, "refs"))
	}
	addGit(gitCommonDir(workdir))
	if dir := resolveGitDir(workdir); dir != "" {
		addGit(dir)
	}
	for _, e := range extra {
		addGit(gitCommonDir(e))
		if dir := resolveGitDir(e); dir != "" {
			addGit(dir)
		}
	}
	return out
}
