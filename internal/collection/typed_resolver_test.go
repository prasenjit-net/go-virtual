package collection

import (
	"net/http"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
)

func TestGetPath(t *testing.T) {
	doc := map[string]any{
		"id": "u1",
		"profile": map[string]any{
			"name": "Alice",
			"tags": []any{"a", "b"},
		},
		"deleted": nil,
		"orders": []any{
			map[string]any{"id": "o1"},
			map[string]any{"id": "o2"},
		},
	}

	tests := []struct {
		name      string
		path      string
		wantFound bool
		checkVal  bool
		want      any
	}{
		{"root", "", true, false, nil},
		{"top-level", "id", true, true, "u1"},
		{"nested", "profile.name", true, true, "Alice"},
		{"array index", "profile.tags.1", true, true, "b"},
		{"array of objects", "orders.0.id", true, true, "o1"},
		{"missing key", "profile.age", false, false, nil},
		{"missing nested parent", "missing.deep", false, false, nil},
		{"explicit null value found", "deleted", true, true, nil},
		{"index out of range", "profile.tags.5", false, false, nil},
		{"non-numeric index", "profile.tags.x", false, false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := GetPath(doc, tt.path)
			if found != tt.wantFound {
				t.Fatalf("GetPath(%q) found = %v, want %v", tt.path, found, tt.wantFound)
			}
			if tt.checkVal && got != tt.want {
				t.Fatalf("GetPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestResolveValueBinding_Literal(t *testing.T) {
	v, found, err := ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceLiteral, Value: []byte("42")}, nil)
	if err != nil || !found {
		t.Fatalf("unexpected err=%v found=%v", err, found)
	}
	if f, ok := v.(float64); !ok || f != 42 {
		t.Fatalf("got %v (%T), want 42", v, v)
	}

	_, found, err = ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceLiteral}, nil)
	if err != nil || found {
		t.Fatalf("empty literal should be not-found, got found=%v err=%v", found, err)
	}

	_, _, err = ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceLiteral, Value: []byte("not-json")}, nil)
	if err == nil {
		t.Fatal("expected an error for invalid literal JSON")
	}
}

func TestResolveValueBinding_RequestSources(t *testing.T) {
	req := &TypedRequestContext{
		PathParams:  map[string]string{"id": "123"},
		QueryParams: map[string][]string{"status": {"active", "ignored"}},
		Headers:     http.Header{"X-Tenant-Id": []string{"t1"}},
		Body:        `{"customer":{"email":"a@example.com"},"active":true,"count":3}`,
	}
	ctx := &BindingContext{Request: req}

	v, found, _ := ResolveValueBinding(models.ValueBinding{Source: models.ValueSourcePath, Key: "id"}, ctx)
	if !found || v != "123" {
		t.Fatalf("path: got %v found=%v", v, found)
	}

	v, found, _ = ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceQuery, Key: "status"}, ctx)
	if !found || v != "active" {
		t.Fatalf("query: got %v found=%v", v, found)
	}

	v, found, _ = ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceHeader, Key: "x-tenant-id"}, ctx)
	if !found || v != "t1" {
		t.Fatalf("header (case-insensitive): got %v found=%v", v, found)
	}

	v, found, _ = ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceBody, Key: "customer.email"}, ctx)
	if !found || v != "a@example.com" {
		t.Fatalf("body string: got %v found=%v", v, found)
	}

	v, found, _ = ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceBody, Key: "active"}, ctx)
	if !found {
		t.Fatal("body bool: not found")
	}
	if b, ok := v.(bool); !ok || !b {
		t.Fatalf("body bool: got %v (%T), want true", v, v)
	}

	v, found, _ = ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceBody, Key: "count"}, ctx)
	if !found {
		t.Fatal("body number: not found")
	}
	if f, ok := v.(float64); !ok || f != 3 {
		t.Fatalf("body number: got %v (%T), want 3", v, v)
	}

	_, found, _ = ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceQuery, Key: "missing"}, ctx)
	if found {
		t.Fatal("missing query param should not be found")
	}
}

