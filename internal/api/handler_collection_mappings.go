package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/prasenjit/go-virtual/internal/models"
)

// ListCollectionMappings returns all collection mappings for a response config.
// GET /_api/operations/:id/responses/:respId/mappings
func (h *Handler) ListCollectionMappings(c *gin.Context) {
	responseConfigID := c.Param("respId")
	if _, err := h.store.GetResponseConfig(responseConfigID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "response config not found"})
		return
	}

	mappings, err := h.store.GetCollectionMappingsByResponse(responseConfigID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if mappings == nil {
		mappings = []*models.CollectionMapping{}
	}
	c.JSON(http.StatusOK, mappings)
}

// CreateCollectionMapping attaches a new collection mapping to a response config.
// POST /_api/operations/:id/responses/:respId/mappings
func (h *Handler) CreateCollectionMapping(c *gin.Context) {
	responseConfigID := c.Param("respId")
	if _, err := h.store.GetResponseConfig(responseConfigID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "response config not found"})
		return
	}

	var input models.CollectionMappingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.CollectionName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collectionName is required"})
		return
	}
	if input.OutputKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outputKey is required"})
		return
	}
	if input.Operation == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "operation is required"})
		return
	}

	cm := &models.CollectionMapping{
		ID:               generateID(),
		ResponseConfigID: responseConfigID,
		CollectionName:   input.CollectionName,
		Name:             input.Name,
		Operation:        input.Operation,
		FilterRules:      input.FilterRules,
		DataRules:        input.DataRules,
		OutputKey:        input.OutputKey,
		Order:            input.Order,
		Enabled:          input.Enabled,
	}
	if cm.FilterRules == nil {
		cm.FilterRules = []models.FieldMappingRule{}
	}
	if cm.DataRules == nil {
		cm.DataRules = []models.FieldMappingRule{}
	}

	if err := h.store.CreateCollectionMapping(cm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cm)
}

// GetCollectionMapping returns a single collection mapping by ID.
// GET /_api/mappings/:mappingId
func (h *Handler) GetCollectionMapping(c *gin.Context) {
	id := c.Param("mappingId")
	cm, err := h.store.GetCollectionMapping(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "collection mapping not found"})
		return
	}
	c.JSON(http.StatusOK, cm)
}

// UpdateCollectionMapping replaces a collection mapping's configuration.
// PUT /_api/mappings/:mappingId
func (h *Handler) UpdateCollectionMapping(c *gin.Context) {
	id := c.Param("mappingId")
	existing, err := h.store.GetCollectionMapping(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "collection mapping not found"})
		return
	}

	var input models.CollectionMappingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing.CollectionName = input.CollectionName
	existing.Name = input.Name
	existing.Operation = input.Operation
	existing.FilterRules = input.FilterRules
	existing.DataRules = input.DataRules
	existing.OutputKey = input.OutputKey
	existing.Order = input.Order
	existing.Enabled = input.Enabled
	if existing.FilterRules == nil {
		existing.FilterRules = []models.FieldMappingRule{}
	}
	if existing.DataRules == nil {
		existing.DataRules = []models.FieldMappingRule{}
	}

	if err := h.store.UpdateCollectionMapping(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, existing)
}

// DeleteCollectionMapping removes a collection mapping.
// DELETE /_api/mappings/:mappingId
func (h *Handler) DeleteCollectionMapping(c *gin.Context) {
	id := c.Param("mappingId")
	if _, err := h.store.GetCollectionMapping(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "collection mapping not found"})
		return
	}
	if err := h.store.DeleteCollectionMapping(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
