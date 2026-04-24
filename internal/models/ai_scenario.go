package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	AIScenarioKindSuccess = "success"
	AIScenarioKindError   = "error"

	DefaultAIScenarioSuccess     = "success"
	DefaultAIScenarioClientError = "client_error"
	DefaultAIScenarioServerError = "server_error"
)

// AIScenario steers runtime AI generation toward a specific response shape.
type AIScenario struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	ResponseKind string    `json:"responseKind"`
	StatusCode   int       `json:"statusCode,omitempty"`
	Count        int       `json:"count,omitempty"`
	Instructions string    `json:"instructions,omitempty"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func NormalizeAIScenarioKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case AIScenarioKindError:
		return AIScenarioKindError
	default:
		return AIScenarioKindSuccess
	}
}

// DefaultAIScenarios returns the default seeded scenarios for a new or
// previously unconfigured application.
func DefaultAIScenarios() []AIScenario {
	now := time.Now()
	return []AIScenario{
		{
			ID:           uuid.NewString(),
			Name:         DefaultAIScenarioSuccess,
			Description:  "Generate a successful response using the operation's default success status.",
			ResponseKind: AIScenarioKindSuccess,
			Enabled:      true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           uuid.NewString(),
			Name:         DefaultAIScenarioClientError,
			Description:  "Generate a client error response with status 400.",
			ResponseKind: AIScenarioKindError,
			StatusCode:   400,
			Enabled:      true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           uuid.NewString(),
			Name:         DefaultAIScenarioServerError,
			Description:  "Generate a server error response with status 500.",
			ResponseKind: AIScenarioKindError,
			StatusCode:   500,
			Enabled:      true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
}

func normalizeAIScenarioName(name string) string {
	return strings.TrimSpace(name)
}

func NormalizeAIScenarios(scenarios []AIScenario) []AIScenario {
	if len(scenarios) == 0 {
		return DefaultAIScenarios()
	}

	normalized := append([]AIScenario{}, scenarios...)
	for i := range normalized {
		scenario := &normalized[i]
		scenario.Name = normalizeAIScenarioName(scenario.Name)
		scenario.Description = strings.TrimSpace(scenario.Description)
		scenario.Instructions = strings.TrimSpace(scenario.Instructions)
		scenario.ResponseKind = NormalizeAIScenarioKind(scenario.ResponseKind)
		if scenario.ID == "" {
			scenario.ID = uuid.NewString()
		}
		if scenario.CreatedAt.IsZero() {
			scenario.CreatedAt = time.Now()
		}
		if scenario.UpdatedAt.IsZero() {
			scenario.UpdatedAt = scenario.CreatedAt
		}
	}
	return normalized
}

// FindAIScenario resolves an enabled or disabled scenario by name.
func FindAIScenario(scenarios []AIScenario, name string) *AIScenario {
	needle := strings.TrimSpace(name)
	if needle == "" {
		return nil
	}
	for i := range scenarios {
		if strings.EqualFold(scenarios[i].Name, needle) {
			return &scenarios[i]
		}
	}
	return nil
}
