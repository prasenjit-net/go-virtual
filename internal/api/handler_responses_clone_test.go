package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
)

func TestCloneResponseConfig_Success(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	r.POST("/responses/:id/clone", handler.CloneResponseConfig)

	source := &models.ResponseConfig{
		ID:          "resp-1",
		OperationID: "op-1",
		Name:        "Recorded response",
		Description: "captured from proxy",
		Tag:         "blue",
		Priority:    7,
		Conditions: []models.Condition{
			{Source: models.SourceSignature, Key: "", Operator: models.OpEquals, Value: "abc123"},
			{Source: models.SourceQuery, Key: "status", Operator: models.OpEquals, Value: "ok"},
		},
		StatusCode: 201,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       `{"ok":true}`,
		Delay:      20,
		Enabled:    true,
		Recorded:   true,
		Origin:     models.ResponseOriginProxy,
	}
	if err := store.CreateResponseConfig(source); err != nil {
		t.Fatalf("CreateResponseConfig: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/responses/resp-1/clone", bytes.NewBufferString(`{"name":"Manual clone"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var clone models.ResponseConfig
	if err := json.Unmarshal(w.Body.Bytes(), &clone); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if clone.ID == "" || clone.ID == source.ID {
		t.Fatalf("expected a new clone ID, got %q", clone.ID)
	}
	if clone.Name != "Manual clone" {
		t.Fatalf("expected clone name to be updated, got %q", clone.Name)
	}
	if clone.Origin != models.ResponseOriginManual || clone.Recorded {
		t.Fatalf("expected manual non-recorded clone, got origin=%q recorded=%v", clone.Origin, clone.Recorded)
	}
	if len(clone.Conditions) != 1 || clone.Conditions[0].Source != models.SourceQuery {
		t.Fatalf("expected signature conditions to be stripped, got %#v", clone.Conditions)
	}
	if clone.Headers["Content-Type"] != "application/json" || clone.Body != source.Body {
		t.Fatalf("expected headers/body to be copied, got %#v", clone)
	}

	configs, err := store.GetResponseConfigsByOperation("op-1")
	if err != nil {
		t.Fatalf("GetResponseConfigsByOperation: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 response configs after clone, got %d", len(configs))
	}
}

func TestCloneResponseConfig_NotFound(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/responses/:id/clone", handler.CloneResponseConfig)

	req := httptest.NewRequest(http.MethodPost, "/responses/missing/clone", bytes.NewBufferString(`{"name":"Clone"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCloneResponseConfig_MissingName(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	r.POST("/responses/:id/clone", handler.CloneResponseConfig)

	if err := store.CreateResponseConfig(&models.ResponseConfig{ID: "resp-1", OperationID: "op-1", Name: "Source", StatusCode: 200}); err != nil {
		t.Fatalf("CreateResponseConfig: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/responses/resp-1/clone", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
