package models

// ResponseConfig represents a configured response for an operation
type ResponseConfig struct {
	ID          string            `json:"id"`
	OperationID string            `json:"operationId"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Priority    int               `json:"priority"` // Lower = higher priority (0 is highest)
	Conditions  []Condition       `json:"conditions"`
	StatusCode  int               `json:"statusCode"`
	Headers     map[string]string `json:"headers"` // Can contain template variables
	Body        string            `json:"body"`    // Can contain template variables
	Delay       int               `json:"delay"`   // Response delay in milliseconds
	Enabled     bool              `json:"enabled"`
}

// ResponseConfigInput represents input for creating/updating a response config
type ResponseConfigInput struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Priority    int               `json:"priority"`
	Conditions  []Condition       `json:"conditions"`
	StatusCode  int               `json:"statusCode"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	Delay       int               `json:"delay"`
	Enabled     bool              `json:"enabled"`
}

// ResponseConfigUpdate represents input for updating a response config
type ResponseConfigUpdate struct {
	Name        *string            `json:"name,omitempty"`
	Description *string            `json:"description,omitempty"`
	Priority    *int               `json:"priority,omitempty"`
	Conditions  *[]Condition       `json:"conditions,omitempty"`
	StatusCode  *int               `json:"statusCode,omitempty"`
	Headers     *map[string]string `json:"headers,omitempty"`
	Body        *string            `json:"body,omitempty"`
	Delay       *int               `json:"delay,omitempty"`
	Enabled     *bool              `json:"enabled,omitempty"`
}
