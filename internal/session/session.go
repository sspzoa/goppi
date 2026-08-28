package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"crypto/rand"
	"encoding/hex"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
)

type File struct {
	ID        string             `json:"id"`
	Title     string             `json:"title,omitempty"`
	UpdatedAt time.Time          `json:"updated_at"`
	WorkDir   string             `json:"workdir"`
	Model     string             `json:"model"`
	Effort    string             `json:"reasoning_effort,omitempty"`
	CacheKey  string             `json:"prompt_cache_key,omitempty"`
	Messages  []provider.Message `json:"messages"`
}

func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func dir() (string, error) {
	root, err := config.UserDataDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(root, "sessions")
	return d, os.MkdirAll(d, 0o755)
}

func pathFor(id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("empty session id")
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
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	ptr, err := lastPointer()
	if err != nil {
		return err
	}
	return os.WriteFile(ptr, []byte(f.ID+"\n"), 0o644)
}

func Load(id string) (File, error) {
	var f File
	path, err := pathFor(id)
	if err != nil {
		return f, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, err
	}
	if f.ID == "" {
		f.ID = id
	}
	return f, nil
}

func LoadLast() (File, error) {
	ptr, err := lastPointer()
	if err != nil {
		return File{}, err
	}
	data, err := os.ReadFile(ptr)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return Load(id)
		}
	}
	// migrate leftover last.json
	legacy := File{}
	root, err := config.UserDataDir()
	if err != nil {
		return File{}, err
	}
	raw, err := os.ReadFile(filepath.Join(root, "last.json"))
	if err != nil {
		return File{}, fmt.Errorf("no previous session")
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return File{}, err
	}
	if legacy.ID == "" {
		legacy.ID = NewID()
	}
	if err := Save(legacy); err != nil {
		return File{}, err
	}
	return legacy, nil
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
	if err := os.Remove(path); err != nil {
		return err
	}
	ptr, err := lastPointer()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(ptr)
	if err == nil && strings.TrimSpace(string(data)) == id {
		_ = os.Remove(ptr)
	}
	return nil
}

func Persist(cfg config.Config, id string, messages []provider.Message) (string, error) {
	if id == "" {
		id = NewID()
	}
	err := Save(File{
		ID:       id,
		Title:    TitleFrom(messages),
		WorkDir:  cfg.WorkDir,
		Model:    cfg.Model,
		Effort:   cfg.ReasoningEffort,
		CacheKey: cfg.PromptCacheKey,
		Messages: messages,
	})
	return id, err
}

func TitleFrom(messages []provider.Message) string {
	for _, m := range messages {
		if m.Role == provider.RoleUser && strings.TrimSpace(m.Content) != "" {
			t := strings.Join(strings.Fields(m.Content), " ")
			if len([]rune(t)) > 60 {
				return string([]rune(t)[:60]) + "…"
			}
			return t
		}
	}
	return "untitled"
}

func ExportMarkdown(f File) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", f.Title)
	fmt.Fprintf(&b, "- id: `%s`\n", f.ID)
	fmt.Fprintf(&b, "- model: %s\n", f.Model)
	fmt.Fprintf(&b, "- workdir: `%s`\n", f.WorkDir)
	if !f.UpdatedAt.IsZero() {
		fmt.Fprintf(&b, "- updated: %s\n", f.UpdatedAt.Format(time.RFC3339))
	}
	b.WriteString("\n")
	for _, m := range f.Messages {
		switch m.Role {
		case provider.RoleUser:
			fmt.Fprintf(&b, "## user\n\n%s\n\n", m.Content)
		case provider.RoleAssistant:
			if m.Reasoning != "" {
				fmt.Fprintf(&b, "<details><summary>reasoning</summary>\n\n%s\n\n</details>\n\n", m.Reasoning)
			}
			if strings.TrimSpace(m.Content) != "" {
				fmt.Fprintf(&b, "## assistant\n\n%s\n\n", m.Content)
			}
			for _, tc := range m.ToolCalls {
				fmt.Fprintf(&b, "```tool %s\n%s\n```\n\n", tc.Name, tc.Input)
			}
		}
	}
	return b.String()
}
