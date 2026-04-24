package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prasenjit/go-virtual/internal/models"
)

// ListAIScenarios returns all global AI scenarios.
func (h *Handler) ListAIScenarios(c *gin.Context) {
	scenarios, err := h.store.ListAIScenarios()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"scenarios": scenarios})
}

// CreateAIScenario creates a global AI scenario.
func (h *Handler) CreateAIScenario(c *gin.Context) {
	var input struct {
		Scenario models.AIScenario `json:"scenario"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scenarios, err := h.store.ListAIScenarios()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	input.Scenario.ID = uuid.NewString()
	input.Scenario.CreatedAt = now
	input.Scenario.UpdatedAt = now

	next := make([]models.AIScenario, 0, len(scenarios)+1)
	for _, scenario := range scenarios {
		if scenario != nil {
			next = append(next, *scenario)
		}
	}
	next = append(next, input.Scenario)
	next = models.NormalizeAIScenarios(next)
	if err := validateAIScenarios(next); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created := next[len(next)-1]
	if err := h.store.CreateAIScenario(&created); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"scenario": created})
}

// UpdateAIScenario updates an existing global AI scenario.
func (h *Handler) UpdateAIScenario(c *gin.Context) {
	scenarioID := c.Param("scenarioId")

	current, err := h.store.GetAIScenario(scenarioID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Scenario not found"})
		return
	}

	var input struct {
		Scenario models.AIScenario `json:"scenario"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scenarios, err := h.store.ListAIScenarios()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	next := make([]models.AIScenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		if scenario == nil {
			continue
		}
		if scenario.ID == scenarioID {
			updated := input.Scenario
			updated.ID = current.ID
			updated.CreatedAt = current.CreatedAt
			updated.UpdatedAt = time.Now()
			next = append(next, updated)
			continue
		}
		next = append(next, *scenario)
	}

	next = models.NormalizeAIScenarios(next)
	if err := validateAIScenarios(next); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated := models.FindAIScenario(next, current.Name)
	if updated == nil {
		updated = models.FindAIScenario(next, input.Scenario.Name)
	}
	if updated == nil {
		for i := range next {
			if next[i].ID == scenarioID {
				updated = &next[i]
				break
			}
		}
	}
	if updated == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update scenario"})
		return
	}

	if err := h.store.UpdateAIScenario(updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"scenario": updated})
}

// DeleteAIScenario removes a global AI scenario.
func (h *Handler) DeleteAIScenario(c *gin.Context) {
	scenarioID := c.Param("scenarioId")

	if _, err := h.store.GetAIScenario(scenarioID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Scenario not found"})
		return
	}

	if err := h.store.DeleteAIScenario(scenarioID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scenario deleted"})
}
