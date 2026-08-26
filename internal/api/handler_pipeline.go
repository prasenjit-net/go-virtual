package api

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

// ListSpecPipeline returns all pipeline steps for a spec scope, sorted by Order.
func (h *Handler) ListSpecPipeline(c *gin.Context) {
	specID := c.Param("id")
	if _, err := h.store.GetSpec(specID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "spec not found"})
		return
	}
	steps, err := h.buildPipelineSteps(
		func() ([]*models.ScriptBinding, error) { return h.store.GetSpecScriptBindings(specID) },
		func() ([]*models.ValidationRule, error) { return h.store.ListValidationRulesBySpec(specID) },
		func() ([]*models.CollectionMapping, error) { return h.store.GetCollectionMappingsBySpec(specID) },
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"steps": steps})
}

// ReorderSpecPipeline reassigns Order values across all step types for a spec scope.
func (h *Handler) ReorderSpecPipeline(c *gin.Context) {
	specID := c.Param("id")
	if _, err := h.store.GetSpec(specID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "spec not found"})
		return
	}
	h.applyPipelineReorder(c,
		func() ([]*models.ScriptBinding, error) { return h.store.GetSpecScriptBindings(specID) },
		func() ([]*models.ValidationRule, error) { return h.store.ListValidationRulesBySpec(specID) },
		func() ([]*models.CollectionMapping, error) { return h.store.GetCollectionMappingsBySpec(specID) },
	)
}

// ListOperationPipeline returns all pipeline steps for an operation scope, sorted by Order.
func (h *Handler) ListOperationPipeline(c *gin.Context) {
	operationID := c.Param("id")
	if _, err := h.store.GetOperation(operationID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "operation not found"})
		return
	}
	steps, err := h.buildPipelineSteps(
		func() ([]*models.ScriptBinding, error) { return h.store.GetScriptBindings(operationID) },
		func() ([]*models.ValidationRule, error) { return h.store.ListValidationRulesByOperation(operationID) },
		func() ([]*models.CollectionMapping, error) { return h.store.GetCollectionMappingsByOperation(operationID) },
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"steps": steps})
}

// ReorderOperationPipeline reassigns Order values across all step types for an operation scope.
func (h *Handler) ReorderOperationPipeline(c *gin.Context) {
	operationID := c.Param("id")
	if _, err := h.store.GetOperation(operationID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "operation not found"})
		return
	}
	h.applyPipelineReorder(c,
		func() ([]*models.ScriptBinding, error) { return h.store.GetScriptBindings(operationID) },
		func() ([]*models.ValidationRule, error) { return h.store.ListValidationRulesByOperation(operationID) },
		func() ([]*models.CollectionMapping, error) { return h.store.GetCollectionMappingsByOperation(operationID) },
	)
}

// ListResponsePipeline returns all pipeline steps for a response scope (scripts + collections only).
func (h *Handler) ListResponsePipeline(c *gin.Context) {
	responseID := c.Param("id")
	if _, err := h.store.GetResponseConfig(responseID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "response config not found"})
		return
	}
	steps, err := h.buildPipelineSteps(
		func() ([]*models.ScriptBinding, error) { return h.store.GetResponseScriptBindings(responseID) },
		nil,
		func() ([]*models.CollectionMapping, error) { return h.store.GetCollectionMappingsByResponse(responseID) },
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"steps": steps})
}

// ReorderResponsePipeline reassigns Order values across step types for a response scope.
func (h *Handler) ReorderResponsePipeline(c *gin.Context) {
	responseID := c.Param("id")
	if _, err := h.store.GetResponseConfig(responseID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "response config not found"})
		return
	}
	h.applyPipelineReorder(c,
		func() ([]*models.ScriptBinding, error) { return h.store.GetResponseScriptBindings(responseID) },
		nil,
		func() ([]*models.CollectionMapping, error) { return h.store.GetCollectionMappingsByResponse(responseID) },
	)
}

