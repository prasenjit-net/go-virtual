package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
)

// ── ListSpecs ────────────────────────────────────────────────────────────────

func TestListSpecs_Empty(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/specs", handler.ListSpecs)

	req := httptest.NewRequest("GET", "/specs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	var result []interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 0 {
		t.Errorf("Expected empty array, got %d items", len(result))
	}
}

func TestListSpecs_WithSpecs(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API 1", Version: "1.0", Enabled: true})
	store.CreateSpec(&models.Spec{ID: "spec-2", Name: "API 2", Version: "2.0", Enabled: false})

	r.GET("/specs", handler.ListSpecs)
	req := httptest.NewRequest("GET", "/specs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	var result []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 2 {
		t.Errorf("Expected 2 specs, got %d", len(result))
	}
}

// ── CreateSpec ───────────────────────────────────────────────────────────────

func TestCreateSpec_ValidSpec(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/specs", handler.CreateSpec)

	specContent := `
openapi: "3.0.0"
info:
  title: Test API
  version: "1.0.0"
paths:
  /users:
    get:
      summary: List users
      responses:
        "200":
          description: Success
`
	body := map[string]string{"content": specContent, "basePath": "/api/v1"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/specs", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["id"] == nil || result["id"] == "" {
		t.Error("Expected id to be set")
	}
	if result["name"] != "Test API" {
		t.Errorf("Expected name 'Test API', got %v", result["name"])
	}
}

func TestCreateSpec_InvalidSpec(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/specs", handler.CreateSpec)

	body := map[string]string{"content": "invalid: yaml: content: here"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/specs", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestCreateSpec_InvalidJSON(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/specs", handler.CreateSpec)

	req := httptest.NewRequest("POST", "/specs", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ── GetSpec ──────────────────────────────────────────────────────────────────

func TestGetSpec_Exists(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API 1", Version: "1.0", Content: "spec content"})
	r.GET("/specs/:id", handler.GetSpec)

	req := httptest.NewRequest("GET", "/specs/spec-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	var result models.Spec
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.ID != "spec-1" {
		t.Errorf("Expected id 'spec-1', got %q", result.ID)
	}
}

func TestGetSpec_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/specs/:id", handler.GetSpec)

	req := httptest.NewRequest("GET", "/specs/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// ── UpdateSpec ───────────────────────────────────────────────────────────────

func TestUpdateSpec(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "Old Name", Version: "1.0", Enabled: true})
	r.PUT("/specs/:id", handler.UpdateSpec)

	update := map[string]interface{}{"name": "New Name", "enabled": false}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/specs/spec-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	updatedSpec, _ := store.GetSpec("spec-1")
	if updatedSpec.Name != "New Name" {
		t.Errorf("Expected name 'New Name', got %q", updatedSpec.Name)
	}
	if updatedSpec.Enabled != false {
		t.Error("Expected enabled to be false")
	}
}

func TestUpdateSpec_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/specs/:id", handler.UpdateSpec)

	update := map[string]interface{}{"name": "New Name"}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/specs/nonexistent", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestUpdateSpec_InvalidJSON(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API"})
	r.PUT("/specs/:id", handler.UpdateSpec)

	req := httptest.NewRequest("PUT", "/specs/spec-1", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateSpec_WithBasePath(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API", BasePath: "/v1"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Path: "/users", FullPath: "/v1/users"})
	r.PUT("/specs/:id", handler.UpdateSpec)

	update := map[string]interface{}{"basePath": "/v2"}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/specs/spec-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	op, _ := store.GetOperation("op-1")
	if op.FullPath != "/v2/users" {
		t.Errorf("expected FullPath '/v2/users', got %q", op.FullPath)
	}
}

func TestUpdateSpec_WithBackendAndProxyMode(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API", BackendURI: "http://be"})
	r.PUT("/specs/:id", handler.UpdateSpec)

	update := map[string]interface{}{"backendUri": "http://new-backend", "proxyMode": true}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/specs/spec-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	updated, _ := store.GetSpec("spec-1")
	if updated.BackendURI != "http://new-backend" {
		t.Errorf("expected backendUri updated, got %q", updated.BackendURI)
	}
	if !updated.ProxyMode {
		t.Error("expected ProxyMode to be enabled")
	}
}

func TestUpdateSpec_ProxyModeWithoutBackendURI(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API"}) // no BackendURI
	r.PUT("/specs/:id", handler.UpdateSpec)

	update := map[string]interface{}{"proxyMode": true}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/specs/spec-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when enabling proxyMode without backendUri, got %d", w.Code)
	}
}

