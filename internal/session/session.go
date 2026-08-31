package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"crypto/rand"
	"encoding/hex"
	"regexp"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/tools"
)

type File struct {
	ID         string             `json:"id"`
	Title      string             `json:"title,omitempty"`
	UpdatedAt  time.Time          `json:"updated_at"`
	WorkDir    string             `json:"workdir"`
	ExtraDirs  []string           `json:"extra_dirs,omitempty"`
	Worktree   string             `json:"worktree,omitempty"`
	Model      string             `json:"model"`
	Effort     string             `json:"reasoning_effort,omitempty"`
	CacheKey   string             `json:"prompt_cache_key,omitempty"`
	Mode       string             `json:"mode,omitempty"`
	Todos      []tools.Todo       `json:"todos,omitempty"`
	Usage      provider.Usage     `json:"usage,omitempty"`
	TotalUsage provider.Usage     `json:"total_usage,omitempty"`
	Messages   []provider.Message `json:"messages"`
}

const (
	maxSessionBytes = 8 << 20
	maxLastBytes    = 64
)

var idRe = regexp.MustCompile(`^[0-9a-f]{16}$`)

func ValidID(id string) bool {
	return idRe.MatchString(id)
}

func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		n := uint64(time.Now().UnixNano())
		for i := range b {
			b[i] = byte(n >> (8 * i))
		}
	}
	return hex.EncodeToString(b[:])
}

func NewCacheKey() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("goppi-%d", time.Now().UnixNano())
	}
	return "goppi-" + hex.EncodeToString(b[:])
}

func dir() (string, error) {
	return config.EnsureDataSubdir("sessions")
}

func pathFor(id string) (string, error) {
	if !ValidID(id) {
		return "", fmt.Errorf("invalid session id")
	}
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, id+".json"), nil
}

func lastPointer() (string, error) {
	root, err := config.UserDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "last"), nil
}

