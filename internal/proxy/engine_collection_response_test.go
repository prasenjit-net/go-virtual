package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

const collResponseTestSpecContent = `{
  "openapi": "3.0.0",
  "info": {"title": "t", "version": "1.0"},
  "paths": {
    "/users/{id}": {
      "get": {
        "operationId": "getUser",
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {
                "example": {"id": "placeholder", "name": "placeholder"}
              }
            }
          }
        }
      }
    }
  }
}`

// setupCollectionTestEngine wires a session manager and an in-memory
// collection backend so Collection Responses can run their queries.
func setupCollectionTestEngine(t *testing.T) (*Engine, storage.Storage, *store.MemoryCollectionBackend) {
	t.Helper()
	s := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)

	engine := NewEngine(s, collector, tracingSvc)
	sessionManager := store.NewSessionManager(context.Background(), nil, config.SessionConfig{})
	engine.SetSessionManager(sessionManager, "X-Session-Id")
	backend := store.NewMemoryCollectionBackend()
	engine.SetCollectionBackend(backend)
	return engine, s, backend
}

func setupUserSpecAndOperation(t *testing.T, s storage.Storage) {
	t.Helper()
	spec := &models.Spec{
		ID:                 "spec-1",
		Name:               "Test API",
		BasePath:           "/api",
		Enabled:            true,
		UseExampleFallback: false,
		Content:            collResponseTestSpecContent,
	}
	if err := s.CreateSpec(spec); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	op := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users/{id}",
		FullPath: "/api/users/{id}",
	}
	if err := s.CreateOperation(op); err != nil {
		t.Fatalf("CreateOperation: %v", err)
	}
}

func collectionResponseConfig(id string, priority int) *models.ResponseConfig {
	return &models.ResponseConfig{
		ID:          id,
		OperationID: "op-1",
		Name:        "Collection " + id,
		StatusCode:  200,
		Priority:    priority,
		Enabled:     true,
		Kind:        models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{
				CollectionName: "users",
				FilterRules: []models.CollectionFilter{
					{TargetPath: "_id", Value: models.ValueBinding{Source: models.ValueSourcePath, Key: "id"}},
				},
			},
		},
	}
}

