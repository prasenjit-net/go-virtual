//go:build !unit

package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestNewMongoChangeWatcher(t *testing.T) {
	w := NewMongoChangeWatcher(nil, "gv_", []string{"specs", "operations"})
	if len(w.collections) != 2 {
		t.Fatalf("expected 2 watched collections, got %d", len(w.collections))
	}
	if w.collections[0].logical != "specs" || w.collections[0].full != "gv_specs" {
		t.Fatalf("unexpected first collection: %#v", w.collections[0])
	}
}

func TestMongoChangeWatcherWatchAndHelpers(t *testing.T) {
	w := NewMongoChangeWatcher(nil, "gv_", nil)
	if err := w.Watch(context.Background(), func(ChangeEvent) {}); err != nil {
		t.Fatalf("expected nil error for empty watcher, got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wc := watchedCollection{logical: "specs", full: "gv_specs"}
	if err := w.watchCollection(ctx, wc, func(ChangeEvent) {}); err != nil {
		t.Fatalf("expected nil error on cancelled context, got %v", err)
	}

	evt := bsonToChangeEvent("specs", bson.M{
		"operationType": "replace",
		"documentKey":   bson.M{"_id": "doc-1"},
		"fullDocument":  bson.M{"name": "demo", "nested": bson.M{"count": 2}},
	})
	if evt.Collection != "specs" || evt.Operation != ChangeOpReplace || evt.DocumentID != "doc-1" {
		t.Fatalf("unexpected change event: %#v", evt)
	}
	nested := evt.FullDoc["nested"].(map[string]any)
	if nested["count"] != 2 {
		t.Fatalf("unexpected nested document: %#v", evt.FullDoc)
	}

	if !isChangeStreamUnsupportedError(mongo.CommandError{Code: 40573}) {
		t.Fatal("expected replica-set command error to be recognised")
	}
	if !isChangeStreamUnsupportedError(errors.New("change stream only supported on replica set deployments")) {
		t.Fatal("expected message-based unsupported error to be recognised")
	}
	if isChangeStreamUnsupportedError(nil) {
		t.Fatal("expected nil error to be false")
	}
}

func TestNewPollingWatcherAndWatchCancel(t *testing.T) {
	configured := NewPollingWatcher(nil, "gv_", []string{"specs", "responses"}, 5*time.Millisecond)
	if len(configured.collections) != 2 {
		t.Fatalf("expected 2 collections, got %d", len(configured.collections))
	}
	if configured.collections[1].full != "gv_responses" {
		t.Fatalf("unexpected full name: %#v", configured.collections[1])
	}

	w := NewPollingWatcher(nil, "gv_", nil, 5*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(15 * time.Millisecond)
		cancel()
	}()
	if err := w.Watch(ctx, func(ChangeEvent) {}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
