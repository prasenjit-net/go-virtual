package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

// ListResponseConfigs returns all response configs for an operation
func (h *Handler) ListResponseConfigs(c *gin.Context) {
	opID := c.Param("id")

	configs, err := h.store.GetResponseConfigsByOperation(opID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, configs)
}

// CreateResponseConfig creates a new response config
func (h *Handler) CreateResponseConfig(c *gin.Context) {
	opID := c.Param("id")

	// Verify operation exists
	_, err := h.store.GetOperation(opID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Operation not found"})
		return
	}

	var input models.ResponseConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate ID
	cfg := &models.ResponseConfig{
		ID:          generateID(),
		OperationID: opID,
		Name:        input.Name,
		Description: input.Description,
		Tag:         normalizeTag(input.Tag),
		Priority:    input.Priority,
		Conditions:  input.Conditions,
		StatusCode:  input.StatusCode,
		Headers:     input.Headers,
		Body:        input.Body,
		Delay:       input.Delay,
		Enabled:     input.Enabled,
	}

	if cfg.Tag == "" {
		cfg.Tag = models.DefaultTagName
	}
	if err := h.ensureTagExists(cfg.Tag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults
	if cfg.StatusCode == 0 {
		cfg.StatusCode = 200
	}
	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}
	if cfg.Conditions == nil {
		cfg.Conditions = make([]models.Condition, 0)
	}

	if err := h.store.CreateResponseConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, cfg)
}

// GetResponseConfig returns a single response config
func (h *Handler) GetResponseConfig(c *gin.Context) {
	id := c.Param("id")

	cfg, err := h.store.GetResponseConfig(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Response config not found"})
		return
	}

	c.JSON(http.StatusOK, cfg)
}

// UpdateResponseConfig updates a response config
func (h *Handler) UpdateResponseConfig(c *gin.Context) {
	id := c.Param("id")

	cfg, err := h.store.GetResponseConfig(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Response config not found"})
		return
	}

	var update models.ResponseConfigUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Apply updates
	if update.Name != nil {
		cfg.Name = *update.Name
	}
	if update.Description != nil {
		cfg.Description = *update.Description
	}
	if update.Tag != nil {
		cfg.Tag = normalizeTag(*update.Tag)
		if cfg.Tag == "" {
			cfg.Tag = models.DefaultTagName
		}
		if err := h.ensureTagExists(cfg.Tag); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if update.Priority != nil {
		cfg.Priority = *update.Priority
	}
	if update.Conditions != nil {
		cfg.Conditions = *update.Conditions
	}
	if update.StatusCode != nil {
		cfg.StatusCode = *update.StatusCode
	}
	if update.Headers != nil {
		cfg.Headers = *update.Headers
	}
	if update.Body != nil {
		cfg.Body = *update.Body
	}
	if update.Delay != nil {
		cfg.Delay = *update.Delay
	}
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}

	if err := h.store.UpdateResponseConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cfg)
}

// DeleteResponseConfig deletes a response config
func (h *Handler) DeleteResponseConfig(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.DeleteResponseConfig(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Response config not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Response config deleted"})
}

// UpdateResponsePriority updates the priority of a response config
func (h *Handler) UpdateResponsePriority(c *gin.Context) {
	id := c.Param("id")

	cfg, err := h.store.GetResponseConfig(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Response config not found"})
		return
	}

	var input struct {
		Priority int `json:"priority"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg.Priority = input.Priority

	if err := h.store.UpdateResponseConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cfg)
}
