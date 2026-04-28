package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/store"
)

func newRedisManager(t *testing.T) (*miniredis.Miniredis, *store.RedisSessionManager) {
	t.Helper()

	redisSrv := miniredis.RunT(t)
	gs, err := store.NewGlobalStore(tempStorePath(t))
	if err != nil {
		t.Fatalf("NewGlobalStore: %v", err)
	}
	if err := gs.Set("seed", "value"); err != nil {
		t.Fatalf("GlobalStore.Set(seed): %v", err)
	}

	cfg := config.SessionConfig{
		StoreType:         config.SessionStoreRedis,
		HeaderName:        "X-Virtual-Session-Id",
		InactivityTimeout: 30 * time.Minute,
		MaxSessions:       100,
		Redis: config.RedisSessionConfig{
			Addr:      redisSrv.Addr(),
			KeyPrefix: "test:sessions",
		},
	}
	cfg.Normalize()

	manager, err := store.NewRedisSessionManager(context.Background(), gs, cfg)
	if err != nil {
		t.Fatalf("NewRedisSessionManager: %v", err)
	}
	return redisSrv, manager
}

func TestRedisSessionManager_GetOrCreateAndResume(t *testing.T) {
	_, manager := newRedisManager(t)

	sess, isNew, err := manager.GetOrCreate("")
	if err != nil {
		t.Fatalf("GetOrCreate new session: %v", err)
	}
	if !isNew {
		t.Fatal("expected first redis session to be new")
	}
	if got, ok := sess.Get("seed"); !ok || got != "value" {
		t.Fatalf("expected redis session to start with global-store seed, got %v, %v", got, ok)
	}
	if err := sess.Set("counter", 2); err != nil {
		t.Fatalf("Session.Set(counter): %v", err)
	}

	sessionID := sess.Info(false).ID
	resumed, isNew, err := manager.GetOrCreate(sessionID)
	if err != nil {
		t.Fatalf("GetOrCreate resume session: %v", err)
	}
	if isNew {
		t.Fatal("expected redis session resume to return existing session")
	}
	if got, ok := resumed.Get("counter"); !ok || got != float64(2) {
		t.Fatalf("expected resumed redis session counter=2, got %v, %v", got, ok)
	}
}

func TestRedisSessionManager_ListAndInvalidate(t *testing.T) {
	_, manager := newRedisManager(t)

	first, _, err := manager.GetOrCreate("session-a")
	if err != nil {
		t.Fatalf("GetOrCreate(session-a): %v", err)
	}
	if _, _, err := manager.GetOrCreate("session-b"); err != nil {
		t.Fatalf("GetOrCreate(session-b): %v", err)
	}

	infos, err := manager.ActiveSessions()
	if err != nil {
		t.Fatalf("ActiveSessions(): %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 active redis sessions, got %d", len(infos))
	}

	if err := manager.Invalidate(first.Info(false).ID); err != nil {
		t.Fatalf("Invalidate(session-a): %v", err)
	}
	count, err := manager.Count()
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 redis session after invalidate, got %d", count)
	}

	if err := manager.InvalidateAll(); err != nil {
		t.Fatalf("InvalidateAll(): %v", err)
	}
	count, err = manager.Count()
	if err != nil {
		t.Fatalf("Count() after invalidate all: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 redis sessions after invalidate all, got %d", count)
	}
}

// ── redisSession.Has ──────────────────────────────────────────────────────────

func TestRedisSession_Has(t *testing.T) {
_, manager := newRedisManager(t)

sess, _, err := manager.GetOrCreate("")
if err != nil {
t.Fatalf("GetOrCreate: %v", err)
}

if !sess.Has("seed") {
t.Error("expected Has('seed') = true")
}
if sess.Has("no-such-key") {
t.Error("expected Has('no-such-key') = false")
}
}

// ── redisSession.Delete ───────────────────────────────────────────────────────

func TestRedisSession_Delete(t *testing.T) {
_, manager := newRedisManager(t)

sess, _, err := manager.GetOrCreate("")
if err != nil {
t.Fatalf("GetOrCreate: %v", err)
}
if err := sess.Set("extra", "value"); err != nil {
t.Fatalf("Set: %v", err)
}
if err := sess.Delete("extra"); err != nil {
t.Fatalf("Delete: %v", err)
}
if sess.Has("extra") {
t.Error("expected 'extra' to be deleted")
}
// Delete the last key so hasStore=false path is covered
if err := sess.Delete("seed"); err != nil {
t.Fatalf("Delete seed: %v", err)
}
if sess.Has("seed") {
t.Error("expected 'seed' to be deleted")
}
}

