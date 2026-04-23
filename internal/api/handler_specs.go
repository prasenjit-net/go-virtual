package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prasenjit/go-virtual/internal/models"
)

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
			"mode":               spec.Mode,
			"backendUri":         spec.BackendURI,
			"proxyMode":          spec.ProxyMode,
			"modePolicy":         spec.EffectiveModePolicy(),
			"aiScenarios":        spec.AIScenarios,
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
		"id":                 parseResult.Spec.ID,
		"name":               parseResult.Spec.Name,
		"version":            parseResult.Spec.Version,
		"description":        parseResult.Spec.Description,
		"basePath":           parseResult.Spec.BasePath,
		"enabled":            parseResult.Spec.Enabled,
		"tracing":            parseResult.Spec.Tracing,
		"useExampleFallback": parseResult.Spec.UseExampleFallback,
		"enabledTags":        parseResult.Spec.EnabledTags,
		"mode":               parseResult.Spec.EffectiveMode(),
		"backendUri":         parseResult.Spec.BackendURI,
		"proxyMode":          parseResult.Spec.ProxyMode,
		"modePolicy":         parseResult.Spec.EffectiveModePolicy(),
		"aiScenarios":        parseResult.Spec.AIScenarios,
		"createdAt":          parseResult.Spec.CreatedAt,
		"updatedAt":          parseResult.Spec.UpdatedAt,
		"operationCount":     len(parseResult.Operations),
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
		if spec.BackendURI == "" {
			policy := spec.EffectiveModePolicy()
			policy.Proxy.Enabled = false
			if err := h.setSpecModePolicy(spec, policy); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
	}
	if update.Mode != nil {
		if err := h.applySpecMode(spec, *update.Mode); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if update.ProxyMode != nil {
		targetMode := spec.EffectiveMode()
		if *update.ProxyMode {
			targetMode = models.SpecModeProxy
		} else if spec.EffectiveMode() == models.SpecModeProxy {
			targetMode = models.SpecModeStandard
		}
		if err := h.applySpecMode(spec, targetMode); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if update.ModePolicy != nil {
		if err := h.setSpecModePolicy(spec, *update.ModePolicy); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if update.AIScenarios != nil {
		if err := validateAIScenarios(*update.AIScenarios); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		spec.AIScenarios = *update.AIScenarios
		spec.NormalizeAIScenarios()
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
		policy := spec.EffectiveModePolicy()
		policy.Proxy.Enabled = false
		if err := h.setSpecModePolicy(spec, policy); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	spec.UpdatedAt = time.Now()

	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.proxyEngine.ReloadRoutes()

	c.JSON(http.StatusOK, gin.H{"backendUri": spec.BackendURI, "proxyMode": spec.ProxyMode})
}

// SetSpecMode sets the execution mode for a spec.
func (h *Handler) SetSpecMode(c *gin.Context) {
	id := c.Param("id")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	var input struct {
		Mode string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.applySpecMode(spec, input.Mode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	spec.UpdatedAt = time.Now()

	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.proxyEngine.ReloadRoutes()

	c.JSON(http.StatusOK, gin.H{"mode": spec.Mode, "proxyMode": spec.ProxyMode})
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

	targetMode := spec.EffectiveMode()
	if input.Enabled {
		targetMode = models.SpecModeProxy
	} else if spec.EffectiveMode() == models.SpecModeProxy {
		targetMode = models.SpecModeStandard
	}
	if err := h.applySpecMode(spec, targetMode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	spec.UpdatedAt = time.Now()

	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.proxyEngine.ReloadRoutes()

	c.JSON(http.StatusOK, gin.H{"proxyMode": spec.ProxyMode})
}

func (h *Handler) applySpecMode(spec *models.Spec, mode string) error {
	mode = models.NormalizeSpecMode(mode)
	policy := spec.EffectiveModePolicy()
	switch mode {
	case models.SpecModeProxy:
		if spec.BackendURI == "" {
			return fmt.Errorf("backendUri must be set before enabling proxy mode")
		}
		policy.Configured = true
		policy.AI.Enabled = false
		policy.Proxy.Enabled = true
	case models.SpecModeAI:
		if h.aiGenerator == nil || !h.aiGenerator.IsConfigured() {
			return fmt.Errorf("OpenAI API key must be configured before enabling AI mode")
		}
		policy.Configured = true
		policy.AI.Enabled = true
		policy.Proxy.Enabled = false
	default:
		policy.Configured = true
		policy.AI.Enabled = false
		policy.Proxy.Enabled = false
	}
	return h.setSpecModePolicy(spec, policy)
}

func (h *Handler) setSpecModePolicy(spec *models.Spec, policy models.ModePolicy) error {
	policy.Normalize()
	if err := h.validateModePolicy(spec, policy); err != nil {
		return err
	}
	policy.Configured = true
	spec.ModePolicy = policy
	spec.NormalizeMode()
	return nil
}

// GetSpecModePolicy returns the spec-scoped fallback mode policy.
func (h *Handler) GetSpecModePolicy(c *gin.Context) {
	id := c.Param("id")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"modePolicy": spec.EffectiveModePolicy()})
}

// UpdateSpecModePolicy updates the spec-scoped fallback mode policy.
func (h *Handler) UpdateSpecModePolicy(c *gin.Context) {
	id := c.Param("id")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	var input struct {
		ModePolicy models.ModePolicy `json:"modePolicy"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.setSpecModePolicy(spec, input.ModePolicy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	spec.UpdatedAt = time.Now()

	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.proxyEngine.ReloadRoutes()
	c.JSON(http.StatusOK, gin.H{"modePolicy": spec.ModePolicy})
}

// ListAIScenarios returns all AI scenarios for a spec.
func (h *Handler) ListAIScenarios(c *gin.Context) {
	id := c.Param("id")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"scenarios": spec.AIScenarios})
}

// CreateAIScenario adds a new AI scenario to a spec.
func (h *Handler) CreateAIScenario(c *gin.Context) {
	id := c.Param("id")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	var input struct {
		Scenario models.AIScenario `json:"scenario"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	input.Scenario.ID = uuid.NewString()
	input.Scenario.CreatedAt = now
	input.Scenario.UpdatedAt = now

	next := append(append([]models.AIScenario{}, spec.AIScenarios...), input.Scenario)
	if err := validateAIScenarios(next); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	spec.AIScenarios = next
	spec.NormalizeAIScenarios()
	spec.UpdatedAt = now
	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"scenario": scenarioByID(spec.AIScenarios, input.Scenario.ID)})
}

// UpdateAIScenario updates an existing AI scenario on a spec.
func (h *Handler) UpdateAIScenario(c *gin.Context) {
	id := c.Param("id")
	scenarioID := c.Param("scenarioId")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	var input struct {
		Scenario models.AIScenario `json:"scenario"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	index := scenarioIndexByID(spec.AIScenarios, scenarioID)
	if index < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Scenario not found"})
		return
	}

	current := spec.AIScenarios[index]
	updated := input.Scenario
	updated.ID = current.ID
	updated.CreatedAt = current.CreatedAt
	updated.UpdatedAt = time.Now()

	next := append([]models.AIScenario{}, spec.AIScenarios...)
	next[index] = updated
	if err := validateAIScenarios(next); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	spec.AIScenarios = next
	spec.NormalizeAIScenarios()
	spec.UpdatedAt = time.Now()
	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"scenario": scenarioByID(spec.AIScenarios, scenarioID)})
}

// DeleteAIScenario removes an AI scenario from a spec.
func (h *Handler) DeleteAIScenario(c *gin.Context) {
	id := c.Param("id")
	scenarioID := c.Param("scenarioId")

	spec, err := h.store.GetSpec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		return
	}

	index := scenarioIndexByID(spec.AIScenarios, scenarioID)
	if index < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Scenario not found"})
		return
	}

	spec.AIScenarios = append(spec.AIScenarios[:index], spec.AIScenarios[index+1:]...)
	spec.UpdatedAt = time.Now()
	if err := h.store.UpdateSpec(spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scenario deleted"})
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

// ── Tag helpers ───────────────────────────────────────────────────────────────

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
	return err
}

func validateAIScenarios(scenarios []models.AIScenario) error {
	seenNames := make(map[string]struct{}, len(scenarios))
	for i := range scenarios {
		scenario := scenarios[i]
		name := strings.TrimSpace(scenario.Name)
		if name == "" {
			return fmt.Errorf("scenario %d name is required", i)
		}
		key := strings.ToLower(name)
		if _, exists := seenNames[key]; exists {
			return fmt.Errorf("scenario name %q already exists", name)
		}
		seenNames[key] = struct{}{}

		if scenario.StatusCode < 0 || scenario.StatusCode > 999 {
			return fmt.Errorf("scenario %q has invalid status code %d", name, scenario.StatusCode)
		}
		if scenario.Count < 0 {
			return fmt.Errorf("scenario %q count must be zero or greater", name)
		}
	}
	return nil
}

func scenarioIndexByID(scenarios []models.AIScenario, scenarioID string) int {
	for i := range scenarios {
		if scenarios[i].ID == scenarioID {
			return i
		}
	}
	return -1
}

func scenarioByID(scenarios []models.AIScenario, scenarioID string) *models.AIScenario {
	for i := range scenarios {
		if scenarios[i].ID == scenarioID {
			return &scenarios[i]
		}
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
