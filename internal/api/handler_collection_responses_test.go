package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

const collRespTestSpecContent = `{
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
    },
    "/users": {
      "get": {
        "operationId": "listUsers",
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {
                "example": [{"id": "placeholder"}]
              }
            }
          }
        }
      }
    }
  }
}`

// setupCollectionResponseTestHandler wires a collection backend so
// h.collResponseSvc is non-nil, and seeds a spec + two operations.
func setupCollectionResponseTestHandler(t *testing.T) (*Handler, storage.Storage, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	s := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)
	proxyEngine := proxy.NewEngine(s, collector, tracingSvc)
	backend := store.NewMemoryCollectionBackend()

	handler := NewHandler(HandlerConfig{
		Store:             s,
		StatsCollector:    collector,
		TracingService:    tracingSvc,
		ProxyEngine:       proxyEngine,
		CollectionBackend: backend,
	})

	if err := s.CreateSpec(&models.Spec{ID: "spec-1", Content: collRespTestSpecContent}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateOperation(&models.Operation{ID: "op-user", SpecID: "spec-1", Method: "GET", Path: "/users/{id}"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateOperation(&models.Operation{ID: "op-users", SpecID: "spec-1", Method: "GET", Path: "/users"}); err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	return handler, s, r
}

func validCollectionResponsePayload() map[string]any {
	return map[string]any{
		"name":       "Users",
		"statusCode": 200,
		"kind":       "collection",
		"collectionResponse": map[string]any{
			"primary": map[string]any{
				"collectionName": "users",
				"filterRules": []map[string]any{
					{"targetPath": "_id", "value": map[string]any{"source": "path", "key": "id"}},
				},
			},
		},
	}
}

func TestCreateResponseConfig_Collection_Success(t *testing.T) {
	handler, _, r := setupCollectionResponseTestHandler(t)
	r.POST("/operations/:id/responses", handler.CreateResponseConfig)

	jsonBody, _ := json.Marshal(validCollectionResponsePayload())
	req := httptest.NewRequest("POST", "/operations/op-user/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var result models.ResponseConfig
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Kind != models.ResponseKindCollection {
		t.Fatalf("expected kind=collection, got %q", result.Kind)
	}
	if result.CollectionResponse == nil || result.CollectionResponse.Primary.CollectionName != "users" {
		t.Fatalf("collectionResponse not persisted: %#v", result.CollectionResponse)
	}
	if result.Body != "" {
		t.Fatalf("expected empty body, got %q", result.Body)
	}
}

func TestCreateResponseConfig_Collection_MissingConfig(t *testing.T) {
	handler, _, r := setupCollectionResponseTestHandler(t)
	r.POST("/operations/:id/responses", handler.CreateResponseConfig)

	body := map[string]any{"name": "Users", "statusCode": 200, "kind": "collection"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-user/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateResponseConfig_Manual_RejectsCollectionResponseField(t *testing.T) {
	handler, _, r := setupCollectionResponseTestHandler(t)
	r.POST("/operations/:id/responses", handler.CreateResponseConfig)

	payload := validCollectionResponsePayload()
	delete(payload, "kind") // defaults to manual
	jsonBody, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/operations/op-user/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateResponseConfig_Collection_InvalidFilterSource(t *testing.T) {
	handler, _, r := setupCollectionResponseTestHandler(t)
	r.POST("/operations/:id/responses", handler.CreateResponseConfig)

	payload := map[string]any{
		"name":       "Users",
		"statusCode": 200,
		"kind":       "collection",
		"collectionResponse": map[string]any{
			"primary": map[string]any{
				"collectionName": "users",
				"filterRules": []map[string]any{
					{"targetPath": "_id", "value": map[string]any{"source": "document", "key": "x"}},
				},
			},
		},
	}
	jsonBody, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/operations/op-user/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateResponseConfig_Collection_PrimarySourceRejectedForArrayRoot(t *testing.T) {
	handler, _, r := setupCollectionResponseTestHandler(t)
	r.POST("/operations/:id/responses", handler.CreateResponseConfig)

	payload := map[string]any{
		"name":       "Users",
		"statusCode": 200,
		"kind":       "collection",
		"collectionResponse": map[string]any{
			"primary": map[string]any{"collectionName": "users"},
			"additionalMappers": []map[string]any{
				{
					"outputKey":      "plan",
					"mode":           "find-one",
					"collectionName": "plans",
					"filterRules": []map[string]any{
						{"targetPath": "_id", "value": map[string]any{"source": "primary", "key": "planId"}},
					},
				},
			},
		},
	}
	jsonBody, _ := json.Marshal(payload)
	// /users (op-users) has an array-rooted 200 response.
	req := httptest.NewRequest("POST", "/operations/op-users/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateResponseConfig_CannotAddCollectionResponseToManual(t *testing.T) {
	handler, s, r := setupCollectionResponseTestHandler(t)
	if err := s.CreateResponseConfig(&models.ResponseConfig{ID: "resp-1", OperationID: "op-user", Name: "Manual", StatusCode: 200, Body: "{}"}); err != nil {
		t.Fatal(err)
	}
	r.PUT("/responses/:id", handler.UpdateResponseConfig)

	body := map[string]any{
		"collectionResponse": map[string]any{"primary": map[string]any{"collectionName": "users"}},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/responses/resp-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (kind is immutable), got %d: %s", w.Code, w.Body.String())
	}
}

func TestCloneResponseConfig_PreservesCollectionKind(t *testing.T) {
	handler, s, r := setupCollectionResponseTestHandler(t)
	src := &models.ResponseConfig{
		ID:          "resp-1",
		OperationID: "op-user",
		Name:        "Users",
		StatusCode:  200,
		Kind:        models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{CollectionName: "users"},
		},
	}
	if err := s.CreateResponseConfig(src); err != nil {
		t.Fatal(err)
	}
	r.POST("/responses/:id/clone", handler.CloneResponseConfig)

	body := map[string]any{"name": "Users Clone"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/responses/resp-1/clone", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var clone models.ResponseConfig
	if err := json.Unmarshal(w.Body.Bytes(), &clone); err != nil {
		t.Fatal(err)
	}
	if clone.Kind != models.ResponseKindCollection {
		t.Fatalf("expected cloned kind=collection, got %q", clone.Kind)
	}
	if clone.CollectionResponse == nil || clone.CollectionResponse.Primary.CollectionName != "users" {
		t.Fatalf("clone did not preserve collectionResponse: %#v", clone.CollectionResponse)
	}
}

func TestCreateResponseScriptBinding_RejectedForCollectionResponse(t *testing.T) {
	handler, s, r := setupCollectionResponseTestHandler(t)
	src := &models.ResponseConfig{
		ID:          "resp-1",
		OperationID: "op-user",
		Name:        "Users",
		StatusCode:  200,
		Kind:        models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{CollectionName: "users"},
		},
	}
	if err := s.CreateResponseConfig(src); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateScript(&models.Script{ID: "script-1", Name: "s", Source: "def main():\n  return {}"}); err != nil {
		t.Fatal(err)
	}
	r.POST("/operations/:id/responses/:respId/scripts", handler.CreateResponseScriptBinding)

	body := map[string]any{"scriptId": "script-1", "outputKey": "out", "enabled": true}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-user/responses/resp-1/scripts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateCollectionMapping_RejectedForCollectionResponse(t *testing.T) {
	handler, s, r := setupCollectionResponseTestHandler(t)
	src := &models.ResponseConfig{
		ID:          "resp-1",
		OperationID: "op-user",
		Name:        "Users",
		StatusCode:  200,
		Kind:        models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{CollectionName: "users"},
		},
	}
	if err := s.CreateResponseConfig(src); err != nil {
		t.Fatal(err)
	}
	r.POST("/operations/:id/responses/:respId/mappings", handler.CreateCollectionMapping)

	body := map[string]any{"collectionName": "orders", "operation": "find-one", "outputKey": "out"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-user/responses/resp-1/mappings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPreviewCollectionResponse(t *testing.T) {
	handler, _, r := setupCollectionResponseTestHandler(t)
	r.POST("/operations/:id/collection-responses/preview", handler.PreviewCollectionResponse)

	// Seed via the collection backend directly is not exposed here; instead
	// verify the no-match (empty collection) path renders a clean preview.
	payload := map[string]any{
		"statusCode": 200,
		"collectionResponse": map[string]any{
			"primary": map[string]any{
				"collectionName": "users",
				"filterRules": []map[string]any{
					{"targetPath": "_id", "value": map[string]any{"source": "path", "key": "id"}},
				},
			},
		},
		"request": map[string]any{"pathParams": map[string]any{"id": "42"}},
	}
	jsonBody, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/operations/op-user/collection-responses/preview", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result collectionResponsePreviewResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Matched {
		t.Fatal("expected no match against an empty collection")
	}
	if result.RootKind != models.RootKindObject {
		t.Fatalf("expected object root, got %q", result.RootKind)
	}
}
