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

const maxReadBytes = 512 * 1024

type readFile struct{ workdir string }

func (readFile) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "read_file",
		Description: "Read a UTF-8 text file. Returns numbered lines. Use offset/limit for long files.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"path":{"type":"string","description":"File path, relative to workdir or absolute"},
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
	path, err := resolve(t.workdir, args.Path)
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

type writeFile struct{ workdir string }

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
	path, err := resolve(t.workdir, args.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(args.Contents), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(args.Contents), path), nil
}

type editFile struct{ workdir string }

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
	path, err := resolve(t.workdir, args.Path)
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
	updated := bytes.Replace(data, []byte(args.OldString), []byte(args.NewString), 1)
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("edited %s", path), nil
}

type globFiles struct{ workdir string }

func (globFiles) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "glob",
		Description: "Find files under workdir matching a glob. ** is supported.",
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
	matches, err := walkMatch(t.workdir, args.Pattern, 200)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "(no matches)", nil
	}
	return strings.Join(matches, "\n"), nil
}

type grepFiles struct{ workdir string }

func (grepFiles) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "grep",
		Description: "Search file contents with a regular expression. Skips binary-looking files and common junk dirs.",
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
	root := t.workdir
	if args.Path != "" {
		root, err = resolve(t.workdir, args.Path)
		if err != nil {
			return "", err
		}
	}
	var hits []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
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
			if strings.ContainsRune(line, 0) {
				return nil
			}
			if re.MatchString(line) {
				rel := path
				if r, err := filepath.Rel(t.workdir, path); err == nil {
					rel = r
				}
				hits = append(hits, fmt.Sprintf("%s:%d:%s", rel, n, line))
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
	if len(hits) == 0 {
		return "(no matches)", nil
	}
	return strings.Join(hits, "\n"), nil
}

func resolve(workdir, p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(workdir, p)
	}
	return filepath.Abs(p)
}

func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "bin", ".goppi":
		return true
	}
	return false
}

func walkMatch(root, pattern string, limit int) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
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
