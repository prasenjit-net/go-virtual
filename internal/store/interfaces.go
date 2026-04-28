package store

import "github.com/prasenjit/go-virtual/internal/models"

// GlobalStoreBackend is the interface satisfied by any persistent key-value
// store implementation (file-backed, MongoDB, etc.) used as the global store.
type GlobalStoreBackend interface {
	Get(key string) (any, bool)
	Set(key string, value any) error
	Delete(key string) error
	Clear() error
	Len() int
	List() []models.StoreEntry
	Snapshot() map[string]any
}

// SessionState is the runtime view of one request/session-scoped store.
type SessionState interface {
	Get(key string) (any, bool)
	Set(key string, value any) error
	Has(key string) bool
	Delete(key string) error
	Keys() []string
	Snapshot() map[string]any
	Info(includeSnapshot bool) models.SessionInfo
}

// SessionRegistry manages live sessions for one backend.
type SessionRegistry interface {
	GetOrCreate(rawID string) (SessionState, bool, error)
	Get(sessionID string) (SessionState, bool, error)
	Invalidate(sessionID string) error
	InvalidateAll() error
	ActiveSessions() ([]models.SessionInfo, error)
	Count() (int, error)
}
