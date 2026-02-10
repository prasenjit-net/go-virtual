package api


import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/tracing"
	"testing/fstest"
)

func setupTestRouter() *Router {
	store := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)
	proxyEngine := proxy.NewEngine(store, collector, tracingSvc)
	return NewRouter(store, collector, tracingSvc, proxyEngine)
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
