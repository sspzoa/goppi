package session

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sspzoa/goppi/internal/config"
)

type Lock struct {
	ID string
	f  *os.File
}

func lockPath(id string) (string, error) {
	if !ValidID(id) {
		return "", fmt.Errorf("invalid session id")
	}
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, id+".lock"), nil
}

func Hold(id string) (*Lock, error) {
	path, err := lockPath(id)
	if err != nil {
		return nil, err
	}
	if err := config.SecretLinkError(path); err != nil {
		return nil, err
	}
	f, err := openLock(path)
	if err != nil {
		return nil, err
	}
	if err := flockExclusive(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("session %s is already in use", id)
	}
	return &Lock{ID: id, f: f}, nil
}

func (l *Lock) Release() {
	if l == nil || l.f == nil {
		return
	}
	flockUnlock(l.f)
	_ = l.f.Close()
	l.f = nil
}
