package lsp

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sspzoa/goppi/internal/config"
)

func autoGopls(workdir string) (config.LSPServer, bool) {
	if _, err := exec.LookPath("gopls"); err != nil {
		return config.LSPServer{}, false
	}
	if !looksLikeGo(workdir) {
		return config.LSPServer{}, false
	}
	return config.LSPServer{Command: "gopls", Language: "go"}, true
}

func looksLikeGo(workdir string) bool {
	if _, err := os.Stat(filepath.Join(workdir, "go.mod")); err == nil {
		return true
	}
	matches, err := filepath.Glob(filepath.Join(workdir, "*.go"))
	return err == nil && len(matches) > 0
}
