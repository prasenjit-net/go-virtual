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