func TestResolveValueBinding_PrimaryDocumentMapper(t *testing.T) {
	primary := map[string]any{"planId": "p1", "profile": map[string]any{"name": "Alice"}}
	document := map[string]any{"customer": map[string]any{"email": "b@example.com"}}
	mappers := map[string]any{
		"plan": map[string]any{"label": "Gold"},
	}
	ctx := &BindingContext{Primary: primary, Document: document, Mappers: mappers}

	v, found, _ := ResolveValueBinding(models.ValueBinding{Source: models.ValueSourcePrimary, Key: "planId"}, ctx)
	if !found || v != "p1" {
		t.Fatalf("primary: got %v found=%v", v, found)
	}

	v, found, _ = ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceDocument, Key: "customer.email"}, ctx)
	if !found || v != "b@example.com" {
		t.Fatalf("document: got %v found=%v", v, found)
	}

	v, found, _ = ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceMapper, Key: "plan.label"}, ctx)
	if !found || v != "Gold" {
		t.Fatalf("mapper: got %v found=%v", v, found)
	}

	_, found, _ = ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceMapper, Key: "missing.label"}, ctx)
	if found {
		t.Fatal("unknown mapper output key should not be found")
	}

	// A mapper key with no "." addresses the whole mapper result.
	v, found, _ = ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceMapper, Key: "plan"}, ctx)
	if !found {
		t.Fatal("mapper key without a path should resolve the whole result")
	}
	if m, ok := v.(map[string]any); !ok || m["label"] != "Gold" {
		t.Fatalf("mapper whole-result: got %v", v)
	}
}

func TestResolveValueBinding_NilContext(t *testing.T) {
	sources := []models.ValueSource{
		models.ValueSourcePath, models.ValueSourceQuery, models.ValueSourceHeader,
		models.ValueSourceBody, models.ValueSourcePrimary, models.ValueSourceDocument, models.ValueSourceMapper,
	}
	for _, src := range sources {
		v, found, err := ResolveValueBinding(models.ValueBinding{Source: src, Key: "x"}, nil)
		if err != nil || found || v != nil {
			t.Fatalf("source %q with nil ctx: got v=%v found=%v err=%v", src, v, found, err)
		}
	}
}

func TestResolveValueBinding_UnknownSource(t *testing.T) {
	v, found, err := ResolveValueBinding(models.ValueBinding{Source: "bogus"}, nil)
	if err != nil || found || v != nil {
		t.Fatalf("got v=%v found=%v err=%v", v, found, err)
	}
}

func TestResolveValueBinding_HeaderNotFound(t *testing.T) {
	ctx := &BindingContext{Request: &TypedRequestContext{Headers: http.Header{}}}
	_, found, _ := ResolveValueBinding(models.ValueBinding{Source: models.ValueSourceHeader, Key: "X-Missing"}, ctx)
	if found {
		t.Fatal("missing header should not be found")
	}
}

func TestResolveFilterMap(t *testing.T) {
	filters := []models.CollectionFilter{
		{TargetPath: "tenantId", Value: models.ValueBinding{Source: models.ValueSourceHeader, Key: "X-Tenant-Id"}},
		{TargetPath: "active", Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: []byte("true")}},
	}
	ctx := &BindingContext{Request: &TypedRequestContext{Headers: http.Header{"X-Tenant-Id": []string{"t1"}}}}

	out, err := ResolveFilterMap(filters, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["tenantId"] != "t1" {
		t.Fatalf("tenantId = %v, want t1", out["tenantId"])
	}
	if b, ok := out["active"].(bool); !ok || !b {
		t.Fatalf("active = %v, want true", out["active"])
	}
}

func TestResolveFilterMap_Error(t *testing.T) {
	filters := []models.CollectionFilter{
		{TargetPath: "x", Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: []byte("not-json")}},
	}
	_, err := ResolveFilterMap(filters, &BindingContext{})
	if err == nil {
		t.Fatal("expected an error for an invalid literal filter value")
	}
}
