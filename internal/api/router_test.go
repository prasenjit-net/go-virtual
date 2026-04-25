package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

func setupTestRouter() *Router {
	store := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)
	proxyEngine := proxy.NewEngine(store, collector, tracingSvc)
	return NewRouter(RouterConfig{
		Store:          store,
		StatsCollector: collector,
		TracingService: tracingSvc,
		ProxyEngine:    proxyEngine,
	})
}

func TestRouter_CORSOptions(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest("OPTIONS", "/_api/health", nil)
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatalf("expected CORS headers to be set")
	}
}

func TestRouter_StatsStreamRoute(t *testing.T) {
	router := setupTestRouter()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/_api/stats/stream", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		router.Handler().ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stats stream route did not exit after context cancellation")
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", got)
	}
	if !strings.Contains(w.Body.String(), "event: stats") {
		t.Fatalf("expected stats SSE event, got %q", w.Body.String())
	}
}

func TestServeUIFromFS_MissingDir(t *testing.T) {
	router := setupTestRouter()
	router.ServeUIFromFS(filepath.Join(t.TempDir(), "missing"))

	req := httptest.NewRequest("GET", "/_ui/anything", nil)
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}
}

func TestServeUIFromFS_IndexFallback(t *testing.T) {
	router := setupTestRouter()

	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html>index</html>"), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	router.ServeUIFromFS(dir)

	req := httptest.NewRequest("GET", "/_ui/unknown", nil)
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if w.Body.String() == "" {
		t.Fatalf("expected index content to be served")
	}
}

func TestServeEmbeddedUI(t *testing.T) {
	router := setupTestRouter()

	uiFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>embedded</html>")},
		"app.js":     &fstest.MapFile{Data: []byte("console.log('app')")},
	}

	router.ServeEmbeddedUI(uiFS)

	req := httptest.NewRequest("GET", "/_ui/app.js", nil)
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/_ui/unknown", nil)
	w = httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if w.Body.String() == "" {
		t.Fatalf("expected index fallback")
	}

	req = httptest.NewRequest("GET", "/_ui", nil)
	w = httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected status 301, got %d", w.Code)
	}
}

func TestServeEmbeddedUI_DoesNotHijackProxyRoutes(t *testing.T) {
	router := setupTestRouter()

	uiFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>embedded</html>")},
	}

	router.ServeEmbeddedUI(uiFS)

	req := httptest.NewRequest("GET", "/pets", nil)
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	if w.Code == http.StatusOK && strings.Contains(w.Body.String(), "embedded") {
		t.Fatalf("expected non-UI routes to bypass the SPA handler")
	}
}

func TestServeDocsFromFS_MissingDir(t *testing.T) {
	router := setupTestRouter()
	router.ServeDocsFromFS(filepath.Join(t.TempDir(), "missing"))

	req := httptest.NewRequest("GET", "/_docs/anything", nil)
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}
}

func TestServeDocsFromFS(t *testing.T) {
	router := setupTestRouter()
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html>docs</html>"), 0644); err != nil {
		t.Fatalf("failed to write docs index: %v", err)
	}

	router.ServeDocsFromFS(dir)

	req := httptest.NewRequest("GET", "/_docs/", nil)
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "docs") {
		t.Fatalf("expected docs file to be served, got status=%d body=%q", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("GET", "/_docs", nil)
	w = httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected /_docs redirect, got %d", w.Code)
	}
}

func TestServeEmbeddedDocs(t *testing.T) {
	router := setupTestRouter()
	docsFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>embedded docs</html>")},
		"guide.html": &fstest.MapFile{Data: []byte("<html>guide</html>")},
	}

	router.ServeEmbeddedDocs(docsFS)

	req := httptest.NewRequest("GET", "/_docs/guide.html", nil)
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "guide") {
		t.Fatalf("expected embedded guide to be served, got status=%d body=%q", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("GET", "/_docs/", nil)
	w = httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "embedded docs") {
		t.Fatalf("expected embedded index to be served, got status=%d body=%q", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("GET", "/_docs/missing.html", nil)
	w = httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing embedded doc, got %d", w.Code)
	}
}

func TestHeadlessRouter_AdminAPIDisabled(t *testing.T) {
	stor := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)
	proxyEngine := proxy.NewEngine(stor, collector, tracingSvc)
	router := NewRouter(RouterConfig{
		Store:          stor,
		StatsCollector: collector,
		TracingService: tracingSvc,
		ProxyEngine:    proxyEngine,
		Headless:       true,
	})

	// Admin API should return 404 (not registered)
	for _, path := range []string{"/_api/health", "/_api/specs", "/_api/routes"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		router.Handler().ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			t.Errorf("expected admin route %s to be disabled in headless mode, got 200", path)
		}
	}
}

func TestHeadlessRouter_UIDisabled(t *testing.T) {
	stor := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)
	proxyEngine := proxy.NewEngine(stor, collector, tracingSvc)
	router := NewRouter(RouterConfig{
		Store:          stor,
		StatsCollector: collector,
		TracingService: tracingSvc,
		ProxyEngine:    proxyEngine,
		Headless:       true,
	})

	// UI routes should not be registered; request goes to proxy (which returns 404 for no match)
	req := httptest.NewRequest("GET", "/_ui/", nil)
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)
	// Proxy returns 404 because no spec matches /_ui/ — but it must NOT return 200 as if UI was served
	if w.Code == http.StatusOK {
		t.Errorf("expected /_ui/ to NOT be served in headless mode")
	}
}
