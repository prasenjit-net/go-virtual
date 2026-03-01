package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
)

// setupScriptTest registers all script and binding routes and returns the handler.
func setupScriptTest(t *testing.T) (*Handler, storage.Storage, *gin.Engine) {
	t.Helper()
	handler, store, r := setupTestHandler(t)
	r.GET("/scripts", handler.ListScripts)
	r.POST("/scripts", handler.CreateScript)
	r.POST("/scripts/validate", handler.ValidateScript)
	r.GET("/scripts/:id", handler.GetScript)
	r.PUT("/scripts/:id", handler.UpdateScript)
	r.DELETE("/scripts/:id", handler.DeleteScript)
	r.POST("/scripts/:id/test", handler.TestScript)
	r.GET("/operations/:id/scripts", handler.ListScriptBindings)
	r.POST("/operations/:id/scripts", handler.CreateScriptBinding)
	r.PUT("/operations/:id/scripts/reorder", handler.ReorderScriptBindings)
	r.PUT("/operations/:id/scripts/:bindingId", handler.UpdateScriptBinding)
	r.DELETE("/operations/:id/scripts/:bindingId", handler.DeleteScriptBinding)
	return handler, store, r
}

// ── ListScripts ───────────────────────────────────────────────────────────────

func TestListScripts_Empty(t *testing.T) {
	_, _, r := setupScriptTest(t)

	req := httptest.NewRequest("GET", "/scripts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
	var result []interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 0 {
		t.Errorf("Expected empty list, got %d items", len(result))
	}
}

func TestListScripts_OmitsSource(t *testing.T) {
	_, store, r := setupScriptTest(t)

	store.CreateScript(&models.Script{ID: "s1", Name: "S1", Source: "def run(req): return 1", Enabled: true})
	store.CreateScript(&models.Script{ID: "s2", Name: "S2", Source: "def run(req): return 2", Enabled: true})

	req := httptest.NewRequest("GET", "/scripts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
	var result []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 2 {
		t.Fatalf("Expected 2 scripts, got %d", len(result))
	}
	for _, item := range result {
		if _, ok := item["source"]; ok {
			t.Error("List response must not include 'source' field")
		}
	}
}

// ── CreateScript ──────────────────────────────────────────────────────────────

func TestCreateScript_Valid(t *testing.T) {
	_, _, r := setupScriptTest(t)

	body := map[string]interface{}{
		"name":        "Test Script",
		"description": "A test",
		"source":      "def run(req):\n    return {\"ok\": True}",
		"timeout":     100,
		"enabled":     true,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/scripts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["id"] == nil || result["id"] == "" {
		t.Error("Expected id in response")
	}
	if result["name"] != "Test Script" {
		t.Errorf("Expected name 'Test Script', got %v", result["name"])
	}
	if result["source"] == nil {
		t.Error("Expected source field in response")
	}
}

func TestCreateScript_InvalidSource(t *testing.T) {
	_, _, r := setupScriptTest(t)

	body := map[string]interface{}{
		"name":    "Bad Script",
		"source":  "def run(req  # missing paren",
		"timeout": 100,
		"enabled": true,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/scripts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("Expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

// ── GetScript ─────────────────────────────────────────────────────────────────

func TestGetScript_IncludesSource(t *testing.T) {
	_, store, r := setupScriptTest(t)

	store.CreateScript(&models.Script{
		ID:      "scr-1",
		Name:    "My Script",
		Source:  "def run(req): return 1",
		Timeout: 50,
		Enabled: true,
	})

	req := httptest.NewRequest("GET", "/scripts/scr-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["source"] == nil {
		t.Error("Expected 'source' in GET /:id response")
	}
	if result["source"] != "def run(req): return 1" {
		t.Errorf("source mismatch: got %v", result["source"])
	}
}

func TestGetScript_NotFound(t *testing.T) {
	_, _, r := setupScriptTest(t)

	req := httptest.NewRequest("GET", "/scripts/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d", w.Code)
	}
}

// ── UpdateScript ──────────────────────────────────────────────────────────────

func TestUpdateScript_Valid(t *testing.T) {
	_, store, r := setupScriptTest(t)

	store.CreateScript(&models.Script{
		ID:      "s1",
		Name:    "Original",
		Source:  "def run(req): return 1",
		Enabled: true,
		Timeout: 100,
	})

	body := map[string]interface{}{
		"name":        "Updated",
		"description": "new desc",
		"source":      "def run(req): return 2",
		"timeout":     200,
		"enabled":     false,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/scripts/s1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["name"] != "Updated" {
		t.Errorf("Expected name 'Updated', got %v", result["name"])
	}
	if result["source"] == nil {
		t.Error("Expected source in update response")
	}
}

// ── DeleteScript ──────────────────────────────────────────────────────────────

func TestDeleteScript_Success(t *testing.T) {
	_, store, r := setupScriptTest(t)

	store.CreateScript(&models.Script{ID: "s1", Name: "ToDelete", Source: "def run(req): return 1", Enabled: true})

	req := httptest.NewRequest("DELETE", "/scripts/s1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Fatalf("Expected 204 or 200, got %d", w.Code)
	}
	req2 := httptest.NewRequest("GET", "/scripts/s1", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Errorf("Expected 404 after delete, got %d", w2.Code)
	}
}

// ── ValidateScript ────────────────────────────────────────────────────────────

func TestValidateScript_Valid(t *testing.T) {
	_, _, r := setupScriptTest(t)

	body := map[string]string{"source": "def run(req):\n    return {\"ok\": True}"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/scripts/validate", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["valid"] != true {
		t.Errorf("Expected valid=true, got %v", result["valid"])
	}
}

func TestValidateScript_Invalid(t *testing.T) {
	_, _, r := setupScriptTest(t)

	body := map[string]string{"source": "def run(req  # bad"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/scripts/validate", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["valid"] != false {
		t.Errorf("Expected valid=false, got %v", result["valid"])
	}
	if result["error"] == nil || result["error"] == "" {
		t.Error("Expected error message in response")
	}
}

// ── TestScript ────────────────────────────────────────────────────────────────

func TestTestScript_Success(t *testing.T) {
	_, store, r := setupScriptTest(t)

	store.CreateScript(&models.Script{
		ID:      "s1",
		Name:    "Calc",
		Source:  "def run(req):\n    return {\"doubled\": 84}",
		Timeout: 100,
		Enabled: true,
	})

	body := map[string]interface{}{
		"input": map[string]interface{}{
			"path":  map[string]string{},
			"query": map[string]string{},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/scripts/s1/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["error"] != nil && result["error"] != "" {
		t.Errorf("Expected no error, got %v", result["error"])
	}
	out, ok := result["output"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map output, got %T", result["output"])
	}
	if out["doubled"] != float64(84) {
		t.Errorf("doubled: got %v, want 84", out["doubled"])
	}
}

// ── Script Bindings ───────────────────────────────────────────────────────────

func TestListScriptBindings_Empty(t *testing.T) {
	_, store, r := setupScriptTest(t)
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/test", FullPath: "/test"})

	req := httptest.NewRequest("GET", "/operations/op-1/scripts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
	var result []interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 0 {
		t.Errorf("Expected empty list, got %d items", len(result))
	}
}

func TestCreateScriptBinding_Success(t *testing.T) {
	_, store, r := setupScriptTest(t)

	store.CreateScript(&models.Script{ID: "s1", Name: "S1", Source: "def run(req): return 1", Enabled: true})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/test", FullPath: "/test"})

	body := map[string]interface{}{"scriptId": "s1", "outputKey": "result", "order": 0, "enabled": true}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-1/scripts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["id"] == nil {
		t.Error("Expected id in response")
	}
	if result["outputKey"] != "result" {
		t.Errorf("outputKey: got %v, want 'result'", result["outputKey"])
	}
}

func TestUpdateScriptBinding_Success(t *testing.T) {
	_, store, r := setupScriptTest(t)

	store.CreateScript(&models.Script{ID: "s1", Name: "S1", Source: "def run(req): return 1", Enabled: true})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/test", FullPath: "/test"})
	store.CreateScriptBinding(&models.ScriptBinding{
		ID: "b1", OperationID: "op-1", ScriptID: "s1", OutputKey: "old", Order: 0, Enabled: true,
	})

	body := map[string]interface{}{"scriptId": "s1", "outputKey": "new", "order": 5, "enabled": false}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/operations/op-1/scripts/b1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["outputKey"] != "new" {
		t.Errorf("outputKey: got %v, want 'new'", result["outputKey"])
	}
}

func TestDeleteScriptBinding_Success(t *testing.T) {
	_, store, r := setupScriptTest(t)

	store.CreateScript(&models.Script{ID: "s1", Name: "S1", Source: "def run(req): return 1", Enabled: true})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/test", FullPath: "/test"})
	store.CreateScriptBinding(&models.ScriptBinding{
		ID: "b1", OperationID: "op-1", ScriptID: "s1", OutputKey: "out", Order: 0, Enabled: true,
	})

	req := httptest.NewRequest("DELETE", "/operations/op-1/scripts/b1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Fatalf("Expected 204 or 200, got %d", w.Code)
	}
	req2 := httptest.NewRequest("GET", "/operations/op-1/scripts", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	var bindings []interface{}
	json.Unmarshal(w2.Body.Bytes(), &bindings)
	if len(bindings) != 0 {
		t.Errorf("Expected 0 bindings after delete, got %d", len(bindings))
	}
}

func TestReorderScriptBindings_Success(t *testing.T) {
	_, store, r := setupScriptTest(t)

	store.CreateScript(&models.Script{ID: "s1", Name: "S1", Source: "def run(req): return 1", Enabled: true})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/test", FullPath: "/test"})
	store.CreateScriptBinding(&models.ScriptBinding{ID: "b1", OperationID: "op-1", ScriptID: "s1", OutputKey: "a", Order: 0, Enabled: true})
	store.CreateScriptBinding(&models.ScriptBinding{ID: "b2", OperationID: "op-1", ScriptID: "s1", OutputKey: "b", Order: 1, Enabled: true})

	body := []map[string]interface{}{{"id": "b1", "order": 1}, {"id": "b2", "order": 0}}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/operations/op-1/scripts/reorder", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 2 {
		t.Fatalf("Expected 2 bindings in response, got %d", len(result))
	}
}