func TestServeHTTP_CollectionResponse_MatchesAndFillsFromSpecTemplate(t *testing.T) {
	engine, s, backend := setupCollectionTestEngine(t)
	setupUserSpecAndOperation(t, s)
	if _, err := backend.SeedInsert("users", map[string]any{"_id": "42", "id": "42", "name": "Alice"}); err != nil {
		t.Fatal(err)
	}

	cfg := collectionResponseConfig("cr-1", 1)
	if err := s.CreateResponseConfig(cfg); err != nil {
		t.Fatal(err)
	}
	engine.ReloadRoutes()

	req := httptest.NewRequest("GET", "/api/users/42", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %s", w.Body.String())
	}
	if body["id"] != "42" || body["name"] != "Alice" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestServeHTTP_CollectionResponse_EmptyFallsThroughToManual(t *testing.T) {
	engine, s, _ := setupCollectionTestEngine(t)
	setupUserSpecAndOperation(t, s)

	collCfg := collectionResponseConfig("cr-1", 1) // higher priority, but its collection is empty
	manualCfg := &models.ResponseConfig{
		ID:          "manual-1",
		OperationID: "op-1",
		Name:        "Manual fallback",
		StatusCode:  200,
		Priority:    2,
		Enabled:     true,
		Body:        `{"id": "{{.path.id}}", "name": "manual"}`,
		Headers:     map[string]string{"Content-Type": "application/json"},
	}
	if err := s.CreateResponseConfig(collCfg); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateResponseConfig(manualCfg); err != nil {
		t.Fatal(err)
	}
	engine.ReloadRoutes()

	req := httptest.NewRequest("GET", "/api/users/42", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %s", w.Body.String())
	}
	if body["name"] != "manual" {
		t.Fatalf("expected the manual fallback to be served, got %#v", body)
	}
}

func TestServeHTTP_CollectionResponse_MatchOnEmptyRendersInsteadOfFallingThrough(t *testing.T) {
	engine, s, _ := setupCollectionTestEngine(t)
	setupUserSpecAndOperation(t, s)

	collCfg := collectionResponseConfig("cr-1", 1)
	collCfg.CollectionResponse.MatchOnEmpty = true
	manualCfg := &models.ResponseConfig{
		ID:          "manual-1",
		OperationID: "op-1",
		Name:        "Manual fallback",
		StatusCode:  200,
		Priority:    2,
		Enabled:     true,
		Body:        `{"id": "{{.path.id}}", "name": "manual"}`,
		Headers:     map[string]string{"Content-Type": "application/json"},
	}
	if err := s.CreateResponseConfig(collCfg); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateResponseConfig(manualCfg); err != nil {
		t.Fatal(err)
	}
	engine.ReloadRoutes()

	req := httptest.NewRequest("GET", "/api/users/42", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "null" {
		t.Fatalf("expected the empty-but-matched collection response to render null, got: %s", w.Body.String())
	}
}

func TestServeHTTP_CollectionResponse_RespectsPriorityOverManual(t *testing.T) {
	engine, s, backend := setupCollectionTestEngine(t)
	setupUserSpecAndOperation(t, s)
	if _, err := backend.SeedInsert("users", map[string]any{"_id": "42", "id": "42", "name": "Alice"}); err != nil {
		t.Fatal(err)
	}

	// Manual response has higher priority (lower number) than the collection response.
	manualCfg := &models.ResponseConfig{
		ID:          "manual-1",
		OperationID: "op-1",
		Name:        "Manual first",
		StatusCode:  200,
		Priority:    1,
		Enabled:     true,
		Body:        `{"id": "{{.path.id}}", "name": "manual"}`,
		Headers:     map[string]string{"Content-Type": "application/json"},
	}
	collCfg := collectionResponseConfig("cr-1", 2)
	if err := s.CreateResponseConfig(manualCfg); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateResponseConfig(collCfg); err != nil {
		t.Fatal(err)
	}
	engine.ReloadRoutes()

	req := httptest.NewRequest("GET", "/api/users/42", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %s", w.Body.String())
	}
	if body["name"] != "manual" {
		t.Fatalf("expected the higher-priority manual response to win, got %#v", body)
	}
}

// erroringCollectionBackend always fails GetAll, simulating a backend outage.
type erroringCollectionBackend struct{}

func (erroringCollectionBackend) GetAll(string) ([]map[string]any, error) {
	return nil, fmt.Errorf("simulated backend outage")
}
func (erroringCollectionBackend) SeedInsert(string, map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("simulated backend outage")
}
func (erroringCollectionBackend) SeedClear(string) error {
	return fmt.Errorf("simulated backend outage")
}
func (erroringCollectionBackend) ListCollections() ([]string, error) {
	return nil, fmt.Errorf("simulated backend outage")
}
func (erroringCollectionBackend) DropCollection(string) error {
	return fmt.Errorf("simulated backend outage")
}

func TestServeHTTP_CollectionResponse_BackendErrorReturns500AndDoesNotFallThrough(t *testing.T) {
	s := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)
	engine := NewEngine(s, collector, tracingSvc)
	sessionManager := store.NewSessionManager(context.Background(), nil, config.SessionConfig{})
	engine.SetSessionManager(sessionManager, "X-Session-Id")
	engine.SetCollectionBackend(erroringCollectionBackend{})

	setupUserSpecAndOperation(t, s)
	collCfg := collectionResponseConfig("cr-1", 1)
	manualCfg := &models.ResponseConfig{
		ID:          "manual-1",
		OperationID: "op-1",
		Name:        "Manual fallback",
		StatusCode:  200,
		Priority:    2,
		Enabled:     true,
		Body:        `{"id": "{{.path.id}}", "name": "manual"}`,
		Headers:     map[string]string{"Content-Type": "application/json"},
	}
	if err := s.CreateResponseConfig(collCfg); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateResponseConfig(manualCfg); err != nil {
		t.Fatal(err)
	}
	engine.ReloadRoutes()

	req := httptest.NewRequest("GET", "/api/users/42", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on a collection backend error, got %d: %s", w.Code, w.Body.String())
	}
}
