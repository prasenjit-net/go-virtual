package models

import "time"

// ValidationRule defines a named request validation attached to a spec or operation.
type ValidationRule struct {
	ID            string            `json:"id"`
	SpecID        string            `json:"specId,omitempty"`
	OperationID   string            `json:"operationId,omitempty"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Order         int               `json:"order"`
	Enabled       bool              `json:"enabled"`
	ConditionTree *ConditionNode    `json:"conditionTree"`
	OnSuccess     map[string]string `json:"onSuccess,omitempty"`
	OnFailure     map[string]string `json:"onFailure,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

// ValidationInput is the create/update payload (no id/timestamps).
type ValidationInput struct {
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Order         int               `json:"order"`
	Enabled       bool              `json:"enabled"`
	ConditionTree *ConditionNode    `json:"conditionTree"`
	OnSuccess     map[string]string `json:"onSuccess,omitempty"`
	OnFailure     map[string]string `json:"onFailure,omitempty"`
}

// ConditionNode is a node in the AND/OR/NOT condition tree.
// Either Condition (leaf) or Operator+Children (group) is set.
type ConditionNode struct {
	Condition *Condition       `json:"condition,omitempty"`
	Operator  string           `json:"operator,omitempty"` // "AND" | "OR" | "NOT"
	Children  []*ConditionNode `json:"children,omitempty"`
}

// ValidationResult holds the outcome of a single ValidationRule evaluation.
type ValidationResult struct {
	Status     string            // "pass" | "fail"
	Properties map[string]string // from OnSuccess or OnFailure
}

// ValidationTrace is the trace entry for one evaluated validation rule.
type ValidationTrace struct {
	RuleID     string            `json:"ruleId"`
	RuleName   string            `json:"ruleName"`
	Scope      string            `json:"scope"` // "spec" | "operation"
	Status     string            `json:"status"`
	Properties map[string]string `json:"properties,omitempty"`
	DurationMs int64             `json:"durationMs"`
}
