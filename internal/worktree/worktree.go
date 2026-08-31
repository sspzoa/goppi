package worktree

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/session"
)

type Checkout struct {
	ID     string
	Path   string
	Branch string
	Repo   string
}

func Branch(id string) string {
	return "goppi/" + id
}

func Ensure(repoDir, id string) (Checkout, error) {
	if !session.ValidID(id) {
		return Checkout{}, fmt.Errorf("invalid session id")
	}
	root, err := toplevel(repoDir)
	if err != nil {
		return Checkout{}, err
	}
	dest, err := destPath(root, id)
	if err != nil {
		return Checkout{}, err
	}
	if isWorktree(dest) {
		return Checkout{ID: id, Path: dest, Branch: Branch(id), Repo: root}, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return Checkout{}, err
	}
	branch := Branch(id)
	if branchExists(root, branch) {
		if _, err := git(root, "worktree", "add", dest, branch); err != nil {
			return Checkout{}, err
		}
	} else {
		if _, err := git(root, "worktree", "add", "-b", branch, dest); err != nil {
			return Checkout{}, err
		}
	}
	return Checkout{ID: id, Path: dest, Branch: branch, Repo: root}, nil
}

func Remove(id string) error {
	if !session.ValidID(id) {
		return fmt.Errorf("invalid session id")
	}
	wt, err := Find(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return err
	}
	if _, err := git(wt.Repo, "worktree", "remove", "--force", wt.Path); err != nil {
		_ = os.RemoveAll(wt.Path)
		if _, pruneErr := git(wt.Repo, "worktree", "prune"); pruneErr != nil && err != nil {
			return err
		}
	}
	_, _ = git(wt.Repo, "branch", "-D", wt.Branch)
	return nil
}

func Find(id string) (Checkout, error) {
	if !session.ValidID(id) {
		return Checkout{}, fmt.Errorf("invalid session id")
	}
	base, err := worktreeBase()
	if err != nil {
		return Checkout{}, err
	}
	matches, _ := filepath.Glob(filepath.Join(base, "*", id))
	if len(matches) == 0 {
		return Checkout{}, fmt.Errorf("worktree %s not found", id)
	}
	path := matches[0]
	repo, err := toplevel(path)
	if err != nil {
		repo, _ = gitCommonRoot(path)
	}
	return Checkout{ID: id, Path: path, Branch: Branch(id), Repo: repo}, nil
}

func List() ([]Checkout, error) {
	base, err := worktreeBase()
	if err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Checkout
	for _, repoEnt := range ents {
		if !repoEnt.IsDir() {
			continue
		}
		kids, err := os.ReadDir(filepath.Join(base, repoEnt.Name()))
		if err != nil {
			continue
		}
		for _, kid := range kids {
			if !kid.IsDir() || !session.ValidID(kid.Name()) {
				continue
			}
			wt, err := Find(kid.Name())
			if err != nil {
				continue
			}
			out = append(out, wt)
		}
	}
	return out, nil
}

func worktreeBase() (string, error) {
	root, err := config.UserDataDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(root, "worktrees")
	if err := config.SecretLinkError(base); err != nil {
		return "", err
	}
	return base, nil
}

func destPath(repo, id string) (string, error) {
	base, err := config.EnsureDataSubdir("worktrees")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, repoKey(repo), id), nil
}

func repoKey(repo string) string {
	sum := sha256.Sum256([]byte(repo))
	return hex.EncodeToString(sum[:6])
}

func toplevel(dir string) (string, error) {
	out, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %s", dir)
	}
	return out, nil
}

func gitCommonRoot(dir string) (string, error) {
	out, err := git(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(dir, out)
	}
	return filepath.Clean(filepath.Join(out, "..")), nil
}

func isWorktree(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return false
	}
	_, err := git(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

func branchExists(repo, branch string) bool {
	_, err := git(repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

func gitEnv() []string {
	return append(config.ScrubEnv(os.Environ()), "GIT_TERMINAL_PROMPT=0")
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}
	}()
	out, err := cmd.CombinedOutput()
	close(done)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(string(out)), nil
}
