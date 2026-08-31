package mcp

import "sync"

const maxErrCap = 8 << 10

type errCap struct {
	mu sync.Mutex
	b  []byte
}

func (e *errCap) Write(p []byte) (int, error) {
	if e == nil {
		return len(p), nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.b = append(e.b, p...)
	if len(e.b) > maxErrCap {
		e.b = append([]byte(nil), e.b[len(e.b)-maxErrCap:]...)
	}
	return len(p), nil
}

func (e *errCap) String() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return string(e.b)
}
