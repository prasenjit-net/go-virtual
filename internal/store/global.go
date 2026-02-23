package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
)

// globalStoreFile is the JSON format written to disk.
type globalStoreFile struct {
	UpdatedAt time.Time          `json:"updatedAt"`
	Entries   map[string]any     `json:"entries"`
}

// GlobalStore holds the application-wide persistent key-value store.
// It is the single source of truth for store data — sessions receive a
// deep-copy snapshot at creation time.
type GlobalStore struct {
	mu       sync.RWMutex
	entries  map[string]models.StoreEntry
	filePath string
}

// NewGlobalStore loads (or creates) the store from the given file path.
func NewGlobalStore(filePath string) (*GlobalStore, error) {
	gs := &GlobalStore{
		entries:  make(map[string]models.StoreEntry),
		filePath: filePath,
	}

	if err := gs.load(); err != nil {
		return nil, err
	}

	return gs, nil
}

// load reads the store file. If the file does not exist an empty store is used.
func (g *GlobalStore) load() error {
	data, err := os.ReadFile(g.filePath)
	if os.IsNotExist(err) {
		// First run — persist an empty store immediately
		return g.save()
	}
	if err != nil {
		return fmt.Errorf("store: read %s: %w", g.filePath, err)
	}

	var f globalStoreFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("store: parse %s: %w", g.filePath, err)
	}

	now := time.Now()
	for k, v := range f.Entries {
		g.entries[k] = models.StoreEntry{
			Key:       k,
			Value:     v,
			CreatedAt: now, // creation time not stored per-entry on disk (simplification)
			UpdatedAt: now,
		}
	}

	return nil
}

// save atomically rewrites the store file.
func (g *GlobalStore) save() error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(g.filePath), 0o755); err != nil {
		return fmt.Errorf("store: mkdir: %w", err)
	}

	flat := make(map[string]any, len(g.entries))
	for k, e := range g.entries {
		flat[k] = e.Value
	}

	f := globalStoreFile{
		UpdatedAt: time.Now(),
		Entries:   flat,
	}

	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}

	// Atomic write: write to .tmp then rename
	tmpPath := g.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("store: write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, g.filePath); err != nil {
		return fmt.Errorf("store: rename: %w", err)
	}

	return nil
}

// Get returns the value for a key. ok is false when the key is absent.
func (g *GlobalStore) Get(key string) (any, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	e, ok := g.entries[key]
	if !ok {
		return nil, false
	}
	return e.Value, true
}

// Set upserts a key-value pair and persists the store to disk.
func (g *GlobalStore) Set(key string, value any) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	existing, exists := g.entries[key]
	createdAt := now
	if exists {
		createdAt = existing.CreatedAt
	}

	g.entries[key] = models.StoreEntry{
		Key:       key,
		Value:     value,
		CreatedAt: createdAt,
		UpdatedAt: now,
	}

	return g.save()
}

// Delete removes a key and persists the store to disk.
func (g *GlobalStore) Delete(key string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.entries, key)

	return g.save()
}

// Clear removes all keys and persists the store to disk.
func (g *GlobalStore) Clear() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.entries = make(map[string]models.StoreEntry)

	return g.save()
}

// List returns all entries sorted by key.
func (g *GlobalStore) List() []models.StoreEntry {
	g.mu.RLock()
	defer g.mu.RUnlock()

	entries := make([]models.StoreEntry, 0, len(g.entries))
	for _, e := range g.entries {
		entries = append(entries, e)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	return entries
}

// Snapshot returns a deep copy of all current values, keyed by entry key.
// Used to seed a new session store.
func (g *GlobalStore) Snapshot() map[string]any {
	g.mu.RLock()
	defer g.mu.RUnlock()

	snapshot := make(map[string]any, len(g.entries))
	for k, e := range g.entries {
		snapshot[k] = deepCopy(e.Value)
	}

	return snapshot
}

// deepCopy creates a simple deep copy of JSON-compatible values by
// marshalling and unmarshalling. For performance-critical paths a more
// efficient approach could be used, but this is correct and simple.
func deepCopy(v any) any {
	if v == nil {
		return nil
	}

	data, err := json.Marshal(v)
	if err != nil {
		return v
	}

	var copy any
	if err := json.Unmarshal(data, &copy); err != nil {
		return v
	}

	return copy
}