// ── redisSession.Keys ─────────────────────────────────────────────────────────

func TestRedisSession_Keys(t *testing.T) {
_, manager := newRedisManager(t)

sess, _, err := manager.GetOrCreate("")
if err != nil {
t.Fatalf("GetOrCreate: %v", err)
}
if err := sess.Set("alpha", "1"); err != nil {
t.Fatalf("Set alpha: %v", err)
}
if err := sess.Set("beta", "2"); err != nil {
t.Fatalf("Set beta: %v", err)
}

keys := sess.Keys()
found := make(map[string]bool)
for _, k := range keys {
found[k] = true
}
for _, expected := range []string{"seed", "alpha", "beta"} {
if !found[expected] {
t.Errorf("key %q missing from Keys()", expected)
}
}
}

// ── redisSession.Snapshot ─────────────────────────────────────────────────────

func TestRedisSession_Snapshot(t *testing.T) {
_, manager := newRedisManager(t)

sess, _, err := manager.GetOrCreate("")
if err != nil {
t.Fatalf("GetOrCreate: %v", err)
}
snap := sess.Snapshot()
if snap == nil {
t.Fatal("Snapshot() returned nil")
}
if snap["seed"] != "value" {
t.Errorf("snapshot[seed] = %v, want 'value'", snap["seed"])
}
}

// ── RedisSessionManager.Get ───────────────────────────────────────────────────

func TestRedisSessionManager_Get(t *testing.T) {
_, manager := newRedisManager(t)

sess, _, err := manager.GetOrCreate("get-test-session")
if err != nil {
t.Fatalf("GetOrCreate: %v", err)
}
id := sess.Info(false).ID

loaded, ok, err := manager.Get(id)
if err != nil {
t.Fatalf("Get: %v", err)
}
if !ok {
t.Fatal("expected Get to find existing session")
}
if loaded.Info(false).ID != id {
t.Errorf("loaded session ID mismatch")
}
}

// ── redisSession.Info(includeSnapshot=true) ───────────────────────────────────

func TestRedisSession_Info_WithSnapshot(t *testing.T) {
_, manager := newRedisManager(t)

sess, _, err := manager.GetOrCreate("")
if err != nil {
t.Fatalf("GetOrCreate: %v", err)
}
info := sess.Info(true)
if info.StoreSnapshot == nil {
t.Error("expected non-nil StoreSnapshot with includeSnapshot=true")
}
if _, ok := info.StoreSnapshot["seed"]; !ok {
t.Error("expected 'seed' in StoreSnapshot")
}
}

// ── nil global store (copySessionMap nil path) ────────────────────────────────

func TestRedisSession_NilGlobal(t *testing.T) {
redisSrv := miniredis.RunT(t)
cfg := config.SessionConfig{
StoreType:         config.SessionStoreRedis,
HeaderName:        "X-Virtual-Session-Id",
InactivityTimeout: 30 * time.Minute,
MaxSessions:       100,
Redis: config.RedisSessionConfig{
Addr:      redisSrv.Addr(),
KeyPrefix: "test:sessions",
},
}
cfg.Normalize()
manager, err := store.NewRedisSessionManager(context.Background(), nil, cfg)
if err != nil {
t.Fatalf("NewRedisSessionManager(nil global): %v", err)
}
sess, isNew, err := manager.GetOrCreate("")
if err != nil {
t.Fatalf("GetOrCreate with nil global: %v", err)
}
if !isNew {
t.Fatal("expected new session")
}
if sess.Has("anything") {
t.Error("session should be empty when global is nil")
}
}

// ── MaxSessions=0 → evictOldestIfNeeded early return ─────────────────────────

