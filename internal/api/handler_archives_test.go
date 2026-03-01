package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/archive"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
)

// setupTestHandlerWithArchives creates a Handler with an ArchiveManager wired in.
func setupTestHandlerWithArchives(t *testing.T) (*Handler, storage.Storage, *gin.Engine) {
	t.Helper()
	handler, stor, r := setupTestHandler(t)

	gs, err := store.NewGlobalStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("NewGlobalStore: %v", err)
	}
	am, err := archive.NewArchiveManager(t.TempDir(), stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}
	handler.SetArchiveManager(am)
	return handler, stor, r
}

// ── ListArchives ──────────────────────────────────────────────────────────────

func TestListArchives_NoManager(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/archives", handler.ListArchives)

	req := httptest.NewRequest("GET", "/archives", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestListArchives_Empty(t *testing.T) {
	handler, _, r := setupTestHandlerWithArchives(t)
	r.GET("/archives", handler.ListArchives)

	req := httptest.NewRequest("GET", "/archives", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var list []interface{}
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

// ── CreateArchive ─────────────────────────────────────────────────────────────

func TestCreateArchive_NoManager(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/archives", handler.CreateArchive)

	req := httptest.NewRequest("POST", "/archives", bytes.NewBufferString(`{"label":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestCreateArchive_Success(t *testing.T) {
	handler, _, r := setupTestHandlerWithArchives(t)
	r.POST("/archives", handler.CreateArchive)

	req := httptest.NewRequest("POST", "/archives", bytes.NewBufferString(`{"label":"my-snapshot"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var meta map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&meta); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if meta["label"] != "my-snapshot" {
		t.Errorf("label = %v, want 'my-snapshot'", meta["label"])
	}
	if meta["id"] == "" {
		t.Error("expected non-empty id")
	}
}

// ── GetArchive ────────────────────────────────────────────────────────────────

func TestGetArchive_Success(t *testing.T) {
	handler, _, r := setupTestHandlerWithArchives(t)
	r.POST("/archives", handler.CreateArchive)
	r.GET("/archives/:id", handler.GetArchive)

	req := httptest.NewRequest("POST", "/archives", bytes.NewBufferString(`{"label":"get-test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var meta map[string]interface{}
	json.NewDecoder(w.Body).Decode(&meta)
	id := meta["id"].(string)

	req2 := httptest.NewRequest("GET", "/archives/"+id, nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}

func TestGetArchive_NotFound(t *testing.T) {
	handler, _, r := setupTestHandlerWithArchives(t)
	r.GET("/archives/:id", handler.GetArchive)

	req := httptest.NewRequest("GET", "/archives/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── DeleteArchive ─────────────────────────────────────────────────────────────

func TestDeleteArchive_Success(t *testing.T) {
	handler, _, r := setupTestHandlerWithArchives(t)
	r.POST("/archives", handler.CreateArchive)
	r.DELETE("/archives/:id", handler.DeleteArchive)

	req := httptest.NewRequest("POST", "/archives", bytes.NewBufferString(`{"label":"to-delete"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var meta map[string]interface{}
	json.NewDecoder(w.Body).Decode(&meta)
	id := meta["id"].(string)

	req2 := httptest.NewRequest("DELETE", "/archives/"+id, nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w2.Code)
	}
}

func TestDeleteArchive_NotFound(t *testing.T) {
	handler, _, r := setupTestHandlerWithArchives(t)
	r.DELETE("/archives/:id", handler.DeleteArchive)

	req := httptest.NewRequest("DELETE", "/archives/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── RestoreArchive ────────────────────────────────────────────────────────────

func TestRestoreArchive_NoManager(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/archives/:id/restore", handler.RestoreArchive)

	req := httptest.NewRequest("POST", "/archives/some-id/restore", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestRestoreArchive_Success(t *testing.T) {
	handler, _, r := setupTestHandlerWithArchives(t)
	r.POST("/archives", handler.CreateArchive)
	r.POST("/archives/:id/restore", handler.RestoreArchive)

	req := httptest.NewRequest("POST", "/archives", bytes.NewBufferString(`{"label":"snap"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var meta map[string]interface{}
	json.NewDecoder(w.Body).Decode(&meta)
	id := meta["id"].(string)

	input := `{"createBackupFirst":false,"wipeFirst":false}`
	req2 := httptest.NewRequest("POST", "/archives/"+id+"/restore", bytes.NewBufferString(input))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestRestoreArchive_NotFound(t *testing.T) {
	handler, _, r := setupTestHandlerWithArchives(t)
	r.POST("/archives/:id/restore", handler.RestoreArchive)

	req := httptest.NewRequest("POST", "/archives/nonexistent/restore", bytes.NewBufferString(`{"wipeFirst":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// ── DownloadArchive / UploadArchive ───────────────────────────────────────────

func TestDownloadArchive_NotFound(t *testing.T) {
	handler, _, r := setupTestHandlerWithArchives(t)
	r.GET("/archives/:id/download", handler.DownloadArchive)

	req := httptest.NewRequest("GET", "/archives/nonexistent/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUploadArchive_NoManager(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.POST("/archives/upload", handler.UploadArchive)

	req := httptest.NewRequest("POST", "/archives/upload", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}
