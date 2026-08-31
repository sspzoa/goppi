//go:build darwin

package tools

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func wrapSandbox(cmd *exec.Cmd, workdir, mode string, extra ...string) error {
	if !sandboxOn(mode) {
		return nil
	}
	exe, err := exec.LookPath("sandbox-exec")
	if err != nil {
		return fmt.Errorf("sandbox workspace needs sandbox-exec: %w (or GOPPI_SANDBOX=off)", err)
	}
	cmd.Args = append([]string{"sandbox-exec", "-p", darwinProfile(workdir, mode, extra...)}, cmd.Args...)
	cmd.Path = exe
	return nil
}

func darwinProfile(workdir, mode string, extra ...string) string {
	var b strings.Builder
	b.WriteString("(version 1)\n(allow default)\n(deny file-write*)\n(allow file-write-data\n")
	b.WriteString("  (literal \"/dev/null\")\n  (literal \"/dev/zero\")\n  (literal \"/dev/dtracehelper\")\n  (literal \"/dev/tty\"))\n")
	b.WriteString("(allow file-write*\n")
	for _, root := range sandboxWriteRoots(workdir, extra...) {
		fmt.Fprintf(&b, "  (subpath %s)\n", quoteSeatbelt(root))
	}
	b.WriteString(")\n")
	for _, p := range sandboxGitDeny(workdir, extra...) {
		switch filepath.Base(p) {
		case "config", "HEAD", "packed-refs":
			fmt.Fprintf(&b, "(deny file-write*\n  (literal %s))\n", quoteSeatbelt(p))
			continue
		}
		fmt.Fprintf(&b, "(deny file-write*\n  (subpath %s))\n", quoteSeatbelt(p))
	}
	for _, p := range sandboxPrivDeny() {
		fmt.Fprintf(&b, "(deny process-exec\n  (literal %s))\n", quoteSeatbelt(p))
	}
	if sandboxNetOff(mode) {
		b.WriteString("(deny network*)\n")
	}
	return b.String()
}

func quoteSeatbelt(p string) string {
	p = strings.ReplaceAll(p, `\`, `\\`)
	p = strings.ReplaceAll(p, `"`, `\"`)
	return `"` + p + `"`
}
