package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/models"
)

// ── Stats ─────────────────────────────────────────────────────────────────────

func TestGetGlobalStats(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/stats", handler.GetGlobalStats)

	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if _, ok := result["totalRequests"]; !ok {
		t.Error("Expected totalRequests field")
	}
	if _, ok := result["totalErrors"]; !ok {
		t.Error("Expected totalErrors field")
	}
}

func TestResetStats(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/stats/reset", handler.ResetStats)

	req := httptest.NewRequest("POST", "/stats/reset", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetSpecStatsAndOperationStats(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "Spec", Enabled: true})

	r.GET("/stats/specs/:id", handler.GetSpecStats)
	req := httptest.NewRequest("GET", "/stats/specs/spec-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	r.GET("/stats/operations/:id", handler.GetOperationStats)
	req = httptest.NewRequest("GET", "/stats/operations/op-1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	var msg map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &msg)
	if msg["message"] == nil {
		t.Fatalf("expected message when stats are missing")
	}
}

func TestGetSpecStats_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/stats/specs/:id", handler.GetSpecStats)

	req := httptest.NewRequest("GET", "/stats/specs/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── Traces ────────────────────────────────────────────────────────────────────

func TestListTraces(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/traces", handler.ListTraces)

	req := httptest.NewRequest("GET", "/traces", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestListTraces_WithFilters(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/traces", handler.ListTraces)

	handler.tracingService.RecordTrace(&models.Trace{
		SpecID:      "spec-1",
		OperationID: "op-1",
		Request:     models.TraceRequest{Method: "POST", Path: "/items"},
		Response:    models.TraceResponse{StatusCode: 201},
	})

	req := httptest.NewRequest("GET", "/traces?operationId=op-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/traces?method=POST", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetTrace(t *testing.T) {
	handler, _, r := setupTestHandler(t)

	trace := &models.Trace{
		SpecID:      "spec-1",
		OperationID: "op-1",
		Request:     models.TraceRequest{Method: "GET", Path: "/users"},
		Response:    models.TraceResponse{StatusCode: 200},
	}
	handler.tracingService.RecordTrace(trace)

	r.GET("/traces/:id", handler.GetTrace)
	req := httptest.NewRequest("GET", "/traces/"+trace.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/traces/missing", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestClearTraces(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.DELETE("/traces", handler.ClearTraces)

	req := httptest.NewRequest("DELETE", "/traces", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestClearTraces_BySpec(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.DELETE("/traces", handler.ClearTraces)

	handler.tracingService.RecordTrace(&models.Trace{
		SpecID:   "spec-1",
		Request:  models.TraceRequest{Method: "GET", Path: "/x"},
		Response: models.TraceResponse{StatusCode: 200},
	})

	req := httptest.NewRequest("DELETE", "/traces?specId=spec-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ── System endpoints ──────────────────────────────────────────────────────────

func TestHealthCheck(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/health", handler.HealthCheck)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", result["status"])
	}
}

func TestVersion(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/version", handler.Version)

	req := httptest.NewRequest("GET", "/version", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["version"] == nil {
		t.Fatalf("expected version field in response")
	}
}

func TestGetRoutes(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "API 1", BasePath: "/api", Enabled: true})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users", FullPath: "/api/users"})
	handler.proxyEngine.ReloadRoutes()

	r.GET("/routes", handler.GetRoutes)
	req := httptest.NewRequest("GET", "/routes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ── Branding ──────────────────────────────────────────────────────────────────

func TestGetBranding_Defaults(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/branding", handler.GetBranding)

	req := httptest.NewRequest("GET", "/branding", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if _, ok := result["appTitle"]; !ok {
		t.Error("Expected appTitle in response")
	}
	if _, ok := result["appSubtitle"]; !ok {
		t.Error("Expected appSubtitle in response")
	}
}

func TestGetBranding_WithCustomValues(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	handler.SetBranding(config.BrandingConfig{
		AppTitle:    "My Custom App",
		AppSubtitle: "Great Subtitle",
	})
	r.GET("/branding", handler.GetBranding)

	req := httptest.NewRequest("GET", "/branding", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result["appTitle"] != "My Custom App" {
		t.Errorf("Expected appTitle 'My Custom App', got %q", result["appTitle"])
	}
	if result["appSubtitle"] != "Great Subtitle" {
		t.Errorf("Expected appSubtitle 'Great Subtitle', got %q", result["appSubtitle"])
	}
}

func TestSetBranding_NormalizesEmptyValues(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	handler.SetBranding(config.BrandingConfig{}) // empty → defaults
	r.GET("/branding", handler.GetBranding)

	req := httptest.NewRequest("GET", "/branding", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result["appTitle"] != "go-virtual" {
		t.Errorf("Expected default appTitle 'go-virtual', got %q", result["appTitle"])
	}
	if result["appSubtitle"] != "API Mock & Virtualization" {
		t.Errorf("Expected default appSubtitle 'API Mock & Virtualization', got %q", result["appSubtitle"])
	}
}

// ── ValidateTemplate ──────────────────────────────────────────────────────────

func TestValidateTemplate_Valid(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/validate-template", handler.ValidateTemplate)

	body := map[string]string{"body": `{"id": "{{path "id"}}"}`}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/validate-template", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestValidateTemplate_Invalid(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/validate-template", handler.ValidateTemplate)

	body := map[string]string{"body": `{{.unclosed`}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/validate-template", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestValidateTemplate_InvalidJSON(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/validate-template", handler.ValidateTemplate)

	req := httptest.NewRequest("POST", "/validate-template", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── Tags ──────────────────────────────────────────────────────────────────────

func TestTagLifecycleAndSpecTags(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	r.POST("/tags", handler.CreateTag)
	createBody := []byte(`{"name":"Blue","description":"Primary"}`)
	req := httptest.NewRequest("POST", "/tags", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}

	r.GET("/tags", handler.ListTags)
	req = httptest.NewRequest("GET", "/tags", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	r.PUT("/tags/:name", handler.UpdateTag)
	updateBody := []byte(`{"name":"blue","description":"Updated"}`)
	req = httptest.NewRequest("PUT", "/tags/blue", bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "Spec"})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	store.CreateResponseConfig(&models.ResponseConfig{ID: "resp-1", OperationID: "op-1", Tag: "blue", StatusCode: 200})

	r.PUT("/specs/:id/tags", handler.UpdateSpecTags)
	updateTags := []byte(`{"tags":["blue","BLUE"," "]}`)
	req = httptest.NewRequest("PUT", "/specs/spec-1/tags", bytes.NewReader(updateTags))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	r.GET("/specs/:id/tags", handler.GetSpecTags)
	req = httptest.NewRequest("GET", "/specs/spec-1/tags", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	badTags := []byte(`{"tags":["missing"]}`)
	req = httptest.NewRequest("PUT", "/specs/spec-1/tags", bytes.NewReader(badTags))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	r.DELETE("/tags/:name", handler.DeleteTag)
	req = httptest.NewRequest("DELETE", "/tags/blue", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	updatedSpec, _ := store.GetSpec("spec-1")
	if len(updatedSpec.EnabledTags) != 0 {
		t.Fatalf("expected enabled tags to be cleared")
	}
	updatedCfg, _ := store.GetResponseConfig("resp-1")
	if updatedCfg.Tag != models.DefaultTagName {
		t.Fatalf("expected response tag to be default")
	}
}

func TestRenameTagUsage(t *testing.T) {
	handler, store, _ := setupTestHandler(t)

	store.CreateSpec(&models.Spec{ID: "spec-1", Name: "Spec", EnabledTags: []string{"old"}})
	store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	store.CreateResponseConfig(&models.ResponseConfig{ID: "resp-1", OperationID: "op-1", Tag: "old", StatusCode: 200})

	handler.renameTagUsage("old", "new")

	updatedSpec, _ := store.GetSpec("spec-1")
	if len(updatedSpec.EnabledTags) != 1 || updatedSpec.EnabledTags[0] != "new" {
		t.Fatalf("expected enabled tag to be renamed")
	}
	updatedCfg, _ := store.GetResponseConfig("resp-1")
	if updatedCfg.Tag != "new" {
		t.Fatalf("expected response tag to be renamed")
	}
}

func TestCreateTag_InvalidJSON(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/tags", handler.CreateTag)

	req := httptest.NewRequest("POST", "/tags", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateTag_EmptyName(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/tags", handler.CreateTag)

	req := httptest.NewRequest("POST", "/tags", bytes.NewReader([]byte(`{"name":"   "}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateTag_DefaultTagName(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/tags", handler.CreateTag)

	req := httptest.NewRequest("POST", "/tags", bytes.NewReader([]byte(`{"name":"default"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateTag_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/tags/:name", handler.UpdateTag)

	req := httptest.NewRequest("PUT", "/tags/nonexistent", bytes.NewReader([]byte(`{"name":"nonexistent","description":"x"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateTag_InvalidJSON(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateTag(&models.Tag{Name: "blue"})
	r.PUT("/tags/:name", handler.UpdateTag)

	req := httptest.NewRequest("PUT", "/tags/blue", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateTag_RenameAttempt(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	store.CreateTag(&models.Tag{Name: "blue"})
	r.PUT("/tags/:name", handler.UpdateTag)

	req := httptest.NewRequest("PUT", "/tags/blue", bytes.NewReader([]byte(`{"name":"green","description":"x"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDeleteTag_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.DELETE("/tags/:name", handler.DeleteTag)

	req := httptest.NewRequest("DELETE", "/tags/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteTag_DefaultTag(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.DELETE("/tags/:name", handler.DeleteTag)

	req := httptest.NewRequest("DELETE", "/tags/default", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
