//go:build !unit

package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/google/uuid"
	"github.com/prasenjit/go-virtual/internal/config"
)

const mongoCollectionPrefix = "col_"

// MongoCollectionBackend stores each named collection as its own MongoDB
// collection named <prefix>col_<name>. Documents are stored natively
// (no genericDoc wrapper). A UUID _id is assigned on insert if absent.
type MongoCollectionBackend struct {
	client   *mongo.Client
	database string
	prefix   string
	mu       sync.Map // map[string]*mongo.Collection — cached per name
}

// NewMongoCollectionBackend returns a MongoCollectionBackend using the provided
// connected client. prefix is the collection-name prefix (e.g. "gv_").
func NewMongoCollectionBackend(client *mongo.Client, database, prefix string) *MongoCollectionBackend {
	return &MongoCollectionBackend{
		client:   client,
		database: database,
		prefix:   prefix,
	}
}

// NewMongoCollectionBackendFromConfig connects to MongoDB using cfg and returns
// a MongoCollectionBackend.
func NewMongoCollectionBackendFromConfig(cfg config.MongoConfig) (*MongoCollectionBackend, error) {
	if cfg.URI == "" {
		return nil, fmt.Errorf("mongo collection backend: URI must not be empty")
	}
	timeout := time.Duration(cfg.ConnectTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	opts := options.Client().ApplyURI(cfg.URI).SetConnectTimeout(timeout)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongo collection backend: connect: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo collection backend: ping: %w", err)
	}

	return NewMongoCollectionBackend(client, cfg.Database, cfg.CollectionPrefix), nil
}

func (m *MongoCollectionBackend) col(name string) *mongo.Collection {
	fullName := m.prefix + mongoCollectionPrefix + name
	v, _ := m.mu.LoadOrStore(fullName, m.client.Database(m.database).Collection(fullName))
	return v.(*mongo.Collection)
}

func (m *MongoCollectionBackend) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

// GetAll returns all documents from the named collection.
func (m *MongoCollectionBackend) GetAll(collection string) ([]map[string]any, error) {
	ctx, cancel := m.ctx()
	defer cancel()

	cursor, err := m.col(collection).Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("mongo collection %s: find: %w", collection, err)
	}
	defer cursor.Close(ctx)

	var docs []map[string]any
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, fmt.Errorf("mongo collection %s: decode: %w", collection, err)
		}
		docs = append(docs, bsonMToMap(raw))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return docs, nil
}

// SeedInsert inserts one document into the named collection's base.
func (m *MongoCollectionBackend) SeedInsert(collection string, doc map[string]any) (map[string]any, error) {
	ctx, cancel := m.ctx()
	defer cancel()

	d := copyDoc(doc)
	if _, ok := d["_id"]; !ok {
		d["_id"] = uuid.New().String()
	}

	bdoc := mapToBsonM(d)
	if _, err := m.col(collection).InsertOne(ctx, bdoc); err != nil {
		return nil, fmt.Errorf("mongo collection %s: insert: %w", collection, err)
	}
	return copyDoc(d), nil
}

// SeedClear deletes all documents from the named collection.
func (m *MongoCollectionBackend) SeedClear(collection string) error {
	ctx, cancel := m.ctx()
	defer cancel()

	if _, err := m.col(collection).DeleteMany(ctx, bson.M{}); err != nil {
		return fmt.Errorf("mongo collection %s: clear: %w", collection, err)
	}
	return nil
}

// ListCollections returns the names of MongoDB collections whose names match
// the <prefix>col_<name> pattern in this database.
func (m *MongoCollectionBackend) ListCollections() ([]string, error) {
	ctx, cancel := m.ctx()
	defer cancel()

	fullPrefix := m.prefix + mongoCollectionPrefix
	filter := bson.M{"name": bson.M{"$regex": "^" + fullPrefix}}
	specs, err := m.client.Database(m.database).ListCollectionSpecifications(ctx, filter, options.ListCollections().SetAuthorizedCollections(true))
	if err != nil {
		return nil, fmt.Errorf("mongo list collections: %w", err)
	}

	names := make([]string, 0, len(specs))
	for _, s := range specs {
		if len(s.Name) > len(fullPrefix) {
			names = append(names, s.Name[len(fullPrefix):])
		}
	}
	return names, nil
}

// DropCollection drops the underlying MongoDB collection entirely.
func (m *MongoCollectionBackend) DropCollection(collection string) error {
	ctx, cancel := m.ctx()
	defer cancel()

	if err := m.col(collection).Drop(ctx); err != nil {
		return fmt.Errorf("mongo collection %s: drop: %w", collection, err)
	}
	// evict from cache
	m.mu.Delete(m.prefix + mongoCollectionPrefix + collection)
	return nil
}

// bsonMToMap converts a bson.M to map[string]any, remapping "_id" from bson types.
func bsonMToMap(raw bson.M) map[string]any {
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		switch tv := v.(type) {
		case bson.M:
			out[k] = bsonMToMap(tv)
		default:
			out[k] = v
		}
	}
	return out
}

// mapToBsonM converts map[string]any to bson.M.
func mapToBsonM(m map[string]any) bson.M {
	out := make(bson.M, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