// buildPipelineSteps merges bindings, rules (optional), and mappings into a sorted slice.
func (h *Handler) buildPipelineSteps(
	getBindings func() ([]*models.ScriptBinding, error),
	getRules func() ([]*models.ValidationRule, error),
	getMappings func() ([]*models.CollectionMapping, error),
) ([]models.PipelineStep, error) {
	var steps []models.PipelineStep

	if getBindings != nil {
		bindings, err := getBindings()
		if err != nil {
			return nil, err
		}
		for _, b := range bindings {
			if b != nil {
				b := b
				steps = append(steps, models.PipelineStep{
					Type:   models.PipelineStepScript,
					Order:  b.Order,
					Script: b,
				})
			}
		}
	}

	if getRules != nil {
		rules, err := getRules()
		if err != nil {
			return nil, err
		}
		for _, r := range rules {
			if r != nil {
				r := r
				steps = append(steps, models.PipelineStep{
					Type:       models.PipelineStepValidation,
					Order:      r.Order,
					Validation: r,
				})
			}
		}
	}

	if getMappings != nil {
		mappings, err := getMappings()
		if err != nil {
			return nil, err
		}
		for _, m := range mappings {
			if m != nil {
				m := m
				steps = append(steps, models.PipelineStep{
					Type:       models.PipelineStepCollection,
					Order:      m.Order,
					Collection: m,
				})
			}
		}
	}

	sort.Slice(steps, func(i, j int) bool {
		if steps[i].Order != steps[j].Order {
			return steps[i].Order < steps[j].Order
		}
		return pipelineTypeRank(steps[i].Type) < pipelineTypeRank(steps[j].Type)
	})

	if steps == nil {
		steps = []models.PipelineStep{}
	}
	return steps, nil
}

func pipelineTypeRank(t models.PipelineStepType) int {
	switch t {
	case models.PipelineStepScript:
		return 0
	case models.PipelineStepValidation:
		return 1
	default:
		return 2
	}
}

// applyPipelineReorder reads a []PipelineReorderItem from the request body and
// assigns Order = index * 10 to each referenced entity.
// getBindings/getRules/getMappings load all entities for the scope to look up by ID.
func (h *Handler) applyPipelineReorder(
	c *gin.Context,
	getBindings func() ([]*models.ScriptBinding, error),
	getRules func() ([]*models.ValidationRule, error),
	getMappings func() ([]*models.CollectionMapping, error),
) {
	var items []models.PipelineReorderItem
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build ID-keyed maps so we can look up entities without per-item storage queries.
	bindingByID := map[string]*models.ScriptBinding{}
	ruleByID := map[string]*models.ValidationRule{}
	mappingByID := map[string]*models.CollectionMapping{}

	if getBindings != nil {
		bs, err := getBindings()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, b := range bs {
			if b != nil {
				bindingByID[b.ID] = b
			}
		}
	}
	if getRules != nil {
		rs, err := getRules()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, r := range rs {
			if r != nil {
				ruleByID[r.ID] = r
			}
		}
	}
	if getMappings != nil {
		ms, err := getMappings()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, m := range ms {
			if m != nil {
				mappingByID[m.ID] = m
			}
		}
	}

	for i, item := range items {
		order := i * 10
		switch item.Type {
		case models.PipelineStepScript:
			b, ok := bindingByID[item.ID]
			if !ok {
				c.JSON(http.StatusNotFound, gin.H{"error": "script binding not found: " + item.ID})
				return
			}
			b.Order = order
			if err := h.store.UpdateScriptBinding(b); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		case models.PipelineStepValidation:
			r, ok := ruleByID[item.ID]
			if !ok {
				c.JSON(http.StatusNotFound, gin.H{"error": "validation rule not found: " + item.ID})
				return
			}
			r.Order = order
			if _, err := h.store.UpdateValidationRule(r); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		case models.PipelineStepCollection:
			m, ok := mappingByID[item.ID]
			if !ok {
				c.JSON(http.StatusNotFound, gin.H{"error": "collection mapping not found: " + item.ID})
				return
			}
			m.Order = order
			if err := h.store.UpdateCollectionMapping(m); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown step type: " + string(item.Type)})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"reordered": len(items)})
}
