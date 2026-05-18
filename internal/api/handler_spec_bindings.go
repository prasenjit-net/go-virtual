package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

// ListSpecScriptBindings returns all script bindings for a spec.
func (h *Handler) ListSpecScriptBindings(c *gin.Context) {
	specID := c.Param("id")
	if _, err := h.store.GetSpec(specID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "spec not found"})
		return
	}

	bindings, err := h.store.GetSpecScriptBindings(specID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bindings)
}

// CreateSpecScriptBinding attaches a script to a spec.
func (h *Handler) CreateSpecScriptBinding(c *gin.Context) {
	specID := c.Param("id")
	if _, err := h.store.GetSpec(specID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "spec not found"})
		return
	}

	var input models.ScriptBindingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.ScriptID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scriptId is required"})
		return
	}
	if input.OutputKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outputKey is required"})
		return
	}

	script, err := h.store.GetScript(input.ScriptID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "script not found: " + input.ScriptID})
		return
	}

	binding := &models.ScriptBinding{
		ID:         generateID(),
		SpecID:     specID,
		ScriptID:   input.ScriptID,
		ScriptName: script.Name,
		OutputKey:  input.OutputKey,
		Order:      input.Order,
		Enabled:    input.Enabled,
	}

	if err := h.store.CreateScriptBinding(binding); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, binding)
}

// UpdateSpecScriptBinding updates binding metadata (outputKey, order, enabled).
func (h *Handler) UpdateSpecScriptBinding(c *gin.Context) {
	specID := c.Param("id")
	bindingID := c.Param("bindingId")

	bindings, err := h.store.GetSpecScriptBindings(specID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var existing *models.ScriptBinding
	for _, b := range bindings {
		if b.ID == bindingID {
			existing = b
			break
		}
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script binding not found"})
		return
	}

	var input models.ScriptBindingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.OutputKey != "" {
		existing.OutputKey = input.OutputKey
	}
	existing.Order = input.Order
	existing.Enabled = input.Enabled

	if err := h.store.UpdateScriptBinding(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, existing)
}

// DeleteSpecScriptBinding detaches a script from a spec.
func (h *Handler) DeleteSpecScriptBinding(c *gin.Context) {
	specID := c.Param("id")
	bindingID := c.Param("bindingId")

	bindings, err := h.store.GetSpecScriptBindings(specID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	found := false
	for _, b := range bindings {
		if b.ID == bindingID {
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "script binding not found"})
		return
	}

	if err := h.store.DeleteScriptBinding(bindingID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// ReorderSpecScriptBindings bulk-updates the Order of bindings for a spec.
func (h *Handler) ReorderSpecScriptBindings(c *gin.Context) {
	specID := c.Param("id")
	if _, err := h.store.GetSpec(specID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "spec not found"})
		return
	}

	var input []struct {
		ID    string `json:"id"`
		Order int    `json:"order"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bindings, err := h.store.GetSpecScriptBindings(specID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	byID := make(map[string]*models.ScriptBinding, len(bindings))
	for _, b := range bindings {
		byID[b.ID] = b
	}

	for _, item := range input {
		if b, ok := byID[item.ID]; ok {
			b.Order = item.Order
			if err := h.store.UpdateScriptBinding(b); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}

	updated, _ := h.store.GetSpecScriptBindings(specID)
	c.JSON(http.StatusOK, updated)
}
