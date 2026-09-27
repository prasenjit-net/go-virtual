package collection

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/store"
)

func TestLegacyMappingDefaults(t *testing.T) {
	fallback := "fallback"
	sess := store.NewEphemeralSession(map[string]any{"empty": "", "null": nil, "false": false, "zero": 0})
	req := &RequestContext{PathParams: map[string]string{"empty": ""}, QueryParams: map[string][]string{"empty": {""}, "none": {}}, Headers: http.Header{"x-empty": {""}, "X-Zero": {}}, Body: `{"empty":"","null":null,"false":false,"zero":0}`, Session: sess}
	cases := []struct{ source, key, want string }{
		{"path", "missing", "fallback"}, {"path", "empty", ""},
		{"query", "missing", "fallback"}, {"query", "empty", ""}, {"query", "none", "fallback"},
		{"header", "missing", "fallback"}, {"header", "X-EMPTY", ""}, {"header", "X-Zero", "fallback"},
		{"body", "missing", "fallback"}, {"body", "empty", ""}, {"body", "null", ""}, {"body", "false", "false"}, {"body", "zero", "0"},
		{"session", "missing", "fallback"}, {"session", "empty", ""}, {"session", "null", ""}, {"session", "false", "false"}, {"session", "zero", "0"},
		{"store", "missing", "fallback"},
	}
	for _, tc := range cases {
		t.Run(tc.source+"/"+tc.key, func(t *testing.T) {
			rule := models.FieldMappingRule{SourceType: tc.source, SourceKey: tc.key, DefaultValue: &fallback}
			if got := Resolve(rule, req); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
	rule := models.FieldMappingRule{SourceType: "path", SourceKey: "id", DefaultValue: &fallback}
	if Resolve(rule, nil) != "fallback" {
		t.Fatal("unavailable source should use default")
	}
	rule.DefaultValue = nil
	if Resolve(rule, nil) != "id" {
		t.Fatal("legacy no-default nil request behavior changed")
	}
}

func TestTypedMappingDefaultsPresence(t *testing.T) {
	ctx := &BindingContext{Request: &TypedRequestContext{Headers: http.Header{"x-empty": {""}}, Body: `{"null":null,"false":false,"zero":0,"empty":"","object":{},"array":[],"nested":null}`}, Document: map[string]any{"nil": nil}, Mappers: map[string]any{"plan": nil, "plans": []any{}}}
	cases := []struct {
		source models.ValueSource
		key    string
		want   any
	}{
		{models.ValueSourceBody, "null", nil}, {models.ValueSourceBody, "false", false}, {models.ValueSourceBody, "zero", float64(0)},
		{models.ValueSourceBody, "empty", ""}, {models.ValueSourceBody, "object", map[string]any{}}, {models.ValueSourceBody, "array", []any{}},
		{models.ValueSourceBody, "missing", "fallback"}, {models.ValueSourceBody, "nested.city", "fallback"},
		{models.ValueSourceHeader, "X-EMPTY", ""}, {models.ValueSourceHeader, "X-Missing", "fallback"},
		{models.ValueSourceDocument, "nil", nil}, {models.ValueSourceDocument, "nil.child", "fallback"},
		{models.ValueSourcePrimary, "name", "fallback"}, {models.ValueSourceMapper, "plan.label", "fallback"}, {models.ValueSourceMapper, "plans.0.label", "fallback"},
	}
	for _, tc := range cases {
		t.Run(string(tc.source)+"/"+tc.key, func(t *testing.T) {
			v, found, err := ResolveValueBinding(models.ValueBinding{Source: tc.source, Key: tc.key, DefaultValue: json.RawMessage(`"fallback"`)}, ctx)
			if err != nil || !found || !reflect.DeepEqual(v, tc.want) {
				t.Fatalf("got %#v found=%v err=%v; want %#v", v, found, err, tc.want)
			}
		})
	}
	for _, raw := range []string{`null`, `false`, `0`, `""`, `{}`, `[]`} {
		b := models.ValueBinding{Source: models.ValueSourceBody, Key: "missing", DefaultValue: json.RawMessage(raw)}
		got, found, err := ResolveValueBinding(b, nil)
		var want any
		json.Unmarshal([]byte(raw), &want)
		if err != nil || !found || !reflect.DeepEqual(got, want) {
			t.Fatalf("default %s: %v %v %v", raw, got, found, err)
		}
	}
}

func TestRequiredDefaultsAndIsolation(t *testing.T) {
	b := models.ValueBinding{Source: models.ValueSourceBody, Key: "missing", DefaultValue: json.RawMessage(`{"nested":{"count":1}}`)}
	rules := []models.CollectionFilter{{TargetPath: "settings", Value: b}}
	first, err := ResolveRequiredMap(rules, nil)
	if err != nil {
		t.Fatal(err)
	}
	first["settings"].(map[string]any)["nested"].(map[string]any)["count"] = 99
	next, err := ResolveRequiredMap(rules, nil)
	if err != nil || next["settings"].(map[string]any)["nested"].(map[string]any)["count"] != float64(1) {
		t.Fatal("defaults share mutable data")
	}
	rules[0].Value.DefaultValue = nil
	if _, err := ResolveRequiredMap(rules, nil); err == nil {
		t.Fatal("missing required binding without default accepted")
	}
	for _, invalid := range []models.ValueBinding{
		{Source: "unknown", DefaultValue: json.RawMessage(`true`)},
		{Source: models.ValueSourceLiteral, Value: json.RawMessage(`invalid`), DefaultValue: json.RawMessage(`true`)},
		{Source: models.ValueSourceBody, Key: "missing", DefaultValue: json.RawMessage(`invalid`)},
	} {
		if _, _, err := ResolveValueBinding(invalid, nil); err == nil {
			t.Fatal("default hid a binding error")
		}
	}
}
