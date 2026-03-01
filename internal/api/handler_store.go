package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ListStoreEntries returns all entries in the global store.
func (h *Handler) ListStoreEntries(c *gin.Context) {
	if h.globalStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store not enabled"})
		return
	}
	c.JSON(http.StatusOK, h.globalStore.List())
}

// GetStoreEntry returns a single entry by key.
func (h *Handler) GetStoreEntry(c *gin.Context) {
	if h.globalStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store not enabled"})
		return
	}

	key := c.Param("key")
	val, ok := h.globalStore.Get(key)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"key": key, "value": val})
}

// UpsertStoreEntry creates or updates a store entry.
// Body: { "value": <any JSON> }
func (h *Handler) UpsertStoreEntry(c *gin.Context) {
	if h.globalStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store not enabled"})
		return
	}

	key := c.Param("key")
	if strings.TrimSpace(key) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key must not be empty"})
		return
	}

	var body struct {
		Value json.RawMessage `json:"value"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	var val any
	if err := json.Unmarshal(body.Value, &val); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value must be valid JSON: " + err.Error()})
		return
	}

	if err := h.globalStore.Set(key, val); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	v, _ := h.globalStore.Get(key)
	c.JSON(http.StatusOK, gin.H{"key": key, "value": v})
}

// DeleteStoreEntry removes a key from the global store.
func (h *Handler) DeleteStoreEntry(c *gin.Context) {
	if h.globalStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store not enabled"})
		return
	}

	key := c.Param("key")
	if err := h.globalStore.Delete(key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// ClearStore removes all entries from the global store.
// Requires query param ?confirm=true as a safety guard.
func (h *Handler) ClearStore(c *gin.Context) {
	if h.globalStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store not enabled"})
		return
	}

	if c.Query("confirm") != "true" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pass ?confirm=true to clear all store entries"})
		return
	}

	if err := h.globalStore.Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
