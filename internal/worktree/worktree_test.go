package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sspzoa/goppi/internal/session"
)

func TestEnsureIsolatesWrites(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	repo := initRepo(t)
	id := session.NewID()
	wt, err := Ensure(repo, id)
	if err != nil {
		t.Fatal(err)
	}
	if wt.Path == repo {
		t.Fatal("worktree must not be the main checkout")
	}
	if err := os.WriteFile(filepath.Join(wt.Path, "agent.txt"), []byte("isolated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "agent.txt")); err == nil {
		t.Fatal("main checkout must stay clean")
	}
	again, err := Ensure(repo, id)
	if err != nil {
		t.Fatal(err)
	}
	if again.Path != wt.Path {
		t.Fatalf("reuse %q vs %q", again.Path, wt.Path)
	}
	listed, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != id {
		t.Fatalf("list %+v", listed)
	}
	if err := Remove(id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt.Path); err == nil {
		t.Fatal("worktree path should be gone")
	}
	if _, err := Find(id); err == nil {
		t.Fatal("expected missing after remove")
	}
}

func TestRemoveMissingIsOK(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	if err := Remove(session.NewID()); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureRejectsBadID(t *testing.T) {
	if _, err := Ensure(t.TempDir(), "not-an-id"); err == nil {
		t.Fatal("expected invalid id")
	}
}

func TestEnsureRequiresGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	if _, err := Ensure(t.TempDir(), session.NewID()); err == nil {
		t.Fatal("expected not a git repository")
	}
}

func TestDestPathRejectsWorktreesSymlink(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	target := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "worktrees")); err != nil {
		t.Fatal(err)
	}
	if _, err := destPath("/tmp/repo", session.NewID()); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("got %v", err)
	}
}

func TestGitEnvDropsSecrets(t *testing.T) {
	t.Setenv("UPSTAGE_API_KEY", "up_must_not_appear")
	t.Setenv("GOPPI_API_KEY", "up_also_hidden")
	t.Setenv("GITHUB_TOKEN", "ghp_must_not")
	got := strings.Join(gitEnv(), "\n")
	for _, leak := range []string{"UPSTAGE_API_KEY", "GOPPI_API_KEY", "GITHUB_TOKEN", "up_must_not_appear"} {
		if strings.Contains(got, leak) {
			t.Fatalf("leaked %s", leak)
		}
	}
	if !strings.Contains(got, "GIT_TERMINAL_PROMPT=0") {
		t.Fatal("missing GIT_TERMINAL_PROMPT")
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=goppi", "GIT_AUTHOR_EMAIL=goppi@test",
			"GIT_COMMITTER_NAME=goppi", "GIT_COMMITTER_EMAIL=goppi@test")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init")
	run("config", "user.email", "goppi@test")
	run("config", "user.name", "goppi")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README")
	run("commit", "-m", "init")
	return dir
}
