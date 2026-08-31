package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxBytes = 16 << 10

type Skill struct {
	Name string
	Path string
	Body string
}

func Load(workdir string) []Skill {
	var dirs []string
	if workdir != "" {
		dirs = append(dirs, filepath.Join(workdir, ".goppi", "skills"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".config", "goppi", "skills"))
	}
	seen := map[string]bool{}
	var out []Skill
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if seen[name] {
				continue
			}
			path := filepath.Join(dir, name, "SKILL.md")
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			body := strings.TrimSpace(string(data))
			if body == "" {
				continue
			}
			if len(body) > maxBytes {
				body = body[:maxBytes] + "\n… truncated"
			}
			seen[name] = true
			out = append(out, Skill{Name: name, Path: path, Body: body})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func Names(list []Skill) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		out = append(out, s.Name)
	}
	return out
}

func Lookup(list []Skill, name string) (Skill, bool) {
	for _, s := range list {
		if s.Name == name {
			return s, true
		}
	}
	return Skill{}, false
}
