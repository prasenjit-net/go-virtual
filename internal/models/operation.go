package models

// ConditionalModeConfig controls whether a fallback mode is enabled and, when
// enabled, which request conditions must match before it is selected.
type ConditionalModeConfig struct {
	Enabled           bool        `json:"enabled"`
	DisableRecording  bool        `json:"disableRecording,omitempty"`
	Conditions        []Condition `json:"conditions,omitempty"`
}

// ModePolicy defines fallback mode behavior.
// Standard fallback is implicit; AI and proxy are optional conditional modes.
type ModePolicy struct {
	Configured bool                  `json:"configured,omitempty"`
	AI         ConditionalModeConfig `json:"ai"`
	Proxy      ConditionalModeConfig `json:"proxy"`
}

// DefaultModePolicy returns the default fallback policy.
func DefaultModePolicy() ModePolicy {
	return ModePolicy{
		Configured: false,
		AI:         ConditionalModeConfig{Enabled: false, Conditions: []Condition{}},
		Proxy:      ConditionalModeConfig{Enabled: false, Conditions: []Condition{}},
	}
}

// Normalize ensures slices are non-nil so API responses remain stable.
func (p *ModePolicy) Normalize() {
	if p == nil {
		return
	}
	if p.AI.Conditions == nil {
		p.AI.Conditions = []Condition{}
	}
	if p.Proxy.Conditions == nil {
		p.Proxy.Conditions = []Condition{}
	}
}

// LegacyModePolicy maps the legacy spec-wide mode model to the new
// conditional fallback policy for backward compatibility.
func LegacyModePolicy(spec *Spec) ModePolicy {
	policy := DefaultModePolicy()
	if spec == nil {
		return policy
	}

	mode := NormalizeSpecMode(spec.Mode)
	if spec.Mode == "" && spec.ProxyMode {
		mode = SpecModeProxy
	}

	switch mode {
	case SpecModeAI:
		policy.AI.Enabled = true
	case SpecModeProxy:
		policy.Proxy.Enabled = true
	}
	return policy
}

// Operation represents an API operation from an OpenAPI spec
type Operation struct {
	ID                   string           `json:"id"`
	SpecID               string           `json:"specId"`
	Method               string           `json:"method"`      // GET, POST, PUT, DELETE, PATCH, etc.
	Path                 string           `json:"path"`        // Path pattern e.g., /users/{id}
	FullPath             string           `json:"fullPath"`    // BasePath + Path
	OperationID          string           `json:"operationId"` // From OpenAPI spec
	Summary              string           `json:"summary"`
	Description          string           `json:"description"`
	Tags                 []string         `json:"tags"`
	Responses            []ResponseConfig `json:"responses,omitempty"`
	ExampleResponse      *ExampleResponse `json:"exampleResponse,omitempty"` // From OpenAPI spec
	SignatureConfig      *SignatureConfig `json:"signatureConfig,omitempty"` // Controls request signature generation
	DeclaredPathParams   []string         `json:"declaredPathParams,omitempty"`
	DeclaredQueryParams  []string         `json:"declaredQueryParams,omitempty"`
	DeclaredHeaderParams []string         `json:"declaredHeaderParams,omitempty"`
	DeclaredBodyFields   []string         `json:"declaredBodyFields,omitempty"`
	HasRequestBody       bool             `json:"hasRequestBody,omitempty"`
}

// ExampleResponse holds example response data from the OpenAPI spec
type ExampleResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body"`
}

// OperationSummary is a lightweight version for listings
type OperationSummary struct {
	ID                 string `json:"id"`
	SpecID             string `json:"specId"`
	Method             string `json:"method"`
	Path               string `json:"path"`
	FullPath           string `json:"fullPath"`
	OperationID        string `json:"operationId"`
	Summary            string `json:"summary"`
	ResponseCount      int    `json:"responseCount"`
	HasExampleResponse bool   `json:"hasExampleResponse"`
}
