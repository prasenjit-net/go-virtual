package store

import (
	"sync"

	"github.com/prasenjit/go-virtual/internal/models"
)

var _ SessionState = (*LazySession)(nil)

// LazySession is a SessionState that defers session creation until the first
// store operation is performed. This allows the proxy engine to pass a session
// placeholder to scripts without eagerly allocating a session for every request.
//
// Once materialised the inner *Session is permanent for the lifetime of the
// request; all subsequent operations delegate directly to it.
type LazySession struct {
	mu      sync.Mutex
	manager SessionRegistry
	inner   SessionState
}

// NewLazySession returns a LazySession backed by the given registry.
// No session is created until the first store operation is called.
func NewLazySession(manager SessionRegistry) *LazySession {
	return &LazySession{manager: manager}
}

// Materialized returns the underlying SessionState if one has been created, or nil
// if no store operation has been performed yet.
func (l *LazySession) Materialized() SessionState {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inner
}

func (l *LazySession) materialize() SessionState {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inner == nil {
		sess, _, _ := l.manager.GetOrCreate("")
		l.inner = sess
	}
	return l.inner
}

// ── SessionState implementation ──────────────────────────────────────────────

func (l *LazySession) Get(key string) (any, bool)     { return l.materialize().Get(key) }
func (l *LazySession) Set(key string, value any) error { return l.materialize().Set(key, value) }
func (l *LazySession) Has(key string) bool             { return l.materialize().Has(key) }
func (l *LazySession) Delete(key string) error         { return l.materialize().Delete(key) }
func (l *LazySession) Keys() []string                  { return l.materialize().Keys() }

func (l *LazySession) Snapshot() map[string]any {
	l.mu.Lock()
	inner := l.inner
	l.mu.Unlock()
	if inner == nil {
		return make(map[string]any)
	}
	return inner.Snapshot()
}

func (l *LazySession) Info(includeSnapshot bool) models.SessionInfo {
	l.mu.Lock()
	inner := l.inner
	l.mu.Unlock()
	if inner == nil {
		return models.SessionInfo{}
	}
	return inner.Info(includeSnapshot)
}
