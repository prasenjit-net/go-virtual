package api

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/parser"
	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/scripting"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/template"
	"github.com/prasenjit/go-virtual/internal/tracing"
	"github.com/prasenjit/go-virtual/internal/version"
)

// Handler handles API requests
type Handler struct {
	store          storage.Storage
	statsCollector *stats.Collector
	tracingService *tracing.Service
	proxyEngine    *proxy.Engine
	parser         *parser.Parser
	templateEngine *template.Engine
	scriptEngine   *scripting.ScriptEngine
	branding       config.BrandingConfig
}

// NewHandler creates a new API handler
func NewHandler(store storage.Storage, statsCollector *stats.Collector, tracingService *tracing.Service, proxyEngine *proxy.Engine) *Handler {
	return &Handler{
		store:          store,
		statsCollector: statsCollector,
		tracingService: tracingService,
		proxyEngine:    proxyEngine,
		parser:         parser.NewParser(),
		templateEngine: template.NewEngine(),
		scriptEngine:   scripting.NewScriptEngine(store, 100),
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
			"id":                 spec.ID,
			"name":               spec.Name,
			"version":            spec.Version,
			"description":        spec.Description,
			"basePath":           spec.BasePath,
			"enabled":            spec.Enabled,
			"tracing":            spec.Tracing,
			"useExampleFallback": spec.UseExampleFallback,
			"enabledTags":        spec.EnabledTags,
			"createdAt":          spec.CreatedAt,
			"updatedAt":          spec.UpdatedAt,
			"operationCount":     len(ops),
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
	parseResult.Spec.EnabledTags = []string{}
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
	if update.UseExampleFallback != nil {
		spec.UseExampleFallback = *update.UseExampleFallback
	}
	if update.BackendURI != nil {
		spec.BackendURI = *update.BackendURI
		// Disabling backend URI also disables proxy mode
		if spec.BackendURI == "" {
			spec.ProxyMode = false
		}
	}
	if update.ProxyMode != nil {
		// Proxy mode requires a backend URI
		if *update.ProxyMode && spec.BackendURI == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "backendUri must be set before enabling proxyMode"})
			return
		}
		spec.ProxyMode = *update.ProxyMode
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

// ToggleExampleFallback toggles example fallback responses for a spec
func (h *Handler) ToggleExampleFallback(c *gin.Context) {
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
		spec.UseExampleFallback = !spec.UseExampleFallback
	} else {
		spec.UseExampleFallback = input.Enabled
	}

	spec.UpdatedAt = time.Now()

	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Reload routes to apply the change
	h.proxyEngine.ReloadRoutes()

	c.JSON(http.StatusOK, gin.H{"useExampleFallback": spec.UseExampleFallback})
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

// UpdateTag updates a tag (supports rename)
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

// GetSpecTags returns enabled tags for a spec
func (h *Handler) GetSpecTags(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "spec id is required"})
		return
	}

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"enabledTags": spec.EnabledTags})
}

// UpdateSpecTags updates enabled tags for a spec
func (h *Handler) UpdateSpecTags(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "spec id is required"})
		return
	}

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	var input struct {
		Tags []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validated, err := h.validateTagList(input.Tags)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	spec.EnabledTags = validated
	spec.UpdatedAt = time.Now()
	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"enabledTags": spec.EnabledTags})
}

