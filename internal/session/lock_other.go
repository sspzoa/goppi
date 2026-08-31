//go:build !unix && !windows

package session

import "os"

func openLock(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
}

func flockExclusive(*os.File) error { return nil }

func flockUnlock(*os.File) {}
