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

// ---- SetBackendURI ----

func TestSetBackendURI(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "API"}
	store.CreateSpec(spec)

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

	spec := &models.Spec{ID: "spec-1", Name: "API", BackendURI: "http://old", ProxyMode: true}
	store.CreateSpec(spec)

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

// ---- ToggleProxyMode ----

func TestToggleProxyMode_Enable(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "API", BackendURI: "http://backend"}
	store.CreateSpec(spec)

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

	spec := &models.Spec{ID: "spec-1", Name: "API"} // no BackendURI
	store.CreateSpec(spec)

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

	spec := &models.Spec{ID: "spec-1", Name: "API", BackendURI: "http://be", ProxyMode: false}
	store.CreateSpec(spec)

	r.PUT("/specs/:id/proxy-mode", handler.ToggleProxyMode)

	// No body → toggles
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

// ---- GetSignatureConfig / UpdateSignatureConfig ----

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

	op := &models.Operation{
		ID:     "op-1",
		SpecID: "spec-1",
		SignatureConfig: &models.SignatureConfig{
			PathParams: []string{"id"},
		},
	}
	store.CreateOperation(op)

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

// ---- ValidateTemplate ----

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

// ---- UpdateSpec with backend/proxyMode fields ----

func TestUpdateSpec_WithBackendAndProxyMode(t *testing.T) {
	handler, store, r := setupTestHandler(t)

	spec := &models.Spec{ID: "spec-1", Name: "API", BackendURI: "http://be"}
	store.CreateSpec(spec)

	r.PUT("/specs/:id", handler.UpdateSpec)

	trueVal := true
	update := map[string]interface{}{
		"backendUri": "http://new-backend",
		"proxyMode":  trueVal,
	}
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

	spec := &models.Spec{ID: "spec-1", Name: "API"} // no BackendURI
	store.CreateSpec(spec)

	r.PUT("/specs/:id", handler.UpdateSpec)

	trueVal := true
	update := map[string]interface{}{"proxyMode": trueVal}
	jsonBody, _ := json.Marshal(update)
	req := httptest.NewRequest("PUT", "/specs/spec-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when enabling proxyMode without backendUri, got %d", w.Code)
	}
}

// ---- Additional error-path tests to hit 80% coverage ----

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

	// No body → should toggle from true → false
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

	// No body → should toggle from false → true
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

	newBase := "/v2"
	update := map[string]interface{}{"basePath": newBase}
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

	desc := "updated desc"
	prio := 3
	delay := 50
	enabled := true
	code := 201
	body := `{"ok":true}`
	headers := map[string]string{"X-Custom": "value"}
	conditions := []models.Condition{{Source: "query", Key: "env", Operator: "eq", Value: "prod"}}

	update := map[string]interface{}{
		"description": desc,
		"priority":    prio,
		"delay":       delay,
		"enabled":     enabled,
		"statusCode":  code,
		"body":        body,
		"headers":     headers,
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
	if cfg.Description != desc {
		t.Errorf("expected description %q, got %q", desc, cfg.Description)
	}
	if cfg.Priority != prio {
		t.Errorf("expected priority %d, got %d", prio, cfg.Priority)
	}
	if cfg.Delay != delay {
		t.Errorf("expected delay %d, got %d", delay, cfg.Delay)
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

	// Empty tag should be set to default
	update := map[string]interface{}{"tag": ""}
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

func TestListTraces_WithFilters(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/traces", handler.ListTraces)

	// Seed a trace
	handler.tracingService.RecordTrace(&models.Trace{
		SpecID:      "spec-1",
		OperationID: "op-1",
		Request:     models.TraceRequest{Method: "POST", Path: "/items"},
		Response:    models.TraceResponse{StatusCode: 201},
	})

	// Filter by operationId
	req := httptest.NewRequest("GET", "/traces?operationId=op-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Filter by method
	req = httptest.NewRequest("GET", "/traces?method=POST", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
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

	body := []byte(`{"name":"   "}`) // normalizes to empty
	req := httptest.NewRequest("POST", "/tags", bytes.NewReader(body))
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

	body := []byte(`{"name":"default"}`)
	req := httptest.NewRequest("POST", "/tags", bytes.NewReader(body))
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

	body := []byte(`{"name":"nonexistent","description":"x"}`)
	req := httptest.NewRequest("PUT", "/tags/nonexistent", bytes.NewReader(body))
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

	// Try to rename blue → green – should be rejected
	body := []byte(`{"name":"green","description":"x"}`)
	req := httptest.NewRequest("PUT", "/tags/blue", bytes.NewReader(body))
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

	body := map[string]interface{}{
		"name":       "Tagged",
		"statusCode": 200,
		"tag":        "blue",
	}
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

	body := map[string]interface{}{
		"name":       "Bad Tag",
		"statusCode": 200,
		"tag":        "nonexistent-tag",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-1/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
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
