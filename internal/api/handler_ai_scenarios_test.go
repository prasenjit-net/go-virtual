package api

import (
"bytes"
"encoding/json"
"net/http"
"net/http/httptest"
"testing"

"github.com/gin-gonic/gin"
)

func TestListAIScenarios_Empty(t *testing.T) {
handler, _, r := setupTestHandler(t)
r.GET("/ai/scenarios", handler.ListAIScenarios)

req := httptest.NewRequest("GET", "/ai/scenarios", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusOK {
t.Fatalf("expected 200, got %d", w.Code)
}
var body map[string]interface{}
if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
t.Fatalf("decode: %v", err)
}
}

func TestCreateAndListAIScenario(t *testing.T) {
handler, _, r := setupTestHandler(t)
r.POST("/ai/scenarios", handler.CreateAIScenario)
r.GET("/ai/scenarios", handler.ListAIScenarios)

body := bytes.NewBufferString(`{"scenario":{"name":"test","prompt":"generate","enabled":true}}`)
req := httptest.NewRequest("POST", "/ai/scenarios", body)
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusCreated {
t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
}

var created struct {
Scenario struct {
ID string `json:"id"`
} `json:"scenario"`
}
if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
t.Fatalf("decode: %v", err)
}
if created.Scenario.ID == "" {
t.Error("expected non-empty scenario ID")
}

// List should return it
req2 := httptest.NewRequest("GET", "/ai/scenarios", nil)
w2 := httptest.NewRecorder()
r.ServeHTTP(w2, req2)
if w2.Code != http.StatusOK {
t.Fatalf("list expected 200, got %d", w2.Code)
}
}

func TestCreateAIScenario_BadRequest(t *testing.T) {
handler, _, r := setupTestHandler(t)
r.POST("/ai/scenarios", handler.CreateAIScenario)

req := httptest.NewRequest("POST", "/ai/scenarios", bytes.NewBufferString(`not-json`))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusBadRequest {
t.Errorf("expected 400, got %d", w.Code)
}
}

func TestUpdateAndDeleteAIScenario(t *testing.T) {
gin.SetMode(gin.TestMode)
handler, _, r := setupTestHandler(t)
r.POST("/ai/scenarios", handler.CreateAIScenario)
r.PUT("/ai/scenarios/:scenarioId", handler.UpdateAIScenario)
r.DELETE("/ai/scenarios/:scenarioId", handler.DeleteAIScenario)

// Create first
body := bytes.NewBufferString(`{"scenario":{"name":"old","prompt":"old","enabled":true}}`)
req := httptest.NewRequest("POST", "/ai/scenarios", body)
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
r.ServeHTTP(w, req)
if w.Code != http.StatusCreated {
t.Fatalf("create: expected 201, got %d", w.Code)
}
var created struct {
Scenario struct{ ID string } `json:"scenario"`
}
json.NewDecoder(w.Body).Decode(&created)
id := created.Scenario.ID

// Update
upBody := bytes.NewBufferString(`{"scenario":{"name":"new","prompt":"updated","enabled":true}}`)
req2 := httptest.NewRequest("PUT", "/ai/scenarios/"+id, upBody)
req2.Header.Set("Content-Type", "application/json")
w2 := httptest.NewRecorder()
r.ServeHTTP(w2, req2)
if w2.Code != http.StatusOK {
t.Errorf("update: expected 200, got %d: %s", w2.Code, w2.Body.String())
}

// Delete
req3 := httptest.NewRequest("DELETE", "/ai/scenarios/"+id, nil)
w3 := httptest.NewRecorder()
r.ServeHTTP(w3, req3)
if w3.Code != http.StatusOK {
t.Errorf("delete: expected 204, got %d", w3.Code)
}
}

func TestUpdateAIScenario_BadJSON(t *testing.T) {
gin.SetMode(gin.TestMode)
handler, _, r := setupTestHandler(t)
r.POST("/ai/scenarios", handler.CreateAIScenario)
r.PUT("/ai/scenarios/:scenarioId", handler.UpdateAIScenario)

// Create first
body := bytes.NewBufferString(`{"scenario":{"name":"test","prompt":"test","enabled":true}}`)
req := httptest.NewRequest("POST", "/ai/scenarios", body)
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
r.ServeHTTP(w, req)
if w.Code != http.StatusCreated {
t.Fatalf("create: expected 201, got %d", w.Code)
}
var created struct {
Scenario struct{ ID string } `json:"scenario"`
}
json.NewDecoder(w.Body).Decode(&created)

// Update with bad JSON
req2 := httptest.NewRequest("PUT", "/ai/scenarios/"+created.Scenario.ID, bytes.NewBufferString("not json"))
req2.Header.Set("Content-Type", "application/json")
w2 := httptest.NewRecorder()
r.ServeHTTP(w2, req2)
if w2.Code != http.StatusBadRequest {
t.Errorf("expected 400, got %d: %s", w2.Code, w2.Body.String())
}
}

func TestCreateAIScenario_BadJSON(t *testing.T) {
gin.SetMode(gin.TestMode)
handler, _, r := setupTestHandler(t)
r.POST("/ai/scenarios", handler.CreateAIScenario)

req := httptest.NewRequest("POST", "/ai/scenarios", bytes.NewBufferString("not json"))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
r.ServeHTTP(w, req)
if w.Code != http.StatusBadRequest {
t.Errorf("expected 400, got %d", w.Code)
}
}
