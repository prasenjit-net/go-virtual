package models

import "testing"

func TestNormalizeResponseOrigin(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		recorded bool
		want     string
	}{
		{name: "proxy", origin: ResponseOriginProxy, recorded: true, want: ResponseOriginProxy},
		{name: "ai", origin: ResponseOriginAI, recorded: true, want: ResponseOriginAI},
		{name: "manual", origin: ResponseOriginManual, recorded: false, want: ResponseOriginManual},
		{name: "unknown recorded", origin: "other", recorded: true, want: ResponseOriginProxy},
		{name: "unknown manual", origin: "other", recorded: false, want: ResponseOriginManual},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeResponseOrigin(tt.origin, tt.recorded); got != tt.want {
				t.Fatalf("NormalizeResponseOrigin(%q, %v) = %q, want %q", tt.origin, tt.recorded, got, tt.want)
			}
		})
	}
}

func TestResponseConfigNormalizeOrigin(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *ResponseConfig
		wantOrig string
		wantRec  bool
	}{
		{
			name:     "manual clears recorded flag",
			cfg:      &ResponseConfig{Origin: ResponseOriginManual, Recorded: true},
			wantOrig: ResponseOriginManual,
			wantRec:  false,
		},
		{
			name:     "proxy sets recorded flag",
			cfg:      &ResponseConfig{Origin: ResponseOriginProxy, Recorded: false},
			wantOrig: ResponseOriginProxy,
			wantRec:  true,
		},
		{
			name:     "ai preserves recorded flag",
			cfg:      &ResponseConfig{Origin: ResponseOriginAI, Recorded: true},
			wantOrig: ResponseOriginAI,
			wantRec:  true,
		},
		{
			name:     "unknown recorded becomes proxy",
			cfg:      &ResponseConfig{Origin: "other", Recorded: true},
			wantOrig: ResponseOriginProxy,
			wantRec:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.NormalizeOrigin()
			if tt.cfg.Origin != tt.wantOrig || tt.cfg.Recorded != tt.wantRec {
				t.Fatalf("NormalizeOrigin() => origin=%q recorded=%v, want origin=%q recorded=%v", tt.cfg.Origin, tt.cfg.Recorded, tt.wantOrig, tt.wantRec)
			}
		})
	}
}

func TestResponseConfigEffectiveOrigin(t *testing.T) {
	if got := (*ResponseConfig)(nil).EffectiveOrigin(); got != ResponseOriginManual {
		t.Fatalf("nil EffectiveOrigin = %q, want %q", got, ResponseOriginManual)
	}

	cfg := &ResponseConfig{Origin: "", Recorded: true}
	if got := cfg.EffectiveOrigin(); got != ResponseOriginProxy {
		t.Fatalf("EffectiveOrigin() = %q, want %q", got, ResponseOriginProxy)
	}
}
