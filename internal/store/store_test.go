package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/store"
	"go.starlark.net/starlark"
)

// ── GlobalStore tests ─────────────────────────────────────────────────────

func tempStorePath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "store.json")
}

func TestGlobalStore_NewCreatesFile(t *testing.T) {
	path := tempStorePath(t)
	gs, err := store.NewGlobalStore(path)
	if err != nil {
		t.Fatalf("NewGlobalStore: %v", err)
	}
	if gs == nil {
		t.Fatal("expected non-nil GlobalStore")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("store.json should have been created on first run")
	}
}

func TestGlobalStore_SetAndGet(t *testing.T) {
	gs, _ := store.NewGlobalStore(tempStorePath(t))

	if err := gs.Set("foo", "bar"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, ok := gs.Get("foo")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if v != "bar" {
		t.Errorf("expected 'bar', got %v", v)
	}
}

func TestGlobalStore_GetMissing(t *testing.T) {
	gs, _ := store.NewGlobalStore(tempStorePath(t))

	_, ok := gs.Get("missing")
	if ok {
		t.Fatal("expected ok=false for missing key")
	}
}

func TestGlobalStore_Delete(t *testing.T) {
	gs, _ := store.NewGlobalStore(tempStorePath(t))

	_ = gs.Set("k", 42.0)
	if err := gs.Delete("k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, ok := gs.Get("k")
	if ok {
		t.Fatal("key should be gone after delete")
	}
}

func TestGlobalStore_Clear(t *testing.T) {
	gs, _ := store.NewGlobalStore(tempStorePath(t))

	_ = gs.Set("a", 1.0)
	_ = gs.Set("b", 2.0)
	if err := gs.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	entries := gs.List()
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries after clear, got %d", len(entries))
	}
}

func TestGlobalStore_List(t *testing.T) {
	gs, _ := store.NewGlobalStore(tempStorePath(t))

	_ = gs.Set("z", "last")
	_ = gs.Set("a", "first")
	_ = gs.Set("m", "middle")

	entries := gs.List()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Should be sorted by key
	if entries[0].Key != "a" || entries[1].Key != "m" || entries[2].Key != "z" {
		t.Errorf("list not sorted: %v", entries)
	}
}

func TestGlobalStore_Persistence(t *testing.T) {
	path := tempStorePath(t)

	gs, _ := store.NewGlobalStore(path)
	_ = gs.Set("counter", 99.0)

	// Reload from file
	gs2, err := store.NewGlobalStore(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	v, ok := gs2.Get("counter")
	if !ok {
		t.Fatal("counter should persist")
	}
	if v.(float64) != 99.0 {
		t.Errorf("expected 99, got %v", v)
	}
}

func TestGlobalStore_Snapshot(t *testing.T) {
	gs, _ := store.NewGlobalStore(tempStorePath(t))

	_ = gs.Set("x", map[string]any{"nested": true})

	snap := gs.Snapshot()
	if _, ok := snap["x"]; !ok {
		t.Fatal("snapshot should include key x")
	}

	// Modify snapshot — should not affect store
	snap["x"] = "overwritten"
	v, _ := gs.Get("x")
	if v.(map[string]any)["nested"] != true {
		t.Fatal("snapshot mutation leaked into store")
	}
}

// ── Session tests ─────────────────────────────────────────────────────────

func makeSession(snapshot map[string]any) *store.Session {
	// Use the SessionManager to create a session via GetOrCreate
	gs, _ := store.NewGlobalStore(filepath.Join(os.TempDir(), "test-session-"+time.Now().Format("150405.000000000")+".json"))
	for k, v := range snapshot {
		_ = gs.Set(k, v)
	}
	cfg := config.SessionConfig{
		HeaderName:        "X-Virtual-Session-Id",
		InactivityTimeout: 30 * time.Minute,
		MaxSessions:       100,
	}
	sm := store.NewSessionManager(context.Background(), gs, cfg)
	sess, _ := sm.GetOrCreate("")
	return sess
}

func TestSession_GetSet(t *testing.T) {
	gs, _ := store.NewGlobalStore(tempStorePath(t))
	_ = gs.Set("seed", "value")

	cfg := config.SessionConfig{HeaderName: "X-Session", InactivityTimeout: 30 * time.Minute, MaxSessions: 100}
	sm := store.NewSessionManager(context.Background(), gs, cfg)
	sess, isNew := sm.GetOrCreate("")

	if !isNew {
		t.Fatal("first session should be new")
	}

	v, ok := sess.Get("seed")
	if !ok || v != "value" {
		t.Errorf("expected seed=value from snapshot, got %v, %v", v, ok)
	}

	sess.Set("counter", 1)
	v2, ok2 := sess.Get("counter")
	if !ok2 || v2 != 1 {
		t.Errorf("expected counter=1, got %v", v2)
	}

	// Must not have propagated to global
	_, globalHas := gs.Get("counter")
	if globalHas {
		t.Fatal("session write should not propagate to global")
	}
}

func TestSession_Has(t *testing.T) {
	sess := makeSession(map[string]any{"exists": "yes"})

	if !sess.Has("exists") {
		t.Fatal("expected Has to return true for existing key")
	}
	if sess.Has("missing") {
		t.Fatal("expected Has to return false for missing key")
	}
}

func TestSession_Delete(t *testing.T) {
	sess := makeSession(map[string]any{"del": "me"})

	sess.Delete("del")
	if sess.Has("del") {
		t.Fatal("key should be gone after delete")
	}
}

func TestSession_Keys(t *testing.T) {
	sess := makeSession(map[string]any{"b": 1, "a": 2, "c": 3})
	keys := sess.Keys()

	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("keys not sorted: %v", keys)
	}
}

func TestSession_Snapshot(t *testing.T) {
	sess := makeSession(nil)
	sess.Set("x", 42)

	snap := sess.Snapshot()
	if snap["x"] != 42 {
		t.Errorf("expected snapshot[x]=42, got %v", snap["x"])
	}

	// Mutate snapshot — should not affect session
	snap["x"] = 999
	v, _ := sess.Get("x")
	if v != 42 {
		t.Fatal("snapshot mutation should not affect session")
	}
}

// ── SessionManager tests ──────────────────────────────────────────────────

func newManager(t *testing.T) (*store.GlobalStore, *store.SessionManager) {
	t.Helper()
	gs, _ := store.NewGlobalStore(tempStorePath(t))
	cfg := config.SessionConfig{
		HeaderName:        "X-Session",
		InactivityTimeout: 30 * time.Minute,
		MaxSessions:       1000,
	}
	sm := store.NewSessionManager(context.Background(), gs, cfg)
	return gs, sm
}

func TestSessionManager_GetOrCreate_NewSession(t *testing.T) {
	_, sm := newManager(t)

	sess, isNew := sm.GetOrCreate("")
	if !isNew {
		t.Fatal("expected new session")
	}
	if sess.ID == "" {
		t.Fatal("session ID must not be empty")
	}
}

func TestSessionManager_GetOrCreate_Resume(t *testing.T) {
	_, sm := newManager(t)

	sess1, _ := sm.GetOrCreate("")
	sess2, isNew := sm.GetOrCreate(sess1.ID)

	if isNew {
		t.Fatal("should have resumed existing session")
	}
	if sess1.ID != sess2.ID {
		t.Fatal("should return same session on resume")
	}
}

func TestSessionManager_GetOrCreate_UnknownIDCreatesNew(t *testing.T) {
	_, sm := newManager(t)

	sess, isNew := sm.GetOrCreate("my-custom-session-id")
	if !isNew {
		t.Fatal("unknown ID should produce a new session")
	}
	// The provided ID must be adopted as the session ID
	if sess.ID != "my-custom-session-id" {
		t.Fatalf("expected session ID 'my-custom-session-id', got %q", sess.ID)
	}
}

func TestSessionManager_GetOrCreate_UnknownIDCanBeResumed(t *testing.T) {
	_, sm := newManager(t)

	// First request with a custom ID creates the session
	sess1, isNew := sm.GetOrCreate("user-session-abc")
	if !isNew {
		t.Fatal("first request should create a new session")
	}
	if sess1.ID != "user-session-abc" {
		t.Fatalf("expected ID 'user-session-abc', got %q", sess1.ID)
	}

	// Second request with the same custom ID resumes the session
	sess2, isNew := sm.GetOrCreate("user-session-abc")
	if isNew {
		t.Fatal("second request with same ID should resume existing session")
	}
	if sess2.ID != "user-session-abc" {
		t.Fatalf("expected ID 'user-session-abc', got %q", sess2.ID)
	}
}

func TestSessionManager_Invalidate(t *testing.T) {
	_, sm := newManager(t)

	sess, _ := sm.GetOrCreate("")
	sm.Invalidate(sess.ID)

	_, ok := sm.Get(sess.ID)
	if ok {
		t.Fatal("session should not exist after invalidate")
	}
}

func TestSessionManager_InvalidateAll(t *testing.T) {
	_, sm := newManager(t)

	sm.GetOrCreate("")
	sm.GetOrCreate("")
	sm.GetOrCreate("")

	if sm.Count() != 3 {
		t.Fatalf("expected 3 sessions, got %d", sm.Count())
	}

	sm.InvalidateAll()

	if sm.Count() != 0 {
		t.Fatalf("expected 0 sessions after InvalidateAll, got %d", sm.Count())
	}
}

func TestSessionManager_MaxSessions_Evicts(t *testing.T) {
	gs, _ := store.NewGlobalStore(tempStorePath(t))
	cfg := config.SessionConfig{
		HeaderName:        "X-Session",
		InactivityTimeout: 30 * time.Minute,
		MaxSessions:       3,
	}
	sm := store.NewSessionManager(context.Background(), gs, cfg)

	sm.GetOrCreate("")
	sm.GetOrCreate("")
	sm.GetOrCreate("")
	// This one should evict the oldest
	sm.GetOrCreate("")

	if sm.Count() > 3 {
		t.Errorf("expected at most 3 sessions, got %d", sm.Count())
	}
}

func TestSessionManager_ActiveSessions(t *testing.T) {
	_, sm := newManager(t)

	sm.GetOrCreate("")
	sm.GetOrCreate("")

	infos := sm.ActiveSessions()
	if len(infos) != 2 {
		t.Fatalf("expected 2 active sessions, got %d", len(infos))
	}
}

// ── StoreBuiltin tests ────────────────────────────────────────────────────

func makeBuiltin() (*store.StoreBuiltin, *store.Session) {
	gs, _ := store.NewGlobalStore(filepath.Join(os.TempDir(), "builtin-test-"+time.Now().Format("150405.000000000")+".json"))
	_ = gs.Set("seed", "hello")

	cfg := config.SessionConfig{HeaderName: "X-Session", InactivityTimeout: 30 * time.Minute, MaxSessions: 100}
	sm := store.NewSessionManager(context.Background(), gs, cfg)
	sess, _ := sm.GetOrCreate("")

	var log []interface{}
	_ = log
	return store.NewStoreBuiltin(sess, nil), sess
}

func callMethod(t *testing.T, sb *store.StoreBuiltin, name string, args ...starlark.Value) starlark.Value {
	t.Helper()
	attr, err := sb.Attr(name)
	if err != nil || attr == nil {
		t.Fatalf("Attr(%q) failed: %v / %v", name, attr, err)
	}
	fn := attr.(*starlark.Builtin)
	tuple := make(starlark.Tuple, len(args))
	for i, a := range args {
		tuple[i] = a
	}
	result, err := starlark.Call(&starlark.Thread{}, fn, tuple, nil)
	if err != nil {
		t.Fatalf("Call %s: %v", name, err)
	}
	return result
}

func TestStoreBuiltin_GetExisting(t *testing.T) {
	sb, _ := makeBuiltin()
	v := callMethod(t, sb, "get", starlark.String("seed"))
	if v.(starlark.String) != "hello" {
		t.Errorf("expected 'hello', got %v", v)
	}
}

func TestStoreBuiltin_GetMissing_ReturnsNone(t *testing.T) {
	sb, _ := makeBuiltin()
	v := callMethod(t, sb, "get", starlark.String("missing"))
	if v != starlark.None {
		t.Errorf("expected None, got %v", v)
	}
}

func TestStoreBuiltin_GetWithDefault(t *testing.T) {
	sb, _ := makeBuiltin()
	v := callMethod(t, sb, "get", starlark.String("missing"), starlark.MakeInt(42))
	if v.(starlark.Int).BigInt().Int64() != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}

func TestStoreBuiltin_Set(t *testing.T) {
	sb, sess := makeBuiltin()
	callMethod(t, sb, "set", starlark.String("counter"), starlark.MakeInt(1))

	v, ok := sess.Get("counter")
	if !ok {
		t.Fatal("key should exist after set")
	}
	if v.(int64) != 1 {
		t.Errorf("expected 1, got %v", v)
	}
}

func TestStoreBuiltin_Has(t *testing.T) {
	sb, _ := makeBuiltin()
	v := callMethod(t, sb, "has", starlark.String("seed"))
	if v.(starlark.Bool) != starlark.True {
		t.Error("expected has(seed) = True")
	}

	v2 := callMethod(t, sb, "has", starlark.String("missing"))
	if v2.(starlark.Bool) != starlark.False {
		t.Error("expected has(missing) = False")
	}
}

func TestStoreBuiltin_Delete(t *testing.T) {
	sb, sess := makeBuiltin()
	callMethod(t, sb, "delete", starlark.String("seed"))

	if sess.Has("seed") {
		t.Fatal("seed should be gone after delete")
	}
}

func TestStoreBuiltin_Keys(t *testing.T) {
	sb, _ := makeBuiltin()
	callMethod(t, sb, "set", starlark.String("z"), starlark.String("last"))
	callMethod(t, sb, "set", starlark.String("a"), starlark.String("first"))

	v := callMethod(t, sb, "keys")
	lst := v.(*starlark.List)
	if lst.Len() < 2 {
		t.Fatalf("expected at least 2 keys, got %d", lst.Len())
	}
}

func TestStoreBuiltin_AccessLog(t *testing.T) {
	gs, _ := store.NewGlobalStore(tempStorePath(t))
	_ = gs.Set("x", "val")

	cfg := config.SessionConfig{HeaderName: "X-Session", InactivityTimeout: 30 * time.Minute, MaxSessions: 100}
	sm := store.NewSessionManager(context.Background(), gs, cfg)
	sess, _ := sm.GetOrCreate("")

	// NewStoreBuiltin with nil accessLog must not panic on any operation
	sb := store.NewStoreBuiltin(sess, nil)
	callMethod(t, sb, "get", starlark.String("x"))
	callMethod(t, sb, "set", starlark.String("y"), starlark.MakeInt(1))
	callMethod(t, sb, "has", starlark.String("x"))
	callMethod(t, sb, "delete", starlark.String("x"))
	callMethod(t, sb, "keys")
}

func TestStoreBuiltin_AttrNames(t *testing.T) {
	sb, _ := makeBuiltin()
	names := sb.AttrNames()

	expected := map[string]bool{"get": true, "set": true, "has": true, "delete": true, "keys": true}
	for _, n := range names {
		if !expected[n] {
			t.Errorf("unexpected attr: %s", n)
		}
		delete(expected, n)
	}
	if len(expected) != 0 {
		t.Errorf("missing attrs: %v", expected)
	}
}
