package store

import (
	"sort"
	"sync"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
)

// Session holds one active session and its private store copy.
// All methods are safe for concurrent use.
type Session struct {
	ID         string
	CreatedAt  time.Time
	LastActive time.Time

	mu    sync.Mutex
	store map[string]any // private copy seeded from GlobalStore.Snapshot()
}

func newSession(id string, snapshot map[string]any) *Session {
	return &Session{
		ID:         id,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		store:      snapshot,
	}
}

// Get returns the value for a key. ok is false when the key is absent.
func (s *Session) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.store[key]
	return v, ok
}

// Set stores a value under the given key (session-local, never propagates to global).
func (s *Session) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store[key] = value
}

// Has returns true if the key exists in the session store.
func (s *Session) Has(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.store[key]
	return ok
}

// Delete removes a key from the session store.
func (s *Session) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.store, key)
}

// Keys returns all keys in the session store, sorted.
func (s *Session) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys := make([]string, 0, len(s.store))
	for k := range s.store {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Snapshot returns a read-only shallow copy of the session store for tracing/inspector.
func (s *Session) Snapshot() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := make(map[string]any, len(s.store))
	for k, v := range s.store {
		snap[k] = v
	}
	return snap
}

// touch updates the last-active timestamp. Called by SessionManager on every access.
func (s *Session) touch() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.LastActive = time.Now()
}

// Info returns a read-only summary of this session.
func (s *Session) Info(includeSnapshot bool) models.SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	info := models.SessionInfo{
		ID:         s.ID,
		CreatedAt:  s.CreatedAt,
		LastActive: s.LastActive,
		EntryCount: len(s.store),
	}

	if includeSnapshot {
		snap := make(map[string]any, len(s.store))
		for k, v := range s.store {
			snap[k] = v
		}
		info.StoreSnapshot = snap
	}

	return info
}
