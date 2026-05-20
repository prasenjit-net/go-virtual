//go:build unit

package template

import (
	"strings"
	"testing"
)

func TestFormatFuncs_NumberFormat(t *testing.T) {
	fm := buildFormatFuncMap()
	nf := fm["numberFormat"].(func(interface{}) string)

	tests := []struct {
		v    interface{}
		want string
	}{
		{1234567.89, "1,234,567.89"},
		{1000.0, "1,000.00"},
		{999.0, "999.00"},
		{-1234.5, "-1,234.50"},
	}

	for _, tt := range tests {
		got := nf(tt.v)
		if got != tt.want {
			t.Errorf("numberFormat(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestFormatFuncs_Currency(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{currency 19.99 "$"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "$19.99" {
		t.Errorf("expected $19.99, got %q", result)
	}
}

func TestFormatFuncs_DateFmt(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{now | dateFmt "2006"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 4 {
		t.Errorf("expected 4-char year, got %q", result)
	}
}

func TestFormatFuncs_NumberFormatTemplate(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{numberFormat 1234}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "1,234") {
		t.Errorf("expected formatted number, got %q", result)
	}
}

func TestFormatFuncs_DurationHuman(t *testing.T) {
	fm := buildFormatFuncMap()
	dh := fm["durationHuman"].(func(interface{}) string)

	// Under 1 minute
	r1 := dh(float64(30 * 1e9))
	if !strings.HasSuffix(r1, "s") {
		t.Errorf("expected seconds, got %q", r1)
	}

	// Under 1 hour
	r2 := dh(float64(90 * 1e9))
	if !strings.Contains(r2, "m") {
		t.Errorf("expected minutes, got %q", r2)
	}

	// Over 1 hour
	r3 := dh(float64(3700 * 1e9))
	if !strings.HasPrefix(r3, "1h") {
		t.Errorf("expected hours, got %q", r3)
	}
}
