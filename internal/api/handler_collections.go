package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

// collectionGuard is a small helper that returns early when the global store is
// unavailable and ensures the collection name is not the empty string.
func (h *Handler) collectionGuard(c *gin.Context) (name string, ok bool) {
	if h.globalStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store not enabled"})
		return "", false
	}
	name = c.Param("name")
	if strings.TrimSpace(name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collection name must not be empty"})
		return "", false
	}
	return name, true
}

// collectionKey returns the flat store key for the given collection name.
func collectionKey(name string) string { return models.CollectionKeyPrefix + name }

// loadDocs reads the document slice for a collection from the global store.
// Returns an empty (non-nil) slice when the collection doesn't exist yet.
func (h *Handler) loadDocs(name string) []map[string]any {
	raw, ok := h.globalStore.Get(collectionKey(name))
	if !ok || raw == nil {
		return []map[string]any{}
	}
	switch v := raw.(type) {
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]any:
		return v
	}
	return []map[string]any{}
}

// saveDocs writes the document slice back to the global store.
func (h *Handler) saveDocs(name string, docs []map[string]any) error {
	raw := make([]any, len(docs))
	for i, d := range docs {
		raw[i] = d
	}
	return h.globalStore.Set(collectionKey(name), raw)
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// ListCollections returns a summary of all named collections (name + count).
// GET /_api/store/collections
func (h *Handler) ListCollections(c *gin.Context) {
	if h.globalStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store not enabled"})
		return
	}
	entries := h.globalStore.List()
	result := []models.CollectionInfo{}
	for _, e := range entries {
		if !strings.HasPrefix(e.Key, models.CollectionKeyPrefix) {
			continue
		}
		name := strings.TrimPrefix(e.Key, models.CollectionKeyPrefix)
		count := 0
		if arr, ok := e.Value.([]any); ok {
			count = len(arr)
		}
		result = append(result, models.CollectionInfo{Name: name, Count: count})
	}
	c.JSON(http.StatusOK, result)
}

// GetCollection returns all documents in a collection.
// GET /_api/store/collections/:name
func (h *Handler) GetCollection(c *gin.Context) {
	name, ok := h.collectionGuard(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, h.loadDocs(name))
}

// InsertCollectionDoc appends one document to a collection.
// POST /_api/store/collections/:name
// Body: any JSON object
func (h *Handler) InsertCollectionDoc(c *gin.Context) {
	name, ok := h.collectionGuard(c)
	if !ok {
		return
	}
	var doc json.RawMessage
	if err := c.ShouldBindJSON(&doc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body: " + err.Error()})
		return
	}
	var m map[string]any
	if err := json.Unmarshal(doc, &m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be a JSON object"})
		return
	}
	docs := h.loadDocs(name)
	docs = append(docs, m)
	if err := h.saveDocs(name, docs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, m)
}

// UpdateCollectionDoc replaces the document at the given zero-based index.
// PUT /_api/store/collections/:name/:index
// Body: JSON object with updated fields (merged, not replaced)
func (h *Handler) UpdateCollectionDoc(c *gin.Context) {
	name, ok := h.collectionGuard(c)
	if !ok {
		return
	}
	idx, ok := parseIndexParam(c, "index")
	if !ok {
		return
	}
	var doc json.RawMessage
	if err := c.ShouldBindJSON(&doc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body: " + err.Error()})
		return
	}
	var changes map[string]any
	if err := json.Unmarshal(doc, &changes); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be a JSON object"})
		return
	}
	docs := h.loadDocs(name)
	if idx < 0 || idx >= len(docs) {
		c.JSON(http.StatusNotFound, gin.H{"error": "index out of range"})
		return
	}
	for k, v := range changes {
		docs[idx][k] = v
	}
	if err := h.saveDocs(name, docs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, docs[idx])
}

// DeleteCollectionDoc removes the document at the given zero-based index.
// DELETE /_api/store/collections/:name/:index
func (h *Handler) DeleteCollectionDoc(c *gin.Context) {
	name, ok := h.collectionGuard(c)
	if !ok {
		return
	}
	idx, ok := parseIndexParam(c, "index")
	if !ok {
		return
	}
	docs := h.loadDocs(name)
	if idx < 0 || idx >= len(docs) {
		c.JSON(http.StatusNotFound, gin.H{"error": "index out of range"})
		return
	}
	docs = append(docs[:idx], docs[idx+1:]...)
	if err := h.saveDocs(name, docs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ClearCollection removes all documents from a collection.
// DELETE /_api/store/collections/:name
func (h *Handler) ClearCollection(c *gin.Context) {
	name, ok := h.collectionGuard(c)
	if !ok {
		return
	}
	if err := h.saveDocs(name, nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ── helper ───────────────────────────────────────────────────────────────────

func parseIndexParam(c *gin.Context, param string) (int, bool) {
	s := c.Param(param)
	n, err := json.Number(s).Int64()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "index must be an integer"})
		return 0, false
	}
	return int(n), true
}
