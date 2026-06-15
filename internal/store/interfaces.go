package store

import "github.com/prasenjit/go-virtual/internal/models"

// CollectionEventKeyPrefix is the session-store key prefix under which a
// session's per-collection mutation event log is kept.
const CollectionEventKeyPrefix = "__cevt__"

// CollectionEvent records one mutation a session performed on a named
// collection. Events are replayed on top of the global base to produce the
// session's isolated view.
type CollectionEvent struct {
	Op     string         `json:"op"`               // "insert"|"update"|"upsert"|"delete"|"clear"
	Filter map[string]any `json:"filter,omitempty"` // update / upsert / delete
	Data   map[string]any `json:"data,omitempty"`   // insert / update / upsert
}

// CollectionBackend manages the persistent, globally-shared base state of
// named document collections. Sessions overlay their own CollectionEvents on
// top of this base to produce an isolated view without modifying the base.
type CollectionBackend interface {
	// GetAll returns all documents currently in the named collection's base.
	GetAll(collection string) ([]map[string]any, error)
	// SeedInsert appends one document to the collection's base. A UUID _id is
	// auto-assigned if the document does not already contain one.
	SeedInsert(collection string, doc map[string]any) (map[string]any, error)
	// SeedClear removes all documents from the named collection's base.
	SeedClear(collection string) error
	// ListCollections returns the names of all collections that have been
	// created (i.e. have at least one seed document or an empty base file).
	ListCollections() ([]string, error)
	// DropCollection permanently removes the named collection and all its base
	// documents from storage.
	DropCollection(collection string) error
}

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
