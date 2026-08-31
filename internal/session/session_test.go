package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
)

func TestPersistRecordsWorktree(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = "/tmp/wt/abc"
	cfg.Worktree = true
	cfg.Model = "solar-pro4"
	id, err := Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "isolated"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Worktree != "/tmp/wt/abc" {
		t.Fatalf("worktree %q", got.Worktree)
	}
}

func TestPersistRecordsExtraDirs(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	extra := t.TempDir()
	cfg.ExtraDirs = []string{extra}
	cfg.Model = "solar-pro4"
	id, err := Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "extra roots"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ExtraDirs) != 1 || got.ExtraDirs[0] != extra {
		t.Fatalf("extra_dirs %v want [%s]", got.ExtraDirs, extra)
	}
}

func TestPersistListDelete(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = "/tmp/proj"
	cfg.Model = "solar-pro4"
	id, err := Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "hello world"},
	})
	if err != nil {
		t.Fatal(err)
	}
	items, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != id {
		t.Fatalf("list = %+v", items)
	}
	if items[0].Title != "hello world" {
		t.Fatalf("title = %q", items[0].Title)
	}
	last, err := LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if last.ID != id {
		t.Fatalf("last = %s", last.ID)
	}
	if err := Delete(id); err != nil {
		t.Fatal(err)
	}
	items, err = List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty, got %+v", items)
	}
}

func TestLoadLastRejectsBadPointer(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	if err := os.WriteFile(filepath.Join(root, "last"), []byte("../etc/passwd\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLast(); err == nil {
		t.Fatal("expected invalid last pointer")
	}
}

func TestLoadLastFallsBackWhenPointerStale(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	cfg := config.Default()
	older, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "older"}})
	if err != nil {
		t.Fatal(err)
	}
	newer, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "newer"}})
	if err != nil {
		t.Fatal(err)
	}
	path, err := pathFor(newer)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != older {
		t.Fatalf("got %s want %s", got.ID, older)
	}
	ptr, err := lastPointer()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(ptr)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != older {
		t.Fatalf("last pointer %q", data)
	}
}

func TestLoadLastFallsBackAfterDelete(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	first, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "keep"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "gone"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := Delete(second); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != first {
		t.Fatalf("got %s want %s", got.ID, first)
	}
}

func TestLoadUsesFilenameID(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	id := "cccccccccccccccc"
	dir := filepath.Join(root, "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(`{"id":"dddddddddddddddd","messages":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id {
		t.Fatalf("id %q", got.ID)
	}
}

func TestProblemsFindsCorrupt(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	cfg := config.Default()
	if _, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "ok"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sessions", "aaaaaaaaaaaaaaaa.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	probs := Problems()
	if len(probs) != 1 || !strings.Contains(probs[0], "aaaaaaaaaaaaaaaa.json") {
		t.Fatalf("%v", probs)
	}
}

func TestValidID(t *testing.T) {
	id := NewID()
	if !ValidID(id) {
		t.Fatalf("NewID %q is not valid", id)
	}
	for _, bad := range []string{"", "abc", "../tmp/x", "ABCDEF0123456789", "7a3f2c18123456789", "/etc/passwd"} {
		if ValidID(bad) {
			t.Fatalf("%q should be invalid", bad)
		}
		if _, err := pathFor(bad); err == nil {
			t.Fatalf("pathFor(%q) should fail", bad)
		}
	}
}

func TestSessionFileIsPrivate(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	id, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	path, err := pathFor(id)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("session mode %o, want 0600", st.Mode().Perm())
	}
}

