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

// seedResponseConfig creates the prerequisite spec, operation, and response config.
func seedResponseConfig(t *testing.T) (*Handler, storage.Storage, *gin.Engine) {
	t.Helper()
	handler, store, r := setupTestHandler(t)
	_ = store.CreateSpec(&models.Spec{ID: "spec-1", Name: "S"})
	_ = store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/test"})
	_ = store.CreateResponseConfig(&models.ResponseConfig{ID: "rc-1", OperationID: "op-1", Name: "Default", StatusCode: 200})
	return handler, store, r
}

// ── ListCollectionMappings ────────────────────────────────────────────────────

func TestListCollectionMappings(t *testing.T) {
	handler, store, r := seedResponseConfig(t)
	_ = store.CreateCollectionMapping(&models.CollectionMapping{
		ID:               "cm-1",
		ResponseConfigID: "rc-1",
		CollectionName:   "users",
		Operation:        models.ColOpFindOne,
		OutputKey:        "user",
		Order:            0,
		Enabled:          true,
	})

	r.GET("/operations/:id/responses/:respId/mappings", handler.ListCollectionMappings)
	req := httptest.NewRequest("GET", "/operations/op-1/responses/rc-1/mappings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var mappings []models.CollectionMapping
	json.Unmarshal(w.Body.Bytes(), &mappings)
	if len(mappings) != 1 {
		t.Errorf("expected 1 mapping, got %d", len(mappings))
	}
}

func TestListCollectionMappings_Empty(t *testing.T) {
	handler, _, r := seedResponseConfig(t)

	r.GET("/operations/:id/responses/:respId/mappings", handler.ListCollectionMappings)
	req := httptest.NewRequest("GET", "/operations/op-1/responses/rc-1/mappings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var mappings []models.CollectionMapping
	json.Unmarshal(w.Body.Bytes(), &mappings)
	if len(mappings) != 0 {
		t.Errorf("expected 0 mappings, got %d", len(mappings))
	}
}

func TestListCollectionMappings_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/operations/:id/responses/:respId/mappings", handler.ListCollectionMappings)

	req := httptest.NewRequest("GET", "/operations/op-1/responses/nonexistent/mappings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── CreateCollectionMapping ───────────────────────────────────────────────────

func TestCreateCollectionMapping(t *testing.T) {
	handler, _, r := seedResponseConfig(t)
	r.POST("/operations/:id/responses/:respId/mappings", handler.CreateCollectionMapping)

	body := map[string]interface{}{
		"collectionName": "users",
		"outputKey":      "result",
		"operation":      string(models.ColOpFindMany),
		"enabled":        true,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-1/responses/rc-1/mappings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var cm models.CollectionMapping
	json.Unmarshal(w.Body.Bytes(), &cm)
	if cm.CollectionName != "users" {
		t.Errorf("expected collectionName=users, got %q", cm.CollectionName)
	}
	if cm.OutputKey != "result" {
		t.Errorf("expected outputKey=result, got %q", cm.OutputKey)
	}
	if cm.ResponseConfigID != "rc-1" {
		t.Errorf("expected responseConfigId=rc-1, got %q", cm.ResponseConfigID)
	}
	if cm.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCreateCollectionMapping_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/operations/:id/responses/:respId/mappings", handler.CreateCollectionMapping)

	body := map[string]interface{}{
		"collectionName": "users",
		"outputKey":      "result",
		"operation":      "find-one",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-1/responses/nonexistent/mappings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCreateCollectionMapping_MissingCollectionName(t *testing.T) {
	handler, _, r := seedResponseConfig(t)
	r.POST("/operations/:id/responses/:respId/mappings", handler.CreateCollectionMapping)

	body := map[string]interface{}{
		"outputKey": "result",
		"operation": "find-one",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-1/responses/rc-1/mappings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing collectionName, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateCollectionMapping_MissingOutputKey(t *testing.T) {
	handler, _, r := seedResponseConfig(t)
	r.POST("/operations/:id/responses/:respId/mappings", handler.CreateCollectionMapping)

	body := map[string]interface{}{
		"collectionName": "users",
		"operation":      "find-one",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-1/responses/rc-1/mappings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing outputKey, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateCollectionMapping_MissingOperation(t *testing.T) {
	handler, _, r := seedResponseConfig(t)
	r.POST("/operations/:id/responses/:respId/mappings", handler.CreateCollectionMapping)

	body := map[string]interface{}{
		"collectionName": "users",
		"outputKey":      "result",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/operations/op-1/responses/rc-1/mappings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing operation, got %d: %s", w.Code, w.Body.String())
	}
}

// ── GetCollectionMapping ──────────────────────────────────────────────────────

func TestGetCollectionMapping(t *testing.T) {
	handler, store, r := seedResponseConfig(t)
	_ = store.CreateCollectionMapping(&models.CollectionMapping{
		ID:               "cm-1",
		ResponseConfigID: "rc-1",
		CollectionName:   "orders",
		Operation:        models.ColOpFindOne,
		OutputKey:        "order",
	})

	r.GET("/mappings/:mappingId", handler.GetCollectionMapping)
	req := httptest.NewRequest("GET", "/mappings/cm-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var cm models.CollectionMapping
	json.Unmarshal(w.Body.Bytes(), &cm)
	if cm.CollectionName != "orders" {
		t.Errorf("expected collectionName=orders, got %q", cm.CollectionName)
	}
}

func TestGetCollectionMapping_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/mappings/:mappingId", handler.GetCollectionMapping)
	req := httptest.NewRequest("GET", "/mappings/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── UpdateCollectionMapping ───────────────────────────────────────────────────

func TestUpdateCollectionMapping(t *testing.T) {
	handler, store, r := seedResponseConfig(t)
	_ = store.CreateCollectionMapping(&models.CollectionMapping{
		ID:               "cm-1",
		ResponseConfigID: "rc-1",
		CollectionName:   "old",
		Operation:        models.ColOpFindOne,
		OutputKey:        "x",
	})

	r.PUT("/mappings/:mappingId", handler.UpdateCollectionMapping)

	body := map[string]interface{}{
		"collectionName": "new",
		"operation":      string(models.ColOpFindMany),
		"outputKey":      "y",
		"enabled":        true,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/mappings/cm-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var cm models.CollectionMapping
	json.Unmarshal(w.Body.Bytes(), &cm)
	if cm.CollectionName != "new" {
		t.Errorf("expected collectionName=new, got %q", cm.CollectionName)
	}
	if cm.OutputKey != "y" {
		t.Errorf("expected outputKey=y, got %q", cm.OutputKey)
	}
}

func TestUpdateCollectionMapping_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.PUT("/mappings/:mappingId", handler.UpdateCollectionMapping)

	body := map[string]interface{}{"collectionName": "x", "operation": "find-one", "outputKey": "y"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/mappings/nonexistent", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── DeleteCollectionMapping ───────────────────────────────────────────────────

func TestDeleteCollectionMapping(t *testing.T) {
	handler, store, r := seedResponseConfig(t)
	_ = store.CreateCollectionMapping(&models.CollectionMapping{
		ID:               "cm-1",
		ResponseConfigID: "rc-1",
		CollectionName:   "toDelete",
		Operation:        models.ColOpInsert,
		OutputKey:        "x",
	})

	r.DELETE("/mappings/:mappingId", handler.DeleteCollectionMapping)
	req := httptest.NewRequest("DELETE", "/mappings/cm-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify deleted
	_, err := store.GetCollectionMapping("cm-1")
	if err == nil {
		t.Error("expected mapping to be deleted")
	}
}

func TestDeleteCollectionMapping_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.DELETE("/mappings/:mappingId", handler.DeleteCollectionMapping)
	req := httptest.NewRequest("DELETE", "/mappings/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