func (h *Handler) validateTagList(tags []string) ([]string, error) {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := normalizeTag(tag)
		if normalized == "" || normalized == models.DefaultTagName {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		if _, err := h.store.GetTag(normalized); err != nil {
			return nil, err
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	sort.Strings(result)
	return result, nil
}

func (h *Handler) ensureTagExists(tag string) error {
	if tag == models.DefaultTagName {
		return nil
	}
	_, err := h.store.GetTag(tag)
	if err != nil {
		return err
	}
	return nil
}

func (h *Handler) replaceTagWithDefault(tag string) {
	ops, _ := h.store.GetAllOperations()
	for _, op := range ops {
		cfgs, _ := h.store.GetResponseConfigsByOperation(op.ID)
		for _, cfg := range cfgs {
			if normalizeTag(cfg.Tag) == tag {
				cfg.Tag = models.DefaultTagName
				h.store.UpdateResponseConfig(cfg)
			}
		}
	}

	specs, _ := h.store.GetAllSpecs()
	for _, spec := range specs {
		updated := make([]string, 0, len(spec.EnabledTags))
		for _, enabled := range spec.EnabledTags {
			if normalizeTag(enabled) != tag {
				updated = append(updated, normalizeTag(enabled))
			}
		}
		spec.EnabledTags = updated
		h.store.UpdateSpec(spec)
	}
}

func (h *Handler) renameTagUsage(oldName, newName string) {
	ops, _ := h.store.GetAllOperations()
	for _, op := range ops {
		cfgs, _ := h.store.GetResponseConfigsByOperation(op.ID)
		for _, cfg := range cfgs {
			if normalizeTag(cfg.Tag) == oldName {
				cfg.Tag = newName
				h.store.UpdateResponseConfig(cfg)
			}
		}
	}

	specs, _ := h.store.GetAllSpecs()
	for _, spec := range specs {
		updated := make([]string, 0, len(spec.EnabledTags))
		for _, enabled := range spec.EnabledTags {
			if normalizeTag(enabled) == oldName {
				updated = append(updated, newName)
			} else {
				updated = append(updated, normalizeTag(enabled))
			}
		}
		spec.EnabledTags = updated
		h.store.UpdateSpec(spec)
	}
}

func normalizeTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}

// SetBackendURI sets or clears the backend URI for a spec
func (h *Handler) SetBackendURI(c *gin.Context) {
	id := c.Param("id")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	var input struct {
		BackendURI string `json:"backendUri"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	spec.BackendURI = input.BackendURI
	if spec.BackendURI == "" {
		spec.ProxyMode = false
	}
	spec.UpdatedAt = time.Now()

	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.proxyEngine.ReloadRoutes()

	c.JSON(http.StatusOK, gin.H{"backendUri": spec.BackendURI, "proxyMode": spec.ProxyMode})
}

// ToggleProxyMode enables or disables proxy recording mode for a spec
func (h *Handler) ToggleProxyMode(c *gin.Context) {
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
		input.Enabled = !spec.ProxyMode
	}

	if input.Enabled && spec.BackendURI == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "backendUri must be set before enabling proxy mode"})
		return
	}

	spec.ProxyMode = input.Enabled
	spec.UpdatedAt = time.Now()

	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.proxyEngine.ReloadRoutes()

	c.JSON(http.StatusOK, gin.H{"proxyMode": spec.ProxyMode})
}

// GetSignatureConfig returns the signature configuration for an operation
func (h *Handler) GetSignatureConfig(c *gin.Context) {
	id := c.Param("id")

	op, err := h.store.GetOperation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Operation not found"})
		return
	}

	if op.SignatureConfig == nil {
		// Return default config (nil means "all path params + all query params + full body")
		c.JSON(http.StatusOK, gin.H{"signatureConfig": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"signatureConfig": op.SignatureConfig})
}

// UpdateSignatureConfig updates the signature configuration for an operation
func (h *Handler) UpdateSignatureConfig(c *gin.Context) {
	id := c.Param("id")

	op, err := h.store.GetOperation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Operation not found"})
		return
	}

	var input struct {
		SignatureConfig *models.SignatureConfig `json:"signatureConfig"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	op.SignatureConfig = input.SignatureConfig

	if err := h.store.UpdateOperation(op); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Reload routes so the engine picks up the new config
	h.proxyEngine.ReloadRoutes()

	c.JSON(http.StatusOK, gin.H{"signatureConfig": op.SignatureConfig})
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

// Version returns version metadata for the running binary
func (h *Handler) Version(c *gin.Context) {
	info := version.Get()
	c.JSON(http.StatusOK, info)
}

// SetBranding sets the branding configuration used by GetBranding.
func (h *Handler) SetBranding(b config.BrandingConfig) {
	if b.AppTitle == "" {
		b.AppTitle = "go-virtual"
	}
	if b.AppSubtitle == "" {
		b.AppSubtitle = "API Mock & Virtualization"
	}
	h.branding = b
}

// GetBranding returns the UI branding configuration (app title, subtitle).
func (h *Handler) GetBranding(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"appTitle":    h.branding.AppTitle,
		"appSubtitle": h.branding.AppSubtitle,
	})
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

// ---- Script handlers ----

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

	output, durationMs, execErr := h.scriptEngine.TestScript(c.Request.Context(), script, input)

	resp := gin.H{
		"output":     output,
		"durationMs": durationMs,
		"error":      nil,
	}
	if execErr != nil {
		resp["error"] = execErr.Error()
		resp["output"] = nil
	}

	c.JSON(http.StatusOK, resp)
}

// ---- Script Binding handlers ----

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
