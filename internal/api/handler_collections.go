package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

// collectionGuard returns early when the CollectionBackend is unavailable or
// the collection name is empty.
func (h *Handler) collectionGuard(c *gin.Context) (name string, ok bool) {
	if h.collectionBackend == nil {
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

// ── Handlers ─────────────────────────────────────────────────────────────────

// ListCollections returns a summary of all named collections (name + count).
// GET /_api/store/collections
func (h *Handler) ListCollections(c *gin.Context) {
	if h.collectionBackend == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store not enabled"})
		return
	}
	names, err := h.collectionBackend.ListCollections()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := make([]models.CollectionInfo, 0, len(names))
	for _, name := range names {
		docs, err := h.collectionBackend.GetAll(name)
		count := 0
		if err == nil {
			count = len(docs)
		}
		result = append(result, models.CollectionInfo{Name: name, Count: count})
	}
	c.JSON(http.StatusOK, result)
}

// GetCollection returns all base documents in a collection.
// GET /_api/store/collections/:name
func (h *Handler) GetCollection(c *gin.Context) {
	name, ok := h.collectionGuard(c)
	if !ok {
		return
	}
	docs, err := h.collectionBackend.GetAll(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if docs == nil {
		docs = []map[string]any{}
	}
	c.JSON(http.StatusOK, docs)
}

// InsertCollectionDoc seeds one document into a collection's base.
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
	inserted, err := h.collectionBackend.SeedInsert(name, m)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, inserted)
}

// UpdateCollectionDoc replaces the document at the given zero-based index.
// PUT /_api/store/collections/:name/:index
// Body: JSON object with fields to merge into the existing document
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
	docs, err := h.collectionBackend.GetAll(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if idx < 0 || idx >= len(docs) {
		c.JSON(http.StatusNotFound, gin.H{"error": "index out of range"})
		return
	}
	for k, v := range changes {
		docs[idx][k] = v
	}
	// Rewrite the collection base: clear then re-seed
	if err := h.collectionBackend.SeedClear(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, d := range docs {
		if _, err := h.collectionBackend.SeedInsert(name, d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
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
	docs, err := h.collectionBackend.GetAll(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if idx < 0 || idx >= len(docs) {
		c.JSON(http.StatusNotFound, gin.H{"error": "index out of range"})
		return
	}
	docs = append(docs[:idx], docs[idx+1:]...)
	if err := h.collectionBackend.SeedClear(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, d := range docs {
		if _, err := h.collectionBackend.SeedInsert(name, d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// ClearCollection removes all base documents from a collection.
// DELETE /_api/store/collections/:name
func (h *Handler) ClearCollection(c *gin.Context) {
	name, ok := h.collectionGuard(c)
	if !ok {
		return
	}
	if err := h.collectionBackend.SeedClear(name); err != nil {
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