func Save(f File) error {
	if f.ID == "" {
		f.ID = NewID()
	}
	if f.Title == "" {
		f.Title = TitleFrom(f.Messages)
	} else {
		f.Title = SafeTitle(f.Title)
	}
	f.UpdatedAt = time.Now()
	path, err := pathFor(f.ID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > maxSessionBytes {
		return fmt.Errorf("session too large to save (%d bytes, max %d)", len(data), maxSessionBytes)
	}
	if err := writeAtomic(path, data, 0o600); err != nil {
		return err
	}
	ptr, err := lastPointer()
	if err != nil {
		return err
	}
	return writeAtomic(ptr, []byte(f.ID+"\n"), 0o600)
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".goppi-*.tmp")
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

func Resolve(q string) (File, error) {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return File{}, fmt.Errorf("empty session id")
	}
	if ValidID(q) {
		return Load(q)
	}
	if strings.ContainsAny(q, `/\`) || strings.Contains(q, "..") {
		return File{}, fmt.Errorf("invalid session id")
	}
	items, err := List()
	if err != nil {
		return File{}, err
	}
	var hits []File
	for _, f := range items {
		if strings.HasPrefix(f.ID, q) {
			hits = append(hits, f)
		}
	}
	switch len(hits) {
	case 0:
		return File{}, fmt.Errorf("session %s: not found", q)
	case 1:
		return hits[0], nil
	default:
		return File{}, fmt.Errorf("session %s: %d matches", q, len(hits))
	}
}

func Load(id string) (File, error) {
	var f File
	path, err := pathFor(id)
	if err != nil {
		return f, err
	}
	if err := config.SecretLinkError(path); err != nil {
		return f, err
	}
	st, err := os.Lstat(path)
	if err != nil {
		return f, err
	}
	if st.Size() > maxSessionBytes {
		return f, fmt.Errorf("session %s too large (%d bytes)", id, st.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("session %s: %w", id, err)
	}
	f.ID = id
	return f, nil
}

var errInvalidLast = errors.New("invalid last pointer")

func LoadLast() (File, error) {
	if f, err := loadPointer(); err == nil {
		return f, nil
	} else if errors.Is(err, errInvalidLast) {
		return File{}, err
	}
	if f, err := loadLegacy(); err == nil {
		return f, nil
	}
	return loadNewest()
}

func loadPointer() (File, error) {
	ptr, err := lastPointer()
	if err != nil {
		return File{}, err
	}
	id, err := readLastID(ptr)
	if err != nil {
		return File{}, err
	}
	return Load(id)
}

func readLastID(ptr string) (string, error) {
	if err := config.SecretLinkError(ptr); err != nil {
		return "", err
	}
	st, err := os.Lstat(ptr)
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("last pointer is not a regular file")
	}
	if st.Size() > maxLastBytes {
		return "", fmt.Errorf("last pointer too large (%d bytes)", st.Size())
	}
	data, err := os.ReadFile(ptr)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", fmt.Errorf("empty last pointer")
	}
	if !ValidID(id) {
		return "", errInvalidLast
	}
	return id, nil
}

func loadLegacy() (File, error) {
	root, err := config.UserDataDir()
	if err != nil {
		return File{}, err
	}
	legacyPath := filepath.Join(root, "last.json")
	if err := config.SecretLinkError(legacyPath); err != nil {
		return File{}, err
	}
	raw, err := os.ReadFile(legacyPath)
	if err != nil {
		return File{}, err
	}
	var legacy File
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return File{}, err
	}
	if legacy.ID == "" {
		legacy.ID = NewID()
	}
	if !ValidID(legacy.ID) {
		legacy.ID = NewID()
	}
	if err := Save(legacy); err != nil {
		return File{}, err
	}
	return legacy, nil
}

func loadNewest() (File, error) {
	items, err := List()
	if err != nil {
		return File{}, err
	}
	if len(items) == 0 {
		return File{}, fmt.Errorf("no previous session")
	}
	f := items[0]
	if ptr, err := lastPointer(); err == nil {
		_ = writeAtomic(ptr, []byte(f.ID+"\n"), 0o600)
	}
	return f, nil
}

func Problems() []string {
	d, err := dir()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []string{err.Error()}
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []string{err.Error()}
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		if !ValidID(id) {
			out = append(out, e.Name()+": invalid id")
			continue
		}
		if _, err := Load(id); err != nil {
			out = append(out, e.Name()+": "+err.Error())
		}
	}
	return out
}

func List() ([]File, error) {
	d, err := dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		return nil, err
	}
	var out []File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		f, err := Load(id)
		if err != nil {
			continue
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func Delete(id string) error {
	path, err := pathFor(id)
	if err != nil {
		return err
	}
	lk, err := Hold(id)
	if err != nil {
		return err
	}
	defer func() {
		lk.Release()
		if lp, err := lockPath(id); err == nil {
			_ = os.Remove(lp)
		}
	}()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if root, err := config.UserDataDir(); err == nil {
		_ = os.Remove(filepath.Join(root, "exports", id+".md"))
	}
	ptr, err := lastPointer()
	if err != nil {
		return nil
	}
	got, err := readLastID(ptr)
	if err == nil && got == id {
		_ = os.Remove(ptr)
	}
	return nil
}

func Persist(cfg config.Config, id string, messages []provider.Message) (string, error) {
	return PersistFull(cfg, File{ID: id, Messages: messages})
}

func PersistFull(cfg config.Config, f File) (string, error) {
	if f.ID == "" {
		f.ID = NewID()
	}
	if f.Title == "" {
		f.Title = TitleFrom(f.Messages)
	}
	if f.Mode == "" {
		f.Mode = cfg.Mode
	}
	f.WorkDir = cfg.WorkDir
	if len(cfg.ExtraDirs) > 0 {
		f.ExtraDirs = append([]string(nil), cfg.ExtraDirs...)
	}
	if cfg.Worktree {
		f.Worktree = cfg.WorkDir
	}
	f.Model = cfg.Model
	f.Effort = cfg.ReasoningEffort
	if cfg.PromptCacheKey != "" {
		f.CacheKey = cfg.PromptCacheKey
	}
	err := Save(f)
	return f.ID, err
}

func SafeTitle(s string) string {
	return tools.RedactSecrets(s)
}

func TitleFrom(messages []provider.Message) string {
	for _, m := range messages {
		if m.Role == provider.RoleUser && strings.TrimSpace(m.Content) != "" {
			t := strings.Join(strings.Fields(SafeTitle(m.Content)), " ")
			if t == "" {
				return "untitled"
			}
			if len([]rune(t)) > 60 {
				return string([]rune(t)[:60]) + "…"
			}
			return t
		}
	}
	return "untitled"
}

func WriteMarkdown(f File) (string, error) {
	if !ValidID(f.ID) {
		return "", fmt.Errorf("invalid session id")
	}
	dir, err := config.EnsureDataSubdir("exports")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, f.ID+".md")
	data := []byte(ExportMarkdown(f))
	if len(data) > maxSessionBytes {
		return "", fmt.Errorf("export too large (%d bytes, max %d)", len(data), maxSessionBytes)
	}
	if err := writeAtomic(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func ExportMarkdown(f File) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", SafeTitle(f.Title))
	fmt.Fprintf(&b, "- id: `%s`\n", f.ID)
	fmt.Fprintf(&b, "- model: %s\n", f.Model)
	fmt.Fprintf(&b, "- workdir: `%s`\n", f.WorkDir)
	if !f.UpdatedAt.IsZero() {
		fmt.Fprintf(&b, "- updated: %s\n", f.UpdatedAt.Format(time.RFC3339))
	}
	if f.Usage.InputTokens > 0 || f.Usage.OutputTokens > 0 || f.TotalUsage.InputTokens > 0 {
		fmt.Fprintf(&b, "- tokens last: %d→%d r%d\n", f.Usage.InputTokens, f.Usage.OutputTokens, f.Usage.ReasoningTokens)
		fmt.Fprintf(&b, "- tokens Σ: %d→%d r%d\n", f.TotalUsage.InputTokens, f.TotalUsage.OutputTokens, f.TotalUsage.ReasoningTokens)
	}
	b.WriteString("\n")
	for _, m := range f.Messages {
		switch m.Role {
		case provider.RoleUser:
			fmt.Fprintf(&b, "## user\n\n%s\n\n", tools.RedactSecrets(m.Content))
			for _, img := range m.Images {
				if img.Path != "" {
					fmt.Fprintf(&b, "![%s](%s)\n\n", img.Path, img.Path)
				}
			}
		case provider.RoleAssistant:
			if m.Reasoning != "" {
				fmt.Fprintf(&b, "<details><summary>reasoning</summary>\n\n%s\n\n</details>\n\n", tools.RedactSecrets(m.Reasoning))
			}
			if strings.TrimSpace(m.Content) != "" {
				fmt.Fprintf(&b, "## assistant\n\n%s\n\n", tools.RedactSecrets(m.Content))
			}
			for _, tc := range m.ToolCalls {
				fmt.Fprintf(&b, "```tool %s\n%s\n```\n\n", tc.Name, tools.RedactSecrets(string(tc.Input)))
			}
		}
	}
	return b.String()
}