func TestRedisSessionManager_MaxSessionsZero(t *testing.T) {
redisSrv := miniredis.RunT(t)
gs, err := store.NewGlobalStore(tempStorePath(t))
if err != nil {
t.Fatalf("NewGlobalStore: %v", err)
}
cfg := config.SessionConfig{
StoreType:         config.SessionStoreRedis,
HeaderName:        "X-Virtual-Session-Id",
InactivityTimeout: 30 * time.Minute,
MaxSessions:       0,
Redis: config.RedisSessionConfig{
Addr:      redisSrv.Addr(),
KeyPrefix: "test:sessions",
},
}
cfg.Normalize()
cfg.MaxSessions = 0 // override: Normalize sets <=0 to 10000
manager, err := store.NewRedisSessionManager(context.Background(), gs, cfg)
if err != nil {
t.Fatalf("NewRedisSessionManager: %v", err)
}
for i := 0; i < 5; i++ {
if _, _, err := manager.GetOrCreate(""); err != nil {
t.Fatalf("GetOrCreate[%d]: %v", i, err)
}
}
}

// ── EvictOldest path ──────────────────────────────────────────────────────────

func TestRedisSessionManager_EvictOldest(t *testing.T) {
redisSrv := miniredis.RunT(t)
gs, err := store.NewGlobalStore(tempStorePath(t))
if err != nil {
t.Fatalf("NewGlobalStore: %v", err)
}
cfg := config.SessionConfig{
StoreType:         config.SessionStoreRedis,
HeaderName:        "X-Virtual-Session-Id",
InactivityTimeout: 30 * time.Minute,
MaxSessions:       2,
Redis: config.RedisSessionConfig{
Addr:      redisSrv.Addr(),
KeyPrefix: "test:sessions",
},
}
cfg.Normalize()
manager, err := store.NewRedisSessionManager(context.Background(), gs, cfg)
if err != nil {
t.Fatalf("NewRedisSessionManager: %v", err)
}
for i := 0; i < 3; i++ {
time.Sleep(5 * time.Millisecond) // ensure distinct timestamps
if _, _, err := manager.GetOrCreate(""); err != nil {
t.Fatalf("GetOrCreate[%d]: %v", i, err)
}
}
count, err := manager.Count()
if err != nil {
t.Fatalf("Count: %v", err)
}
if count > 2 {
t.Errorf("expected ≤2 sessions after eviction, got %d", count)
}
}

// ── Stale session pruning ─────────────────────────────────────────────────────

func TestRedisSessionManager_StaleSession(t *testing.T) {
redisSrv, manager := newRedisManager(t)

sess, _, err := manager.GetOrCreate("stale-session")
if err != nil {
t.Fatalf("GetOrCreate: %v", err)
}
id := sess.Info(false).ID

// Remove the metadata key to simulate a stale session (data in index but no meta)
redisSrv.Del("test:sessions:meta:" + id)

infos, err := manager.ActiveSessions()
if err != nil {
t.Fatalf("ActiveSessions: %v", err)
}
for _, info := range infos {
if info.ID == id {
t.Errorf("stale session %q should not appear in ActiveSessions", id)
}
}
}

// ── Ping failure during NewRedisSessionManager ────────────────────────────────

func TestRedisSessionManager_PingError(t *testing.T) {
srv, err := miniredis.Run()
if err != nil {
t.Skipf("miniredis.Run(): %v", err)
}
addr := srv.Addr()
srv.Close() // close before creating manager so Ping fails

gs, err := store.NewGlobalStore(tempStorePath(t))
if err != nil {
t.Fatalf("NewGlobalStore: %v", err)
}
cfg := config.SessionConfig{
StoreType:         config.SessionStoreRedis,
HeaderName:        "X-Virtual-Session-Id",
InactivityTimeout: 30 * time.Minute,
MaxSessions:       100,
Redis: config.RedisSessionConfig{
Addr:      addr,
KeyPrefix: "test:sessions",
},
}
cfg.Normalize()
_, err = store.NewRedisSessionManager(context.Background(), gs, cfg)
if err == nil {
t.Fatal("expected Ping error when Redis is closed")
}
}

// ── loadSession with bad createdAt timestamp (also covers ActiveSessions error) ──

