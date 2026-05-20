package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
)

func TestTestScriptSource_Success(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/scripts/test-source", handler.TestScriptSource)

	body := map[string]any{
		"source":  "def run(req):\n    return {\"path\": req.path(\"id\", \"\"), \"body\": req.body(\"name\", \"\")}",
		"timeout": 100,
		"input": map[string]any{
			"path":   map[string]string{"id": "42"},
			"query":  map[string]string{},
			"header": map[string]string{},
			"body":   map[string]any{"name": "alice"},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/scripts/test-source", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	output, ok := result["output"].(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %T", result["output"])
	}
	if output["path"] != "42" || output["body"] != "alice" {
		t.Fatalf("unexpected output: %#v", output)
	}
	if result["error"] != nil {
		t.Fatalf("expected nil error, got %v", result["error"])
	}
}

func TestTestScriptSource_ValidationAndRuntimeErrors(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/scripts/test-source", handler.TestScriptSource)

	t.Run("missing-source", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/scripts/test-source", bytes.NewBufferString(`{"timeout":100}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("runtime-error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/scripts/test-source", bytes.NewBufferString(`{"source":"def run(req):\n    return 1 // 0","timeout":100}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if result["error"] == nil || result["error"] == "" {
			t.Fatalf("expected runtime error, got %#v", result)
		}
		if result["output"] != nil {
			t.Fatalf("expected nil output on error, got %#v", result["output"])
		}
	})
}

func TestTestScript_HandlerErrors(t *testing.T) {
	handler, store, r := setupTestHandler(t)
	r.POST("/scripts/:id/test", handler.TestScript)

	t.Run("not-found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/scripts/missing/test", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})

	t.Run("bad-json", func(t *testing.T) {
		script := &models.Script{ID: "script-json", Name: "JSON", Source: "def run(req): return 1", Timeout: 100, Enabled: true, UpdatedAt: time.Now()}
		if err := store.CreateScript(script); err != nil {
			t.Fatalf("CreateScript: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/scripts/script-json/test", bytes.NewBufferString(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("runtime-error", func(t *testing.T) {
		script := &models.Script{ID: "script-runtime", Name: "Runtime", Source: "def run(req):\n    return 1 // 0", Timeout: 100, Enabled: true, UpdatedAt: time.Now()}
		if err := store.CreateScript(script); err != nil {
			t.Fatalf("CreateScript: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/scripts/script-runtime/test", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if result["error"] == nil || result["error"] == "" {
			t.Fatalf("expected runtime error, got %#v", result)
		}
		if result["output"] != nil {
			t.Fatalf("expected nil output on error, got %#v", result["output"])
		}
	})
}
