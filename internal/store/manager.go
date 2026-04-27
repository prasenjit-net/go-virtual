package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/models"
)

// SessionManager creates, retrieves, and expires sessions.
// All methods are safe for concurrent use.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	global   GlobalStoreBackend
	cfg      config.SessionConfig
}

var _ SessionRegistry = (*SessionManager)(nil)

// NewSessionManager creates a SessionManager and starts the background expiry loop.
// The loop runs until the provided context is cancelled.
func NewSessionManager(ctx context.Context, global GlobalStoreBackend, cfg config.SessionConfig) *SessionManager {
	m := &SessionManager{
		sessions: make(map[string]*Session),
		global:   global,
		cfg:      cfg,
	}

	go m.expiryLoop(ctx)

	return m
}

// GetOrCreate resolves or creates a session:
//   - If rawID is non-empty and exists in the registry → resume it, touch lastActive.
//   - If rawID is non-empty but unknown → create a new session using rawID as the ID.
//   - If rawID is empty → generate a new UUID v4 session ID.
//
// In all cases the session is seeded from the current global store snapshot.
func (m *SessionManager) GetOrCreate(rawID string) (SessionState, bool, error) {
	// Fast path: try to find an existing valid session
	if rawID != "" {
		m.mu.RLock()
		sess, ok := m.sessions[rawID]
		m.mu.RUnlock()

		if ok {
			sess.touch()
			return sess, false, nil // false = existing session
		}
	}

	// Slow path: create a new session
	m.mu.Lock()
	defer m.mu.Unlock()

	// Re-check under write lock in case another goroutine just created it
	if rawID != "" {
		if sess, ok := m.sessions[rawID]; ok {
			sess.touch()
			return sess, false, nil
		}
	}

	// Enforce max sessions cap: evict the least-recently-active session
	if m.cfg.MaxSessions > 0 && len(m.sessions) >= m.cfg.MaxSessions {
		m.evictOldestLocked()
	}

	// Use the caller-supplied ID if provided, otherwise generate a fresh UUID.
	id := rawID
	if id == "" {
		id = uuid.New().String()
	}
	var snapshot map[string]any
	if m.global != nil {
		snapshot = m.global.Snapshot()
	}
	sess := newSession(id, snapshot)
	m.sessions[id] = sess

	return sess, true, nil // true = new session
}

// Get returns a session by ID without creating one. ok is false if not found.
func (m *SessionManager) Get(sessionID string) (SessionState, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[sessionID]
	return sess, ok, nil
}

// Invalidate removes a session from the registry.
func (m *SessionManager) Invalidate(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	return nil
}

// InvalidateAll removes all sessions from the registry.
func (m *SessionManager) InvalidateAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions = make(map[string]*Session)
	return nil
}

// ActiveSessions returns metadata about all live sessions, sorted by last-active descending.
func (m *SessionManager) ActiveSessions() ([]models.SessionInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]models.SessionInfo, 0, len(m.sessions))
	for _, sess := range m.sessions {
		infos = append(infos, sess.Info(false))
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].LastActive.After(infos[j].LastActive)
	})

	return infos, nil
}

// Count returns the number of active sessions.
func (m *SessionManager) Count() (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.sessions), nil
}

// expiryLoop runs every minute and removes sessions that have been idle
// longer than the configured inactivity timeout.
func (m *SessionManager) expiryLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.evictExpired()
		}
	}
}

func (m *SessionManager) evictExpired() {
	if m.cfg.InactivityTimeout <= 0 {
		return
	}

	cutoff := time.Now().Add(-m.cfg.InactivityTimeout)

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, sess := range m.sessions {
		if sess.LastActive.Before(cutoff) {
			delete(m.sessions, id)
		}
	}
}

// evictOldestLocked removes the session with the oldest LastActive timestamp.
// Caller must hold m.mu write lock.
func (m *SessionManager) evictOldestLocked() {
	var oldestID string
	var oldestTime time.Time

	first := true
	for id, sess := range m.sessions {
		if first || sess.LastActive.Before(oldestTime) {
			oldestID = id
			oldestTime = sess.LastActive
			first = false
		}
	}

	if oldestID != "" {
		delete(m.sessions, oldestID)
	}
}
