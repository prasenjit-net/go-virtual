package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

// ListScriptBindings returns all bindings for an operation.
func (h *Handler) ListScriptBindings(c *gin.Context) {
	operationID := c.Param("id")
	if _, err := h.store.GetOperation(operationID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "operation not found"})
		return
	}

	bindings, err := h.store.GetScriptBindings(operationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bindings)
}

// CreateScriptBinding attaches a script to an operation.
func (h *Handler) CreateScriptBinding(c *gin.Context) {
	operationID := c.Param("id")
	if _, err := h.store.GetOperation(operationID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "operation not found"})
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
		ID:          generateID(),
		OperationID: operationID,
		ScriptID:    input.ScriptID,
		ScriptName:  script.Name,
		OutputKey:   input.OutputKey,
		Order:       input.Order,
		Enabled:     input.Enabled,
	}

	if err := h.store.CreateScriptBinding(binding); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, binding)
}

// UpdateScriptBinding updates binding metadata (outputKey, order, enabled).
func (h *Handler) UpdateScriptBinding(c *gin.Context) {
	operationID := c.Param("id")
	bindingID := c.Param("bindingId")

	bindings, err := h.store.GetScriptBindings(operationID)
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

// DeleteScriptBinding detaches a script from an operation.
func (h *Handler) DeleteScriptBinding(c *gin.Context) {
	operationID := c.Param("id")
	bindingID := c.Param("bindingId")

	bindings, err := h.store.GetScriptBindings(operationID)
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

// ReorderScriptBindings bulk-updates the Order of bindings for an operation.
func (h *Handler) ReorderScriptBindings(c *gin.Context) {
	operationID := c.Param("id")
	if _, err := h.store.GetOperation(operationID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "operation not found"})
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

	bindings, err := h.store.GetScriptBindings(operationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build index
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

	// Return updated list
	updated, _ := h.store.GetScriptBindings(operationID)
	c.JSON(http.StatusOK, updated)
}
