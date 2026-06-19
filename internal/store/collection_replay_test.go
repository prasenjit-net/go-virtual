package store_test

import (
	"encoding/json"
	"testing"

	"github.com/prasenjit/go-virtual/internal/store"
)

func TestReplayEvents_Insert(t *testing.T) {
	base := []map[string]any{{"_id": "1", "name": "alice"}}
	events := []store.CollectionEvent{
		{Op: "insert", Data: map[string]any{"name": "bob"}},
	}
	result := store.ReplayEvents(base, events)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	if result[1]["name"] != "bob" {
		t.Errorf("expected bob, got %v", result[1]["name"])
	}
	// Auto-assigned _id
	if result[1]["_id"] == nil || result[1]["_id"] == "" {
		t.Error("expected _id to be auto-assigned on insert")
	}
}

func TestReplayEvents_InsertPreservesID(t *testing.T) {
	result := store.ReplayEvents(nil, []store.CollectionEvent{
		{Op: "insert", Data: map[string]any{"_id": "custom", "x": 1}},
	})
	if result[0]["_id"] != "custom" {
		t.Errorf("expected custom _id, got %v", result[0]["_id"])
	}
}

func TestReplayEvents_Update(t *testing.T) {
	base := []map[string]any{
		{"_id": "1", "name": "alice", "age": 30},
		{"_id": "2", "name": "bob"},
	}
	events := []store.CollectionEvent{
		{Op: "update", Filter: map[string]any{"name": "alice"}, Data: map[string]any{"age": 31, "city": "Paris"}},
	}
	result := store.ReplayEvents(base, events)
	if result[0]["age"] != 31 || result[0]["city"] != "Paris" {
		t.Errorf("update not applied: %v", result[0])
	}
	// bob unaffected
	if _, ok := result[1]["city"]; ok {
		t.Error("bob should not have city")
	}
}

func TestReplayEvents_UpdateNoMatch(t *testing.T) {
	base := []map[string]any{{"_id": "1", "name": "alice"}}
	events := []store.CollectionEvent{
		{Op: "update", Filter: map[string]any{"name": "nobody"}, Data: map[string]any{"age": 99}},
	}
	result := store.ReplayEvents(base, events)
	if _, ok := result[0]["age"]; ok {
		t.Error("age should not be set when no match")
	}
}

func TestReplayEvents_Upsert_Found(t *testing.T) {
	base := []map[string]any{{"_id": "1", "sku": "A1", "qty": 5}}
	events := []store.CollectionEvent{
		{Op: "upsert", Filter: map[string]any{"sku": "A1"}, Data: map[string]any{"qty": 10}},
	}
	result := store.ReplayEvents(base, events)
	if len(result) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(result))
	}
	if result[0]["qty"] != 10 {
		t.Errorf("expected qty=10, got %v", result[0]["qty"])
	}
}

func TestReplayEvents_Upsert_NotFound(t *testing.T) {
	base := []map[string]any{{"_id": "1", "sku": "A1"}}
	events := []store.CollectionEvent{
		{Op: "upsert", Filter: map[string]any{"sku": "B2"}, Data: map[string]any{"qty": 7}},
	}
	result := store.ReplayEvents(base, events)
	if len(result) != 2 {
		t.Fatalf("expected 2 docs after upsert insert, got %d", len(result))
	}
	if result[1]["sku"] != "B2" || result[1]["qty"] != 7 {
		t.Errorf("unexpected upserted doc: %v", result[1])
	}
	if result[1]["_id"] == nil || result[1]["_id"] == "" {
		t.Error("upserted doc should have _id")
	}
}

func TestReplayEvents_Delete(t *testing.T) {
	base := []map[string]any{
		{"_id": "1", "name": "alice"},
		{"_id": "2", "name": "bob"},
		{"_id": "3", "name": "alice"},
	}
	events := []store.CollectionEvent{
		{Op: "delete", Filter: map[string]any{"name": "alice"}},
	}
	result := store.ReplayEvents(base, events)
	// Only first match deleted
	if len(result) != 2 {
		t.Fatalf("expected 2 remaining, got %d", len(result))
	}
	if result[0]["_id"] != "2" {
		t.Errorf("expected bob first, got %v", result[0])
	}
	if result[1]["_id"] != "3" {
		t.Errorf("expected second alice remaining, got %v", result[1])
	}
}

func TestReplayEvents_Clear(t *testing.T) {
	base := []map[string]any{{"_id": "1"}, {"_id": "2"}}
	events := []store.CollectionEvent{{Op: "clear"}}
	result := store.ReplayEvents(base, events)
	if len(result) != 0 {
		t.Errorf("expected empty after clear, got %d", len(result))
	}
}

func TestReplayEvents_BaseNotMutated(t *testing.T) {
	base := []map[string]any{{"_id": "1", "name": "alice"}}
	events := []store.CollectionEvent{
		{Op: "update", Filter: map[string]any{"_id": "1"}, Data: map[string]any{"name": "mutated"}},
	}
	store.ReplayEvents(base, events)
	if base[0]["name"] != "alice" {
		t.Error("base must not be mutated by ReplayEvents")
	}
}

func TestReplayEvents_EmptyEvents(t *testing.T) {
	base := []map[string]any{{"_id": "1", "name": "alice"}}
	result := store.ReplayEvents(base, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(result))
	}
}

// ── LoadEvents ────────────────────────────────────────────────────────────────

func TestLoadEvents_NilWhenAbsent(t *testing.T) {
	sess := store.NewEphemeralSession(nil)
	evts := store.LoadEvents(sess, "users")
	if evts != nil {
		t.Errorf("expected nil, got %v", evts)
	}
}

func TestLoadEvents_NativeSlice(t *testing.T) {
	sess := store.NewEphemeralSession(nil)
	want := []store.CollectionEvent{{Op: "insert", Data: map[string]any{"x": 1}}}
	sess.Set(store.CollectionEventKeyPrefix+"users", want)
	got := store.LoadEvents(sess, "users")
	if len(got) != 1 || got[0].Op != "insert" {
		t.Errorf("expected insert event, got %v", got)
	}
}

func TestLoadEvents_JSONRoundTrip(t *testing.T) {
	// Simulates Redis deserialization where the slice comes back as []any
	sess := store.NewEphemeralSession(nil)
	evts := []store.CollectionEvent{{Op: "delete", Filter: map[string]any{"name": "bob"}}}
	raw, _ := json.Marshal(evts)
	var asAny []any
	json.Unmarshal(raw, &asAny)
	sess.Set(store.CollectionEventKeyPrefix+"users", asAny)

	got := store.LoadEvents(sess, "users")
	if len(got) != 1 || got[0].Op != "delete" {
		t.Errorf("expected delete event from []any, got %v", got)
	}
}

func TestLoadEvents_StringJSON(t *testing.T) {
	// Simulates Redis string serialization
	sess := store.NewEphemeralSession(nil)
	evts := []store.CollectionEvent{{Op: "clear"}}
	b, _ := json.Marshal(evts)
	sess.Set(store.CollectionEventKeyPrefix+"users", string(b))

	got := store.LoadEvents(sess, "users")
	if len(got) != 1 || got[0].Op != "clear" {
		t.Errorf("expected clear event from string, got %v", got)
	}
}

func TestLoadEvents_UnknownType(t *testing.T) {
	sess := store.NewEphemeralSession(nil)
	sess.Set(store.CollectionEventKeyPrefix+"users", 42) // integer — unsupported
	got := store.LoadEvents(sess, "users")
	if got != nil {
		t.Errorf("expected nil for unknown type, got %v", got)
	}
}
