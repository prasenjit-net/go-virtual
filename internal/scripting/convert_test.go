package scripting

import (
	"math"
	"reflect"
	"testing"

	"go.starlark.net/starlark"
)

func TestStarToGo_Primitives(t *testing.T) {
	tests := []struct {
		name     string
		input    starlark.Value
		expected any
	}{
		{"nil", nil, nil},
		{"None", starlark.None, nil},
		{"true", starlark.Bool(true), true},
		{"false", starlark.Bool(false), false},
		{"int", starlark.MakeInt(42), int64(42)},
		{"int zero", starlark.MakeInt(0), int64(0)},
		{"negative int", starlark.MakeInt(-7), int64(-7)},
		{"float", starlark.Float(3.14), float64(3.14)},
		{"string", starlark.String("hello"), "hello"},
		{"empty string", starlark.String(""), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StarToGo(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("StarToGo(%v) = %v (%T), want %v (%T)", tt.input, got, got, tt.expected, tt.expected)
			}
		})
	}
}

func TestStarToGo_List(t *testing.T) {
	list := starlark.NewList([]starlark.Value{
		starlark.String("a"),
		starlark.MakeInt(1),
		starlark.Bool(true),
	})
	got := StarToGo(list)
	want := []any{"a", int64(1), true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StarToGo(list) = %v, want %v", got, want)
	}
}

func TestStarToGo_Tuple(t *testing.T) {
	tup := starlark.Tuple{starlark.String("x"), starlark.MakeInt(99)}
	got := StarToGo(tup)
	want := []any{"x", int64(99)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StarToGo(tuple) = %v, want %v", got, want)
	}
}

func TestStarToGo_Dict(t *testing.T) {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("name"), starlark.String("Alice"))
	_ = d.SetKey(starlark.String("age"), starlark.MakeInt(30))
	got := StarToGo(d)
	want := map[string]any{"name": "Alice", "age": int64(30)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StarToGo(dict) = %v, want %v", got, want)
	}
}

func TestStarToGo_NestedDict(t *testing.T) {
	inner := new(starlark.Dict)
	_ = inner.SetKey(starlark.String("x"), starlark.Float(1.5))

	outer := new(starlark.Dict)
	_ = outer.SetKey(starlark.String("nested"), inner)

	got := StarToGo(outer)
	want := map[string]any{"nested": map[string]any{"x": float64(1.5)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StarToGo(nested dict) = %v, want %v", got, want)
	}
}

func TestGoToStar_Primitives(t *testing.T) {
	tests := []struct {
		name  string
		input any
		check func(starlark.Value) bool
	}{
		{"nil", nil, func(v starlark.Value) bool { return v == starlark.None }},
		{"bool true", true, func(v starlark.Value) bool { b, ok := v.(starlark.Bool); return ok && bool(b) }},
		{"bool false", false, func(v starlark.Value) bool { b, ok := v.(starlark.Bool); return ok && !bool(b) }},
		{"int", int(5), func(v starlark.Value) bool { i, ok := v.(starlark.Int); if !ok { return false }; n, _ := i.Int64(); return n == 5 }},
		{"int64", int64(100), func(v starlark.Value) bool { i, ok := v.(starlark.Int); if !ok { return false }; n, _ := i.Int64(); return n == 100 }},
		{"float64", float64(2.5), func(v starlark.Value) bool { f, ok := v.(starlark.Float); return ok && math.Abs(float64(f)-2.5) < 1e-9 }},
		{"string", "hello", func(v starlark.Value) bool { s, ok := v.(starlark.String); return ok && string(s) == "hello" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GoToStar(tt.input)
			if !tt.check(got) {
				t.Errorf("GoToStar(%v) = %v (%T), unexpected value", tt.input, got, got)
			}
		})
	}
}

func TestGoToStar_SliceAndMap(t *testing.T) {
	slice := []any{"a", int64(1)}
	sv := GoToStar(slice)
	list, ok := sv.(*starlark.List)
	if !ok || list.Len() != 2 {
		t.Fatalf("GoToStar(slice) expected *starlark.List len=2, got %T", sv)
	}

	m := map[string]any{"key": "val"}
	dv := GoToStar(m)
	d, ok := dv.(*starlark.Dict)
	if !ok || d.Len() != 1 {
		t.Fatalf("GoToStar(map) expected *starlark.Dict len=1, got %T", dv)
	}
}

func TestRoundTrip_DictViaGoToStar(t *testing.T) {
	original := map[string]any{
		"total":     float64(37.5),
		"freeShip":  true,
		"name":      "Widget",
		"itemCount": int64(3),
	}
	star := GoToStar(original)
	back := StarToGo(star)
	if !reflect.DeepEqual(back, original) {
		t.Errorf("round-trip mismatch: got %v, want %v", back, original)
	}
}
