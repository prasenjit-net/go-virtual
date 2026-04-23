package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
)

// ── ListOperations ────────────────────────────────────────────────────────────

func TestListOperations(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API 1"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users"})
	store.CreateOperation(&models.Operation{ID: "op-2", SpecID: "spec-1", Method: "POST", Path: "/users"})

	r.GET("/specs/:id/operations", handler.ListOperations)
	req := httptest.NewRequest("GET", "/specs/spec-1/operations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	var result []models.Operation
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(result))
	}
}

// ── GetOperation ──────────────────────────────────────────────────────────────

func TestGetOperation(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API 1"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users", Summary: "List users"})

	r.GET("/operations/:id", handler.GetOperation)
	req := httptest.NewRequest("GET", "/operations/op-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	var result models.Operation
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.ID != "op-1" {
		t.Errorf("Expected id 'op-1', got %q", result.ID)
	}
	if result.Summary != "List users" {
		t.Errorf("Expected summary 'List users', got %q", result.Summary)
	}
}

func TestGetOperation_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/operations/:id", handler.GetOperation)

	req := httptest.NewRequest("GET", "/operations/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// ── GetSignatureConfig ────────────────────────────────────────────────────────

func TestGetSignatureConfig_Default(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	r.GET("/operations/:id/signature", handler.GetSignatureConfig)

	req := httptest.NewRequest("GET", "/operations/op-1/signature", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetSignatureConfig_WithConfig(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateOperation(&models.Operation{
		ID:     "op-1",
		SpecID: "spec-1",
		SignatureConfig: &models.SignatureConfig{
			PathParams:  []string{"id"},
			IncludeBody: true,
		},
	})
	r.GET("/operations/:id/signature", handler.GetSignatureConfig)

	req := httptest.NewRequest("GET", "/operations/op-1/signature", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["signatureConfig"] == nil {
		t.Error("expected signatureConfig to be present")
	}
}

func TestGetSignatureConfig_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/operations/:id/signature", handler.GetSignatureConfig)

	req := httptest.NewRequest("GET", "/operations/missing/signature", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── UpdateSignatureConfig ─────────────────────────────────────────────────────

func TestUpdateSignatureConfig(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	r.PUT("/operations/:id/signature", handler.UpdateSignatureConfig)

	body := map[string]interface{}{
		"signatureConfig": map[string]interface{}{
			"pathParams":    []string{"id"},
			"queryParams":   []string{},
			"headers":       []string{"X-Tenant"},
			"includeBody":   false,
			"bodyJsonPaths": []string{},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/operations/op-1/signature", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	updated, _ := store.GetOperation("op-1")
	if updated.SignatureConfig == nil {
		t.Fatal("expected SignatureConfig to be set")
	}
	if len(updated.SignatureConfig.PathParams) != 1 || updated.SignatureConfig.PathParams[0] != "id" {
		t.Errorf("unexpected PathParams: %v", updated.SignatureConfig.PathParams)
	}
}

func TestUpdateSignatureConfig_ClearConfig(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateOperation(&models.Operation{
		ID:     "op-1",
		SpecID: "spec-1",
		SignatureConfig: &models.SignatureConfig{
			PathParams: []string{"id"},
		},
	})
	r.PUT("/operations/:id/signature", handler.UpdateSignatureConfig)

	body := map[string]interface{}{"signatureConfig": nil}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/operations/op-1/signature", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	updated, _ := store.GetOperation("op-1")
	if updated.SignatureConfig != nil {
		t.Error("expected SignatureConfig to be cleared")
	}
}

func TestUpdateSignatureConfig_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/operations/:id/signature", handler.UpdateSignatureConfig)

	body := map[string]interface{}{"signatureConfig": nil}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/operations/missing/signature", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateSignatureConfig_InvalidJSON(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	r.PUT("/operations/:id/signature", handler.UpdateSignatureConfig)

	req := httptest.NewRequest("PUT", "/operations/op-1/signature", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── ModePolicy ────────────────────────────────────────────────────────────────

func TestGetModePolicy_Default(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	r.GET("/operations/:id/mode-policy", handler.GetModePolicy)

	req := httptest.NewRequest("GET", "/operations/op-1/mode-policy", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]models.OperationModePolicy
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["modePolicy"].AI.Enabled {
		t.Fatal("expected AI disabled by default")
	}
	if result["modePolicy"].Proxy.Enabled {
		t.Fatal("expected proxy disabled by default")
	}
}

func TestUpdateModePolicy(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API", BackendURI: "http://backend"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	r.PUT("/operations/:id/mode-policy", handler.UpdateModePolicy)

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
	req := httptest.NewRequest("PUT", "/operations/op-1/mode-policy", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	updatedSpec, _ := store.GetSpec("spec-1")
	if !updatedSpec.ModePolicy.Configured {
		t.Fatal("expected mode policy to be marked configured")
	}
	if !updatedSpec.ModePolicy.Proxy.Enabled {
		t.Fatal("expected proxy enabled")
	}
	if len(updatedSpec.ModePolicy.Proxy.Conditions) != 1 {
		t.Fatalf("expected proxy condition saved, got %d", len(updatedSpec.ModePolicy.Proxy.Conditions))
	}
}

func TestUpdateModePolicy_RequiresBackend(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	r.PUT("/operations/:id/mode-policy", handler.UpdateModePolicy)

	body := map[string]any{
		"modePolicy": map[string]any{
			"ai":    map[string]any{"enabled": false, "conditions": []map[string]any{}},
			"proxy": map[string]any{"enabled": true, "conditions": []map[string]any{}},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/operations/op-1/mode-policy", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
