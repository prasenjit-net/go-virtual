package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
)

func setupSpecBindingTest(t *testing.T) (*Handler, storage.Storage, func(*http.Request) *httptest.ResponseRecorder) {
	t.Helper()
	handler, store, r := setupTestHandler(t)
	r.GET("/specs/:id/scripts", handler.ListSpecScriptBindings)
	r.POST("/specs/:id/scripts", handler.CreateSpecScriptBinding)
	r.PUT("/specs/:id/scripts/:bindingId", handler.UpdateSpecScriptBinding)
	r.DELETE("/specs/:id/scripts/:bindingId", handler.DeleteSpecScriptBinding)
	r.PUT("/specs/:id/scripts/reorder", handler.ReorderSpecScriptBindings)

	do := func(req *http.Request) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	return handler, store, do
}

func seedSpecBindingData(t *testing.T, store storage.Storage) {
	t.Helper()
	if err := store.CreateSpec(&models.Spec{ID: "spec-1", Name: "Spec One"}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	if err := store.CreateScript(&models.Script{ID: "script-1", Name: "Script One", Source: "def run(req): return 1", Enabled: true}); err != nil {
		t.Fatalf("CreateScript: %v", err)
	}
}

func TestListSpecScriptBindings(t *testing.T) {
	_, store, do := setupSpecBindingTest(t)
	seedSpecBindingData(t, store)
	if err := store.CreateScriptBinding(&models.ScriptBinding{ID: "bind-1", SpecID: "spec-1", ScriptID: "script-1", OutputKey: "result", Order: 0, Enabled: true}); err != nil {
		t.Fatalf("CreateScriptBinding: %v", err)
	}

	w := do(httptest.NewRequest(http.MethodGet, "/specs/spec-1/scripts", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var bindings []models.ScriptBinding
	if err := json.Unmarshal(w.Body.Bytes(), &bindings); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(bindings) != 1 || bindings[0].ID != "bind-1" {
		t.Fatalf("unexpected bindings: %#v", bindings)
	}
}

func TestListSpecScriptBindings_NotFound(t *testing.T) {
	_, _, do := setupSpecBindingTest(t)
	w := do(httptest.NewRequest(http.MethodGet, "/specs/missing/scripts", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCreateSpecScriptBinding_Success(t *testing.T) {
	_, store, do := setupSpecBindingTest(t)
	seedSpecBindingData(t, store)

	req := httptest.NewRequest(http.MethodPost, "/specs/spec-1/scripts", bytes.NewBufferString(`{"scriptId":"script-1","outputKey":"payload","order":2,"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := do(req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	bindings, err := store.GetSpecScriptBindings("spec-1")
	if err != nil {
		t.Fatalf("GetSpecScriptBindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].OutputKey != "payload" || bindings[0].ScriptName != "Script One" {
		t.Fatalf("unexpected bindings: %#v", bindings)
	}
}

func TestCreateSpecScriptBinding_Validation(t *testing.T) {
	_, store, do := setupSpecBindingTest(t)
	seedSpecBindingData(t, store)

	t.Run("missing-script-id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/specs/spec-1/scripts", bytes.NewBufferString(`{"outputKey":"payload"}`))
		req.Header.Set("Content-Type", "application/json")
		w := do(req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("script-not-found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/specs/spec-1/scripts", bytes.NewBufferString(`{"scriptId":"missing","outputKey":"payload"}`))
		req.Header.Set("Content-Type", "application/json")
		w := do(req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestUpdateSpecScriptBinding_Success(t *testing.T) {
	_, store, do := setupSpecBindingTest(t)
	seedSpecBindingData(t, store)
	if err := store.CreateScriptBinding(&models.ScriptBinding{ID: "bind-1", SpecID: "spec-1", ScriptID: "script-1", OutputKey: "old", Order: 0, Enabled: true}); err != nil {
		t.Fatalf("CreateScriptBinding: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/specs/spec-1/scripts/bind-1", bytes.NewBufferString(`{"outputKey":"new","order":5,"enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	bindings, err := store.GetSpecScriptBindings("spec-1")
	if err != nil {
		t.Fatalf("GetSpecScriptBindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].OutputKey != "new" || bindings[0].Order != 5 || bindings[0].Enabled {
		t.Fatalf("unexpected bindings: %#v", bindings)
	}
}

func TestUpdateSpecScriptBinding_NotFound(t *testing.T) {
	_, store, do := setupSpecBindingTest(t)
	seedSpecBindingData(t, store)

	req := httptest.NewRequest(http.MethodPut, "/specs/spec-1/scripts/missing", bytes.NewBufferString(`{"outputKey":"new"}`))
	req.Header.Set("Content-Type", "application/json")
	w := do(req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteSpecScriptBinding_Success(t *testing.T) {
	_, store, do := setupSpecBindingTest(t)
	seedSpecBindingData(t, store)
	if err := store.CreateScriptBinding(&models.ScriptBinding{ID: "bind-1", SpecID: "spec-1", ScriptID: "script-1", OutputKey: "payload", Order: 0, Enabled: true}); err != nil {
		t.Fatalf("CreateScriptBinding: %v", err)
	}

	w := do(httptest.NewRequest(http.MethodDelete, "/specs/spec-1/scripts/bind-1", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	bindings, err := store.GetSpecScriptBindings("spec-1")
	if err != nil {
		t.Fatalf("GetSpecScriptBindings: %v", err)
	}
	if len(bindings) != 0 {
		t.Fatalf("expected bindings to be deleted, got %#v", bindings)
	}
}

func TestDeleteSpecScriptBinding_NotFound(t *testing.T) {
	_, store, do := setupSpecBindingTest(t)
	seedSpecBindingData(t, store)

	w := do(httptest.NewRequest(http.MethodDelete, "/specs/spec-1/scripts/missing", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestReorderSpecScriptBindings_Success(t *testing.T) {
	_, store, do := setupSpecBindingTest(t)
	seedSpecBindingData(t, store)
	if err := store.CreateScriptBinding(&models.ScriptBinding{ID: "bind-1", SpecID: "spec-1", ScriptID: "script-1", OutputKey: "a", Order: 0, Enabled: true}); err != nil {
		t.Fatalf("CreateScriptBinding bind-1: %v", err)
	}
	if err := store.CreateScriptBinding(&models.ScriptBinding{ID: "bind-2", SpecID: "spec-1", ScriptID: "script-1", OutputKey: "b", Order: 1, Enabled: true}); err != nil {
		t.Fatalf("CreateScriptBinding bind-2: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/specs/spec-1/scripts/reorder", bytes.NewBufferString(`[{"id":"bind-1","order":3},{"id":"bind-2","order":1}]`))
	req.Header.Set("Content-Type", "application/json")
	w := do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	bindings, err := store.GetSpecScriptBindings("spec-1")
	if err != nil {
		t.Fatalf("GetSpecScriptBindings: %v", err)
	}
	if len(bindings) != 2 || bindings[0].ID != "bind-2" || bindings[1].ID != "bind-1" {
		t.Fatalf("unexpected reordered bindings: %#v", bindings)
	}
}

func TestReorderSpecScriptBindings_NotFound(t *testing.T) {
	_, _, do := setupSpecBindingTest(t)
	req := httptest.NewRequest(http.MethodPut, "/specs/missing/scripts/reorder", bytes.NewBufferString(`[]`))
	req.Header.Set("Content-Type", "application/json")
	w := do(req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
