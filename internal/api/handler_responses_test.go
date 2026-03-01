package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
)

// ── ListResponseConfigs ───────────────────────────────────────────────────────

func TestListResponseConfigs(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users"})
	store.CreateResponseConfig(&models.ResponseConfig{ID: "config-1", OperationID: "op-1", Name: "Success", Priority: 1})
	store.CreateResponseConfig(&models.ResponseConfig{ID: "config-2", OperationID: "op-1", Name: "Error", Priority: 2})

	r.GET("/operations/:id/responses", handler.ListResponseConfigs)
	req := httptest.NewRequest("GET", "/operations/op-1/responses", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	var result []models.ResponseConfig
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 2 {
		t.Errorf("Expected 2 configs, got %d", len(result))
	}
}

// ── CreateResponseConfig ──────────────────────────────────────────────────────

func TestCreateResponseConfig(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users"})

	r.POST("/operations/:id/responses", handler.CreateResponseConfig)

	body := map[string]interface{}{
		"name":       "Success Response",
		"statusCode": 200,
		"body":       `{"message": "OK"}`,
		"headers":    map[string]string{"Content-Type": "application/json"},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-1/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
	var result models.ResponseConfig
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Name != "Success Response" {
		t.Errorf("Expected name 'Success Response', got %q", result.Name)
	}
	if result.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", result.StatusCode)
	}
}

func TestCreateResponseConfig_OperationNotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/operations/:id/responses", handler.CreateResponseConfig)

	body := map[string]interface{}{"name": "Success", "statusCode": 200}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/nonexistent/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestCreateResponseConfig_InvalidJSON(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateSpec(&models.Spec{ID: "spec-1"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	r.POST("/operations/:id/responses", handler.CreateResponseConfig)

	req := httptest.NewRequest("POST", "/operations/op-1/responses", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateResponseConfig_WithTag(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateSpec(&models.Spec{ID: "spec-1"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	store.CreateTag(&models.Tag{Name: "blue"})
	r.POST("/operations/:id/responses", handler.CreateResponseConfig)

	body := map[string]interface{}{"name": "Tagged", "statusCode": 200, "tag": "blue"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-1/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateResponseConfig_InvalidTag(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateSpec(&models.Spec{ID: "spec-1"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	r.POST("/operations/:id/responses", handler.CreateResponseConfig)

	body := map[string]interface{}{"name": "Bad Tag", "statusCode": 200, "tag": "nonexistent-tag"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-1/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ── GetResponseConfig ─────────────────────────────────────────────────────────

func TestGetResponseConfig(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateResponseConfig(&models.ResponseConfig{ID: "config-1", OperationID: "op-1", Name: "Default", StatusCode: 200})
	r.GET("/responses/:id", handler.GetResponseConfig)

	req := httptest.NewRequest("GET", "/responses/config-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	var result models.ResponseConfig
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.ID != "config-1" {
		t.Errorf("Expected id 'config-1', got %q", result.ID)
	}
}

func TestGetResponseConfig_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/responses/:id", handler.GetResponseConfig)

	req := httptest.NewRequest("GET", "/responses/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── UpdateResponseConfig ──────────────────────────────────────────────────────

func TestUpdateResponseConfig(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	store.CreateResponseConfig(&models.ResponseConfig{ID: "config-1", OperationID: "op-1", Name: "Old Name", StatusCode: 200})

	r.PUT("/responses/:id", handler.UpdateResponseConfig)

	update := map[string]interface{}{"name": "New Name", "statusCode": 201, "body": `{"updated": true}`}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/responses/config-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	updated, _ := store.GetResponseConfig("config-1")
	if updated.Name != "New Name" {
		t.Errorf("Expected name 'New Name', got %q", updated.Name)
	}
	if updated.StatusCode != 201 {
		t.Errorf("Expected status code 201, got %d", updated.StatusCode)
	}
}

func TestUpdateResponseConfig_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/responses/:id", handler.UpdateResponseConfig)

	update := map[string]interface{}{"name": "New"}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/responses/nonexistent", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateResponseConfig_InvalidJSON(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateResponseConfig(&models.ResponseConfig{ID: "cfg-1", OperationID: "op-1", StatusCode: 200})
	r.PUT("/responses/:id", handler.UpdateResponseConfig)

	req := httptest.NewRequest("PUT", "/responses/cfg-1", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateResponseConfig_AllFields(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateResponseConfig(&models.ResponseConfig{ID: "cfg-1", OperationID: "op-1", StatusCode: 200})
	r.PUT("/responses/:id", handler.UpdateResponseConfig)

	conditions := []models.Condition{{Source: "query", Key: "env", Operator: "eq", Value: "prod"}}
	update := map[string]interface{}{
		"description": "updated desc",
		"priority":    3,
		"delay":       50,
		"enabled":     true,
		"statusCode":  201,
		"body":        `{"ok":true}`,
		"headers":     map[string]string{"X-Custom": "value"},
		"conditions":  conditions,
	}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/responses/cfg-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	cfg, _ := store.GetResponseConfig("cfg-1")
	if cfg.Description != "updated desc" {
		t.Errorf("expected description 'updated desc', got %q", cfg.Description)
	}
	if cfg.Priority != 3 {
		t.Errorf("expected priority 3, got %d", cfg.Priority)
	}
	if cfg.Delay != 50 {
		t.Errorf("expected delay 50, got %d", cfg.Delay)
	}
	if !cfg.Enabled {
		t.Error("expected enabled to be true")
	}
}

func TestUpdateResponseConfig_WithTag(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateTag(&models.Tag{Name: "blue"})
	store.CreateResponseConfig(&models.ResponseConfig{ID: "cfg-1", OperationID: "op-1", StatusCode: 200})
	r.PUT("/responses/:id", handler.UpdateResponseConfig)

	update := map[string]interface{}{"tag": "blue"}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/responses/cfg-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	cfg, _ := store.GetResponseConfig("cfg-1")
	if cfg.Tag != "blue" {
		t.Errorf("expected tag 'blue', got %q", cfg.Tag)
	}
}

func TestUpdateResponseConfig_WithInvalidTag(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateResponseConfig(&models.ResponseConfig{ID: "cfg-1", OperationID: "op-1", StatusCode: 200})
	r.PUT("/responses/:id", handler.UpdateResponseConfig)

	update := map[string]interface{}{"tag": "nonexistent-tag"}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/responses/cfg-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateResponseConfig_EmptyTag(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateResponseConfig(&models.ResponseConfig{ID: "cfg-1", OperationID: "op-1", StatusCode: 200, Tag: "blue"})
	r.PUT("/responses/:id", handler.UpdateResponseConfig)

	update := map[string]interface{}{"tag": ""} // empty → default
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/responses/cfg-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	cfg, _ := store.GetResponseConfig("cfg-1")
	if cfg.Tag != models.DefaultTagName {
		t.Errorf("expected tag to be default, got %q", cfg.Tag)
	}
}

// ── DeleteResponseConfig ──────────────────────────────────────────────────────

func TestDeleteResponseConfig(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	store.CreateResponseConfig(&models.ResponseConfig{ID: "config-1", OperationID: "op-1", Name: "Default"})

	r.DELETE("/responses/:id", handler.DeleteResponseConfig)
	req := httptest.NewRequest("DELETE", "/responses/config-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if _, err := store.GetResponseConfig("config-1"); err == nil {
		t.Error("Expected config to be deleted")
	}
}

func TestDeleteResponseConfig_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.DELETE("/responses/:id", handler.DeleteResponseConfig)

	req := httptest.NewRequest("DELETE", "/responses/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── UpdateResponsePriority ────────────────────────────────────────────────────

func TestUpdateResponsePriority(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateResponseConfig(&models.ResponseConfig{ID: "resp-1", OperationID: "op-1", Priority: 5, StatusCode: 200})
	r.PUT("/responses/:id/priority", handler.UpdateResponsePriority)

	body := []byte(`{"priority":2}`)
	req := httptest.NewRequest("PUT", "/responses/resp-1/priority", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	var result models.ResponseConfig
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Priority != 2 {
		t.Fatalf("expected priority to be updated")
	}
}

func TestUpdateResponsePriority_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/responses/:id/priority", handler.UpdateResponsePriority)

	body := []byte(`{"priority":1}`)
	req := httptest.NewRequest("PUT", "/responses/nonexistent/priority", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateResponsePriority_InvalidJSON(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateResponseConfig(&models.ResponseConfig{ID: "cfg-1", OperationID: "op-1", Priority: 1})
	r.PUT("/responses/:id/priority", handler.UpdateResponsePriority)

	req := httptest.NewRequest("PUT", "/responses/cfg-1/priority", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
