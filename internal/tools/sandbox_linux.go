//go:build linux

package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/sspzoa/goppi/internal/config"
	"golang.org/x/sys/unix"
)

func init() {
	if os.Getenv("GOPPI_SANDBOX_HELPER") != "1" {
		return
	}
	os.Exit(runSandboxHelper())
}

func wrapSandbox(cmd *exec.Cmd, workdir, mode string, extra ...string) error {
	if !sandboxOn(mode) {
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("sandbox: %w", err)
	}
	wd, err := absExisting(workdir)
	if err != nil {
		return fmt.Errorf("sandbox workdir: %w", err)
	}
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(config.ScrubEnv(cmd.Env), "GOPPI_SANDBOX_HELPER=1", "GOPPI_SANDBOX_WORKDIR="+wd, "GOPPI_SANDBOX_MODE="+mode)
	if paths := existingAbsList(extra); len(paths) > 0 {
		cmd.Env = append(cmd.Env, "GOPPI_SANDBOX_EXTRA="+strings.Join(paths, string(os.PathListSeparator)))
	}
	cmd.Args = append([]string{self}, cmd.Args...)
	cmd.Path = self
	return nil
}

func runSandboxHelper() int {
	if err := restrictLinux(os.Getenv("GOPPI_SANDBOX_WORKDIR"), os.Getenv("GOPPI_SANDBOX_MODE"), splitPathList(os.Getenv("GOPPI_SANDBOX_EXTRA"))...); err != nil {
		fmt.Fprintf(os.Stderr, "goppi sandbox: %v\n", err)
		return 1
	}
	args := os.Args[1:]
	if len(args) == 0 {
		return 2
	}
	path, err := exec.LookPath(args[0])
	if err != nil {
		path = args[0]
	}
	err = unix.Exec(path, args, config.ScrubEnv(stripHelperEnv(os.Environ())))
	fmt.Fprintf(os.Stderr, "goppi sandbox exec: %v\n", err)
	return 1
}

func stripHelperEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		if key == "GOPPI_SANDBOX_HELPER" || key == "GOPPI_SANDBOX_WORKDIR" || key == "GOPPI_SANDBOX_MODE" {
			continue
		}
		out = append(out, e)
	}
	return out
}

func existingAbsList(paths []string) []string {
	var out []string
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			abs = real
		}
		if _, err := os.Stat(abs); err != nil {
			continue
		}
		out = append(out, abs)
	}
	return out
}

func splitPathList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return filepath.SplitList(s)
}

func absExisting(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	return abs, nil
}

func restrictLinux(workdir, mode string, extra ...string) error {
	if sandboxNetOff(mode) {
		if err := denyNetworkLinux(); err != nil {
			return err
		}
	}
	abi, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 {
		return fmt.Errorf("landlock unavailable: %w (or GOPPI_SANDBOX=off)", errno)
	}
	access := landlockWriteAccess(int(abi))
	attr := unix.LandlockRulesetAttr{Access_fs: access}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr.Access_fs), 0)
	if errno != 0 {
		return fmt.Errorf("landlock ruleset: %w", errno)
	}
	ruleset := int(fd)
	defer unix.Close(ruleset)

	for _, root := range append(sandboxWriteRoots(workdir, extra...), "/dev/null") {
		if err := addLandlockPath(ruleset, root, access); err != nil {
			return err
		}
	}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("no_new_privs: %w", err)
	}
	_, _, errno = unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, uintptr(ruleset), 0, 0)
	if errno != 0 {
		return fmt.Errorf("landlock restrict: %w", errno)
	}
	return nil
}

func denyNetworkLinux() error {
	if err := unix.Unshare(unix.CLONE_NEWUSER | unix.CLONE_NEWNET); err != nil {
		if err2 := unix.Unshare(unix.CLONE_NEWNET); err2 == nil {
			return nil
		}
		return fmt.Errorf("network namespace: %w (or GOPPI_SANDBOX=workspace)", err)
	}
	uid, gid := os.Getuid(), os.Getgid()
	_ = os.WriteFile("/proc/self/setgroups", []byte("deny\n"), 0644)
	if err := os.WriteFile("/proc/self/uid_map", []byte(fmt.Sprintf("%d %d 1\n", uid, uid)), 0644); err != nil {
		return fmt.Errorf("uid_map: %w (or GOPPI_SANDBOX=workspace)", err)
	}
	if err := os.WriteFile("/proc/self/gid_map", []byte(fmt.Sprintf("%d %d 1\n", gid, gid)), 0644); err != nil {
		return fmt.Errorf("gid_map: %w (or GOPPI_SANDBOX=workspace)", err)
	}
	return nil
}

func landlockWriteAccess(abi int) uint64 {
	access := uint64(unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
		unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
		unix.LANDLOCK_ACCESS_FS_REMOVE_DIR |
		unix.LANDLOCK_ACCESS_FS_MAKE_CHAR |
		unix.LANDLOCK_ACCESS_FS_MAKE_DIR |
		unix.LANDLOCK_ACCESS_FS_MAKE_REG |
		unix.LANDLOCK_ACCESS_FS_MAKE_SOCK |
		unix.LANDLOCK_ACCESS_FS_MAKE_FIFO |
		unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
		unix.LANDLOCK_ACCESS_FS_MAKE_SYM)
	if abi >= 2 {
		access |= unix.LANDLOCK_ACCESS_FS_REFER
	}
	if abi >= 3 {
		access |= unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}
	if abi >= 4 {
		access |= unix.LANDLOCK_ACCESS_FS_IOCTL_DEV
	}
	return access
}

func addLandlockPath(ruleset int, path string, access uint64) error {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	allowed := access
	if !info.IsDir() {
		allowed = unix.LANDLOCK_ACCESS_FS_WRITE_FILE
		if access&unix.LANDLOCK_ACCESS_FS_TRUNCATE != 0 {
			allowed |= unix.LANDLOCK_ACCESS_FS_TRUNCATE
		}
		if access&unix.LANDLOCK_ACCESS_FS_IOCTL_DEV != 0 {
			allowed |= unix.LANDLOCK_ACCESS_FS_IOCTL_DEV
		}
	}
	fd, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil
	}
	defer unix.Close(fd)
	rule := unix.LandlockPathBeneathAttr{
		Allowed_access: allowed,
		Parent_fd:      int32(fd),
	}
	_, _, errno := unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, uintptr(ruleset), unix.LANDLOCK_RULE_PATH_BENEATH, uintptr(unsafe.Pointer(&rule)), 0, 0, 0)
	if errno != 0 {
		return fmt.Errorf("landlock rule %s: %w", path, errno)
	}
	return nil
}