func TestPersistOmitsImageBytes(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	id, err := Persist(cfg, "", []provider.Message{{
		Role:    provider.RoleUser,
		Content: "look",
		Images: []provider.Image{{
			Path: "shot.png",
			MIME: "image/png",
			URL:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB",
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	path, err := pathFor(id)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "base64") || strings.Contains(string(raw), "iVBORw0K") {
		t.Fatalf("session leaked image bytes:\n%s", raw)
	}
	got, err := Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 1 || len(got.Messages[0].Images) != 1 || got.Messages[0].Images[0].Path != "shot.png" {
		t.Fatalf("%+v", got.Messages)
	}
	if got.Messages[0].Images[0].URL != "" {
		t.Fatal("reloaded session should not keep data URL")
	}
	md := ExportMarkdown(got)
	if !strings.Contains(md, "shot.png") {
		t.Fatalf("export missing image path:\n%s", md)
	}
}

func TestListSkipsCorruptSession(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	cfg := config.Default()
	id, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "keep"}})
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(root, "sessions", "aaaaaaaaaaaaaaaa.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != id {
		t.Fatalf("list %+v", items)
	}
}

func TestLoadRejectsHugeSession(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	id := "bbbbbbbbbbbbbbbb"
	dir := filepath.Join(root, "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), make([]byte, maxSessionBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(id); err == nil {
		t.Fatal("expected too large")
	}
}

func TestResolvePrefix(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	id, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "pref"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(id[:8])
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id || got.Title != "pref" {
		t.Fatalf("%+v", got)
	}
	if _, err := Resolve("deadbeef"); err == nil {
		t.Fatal("missing id")
	}
	if _, err := Resolve("../etc/passwd"); err == nil {
		t.Fatal("traversal")
	}
}

func TestResolveAmbiguous(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	if _, err := Persist(cfg, "aaaaaaaaaaaaaaaa", []provider.Message{{Role: provider.RoleUser, Content: "a"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Persist(cfg, "aaaaaaaaaaaaaaab", []provider.Message{{Role: provider.RoleUser, Content: "b"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve("aaaaaaaa"); err == nil {
		t.Fatal("expected ambiguous")
	}
}

func TestPersistRoundTripUsage(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	id, err := PersistFull(cfg, File{
		Messages:   []provider.Message{{Role: provider.RoleUser, Content: "tokens"}},
		Usage:      provider.Usage{InputTokens: 12, OutputTokens: 3, ReasoningTokens: 1},
		TotalUsage: provider.Usage{InputTokens: 40, OutputTokens: 8, ReasoningTokens: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Usage.InputTokens != 12 || got.Usage.ReasoningTokens != 1 {
		t.Fatalf("usage %+v", got.Usage)
	}
	if got.TotalUsage.InputTokens != 40 || got.TotalUsage.OutputTokens != 8 {
		t.Fatalf("total %+v", got.TotalUsage)
	}
	md := ExportMarkdown(got)
	if !strings.Contains(md, "tokens last: 12→3 r1") || !strings.Contains(md, "tokens Σ: 40→8 r2") {
		t.Fatalf("%s", md)
	}
}

func TestDeleteMissingAndExport(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	if err := Delete("0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	if err := Delete("../etc/passwd"); err == nil {
		t.Fatal("invalid id")
	}
	cfg := config.Default()
	id, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "export me"}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := Load(id)
	if err != nil {
		t.Fatal(err)
	}
	path, err := WriteMarkdown(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := Delete(id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("export should be removed")
	}
	if _, err := Load(id); err == nil {
		t.Fatal("session should be gone")
	}
	if err := Delete(id); err != nil {
		t.Fatal(err)
	}
}

func TestWriteMarkdownIsPrivate(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	id, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "export me"}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := Load(id)
	if err != nil {
		t.Fatal(err)
	}
	path, err := WriteMarkdown(f)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != id+".md" {
		t.Fatalf("path %s", path)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("export mode %04o", st.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "export me") {
		t.Fatalf("%s", data)
	}
}

func TestDirRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	target := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "sessions")); err != nil {
		t.Fatal(err)
	}
	if _, err := dir(); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("got %v", err)
	}
}

func TestWriteMarkdownRejectsExportSymlink(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	target := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "exports")); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteMarkdown(File{ID: "aaaaaaaaaaaaaaaa", Title: "x"}); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("got %v", err)
	}
}

func TestLoadRejectsTraversal(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	if _, err := Load("../../../tmp/evil"); err == nil {
		t.Fatal("traversal id should fail")
	}
}

func TestLoadRejectsSymlink(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	id, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "real"}})
	if err != nil {
		t.Fatal(err)
	}
	root, err := config.UserDataDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "sessions", id+".json")
	decoy := filepath.Join(root, "decoy.json")
	if err := os.WriteFile(decoy, []byte(`{"id":"`+id+`","messages":[{"role":"user","content":"via-link"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(decoy, path); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(id); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("got %v", err)
	}
}

func TestLoadLastIgnoresSymlinkPointer(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	id, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "kept"}})
	if err != nil {
		t.Fatal(err)
	}
	root, err := config.UserDataDir()
	if err != nil {
		t.Fatal(err)
	}
	ptr := filepath.Join(root, "last")
	outside := filepath.Join(root, "last-link")
	if err := os.WriteFile(outside, []byte("ffffffffffffffff\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(ptr); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, ptr); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id {
		t.Fatalf("followed last symlink to %s, want %s", got.ID, id)
	}
}

func TestLoadLastIgnoresHugePointer(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	id, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "kept"}})
	if err != nil {
		t.Fatal(err)
	}
	root, err := config.UserDataDir()
	if err != nil {
		t.Fatal(err)
	}
	ptr := filepath.Join(root, "last")
	if err := os.WriteFile(ptr, []byte(strings.Repeat("a", 200)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id {
		t.Fatalf("got %s want %s", got.ID, id)
	}
}

func TestDeleteIgnoresSymlinkLast(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	id, err := Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "gone"}})
	if err != nil {
		t.Fatal(err)
	}
	root, err := config.UserDataDir()
	if err != nil {
		t.Fatal(err)
	}
	ptr := filepath.Join(root, "last")
	outside := filepath.Join(root, "last-link")
	if err := os.WriteFile(outside, []byte(id+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(ptr); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, ptr); err != nil {
		t.Fatal(err)
	}
	if err := Delete(id); err != nil {
		t.Fatal(err)
	}
	st, err := os.Lstat(ptr)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&os.ModeSymlink == 0 {
		t.Fatal("delete followed last symlink")
	}
}

func TestTitleFromRedactsSecrets(t *testing.T) {
	got := TitleFrom([]provider.Message{{
		Role:    provider.RoleUser,
		Content: "use sk-abcdefghijklmnopqrst please",
	}})
	if strings.Contains(got, "sk-abcdefghijklmnopqrst") {
		t.Fatalf("%q", got)
	}
	if !strings.Contains(got, "[redacted]") {
		t.Fatalf("%q", got)
	}
}

func TestSaveRedactsExistingTitle(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	id, err := PersistFull(config.Default(), File{
		ID:    "aaaaaaaaaaaaaaaa",
		Title: "key sk-abcdefghijklmnopqrst",
		Messages: []provider.Message{{
			Role:    provider.RoleUser,
			Content: "hi",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Title, "sk-abcdefghijklmnopqrst") {
		t.Fatalf("disk title leaked: %q", got.Title)
	}
}

func TestExportMarkdownRedactsSecrets(t *testing.T) {
	md := ExportMarkdown(File{
		ID:    "aaaaaaaaaaaaaaaa",
		Title: "key up_abcdefghijklmnopqrst",
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "key up_abcdefghijklmnopqrst"},
			{Role: provider.RoleAssistant, Content: "ok", Reasoning: "up_abcdefghijklmnopqrst", ToolCalls: []provider.ToolCall{{Name: "bash", Input: []byte(`{"c":"up_abcdefghijklmnopqrst"}`)}}},
		},
	})
	if strings.Contains(md, "up_abcdefghijklmnopqrst") {
		t.Fatalf("leaked:\n%s", md)
	}
	if !strings.Contains(md, "[redacted]") {
		t.Fatalf("missing redaction:\n%s", md)
	}
}
