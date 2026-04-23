package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

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

func (h *Handler) validateModePolicy(spec *models.Spec, policy models.ModePolicy) error {
	policy.Normalize()

	if err := validateConditions(policy.AI.Conditions); err != nil {
		return fmt.Errorf("invalid ai conditions: %w", err)
	}
	if hasSignatureCondition(policy.AI.Conditions) {
		return fmt.Errorf("invalid ai conditions: signature conditions are only supported on operation responses")
	}
	if err := validateConditions(policy.Proxy.Conditions); err != nil {
		return fmt.Errorf("invalid proxy conditions: %w", err)
	}
	if hasSignatureCondition(policy.Proxy.Conditions) {
		return fmt.Errorf("invalid proxy conditions: signature conditions are only supported on operation responses")
	}
	if policy.AI.Enabled && (h.aiGenerator == nil || !h.aiGenerator.IsConfigured()) {
		return fmt.Errorf("OpenAI API key must be configured before enabling AI mode")
	}
	if policy.Proxy.Enabled && (spec == nil || spec.BackendURI == "") {
		return fmt.Errorf("backendUri must be set before enabling proxy mode")
	}
	return nil
}

func hasSignatureCondition(conditions []models.Condition) bool {
	for _, cond := range conditions {
		if cond.Source == models.SourceSignature {
			return true
		}
	}
	return false
}

func validateConditions(conditions []models.Condition) error {
	validSources := make(map[string]struct{}, len(models.ValidSources()))
	for _, source := range models.ValidSources() {
		validSources[source] = struct{}{}
	}
	validOperators := make(map[string]struct{}, len(models.ValidOperators()))
	for _, operator := range models.ValidOperators() {
		validOperators[operator] = struct{}{}
	}

	for i, cond := range conditions {
		if _, ok := validSources[cond.Source]; !ok {
			return fmt.Errorf("condition %d has invalid source %q", i, cond.Source)
		}
		if _, ok := validOperators[cond.Operator]; !ok {
			return fmt.Errorf("condition %d has invalid operator %q", i, cond.Operator)
		}
		if cond.Source != models.SourceSignature && cond.Key == "" {
			return fmt.Errorf("condition %d key is required", i)
		}
	}
	return nil
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
