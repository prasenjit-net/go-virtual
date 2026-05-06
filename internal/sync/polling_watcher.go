//go:build !unit

package sync

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/prasenjit/go-virtual/internal/logging"
)

// docFingerprint holds a lightweight snapshot used to detect document changes.
type docFingerprint struct {
	dataHash [32]byte // SHA-256 of the raw "data" BSON field
}

// pollCollection tracks the last-seen fingerprints for one collection.
type pollCollection struct {
	logical string
	full    string
	seen    map[string]docFingerprint // document _id → fingerprint
}

// PollingWatcher implements ChangeWatcher by periodically querying MongoDB
// collections for changes.  It is used as a fallback on standalone mongod
// deployments that do not support change streams.
//
// On every tick the watcher fetches all document IDs and their raw "data"
// field from each watched collection, computes SHA-256 fingerprints, and
// emits synthetic ChangeEvents for documents that are new, changed, or absent
// since the last scan.
type PollingWatcher struct {
	db           *mongo.Database
	collections  []*pollCollection
	pollInterval time.Duration
}

// NewPollingWatcher creates a PollingWatcher.
// pollInterval controls how often the collections are scanned.
func NewPollingWatcher(db *mongo.Database, prefix string, logicalCollections []string, pollInterval time.Duration) *PollingWatcher {
	cols := make([]*pollCollection, len(logicalCollections))
	for i, name := range logicalCollections {
		cols[i] = &pollCollection{
			logical: name,
			full:    prefix + name,
			seen:    make(map[string]docFingerprint),
		}
	}
	return &PollingWatcher{
		db:           db,
		collections:  cols,
		pollInterval: pollInterval,
	}
}

// Watch starts the polling loop and blocks until ctx is cancelled.
// It always returns nil (polling never fails permanently).
func (w *PollingWatcher) Watch(ctx context.Context, handler Handler) error {
	logger := logging.Logger("sync.polling_watcher")
	logger.Info("Starting polling-based change watcher",
		"event", "polling_watcher_started",
		"interval", w.pollInterval,
	)

	// Do an initial scan to seed the fingerprints without emitting events.
	// This prevents a burst of false-positive insert events on startup.
	w.seedAll(ctx)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for _, col := range w.collections {
				if err := w.pollOne(ctx, col, handler); err != nil {
					logger.Warn("Polling scan failed",
						"event", "polling_scan_error",
						"collection", col.logical,
						"error", err,
					)
				}
			}
		}
	}
}

// seedAll performs an initial scan to populate the seen fingerprint maps
// without emitting any events. Called once at startup.
func (w *PollingWatcher) seedAll(ctx context.Context) {
	for _, col := range w.collections {
		current, err := w.scanCollection(ctx, col.full)
		if err != nil {
			continue
		}
		col.seen = current
	}
}

// pollOne runs a single scan on col and calls handler for each detected change.
func (w *PollingWatcher) pollOne(ctx context.Context, col *pollCollection, handler Handler) error {
	current, err := w.scanCollection(ctx, col.full)
	if err != nil {
		return err
	}

	// Detect inserts and updates.
	for id, fp := range current {
		prev, existed := col.seen[id]
		if !existed {
			handler(ChangeEvent{
				Collection: col.logical,
				Operation:  ChangeOpInsert,
				DocumentID: id,
			})
		} else if prev.dataHash != fp.dataHash {
			handler(ChangeEvent{
				Collection: col.logical,
				Operation:  ChangeOpReplace,
				DocumentID: id,
			})
		}
	}

	// Detect deletes.
	for id := range col.seen {
		if _, stillPresent := current[id]; !stillPresent {
			handler(ChangeEvent{
				Collection: col.logical,
				Operation:  ChangeOpDelete,
				DocumentID: id,
			})
		}
	}

	col.seen = current
	return nil
}

// scanCollection fetches all documents in the given collection and returns a
// map of document ID → fingerprint.
func (w *PollingWatcher) scanCollection(ctx context.Context, fullName string) (map[string]docFingerprint, error) {
	col := w.db.Collection(fullName)
	scanCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Projection: only _id and data (the JSON blob). The global_store
	// collection uses the same layout as genericDoc so this works for all
	// watched collections.
	cursor, err := col.Find(scanCtx, bson.M{}, nil)
	if err != nil {
		return nil, fmt.Errorf("polling scan: %w", err)
	}
	defer cursor.Close(scanCtx)

	fps := make(map[string]docFingerprint)
	for cursor.Next(scanCtx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			continue
		}
		id, _ := raw["_id"].(string)
		if id == "" {
			continue
		}
		// Compute a fingerprint over the raw data field so we detect updates.
		dataField, _ := raw["data"].(string)
		fp := docFingerprint{
			dataHash: sha256.Sum256([]byte(dataField)),
		}
		fps[id] = fp
	}
	return fps, cursor.Err()
}
