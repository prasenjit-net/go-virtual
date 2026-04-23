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
