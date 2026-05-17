package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

// ListResponseScriptBindings returns all script bindings for a response config.
func (h *Handler) ListResponseScriptBindings(c *gin.Context) {
	responseConfigID := c.Param("respId")
	if _, err := h.store.GetResponseConfig(responseConfigID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "response config not found"})
		return
	}

	bindings, err := h.store.GetResponseScriptBindings(responseConfigID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bindings)
}

// CreateResponseScriptBinding attaches a script to a response config.
func (h *Handler) CreateResponseScriptBinding(c *gin.Context) {
	responseConfigID := c.Param("respId")
	if _, err := h.store.GetResponseConfig(responseConfigID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "response config not found"})
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
		ID:               generateID(),
		ResponseConfigID: responseConfigID,
		ScriptID:         input.ScriptID,
		ScriptName:       script.Name,
		OutputKey:        input.OutputKey,
		Order:            input.Order,
		Enabled:          input.Enabled,
	}

	if err := h.store.CreateScriptBinding(binding); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, binding)
}

// UpdateResponseScriptBinding updates binding metadata (outputKey, order, enabled).
func (h *Handler) UpdateResponseScriptBinding(c *gin.Context) {
	responseConfigID := c.Param("respId")
	bindingID := c.Param("bindingId")

	bindings, err := h.store.GetResponseScriptBindings(responseConfigID)
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

// DeleteResponseScriptBinding detaches a script from a response config.
func (h *Handler) DeleteResponseScriptBinding(c *gin.Context) {
	responseConfigID := c.Param("respId")
	bindingID := c.Param("bindingId")

	bindings, err := h.store.GetResponseScriptBindings(responseConfigID)
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

// ReorderResponseScriptBindings bulk-updates the Order of bindings for a response config.
func (h *Handler) ReorderResponseScriptBindings(c *gin.Context) {
	responseConfigID := c.Param("respId")
	if _, err := h.store.GetResponseConfig(responseConfigID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "response config not found"})
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

	bindings, err := h.store.GetResponseScriptBindings(responseConfigID)
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

	updated, _ := h.store.GetResponseScriptBindings(responseConfigID)
	c.JSON(http.StatusOK, updated)
}
