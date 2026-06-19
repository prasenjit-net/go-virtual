package store

import (
	"sync"

	"github.com/google/uuid"
)

// MemoryCollectionBackend holds named document collections entirely in memory.
// Used when storage type is "memory". Data does not survive process restarts.
type MemoryCollectionBackend struct {
	mu          sync.RWMutex
	collections map[string][]map[string]any
}

// NewMemoryCollectionBackend returns an empty MemoryCollectionBackend.
func NewMemoryCollectionBackend() *MemoryCollectionBackend {
	return &MemoryCollectionBackend{
		collections: make(map[string][]map[string]any),
	}
}

// GetAll returns a copy of all documents in the named collection.
func (m *MemoryCollectionBackend) GetAll(collection string) ([]map[string]any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	docs := m.collections[collection]
	if docs == nil {
		return nil, nil
	}
	out := make([]map[string]any, len(docs))
	for i, d := range docs {
		out[i] = copyDoc(d)
	}
	return out, nil
}

// SeedInsert appends one document to the collection.
func (m *MemoryCollectionBackend) SeedInsert(collection string, doc map[string]any) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := copyDoc(doc)
	if _, ok := d["_id"]; !ok {
		d["_id"] = uuid.New().String()
	}
	m.collections[collection] = append(m.collections[collection], d)
	return copyDoc(d), nil
}

// SeedClear empties the named collection.
func (m *MemoryCollectionBackend) SeedClear(collection string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collections[collection] = nil
	return nil
}

// ListCollections returns the names of all known collections (including empty ones).
func (m *MemoryCollectionBackend) ListCollections() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.collections))
	for name := range m.collections {
		names = append(names, name)
	}
	return names, nil
}

// DropCollection removes the named collection entirely.
func (m *MemoryCollectionBackend) DropCollection(collection string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.collections, collection)
	return nil
}
