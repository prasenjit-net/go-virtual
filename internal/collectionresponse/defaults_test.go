package collectionresponse

import (
	"encoding/json"
	"testing"

	"github.com/prasenjit/go-virtual/internal/collection"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/store"
)

func TestDefaultsSelectUpdateAndRender(t *testing.T) {
	svc, op, _, backend := setupService(t)
	if _, err := backend.SeedInsert("users", map[string]any{"_id": "1", "name": "Alice"}); err != nil {
		t.Fatal(err)
	}
	cfg := updateConfig()
	cfg.CollectionResponse.Primary.FilterRules = []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceQuery, Key: "name", DefaultValue: json.RawMessage(`"Alice"`)}}}
	cfg.CollectionResponse.Primary.DataRules = []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceBody, Key: "name", DefaultValue: json.RawMessage(`"Updated"`)}}}
	cfg.CollectionResponse.AdditionalMappers = []models.NamedQuery{{OutputKey: "plan", Mode: models.ColOpFindOne, CollectionQuery: models.CollectionQuery{CollectionName: "plans"}}}
	cfg.CollectionResponse.Overrides = []models.FieldOverride{
		{TargetPath: "profile.nickname", Value: models.ValueBinding{Source: models.ValueSourceMapper, Key: "plan.label", DefaultValue: json.RawMessage(`"Free"`)}},
		{TargetPath: "id", Value: models.ValueBinding{Source: models.ValueSourceDocument, Key: "absent", DefaultValue: json.RawMessage(`null`)}},
	}
	sess := store.NewEphemeralSession(nil)
	match, err := svc.TryMatch(op, cfg, nil, sess)
	if err != nil || !match.Matched {
		t.Fatalf("default filter failed: %v", err)
	}
	preview, err := svc.Render(cfg, match, nil, sess)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.LoadEvents(sess, "users")) != 0 {
		t.Fatal("preview wrote data")
	}
	var body map[string]any
	if err := json.Unmarshal(preview.Body, &body); err != nil {
		t.Fatal(err)
	}
	if body["name"] != "Alice" || body["profile"].(map[string]any)["nickname"] != "Free" || body["id"] != nil {
		t.Fatalf("preview: %s", preview.Body)
	}
	result, err := svc.ExecuteSelected(cfg, match, nil, sess)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := svc.Render(cfg, result, nil, sess)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rendered.Body, &body); err != nil {
		t.Fatal(err)
	}
	if body["name"] != "Updated" || body["profile"].(map[string]any)["nickname"] != "Free" || body["id"] != nil || len(rendered.Warnings) != 0 {
		t.Fatalf("render: %s %v", rendered.Body, rendered.Warnings)
	}
	// Defaults select real documents; they cannot create a match when the request supplies another value.
	noMatch, err := svc.TryMatch(op, cfg, &collection.TypedRequestContext{QueryParams: map[string][]string{"name": {"Nobody"}}}, sess)
	if err != nil || noMatch.Matched {
		t.Fatal("supplied query value did not override default")
	}
}

func TestOverrideDefaultsInBothRenderModes(t *testing.T) {
	req := &collection.TypedRequestContext{Body: `{"empty":"","nil":null}`}
	overrides := map[string]models.FieldOverride{
		"label": {TargetPath: "label", Value: models.ValueBinding{Source: models.ValueSourceMapper, Key: "plan.label", DefaultValue: json.RawMessage(`"Free"`)}},
		"empty": {TargetPath: "empty", Value: models.ValueBinding{Source: models.ValueSourceBody, Key: "empty", DefaultValue: json.RawMessage(`"wrong"`)}},
		"nil":   {TargetPath: "nil", Value: models.ValueBinding{Source: models.ValueSourceBody, Key: "nil", DefaultValue: json.RawMessage(`"wrong"`)}},
	}
	doc := map[string]any{"label": "document", "empty": "document", "nil": "document"}
	for _, fallback := range []bool{true, false} {
		value, warnings := FillDocument(map[string]any{"label": "example", "empty": "example", "nil": "example"}, doc, overrides, req, nil, fallback)
		got := value.(map[string]any)
		if got["label"] != "Free" || got["empty"] != "" || got["nil"] != nil || len(warnings) != 0 {
			t.Fatalf("template: %v %v", value, warnings)
		}
	}
	value, warnings := fillIdentity(doc, overrides, req, nil)
	if value.(map[string]any)["label"] != "Free" || len(warnings) != 0 || doc["label"] != "document" {
		t.Fatalf("identity: %v %v", value, warnings)
	}
}
