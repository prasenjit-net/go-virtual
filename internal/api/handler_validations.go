package api

import (
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

var validationNameRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ListSpecValidations returns all validation rules for a spec.
func (h *Handler) ListSpecValidations(c *gin.Context) {
	specID := c.Param("id")
	if _, err := h.store.GetSpec(specID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "spec not found"})
		return
	}

	rules, err := h.store.ListValidationRulesBySpec(specID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rules)
}

// ListOperationValidations returns all validation rules for an operation.
func (h *Handler) ListOperationValidations(c *gin.Context) {
	operationID := c.Param("id")
	if _, err := h.store.GetOperation(operationID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "operation not found"})
		return
	}

	rules, err := h.store.ListValidationRulesByOperation(operationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rules)
}

// CreateSpecValidation creates a new validation rule for a spec.
func (h *Handler) CreateSpecValidation(c *gin.Context) {
	specID := c.Param("id")
	if _, err := h.store.GetSpec(specID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "spec not found"})
		return
	}

	var input models.ValidationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if !validationNameRe.MatchString(input.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must match ^[a-zA-Z_][a-zA-Z0-9_]*$"})
		return
	}

	now := time.Now()
	rule := &models.ValidationRule{
		ID:            generateID(),
		SpecID:        specID,
		Name:          input.Name,
		Description:   input.Description,
		Order:         input.Order,
		Enabled:       input.Enabled,
		ConditionTree: input.ConditionTree,
		OnSuccess:     input.OnSuccess,
		OnFailure:     input.OnFailure,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	result, err := h.store.CreateValidationRule(rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// CreateOperationValidation creates a new validation rule for an operation.
func (h *Handler) CreateOperationValidation(c *gin.Context) {
	operationID := c.Param("id")
	if _, err := h.store.GetOperation(operationID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "operation not found"})
		return
	}

	var input models.ValidationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if !validationNameRe.MatchString(input.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must match ^[a-zA-Z_][a-zA-Z0-9_]*$"})
		return
	}

	now := time.Now()
	rule := &models.ValidationRule{
		ID:            generateID(),
		OperationID:   operationID,
		Name:          input.Name,
		Description:   input.Description,
		Order:         input.Order,
		Enabled:       input.Enabled,
		ConditionTree: input.ConditionTree,
		OnSuccess:     input.OnSuccess,
		OnFailure:     input.OnFailure,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	result, err := h.store.CreateValidationRule(rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// GetValidation retrieves a single validation rule by ID.
func (h *Handler) GetValidation(c *gin.Context) {
	id := c.Param("id")

	rule, err := h.store.GetValidationRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "validation rule not found"})
		return
	}

	c.JSON(http.StatusOK, rule)
}

// UpdateValidation updates a validation rule.
func (h *Handler) UpdateValidation(c *gin.Context) {
	id := c.Param("id")

	existing, err := h.store.GetValidationRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "validation rule not found"})
		return
	}

	var input models.ValidationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if !validationNameRe.MatchString(input.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must match ^[a-zA-Z_][a-zA-Z0-9_]*$"})
		return
	}

	existing.Name = input.Name
	existing.Description = input.Description
	existing.Order = input.Order
	existing.Enabled = input.Enabled
	existing.ConditionTree = input.ConditionTree
	existing.OnSuccess = input.OnSuccess
	existing.OnFailure = input.OnFailure
	existing.UpdatedAt = time.Now()

	result, err := h.store.UpdateValidationRule(existing)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DeleteValidation deletes a validation rule by ID.
func (h *Handler) DeleteValidation(c *gin.Context) {
	id := c.Param("id")

	if _, err := h.store.GetValidationRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "validation rule not found"})
		return
	}

	if err := h.store.DeleteValidationRule(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
