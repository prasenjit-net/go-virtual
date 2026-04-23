package models

import (
	"time"
)

const (
	SpecModeStandard = "standard"
	SpecModeAI       = "ai"
	SpecModeProxy    = "proxy"
)

// Spec represents an uploaded OpenAPI specification
type Spec struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	Version            string      `json:"version"`
	Description        string      `json:"description"`
	Content            string      `json:"content"`  // Raw OpenAPI spec (YAML or JSON)
	BasePath           string      `json:"basePath"` // Mounted path prefix for this spec
	Enabled            bool        `json:"enabled"`
	Tracing            bool        `json:"tracing"`            // Enable request tracing
	UseExampleFallback bool        `json:"useExampleFallback"` // Use spec examples as fallback responses
	EnabledTags        []string    `json:"enabledTags"`
	Mode               string      `json:"mode"`
	BackendURI         string      `json:"backendUri"` // Upstream backend URI for proxy recording mode
	ProxyMode          bool        `json:"proxyMode"`  // Forward requests to backend and record responses
	ModePolicy         ModePolicy  `json:"modePolicy"`
	CreatedAt          time.Time   `json:"createdAt"`
	UpdatedAt          time.Time   `json:"updatedAt"`
	Operations         []Operation `json:"operations,omitempty"`
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
	Name               *string     `json:"name,omitempty"`
	BasePath           *string     `json:"basePath,omitempty"`
	Description        *string     `json:"description,omitempty"`
	Enabled            *bool       `json:"enabled,omitempty"`
	Tracing            *bool       `json:"tracing,omitempty"`
	UseExampleFallback *bool       `json:"useExampleFallback,omitempty"`
	Mode               *string     `json:"mode,omitempty"`
	BackendURI         *string     `json:"backendUri,omitempty"`
	ProxyMode          *bool       `json:"proxyMode,omitempty"`
	ModePolicy         *ModePolicy `json:"modePolicy,omitempty"`
}

func NormalizeSpecMode(mode string) string {
	switch mode {
	case SpecModeAI, SpecModeProxy:
		return mode
	default:
		return SpecModeStandard
	}
}

func (s *Spec) NormalizeMode() {
	if s == nil {
		return
	}
	s.ModePolicy = s.EffectiveModePolicy()
	s.Mode = s.EffectiveMode()
	s.ProxyMode = s.Mode == SpecModeProxy
}

func (s *Spec) SetMode(mode string) {
	if s == nil {
		return
	}
	s.Mode = NormalizeSpecMode(mode)
	s.ProxyMode = s.Mode == SpecModeProxy
}

// EffectiveMode returns the primary configured fallback mode for compatibility
// with older API fields. Request-time mode selection still depends on the spec's
// conditional mode policy and current runtime availability.
func (s *Spec) EffectiveMode() string {
	if s == nil {
		return SpecModeStandard
	}
	policy := s.EffectiveModePolicy()
	if policy.AI.Enabled {
		return SpecModeAI
	}
	if policy.Proxy.Enabled {
		return SpecModeProxy
	}
	if policy.Configured {
		return SpecModeStandard
	}
	if s.Mode == "" {
		if s.ProxyMode {
			return SpecModeProxy
		}
		return SpecModeStandard
	}
	return NormalizeSpecMode(s.Mode)
}

func (s *Spec) EffectiveModePolicy() ModePolicy {
	if s == nil {
		return DefaultModePolicy()
	}
	policy := s.ModePolicy
	if !policy.Configured {
		policy = LegacyModePolicy(s)
	}
	policy.Normalize()
	return policy
}
