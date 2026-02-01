package models

// Operation represents an API operation from an OpenAPI spec
type Operation struct {
	ID          string           `json:"id"`
	SpecID      string           `json:"specId"`
	Method      string           `json:"method"`      // GET, POST, PUT, DELETE, PATCH, etc.
	Path        string           `json:"path"`        // Path pattern e.g., /users/{id}
	FullPath    string           `json:"fullPath"`    // BasePath + Path
	OperationID string           `json:"operationId"` // From OpenAPI spec
	Summary     string           `json:"summary"`
	Description string           `json:"description"`
	Tags        []string         `json:"tags"`
	Responses   []ResponseConfig `json:"responses,omitempty"`
}

// OperationSummary is a lightweight version for listings
type OperationSummary struct {
	ID            string `json:"id"`
	SpecID        string `json:"specId"`
	Method        string `json:"method"`
	Path          string `json:"path"`
	FullPath      string `json:"fullPath"`
	OperationID   string `json:"operationId"`
	Summary       string `json:"summary"`
	ResponseCount int    `json:"responseCount"`
}