func TestUpdateSpec_AIModeWithoutOpenAIKey(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API"})
	r.PUT("/specs/:id", handler.UpdateSpec)

	update := map[string]interface{}{"mode": "ai"}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/specs/spec-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when enabling ai mode without OpenAI key, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("OpenAI API key")) {
		t.Fatalf("expected OpenAI API key error, got %s", w.Body.String())
	}
}

func TestSetSpecMode(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API", BackendURI: "http://backend"})
	r.PUT("/specs/:id/mode", handler.SetSpecMode)

	update := map[string]interface{}{"mode": "proxy"}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/specs/spec-1/mode", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	updated, _ := store.GetSpec("spec-1")
	if updated.Mode != models.SpecModeProxy || !updated.ProxyMode || !updated.ModePolicy.Proxy.Enabled {
		t.Fatalf("expected proxy mode enabled, got mode=%q proxyMode=%v policy=%+v", updated.Mode, updated.ProxyMode, updated.ModePolicy)
	}
}

func TestSetSpecMode_InvalidJSON(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API"})
	r.PUT("/specs/:id/mode", handler.SetSpecMode)

	req := httptest.NewRequest("PUT", "/specs/spec-1/mode", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSetSpecMode_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/specs/:id/mode", handler.SetSpecMode)

	req := httptest.NewRequest("PUT", "/specs/missing/mode", bytes.NewReader([]byte(`{"mode":"standard"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetSpecModePolicy(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{
		ID:   "spec-1",
		Name: "API",
		ModePolicy: models.ModePolicy{
			Configured: true,
			AI:         models.ConditionalModeConfig{Enabled: true},
			Proxy:      models.ConditionalModeConfig{Enabled: false},
		},
	})
	r.GET("/specs/:id/mode-policy", handler.GetSpecModePolicy)

	req := httptest.NewRequest("GET", "/specs/spec-1/mode-policy", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]models.ModePolicy
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !result["modePolicy"].AI.Enabled || result["modePolicy"].Proxy.Enabled {
		t.Fatalf("unexpected mode policy: %+v", result["modePolicy"])
	}
}

func TestGetSpecModePolicy_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/specs/:id/mode-policy", handler.GetSpecModePolicy)

	req := httptest.NewRequest("GET", "/specs/missing/mode-policy", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateSpecModePolicy(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API", BackendURI: "http://backend"})
	r.PUT("/specs/:id/mode-policy", handler.UpdateSpecModePolicy)

	body := map[string]any{
		"modePolicy": map[string]any{
			"ai": map[string]any{
				"enabled": false,
				"conditions": []map[string]any{
					{"source": "header", "key": "x-env", "operator": "eq", "value": "test"},
				},
			},
			"proxy": map[string]any{
				"enabled": true,
				"conditions": []map[string]any{
					{"source": "query", "key": "source", "operator": "eq", "value": "upstream"},
				},
			},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/specs/spec-1/mode-policy", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	updatedSpec, _ := store.GetSpec("spec-1")
	if !updatedSpec.ModePolicy.Configured || !updatedSpec.ModePolicy.Proxy.Enabled {
		t.Fatalf("expected updated spec mode policy, got %+v", updatedSpec.ModePolicy)
	}
	if len(updatedSpec.ModePolicy.Proxy.Conditions) != 1 || len(updatedSpec.ModePolicy.AI.Conditions) != 1 {
		t.Fatalf("expected saved conditions, got %+v", updatedSpec.ModePolicy)
	}
}

func TestUpdateSpecModePolicy_InvalidJSON(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API"})
	r.PUT("/specs/:id/mode-policy", handler.UpdateSpecModePolicy)

	req := httptest.NewRequest("PUT", "/specs/spec-1/mode-policy", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateSpecModePolicy_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/specs/:id/mode-policy", handler.UpdateSpecModePolicy)

	req := httptest.NewRequest("PUT", "/specs/missing/mode-policy", bytes.NewReader([]byte(`{"modePolicy":{"ai":{"enabled":false,"conditions":[]},"proxy":{"enabled":false,"conditions":[]}}}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateSpecModePolicy_RejectsSignatureConditions(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API", BackendURI: "http://backend"})
	r.PUT("/specs/:id/mode-policy", handler.UpdateSpecModePolicy)

	body := map[string]any{
		"modePolicy": map[string]any{
			"ai": map[string]any{
				"enabled": false,
				"conditions": []map[string]any{
					{"source": "signature", "operator": "eq", "value": "abc123"},
				},
			},
			"proxy": map[string]any{
				"enabled":    true,
				"conditions": []map[string]any{},
			},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/specs/spec-1/mode-policy", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("signature conditions are only supported on operation responses")) {
		t.Fatalf("expected signature-condition error, got %s", w.Body.String())
	}
}

// ── DeleteSpec ───────────────────────────────────────────────────────────────

func TestDeleteSpec(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API 1"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users"})
	store.CreateResponseConfig(&models.ResponseConfig{ID: "config-1", OperationID: "op-1", Name: "Default"})

	r.DELETE("/specs/:id", handler.DeleteSpec)
	req := httptest.NewRequest("DELETE", "/specs/spec-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if _, err := store.GetSpec("spec-1"); err == nil {
		t.Error("Expected spec to be deleted")
	}
	ops, _ := store.GetOperationsBySpec("spec-1")
	if len(ops) != 0 {
		t.Errorf("Expected operations to be deleted, got %d", len(ops))
	}
}

func TestDeleteSpec_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.DELETE("/specs/:id", handler.DeleteSpec)

	req := httptest.NewRequest("DELETE", "/specs/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// ── EnableSpec / DisableSpec ─────────────────────────────────────────────────

func TestEnableSpec(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API 1", Enabled: false})
	r.PUT("/specs/:id/enable", handler.EnableSpec)

	req := httptest.NewRequest("PUT", "/specs/spec-1/enable", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	updated, _ := store.GetSpec("spec-1")
	if !updated.Enabled {
		t.Error("Expected spec to be enabled")
	}
}

func TestEnableSpec_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/specs/:id/enable", handler.EnableSpec)

	req := httptest.NewRequest("PUT", "/specs/nonexistent/enable", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDisableSpec(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API 1", Enabled: true})
	r.PUT("/specs/:id/disable", handler.DisableSpec)

	req := httptest.NewRequest("PUT", "/specs/spec-1/disable", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	updated, _ := store.GetSpec("spec-1")
	if updated.Enabled {
		t.Error("Expected spec to be disabled")
	}
}

func TestDisableSpec_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/specs/:id/disable", handler.DisableSpec)

	req := httptest.NewRequest("PUT", "/specs/nonexistent/disable", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── ToggleTracing ─────────────────────────────────────────────────────────────

func TestToggleTracing(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API 1", Tracing: false})
	r.PUT("/specs/:id/tracing", handler.ToggleTracing)

	body := map[string]bool{"enabled": true}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/specs/spec-1/tracing", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	updated, _ := store.GetSpec("spec-1")
	if !updated.Tracing {
		t.Error("Expected tracing to be enabled")
	}
}

func TestToggleTracing_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/specs/:id/tracing", handler.ToggleTracing)

	req := httptest.NewRequest("PUT", "/specs/nonexistent/tracing", bytes.NewReader([]byte(`{"enabled":true}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestToggleTracing_NoBody(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API", Tracing: true})
	r.PUT("/specs/:id/tracing", handler.ToggleTracing)

	// Invalid JSON body → toggles from true → false
	req := httptest.NewRequest("PUT", "/specs/spec-1/tracing", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	updated, _ := store.GetSpec("spec-1")
	if updated.Tracing {
		t.Error("expected tracing to be toggled off")
	}
}

// ── ToggleExampleFallback ────────────────────────────────────────────────────

func TestToggleExampleFallback(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API 1", UseExampleFallback: true})
	r.PUT("/specs/:id/example-fallback", handler.ToggleExampleFallback)

	body := map[string]bool{"enabled": false}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/specs/spec-1/example-fallback", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	updated, _ := store.GetSpec("spec-1")
	if updated.UseExampleFallback {
		t.Error("Expected example fallback to be disabled")
	}
}

func TestToggleExampleFallback_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/specs/:id/example-fallback", handler.ToggleExampleFallback)

	req := httptest.NewRequest("PUT", "/specs/nonexistent/example-fallback", bytes.NewReader([]byte(`{"enabled":false}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestToggleExampleFallback_NoBody(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API", UseExampleFallback: false})
	r.PUT("/specs/:id/example-fallback", handler.ToggleExampleFallback)

	// Invalid body → toggles false → true
	req := httptest.NewRequest("PUT", "/specs/spec-1/example-fallback", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	updated, _ := store.GetSpec("spec-1")
	if !updated.UseExampleFallback {
		t.Error("expected UseExampleFallback to be toggled on")
	}
}

// ── SetBackendURI ─────────────────────────────────────────────────────────────

func TestSetBackendURI(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API"})
	r.PUT("/specs/:id/backend", handler.SetBackendURI)

	body := map[string]string{"backendUri": "http://my-backend:9090"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/specs/spec-1/backend", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	updated, _ := store.GetSpec("spec-1")
	if updated.BackendURI != "http://my-backend:9090" {
		t.Errorf("expected backendUri to be set, got %q", updated.BackendURI)
	}
}

func TestSetBackendURI_ClearDisablesProxyMode(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API", BackendURI: "http://old", ProxyMode: true})
	r.PUT("/specs/:id/backend", handler.SetBackendURI)

	body := map[string]string{"backendUri": ""} // clear
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/specs/spec-1/backend", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	updated, _ := store.GetSpec("spec-1")
	if updated.ProxyMode {
		t.Error("expected ProxyMode to be disabled when backendUri is cleared")
	}
}

func TestSetBackendURI_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/specs/:id/backend", handler.SetBackendURI)

	body := map[string]string{"backendUri": "http://x"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/specs/missing/backend", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSetBackendURI_InvalidJSON(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API"})
	r.PUT("/specs/:id/backend", handler.SetBackendURI)

	req := httptest.NewRequest("PUT", "/specs/spec-1/backend", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── ToggleProxyMode ──────────────────────────────────────────────────────────

func TestToggleProxyMode_Enable(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API", BackendURI: "http://backend"})
	r.PUT("/specs/:id/proxy-mode", handler.ToggleProxyMode)

	body := map[string]bool{"enabled": true}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/specs/spec-1/proxy-mode", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	updated, _ := store.GetSpec("spec-1")
	if !updated.ProxyMode {
		t.Error("expected ProxyMode to be enabled")
	}
}

func TestToggleProxyMode_EnableWithoutBackendURI(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API"}) // no BackendURI
	r.PUT("/specs/:id/proxy-mode", handler.ToggleProxyMode)

	body := map[string]bool{"enabled": true}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/specs/spec-1/proxy-mode", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestToggleProxyMode_Toggle(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API", BackendURI: "http://be", ProxyMode: false})
	r.PUT("/specs/:id/proxy-mode", handler.ToggleProxyMode)

	// Invalid JSON → toggles
	req := httptest.NewRequest("PUT", "/specs/spec-1/proxy-mode", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	updated, _ := store.GetSpec("spec-1")
	if !updated.ProxyMode {
		t.Error("expected ProxyMode to be toggled on")
	}
}

func TestToggleProxyMode_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/specs/:id/proxy-mode", handler.ToggleProxyMode)

	req := httptest.NewRequest("PUT", "/specs/missing/proxy-mode", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── Spec tags ────────────────────────────────────────────────────────────────

func TestGetSpecTags_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/specs/:id/tags", handler.GetSpecTags)

	req := httptest.NewRequest("GET", "/specs/nonexistent/tags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateSpecTags_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/specs/:id/tags", handler.UpdateSpecTags)

	body := []byte(`{"tags":[]}`)
	req := httptest.NewRequest("PUT", "/specs/nonexistent/tags", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateSpecTags_InvalidJSON(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API"})
	r.PUT("/specs/:id/tags", handler.UpdateSpecTags)

	req := httptest.NewRequest("PUT", "/specs/spec-1/tags", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateSpec_DoesNotExposeAIScenarios(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/specs", handler.CreateSpec)

	specContent := `
openapi: "3.0.0"
info:
  title: Scenario API
  version: "1.0.0"
paths:
  /users:
    get:
      responses:
        "200":
          description: Success
`
	req := httptest.NewRequest("POST", "/specs", bytes.NewReader([]byte(`{"content":`+strconv.Quote(specContent)+`,"basePath":"/"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, exists := created["aiScenarios"]; exists {
		t.Fatal("expected spec response to omit AI scenarios")
	}
}

func TestListAIScenarios(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/ai-scenarios", handler.ListAIScenarios)

	req := httptest.NewRequest("GET", "/ai-scenarios", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Scenarios []models.AIScenario `json:"scenarios"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Scenarios) != 3 {
		t.Fatalf("expected 3 scenarios, got %d", len(resp.Scenarios))
	}
}

func TestCreateUpdateDeleteAIScenario(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/ai-scenarios", handler.CreateAIScenario)
	r.PUT("/ai-scenarios/:scenarioId", handler.UpdateAIScenario)
	r.DELETE("/ai-scenarios/:scenarioId", handler.DeleteAIScenario)

	createReq := httptest.NewRequest("POST", "/ai-scenarios", bytes.NewReader([]byte(`{"scenario":{"name":"unauthorized","responseKind":"error","statusCode":401,"instructions":"Return auth error","enabled":true}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createW.Code, createW.Body.String())
	}

	var created struct {
		Scenario models.AIScenario `json:"scenario"`
	}
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if created.Scenario.Name != "unauthorized" {
		t.Fatalf("unexpected scenario name %q", created.Scenario.Name)
	}

	updateReq := httptest.NewRequest("PUT", "/ai-scenarios/"+created.Scenario.ID, bytes.NewReader([]byte(`{"scenario":{"name":"unauthorized","responseKind":"error","statusCode":403,"count":2,"enabled":false}}`)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	r.ServeHTTP(updateW, updateReq)
	if updateW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", updateW.Code, updateW.Body.String())
	}

	deleteReq := httptest.NewRequest("DELETE", "/ai-scenarios/"+created.Scenario.ID, nil)
	deleteW := httptest.NewRecorder()
	r.ServeHTTP(deleteW, deleteReq)
	if deleteW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", deleteW.Code, deleteW.Body.String())
	}
}

func TestCreateAIScenario_RejectsDuplicateName(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/ai-scenarios", handler.CreateAIScenario)

	req := httptest.NewRequest("POST", "/ai-scenarios", bytes.NewReader([]byte(`{"scenario":{"name":"success","responseKind":"success","enabled":true}}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeleteAIScenario_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.DELETE("/ai-scenarios/:scenarioId", handler.DeleteAIScenario)

	req := httptest.NewRequest("DELETE", "/ai-scenarios/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ── Private helpers ──────────────────────────────────────────────────────────

func TestEnsureTagExists(t *testing.T) {
	handler, store, _ := setupTestHandler(t)

	if err := handler.ensureTagExists(models.DefaultTagName); err != nil {
		t.Fatalf("expected default tag to be valid")
	}
	if err := store.CreateTag(&models.Tag{Name: "blue"}); err != nil {
		t.Fatalf("CreateTag error: %v", err)
	}
	if err := handler.ensureTagExists("blue"); err != nil {
		t.Fatalf("expected tag to exist")
	}
	if err := handler.ensureTagExists("missing"); err == nil {
		t.Fatalf("expected error for missing tag")
	}
}
