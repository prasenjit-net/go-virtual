package models

import "testing"

func TestSpecEffectiveModePrefersAIWhenConditionalPolicyEnablesBoth(t *testing.T) {
	spec := &Spec{
		ModePolicy: ModePolicy{
			Configured: true,
			AI:         ConditionalModeConfig{Enabled: true},
			Proxy:      ConditionalModeConfig{Enabled: true},
		},
	}

	if got := spec.EffectiveMode(); got != SpecModeAI {
		t.Fatalf("expected primary mode %q, got %q", SpecModeAI, got)
	}
}

func TestSpecEffectiveModeFallsBackToProxyWhenOnlyProxyEnabled(t *testing.T) {
	spec := &Spec{
		ModePolicy: ModePolicy{
			Configured: true,
			AI:         ConditionalModeConfig{Enabled: false},
			Proxy:      ConditionalModeConfig{Enabled: true},
		},
	}

	if got := spec.EffectiveMode(); got != SpecModeProxy {
		t.Fatalf("expected primary mode %q, got %q", SpecModeProxy, got)
	}
}

func TestSpecEffectiveModeConfiguredStandardOverridesLegacyMode(t *testing.T) {
	spec := &Spec{
		Mode: SpecModeProxy,
		ModePolicy: ModePolicy{
			Configured: true,
			AI:         ConditionalModeConfig{Enabled: false},
			Proxy:      ConditionalModeConfig{Enabled: false},
		},
	}

	if got := spec.EffectiveMode(); got != SpecModeStandard {
		t.Fatalf("expected primary mode %q, got %q", SpecModeStandard, got)
	}
}

func TestNormalizeSpecModeAndSetMode(t *testing.T) {
	if got := NormalizeSpecMode("unexpected"); got != SpecModeStandard {
		t.Fatalf("NormalizeSpecMode returned %q", got)
	}

	spec := &Spec{}
	spec.SetMode(SpecModeProxy)
	if spec.Mode != SpecModeProxy || !spec.ProxyMode {
		t.Fatalf("SetMode did not set proxy mode correctly: %+v", spec)
	}

	spec.SetMode("invalid")
	if spec.Mode != SpecModeStandard || spec.ProxyMode {
		t.Fatalf("SetMode should normalize to standard and clear proxy mode: %+v", spec)
	}
}

func TestSpecNormalizeModeAndEffectiveModePolicy(t *testing.T) {
	spec := &Spec{Mode: SpecModeAI, SignatureHeaders: []string{" x-tenant ", "X-Virtual-Test", "X-Tenant"}}
	spec.NormalizeMode()

	if !spec.ModePolicy.AI.Enabled || spec.ModePolicy.Proxy.Enabled {
		t.Fatalf("NormalizeMode should derive AI policy, got %+v", spec.ModePolicy)
	}
	if len(spec.SignatureHeaders) != 1 || spec.SignatureHeaders[0] != "X-Tenant" {
		t.Fatalf("NormalizeMode should normalize signature headers, got %#v", spec.SignatureHeaders)
	}

	legacyProxy := (&Spec{ProxyMode: true}).EffectiveModePolicy()
	if !legacyProxy.Proxy.Enabled || legacyProxy.AI.Enabled {
		t.Fatalf("EffectiveModePolicy should derive legacy proxy mode, got %+v", legacyProxy)
	}
}

func TestNormalizeSignatureConfig(t *testing.T) {
	includeBody := true
	cfg := &SignatureConfig{
		PathParams:        []string{"id", " id ", "tenant"},
		QueryParams:       []string{"status", "status"},
		HeadersConfigured: true,
		Headers:           []string{" x-tenant ", "X-Virtual-Test", "X-Tenant"},
		IncludeBody:       &includeBody,
		BodyJsonPaths:     []string{"user.id", " user.id ", "meta.ts"},
	}

	cfg.Normalize()

	if len(cfg.PathParams) != 2 || cfg.PathParams[0] != "id" || cfg.PathParams[1] != "tenant" {
		t.Fatalf("unexpected normalized path params: %#v", cfg.PathParams)
	}
	if len(cfg.QueryParams) != 1 || cfg.QueryParams[0] != "status" {
		t.Fatalf("unexpected normalized query params: %#v", cfg.QueryParams)
	}
	if len(cfg.Headers) != 1 || cfg.Headers[0] != "X-Tenant" {
		t.Fatalf("unexpected normalized headers: %#v", cfg.Headers)
	}
	if len(cfg.BodyJsonPaths) != 2 {
		t.Fatalf("unexpected normalized body paths: %#v", cfg.BodyJsonPaths)
	}
}
