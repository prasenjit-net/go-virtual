package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/archive"
	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

type multipartBody struct {
	body        *bytes.Buffer
	contentType string
}

func makeMultipartZIP(t *testing.T, zipData []byte) multipartBody {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("archive", "snapshot.zip")
	if err != nil {
		t.Fatalf("createFormFile: %v", err)
	}
	if _, err := fw.Write(zipData); err != nil {
		t.Fatalf("write: %v", err)
	}
	mw.Close()
	return multipartBody{body: &buf, contentType: mw.FormDataContentType()}
}

// setupTestHandlerWithArchives creates a Handler with an ArchiveManager wired in.
func setupTestHandlerWithArchives(t *testing.T) (*Handler, storage.Storage, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	stor := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)
	proxyEngine := proxy.NewEngine(stor, collector, tracingSvc)

	gs, err := store.NewGlobalStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("NewGlobalStore: %v", err)
	}
	am, err := archive.NewArchiveManager(t.TempDir(), stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}

	handler := NewHandler(HandlerConfig{
		Store:          stor,
		StatsCollector: collector,
		TracingService: tracingSvc,
		ProxyEngine:    proxyEngine,
		ArchiveManager: am,
	})

	r := gin.New()
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

// ── SnapshotArchiveManager helpers ───────────────────────────────────────────

func setupTestHandlerWithSnapshot(t *testing.T) (*Handler, storage.Storage, *gin.Engine) {
t.Helper()
gin.SetMode(gin.TestMode)

stor := storage.NewMemoryStorage()
collector := stats.NewCollector()
tracingSvc := tracing.NewService(100)
proxyEngine := proxy.NewEngine(stor, collector, tracingSvc)

gs, err := store.NewGlobalStore(filepath.Join(t.TempDir(), "store.json"))
if err != nil {
t.Fatalf("NewGlobalStore: %v", err)
}
sam := archive.NewSnapshotArchiveManager(stor, gs)

handler := NewHandler(HandlerConfig{
Store:          stor,
StatsCollector: collector,
TracingService: tracingSvc,
ProxyEngine:    proxyEngine,
ArchiveManager: sam,
})

r := gin.New()
return handler, stor, r
}

// ── ArchiveInfo ───────────────────────────────────────────────────────────────

func TestArchiveInfo_FullMode(t *testing.T) {
handler, _, r := setupTestHandlerWithArchives(t)
r.GET("/archives/info", handler.ArchiveInfo)

req := httptest.NewRequest("GET", "/archives/info", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusOK {
t.Fatalf("expected 200, got %d", w.Code)
}
var body map[string]string
if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
t.Fatalf("decode: %v", err)
}
if body["mode"] != "full" {
t.Errorf("expected mode=full, got %q", body["mode"])
}
}

func TestArchiveInfo_SnapshotMode(t *testing.T) {
handler, _, r := setupTestHandlerWithSnapshot(t)
r.GET("/archives/info", handler.ArchiveInfo)

req := httptest.NewRequest("GET", "/archives/info", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusOK {
t.Fatalf("expected 200, got %d", w.Code)
}
var body map[string]string
if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
t.Fatalf("decode: %v", err)
}
if body["mode"] != "snapshot" {
t.Errorf("expected mode=snapshot, got %q", body["mode"])
}
}

