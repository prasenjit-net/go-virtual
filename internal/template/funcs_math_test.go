//go:build unit

package template

import (
	"testing"
)

func TestMathFuncs_Add(t *testing.T) {
	e := NewEngine()
	ctx := &Context{QueryParams: map[string][]string{"page": {"3"}}}
	result, err := e.RenderBodyTemplate(`{{add 1 (toInt (query "page"))}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "4" {
		t.Errorf("expected 4, got %q", result)
	}
}

func TestMathFuncs_Sub(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{sub 10 3}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "7" {
		t.Errorf("expected 7, got %q", result)
	}
}

func TestMathFuncs_Mul(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{mul 4 5}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "20" {
		t.Errorf("expected 20, got %q", result)
	}
}

func TestMathFuncs_Div(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{div 10 4}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "2.5" {
		t.Errorf("expected 2.5, got %q", result)
	}
}

func TestMathFuncs_DivByZero(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{div 10 0}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "0" {
		t.Errorf("expected 0, got %q", result)
	}
}

func TestMathFuncs_Mod(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{mod 7 3}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "1" {
		t.Errorf("expected 1, got %q", result)
	}
}

func TestMathFuncs_ModByZero(t *testing.T) {
	fm := buildMathFuncMap()
	modFn := fm["mod"].(func(interface{}, interface{}) int)
	result := modFn(5, 0)
	if result != 0 {
		t.Errorf("expected 0, got %d", result)
	}
}

func TestMathFuncs_Abs(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{abs -5}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "5" {
		t.Errorf("expected 5, got %q", result)
	}
}

func TestMathFuncs_MaxMin(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	r1, _ := e.RenderBodyTemplate(`{{max 3 7}}`, ctx)
	if r1 != "7" {
		t.Errorf("expected 7, got %q", r1)
	}
	r2, _ := e.RenderBodyTemplate(`{{min 3 7}}`, ctx)
	if r2 != "3" {
		t.Errorf("expected 3, got %q", r2)
	}
}

func TestMathFuncs_CeilFloorRound(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	r1, _ := e.RenderBodyTemplate(`{{ceil 3.2}}`, ctx)
	if r1 != "4" {
		t.Errorf("expected 4, got %q", r1)
	}
	r2, _ := e.RenderBodyTemplate(`{{floor 3.9}}`, ctx)
	if r2 != "3" {
		t.Errorf("expected 3, got %q", r2)
	}
	r3, _ := e.RenderBodyTemplate(`{{round 3.5}}`, ctx)
	if r3 != "4" {
		t.Errorf("expected 4, got %q", r3)
	}
}

func TestMathFuncs_ToIntToFloatToString(t *testing.T) {
	e := NewEngine()
	ctx := &Context{QueryParams: map[string][]string{"n": {"42"}}}
	r1, _ := e.RenderBodyTemplate(`{{toInt (query "n")}}`, ctx)
	if r1 != "42" {
		t.Errorf("expected 42, got %q", r1)
	}
	r2, _ := e.RenderBodyTemplate(`{{toFloat (query "n")}}`, ctx)
	if r2 != "42" {
		t.Errorf("expected 42, got %q", r2)
	}
	r3, _ := e.RenderBodyTemplate(`{{toString 42}}`, ctx)
	if r3 != "42" {
		t.Errorf("expected 42, got %q", r3)
	}
}

func TestMathFuncs_ToNumberTypes(t *testing.T) {
	tests := []struct {
		v    interface{}
		want float64
	}{
		{int32(5), 5},
		{float32(3.14), float64(float32(3.14))},
		{int64(100), 100},
		{"99.9", 99.9},
		{"invalid", 0},
	}
	for _, tt := range tests {
		got := toNumber(tt.v)
		if tt.v == "99.9" {
			if got < 99.89 || got > 99.91 {
				t.Errorf("toNumber(%v) = %v, want ~%v", tt.v, got, tt.want)
			}
		} else if got != tt.want {
			t.Errorf("toNumber(%v) = %v, want %v", tt.v, got, tt.want)
		}
	}
}
