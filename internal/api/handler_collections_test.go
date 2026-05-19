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

func TestCollectionGuard_NoStore(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "name", Value: "users"}}

	name, ok := handler.collectionGuard(c)
	if ok {
		t.Fatal("expected collectionGuard to fail without a store")
	}
	if name != "" {
		t.Fatalf("expected empty name, got %q", name)
	}
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestCollectionGuard_EmptyName(t *testing.T) {
	handler, _, _, _ := setupTestHandlerWithStore(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "name", Value: "   "}}

	name, ok := handler.collectionGuard(c)
	if ok {
		t.Fatal("expected collectionGuard to reject empty collection name")
	}
	if name != "" {
		t.Fatalf("expected empty name, got %q", name)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestParseIndexParam(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "index", Value: "3"}}

		idx, ok := parseIndexParam(c, "index")
		if !ok {
			t.Fatal("expected parseIndexParam to succeed")
		}
		if idx != 3 {
			t.Fatalf("expected 3, got %d", idx)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "index", Value: "abc"}}

		_, ok := parseIndexParam(c, "index")
		if ok {
			t.Fatal("expected parseIndexParam to fail")
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestListCollections_NoStore(t *testing.T) {
	handler, _, r := setupTestHandler(t)
	r.GET("/store/collections", handler.ListCollections)

	req := httptest.NewRequest(http.MethodGet, "/store/collections", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestListCollections_Success(t *testing.T) {
	handler, gs, _, r := setupTestHandlerWithStore(t)
	r.GET("/store/collections", handler.ListCollections)

	if err := gs.Set(models.CollectionKeyPrefix+"users", []any{
		map[string]any{"name": "alice"},
		map[string]any{"name": "bob"},
	}); err != nil {
		t.Fatalf("Set users: %v", err)
	}
	if err := gs.Set(models.CollectionKeyPrefix+"logs", []any{map[string]any{"id": 1}}); err != nil {
		t.Fatalf("Set logs: %v", err)
	}
	if err := gs.Set(models.CollectionKeyPrefix+"weird", "not-an-array"); err != nil {
		t.Fatalf("Set weird: %v", err)
	}
	if err := gs.Set("plain-key", "value"); err != nil {
		t.Fatalf("Set plain-key: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/store/collections", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var infos []models.CollectionInfo
	if err := json.Unmarshal(w.Body.Bytes(), &infos); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("expected 3 collections, got %d", len(infos))
	}

	counts := map[string]int{}
	for _, info := range infos {
		counts[info.Name] = info.Count
	}
	if counts["users"] != 2 || counts["logs"] != 1 || counts["weird"] != 0 {
		t.Fatalf("unexpected counts: %#v", counts)
	}
}

func TestGetCollection_Success(t *testing.T) {
	handler, gs, _, r := setupTestHandlerWithStore(t)
	r.GET("/store/collections/:name", handler.GetCollection)

	if err := gs.Set(models.CollectionKeyPrefix+"users", []any{map[string]any{"name": "alice"}}); err != nil {
		t.Fatalf("Set users: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/store/collections/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var docs []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &docs); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(docs) != 1 || docs[0]["name"] != "alice" {
		t.Fatalf("unexpected docs: %#v", docs)
	}
}

func TestInsertCollectionDoc_Success(t *testing.T) {
	handler, gs, _, r := setupTestHandlerWithStore(t)
	r.POST("/store/collections/:name", handler.InsertCollectionDoc)

	req := httptest.NewRequest(http.MethodPost, "/store/collections/users", bytes.NewBufferString(`{"name":"alice","age":30}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	raw, ok := gs.Get(models.CollectionKeyPrefix + "users")
	if !ok {
		t.Fatal("expected users collection to be stored")
	}
	docs, ok := raw.([]any)
	if !ok || len(docs) != 1 {
		t.Fatalf("unexpected stored docs: %#v", raw)
	}
}

func TestInsertCollectionDoc_InvalidBody(t *testing.T) {
	handler, _, _, r := setupTestHandlerWithStore(t)
	r.POST("/store/collections/:name", handler.InsertCollectionDoc)

	t.Run("invalid-json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/store/collections/users", bytes.NewBufferString(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("non-object", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/store/collections/users", bytes.NewBufferString(`[1,2,3]`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestUpdateCollectionDoc_Success(t *testing.T) {
	handler, gs, _, r := setupTestHandlerWithStore(t)
	r.PUT("/store/collections/:name/:index", handler.UpdateCollectionDoc)

	if err := gs.Set(models.CollectionKeyPrefix+"users", []any{map[string]any{"name": "alice", "age": 30.0}}); err != nil {
		t.Fatalf("Set users: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/store/collections/users/0", bytes.NewBufferString(`{"age":31,"city":"Paris"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var doc map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if doc["name"] != "alice" || doc["age"] != float64(31) || doc["city"] != "Paris" {
		t.Fatalf("unexpected updated doc: %#v", doc)
	}
}

func TestUpdateCollectionDoc_BadIndex(t *testing.T) {
	handler, gs, _, r := setupTestHandlerWithStore(t)
	r.PUT("/store/collections/:name/:index", handler.UpdateCollectionDoc)

	if err := gs.Set(models.CollectionKeyPrefix+"users", []any{map[string]any{"name": "alice"}}); err != nil {
		t.Fatalf("Set users: %v", err)
	}

	t.Run("not-integer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/store/collections/users/nope", bytes.NewBufferString(`{"name":"bob"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("out-of-range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/store/collections/users/5", bytes.NewBufferString(`{"name":"bob"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}

func TestDeleteCollectionDoc_Success(t *testing.T) {
	handler, gs, _, r := setupTestHandlerWithStore(t)
	r.DELETE("/store/collections/:name/:index", handler.DeleteCollectionDoc)

	if err := gs.Set(models.CollectionKeyPrefix+"users", []any{
		map[string]any{"name": "alice"},
		map[string]any{"name": "bob"},
	}); err != nil {
		t.Fatalf("Set users: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/store/collections/users/0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	raw, _ := gs.Get(models.CollectionKeyPrefix + "users")
	docs := raw.([]any)
	if len(docs) != 1 {
		t.Fatalf("expected 1 remaining doc, got %d", len(docs))
	}
	if docs[0].(map[string]any)["name"] != "bob" {
		t.Fatalf("unexpected remaining doc: %#v", docs[0])
	}
}

func TestDeleteCollectionDoc_OutOfRange(t *testing.T) {
	handler, gs, _, r := setupTestHandlerWithStore(t)
	r.DELETE("/store/collections/:name/:index", handler.DeleteCollectionDoc)

	if err := gs.Set(models.CollectionKeyPrefix+"users", []any{map[string]any{"name": "alice"}}); err != nil {
		t.Fatalf("Set users: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/store/collections/users/2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestClearCollection_Success(t *testing.T) {
	handler, gs, _, r := setupTestHandlerWithStore(t)
	r.DELETE("/store/collections/:name", handler.ClearCollection)

	if err := gs.Set(models.CollectionKeyPrefix+"users", []any{
		map[string]any{"name": "alice"},
		map[string]any{"name": "bob"},
	}); err != nil {
		t.Fatalf("Set users: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/store/collections/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	raw, ok := gs.Get(models.CollectionKeyPrefix + "users")
	if !ok {
		t.Fatal("expected cleared collection key to remain present")
	}
	docs, ok := raw.([]any)
	if !ok || len(docs) != 0 {
		t.Fatalf("expected empty collection, got %#v", raw)
	}
}
