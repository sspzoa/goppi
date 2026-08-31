package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sspzoa/goppi/internal/provider"
)

const (
	maxReadBytes  = 512 * 1024
	maxWriteBytes = 2 << 20
)

type readFile struct {
	workdir string
	root    *fileRoot
}

func (readFile) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "read_file",
		Description: "Read a UTF-8 text file. Returns numbered lines. Use offset/limit for long files.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"path":{"type":"string","description":"File path inside workdir (relative or absolute)"},
				"offset":{"type":"integer","description":"1-based start line"},
				"limit":{"type":"integer","description":"Max lines to return"}
			},
			"required":["path"]
		}`),
	}
}

func (t readFile) Run(_ context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}](input)
	if err != nil {
		return "", err
	}
	path, err := scopedResolve(t.workdir, t.root, args.Path)
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var b strings.Builder
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), maxReadBytes)
	start := 1
	if args.Offset > 0 {
		start = args.Offset
	}
	end := int(^uint(0) >> 1)
	if args.Limit > 0 {
		end = start + args.Limit - 1
	}
	n := 0
	written := 0
	for sc.Scan() {
		n++
		if n < start {
			continue
		}
		if n > end {
			break
		}
		fmt.Fprintf(&b, "%6d|%s\n", n, sc.Text())
		written += len(sc.Text()) + 8
		if written > maxReadBytes {
			fmt.Fprintf(&b, "\n… truncated after %d bytes\n", maxReadBytes)
			break
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	if n == 0 {
		return "(empty file)", nil
	}
	if start > n {
		return "", fmt.Errorf("offset %d past end of file (%d lines)", start, n)
	}
	return b.String(), nil
}

type writeFile struct {
	workdir string
	root    *fileRoot
	snaps   *snapStack
}

func (writeFile) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "write_file",
		Description: "Create or overwrite a file with the given contents. Creates parent directories.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"path":{"type":"string"},
				"contents":{"type":"string"}
			},
			"required":["path","contents"]
		}`),
	}
}

func (t writeFile) Run(_ context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path     string `json:"path"`
		Contents string `json:"contents"`
	}](input)
	if err != nil {
		return "", err
	}
	if len(args.Contents) > maxWriteBytes {
		return "", fmt.Errorf("contents exceed %d bytes", maxWriteBytes)
	}
	path, err := scopedResolveWrite(t.workdir, t.root, args.Path)
	if err != nil {
		return "", err
	}
	t.snaps.remember(path)
	if err := writeAtomic(path, []byte(args.Contents), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(args.Contents), path), nil
}

type editFile struct {
	workdir string
	root    *fileRoot
	snaps   *snapStack
}

func (editFile) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "edit_file",
		Description: "Replace exactly one occurrence of old_string with new_string in a file. Fails if the string is missing or not unique.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"path":{"type":"string"},
				"old_string":{"type":"string"},
				"new_string":{"type":"string"}
			},
			"required":["path","old_string","new_string"]
		}`),
	}
}

func (t editFile) Run(_ context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}](input)
	if err != nil {
		return "", err
	}
	if args.OldString == "" {
		return "", fmt.Errorf("old_string is empty")
	}
	path, err := scopedResolveWrite(t.workdir, t.root, args.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	n := bytes.Count(data, []byte(args.OldString))
	if n == 0 {
		return "", fmt.Errorf("old_string not found in %s", path)
	}
	if n > 1 {
		return "", fmt.Errorf("old_string found %d times in %s; make it unique", n, path)
	}
	t.snaps.remember(path)
	updated := bytes.Replace(data, []byte(args.OldString), []byte(args.NewString), 1)
	if len(updated) > maxWriteBytes {
		return "", fmt.Errorf("result would exceed %d bytes", maxWriteBytes)
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := writeAtomic(path, updated, mode); err != nil {
		return "", err
	}
	return fmt.Sprintf("edited %s", path), nil
}

type globFiles struct {
	workdir string
	root    *fileRoot
}

func (globFiles) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "glob",
		Description: "Find files under workdir matching a glob. ** is supported. Honors .gitignore files from the workdir down.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"pattern":{"type":"string","description":"e.g. **/*.go or internal/**/*.go"}
			},
			"required":["pattern"]
		}`),
	}
}

func (t globFiles) Run(_ context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Pattern string `json:"pattern"`
	}](input)
	if err != nil {
		return "", err
	}
	var matches []string
	seen := map[string]bool{}
	remain := 200
	for _, dir := range searchRoots(t.workdir, t.root) {
		if remain <= 0 {
			break
		}
		found, err := walkMatch(dir, args.Pattern, remain, loadIgnore(dir))
		if err != nil {
			return "", err
		}
		for _, rel := range found {
			abs := rel
			if !filepath.IsAbs(rel) {
				abs = filepath.Join(dir, rel)
			}
			show := displayRel(t.workdir, abs)
			if seen[show] {
				continue
			}
			seen[show] = true
			matches = append(matches, show)
			remain--
			if remain <= 0 {
				break
			}
		}
	}
	if len(matches) == 0 {
		return "(no matches)", nil
	}
	return strings.Join(matches, "\n"), nil
}

type grepFiles struct {
	workdir string
	root    *fileRoot
}

