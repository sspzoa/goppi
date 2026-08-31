package tools

import (
	"fmt"
	"os"
	"sync"
)

type snapshot struct {
	path    string
	data    []byte
	existed bool
}

type snapStack struct {
	mu    sync.Mutex
	items []snapshot
}

func (s *snapStack) remember(path string) {
	if s == nil {
		return
	}
	data, err := os.ReadFile(path)
	snap := snapshot{path: path, existed: err == nil}
	if err == nil {
		snap.data = append([]byte(nil), data...)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, snap)
	if len(s.items) > 40 {
		s.items = s.items[len(s.items)-40:]
	}
}

func (s *snapStack) undo() (string, error) {
	if s == nil {
		return "", fmt.Errorf("nothing to undo")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) == 0 {
		return "", fmt.Errorf("nothing to undo")
	}
	snap := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	if !snap.existed {
		if err := os.Remove(snap.path); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		return "removed " + snap.path, nil
	}
	if err := writeAtomic(snap.path, snap.data, 0o644); err != nil {
		return "", err
	}
	return "restored " + snap.path, nil
}

func (r *Registry) UndoLast() (string, error) {
	if r == nil || r.snaps == nil {
		return "", fmt.Errorf("nothing to undo")
	}
	return r.snaps.undo()
}
