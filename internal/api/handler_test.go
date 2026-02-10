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
	"github.com/prasenjit/go-virtual/internal/tracing"
)

func setupTestHandler(t *testing.T) (*Handler, storage.Storage, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	store := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)
	proxyEngine := proxy.NewEngine(store, collector, tracingSvc)

	handler := NewHandler(store, collector, tracingSvc, proxyEngine)

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

	// Add some specs
	spec1 := &models.Spec{ID: "spec-1", Name: "API 1", Version: "1.0", Enabled: true}
	spec2 := &models.Spec{ID: "spec-2", Name: "API 2", Version: "2.0", Enabled: false}
	store.CreateSpec(spec1)
	store.CreateSpec(spec2)

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

	body := map[string]string{
		"content":  specContent,
		"basePath": "/api/v1",
	}
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

	body := map[string]string{
		"content": "invalid: yaml: content: here",
	}
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

func TestGetSpec_Exists(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "API 1", Version: "1.0", Content: "spec content"}
	store.CreateSpec(spec)

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

func TestUpdateResponsePriority(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	_ = store.CreateResponseConfig(&models.ResponseConfig{
		ID:          "resp-1",
		OperationID: "op-1",
		Priority:    5,
		StatusCode:  200,
	})

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
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if result.Priority != 2 {
		t.Fatalf("expected priority to be updated")
	}
}

