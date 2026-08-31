//go:build windows

package session

import (
	"os"

	"golang.org/x/sys/windows"
)

func openLock(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
}

func flockExclusive(f *os.File) error {
	var ol windows.Overlapped
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&ol,
	)
}

func flockUnlock(f *os.File) {
	var ol windows.Overlapped
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
