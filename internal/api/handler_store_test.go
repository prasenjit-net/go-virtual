package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

// setupTestHandlerWithStore creates a Handler with GlobalStore + SessionManager wired in.
func setupTestHandlerWithStore(t *testing.T) (*Handler, *store.GlobalStore, *store.SessionManager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	stor := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)
	proxyEngine := proxy.NewEngine(stor, collector, tracingSvc)

	gs, err := store.NewGlobalStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("NewGlobalStore: %v", err)
	}
	cfg := config.SessionConfig{
		HeaderName:        "X-Virtual-Session-Id",
		InactivityTimeout: 30 * time.Minute,
		MaxSessions:       100,
	}
	sm := store.NewSessionManager(context.Background(), gs, cfg)

	handler := NewHandler(HandlerConfig{
		Store:          stor,
		StatsCollector: collector,
		TracingService: tracingSvc,
		ProxyEngine:    proxyEngine,
		GlobalStore:    gs,
		SessionManager: sm,
	})

	r := gin.New()
	return handler, gs, sm, r
}

// ── ListStoreEntries ──────────────────────────────────────────────────────────

func TestListStoreEntries_NoStore(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/store", handler.ListStoreEntries)

	req := httptest.NewRequest("GET", "/store", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestListStoreEntries_Empty(t *testing.T) {
	handler, _, _, r := setupTestHandlerWithStore(t)
	r.GET("/store", handler.ListStoreEntries)

	req := httptest.NewRequest("GET", "/store", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ── UpsertStoreEntry ──────────────────────────────────────────────────────────

func TestUpsertStoreEntry_Success(t *testing.T) {
	handler, _, _, r := setupTestHandlerWithStore(t)
	r.PUT("/store/:key", handler.UpsertStoreEntry)

	req := httptest.NewRequest("PUT", "/store/greeting", bytes.NewBufferString(`{"value":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ── GetStoreEntry ─────────────────────────────────────────────────────────────

func TestGetStoreEntry_Success(t *testing.T) {
	handler, gs, _, r := setupTestHandlerWithStore(t)
	r.GET("/store/:key", handler.GetStoreEntry)

	_ = gs.Set("greet", "world")

	req := httptest.NewRequest("GET", "/store/greet", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetStoreEntry_NotFound(t *testing.T) {
	handler, _, _, r := setupTestHandlerWithStore(t)
	r.GET("/store/:key", handler.GetStoreEntry)

	req := httptest.NewRequest("GET", "/store/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── DeleteStoreEntry ──────────────────────────────────────────────────────────

func TestDeleteStoreEntry_Success(t *testing.T) {
	handler, gs, _, r := setupTestHandlerWithStore(t)
	r.DELETE("/store/:key", handler.DeleteStoreEntry)

	_ = gs.Set("to-remove", "val")

	req := httptest.NewRequest("DELETE", "/store/to-remove", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

// ── ClearStore ────────────────────────────────────────────────────────────────

func TestClearStore_NoConfirm(t *testing.T) {
	handler, _, _, r := setupTestHandlerWithStore(t)
	r.DELETE("/store", handler.ClearStore)

	req := httptest.NewRequest("DELETE", "/store", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestClearStore_Success(t *testing.T) {
	handler, gs, _, r := setupTestHandlerWithStore(t)
	r.DELETE("/store", handler.ClearStore)

	_ = gs.Set("k", "v")

	req := httptest.NewRequest("DELETE", "/store?confirm=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

// ── ListSessions ──────────────────────────────────────────────────────────────

func TestListSessions_NoManager(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/sessions", handler.ListSessions)

	req := httptest.NewRequest("GET", "/sessions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestListSessions_Empty(t *testing.T) {
	handler, _, _, r := setupTestHandlerWithStore(t)
	r.GET("/sessions", handler.ListSessions)

	req := httptest.NewRequest("GET", "/sessions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ── GetSession ────────────────────────────────────────────────────────────────

func TestGetSession_NotFound(t *testing.T) {
	handler, _, _, r := setupTestHandlerWithStore(t)
	r.GET("/sessions/:id", handler.GetSession)

	req := httptest.NewRequest("GET", "/sessions/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── InvalidateSession ─────────────────────────────────────────────────────────

func TestInvalidateSession_Success(t *testing.T) {
	handler, _, _, r := setupTestHandlerWithStore(t)
	r.DELETE("/sessions/:id", handler.InvalidateSession)

	req := httptest.NewRequest("DELETE", "/sessions/any-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

// ── InvalidateAllSessions ─────────────────────────────────────────────────────

func TestInvalidateAllSessions_NoConfirm(t *testing.T) {
	handler, _, _, r := setupTestHandlerWithStore(t)
	r.DELETE("/sessions", handler.InvalidateAllSessions)

	req := httptest.NewRequest("DELETE", "/sessions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestInvalidateAllSessions_Success(t *testing.T) {
	handler, _, _, r := setupTestHandlerWithStore(t)
	r.DELETE("/sessions", handler.InvalidateAllSessions)

	req := httptest.NewRequest("DELETE", "/sessions?confirm=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestUpsertStoreEntry_InvalidJSON(t *testing.T) {
	handler, _, _, r := setupTestHandlerWithStore(t)
	r.PUT("/store/:key", handler.UpsertStoreEntry)

	req := httptest.NewRequest("PUT", "/store/mykey", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClearStore_MissingConfirm(t *testing.T) {
	handler, _, _, r := setupTestHandlerWithStore(t)
	r.DELETE("/store", handler.ClearStore)

	req := httptest.NewRequest("DELETE", "/store?confirm=false", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetSession_Success(t *testing.T) {
	handler, _, sm, r := setupTestHandlerWithStore(t)
	r.GET("/sessions/:id", handler.GetSession)

	// Create a session
	sess, _, err := sm.GetOrCreate("test-session-id")
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	req := httptest.NewRequest("GET", "/sessions/"+sess.Info(false).ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteStoreEntry_NoStore(t *testing.T) {
handler, _, r := setupTestHandler(t)
r.DELETE("/store/:key", handler.DeleteStoreEntry)

req := httptest.NewRequest("DELETE", "/store/somekey", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusServiceUnavailable {
t.Errorf("expected 503, got %d", w.Code)
}
}

func TestUpsertStoreEntry_NoStore(t *testing.T) {
handler, _, r := setupTestHandler(t)
r.PUT("/store/:key", handler.UpsertStoreEntry)

body := `{"value": "test"}`
req := httptest.NewRequest("PUT", "/store/somekey", bytes.NewBufferString(body))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusServiceUnavailable {
t.Errorf("expected 503, got %d", w.Code)
}
}

func TestClearStore_NoStore(t *testing.T) {
handler, _, r := setupTestHandler(t)
r.DELETE("/store", handler.ClearStore)

req := httptest.NewRequest("DELETE", "/store?confirm=true", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusServiceUnavailable {
t.Errorf("expected 503, got %d", w.Code)
}
}
