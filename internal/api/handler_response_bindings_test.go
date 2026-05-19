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

func setupResponseBindingTest(t *testing.T) (*Handler, storage.Storage, *httptest.ResponseRecorder, func(*http.Request) *httptest.ResponseRecorder) {
	t.Helper()
	handler, store, r := setupTestHandler(t)
	r.GET("/responses/:respId/scripts", handler.ListResponseScriptBindings)
	r.POST("/responses/:respId/scripts", handler.CreateResponseScriptBinding)
	r.PUT("/responses/:respId/scripts/:bindingId", handler.UpdateResponseScriptBinding)
	r.DELETE("/responses/:respId/scripts/:bindingId", handler.DeleteResponseScriptBinding)
	r.PUT("/responses/:respId/scripts/reorder", handler.ReorderResponseScriptBindings)

	do := func(req *http.Request) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	return handler, store, httptest.NewRecorder(), do
}

func seedResponseBindingData(t *testing.T, store storage.Storage) {
	t.Helper()
	if err := store.CreateResponseConfig(&models.ResponseConfig{ID: "resp-1", OperationID: "op-1", Name: "Primary", StatusCode: 200}); err != nil {
		t.Fatalf("CreateResponseConfig: %v", err)
	}
	if err := store.CreateScript(&models.Script{ID: "script-1", Name: "Script One", Source: "def run(req): return 1", Enabled: true}); err != nil {
		t.Fatalf("CreateScript: %v", err)
	}
}

func TestListResponseScriptBindings(t *testing.T) {
	_, store, _, do := setupResponseBindingTest(t)
	seedResponseBindingData(t, store)
	if err := store.CreateScriptBinding(&models.ScriptBinding{ID: "bind-1", ResponseConfigID: "resp-1", ScriptID: "script-1", OutputKey: "result", Order: 0, Enabled: true}); err != nil {
		t.Fatalf("CreateScriptBinding: %v", err)
	}

	w := do(httptest.NewRequest(http.MethodGet, "/responses/resp-1/scripts", nil))
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

func TestListResponseScriptBindings_NotFound(t *testing.T) {
	_, _, _, do := setupResponseBindingTest(t)
	w := do(httptest.NewRequest(http.MethodGet, "/responses/missing/scripts", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCreateResponseScriptBinding_Success(t *testing.T) {
	_, store, _, do := setupResponseBindingTest(t)
	seedResponseBindingData(t, store)

	req := httptest.NewRequest(http.MethodPost, "/responses/resp-1/scripts", bytes.NewBufferString(`{"scriptId":"script-1","outputKey":"payload","order":2,"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := do(req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	bindings, err := store.GetResponseScriptBindings("resp-1")
	if err != nil {
		t.Fatalf("GetResponseScriptBindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].OutputKey != "payload" || bindings[0].ScriptName != "Script One" {
		t.Fatalf("unexpected bindings: %#v", bindings)
	}
}

func TestCreateResponseScriptBinding_Validation(t *testing.T) {
	_, store, _, do := setupResponseBindingTest(t)
	seedResponseBindingData(t, store)

	t.Run("missing-output-key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/responses/resp-1/scripts", bytes.NewBufferString(`{"scriptId":"script-1"}`))
		req.Header.Set("Content-Type", "application/json")
		w := do(req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("script-not-found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/responses/resp-1/scripts", bytes.NewBufferString(`{"scriptId":"missing","outputKey":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		w := do(req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestUpdateResponseScriptBinding_Success(t *testing.T) {
	_, store, _, do := setupResponseBindingTest(t)
	seedResponseBindingData(t, store)
	if err := store.CreateScriptBinding(&models.ScriptBinding{ID: "bind-1", ResponseConfigID: "resp-1", ScriptID: "script-1", OutputKey: "old", Order: 0, Enabled: true}); err != nil {
		t.Fatalf("CreateScriptBinding: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/responses/resp-1/scripts/bind-1", bytes.NewBufferString(`{"outputKey":"new","order":5,"enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	bindings, err := store.GetResponseScriptBindings("resp-1")
	if err != nil {
		t.Fatalf("GetResponseScriptBindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].OutputKey != "new" || bindings[0].Order != 5 || bindings[0].Enabled {
		t.Fatalf("unexpected bindings: %#v", bindings)
	}
}

func TestUpdateResponseScriptBinding_NotFound(t *testing.T) {
	_, store, _, do := setupResponseBindingTest(t)
	seedResponseBindingData(t, store)

	req := httptest.NewRequest(http.MethodPut, "/responses/resp-1/scripts/missing", bytes.NewBufferString(`{"outputKey":"new"}`))
	req.Header.Set("Content-Type", "application/json")
	w := do(req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteResponseScriptBinding_Success(t *testing.T) {
	_, store, _, do := setupResponseBindingTest(t)
	seedResponseBindingData(t, store)
	if err := store.CreateScriptBinding(&models.ScriptBinding{ID: "bind-1", ResponseConfigID: "resp-1", ScriptID: "script-1", OutputKey: "payload", Order: 0, Enabled: true}); err != nil {
		t.Fatalf("CreateScriptBinding: %v", err)
	}

	w := do(httptest.NewRequest(http.MethodDelete, "/responses/resp-1/scripts/bind-1", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	bindings, err := store.GetResponseScriptBindings("resp-1")
	if err != nil {
		t.Fatalf("GetResponseScriptBindings: %v", err)
	}
	if len(bindings) != 0 {
		t.Fatalf("expected bindings to be deleted, got %#v", bindings)
	}
}

func TestDeleteResponseScriptBinding_NotFound(t *testing.T) {
	_, store, _, do := setupResponseBindingTest(t)
	seedResponseBindingData(t, store)

	w := do(httptest.NewRequest(http.MethodDelete, "/responses/resp-1/scripts/missing", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestReorderResponseScriptBindings_Success(t *testing.T) {
	_, store, _, do := setupResponseBindingTest(t)
	seedResponseBindingData(t, store)
	if err := store.CreateScriptBinding(&models.ScriptBinding{ID: "bind-1", ResponseConfigID: "resp-1", ScriptID: "script-1", OutputKey: "a", Order: 0, Enabled: true}); err != nil {
		t.Fatalf("CreateScriptBinding bind-1: %v", err)
	}
	if err := store.CreateScriptBinding(&models.ScriptBinding{ID: "bind-2", ResponseConfigID: "resp-1", ScriptID: "script-1", OutputKey: "b", Order: 1, Enabled: true}); err != nil {
		t.Fatalf("CreateScriptBinding bind-2: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/responses/resp-1/scripts/reorder", bytes.NewBufferString(`[{"id":"bind-1","order":3},{"id":"bind-2","order":1}]`))
	req.Header.Set("Content-Type", "application/json")
	w := do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	bindings, err := store.GetResponseScriptBindings("resp-1")
	if err != nil {
		t.Fatalf("GetResponseScriptBindings: %v", err)
	}
	if len(bindings) != 2 || bindings[0].ID != "bind-2" || bindings[1].ID != "bind-1" {
		t.Fatalf("unexpected reordered bindings: %#v", bindings)
	}
}

func TestReorderResponseScriptBindings_NotFound(t *testing.T) {
	_, _, _, do := setupResponseBindingTest(t)
	req := httptest.NewRequest(http.MethodPut, "/responses/missing/scripts/reorder", bytes.NewBufferString(`[]`))
	req.Header.Set("Content-Type", "application/json")
	w := do(req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