func TestGetSpecStatsAndOperationStats(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	_ = store.CreateSpec(&models.Spec{ID: "spec-1", Name: "Spec", Enabled: true})

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
	_ = json.Unmarshal(w.Body.Bytes(), &msg)
	if msg["message"] == nil {
		t.Fatalf("expected message when stats are missing")
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

	spec := &models.Spec{ID: "spec-1", Name: "Spec"}
	_ = store.CreateSpec(spec)
	_ = store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	_ = store.CreateResponseConfig(&models.ResponseConfig{ID: "resp-1", OperationID: "op-1", Tag: "blue", StatusCode: 200})

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

func TestRenameTagUsage(t *testing.T) {
	handler, store, _ := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "Spec", EnabledTags: []string{"old"}}
	_ = store.CreateSpec(spec)
	_ = store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	_ = store.CreateResponseConfig(&models.ResponseConfig{ID: "resp-1", OperationID: "op-1", Tag: "old", StatusCode: 200})

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
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if result["version"] == nil {
		t.Fatalf("expected version field in response")
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

func TestUpdateSpec(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "Old Name", Version: "1.0", Enabled: true}
	store.CreateSpec(spec)

	r.PUT("/specs/:id", handler.UpdateSpec)

	newName := "New Name"
	update := map[string]interface{}{
		"name":    newName,
		"enabled": false,
	}
	jsonBody, _ := json.Marshal(update)

	req := httptest.NewRequest("PUT", "/specs/spec-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify update
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

func TestDeleteSpec(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "API 1"}
	store.CreateSpec(spec)

	// Add an operation
	op := &models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users"}
	store.CreateOperation(op)

	// Add a response config
	config := &models.ResponseConfig{ID: "config-1", OperationID: "op-1", Name: "Default"}
	store.CreateResponseConfig(config)

	r.DELETE("/specs/:id", handler.DeleteSpec)

	req := httptest.NewRequest("DELETE", "/specs/spec-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify deletion
	_, err := store.GetSpec("spec-1")
	if err == nil {
		t.Error("Expected spec to be deleted")
	}

	// Verify operations deleted
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

func TestListOperations(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "API 1"}
	store.CreateSpec(spec)

	op1 := &models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users"}
	op2 := &models.Operation{ID: "op-2", SpecID: "spec-1", Method: "POST", Path: "/users"}
	store.CreateOperation(op1)
	store.CreateOperation(op2)

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

func TestGetOperation(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "API 1"}
	store.CreateSpec(spec)

	op := &models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users", Summary: "List users"}
	store.CreateOperation(op)

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

func TestListResponseConfigs(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1"}
	store.CreateSpec(spec)

	op := &models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users"}
	store.CreateOperation(op)

	config1 := &models.ResponseConfig{ID: "config-1", OperationID: "op-1", Name: "Success", Priority: 1}
	config2 := &models.ResponseConfig{ID: "config-2", OperationID: "op-1", Name: "Error", Priority: 2}
	store.CreateResponseConfig(config1)
	store.CreateResponseConfig(config2)

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

func TestCreateResponseConfig(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1"}
	store.CreateSpec(spec)

	op := &models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users"}
	store.CreateOperation(op)

	r.POST("/operations/:id/responses", handler.CreateResponseConfig)

	body := map[string]interface{}{
		"name":       "Success Response",
		"statusCode": 200,
		"body":       `{"message": "OK"}`,
		"headers": map[string]string{
			"Content-Type": "application/json",
		},
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

	body := map[string]interface{}{
		"name":       "Success",
		"statusCode": 200,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/operations/nonexistent/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGetResponseConfig(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	config := &models.ResponseConfig{ID: "config-1", OperationID: "op-1", Name: "Default", StatusCode: 200}
	store.CreateResponseConfig(config)

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

func TestUpdateResponseConfig(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1"}
	store.CreateSpec(spec)

	op := &models.Operation{ID: "op-1", SpecID: "spec-1"}
	store.CreateOperation(op)

	config := &models.ResponseConfig{ID: "config-1", OperationID: "op-1", Name: "Old Name", StatusCode: 200}
	store.CreateResponseConfig(config)

	r.PUT("/responses/:id", handler.UpdateResponseConfig)

	update := map[string]interface{}{
		"name":       "New Name",
		"statusCode": 201,
		"body":       `{"updated": true}`,
	}
	jsonBody, _ := json.Marshal(update)

	req := httptest.NewRequest("PUT", "/responses/config-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify update
	updatedConfig, _ := store.GetResponseConfig("config-1")
	if updatedConfig.Name != "New Name" {
		t.Errorf("Expected name 'New Name', got %q", updatedConfig.Name)
	}
	if updatedConfig.StatusCode != 201 {
		t.Errorf("Expected status code 201, got %d", updatedConfig.StatusCode)
	}
}

func TestDeleteResponseConfig(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1"}
	store.CreateSpec(spec)

	op := &models.Operation{ID: "op-1", SpecID: "spec-1"}
	store.CreateOperation(op)

	config := &models.ResponseConfig{ID: "config-1", OperationID: "op-1", Name: "Default"}
	store.CreateResponseConfig(config)

	r.DELETE("/responses/:id", handler.DeleteResponseConfig)

	req := httptest.NewRequest("DELETE", "/responses/config-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify deletion
	_, err := store.GetResponseConfig("config-1")
	if err == nil {
		t.Error("Expected config to be deleted")
	}
}

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

	// Check expected fields
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

func TestGetRoutes(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	// Add a spec and operation
	spec := &models.Spec{ID: "spec-1", Name: "API 1", BasePath: "/api", Enabled: true}
	store.CreateSpec(spec)

	op := &models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users", FullPath: "/api/users"}
	store.CreateOperation(op)

	// Reload proxy routes
	handler.proxyEngine.ReloadRoutes()

	r.GET("/routes", handler.GetRoutes)

	req := httptest.NewRequest("GET", "/routes", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestEnableSpec(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "API 1", Enabled: false}
	store.CreateSpec(spec)

	r.PUT("/specs/:id/enable", handler.EnableSpec)

	req := httptest.NewRequest("PUT", "/specs/spec-1/enable", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify enabled
	updatedSpec, _ := store.GetSpec("spec-1")
	if !updatedSpec.Enabled {
		t.Error("Expected spec to be enabled")
	}
}

func TestDisableSpec(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "API 1", Enabled: true}
	store.CreateSpec(spec)

	r.PUT("/specs/:id/disable", handler.DisableSpec)

	req := httptest.NewRequest("PUT", "/specs/spec-1/disable", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify disabled
	updatedSpec, _ := store.GetSpec("spec-1")
	if updatedSpec.Enabled {
		t.Error("Expected spec to be disabled")
	}
}

func TestToggleTracing(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "API 1", Tracing: false}
	store.CreateSpec(spec)

	r.PUT("/specs/:id/tracing", handler.ToggleTracing)

	// Enable tracing
	body := map[string]bool{"enabled": true}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/specs/spec-1/tracing", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify tracing enabled
	updatedSpec, _ := store.GetSpec("spec-1")
	if !updatedSpec.Tracing {
		t.Error("Expected tracing to be enabled")
	}
}

func TestToggleExampleFallback(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "API 1", UseExampleFallback: true}
	store.CreateSpec(spec)

	r.PUT("/specs/:id/example-fallback", handler.ToggleExampleFallback)

	// Disable example fallback
	body := map[string]bool{"enabled": false}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/specs/spec-1/example-fallback", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify example fallback disabled
	updatedSpec, _ := store.GetSpec("spec-1")
	if updatedSpec.UseExampleFallback {
		t.Error("Expected example fallback to be disabled")
	}
}
