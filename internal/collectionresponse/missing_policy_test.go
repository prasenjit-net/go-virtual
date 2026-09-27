package collectionresponse

import (
	"github.com/prasenjit/go-virtual/internal/collection"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/store"
	"testing"
)

func TestSkippedOverrideUsesNormalFilling(t *testing.T) {
	template := map[string]any{"name": "Example", "nested": map[string]any{"value": "fallback"}}
	doc := map[string]any{"name": "Original", "nested": map[string]any{"value": "Original nested"}}
	overrides := map[string]models.FieldOverride{
		"name":   {Value: models.ValueBinding{Source: models.ValueSourceBody, Key: "missing", SkipWhenMissing: true}},
		"nested": {Value: models.ValueBinding{Source: models.ValueSourceMapper, Key: "absent.value", SkipWhenMissing: true}},
	}
	value, warnings := FillDocument(template, doc, overrides, nil, nil, true)
	if len(warnings) != 0 || value.(map[string]any)["name"] != "Original" || value.(map[string]any)["nested"].(map[string]any)["value"] != "Original nested" {
		t.Fatalf("%v %v", value, warnings)
	}
	value, warnings = fillIdentity(doc, overrides, nil, nil)
	if len(warnings) != 0 || value.(map[string]any)["name"] != "Original" {
		t.Fatalf("identity: %v %v", value, warnings)
	}
	value, _ = FillDocument(template, map[string]any{}, overrides, nil, nil, true)
	if value.(map[string]any)["name"] != "Example" {
		t.Fatalf("fallback: %v", value)
	}
	value, _ = FillDocument(template, doc, overrides, &collection.TypedRequestContext{Body: `{"missing":null}`}, nil, true)
	if value.(map[string]any)["name"] != nil {
		t.Fatal("explicit null was skipped")
	}
}

func TestSkippedUpdateFieldPreservesStoredValue(t *testing.T) {
	svc, op, _, backend := setupService(t)
	backend.SeedInsert("users", map[string]any{"_id": "1", "name": "Alice", "email": "kept@example.com"})
	cfg := updateConfig()
	cfg.CollectionResponse.Primary.DataRules = append(cfg.CollectionResponse.Primary.DataRules,
		models.CollectionFilter{TargetPath: "email", Value: models.ValueBinding{Source: models.ValueSourceBody, Key: "email", SkipWhenMissing: true}})
	sess := store.NewEphemeralSession(nil)
	match, err := svc.TryMatch(op, cfg, nil, sess)
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.ExecuteSelected(cfg, match, nil, sess)
	if err != nil || result.Doc["email"] != "kept@example.com" || result.Doc["name"] != "Confirmed" {
		t.Fatalf("skipped field changed: %v %v", result, err)
	}
}
