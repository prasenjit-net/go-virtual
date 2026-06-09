package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
)

// ── ListSpecValidations ───────────────────────────────────────────────────────

func TestListSpecValidations(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	_ = store.CreateSpec(&models.Spec{ID: "spec-1", Name: "S"})
	_, _ = store.CreateValidationRule(&models.ValidationRule{ID: "vr-1", SpecID: "spec-1", Name: "checkA", Enabled: true})
	_, _ = store.CreateValidationRule(&models.ValidationRule{ID: "vr-2", SpecID: "spec-1", Name: "checkB", Enabled: false})

	r.GET("/specs/:id/validations", handler.ListSpecValidations)
	req := httptest.NewRequest("GET", "/specs/spec-1/validations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var rules []models.ValidationRule
	json.Unmarshal(w.Body.Bytes(), &rules)
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
}

func TestListSpecValidations_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/specs/:id/validations", handler.ListSpecValidations)
	req := httptest.NewRequest("GET", "/specs/nonexistent/validations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── ListOperationValidations ──────────────────────────────────────────────────

func TestListOperationValidations(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	_ = store.CreateSpec(&models.Spec{ID: "spec-1"})
	_ = store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	_, _ = store.CreateValidationRule(&models.ValidationRule{ID: "vr-1", OperationID: "op-1", Name: "checkX", Enabled: true})

	r.GET("/operations/:id/validations", handler.ListOperationValidations)
	req := httptest.NewRequest("GET", "/operations/op-1/validations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var rules []models.ValidationRule
	json.Unmarshal(w.Body.Bytes(), &rules)
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestListOperationValidations_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/operations/:id/validations", handler.ListOperationValidations)
	req := httptest.NewRequest("GET", "/operations/nonexistent/validations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── CreateSpecValidation ──────────────────────────────────────────────────────

func TestCreateSpecValidation(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	_ = store.CreateSpec(&models.Spec{ID: "spec-1", Name: "S"})

	r.POST("/specs/:id/validations", handler.CreateSpecValidation)

	body := map[string]interface{}{
		"name":    "myRule",
		"enabled": true,
		"order":   0,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/specs/spec-1/validations", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var rule models.ValidationRule
	json.Unmarshal(w.Body.Bytes(), &rule)
	if rule.Name != "myRule" {
		t.Errorf("expected name=myRule, got %q", rule.Name)
	}
	if rule.SpecID != "spec-1" {
		t.Errorf("expected specId=spec-1, got %q", rule.SpecID)
	}
	if rule.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCreateSpecValidation_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/specs/:id/validations", handler.CreateSpecValidation)

	body := map[string]interface{}{"name": "myRule"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/specs/nonexistent/validations", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCreateSpecValidation_EmptyName(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	_ = store.CreateSpec(&models.Spec{ID: "spec-1"})
	r.POST("/specs/:id/validations", handler.CreateSpecValidation)

	body := map[string]interface{}{"name": ""}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/specs/spec-1/validations", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty name, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSpecValidation_InvalidName(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	_ = store.CreateSpec(&models.Spec{ID: "spec-1"})
	r.POST("/specs/:id/validations", handler.CreateSpecValidation)

	// Name starts with digit — invalid per regex ^[a-zA-Z_][a-zA-Z0-9_]*$
	body := map[string]interface{}{"name": "1invalid"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/specs/spec-1/validations", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid name, got %d: %s", w.Code, w.Body.String())
	}
}

// ── CreateOperationValidation ─────────────────────────────────────────────────

func TestCreateOperationValidation(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	_ = store.CreateSpec(&models.Spec{ID: "spec-1"})
	_ = store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	r.POST("/operations/:id/validations", handler.CreateOperationValidation)

	body := map[string]interface{}{
		"name":    "opRule",
		"enabled": true,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-1/validations", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var rule models.ValidationRule
	json.Unmarshal(w.Body.Bytes(), &rule)
	if rule.OperationID != "op-1" {
		t.Errorf("expected operationId=op-1, got %q", rule.OperationID)
	}
}

func TestCreateOperationValidation_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/operations/:id/validations", handler.CreateOperationValidation)

	body := map[string]interface{}{"name": "x"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/nonexistent/validations", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCreateOperationValidation_InvalidName(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	_ = store.CreateSpec(&models.Spec{ID: "spec-1"})
	_ = store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	r.POST("/operations/:id/validations", handler.CreateOperationValidation)

	body := map[string]interface{}{"name": "bad-name!"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-1/validations", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid name, got %d", w.Code)
	}
}

// ── GetValidation ─────────────────────────────────────────────────────────────

func TestGetValidation(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	_, _ = store.CreateValidationRule(&models.ValidationRule{ID: "vr-1", SpecID: "spec-1", Name: "myRule"})

	r.GET("/validations/:id", handler.GetValidation)
	req := httptest.NewRequest("GET", "/validations/vr-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var rule models.ValidationRule
	json.Unmarshal(w.Body.Bytes(), &rule)
	if rule.Name != "myRule" {
		t.Errorf("expected name=myRule, got %q", rule.Name)
	}
}

func TestGetValidation_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/validations/:id", handler.GetValidation)
	req := httptest.NewRequest("GET", "/validations/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── UpdateValidation ──────────────────────────────────────────────────────────

func TestUpdateValidation(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	_, _ = store.CreateValidationRule(&models.ValidationRule{ID: "vr-1", SpecID: "spec-1", Name: "original", Enabled: false})

	r.PUT("/validations/:id", handler.UpdateValidation)

	body := map[string]interface{}{
		"name":    "updated",
		"enabled": true,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/validations/vr-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var rule models.ValidationRule
	json.Unmarshal(w.Body.Bytes(), &rule)
	if rule.Name != "updated" {
		t.Errorf("expected name=updated, got %q", rule.Name)
	}
	if !rule.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestUpdateValidation_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/validations/:id", handler.UpdateValidation)

	body := map[string]interface{}{"name": "x"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/validations/nonexistent", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateValidation_InvalidName(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	_, _ = store.CreateValidationRule(&models.ValidationRule{ID: "vr-1", SpecID: "spec-1", Name: "original"})
	r.PUT("/validations/:id", handler.UpdateValidation)

	body := map[string]interface{}{"name": "123bad"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/validations/vr-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid name, got %d", w.Code)
	}
}

// ── DeleteValidation ──────────────────────────────────────────────────────────

func TestDeleteValidation(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	_, _ = store.CreateValidationRule(&models.ValidationRule{ID: "vr-1", SpecID: "spec-1", Name: "toDelete"})

	r.DELETE("/validations/:id", handler.DeleteValidation)
	req := httptest.NewRequest("DELETE", "/validations/vr-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify it's gone
	_, err := store.GetValidationRule("vr-1")
	if err == nil {
		t.Error("expected rule to be deleted")
	}
}

func TestDeleteValidation_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.DELETE("/validations/:id", handler.DeleteValidation)
	req := httptest.NewRequest("DELETE", "/validations/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
