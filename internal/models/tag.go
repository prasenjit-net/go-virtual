package models

import "time"

const DefaultTagName = "default"

// Tag represents a global response tag
// Tags are global across the application and can be enabled per spec.
type Tag struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
