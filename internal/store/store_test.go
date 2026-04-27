package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/store"
	"go.starlark.net/starlark"
)

// ── GlobalStore tests ─────────────────────────────────────────────────────

func tempStorePath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "store.json")
}

func mustGetOrCreate(t *testing.T, sm *store.SessionManager, rawID string) (store.SessionState, bool) {
	t.Helper()
	sess, isNew, err := sm.GetOrCreate(rawID)
	if err != nil {
		t.Fatalf("GetOrCreate(%q): %v", rawID, err)
	}
	return sess, isNew
}

func mustGetSession(t *testing.T, sm *store.SessionManager, sessionID string) (store.SessionState, bool) {
	t.Helper()
	sess, ok, err := sm.Get(sessionID)
	if err != nil {
		t.Fatalf("Get(%q): %v", sessionID, err)
	}
	return sess, ok
}

func mustCountSessions(t *testing.T, sm *store.SessionManager) int {
	t.Helper()
	count, err := sm.Count()
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	return count
}

func mustActiveSessions(t *testing.T, sm *store.SessionManager) []models.SessionInfo {
	t.Helper()
	infos, err := sm.ActiveSessions()
	if err != nil {
		t.Fatalf("ActiveSessions(): %v", err)
	}
	return infos
}

func mustSessionID(sess store.SessionState) string {
	return sess.Info(false).ID
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

func makeSession(t *testing.T, snapshot map[string]any) store.SessionState {
	t.Helper()
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
	sess, _ := mustGetOrCreate(t, sm, "")
	return sess
}

func TestSession_GetSet(t *testing.T) {
	gs, _ := store.NewGlobalStore(tempStorePath(t))
	_ = gs.Set("seed", "value")

	cfg := config.SessionConfig{HeaderName: "X-Session", InactivityTimeout: 30 * time.Minute, MaxSessions: 100}
	sm := store.NewSessionManager(context.Background(), gs, cfg)
	sess, isNew := mustGetOrCreate(t, sm, "")

	if !isNew {
		t.Fatal("first session should be new")
	}

	v, ok := sess.Get("seed")
	if !ok || v != "value" {
		t.Errorf("expected seed=value from snapshot, got %v, %v", v, ok)
	}

	if err := sess.Set("counter", 1); err != nil {
		t.Fatalf("Set(counter): %v", err)
	}
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
	sess := makeSession(t, map[string]any{"exists": "yes"})

	if !sess.Has("exists") {
		t.Fatal("expected Has to return true for existing key")
	}
	if sess.Has("missing") {
		t.Fatal("expected Has to return false for missing key")
	}
}

func TestSession_Delete(t *testing.T) {
	sess := makeSession(t, map[string]any{"del": "me"})

	if err := sess.Delete("del"); err != nil {
		t.Fatalf("Delete(del): %v", err)
	}
	if sess.Has("del") {
		t.Fatal("key should be gone after delete")
	}
}

func TestSession_Keys(t *testing.T) {
	sess := makeSession(t, map[string]any{"b": 1, "a": 2, "c": 3})
	keys := sess.Keys()

	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("keys not sorted: %v", keys)
	}
}

func TestSession_Snapshot(t *testing.T) {
	sess := makeSession(t, nil)
	if err := sess.Set("x", 42); err != nil {
		t.Fatalf("Set(x): %v", err)
	}

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

	sess, isNew := mustGetOrCreate(t, sm, "")
	if !isNew {
		t.Fatal("expected new session")
	}
	if mustSessionID(sess) == "" {
		t.Fatal("session ID must not be empty")
	}
}

func TestSessionManager_GetOrCreate_Resume(t *testing.T) {
	_, sm := newManager(t)

	sess1, _ := mustGetOrCreate(t, sm, "")
	sess2, isNew := mustGetOrCreate(t, sm, mustSessionID(sess1))

	if isNew {
		t.Fatal("should have resumed existing session")
	}
	if mustSessionID(sess1) != mustSessionID(sess2) {
		t.Fatal("should return same session on resume")
	}
}

func TestSessionManager_GetOrCreate_UnknownIDCreatesNew(t *testing.T) {
	_, sm := newManager(t)

	sess, isNew := mustGetOrCreate(t, sm, "my-custom-session-id")
	if !isNew {
		t.Fatal("unknown ID should produce a new session")
	}
	// The provided ID must be adopted as the session ID
	if got := mustSessionID(sess); got != "my-custom-session-id" {
		t.Fatalf("expected session ID 'my-custom-session-id', got %q", got)
	}
}

