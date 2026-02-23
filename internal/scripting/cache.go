package scripting

import (
	"sync"
	"time"
)

// compiledCache caches compiled Starlark programs, keyed by scriptID.
// An entry is invalidated when the script's UpdatedAt changes.
type compiledCache struct {
	mu    sync.RWMutex
	store map[string]cacheEntry
}

type cacheEntry struct {
	updatedAt time.Time
	compiled  CompiledScript
}

func newCompiledCache() *compiledCache {
	return &compiledCache{
		store: make(map[string]cacheEntry),
	}
}

// Get returns a cached compiled script if the updatedAt timestamp still matches.
func (c *compiledCache) Get(id string, updatedAt time.Time) (CompiledScript, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.store[id]
	if !ok {
		return nil, false
	}
	if !entry.updatedAt.Equal(updatedAt) {
		return nil, false
	}
	return entry.compiled, true
}

// Set stores a compiled script with its associated timestamp.
func (c *compiledCache) Set(id string, updatedAt time.Time, cs CompiledScript) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[id] = cacheEntry{
		updatedAt: updatedAt,
		compiled:  cs,
	}
}

// Delete removes a cached entry.
func (c *compiledCache) Delete(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, id)
}
