package store_test

import (
	"testing"

	"github.com/prasenjit/go-virtual/internal/store"
)

func TestFileCollectionBackend_CRUD(t *testing.T) {
	dir := t.TempDir()
	fb, err := store.NewFileCollectionBackend(dir)
	if err != nil {
		t.Fatalf("NewFileCollectionBackend: %v", err)
	}

	// GetAll on non-existent collection returns nil, no error
	docs, err := fb.GetAll("users")
	if err != nil {
		t.Fatalf("GetAll empty: %v", err)
	}
	if docs != nil {
		t.Fatalf("expected nil for missing collection, got %v", docs)
	}

	// SeedInsert — auto-assigns _id when absent
	d1, err := fb.SeedInsert("users", map[string]any{"name": "alice"})
	if err != nil {
		t.Fatalf("SeedInsert alice: %v", err)
	}
	if d1["_id"] == "" {
		t.Error("expected _id to be assigned")
	}
	if d1["name"] != "alice" {
		t.Errorf("expected name=alice, got %v", d1["name"])
	}

	// SeedInsert — preserves existing _id
	d2, err := fb.SeedInsert("users", map[string]any{"_id": "bob-id", "name": "bob"})
	if err != nil {
		t.Fatalf("SeedInsert bob: %v", err)
	}
	if d2["_id"] != "bob-id" {
		t.Errorf("expected _id=bob-id, got %v", d2["_id"])
	}

	// GetAll returns both docs
	all, err := fb.GetAll("users")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(all))
	}

	// ListCollections includes "users"
	names, err := fb.ListCollections()
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
	found := false
	for _, n := range names {
		if n == "users" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'users' in ListCollections, got %v", names)
	}

	// SeedClear empties the collection
	if err := fb.SeedClear("users"); err != nil {
		t.Fatalf("SeedClear: %v", err)
	}
	cleared, err := fb.GetAll("users")
	if err != nil {
		t.Fatalf("GetAll after clear: %v", err)
	}
	if len(cleared) != 0 {
		t.Errorf("expected empty after SeedClear, got %d", len(cleared))
	}

	// File still exists after clear (holds [])
	names2, err := fb.ListCollections()
	if err != nil {
		t.Fatalf("ListCollections after clear: %v", err)
	}
	found2 := false
	for _, n := range names2 {
		if n == "users" {
			found2 = true
		}
	}
	if !found2 {
		t.Error("expected users file to remain after SeedClear")
	}

	// DropCollection removes the file
	if err := fb.DropCollection("users"); err != nil {
		t.Fatalf("DropCollection: %v", err)
	}
	names3, err := fb.ListCollections()
	if err != nil {
		t.Fatalf("ListCollections after drop: %v", err)
	}
	for _, n := range names3 {
		if n == "users" {
			t.Error("expected users to be gone after DropCollection")
		}
	}

	// DropCollection on non-existent is a no-op
	if err := fb.DropCollection("nonexistent"); err != nil {
		t.Errorf("DropCollection non-existent should not error: %v", err)
	}
}

func TestFileCollectionBackend_Persistence(t *testing.T) {
	dir := t.TempDir()
	fb, _ := store.NewFileCollectionBackend(dir)

	if _, err := fb.SeedInsert("items", map[string]any{"sku": "A1"}); err != nil {
		t.Fatal(err)
	}

	// Re-open the same directory — data must survive
	fb2, err := store.NewFileCollectionBackend(dir)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	docs, err := fb2.GetAll("items")
	if err != nil {
		t.Fatalf("GetAll after reopen: %v", err)
	}
	if len(docs) != 1 || docs[0]["sku"] != "A1" {
		t.Errorf("expected persisted doc, got %v", docs)
	}
}

func TestFileCollectionBackend_MultipleCollections(t *testing.T) {
	dir := t.TempDir()
	fb, _ := store.NewFileCollectionBackend(dir)

	fb.SeedInsert("a", map[string]any{"x": 1})
	fb.SeedInsert("b", map[string]any{"y": 2})
	fb.SeedInsert("b", map[string]any{"y": 3})

	names, err := fb.ListCollections()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 collections, got %d: %v", len(names), names)
	}

	docsA, _ := fb.GetAll("a")
	docsB, _ := fb.GetAll("b")
	if len(docsA) != 1 {
		t.Errorf("a: expected 1, got %d", len(docsA))
	}
	if len(docsB) != 2 {
		t.Errorf("b: expected 2, got %d", len(docsB))
	}
}
