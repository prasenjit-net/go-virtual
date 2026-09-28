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

func TestServeHTTP_MainUpdateAndAdditionalMutation(t *testing.T) {
	engine, s, backend := setupCollectionTestEngine(t)
	setupUserSpecAndOperation(t, s)
	spec, _ := s.GetSpec("spec-1")
	spec.Tracing = true
	s.UpdateSpec(spec)
	backend.SeedInsert("users", map[string]any{"_id": "42", "id": "42", "name": "Alice"})
	cfg := collectionResponseConfig("update", 1)
	cfg.CollectionResponse.Primary.Mode = models.ColOpUpdate
	cfg.CollectionResponse.Primary.FilterRules = append(cfg.CollectionResponse.Primary.FilterRules, models.CollectionFilter{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: json.RawMessage(`"Alice"`)}})
	cfg.CollectionResponse.Primary.DataRules = []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: json.RawMessage(`"Updated"`)}}}
	cfg.CollectionResponse.AdditionalMappers = []models.NamedQuery{{OutputKey: "audit", Mode: models.ColOpInsert, CollectionQuery: models.CollectionQuery{CollectionName: "audit", DataRules: []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourcePrimary, Key: "name"}}}}}}
	if err := s.CreateResponseConfig(cfg); err != nil {
		t.Fatal(err)
	}
	fallback := &models.ResponseConfig{ID: "fallback", OperationID: "op-1", Name: "fallback", StatusCode: 200, Priority: 2, Enabled: true, Body: `{"name":"fallback"}`}
	s.CreateResponseConfig(fallback)
	engine.ReloadRoutes()
	first := httptest.NewRecorder()
	engine.ServeHTTP(first, httptest.NewRequest("GET", "/api/users/42", nil))
	if first.Code != 200 {
		t.Fatalf("first request: %d %s", first.Code, first.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["name"] != "Updated" {
		t.Fatalf("pre-update body rendered: %v", body)
	}
	sessionID := first.Header().Get("X-Session-Id")
	if sessionID == "" {
		t.Fatal("missing session header")
	}
	traces := engine.tracingService.GetTraces(&models.TraceFilter{})
	if len(traces) != 1 || traces[0].CollectionResponseRender.PrimaryMapper.RecordCount != 1 || len(traces[0].CollectionResponseRender.AdditionalMappers) != 1 {
		t.Fatalf("missing execution traces: %+v", traces)
	}
	nextReq := httptest.NewRequest("GET", "/api/users/42", nil)
	nextReq.Header.Set("X-Session-Id", sessionID)
	next := httptest.NewRecorder()
	engine.ServeHTTP(next, nextReq)
	if next.Body.String() != `{"name":"fallback"}` {
		t.Fatalf("updated session should no longer match original query: %s", next.Body.String())
	}
	isolated := httptest.NewRecorder()
	engine.ServeHTTP(isolated, httptest.NewRequest("GET", "/api/users/42", nil))
	if isolated.Code != 200 || isolated.Body.String() != first.Body.String() {
		t.Fatal("another session did not retain base state")
	}
}

func TestServeHTTP_UpdateFailurePreservesTraceAndSession(t *testing.T) {
	engine, s, backend := setupCollectionTestEngine(t)
	setupUserSpecAndOperation(t, s)
	spec, _ := s.GetSpec("spec-1")
	spec.Tracing = true
	s.UpdateSpec(spec)
	backend.SeedInsert("users", map[string]any{"_id": "42", "id": "42", "name": "Alice"})
	cfg := collectionResponseConfig("update", 1)
	cfg.CollectionResponse.Primary.Mode = models.ColOpUpdate
	cfg.CollectionResponse.Primary.DataRules = []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: json.RawMessage(`"Updated"`)}}}
	cfg.CollectionResponse.AdditionalMappers = []models.NamedQuery{{OutputKey: "audit", Mode: models.ColOpInsert, CollectionQuery: models.CollectionQuery{CollectionName: "audit", DataRules: []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceBody, Key: "missing"}}}}}}
	s.CreateResponseConfig(cfg)
	s.CreateResponseConfig(&models.ResponseConfig{ID: "fallback", OperationID: "op-1", Name: "fallback", StatusCode: 200, Priority: 2, Enabled: true, Body: `{"fallback":true}`})
	engine.ReloadRoutes()
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest("GET", "/api/users/42", nil))
	if w.Code != 500 || w.Header().Get("X-Session-Id") == "" || !json.Valid(w.Body.Bytes()) {
		t.Fatalf("failure response: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	traces := engine.tracingService.GetTraces(&models.TraceFilter{})
	if len(traces) != 1 {
		t.Fatal("missing trace")
	}
	if traces[0].MatchedConfigID != cfg.ID || traces[0].MatchedConfig != cfg.Name || traces[0].Session == nil || traces[0].Session.ID != w.Header().Get("X-Session-Id") || traces[0].Response.Body != w.Body.String() {
		t.Fatalf("failure metadata lost: %+v", traces[0])
	}
	render := traces[0].CollectionResponseRender
	if render != nil && render.Error == "" {
		t.Fatal("execution error missing from render trace")
	}
	if render == nil || render.PrimaryMapper.RecordCount != 1 || len(render.AdditionalMappers) != 1 || render.AdditionalMappers[0].Error == "" {
		t.Fatalf("partial diagnostics lost: %+v", render)
	}
}

func TestServeHTTP_InsertUpsertConditionOnlySelection(t *testing.T) {
	for _, mode := range []models.CollectionOpType{models.ColOpInsert, models.ColOpUpsert} {
		for _, scenario := range []string{"condition-fails", "condition-passes", "unconditional", "higher-priority"} {
			t.Run(string(mode)+"/"+scenario, func(t *testing.T) {
				engine, s, _ := setupCollectionTestEngine(t)
				setupUserSpecAndOperation(t, s)
				spec, _ := s.GetSpec("spec-1")
				spec.Tracing = true
				s.UpdateSpec(spec)
				cfg := collectionResponseConfig("write", 1)
				cfg.CollectionResponse.Primary.Mode = mode
				if mode == models.ColOpInsert {
					cfg.CollectionResponse.Primary.FilterRules = nil
				}
				cfg.CollectionResponse.Primary.DataRules = []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: json.RawMessage(`"Created"`)}}}
				if scenario != "unconditional" {
					id := "42"
					if scenario == "condition-fails" {
						id = "999"
					}
					cfg.Conditions = []models.Condition{{Source: "path", Key: "id", Operator: "eq", Value: id}}
				}
				if err := s.CreateResponseConfig(cfg); err != nil {
					t.Fatal(err)
				}
				priority := 2
				if scenario == "higher-priority" {
					priority = 0
				}
				if err := s.CreateResponseConfig(&models.ResponseConfig{ID: "fallback", OperationID: "op-1", Name: "fallback", StatusCode: 200, Priority: priority, Enabled: true, Body: `{"name":"fallback"}`}); err != nil {
					t.Fatal(err)
				}
				engine.ReloadRoutes()
				rec := httptest.NewRecorder()
				engine.ServeHTTP(rec, httptest.NewRequest("GET", "/api/users/42", nil))
				if rec.Code != 200 {
					t.Fatalf("response: %d %s", rec.Code, rec.Body.String())
				}
				var body map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				traces := engine.tracingService.GetTraces(&models.TraceFilter{})
				if len(traces) != 1 {
					t.Fatalf("traces: %v", traces)
				}
				if scenario == "condition-fails" || scenario == "higher-priority" {
					if body["name"] != "fallback" || traces[0].CollectionResponseRender != nil {
						t.Fatalf("unselected write executed: %v", body)
					}
				} else {
					if body["name"] != "Created" || body["_id"] != nil || traces[0].CollectionResponseRender.PrimaryMapper.RecordCount != 1 {
						t.Fatalf("write not rendered: %v", body)
					}
				}
			})
		}
	}
}