func (grepFiles) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "grep",
		Description: "Search file contents with a regular expression. Skips binary-looking files, common junk dirs, and .gitignore from the workdir down. Pass path to search an ignored file.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"pattern":{"type":"string","description":"Go regexp"},
				"path":{"type":"string","description":"Directory or file to search. Defaults to workdir"},
				"glob":{"type":"string","description":"Optional filename glob filter, e.g. *.go"}
			},
			"required":["pattern"]
		}`),
	}
}

func (t grepFiles) Run(_ context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Glob    string `json:"glob"`
	}](input)
	if err != nil {
		return "", err
	}
	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return "", fmt.Errorf("regexp: %w", err)
	}
	var roots []string
	if args.Path != "" {
		root, err := scopedResolve(t.workdir, t.root, args.Path)
		if err != nil {
			return "", err
		}
		roots = []string{root}
	} else {
		roots = searchRoots(t.workdir, t.root)
	}
	var hits []string
	for _, root := range roots {
		if len(hits) >= 80 {
			break
		}
		ign := loadIgnore(root)
		if args.Path != "" {
			ign = loadIgnore(t.workdir)
		}
		searchFile := false
		if st, err := os.Stat(root); err == nil && !st.IsDir() {
			searchFile = true
		}
		err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel := path
			if r, err := filepath.Rel(root, path); err == nil {
				rel = r
			}
			if args.Path == "" && !searchFile && ign.ignored(rel, d.IsDir()) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if path != root && skipDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if args.Glob != "" {
				ok, _ := filepath.Match(args.Glob, d.Name())
				if !ok {
					return nil
				}
			}
			if len(hits) >= 80 {
				return io.EOF
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			sc := bufio.NewScanner(f)
			n := 0
			for sc.Scan() {
				n++
				line := sc.Text()
				if len(line) > 8*1024 {
					line = line[:8*1024] + "…"
				}
				if strings.ContainsRune(line, 0) {
					return nil
				}
				if re.MatchString(line) {
					hits = append(hits, fmt.Sprintf("%s:%d:%s", displayRel(t.workdir, path), n, line))
					if len(hits) >= 80 {
						return io.EOF
					}
				}
			}
			return nil
		})
		if err != nil && err != io.EOF {
			return "", err
		}
	}
	if len(hits) == 0 {
		return "(no matches)", nil
	}
	return strings.Join(hits, "\n"), nil
}

func resolve(workdir, p string) (string, error) {
	return resolveAny([]string{workdir}, p)
}

func resolveAny(roots []string, p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if len(roots) == 0 || strings.TrimSpace(roots[0]) == "" {
		return "", fmt.Errorf("empty path")
	}
	primary, err := absRoot(roots[0])
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(primary, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	abs = followExisting(abs)
	for _, raw := range roots {
		root, err := absRoot(raw)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue
		}
		return abs, nil
	}
	return "", fmt.Errorf("path %q is outside workdir", p)
}

func absRoot(workdir string) (string, error) {
	root, err := filepath.Abs(workdir)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	return root, nil
}

func resolveWrite(workdir, p string) (string, error) {
	abs, err := resolve(workdir, p)
	if err != nil {
		return "", err
	}
	if err := denyGitWrite(workdir, abs); err != nil {
		return "", err
	}
	return abs, nil
}

func denyGitWrite(workdir, abs string) error {
	root, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	if rel == ".git" || strings.HasPrefix(rel, ".git/") || strings.Contains(rel, "/.git/") || strings.HasSuffix(rel, "/.git") {
		return fmt.Errorf("refusing to write %s", rel)
	}
	return nil
}

// followExisting resolves symlinks on the longest existing prefix so a
// link inside workdir cannot point a read or write outside it.
func followExisting(p string) string {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	dir, base := filepath.Dir(p), filepath.Base(p)
	for dir != filepath.Dir(dir) {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(real, base)
		}
		base = filepath.Join(filepath.Base(dir), base)
		dir = filepath.Dir(dir)
	}
	return filepath.Clean(p)
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".goppi-write-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(name, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "bin", ".goppi",
		".svn", ".hg", "__pycache__", ".venv", "venv", "target", "coverage":
		return true
	}
	return false
}

func walkMatch(root, pattern string, limit int, ign *ignoreSet) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		if path != root && ign.ignored(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		ok, err := matchGlob(pattern, rel)
		if err != nil || !ok {
			return nil
		}
		matches = append(matches, rel)
		if len(matches) >= limit {
			return io.EOF
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, err
	}
	return matches, nil
}

func matchGlob(pattern, name string) (bool, error) {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)
	if !strings.Contains(pattern, "**") {
		return filepath.Match(pattern, name)
	}
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return false, fmt.Errorf("only one ** is supported")
	}
	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := strings.TrimPrefix(parts[1], "/")
	rest := name
	if prefix != "" {
		if !strings.HasPrefix(name, prefix+"/") && name != prefix {
			return false, nil
		}
		if name == prefix {
			rest = ""
		} else {
			rest = strings.TrimPrefix(name, prefix+"/")
		}
	}
	if suffix == "" {
		return true, nil
	}
	if rest == "" {
		return false, nil
	}
	// suffix may still be a/b/*.go — match against rest and any nested remainder
	if ok, _ := filepath.Match(suffix, rest); ok {
		return true, nil
	}
	for {
		slash := strings.IndexByte(rest, '/')
		if slash < 0 {
			return filepath.Match(suffix, rest)
		}
		rest = rest[slash+1:]
		if ok, _ := filepath.Match(suffix, rest); ok {
			return true, nil
		}
	}
}
