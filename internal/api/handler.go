package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/parser"
	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

// Handler handles API requests
type Handler struct {
	store          storage.Storage
	statsCollector *stats.Collector
	tracingService *tracing.Service
	proxyEngine    *proxy.Engine
	parser         *parser.Parser
}

// NewHandler creates a new API handler
func NewHandler(store storage.Storage, statsCollector *stats.Collector, tracingService *tracing.Service, proxyEngine *proxy.Engine) *Handler {
	return &Handler{
		store:          store,
		statsCollector: statsCollector,
		tracingService: tracingService,
		proxyEngine:    proxyEngine,
		parser:         parser.NewParser(),
	}
}

// ListSpecs returns all specs
func (h *Handler) ListSpecs(c *gin.Context) {
	specs, err := h.store.GetAllSpecs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Don't include full content in list
	result := make([]map[string]interface{}, len(specs))
	for i, spec := range specs {
		ops, _ := h.store.GetOperationsBySpec(spec.ID)
		result[i] = map[string]interface{}{
			"id":              spec.ID,
			"name":            spec.Name,
			"version":         spec.Version,
			"description":     spec.Description,
			"basePath":        spec.BasePath,
			"enabled":         spec.Enabled,
			"tracing":         spec.Tracing,
			"createdAt":       spec.CreatedAt,
			"updatedAt":       spec.UpdatedAt,
			"operationCount":  len(ops),
		}
	}

	c.JSON(http.StatusOK, result)
}

// CreateSpec creates a new spec
func (h *Handler) CreateSpec(c *gin.Context) {
	var input models.SpecInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse the OpenAPI spec
	parseResult, err := h.parser.Parse(input.Content, input.BasePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OpenAPI spec: " + err.Error()})
		return
	}

	// Override name if provided
	if input.Name != "" {
		parseResult.Spec.Name = input.Name
	}
	if input.Description != "" {
		parseResult.Spec.Description = input.Description
	}

	// Save spec
	if err := h.store.CreateSpec(parseResult.Spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save operations
	for _, op := range parseResult.Operations {
		if err := h.store.CreateOperation(op); err != nil {
			// Rollback spec on error
			h.store.DeleteSpec(parseResult.Spec.ID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Reload routes
	h.proxyEngine.ReloadRoutes()

	c.JSON(http.StatusCreated, gin.H{
		"id":              parseResult.Spec.ID,
		"name":            parseResult.Spec.Name,
		"version":         parseResult.Spec.Version,
		"operationCount":  len(parseResult.Operations),
	})
}

// GetSpec returns a single spec
func (h *Handler) GetSpec(c *gin.Context) {
	id := c.Param("id")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	c.JSON(http.StatusOK, spec)
}

// UpdateSpec updates a spec
func (h *Handler) UpdateSpec(c *gin.Context) {
	id := c.Param("id")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	var update models.SpecUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Apply updates
	if update.Name != nil {
		spec.Name = *update.Name
	}
	if update.BasePath != nil {
		spec.BasePath = *update.BasePath
		// Need to update operations' full paths too
		ops, _ := h.store.GetOperationsBySpec(id)
		for _, op := range ops {
			op.FullPath = spec.BasePath + op.Path
			h.store.UpdateOperation(op)
		}
	}
	if update.Description != nil {
		spec.Description = *update.Description
	}
	if update.Enabled != nil {
		spec.Enabled = *update.Enabled
	}
	if update.Tracing != nil {
		spec.Tracing = *update.Tracing
	}

	spec.UpdatedAt = time.Now()

	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Reload routes if base path or enabled changed
	h.proxyEngine.ReloadRoutes()

	c.JSON(http.StatusOK, spec)
}

// DeleteSpec deletes a spec
func (h *Handler) DeleteSpec(c *gin.Context) {
	id := c.Param("id")

	// Get operations first
	ops, _ := h.store.GetOperationsBySpec(id)

	// Delete response configs for each operation
	for _, op := range ops {
		h.store.DeleteResponseConfigsByOperation(op.ID)
	}

	// Delete operations
	h.store.DeleteOperationsBySpec(id)

	// Delete spec
	if err := h.store.DeleteSpec(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	// Clear traces for this spec
	h.tracingService.ClearTracesBySpec(id)

	// Reload routes
	h.proxyEngine.ReloadRoutes()

	c.JSON(http.StatusOK, gin.H{"message": "Spec deleted"})
}

// EnableSpec enables a spec
func (h *Handler) EnableSpec(c *gin.Context) {
	id := c.Param("id")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	spec.Enabled = true
	spec.UpdatedAt = time.Now()

	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.proxyEngine.ReloadRoutes()

	c.JSON(http.StatusOK, gin.H{"message": "Spec enabled"})
}

// DisableSpec disables a spec
func (h *Handler) DisableSpec(c *gin.Context) {
	id := c.Param("id")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	spec.Enabled = false
	spec.UpdatedAt = time.Now()

	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.proxyEngine.ReloadRoutes()

	c.JSON(http.StatusOK, gin.H{"message": "Spec disabled"})
}

// ToggleTracing toggles tracing for a spec
func (h *Handler) ToggleTracing(c *gin.Context) {
	id := c.Param("id")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		// Toggle if no body
		spec.Tracing = !spec.Tracing
	} else {
		spec.Tracing = input.Enabled
	}

	spec.UpdatedAt = time.Now()

	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tracing": spec.Tracing})
}

// ListOperations returns all operations for a spec
func (h *Handler) ListOperations(c *gin.Context) {
	specID := c.Param("id")

	ops, err := h.store.GetOperationsBySpec(specID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to summaries with response counts
	summaries := make([]models.OperationSummary, len(ops))
	for i, op := range ops {
		responses, _ := h.store.GetResponseConfigsByOperation(op.ID)
		summaries[i] = models.OperationSummary{
			ID:                 op.ID,
			SpecID:             op.SpecID,
			Method:             op.Method,
			Path:               op.Path,
			FullPath:           op.FullPath,
			OperationID:        op.OperationID,
			Summary:            op.Summary,
			ResponseCount:      len(responses),
			HasExampleResponse: op.ExampleResponse != nil,
		}
	}

	c.JSON(http.StatusOK, summaries)
}

// GetOperation returns a single operation
func (h *Handler) GetOperation(c *gin.Context) {
	id := c.Param("id")

	op, err := h.store.GetOperation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Operation not found"})
		return
	}

	// Get response configs
	responses, _ := h.store.GetResponseConfigsByOperation(id)
	op.Responses = make([]models.ResponseConfig, len(responses))
	for i, resp := range responses {
		op.Responses[i] = *resp
	}

	c.JSON(http.StatusOK, op)
}

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
		Priority:    input.Priority,
		Conditions:  input.Conditions,
		StatusCode:  input.StatusCode,
		Headers:     input.Headers,
		Body:        input.Body,
		Delay:       input.Delay,
		Enabled:     input.Enabled,
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

// GetRoutes returns registered routes
func (h *Handler) GetRoutes(c *gin.Context) {
	routes := h.proxyEngine.GetRegisteredRoutes()
	c.JSON(http.StatusOK, routes)
}

// HealthCheck returns health status
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// generateID generates a unique ID
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
