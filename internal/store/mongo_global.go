package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/models"
)

const mongoGlobalStoreCollection = "global_store"

// storeEntryDoc is the BSON document persisted for each global store entry.
type storeEntryDoc struct {
	Key       string    `bson:"_id"`
	Value     string    `bson:"value"` // JSON-encoded value
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
}

// MongoGlobalStore is a MongoDB-backed implementation of GlobalStoreBackend.
// It is safe for concurrent use; all writes immediately persist to MongoDB.
type MongoGlobalStore struct {
	mu         sync.RWMutex
	collection *mongo.Collection
	// in-memory cache to satisfy Snapshot() and Len() efficiently
	cache map[string]models.StoreEntry
}

// NewMongoGlobalStoreFromConfig creates a MongoGlobalStore from a MongoConfig.
func NewMongoGlobalStoreFromConfig(cfg config.MongoConfig) (*MongoGlobalStore, error) {
	if cfg.URI == "" {
		return nil, fmt.Errorf("mongo global store: URI must not be empty")
	}
	timeout := time.Duration(cfg.ConnectTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	opts := options.Client().ApplyURI(cfg.URI).SetConnectTimeout(timeout)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongo global store: connect: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo global store: ping: %w", err)
	}

	col := client.Database(cfg.Database).Collection(cfg.CollectionPrefix + mongoGlobalStoreCollection)
	return newMongoGlobalStore(ctx, col)
}

// NewMongoGlobalStore creates a MongoGlobalStore from an existing mongo.Collection.
func NewMongoGlobalStore(col *mongo.Collection) (*MongoGlobalStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return newMongoGlobalStore(ctx, col)
}

func newMongoGlobalStore(ctx context.Context, col *mongo.Collection) (*MongoGlobalStore, error) {
	gs := &MongoGlobalStore{
		collection: col,
		cache:      make(map[string]models.StoreEntry),
	}
	if err := gs.loadAll(ctx); err != nil {
		return nil, err
	}
	return gs, nil
}

// loadAll reads all entries from MongoDB into the in-memory cache.
func (g *MongoGlobalStore) loadAll(ctx context.Context) error {
	cursor, err := g.collection.Find(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("mongo global store: load: %w", err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc storeEntryDoc
		if err := cursor.Decode(&doc); err != nil {
			return fmt.Errorf("mongo global store: decode: %w", err)
		}
		var val any
		if err := json.Unmarshal([]byte(doc.Value), &val); err != nil {
			val = doc.Value // fall back to raw string
		}
		g.cache[doc.Key] = models.StoreEntry{
			Key:       doc.Key,
			Value:     val,
			CreatedAt: doc.CreatedAt,
			UpdatedAt: doc.UpdatedAt,
		}
	}
	return cursor.Err()
}

func (g *MongoGlobalStore) ctxTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

// Get returns the value for a key. ok is false when the key is absent.
func (g *MongoGlobalStore) Get(key string) (any, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	e, ok := g.cache[key]
	if !ok {
		return nil, false
	}
	return e.Value, true
}

// Set upserts a key-value pair and persists it to MongoDB.
func (g *MongoGlobalStore) Set(key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("mongo global store: marshal value: %w", err)
	}

	now := time.Now()

	g.mu.Lock()
	defer g.mu.Unlock()

	createdAt := now
	if existing, ok := g.cache[key]; ok {
		createdAt = existing.CreatedAt
	}

	doc := storeEntryDoc{
		Key:       key,
		Value:     string(data),
		CreatedAt: createdAt,
		UpdatedAt: now,
	}

	ctx, cancel := g.ctxTimeout()
	defer cancel()

	opts := options.Replace().SetUpsert(true)
	if _, err := g.collection.ReplaceOne(ctx, bson.M{"_id": key}, doc, opts); err != nil {
		return fmt.Errorf("mongo global store: upsert: %w", err)
	}

	g.cache[key] = models.StoreEntry{
		Key:       key,
		Value:     value,
		CreatedAt: createdAt,
		UpdatedAt: now,
	}
	return nil
}

// Delete removes a key and persists the change.
func (g *MongoGlobalStore) Delete(key string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	ctx, cancel := g.ctxTimeout()
	defer cancel()

	if _, err := g.collection.DeleteOne(ctx, bson.M{"_id": key}); err != nil {
		return fmt.Errorf("mongo global store: delete: %w", err)
	}
	delete(g.cache, key)
	return nil
}

// Clear removes all keys and persists the change.
func (g *MongoGlobalStore) Clear() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	ctx, cancel := g.ctxTimeout()
	defer cancel()

	if _, err := g.collection.DeleteMany(ctx, bson.M{}); err != nil {
		return fmt.Errorf("mongo global store: clear: %w", err)
	}
	g.cache = make(map[string]models.StoreEntry)
	return nil
}

// Len returns the number of entries currently in the store.
func (g *MongoGlobalStore) Len() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.cache)
}

// List returns all entries sorted by key.
func (g *MongoGlobalStore) List() []models.StoreEntry {
	g.mu.RLock()
	defer g.mu.RUnlock()

	entries := make([]models.StoreEntry, 0, len(g.cache))
	for _, e := range g.cache {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
	return entries
}

// Snapshot returns a deep copy of all current values, keyed by entry key.
func (g *MongoGlobalStore) Snapshot() map[string]any {
	g.mu.RLock()
	defer g.mu.RUnlock()

	snapshot := make(map[string]any, len(g.cache))
	for k, e := range g.cache {
		snapshot[k] = deepCopy(e.Value)
	}
	return snapshot
}
