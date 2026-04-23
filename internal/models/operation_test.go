package models

import "testing"

func TestDefaultModePolicy(t *testing.T) {
	policy := DefaultModePolicy()
	if policy.Configured {
		t.Fatal("default policy should not be configured")
	}
	if policy.AI.Enabled || policy.Proxy.Enabled {
		t.Fatal("default policy should disable AI and proxy")
	}
	if policy.AI.Conditions == nil || policy.Proxy.Conditions == nil {
		t.Fatal("default policy should initialize condition slices")
	}
}

func TestModePolicyNormalize(t *testing.T) {
	policy := &ModePolicy{}
	policy.Normalize()
	if policy.AI.Conditions == nil || policy.Proxy.Conditions == nil {
		t.Fatal("Normalize should initialize nil condition slices")
	}
}

func TestLegacyModePolicy(t *testing.T) {
	tests := []struct {
		name      string
		spec      *Spec
		wantAI    bool
		wantProxy bool
	}{
		{name: "nil", spec: nil},
		{name: "legacy ai mode", spec: &Spec{Mode: SpecModeAI}, wantAI: true},
		{name: "legacy proxy mode", spec: &Spec{Mode: SpecModeProxy}, wantProxy: true},
		{name: "legacy proxy bool", spec: &Spec{ProxyMode: true}, wantProxy: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := LegacyModePolicy(tt.spec)
			if policy.AI.Enabled != tt.wantAI || policy.Proxy.Enabled != tt.wantProxy {
				t.Fatalf("LegacyModePolicy() => ai=%v proxy=%v, want ai=%v proxy=%v", policy.AI.Enabled, policy.Proxy.Enabled, tt.wantAI, tt.wantProxy)
			}
		})
	}
}
