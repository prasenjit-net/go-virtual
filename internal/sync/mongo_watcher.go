//go:build !unit

package sync

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/prasenjit/go-virtual/internal/logging"
)

// watchedCollection pairs a logical collection name (without prefix) with its
// full prefixed name in MongoDB.
type watchedCollection struct {
	logical string // e.g. "specs"
	full    string // e.g. "gv_specs"
}

// MongoChangeWatcher implements ChangeWatcher using MongoDB change streams.
// It watches the specs, operations, responses, and global_store collections
// so that any mutation made by any instance is immediately delivered to the
// registered Handler on every other instance.
//
// Requires the target MongoDB deployment to be a replica set or sharded
// cluster; returns ErrChangeStreamsNotSupported on standalone deployments.
type MongoChangeWatcher struct {
	db          *mongo.Database
	collections []watchedCollection
}

// NewMongoChangeWatcher creates a MongoChangeWatcher that watches the given
// collections (logical names without prefix). The prefix is prepended when
// opening the change stream.
func NewMongoChangeWatcher(db *mongo.Database, prefix string, logicalCollections []string) *MongoChangeWatcher {
	watched := make([]watchedCollection, len(logicalCollections))
	for i, name := range logicalCollections {
		watched[i] = watchedCollection{logical: name, full: prefix + name}
	}
	return &MongoChangeWatcher{db: db, collections: watched}
}

// Watch opens a change stream on each watched collection and fans events out
// to handler. It blocks until ctx is cancelled. Transient errors cause an
// exponential back-off retry. A permanent "not a replica set" error results
// in ErrChangeStreamsNotSupported being returned.
func (w *MongoChangeWatcher) Watch(ctx context.Context, handler Handler) error {
	logger := logging.Logger("sync.mongo_watcher")

	errCh := make(chan error, len(w.collections))
	for _, wc := range w.collections {
		go func(wc watchedCollection) {
			errCh <- w.watchCollection(ctx, wc, handler)
		}(wc)
	}

	var firstFatal error
	for range w.collections {
		if err := <-errCh; err != nil && firstFatal == nil {
			firstFatal = err
			logger.Error("Change stream watcher returned error",
				"event", "change_stream_error",
				"error", err,
			)
		}
	}
	return firstFatal
}

// watchCollection watches a single collection, retrying on transient errors.
func (w *MongoChangeWatcher) watchCollection(ctx context.Context, wc watchedCollection, handler Handler) error {
	logger := logging.Logger("sync.mongo_watcher")

	backoff := 500 * time.Millisecond
	const maxBackoff = 30 * time.Second

	var resumeToken bson.Raw

	for {
		if ctx.Err() != nil {
			return nil
		}

		err := w.streamCollection(ctx, wc, resumeToken, handler, func(token bson.Raw) {
			resumeToken = token
		})

		if err == nil || ctx.Err() != nil {
			// Graceful shutdown or context cancelled.
			return nil
		}

		if isChangeStreamUnsupportedError(err) {
			return ErrChangeStreamsNotSupported
		}

		logger.Warn("Change stream error; retrying",
			"event", "change_stream_retry",
			"collection", wc.logical,
			"error", err,
			"backoff", backoff,
		)

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
		// Invalidate resume token on repeated failures after a threshold so
		// we do a clean open instead of hitting a stale token error forever.
		if backoff >= maxBackoff {
			resumeToken = nil
		}
	}
}

// streamCollection opens a single change stream cursor and reads events until
// ctx is done or an error occurs. onToken is called after every successfully
// processed event so the caller can persist the resume token.
func (w *MongoChangeWatcher) streamCollection(
	ctx context.Context,
	wc watchedCollection,
	resumeToken bson.Raw,
	handler Handler,
	onToken func(bson.Raw),
) error {
	col := w.db.Collection(wc.full)

	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)
	if resumeToken != nil {
		opts.SetResumeAfter(resumeToken)
	}

	cs, err := col.Watch(ctx, mongo.Pipeline{}, opts)
	if err != nil {
		return err
	}
	defer cs.Close(context.Background())

	for cs.Next(ctx) {
		var raw bson.M
		if err := cs.Decode(&raw); err != nil {
			continue
		}

		event := bsonToChangeEvent(wc.logical, raw)
		handler(event)
		onToken(cs.ResumeToken())
	}

	return cs.Err()
}

// bsonToChangeEvent converts a raw change stream document into a ChangeEvent.
func bsonToChangeEvent(logicalCollection string, raw bson.M) ChangeEvent {
	evt := ChangeEvent{Collection: logicalCollection}

	if op, ok := raw["operationType"].(string); ok {
		evt.Operation = ChangeOperation(op)
	}

	if docKey, ok := raw["documentKey"].(bson.M); ok {
		if id, ok := docKey["_id"].(string); ok {
			evt.DocumentID = id
		}
	}

	if fullDoc, ok := raw["fullDocument"].(bson.M); ok {
		evt.FullDoc = bsonMToStringMap(fullDoc)
	}

	return evt
}

// bsonMToStringMap converts a bson.M to map[string]any with string keys.
func bsonMToStringMap(m bson.M) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case bson.M:
			out[k] = bsonMToStringMap(val)
		default:
			out[k] = val
		}
	}
	return out
}

// isChangeStreamUnsupportedError returns true when MongoDB signals that change
// streams are not available on the target deployment (standalone mongod).
func isChangeStreamUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	var cmdErr mongo.CommandError
	if errors.As(err, &cmdErr) {
		// Error code 40573 = "The $changeStream stage is only supported on
		// replica sets" (or sharded clusters).
		if cmdErr.Code == 40573 {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "change stream") && strings.Contains(msg, "replica set")
}
