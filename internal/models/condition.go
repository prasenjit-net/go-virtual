package models

// Condition represents a condition for matching requests
type Condition struct {
	Source   string `json:"source"`   // path, query, header, body
	Key      string `json:"key"`      // Parameter name or JSONPath for body
	Operator string `json:"operator"` // eq, ne, contains, regex, exists, notExists, gt, lt, gte, lte
	Value    string `json:"value"`    // Expected value (can be template)
}

// Supported condition sources
const (
	SourcePath   = "path"
	SourceQuery  = "query"
	SourceHeader = "header"
	SourceBody   = "body"
)

// Supported condition operators
const (
	OpEquals      = "eq"
	OpNotEquals   = "ne"
	OpContains    = "contains"
	OpNotContains = "notContains"
	OpRegex       = "regex"
	OpExists      = "exists"
	OpNotExists   = "notExists"
	OpGreaterThan = "gt"
	OpLessThan    = "lt"
	OpGTE         = "gte"
	OpLTE         = "lte"
	OpStartsWith  = "startsWith"
	OpEndsWith    = "endsWith"
)

// ValidSources returns all valid condition sources
func ValidSources() []string {
	return []string{SourcePath, SourceQuery, SourceHeader, SourceBody}
}

// ValidOperators returns all valid condition operators
func ValidOperators() []string {
	return []string{
		OpEquals, OpNotEquals, OpContains, OpNotContains,
		OpRegex, OpExists, OpNotExists, OpGreaterThan,
		OpLessThan, OpGTE, OpLTE, OpStartsWith, OpEndsWith,
	}
}
