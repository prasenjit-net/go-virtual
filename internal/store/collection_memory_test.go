package store_test

import (
	"testing"

	"github.com/prasenjit/go-virtual/internal/store"
)

func TestMemoryCollectionBackend_CRUD(t *testing.T) {
	mb := store.NewMemoryCollectionBackend()

	// Empty collection returns nil
	docs, err := mb.GetAll("users")
	if err != nil {
		t.Fatalf("GetAll empty: %v", err)
	}
	if docs != nil {
		t.Fatalf("expected nil for missing collection, got %v", docs)
	}

	// SeedInsert auto-assigns _id
	d1, err := mb.SeedInsert("users", map[string]any{"name": "alice"})
	if err != nil {
		t.Fatalf("SeedInsert: %v", err)
	}
	if d1["_id"] == "" || d1["_id"] == nil {
		t.Error("expected _id to be assigned")
	}

	// SeedInsert preserves existing _id
	d2, err := mb.SeedInsert("users", map[string]any{"_id": "bob-id", "name": "bob"})
	if err != nil {
		t.Fatalf("SeedInsert bob: %v", err)
	}
	if d2["_id"] != "bob-id" {
		t.Errorf("expected bob-id, got %v", d2["_id"])
	}

	// GetAll returns copies (mutations don't affect stored docs)
	all, err := mb.GetAll("users")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
	all[0]["name"] = "mutated"
	all2, _ := mb.GetAll("users")
	if all2[0]["name"] == "mutated" {
		t.Error("GetAll must return copies; mutation leaked into backend")
	}

	// ListCollections
	names, err := mb.ListCollections()
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
	if len(names) != 1 || names[0] != "users" {
		t.Errorf("expected [users], got %v", names)
	}

	// SeedClear empties collection
	if err := mb.SeedClear("users"); err != nil {
		t.Fatalf("SeedClear: %v", err)
	}
	cleared, _ := mb.GetAll("users")
	if len(cleared) != 0 {
		t.Errorf("expected empty after SeedClear, got %d", len(cleared))
	}

	// DropCollection removes it from ListCollections
	mb.SeedInsert("things", map[string]any{"x": 1})
	if err := mb.DropCollection("things"); err != nil {
		t.Fatalf("DropCollection: %v", err)
	}
	names2, _ := mb.ListCollections()
	for _, n := range names2 {
		if n == "things" {
			t.Error("things should be dropped")
		}
	}

	// DropCollection on missing is a no-op
	if err := mb.DropCollection("missing"); err != nil {
		t.Errorf("DropCollection missing: %v", err)
	}
}

func TestMemoryCollectionBackend_MultipleCollections(t *testing.T) {
	mb := store.NewMemoryCollectionBackend()

	mb.SeedInsert("a", map[string]any{"v": 1})
	mb.SeedInsert("b", map[string]any{"v": 2})
	mb.SeedInsert("b", map[string]any{"v": 3})

	names, _ := mb.ListCollections()
	if len(names) != 2 {
		t.Fatalf("expected 2 collections, got %d", len(names))
	}

	docsA, _ := mb.GetAll("a")
	docsB, _ := mb.GetAll("b")
	if len(docsA) != 1 {
		t.Errorf("a: expected 1, got %d", len(docsA))
	}
	if len(docsB) != 2 {
		t.Errorf("b: expected 2, got %d", len(docsB))
	}
}
