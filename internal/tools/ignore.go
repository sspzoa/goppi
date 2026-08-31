package tools

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

const maxIgnoreBytes = 256 << 10

type ignoreRule struct {
	neg, dirOnly, anchored bool
	pattern                string
}

type ignoreList []ignoreRule

type ignoreSet struct {
	root  string
	cache map[string]ignoreList
}

func loadIgnore(root string) *ignoreSet {
	return &ignoreSet{root: root, cache: map[string]ignoreList{}}
}

func (s *ignoreSet) layer(dirRel string) ignoreList {
	if s == nil {
		return nil
	}
	dirRel = filepath.ToSlash(dirRel)
	dirRel = strings.TrimPrefix(dirRel, "./")
	if dirRel == "." {
		dirRel = ""
	}
	if got, ok := s.cache[dirRel]; ok {
		return got
	}
	base := s.root
	if dirRel != "" {
		base = filepath.Join(s.root, filepath.FromSlash(dirRel))
	}
	rules := parseIgnoreFile(filepath.Join(base, ".gitignore"))
	if dirRel == "" {
		rules = append(rules, parseIgnoreFile(filepath.Join(s.root, ".git", "info", "exclude"))...)
	}
	if s.cache == nil {
		s.cache = map[string]ignoreList{}
	}
	s.cache[dirRel] = rules
	return rules
}

func parseIgnoreFile(path string) ignoreList {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() || st.Size() > maxIgnoreBytes {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var rules ignoreList
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rule := ignoreRule{}
		if strings.HasPrefix(line, "!") {
			rule.neg = true
			line = strings.TrimPrefix(line, "!")
		}
		if strings.HasPrefix(line, "/") {
			rule.anchored = true
			line = strings.TrimPrefix(line, "/")
		}
		if strings.HasSuffix(line, "/") {
			rule.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		line = filepath.ToSlash(line)
		if line == "" || line == "." {
			continue
		}
		rule.pattern = line
		rules = append(rules, rule)
	}
	return rules
}

func (s *ignoreSet) ignored(rel string, isDir bool) bool {
	if s == nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" || rel == "." {
		return false
	}
	parts := strings.Split(rel, "/")
	acc := ""
	for i, p := range parts {
		if skipDir(p) {
			return true
		}
		if acc == "" {
			acc = p
		} else {
			acc += "/" + p
		}
		dir := isDir || i < len(parts)-1
		if s.decision(acc, dir) {
			return true
		}
	}
	return false
}

func (s *ignoreSet) decision(acc string, isDir bool) bool {
	ign := false
	apply := func(dirRel, name string) {
		if name == "" {
			return
		}
		for _, rule := range s.layer(dirRel) {
			if rule.match(name, isDir) {
				ign = !rule.neg
			}
		}
	}
	apply("", acc)
	segs := strings.Split(acc, "/")
	prefix := ""
	for j := 0; j < len(segs)-1; j++ {
		if prefix == "" {
			prefix = segs[j]
		} else {
			prefix += "/" + segs[j]
		}
		apply(prefix, strings.Join(segs[j+1:], "/"))
	}
	return ign
}

func (rule ignoreRule) match(rel string, isDir bool) bool {
	if rule.dirOnly && !isDir && rel == rule.pattern {
		return false
	}
	if rule.anchored {
		return rel == rule.pattern || strings.HasPrefix(rel, rule.pattern+"/") || globOK(rule.pattern, rel)
	}
	if globOK(rule.pattern, rel) || globOK("**/"+rule.pattern, rel) {
		return true
	}
	base := path.Base(rel)
	if base != rel && globOK(rule.pattern, base) {
		return true
	}
	if strings.Contains(rule.pattern, "/") && strings.HasPrefix(rel, rule.pattern+"/") {
		return true
	}
	return false
}

func globOK(pattern, name string) bool {
	ok, err := matchGlob(pattern, name)
	return err == nil && ok
}