func TestSessionManager_GetOrCreate_UnknownIDCanBeResumed(t *testing.T) {
	_, sm := newManager(t)

	// First request with a custom ID creates the session
	sess1, isNew := mustGetOrCreate(t, sm, "user-session-abc")
	if !isNew {
		t.Fatal("first request should create a new session")
	}
	if got := mustSessionID(sess1); got != "user-session-abc" {
		t.Fatalf("expected ID 'user-session-abc', got %q", got)
	}

	// Second request with the same custom ID resumes the session
	sess2, isNew := mustGetOrCreate(t, sm, "user-session-abc")
	if isNew {
		t.Fatal("second request with same ID should resume existing session")
	}
	if got := mustSessionID(sess2); got != "user-session-abc" {
		t.Fatalf("expected ID 'user-session-abc', got %q", got)
	}
}

func TestSessionManager_Invalidate(t *testing.T) {
	_, sm := newManager(t)

	sess, _ := mustGetOrCreate(t, sm, "")
	if err := sm.Invalidate(mustSessionID(sess)); err != nil {
		t.Fatalf("Invalidate(): %v", err)
	}

	_, ok := mustGetSession(t, sm, mustSessionID(sess))
	if ok {
		t.Fatal("session should not exist after invalidate")
	}
}

func TestSessionManager_InvalidateAll(t *testing.T) {
	_, sm := newManager(t)

	mustGetOrCreate(t, sm, "")
	mustGetOrCreate(t, sm, "")
	mustGetOrCreate(t, sm, "")

	if got := mustCountSessions(t, sm); got != 3 {
		t.Fatalf("expected 3 sessions, got %d", got)
	}

	if err := sm.InvalidateAll(); err != nil {
		t.Fatalf("InvalidateAll(): %v", err)
	}

	if got := mustCountSessions(t, sm); got != 0 {
		t.Fatalf("expected 0 sessions after InvalidateAll, got %d", got)
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

	mustGetOrCreate(t, sm, "")
	mustGetOrCreate(t, sm, "")
	mustGetOrCreate(t, sm, "")
	// This one should evict the oldest
	mustGetOrCreate(t, sm, "")

	if got := mustCountSessions(t, sm); got > 3 {
		t.Errorf("expected at most 3 sessions, got %d", got)
	}
}

func TestSessionManager_ActiveSessions(t *testing.T) {
	_, sm := newManager(t)

	mustGetOrCreate(t, sm, "")
	mustGetOrCreate(t, sm, "")

	infos := mustActiveSessions(t, sm)
	if len(infos) != 2 {
		t.Fatalf("expected 2 active sessions, got %d", len(infos))
	}
}

// ── StoreBuiltin tests ────────────────────────────────────────────────────

func makeBuiltin(t *testing.T) (*store.StoreBuiltin, store.SessionState) {
	t.Helper()
	gs, _ := store.NewGlobalStore(filepath.Join(os.TempDir(), "builtin-test-"+time.Now().Format("150405.000000000")+".json"))
	_ = gs.Set("seed", "hello")

	cfg := config.SessionConfig{HeaderName: "X-Session", InactivityTimeout: 30 * time.Minute, MaxSessions: 100}
	sm := store.NewSessionManager(context.Background(), gs, cfg)
	sess, _ := mustGetOrCreate(t, sm, "")

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
	sb, _ := makeBuiltin(t)
	v := callMethod(t, sb, "get", starlark.String("seed"))
	if v.(starlark.String) != "hello" {
		t.Errorf("expected 'hello', got %v", v)
	}
}

func TestStoreBuiltin_GetMissing_ReturnsNone(t *testing.T) {
	sb, _ := makeBuiltin(t)
	v := callMethod(t, sb, "get", starlark.String("missing"))
	if v != starlark.None {
		t.Errorf("expected None, got %v", v)
	}
}

func TestStoreBuiltin_GetWithDefault(t *testing.T) {
	sb, _ := makeBuiltin(t)
	v := callMethod(t, sb, "get", starlark.String("missing"), starlark.MakeInt(42))
	if v.(starlark.Int).BigInt().Int64() != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}

func TestStoreBuiltin_Set(t *testing.T) {
	sb, sess := makeBuiltin(t)
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
	sb, _ := makeBuiltin(t)
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
	sb, sess := makeBuiltin(t)
	callMethod(t, sb, "delete", starlark.String("seed"))

	if sess.Has("seed") {
		t.Fatal("seed should be gone after delete")
	}
}

func TestStoreBuiltin_Keys(t *testing.T) {
	sb, _ := makeBuiltin(t)
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
	sess, _ := mustGetOrCreate(t, sm, "")

	// NewStoreBuiltin with nil accessLog must not panic on any operation
	sb := store.NewStoreBuiltin(sess, nil)
	callMethod(t, sb, "get", starlark.String("x"))
	callMethod(t, sb, "set", starlark.String("y"), starlark.MakeInt(1))
	callMethod(t, sb, "has", starlark.String("x"))
	callMethod(t, sb, "delete", starlark.String("x"))
	callMethod(t, sb, "keys")
}

func TestStoreBuiltin_AttrNames(t *testing.T) {
	sb, _ := makeBuiltin(t)
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

// ── GlobalStore.Len ──────────────────────────────────────────────────────────

func TestGlobalStore_Len(t *testing.T) {
	gs, _ := store.NewGlobalStore(tempStorePath(t))

	if gs.Len() != 0 {
		t.Errorf("expected Len=0 for empty store, got %d", gs.Len())
	}
	_ = gs.Set("a", 1.0)
	_ = gs.Set("b", 2.0)
	if gs.Len() != 2 {
		t.Errorf("expected Len=2, got %d", gs.Len())
	}
	_ = gs.Delete("a")
	if gs.Len() != 1 {
		t.Errorf("expected Len=1 after delete, got %d", gs.Len())
	}
}

// ── NewEphemeralSession ───────────────────────────────────────────────────────

func TestNewEphemeralSession(t *testing.T) {
	sess := store.NewEphemeralSession(map[string]any{"k": "v"})
	if sess == nil {
		t.Fatal("expected non-nil session")
	}
	if sess.ID != "__ephemeral__" {
		t.Errorf("ID = %q, want __ephemeral__", sess.ID)
	}
	v, ok := sess.Get("k")
	if !ok || v != "v" {
		t.Errorf("expected k=v, got %v, %v", v, ok)
	}
}

func TestNewEphemeralSession_NilSnapshot(t *testing.T) {
	sess := store.NewEphemeralSession(nil)
	if sess == nil {
		t.Fatal("expected non-nil session even with nil snapshot")
	}
	// Should not panic and should have empty store.
	if sess.Has("anything") {
		t.Error("expected empty session store")
	}
}

// ── StoreBuiltin interface methods ───────────────────────────────────────────

func TestStoreBuiltin_Interface(t *testing.T) {
	sb, _ := makeBuiltin(t)

	if got := sb.String(); got != "store" {
		t.Errorf("String() = %q, want 'store'", got)
	}
	if got := sb.Type(); got != "store" {
		t.Errorf("Type() = %q, want 'store'", got)
	}
	if sb.Truth() != starlark.True {
		t.Error("Truth() should be True")
	}
	sb.Freeze() // must not panic
	if _, err := sb.Hash(); err == nil {
		t.Error("Hash() should return an error (store is not hashable)")
	}
}

// ── goToStar / starToGo type coverage ────────────────────────────────────────

func TestStoreBuiltin_TypeConversions(t *testing.T) {
	sess := store.NewEphemeralSession(map[string]any{
		"str":  "hello",
		"num":  42.0,
		"bval": true,
		"list": []any{"a", "b"},
		"dict": map[string]any{"x": 1.0},
	})
	sb := store.NewStoreBuiltin(sess, nil)

	// Execute Starlark that reads complex types (goToStar) and writes them back (starToGo).
	src := `
bval   = store.get("bval")
lval   = store.get("list")
dval   = store.get("dict")
store.set("new_bool",  True)
store.set("new_int",   99)
store.set("new_list",  ["x", "y"])
store.set("new_dict",  {"key": "val"})
store.set("nil_val",   None)
`
	thread := &starlark.Thread{Name: "type-test"}
	_, err := starlark.ExecFile(thread, "test.star", src, starlark.StringDict{"store": sb})
	if err != nil {
		t.Fatalf("ExecFile: %v", err)
	}

	// Verify starToGo round-trips.
	if v, ok := sess.Get("new_bool"); !ok || v != true {
		t.Errorf("new_bool = %v, want true", v)
	}
	if v, ok := sess.Get("new_int"); !ok {
		t.Error("new_int missing")
	} else if n, ok2 := v.(int64); !ok2 || n != 99 {
		t.Errorf("new_int = %v (%T), want int64(99)", v, v)
	}
	if v, ok := sess.Get("new_list"); !ok {
		t.Error("new_list missing")
	} else if sl, ok2 := v.([]any); !ok2 || len(sl) != 2 {
		t.Errorf("new_list = %v, want []any len 2", v)
	}
	if v, ok := sess.Get("new_dict"); !ok {
		t.Error("new_dict missing")
	} else if m, ok2 := v.(map[string]any); !ok2 || m["key"] != "val" {
		t.Errorf("new_dict = %v", v)
	}
	if v, ok := sess.Get("nil_val"); !ok || v != nil {
		t.Errorf("nil_val = %v, want nil", v)
	}
}
