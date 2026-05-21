//go:build unit

package template

import (
	"strings"
	"testing"
)

func TestIterFuncs_Times(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{range $i, $_ := times 3}}{{$i}}{{end}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "012" {
		t.Errorf("expected 012, got %q", result)
	}
}

func TestIterFuncs_TimesZero(t *testing.T) {
	fm := buildIterFuncMap()
	timesFn := fm["times"].(func(int) []int)
	result := timesFn(0)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestIterFuncs_TimesNegative(t *testing.T) {
	fm := buildIterFuncMap()
	timesFn := fm["times"].(func(int) []int)
	result := timesFn(-3)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestIterFuncs_Seq(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{range seq 1 3}}{{.}} {{end}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result) != "1 2 3" {
		t.Errorf("expected '1 2 3', got %q", result)
	}
}

func TestIterFuncs_SeqReversed(t *testing.T) {
	fm := buildIterFuncMap()
	seqFn := fm["seq"].(func(int, int) []int)
	result := seqFn(5, 3)
	if len(result) != 0 {
		t.Errorf("expected empty slice for reversed seq, got %v", result)
	}
}

func TestIterFuncs_List(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{range list "a" "b" "c"}}{{.}}{{end}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "abc" {
		t.Errorf("expected abc, got %q", result)
	}
}

func TestIterFuncs_FirstLast(t *testing.T) {
	fm := buildIterFuncMap()
	listFn := fm["list"].(func(...interface{}) []interface{})
	firstFn := fm["first"].(func([]interface{}) interface{})
	lastFn := fm["last"].(func([]interface{}) interface{})

	items := listFn("x", "y", "z")
	if firstFn(items) != "x" {
		t.Errorf("expected x, got %v", firstFn(items))
	}
	if lastFn(items) != "z" {
		t.Errorf("expected z, got %v", lastFn(items))
	}
}

func TestIterFuncs_FirstLastEmpty(t *testing.T) {
	fm := buildIterFuncMap()
	firstFn := fm["first"].(func([]interface{}) interface{})
	lastFn := fm["last"].(func([]interface{}) interface{})

	empty := []interface{}{}
	if firstFn(empty) != "" {
		t.Errorf("expected empty, got %v", firstFn(empty))
	}
	if lastFn(empty) != "" {
		t.Errorf("expected empty, got %v", lastFn(empty))
	}
}

func TestIterFuncs_Len(t *testing.T) {
	fm := buildIterFuncMap()
	listFn := fm["list"].(func(...interface{}) []interface{})
	lenFn := fm["len"].(func([]interface{}) int)

	items := listFn("a", "b", "c", "d")
	if lenFn(items) != 4 {
		t.Errorf("expected 4, got %d", lenFn(items))
	}
}