func TestArchiveInfo_NoManager(t *testing.T) {
handler, _, r := setupTestHandler(t)
r.GET("/archives/info", handler.ArchiveInfo)

req := httptest.NewRequest("GET", "/archives/info", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusServiceUnavailable {
t.Errorf("expected 503, got %d", w.Code)
}
}

// ── Snapshot mode 405s ────────────────────────────────────────────────────────

func TestListArchives_SnapshotMode_Returns405(t *testing.T) {
handler, _, r := setupTestHandlerWithSnapshot(t)
r.GET("/archives", handler.ListArchives)

req := httptest.NewRequest("GET", "/archives", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusMethodNotAllowed {
t.Errorf("expected 405 in snapshot mode, got %d", w.Code)
}
}

func TestCreateArchive_SnapshotMode_Returns405(t *testing.T) {
handler, _, r := setupTestHandlerWithSnapshot(t)
r.POST("/archives", handler.CreateArchive)

req := httptest.NewRequest("POST", "/archives", bytes.NewBufferString(`{}`))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusMethodNotAllowed {
t.Errorf("expected 405 in snapshot mode, got %d", w.Code)
}
}

// ── DownloadSnapshot ──────────────────────────────────────────────────────────

func TestDownloadSnapshot_SnapshotMode(t *testing.T) {
handler, _, r := setupTestHandlerWithSnapshot(t)
r.GET("/archives/snapshot", handler.DownloadSnapshot)

req := httptest.NewRequest("GET", "/archives/snapshot", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
}
if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
t.Errorf("expected application/zip, got %q", ct)
}
}

func TestDownloadSnapshot_NoManager(t *testing.T) {
handler, _, r := setupTestHandler(t)
r.GET("/archives/snapshot", handler.DownloadSnapshot)

req := httptest.NewRequest("GET", "/archives/snapshot", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusServiceUnavailable {
t.Errorf("expected 503, got %d", w.Code)
}
}

func TestDownloadSnapshot_FullMode_Returns405(t *testing.T) {
handler, _, r := setupTestHandlerWithArchives(t)
r.GET("/archives/snapshot", handler.DownloadSnapshot)

req := httptest.NewRequest("GET", "/archives/snapshot", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusMethodNotAllowed {
t.Errorf("expected 405 in full mode, got %d", w.Code)
}
}

// ── RestoreSnapshot ───────────────────────────────────────────────────────────

func TestRestoreSnapshot_SnapshotMode(t *testing.T) {
// First download from a seeded instance, then restore into a fresh one.
_, srcStor, _ := setupTestHandlerWithSnapshot(t)

gs1, _ := store.NewGlobalStore(filepath.Join(t.TempDir(), "s1.json"))
sam1 := archive.NewSnapshotArchiveManager(srcStor, gs1)
zipBytes, _, err := sam1.DownloadSnapshot()
if err != nil {
t.Fatalf("DownloadSnapshot: %v", err)
}

// Set up a fresh handler and restore into it
handler, _, r := setupTestHandlerWithSnapshot(t)
r.POST("/archives/snapshot/restore", handler.RestoreSnapshot)

body := makeMultipartZIP(t, zipBytes)
req := httptest.NewRequest("POST", "/archives/snapshot/restore", body.body)
req.Header.Set("Content-Type", body.contentType)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
}
}

func TestRestoreSnapshot_NoManager(t *testing.T) {
handler, _, r := setupTestHandler(t)
r.POST("/archives/snapshot/restore", handler.RestoreSnapshot)

req := httptest.NewRequest("POST", "/archives/snapshot/restore", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusServiceUnavailable {
t.Errorf("expected 503, got %d", w.Code)
}
}

func TestRestoreSnapshot_FullMode_Returns405(t *testing.T) {
handler, _, r := setupTestHandlerWithArchives(t)
r.POST("/archives/snapshot/restore", handler.RestoreSnapshot)

req := httptest.NewRequest("POST", "/archives/snapshot/restore", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusMethodNotAllowed {
t.Errorf("expected 405 in full mode, got %d", w.Code)
}
}

func TestRestoreSnapshot_MissingFile(t *testing.T) {
handler, _, r := setupTestHandlerWithSnapshot(t)
r.POST("/archives/snapshot/restore", handler.RestoreSnapshot)

// Send multipart form without an "archive" field
var buf bytes.Buffer
mw := multipart.NewWriter(&buf)
mw.Close()
req := httptest.NewRequest("POST", "/archives/snapshot/restore", &buf)
req.Header.Set("Content-Type", mw.FormDataContentType())
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusBadRequest {
t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
}
}

func TestRestoreSnapshot_CorruptZIP(t *testing.T) {
handler, _, r := setupTestHandlerWithSnapshot(t)
r.POST("/archives/snapshot/restore", handler.RestoreSnapshot)

body := makeMultipartZIP(t, []byte("not-a-zip"))
req := httptest.NewRequest("POST", "/archives/snapshot/restore", body.body)
req.Header.Set("Content-Type", body.contentType)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

// Should fail with 422 or 500 — not 200
if w.Code == http.StatusOK {
t.Errorf("expected error response for corrupt ZIP, got 200")
}
}

func TestUploadArchive_Success(t *testing.T) {
// Build a valid ZIP from a seeded storage, then upload it
srcStor := storage.NewMemoryStorage()
gs1, err := store.NewGlobalStore(filepath.Join(t.TempDir(), "s1.json"))
if err != nil {
t.Fatalf("NewGlobalStore: %v", err)
}
am1, err := archive.NewArchiveManager(t.TempDir(), srcStor, gs1)
if err != nil {
t.Fatalf("NewArchiveManager: %v", err)
}
meta, err := am1.Create("upload-test")
if err != nil {
t.Fatalf("Create: %v", err)
}
path, err := am1.FilePath(meta.ID)
if err != nil {
t.Fatalf("FilePath: %v", err)
}
zipData, err := os.ReadFile(path)
if err != nil {
t.Fatalf("ReadFile: %v", err)
}

handler, _, r := setupTestHandlerWithArchives(t)
r.POST("/archives/upload", handler.UploadArchive)

body := makeMultipartZIP(t, zipData)
req := httptest.NewRequest("POST", "/archives/upload", body.body)
req.Header.Set("Content-Type", body.contentType)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusCreated {
t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
}
}

func TestUploadArchive_SnapshotMode_Returns405(t *testing.T) {
handler, _, r := setupTestHandlerWithSnapshot(t)
r.POST("/archives/upload", handler.UploadArchive)

req := httptest.NewRequest("POST", "/archives/upload", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusMethodNotAllowed {
t.Errorf("expected 405 in snapshot mode, got %d", w.Code)
}
}

func TestDownloadArchive_SnapshotMode_Returns405(t *testing.T) {
handler, _, r := setupTestHandlerWithSnapshot(t)
r.GET("/archives/:id/download", handler.DownloadArchive)

req := httptest.NewRequest("GET", "/archives/some-id/download", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusMethodNotAllowed {
t.Errorf("expected 405 in snapshot mode, got %d", w.Code)
}
}

func TestGetArchive_SnapshotMode_Returns405(t *testing.T) {
handler, _, r := setupTestHandlerWithSnapshot(t)
r.GET("/archives/:id", handler.GetArchive)

req := httptest.NewRequest("GET", "/archives/some-id", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusMethodNotAllowed {
t.Errorf("expected 405 in snapshot mode, got %d", w.Code)
}
}

func TestDeleteArchive_SnapshotMode_Returns405(t *testing.T) {
handler, _, r := setupTestHandlerWithSnapshot(t)
r.DELETE("/archives/:id", handler.DeleteArchive)

req := httptest.NewRequest("DELETE", "/archives/some-id", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusMethodNotAllowed {
t.Errorf("expected 405 in snapshot mode, got %d", w.Code)
}
}

func TestRestoreArchive_SnapshotMode_Returns405(t *testing.T) {
handler, _, r := setupTestHandlerWithSnapshot(t)
r.POST("/archives/:id/restore", handler.RestoreArchive)

req := httptest.NewRequest("POST", "/archives/some-id/restore", bytes.NewBufferString(`{}`))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

if w.Code != http.StatusMethodNotAllowed {
t.Errorf("expected 405 in snapshot mode, got %d", w.Code)
}
}

func TestDownloadArchive_Success(t *testing.T) {
handler, _, r := setupTestHandlerWithArchives(t)
r.POST("/archives", handler.CreateArchive)
r.GET("/archives/:id/download", handler.DownloadArchive)

// Create an archive first
req := httptest.NewRequest("POST", "/archives", bytes.NewBufferString(`{"label":"test-dl"}`))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
r.ServeHTTP(w, req)
if w.Code != http.StatusCreated {
t.Fatalf("create: expected 201, got %d", w.Code)
}
var meta struct {
ID string `json:"id"`
}
if err := json.NewDecoder(w.Body).Decode(&meta); err != nil {
t.Fatalf("decode create: %v", err)
}

// Download it
req2 := httptest.NewRequest("GET", "/archives/"+meta.ID+"/download", nil)
w2 := httptest.NewRecorder()
r.ServeHTTP(w2, req2)

if w2.Code != http.StatusOK {
t.Errorf("download: expected 200, got %d: %s", w2.Code, w2.Body.String())
}
}
