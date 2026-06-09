package collection

import (
	"net/http"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/store"
)

// ── Resolve ──────────────────────────────────────────────────────────────────

func TestResolve_NilReq(t *testing.T) {
	rule := models.FieldMappingRule{
		SourceType: "query",
		SourceKey:  "myKey",
		TargetField: "target",
	}
	got := Resolve(rule, nil)
	if got != "myKey" {
		t.Errorf("nil req: expected SourceKey %q, got %q", "myKey", got)
	}
}

func TestResolve_Literal(t *testing.T) {
	rule := models.FieldMappingRule{
		SourceType:  "literal",
		SourceKey:   "hello-world",
		TargetField: "greeting",
	}
	got := Resolve(rule, &RequestContext{})
	if got != "hello-world" {
		t.Errorf("literal: expected %q, got %q", "hello-world", got)
	}
}

func TestResolve_Path(t *testing.T) {
	rule := models.FieldMappingRule{SourceType: "path", SourceKey: "id"}
	req := &RequestContext{PathParams: map[string]string{"id": "42"}}
	got := Resolve(rule, req)
	if got != "42" {
		t.Errorf("path: expected %q, got %q", "42", got)
	}
}

func TestResolve_PathMissing(t *testing.T) {
	rule := models.FieldMappingRule{SourceType: "path", SourceKey: "missing"}
	req := &RequestContext{PathParams: map[string]string{"id": "42"}}
	got := Resolve(rule, req)
	if got != "" {
		t.Errorf("path missing key: expected empty, got %q", got)
	}
}

func TestResolve_PathNilParams(t *testing.T) {
	rule := models.FieldMappingRule{SourceType: "path", SourceKey: "id"}
	req := &RequestContext{PathParams: nil}
	got := Resolve(rule, req)
	if got != "" {
		t.Errorf("nil PathParams: expected empty, got %q", got)
	}
}

func TestResolve_Query(t *testing.T) {
	rule := models.FieldMappingRule{SourceType: "query", SourceKey: "page"}
	req := &RequestContext{QueryParams: map[string][]string{"page": {"3"}}}
	got := Resolve(rule, req)
	if got != "3" {
		t.Errorf("query: expected %q, got %q", "3", got)
	}
}

func TestResolve_QueryMultipleValues(t *testing.T) {
	rule := models.FieldMappingRule{SourceType: "query", SourceKey: "tag"}
	req := &RequestContext{QueryParams: map[string][]string{"tag": {"first", "second"}}}
	got := Resolve(rule, req)
	if got != "first" {
		t.Errorf("query multiple values: expected first value, got %q", got)
	}
}

func TestResolve_QueryMissing(t *testing.T) {
	rule := models.FieldMappingRule{SourceType: "query", SourceKey: "missing"}
	req := &RequestContext{QueryParams: map[string][]string{}}
	got := Resolve(rule, req)
	if got != "" {
		t.Errorf("query missing key: expected empty, got %q", got)
	}
}

func TestResolve_Header(t *testing.T) {
	rule := models.FieldMappingRule{SourceType: "header", SourceKey: "X-Tenant"}
	req := &RequestContext{Headers: http.Header{"X-Tenant": {"acme"}}}
	got := Resolve(rule, req)
	if got != "acme" {
		t.Errorf("header: expected %q, got %q", "acme", got)
	}
}

func TestResolve_HeaderMissing(t *testing.T) {
	rule := models.FieldMappingRule{SourceType: "header", SourceKey: "X-Missing"}
	req := &RequestContext{Headers: http.Header{}}
	got := Resolve(rule, req)
	if got != "" {
		t.Errorf("header missing key: expected empty, got %q", got)
	}
}

func TestResolve_BodyField(t *testing.T) {
	rule := models.FieldMappingRule{SourceType: "body", SourceKey: "user.name"}
	req := &RequestContext{Body: `{"user":{"name":"Alice"}}`}
	got := Resolve(rule, req)
	if got != "Alice" {
		t.Errorf("body field: expected %q, got %q", "Alice", got)
	}
}

func TestResolve_BodyEmptyKey(t *testing.T) {
	body := `{"x":1}`
	rule := models.FieldMappingRule{SourceType: "body", SourceKey: ""}
	req := &RequestContext{Body: body}
	got := Resolve(rule, req)
	if got != body {
		t.Errorf("body empty key: expected full body, got %q", got)
	}
}

func TestResolve_BodyMissingField(t *testing.T) {
	rule := models.FieldMappingRule{SourceType: "body", SourceKey: "nonexistent"}
	req := &RequestContext{Body: `{"x":1}`}
	got := Resolve(rule, req)
	if got != "" {
		t.Errorf("body missing field: expected empty, got %q", got)
	}
}

func TestResolve_BodyEmptyBody(t *testing.T) {
	rule := models.FieldMappingRule{SourceType: "body", SourceKey: "x"}
	req := &RequestContext{Body: ""}
	got := Resolve(rule, req)
	if got != "" {
		t.Errorf("empty body: expected empty, got %q", got)
	}
}

func TestResolve_Session(t *testing.T) {
	sess := store.NewEphemeralSession(map[string]any{"userID": "u42"})
	rule := models.FieldMappingRule{SourceType: "session", SourceKey: "userID"}
	req := &RequestContext{Session: sess}
	got := Resolve(rule, req)
	if got != "u42" {
		t.Errorf("session: expected %q, got %q", "u42", got)
	}
}

func TestResolve_SessionMissing(t *testing.T) {
	sess := store.NewEphemeralSession(nil)
	rule := models.FieldMappingRule{SourceType: "session", SourceKey: "missing"}
	req := &RequestContext{Session: sess}
	got := Resolve(rule, req)
	if got != "" {
		t.Errorf("session missing key: expected empty, got %q", got)
	}
}

func TestResolve_Stringify(t *testing.T) {
	// Store an integer value; stringify should convert it to its string representation.
	sess := store.NewEphemeralSession(map[string]any{"count": 7})
	rule := models.FieldMappingRule{SourceType: "session", SourceKey: "count"}
	req := &RequestContext{Session: sess}
	got := Resolve(rule, req)
	if got != "7" {
		t.Errorf("stringify: expected %q, got %q", "7", got)
	}
}

func TestResolve_UnknownSourceType(t *testing.T) {
	rule := models.FieldMappingRule{SourceType: "unknown", SourceKey: "x"}
	req := &RequestContext{}
	got := Resolve(rule, req)
	if got != "" {
		t.Errorf("unknown source type: expected empty, got %q", got)
	}
}

// ── ResolveMap ───────────────────────────────────────────────────────────────

func TestResolveMap(t *testing.T) {
	req := &RequestContext{
		PathParams:  map[string]string{"id": "10"},
		QueryParams: map[string][]string{"page": {"2"}},
	}
	rules := []models.FieldMappingRule{
		{SourceType: "path", SourceKey: "id", TargetField: "docID"},
		{SourceType: "query", SourceKey: "page", TargetField: "pageNum"},
		{SourceType: "literal", SourceKey: "fixed", TargetField: "const"},
	}

	m := ResolveMap(rules, req)
	if len(m) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(m))
	}
	if m["docID"] != "10" {
		t.Errorf("docID: expected %q, got %v", "10", m["docID"])
	}
	if m["pageNum"] != "2" {
		t.Errorf("pageNum: expected %q, got %v", "2", m["pageNum"])
	}
	if m["const"] != "fixed" {
		t.Errorf("const: expected %q, got %v", "fixed", m["const"])
	}
}

func TestResolveMap_Empty(t *testing.T) {
	m := ResolveMap(nil, &RequestContext{})
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}
