package collection

import (
	"testing"

	"github.com/prasenjit/go-virtual/internal/store"
)

func newTestOps(name string) *Ops {
	sess := store.NewEphemeralSession(nil)
	return NewOps(name, sess)
}

func TestInsertAssignsID(t *testing.T) {
	ops := newTestOps("users")
	doc, err := ops.Insert(map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	if doc["_id"] == "" {
		t.Error("expected _id to be assigned")
	}
	if doc["name"] != "Alice" {
		t.Error("expected name=Alice")
	}
}

func TestInsertPreservesExistingID(t *testing.T) {
	ops := newTestOps("users")
	doc, err := ops.Insert(map[string]any{"_id": "custom-id", "name": "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	if doc["_id"] != "custom-id" {
		t.Errorf("expected _id=custom-id, got %v", doc["_id"])
	}
}

func TestFindOne(t *testing.T) {
	ops := newTestOps("items")
	ops.Insert(map[string]any{"_id": "1", "color": "red"})
	ops.Insert(map[string]any{"_id": "2", "color": "blue"})

	found, err := ops.FindOne(map[string]any{"_id": "2"})
	if err != nil {
		t.Fatal(err)
	}
	if found == nil {
		t.Fatal("expected a document")
	}
	if found["color"] != "blue" {
		t.Errorf("expected color=blue, got %v", found["color"])
	}
}

func TestFindOneNoMatch(t *testing.T) {
	ops := newTestOps("items")
	found, err := ops.FindOne(map[string]any{"_id": "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Error("expected nil for no match")
	}
}

func TestFindMany(t *testing.T) {
	ops := newTestOps("orders")
	ops.Insert(map[string]any{"status": "pending"})
	ops.Insert(map[string]any{"status": "done"})
	ops.Insert(map[string]any{"status": "pending"})

	results, err := ops.FindMany(map[string]any{"status": "pending"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestUpdate(t *testing.T) {
	ops := newTestOps("users")
	ops.Insert(map[string]any{"_id": "u1", "name": "Alice", "status": "active"})

	updated, err := ops.Update(map[string]any{"_id": "u1"}, map[string]any{"status": "inactive"})
	if err != nil {
		t.Fatal(err)
	}
	if updated == nil {
		t.Fatal("expected updated document")
	}
	if updated["status"] != "inactive" {
		t.Errorf("expected status=inactive, got %v", updated["status"])
	}
	if updated["name"] != "Alice" {
		t.Error("expected name to be preserved")
	}
}

func TestUpdateNoMatch(t *testing.T) {
	ops := newTestOps("users")
	updated, err := ops.Update(map[string]any{"_id": "nope"}, map[string]any{"x": "y"})
	if err != nil {
		t.Fatal(err)
	}
	if updated != nil {
		t.Error("expected nil for no match")
	}
}

func TestUpsertExisting(t *testing.T) {
	ops := newTestOps("users")
	ops.Insert(map[string]any{"_id": "u1", "name": "Alice"})

	doc, err := ops.Upsert(map[string]any{"_id": "u1"}, map[string]any{"name": "Alice Updated"})
	if err != nil {
		t.Fatal(err)
	}
	if doc["name"] != "Alice Updated" {
		t.Errorf("expected updated name, got %v", doc["name"])
	}

	// Confirm only one document exists
	all, _ := ops.FindMany(nil)
	if len(all) != 1 {
		t.Errorf("expected 1 document, got %d", len(all))
	}
}

func TestUpsertInserts(t *testing.T) {
	ops := newTestOps("users")
	doc, err := ops.Upsert(map[string]any{"_id": "new1"}, map[string]any{"name": "New"})
	if err != nil {
		t.Fatal(err)
	}
	if doc["name"] != "New" {
		t.Errorf("expected name=New, got %v", doc["name"])
	}
	all, _ := ops.FindMany(nil)
	if len(all) != 1 {
		t.Errorf("expected 1 document, got %d", len(all))
	}
}

func TestDelete(t *testing.T) {
	ops := newTestOps("users")
	ops.Insert(map[string]any{"_id": "d1", "name": "ToDelete"})

	deleted, err := ops.Delete(map[string]any{"_id": "d1"})
	if err != nil {
		t.Fatal(err)
	}
	if deleted == nil {
		t.Fatal("expected deleted document to be returned")
	}
	if deleted["name"] != "ToDelete" {
		t.Errorf("expected name=ToDelete, got %v", deleted["name"])
	}

	remaining, _ := ops.FindMany(nil)
	if len(remaining) != 0 {
		t.Errorf("expected 0 remaining, got %d", len(remaining))
	}
}

func TestDeleteNoMatch(t *testing.T) {
	ops := newTestOps("users")
	deleted, err := ops.Delete(map[string]any{"_id": "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != nil {
		t.Error("expected nil for no match")
	}
}

func TestSharedSessionWithStarlarkFormat(t *testing.T) {
	// Verify that Ops reads the same format that store.CollectionBuiltin writes.
	// CollectionBuiltin stores as []any, Ops.load() must handle that.
	sess := store.NewEphemeralSession(nil)
	key := "__col__shared"

	// Simulate what CollectionBuiltin.save does
	rawDocs := []any{
		map[string]any{"_id": "x1", "val": "hello"},
	}
	_ = sess.Set(key, rawDocs)

	ops := NewOps("shared", sess)
	found, err := ops.FindOne(map[string]any{"_id": "x1"})
	if err != nil {
		t.Fatal(err)
	}
	if found == nil {
		t.Fatal("expected to find document written by CollectionBuiltin-style save")
	}
	if found["val"] != "hello" {
		t.Errorf("expected val=hello, got %v", found["val"])
	}
}
