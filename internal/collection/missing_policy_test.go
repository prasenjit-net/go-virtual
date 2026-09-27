package collection

import (
	"github.com/prasenjit/go-virtual/internal/models"
	"testing"
)

func TestSkipMissingLegacyMapping(t *testing.T) {
	for _, source := range []string{"path", "query", "header", "body", "session", "store"} {
		rule := models.FieldMappingRule{TargetField: "target", SourceType: source, SourceKey: "missing", SkipWhenMissing: true}
		for _, req := range []*RequestContext{nil, {}} {
			if got := ResolveMap([]models.FieldMappingRule{rule}, req); len(got) != 0 {
				t.Fatalf("%s: %v", source, got)
			}
		}
	}
	for _, key := range []string{"empty", "null", "zero", "false"} {
		rule := models.FieldMappingRule{TargetField: "target", SourceType: "body", SourceKey: key, SkipWhenMissing: true}
		got := ResolveMap([]models.FieldMappingRule{rule}, &RequestContext{Body: `{"empty":"","null":null,"zero":0,"false":false}`})
		if _, exists := got["target"]; !exists {
			t.Fatalf("present %s skipped", key)
		}
	}
}

func TestSkipMissingTypedMaps(t *testing.T) {
	ctx := &BindingContext{Request: &TypedRequestContext{Body: `{"empty":"","null":null,"zero":0,"false":false}`}}
	for _, resolve := range []func([]models.CollectionFilter, *BindingContext) (map[string]any, error){ResolveFilterMap, ResolveRequiredMap} {
		for _, source := range []models.ValueSource{models.ValueSourcePath, models.ValueSourceQuery, models.ValueSourceHeader, models.ValueSourceBody, models.ValueSourcePrimary, models.ValueSourceDocument, models.ValueSourceMapper} {
			rules := []models.CollectionFilter{{TargetPath: "target", Value: models.ValueBinding{Source: source, Key: "missing", SkipWhenMissing: true}}}
			got, err := resolve(rules, ctx)
			if err != nil || len(got) != 0 {
				t.Fatalf("%s: %v %v", source, got, err)
			}
		}
		for _, key := range []string{"empty", "null", "zero", "false"} {
			rules := []models.CollectionFilter{{TargetPath: "target", Value: models.ValueBinding{Source: models.ValueSourceBody, Key: key, SkipWhenMissing: true}}}
			got, err := resolve(rules, ctx)
			if _, exists := got["target"]; err != nil || !exists {
				t.Fatalf("present %s skipped: %v", key, err)
			}
		}
	}
	rules := []models.CollectionFilter{{TargetPath: "target", Value: models.ValueBinding{Source: models.ValueSourceBody, Key: "missing"}}}
	if _, err := ResolveRequiredMap(rules, ctx); err == nil {
		t.Fatal("legacy missing write behavior changed")
	}
	got, err := ResolveFilterMap(rules, ctx)
	if value, exists := got["target"]; err != nil || !exists || value != nil {
		t.Fatal("legacy missing filter behavior changed")
	}
}
