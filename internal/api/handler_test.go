package api

import (
	"regexp"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

// setupTestHandler creates a minimal Handler + gin.Engine for unit tests.
func setupTestHandler(t *testing.T) (*Handler, storage.Storage, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	store := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)
	proxyEngine := proxy.NewEngine(store, collector, tracingSvc)

	handler := NewHandler(HandlerConfig{
		Store:          store,
		StatsCollector: collector,
		TracingService: tracingSvc,
		ProxyEngine:    proxyEngine,
	})

	r := gin.New()
	return handler, store, r
}

func TestNewHandler(t *testing.T) {
	handler, _, _ := setupTestHandler(t)

	if handler == nil {
		t.Fatal("Expected handler to be created")
	}
	if handler.parser == nil {
		t.Error("Expected parser to be initialized")
	}
}

func TestGenerateID(t *testing.T) {
	pattern := regexp.MustCompile(`^\d{14}-[0-9a-f]{8}$`)
	seen := make(map[string]struct{}, 128)

	for i := 0; i < 128; i++ {
		id := generateID()
		if !pattern.MatchString(id) {
			t.Fatalf("unexpected ID format: %q", id)
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate ID generated: %q", id)
		}
		seen[id] = struct{}{}
	}
}
