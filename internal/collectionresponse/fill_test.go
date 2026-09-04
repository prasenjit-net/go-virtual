package collectionresponse

import (
	"encoding/json"
	"testing"

	"github.com/prasenjit/go-virtual/internal/collection"
	"github.com/prasenjit/go-virtual/internal/models"
)

func TestFillDocument_ConventionFill(t *testing.T) {
	template := map[string]any{
		"id":   "placeholder",
		"name": "placeholder",
	}
	doc := map[string]any{"id": "u1", "name": "Alice"}

	v, warnings := FillDocument(template, doc, nil, nil, nil, true)
	m := v.(map[string]any)
	if m["id"] != "u1" || m["name"] != "Alice" {
		t.Fatalf("got %#v", m)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestFillDocument_NestedObjectAndArray(t *testing.T) {
	template := map[string]any{
		"id": "x",
		"customer": map[string]any{
			"name": "x",
		},
		"orders": []any{
			map[string]any{"id": "x", "amount": 0.0},
		},
	}
	doc := map[string]any{
		"id":       "u1",
		"customer": map[string]any{"name": "Alice"},
		"orders": []any{
			map[string]any{"id": "o1", "amount": 42.5},
			map[string]any{"id": "o2", "amount": 10.0},
		},
	}

	v, warnings := FillDocument(template, doc, nil, nil, nil, true)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	m := v.(map[string]any)
	cust := m["customer"].(map[string]any)
	if cust["name"] != "Alice" {
		t.Fatalf("customer.name = %v", cust["name"])
	}
	orders := m["orders"].([]any)
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}
	o0 := orders[0].(map[string]any)
	if o0["id"] != "o1" || o0["amount"] != 42.5 {
		t.Fatalf("orders[0] = %#v", o0)
	}
}

func TestFillDocument_MissingPathFallsBackToExample(t *testing.T) {
	template := map[string]any{"id": "x", "nickname": "anonymous"}
	doc := map[string]any{"id": "u1"}

	v, warnings := FillDocument(template, doc, nil, nil, nil, true)
	m := v.(map[string]any)
	if m["nickname"] != "anonymous" {
		t.Fatalf("expected fallback to example, got %v", m["nickname"])
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", warnings)
	}
}

func TestFillDocument_MissingPathRendersNullWhenFallbackDisabled(t *testing.T) {
	template := map[string]any{"id": "x", "nickname": "anonymous"}
	doc := map[string]any{"id": "u1"}

	v, warnings := FillDocument(template, doc, nil, nil, nil, false)
	m := v.(map[string]any)
	if m["nickname"] != nil {
		t.Fatalf("expected null, got %v", m["nickname"])
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", warnings)
	}
}

func TestFillDocument_ExplicitNullIsNotAWarning(t *testing.T) {
	template := map[string]any{"deletedAt": "x"}
	doc := map[string]any{"deletedAt": nil}

	v, warnings := FillDocument(template, doc, nil, nil, nil, true)
	m := v.(map[string]any)
	if _, ok := m["deletedAt"]; !ok {
		t.Fatal("expected deletedAt key to be present")
	}
	if m["deletedAt"] != nil {
		t.Fatalf("expected nil, got %v", m["deletedAt"])
	}
	if len(warnings) != 0 {
		t.Fatalf("explicit null should not warn, got %v", warnings)
	}
}

func TestFillDocument_Overrides(t *testing.T) {
	template := map[string]any{
		"id":          "x",
		"displayName": "x",
		"planLabel":   "x",
		"requested":   "x",
		"active":      false,
	}
	doc := map[string]any{
		"id":      "u1",
		"profile": map[string]any{"name": "Alice"},
	}
	overrides := map[string]models.FieldOverride{
		"displayName": {TargetPath: "displayName", Value: models.ValueBinding{Source: models.ValueSourceDocument, Key: "profile.name"}},
		"planLabel":   {TargetPath: "planLabel", Value: models.ValueBinding{Source: models.ValueSourceMapper, Key: "plan.label"}},
		"requested":   {TargetPath: "requested", Value: models.ValueBinding{Source: models.ValueSourceQuery, Key: "include"}},
		"active":      {TargetPath: "active", Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: json.RawMessage("true")}},
	}
	req := &collection.TypedRequestContext{QueryParams: map[string][]string{"include": {"orders"}}}
	mappers := map[string]any{"plan": map[string]any{"label": "Gold"}}

	v, warnings := FillDocument(template, doc, overrides, req, mappers, true)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	m := v.(map[string]any)
	if m["displayName"] != "Alice" {
		t.Fatalf("displayName = %v", m["displayName"])
	}
	if m["planLabel"] != "Gold" {
		t.Fatalf("planLabel = %v", m["planLabel"])
	}
	if m["requested"] != "orders" {
		t.Fatalf("requested = %v", m["requested"])
	}
	if m["active"] != true {
		t.Fatalf("active = %v", m["active"])
	}
	// id fills by convention, untouched by overrides
	if m["id"] != "u1" {
		t.Fatalf("id = %v", m["id"])
	}
}

func TestFillDocument_ShapeMismatchWarns(t *testing.T) {
	template := map[string]any{"customer": map[string]any{"name": "x"}}
	doc := map[string]any{"customer": "not-an-object"}

	v, warnings := FillDocument(template, doc, nil, nil, nil, true)
	if len(warnings) == 0 {
		t.Fatal("expected a shape-mismatch warning")
	}
	m := v.(map[string]any)
	cust := m["customer"].(map[string]any)
	if cust["name"] != "x" {
		t.Fatalf("expected fallback to template example inside mismatched object, got %#v", cust)
	}
}

func TestFillDocument_DoesNotMutateSourceDocument(t *testing.T) {
	template := map[string]any{"nested": map[string]any{"a": "x"}}
	doc := map[string]any{"nested": map[string]any{"a": "orig"}}

	v, _ := FillDocument(template, doc, nil, nil, nil, true)
	m := v.(map[string]any)
	nested := m["nested"].(map[string]any)
	nested["a"] = "mutated"

	origNested := doc["nested"].(map[string]any)
	if origNested["a"] != "orig" {
		t.Fatalf("source document was mutated: %v", origNested["a"])
	}
}
