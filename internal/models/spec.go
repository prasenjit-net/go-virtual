package models

import (
	"time"
)

// Spec represents an uploaded OpenAPI specification
type Spec struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	Description string      `json:"description"`
	Content     string      `json:"content"`  // Raw OpenAPI spec (YAML or JSON)
	BasePath    string      `json:"basePath"` // Mounted path prefix for this spec
	Enabled     bool        `json:"enabled"`
	Tracing     bool        `json:"tracing"` // Enable request tracing
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	Operations  []Operation `json:"operations,omitempty"`
}

// SpecInput represents input for creating/updating a spec
type SpecInput struct {
	Name        string `json:"name"`
	Content     string `json:"content"`
	BasePath    string `json:"basePath"`
	Description string `json:"description"`
}

// SpecUpdate represents input for updating spec settings
type SpecUpdate struct {
	Name        *string `json:"name,omitempty"`
	BasePath    *string `json:"basePath,omitempty"`
	Description *string `json:"description,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
	Tracing     *bool   `json:"tracing,omitempty"`
}