func TestCollectionTraceRetainsSelectionAcrossOutcomes(t *testing.T) {
	for _, scenario := range []string{"read", "example-fallback", "not-found", "condition-fails", "selection-error", "insert"} {
		t.Run(scenario, func(t *testing.T) {
			engine, s, backend := setupCollectionTestEngine(t)
			setupUserSpecAndOperation(t, s)
			spec, _ := s.GetSpec("spec-1")
			spec.Tracing = true
			spec.UseExampleFallback = scenario == "example-fallback"
			s.UpdateSpec(spec)
			op, _ := s.GetOperation("op-1")
			op.ExampleResponse = &models.ExampleResponse{StatusCode: 200, Body: `{"name":"example"}`}
			s.CreateOperation(op)
			cfg := collectionResponseConfig("trace-config", 1)
			switch scenario {
			case "read":
				backend.SeedInsert("users", map[string]any{"_id": "42", "id": "42", "name": "Alice"})
			case "condition-fails":
				cfg.Conditions = []models.Condition{{Source: "path", Key: "id", Operator: "eq", Value: "999"}}
			case "selection-error":
				cfg.CollectionResponse.Primary.Mode = models.ColOpUpdate
				cfg.CollectionResponse.Primary.FilterRules[0].Value.Key = "missing"
				cfg.CollectionResponse.Primary.DataRules = []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: json.RawMessage(`"Updated"`)}}}
			case "insert":
				cfg.CollectionResponse.Primary.Mode = models.ColOpInsert
				cfg.CollectionResponse.Primary.FilterRules = nil
				cfg.CollectionResponse.Primary.DataRules = []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: json.RawMessage(`"Created"`)}}}
			}
			s.CreateResponseConfig(cfg)
			engine.ReloadRoutes()
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, httptest.NewRequest("GET", "/api/users/42", nil))
			traces := engine.tracingService.GetTraces(&models.TraceFilter{})
			if len(traces) != 1 {
				t.Fatalf("trace missing: %d %s", w.Code, w.Body.String())
			}
			trace := traces[0]
			if trace.Response.Body != w.Body.String() || trace.Response.StatusCode != w.Code {
				t.Fatal("trace differs from wire response")
			}
			if len(trace.CollectionResponseAttempts) != 1 {
				t.Fatalf("attempt missing: %+v", trace)
			}
			attempt := trace.CollectionResponseAttempts[0]
			if attempt.ResponseConfigID != cfg.ID || attempt.Reason == "" {
				t.Fatalf("attempt: %+v", attempt)
			}
			switch scenario {
			case "read":
				primary := trace.CollectionResponseRender.PrimaryMapper
				if !attempt.QueryExecuted || primary == nil || primary.Operation != models.ColOpFindOne || primary.RecordCount != 1 || primary.Filter["_id"] != "42" || primary.Result.(map[string]any)["name"] != "Alice" {
					t.Fatalf("read trace: %+v", primary)
				}
			case "insert":
				primary := trace.CollectionResponseRender.PrimaryMapper
				if attempt.QueryExecuted || !attempt.Matched || attempt.Mode != "insert" || primary.Data["name"] != "Created" || primary.Result.(map[string]any)["name"] != "Created" {
					t.Fatalf("insert trace: %+v %+v", attempt, primary)
				}
			case "selection-error":
				if w.Code != 500 || attempt.Error == "" || attempt.Matched || attempt.QueryExecuted {
					t.Fatalf("failed attempt: %+v", attempt)
				}
			case "condition-fails":
				if attempt.QueryExecuted || attempt.Matched {
					t.Fatalf("conditions: %+v", attempt)
				}
			default:
				if !attempt.QueryExecuted || attempt.Matched || attempt.Filter["_id"] != "42" {
					t.Fatalf("fallback attempt: %+v", attempt)
				}
			}
		})
	}
}