func TestRedisSessionManager_LoadBadTimestamp(t *testing.T) {
redisSrv, manager := newRedisManager(t)

badID := "bad-ts-session"
redisSrv.HSet("test:sessions:meta:"+badID,
"id", badID,
"createdAt", "NOT-A-TIMESTAMP",
"lastActive", time.Now().Format(time.RFC3339Nano),
)
if _, err := redisSrv.ZAdd("test:sessions:index",
float64(time.Now().UnixMilli()), badID); err != nil {
t.Fatalf("ZAdd: %v", err)
}

// GetOrCreate(badID) → loadSession → bad createdAt → error
if _, _, err := manager.GetOrCreate(badID); err == nil {
t.Error("expected error from GetOrCreate with bad timestamp")
}

// ActiveSessions also hits the bad timestamp → loadSession error
if _, err := manager.ActiveSessions(); err == nil {
t.Error("expected error from ActiveSessions with bad session")
}
}

// ── loadSession with bad JSON data value ─────────────────────────────────────

func TestRedisSessionManager_LoadBadJSON(t *testing.T) {
redisSrv, manager := newRedisManager(t)

badID := "bad-json-session"
now := time.Now().Format(time.RFC3339Nano)
redisSrv.HSet("test:sessions:meta:"+badID,
"id", badID,
"createdAt", now,
"lastActive", now,
)
redisSrv.HSet("test:sessions:data:"+badID, "key1", "not-valid-json{{{")
if _, err := redisSrv.ZAdd("test:sessions:index",
float64(time.Now().UnixMilli()), badID); err != nil {
t.Fatalf("ZAdd: %v", err)
}

if _, _, err := manager.GetOrCreate(badID); err == nil {
t.Error("expected error from GetOrCreate with bad JSON data")
}
}

// ── Multiple Redis error paths when connection is closed ──────────────────────

func TestRedisSessionManager_ClosedRedis(t *testing.T) {
redisSrv, manager := newRedisManager(t)

// Create a session while Redis is up, so we have a valid session ID to look up
sess, _, err := manager.GetOrCreate("existing-session")
if err != nil {
t.Fatalf("GetOrCreate: %v", err)
}
existingID := sess.Info(false).ID

redisSrv.Close() // all subsequent Redis operations fail

// GetOrCreate("") → evictOldestIfNeeded → ActiveSessions → ZRevRange error
if _, _, err := manager.GetOrCreate(""); err == nil {
t.Error("expected error from GetOrCreate('') with closed Redis")
}

// GetOrCreate(existingID) → loadSession → HGetAll error
if _, _, err := manager.GetOrCreate(existingID); err == nil {
t.Error("expected error from GetOrCreate(existingID) with closed Redis")
}

// Count → ActiveSessions error
if _, err := manager.Count(); err == nil {
t.Error("expected error from Count with closed Redis")
}

// Invalidate → pipe.Exec error
if err := manager.Invalidate("any-id"); err == nil {
t.Error("expected error from Invalidate with closed Redis")
}

// InvalidateAll → scanKeys (Scan) error
if err := manager.InvalidateAll(); err == nil {
t.Error("expected error from InvalidateAll with closed Redis")
}

// redisSession.Set → pipe.Exec error
if err := sess.Set("k", "v"); err == nil {
t.Error("expected error from session.Set with closed Redis")
}

// redisSession.Delete → pipe.Exec error
if err := sess.Delete("seed"); err == nil {
t.Error("expected error from session.Delete with closed Redis")
}
}

// ── writeFullSession error path ───────────────────────────────────────────────

func TestRedisSessionManager_WriteSessionError(t *testing.T) {
redisSrv := miniredis.RunT(t)
gs, err := store.NewGlobalStore(tempStorePath(t))
if err != nil {
t.Fatalf("NewGlobalStore: %v", err)
}
cfg := config.SessionConfig{
StoreType:         config.SessionStoreRedis,
HeaderName:        "X-Virtual-Session-Id",
InactivityTimeout: 30 * time.Minute,
MaxSessions:       0, // skip eviction
Redis: config.RedisSessionConfig{
Addr:      redisSrv.Addr(),
KeyPrefix: "test:sessions",
},
}
cfg.Normalize()
manager, err := store.NewRedisSessionManager(context.Background(), gs, cfg)
if err != nil {
t.Fatalf("NewRedisSessionManager: %v", err)
}

redisSrv.Close() // close so writeFullSession fails

if _, _, err := manager.GetOrCreate(""); err == nil {
t.Fatal("expected error from GetOrCreate when Redis is closed for write")
}
}
