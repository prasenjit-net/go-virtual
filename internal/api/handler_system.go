package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/version"
)

// HealthCheck returns health status
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// Version returns version metadata for the running binary
func (h *Handler) Version(c *gin.Context) {
	info := version.Get()
	c.JSON(http.StatusOK, info)
}

// GetBranding returns the UI branding configuration (app title, subtitle).
func (h *Handler) GetBranding(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"appTitle":    h.branding.AppTitle,
		"appSubtitle": h.branding.AppSubtitle,
	})
}

// GetRoutes returns registered routes
func (h *Handler) GetRoutes(c *gin.Context) {
	routes := h.proxyEngine.GetRegisteredRoutes()
	c.JSON(http.StatusOK, routes)
}

// ValidateTemplate validates a body template using the current helper set.
func (h *Handler) ValidateTemplate(c *gin.Context) {
	var input struct {
		Body string `json:"body"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.templateEngine.ValidateBodyTemplate(input.Body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"valid": true})
}

// ── Statistics ────────────────────────────────────────────────────────────────

// GetGlobalStats returns global statistics
func (h *Handler) GetGlobalStats(c *gin.Context) {
	specs, _ := h.store.GetEnabledSpecs()
	ops, _ := h.store.GetAllOperations()

	stats := h.statsCollector.GetGlobalStats(len(specs), len(ops))
	c.JSON(http.StatusOK, stats)
}

// GetSpecStats returns statistics for a spec
func (h *Handler) GetSpecStats(c *gin.Context) {
	id := c.Param("id")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	stats := h.statsCollector.GetSpecStats(id, spec.Name)
	c.JSON(http.StatusOK, stats)
}

// GetOperationStats returns statistics for an operation
func (h *Handler) GetOperationStats(c *gin.Context) {
	id := c.Param("id")

	stats := h.statsCollector.GetOperationStats(id)
	if stats == nil {
		c.JSON(http.StatusOK, gin.H{"message": "No statistics available"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ResetStats resets all statistics
func (h *Handler) ResetStats(c *gin.Context) {
	h.statsCollector.Reset()
	c.JSON(http.StatusOK, gin.H{"message": "Statistics reset"})
}

// ── Tracing ───────────────────────────────────────────────────────────────────

// ListTraces returns traces
func (h *Handler) ListTraces(c *gin.Context) {
	filter := &models.TraceFilter{
		Limit: 100, // Default limit
	}

	// Parse query params
	if specID := c.Query("specId"); specID != "" {
		filter.SpecID = specID
	}
	if opID := c.Query("operationId"); opID != "" {
		filter.OperationID = opID
	}
	if method := c.Query("method"); method != "" {
		filter.Method = method
	}

	traces := h.tracingService.GetTraces(filter)
	c.JSON(http.StatusOK, traces)
}

// GetTrace returns a single trace
func (h *Handler) GetTrace(c *gin.Context) {
	id := c.Param("id")

	trace := h.tracingService.GetTrace(id)
	if trace == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trace not found"})
		return
	}

	c.JSON(http.StatusOK, trace)
}

// ClearTraces clears all traces
func (h *Handler) ClearTraces(c *gin.Context) {
	specID := c.Query("specId")
	if specID != "" {
		h.tracingService.ClearTracesBySpec(specID)
	} else {
		h.tracingService.ClearTraces()
	}
	c.JSON(http.StatusOK, gin.H{"message": "Traces cleared"})
}

// ── Tags ──────────────────────────────────────────────────────────────────────

// ListTags returns all tags
func (h *Handler) ListTags(c *gin.Context) {
	tags, err := h.store.ListTags()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tags)
}

// CreateTag creates a new tag
func (h *Handler) CreateTag(c *gin.Context) {
	var input models.Tag
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input.Name = normalizeTag(input.Name)
	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag name is required"})
		return
	}
	if input.Name == models.DefaultTagName {
		c.JSON(http.StatusBadRequest, gin.H{"error": "default tag cannot be created"})
		return
	}

	input.CreatedAt = time.Now()
	input.UpdatedAt = time.Now()

	if err := h.store.CreateTag(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, input)
}

// UpdateTag updates a tag (description only; name changes are not supported)
func (h *Handler) UpdateTag(c *gin.Context) {
	oldName := normalizeTag(c.Param("name"))
	if oldName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag name is required"})
		return
	}

	existing, err := h.store.GetTag(oldName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tag not found"})
		return
	}

	var input models.Tag
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newName := normalizeTag(input.Name)
	if newName != "" && newName != oldName {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag name cannot be changed"})
		return
	}

	updated := &models.Tag{
		Name:        oldName,
		Description: input.Description,
		CreatedAt:   existing.CreatedAt,
		UpdatedAt:   time.Now(),
	}

	if err := h.store.UpdateTag(oldName, updated); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}

// DeleteTag deletes a tag and reassigns responses to default
func (h *Handler) DeleteTag(c *gin.Context) {
	name := normalizeTag(c.Param("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag name is required"})
		return
	}
	if name == models.DefaultTagName {
		c.JSON(http.StatusBadRequest, gin.H{"error": "default tag cannot be deleted"})
		return
	}

	if err := h.store.DeleteTag(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tag not found"})
		return
	}

	h.replaceTagWithDefault(name)

	c.JSON(http.StatusOK, gin.H{"message": "tag deleted"})
}

// renameTagUsage renames all references to oldName with newName across specs
// and response configs. It is a helper available for internal use and tests.
func (h *Handler) renameTagUsage(oldName, newName string) {
	specs, _ := h.store.GetAllSpecs()
	for _, spec := range specs {
		changed := false
		for i, t := range spec.EnabledTags {
			if t == oldName {
				spec.EnabledTags[i] = newName
				changed = true
			}
		}
		if changed {
			_ = h.store.UpdateSpec(spec)
		}
	}
	ops, _ := h.store.GetAllOperations()
	for _, op := range ops {
		cfgs, _ := h.store.GetResponseConfigsByOperation(op.ID)
		for _, cfg := range cfgs {
			if cfg.Tag == oldName {
				cfg.Tag = newName
				_ = h.store.UpdateResponseConfig(cfg)
			}
		}
	}
}
