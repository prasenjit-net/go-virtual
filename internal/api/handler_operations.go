package api

import (
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
