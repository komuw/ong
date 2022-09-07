package middleware

import "sync"

// Store persists any information(like csrf tokens) that is needed by Ong in its normal running.
type Store interface {
	Set(s string)
	Exists(s string) bool
	Reset()
}

// comptime check.
var _ Store = NewMemStore() //nolint:gochecknoglobals

// memStore persists server-side, in-memory. Use NewMemStore to instantiate.
type memStore struct {
	mu sync.RWMutex // protects m
	m  map[string]struct{}
}

// NewMemStore returns a [store] that persists server-side, in-memory.
// It should not be used if your application is distributed across more than one instance/server.
func NewMemStore() *memStore {
	return &memStore{
		m: map[string]struct{}{},
	}
}

func (s *memStore) Exists(actualToken string) bool {
	if len(actualToken) < 1 {
		return false
	}
	s.mu.RLock()
	_, ok := s.m[actualToken]
	s.mu.RUnlock()
	return ok
}

func (s *memStore) Set(actualToken string) {
	s.mu.Lock()
	s.m[actualToken] = struct{}{}
	s.mu.Unlock()
}

func (s *memStore) Reset() {
	s.mu.Lock()
	s.m = map[string]struct{}{}
	s.mu.Unlock()
}

// used in tests
func (s *memStore) _len() int {
	s.mu.RLock()
	l := len(s.m)
	s.mu.RUnlock()
	return l
}
