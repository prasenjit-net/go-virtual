package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/scripting"
)

// scriptWithSource builds the JSON-serialisable representation of a Script,
// explicitly including the Source field (which has json:"-" on the struct).
func scriptWithSource(s *models.Script) map[string]any {
	return map[string]any{
		"id":          s.ID,
		"name":        s.Name,
		"description": s.Description,
		"timeout":     s.Timeout,
		"enabled":     s.Enabled,
		"createdAt":   s.CreatedAt,
		"updatedAt":   s.UpdatedAt,
		"source":      s.Source,
	}
}

// ListScripts returns all scripts (metadata only, no source).
func (h *Handler) ListScripts(c *gin.Context) {
	scripts, err := h.store.GetAllScripts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]map[string]any, len(scripts))
	for i, s := range scripts {
		result[i] = map[string]any{
			"id":          s.ID,
			"name":        s.Name,
			"description": s.Description,
			"timeout":     s.Timeout,
			"enabled":     s.Enabled,
			"createdAt":   s.CreatedAt,
			"updatedAt":   s.UpdatedAt,
		}
	}
	c.JSON(http.StatusOK, result)
}

// CreateScript creates a new Starlark script.
func (h *Handler) CreateScript(c *gin.Context) {
	var input models.ScriptInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	// Validate source
	if input.Source != "" {
		if err := h.scriptEngine.CompileAndValidate("validate", input.Source); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
	}

	now := time.Now()
	script := &models.Script{
		ID:          generateID(),
		Name:        input.Name,
		Description: input.Description,
		Source:      input.Source,
		Timeout:     input.Timeout,
		Enabled:     input.Enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.store.CreateScript(script); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, scriptWithSource(script))
}

// GetScript retrieves a script by ID (includes source).
func (h *Handler) GetScript(c *gin.Context) {
	id := c.Param("id")
	script, err := h.store.GetScript(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}
	c.JSON(http.StatusOK, scriptWithSource(script))
}

// UpdateScript updates an existing script.
func (h *Handler) UpdateScript(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.store.GetScript(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}

	var input models.ScriptInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate source if provided
	source := input.Source
	if source == "" {
		source = existing.Source
	}
	if source != "" {
		if err := h.scriptEngine.CompileAndValidate(id, source); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
	}

	// Update fields
	if input.Name != "" {
		existing.Name = input.Name
	}
	existing.Description = input.Description
	existing.Source = source
	if input.Timeout >= 0 {
		existing.Timeout = input.Timeout
	}
	existing.Enabled = input.Enabled
	existing.UpdatedAt = time.Now()

	if err := h.store.UpdateScript(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, scriptWithSource(existing))
}

// DeleteScript deletes a script and all its bindings.
func (h *Handler) DeleteScript(c *gin.Context) {
	id := c.Param("id")
	if _, err := h.store.GetScript(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}

	// Remove all bindings for this script
	if err := h.store.DeleteScriptBindingsByScript(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.store.DeleteScript(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// ValidateScript validates Starlark source without saving.
func (h *Handler) ValidateScript(c *gin.Context) {
	var input struct {
		Source string `json:"source"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.scriptEngine.CompileAndValidate("validate", input.Source); err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"valid": true, "error": nil})
}

// TestScript executes a saved script with a mock input.
func (h *Handler) TestScript(c *gin.Context) {
	id := c.Param("id")
	script, err := h.store.GetScript(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}

	var body struct {
		Input *scripting.ScriptInput `json:"input"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := body.Input
	if input == nil {
		input = &scripting.ScriptInput{
			Path:   map[string]string{},
			Query:  map[string]string{},
			Header: map[string]string{},
			Body:   nil,
		}
	}

	output, logs, durationMs, execErr := h.scriptEngine.TestScript(c.Request.Context(), script, input)

	resp := gin.H{
		"output":     output,
		"durationMs": durationMs,
		"logs":       logs,
		"error":      nil,
	}
	if execErr != nil {
		resp["error"] = execErr.Error()
		resp["output"] = nil
	}

	c.JSON(http.StatusOK, resp)
}
