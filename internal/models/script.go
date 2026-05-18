package models

import "time"

// Script represents a user-defined Starlark script resource.
type Script struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Timeout     int       `json:"timeout"`  // Max execution time in ms (0 = use global default)
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	// Source is loaded from <id>.star; intentionally excluded from JSON serialisation.
	Source string `json:"-"`
}

// ScriptInput is used for create/update API calls.
type ScriptInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"` // Starlark source code
	Timeout     int    `json:"timeout"`
	Enabled     bool   `json:"enabled"`
}

// ScriptBinding attaches a Script to a Spec, an Operation, or a ResponseConfig.
// Exactly one of SpecID, OperationID, or ResponseConfigID will be non-empty.
type ScriptBinding struct {
	ID               string `json:"id"`
	SpecID           string `json:"specId,omitempty"`
	OperationID      string `json:"operationId,omitempty"`
	ResponseConfigID string `json:"responseConfigId,omitempty"`
	ScriptID         string `json:"scriptId"`
	ScriptName       string `json:"scriptName,omitempty"` // Denormalised for display
	OutputKey        string `json:"outputKey"`            // Namespace under {{.script.<outputKey>.*}}
	Order            int    `json:"order"`                // Execution order, ascending
	Enabled          bool   `json:"enabled"`
}

// IsSpecBinding reports whether this binding is attached to a spec.
func (b *ScriptBinding) IsSpecBinding() bool {
	return b.SpecID != ""
}

// IsResponseBinding reports whether this binding is attached to a response config
// rather than an operation.
func (b *ScriptBinding) IsResponseBinding() bool {
	return b.ResponseConfigID != ""
}

// ScriptBindingInput is used for create/update binding API calls.
type ScriptBindingInput struct {
	ScriptID  string `json:"scriptId"`
	OutputKey string `json:"outputKey"`
	Order     int    `json:"order"`
	Enabled   bool   `json:"enabled"`
}

// ScriptTrace captures the execution result of a single script binding within a request trace.
type ScriptTrace struct {
	BindingID  string   `json:"bindingId"`
	ScriptID   string   `json:"scriptId"`
	ScriptName string   `json:"scriptName"`
	OutputKey  string   `json:"outputKey"`
	DurationMs float64  `json:"durationMs"`
	Output     any      `json:"output,omitempty"`
	Error      string   `json:"error,omitempty"`
	Logs       []string `json:"logs,omitempty"`
}
