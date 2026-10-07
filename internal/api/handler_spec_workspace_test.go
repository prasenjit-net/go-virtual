package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

func TestGetSpecWorkspaceBundle(t *testing.T) {
	h, s, _ := setupTestHandler(t)
	if err := s.CreateSpec(&models.Spec{ID: "spec-workspace", Name: "Workspace", Content: "openapi: 3.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateOperation(&models.Operation{ID: "op-workspace", SpecID: "spec-workspace", Method: "GET", Path: "/users"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateResponseConfig(&models.ResponseConfig{ID: "response-workspace", OperationID: "op-workspace", Name: "OK", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateCollectionMapping(&models.CollectionMapping{ID: "mapping-workspace", SpecID: "spec-workspace", CollectionName: "users", Operation: models.ColOpFindOne, OutputKey: "user", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.GET("/specs/:id/workspace", h.GetSpecWorkspace)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/specs/spec-workspace/workspace", nil))
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var workspace SpecWorkspace
	if err := json.Unmarshal(w.Body.Bytes(), &workspace); err != nil {
		t.Fatal(err)
	}
	if workspace.Revision == "" || workspace.Spec.ID != "spec-workspace" || len(workspace.Operations) != 1 {
		t.Fatalf("incomplete workspace: %+v", workspace)
	}
	if len(workspace.Operations[0].Responses) != 1 || workspace.Operations[0].Responses[0].Response.ID != "response-workspace" {
		t.Fatalf("response missing from workspace: %+v", workspace.Operations[0])
	}
	if len(workspace.SpecMappings) != 1 || workspace.SpecMappings[0].ID != "mapping-workspace" {
		t.Fatalf("spec mapper missing from workspace: %+v", workspace.SpecMappings)
	}
	if w.Header().Get("ETag") == "" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("missing workspace headers: %v", w.Header())
	}
}

func TestGetSpecWorkspaceMissingSpec(t *testing.T) {
	h, _, _ := setupTestHandler(t)
	r := gin.New()
	r.GET("/specs/:id/workspace", h.GetSpecWorkspace)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/specs/missing/workspace", nil))
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSaveSpecWorkspaceAndRejectStaleRevision(t *testing.T) {
	h, s, r, _ := setupCollectionResponseTestHandler(t)
	r.GET("/specs/:id/workspace", h.GetSpecWorkspace)
	r.PUT("/specs/:id/workspace", h.SaveSpecWorkspace)
	loaded := httptest.NewRecorder()
	r.ServeHTTP(loaded, httptest.NewRequest(http.MethodGet, "/specs/spec-1/workspace", nil))
	var workspace models.SpecWorkspace
	if err := json.Unmarshal(loaded.Body.Bytes(), &workspace); err != nil {
		t.Fatal(err)
	}
	// Response configuration is carried by the workspace's Responses collection.
	// The operation endpoint may also hydrate this derived field, and it must not
	// be treated as an attempted OpenAPI-contract edit during workspace saving.
	workspace.Operations[0].Operation.Responses = []models.ResponseConfig{{ID: "derived-response", OperationID: "op-user", Name: "Derived"}}
	workspace.Spec.Name = "Edited in designer"
	draftResponseID := "draft-response-new"
	draftMappingID := "draft-mapping-new"
	workspace.Operations[0].Responses = append(workspace.Operations[0].Responses, models.SpecWorkspaceResponse{
		Response: &models.ResponseConfig{ID: draftResponseID, OperationID: "op-user", Name: "Draft response", Tag: "default", StatusCode: 201, Headers: map[string]string{}, Enabled: true, Kind: models.ResponseKindManual},
		Scripts:  []*models.ScriptBinding{},
		Mappings: []*models.CollectionMapping{{ID: draftMappingID, ResponseConfigID: draftResponseID, Name: "Created user", CollectionName: "users", Operation: models.ColOpFindOne, OutputKey: "createdUser", Enabled: true}},
	})
	body, err := json.Marshal(workspace)
	if err != nil {
		t.Fatal(err)
	}
	saved := httptest.NewRecorder()
	r.ServeHTTP(saved, httptest.NewRequest(http.MethodPut, "/specs/spec-1/workspace", bytes.NewReader(body)))
	if saved.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", saved.Code, saved.Body.String())
	}
	var result struct {
		Workspace models.SpecWorkspace `json:"workspace"`
		IDMap     map[string]string    `json:"idMap"`
	}
	if err := json.Unmarshal(saved.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.IDMap[draftResponseID] == "" || result.IDMap[draftMappingID] == "" {
		t.Fatalf("temporary IDs were not resolved: %+v", result.IDMap)
	}
	var savedResponse *models.ResponseConfig
	for _, response := range result.Workspace.Operations[0].Responses {
		if response.Response.ID == result.IDMap[draftResponseID] {
			savedResponse = response.Response
		}
	}
	if savedResponse == nil || len(result.Workspace.Operations[0].Responses) != 1 {
		t.Fatalf("new response missing from saved workspace: %+v", result.Workspace.Operations[0].Responses)
	}
	mappings, err := s.GetCollectionMappingsByResponse(savedResponse.ID)
	if err != nil || len(mappings) != 1 || mappings[0].ID != result.IDMap[draftMappingID] {
		t.Fatalf("response mapper was not saved under its new response: %+v, %v", mappings, err)
	}
	stored, err := s.GetSpec("spec-1")
	if err != nil || stored.Name != "Edited in designer" {
		t.Fatalf("workspace change was not saved: %+v, %v", stored, err)
	}
	stale := httptest.NewRecorder()
	r.ServeHTTP(stale, httptest.NewRequest(http.MethodPut, "/specs/spec-1/workspace", bytes.NewReader(body)))
	if stale.Code != http.StatusConflict {
		t.Fatalf("expected stale save to conflict, got %d: %s", stale.Code, stale.Body.String())
	}
	if stored.Name != "Edited in designer" {
		t.Fatal("stale save overwrote the server workspace")
	}
}
