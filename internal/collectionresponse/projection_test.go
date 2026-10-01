package collectionresponse

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/parser"
)

func mapperOverride(key string) models.FieldOverride {
	return models.FieldOverride{Value: models.ValueBinding{Source: models.ValueSourceMapper, Key: key}}
}

func TestMapperObjectArrayProjectionAndOverrides(t *testing.T) {
	shape := map[string]any{"city": "Example", "postalCode": "00000", "details": map[string]any{"label": "Example"}}
	template := map[string]any{"addresses": []any{shape}, "addrs": []any{shape}, "city": "Example", "unrelated": "Example"}
	addresses := []any{map[string]any{"city": "Boston", "postalCode": "02108", "_id": "secret", "employeeId": "123", "details": map[string]any{"label": "Home", "hidden": true}}}
	mappers := map[string]any{"addresses": addresses, "unused": "must not appear"}
	doc := map[string]any{"addresses": []any{map[string]any{"city": "Primary"}}, "unrelated": "Primary"}
	overrides := map[string]models.FieldOverride{
		"addrs":                 mapperOverride("addresses"),
		"city":                  mapperOverride("addresses.0.city"),
		"addresses.city":        {Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: json.RawMessage(`"Override"`)}},
		"addrs.0.details.label": {Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: json.RawMessage(`"Indexed"`)}},
	}
	result, warnings, mappings := fillDocumentWithTrace(template, doc, overrides, nil, mappers, true)
	raw, _ := json.Marshal(result)
	if len(warnings) != 0 || strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "employeeId") || strings.Contains(string(raw), "hidden") || strings.Contains(string(raw), "unused") {
		t.Fatalf("projection: %s %v", raw, warnings)
	}
	out := result.(map[string]any)
	if out["addresses"].([]any)[0].(map[string]any)["city"] != "Override" || out["city"] != "Boston" || out["unrelated"] != "Primary" {
		t.Fatalf("precedence: %v", out)
	}
	if out["addrs"].([]any)[0].(map[string]any)["details"].(map[string]any)["label"] != "Indexed" {
		t.Fatal("nested override not applied")
	}
	if addresses[0].(map[string]any)["city"] != "Boston" {
		t.Fatal("mapper output mutated")
	}
	if len(mappings) == 0 {
		t.Fatal("field source trace missing")
	}
}

func TestMapperPrecedenceMissingNullAndEmpty(t *testing.T) {
	template := map[string]any{"addresses": []any{map[string]any{"city": "Example"}}}
	doc := map[string]any{"addresses": []any{map[string]any{"city": "Primary"}}}
	for _, value := range []any{nil, []any{}} {
		out, _ := FillDocument(template, doc, nil, nil, map[string]any{"addresses": value}, true)
		if !reflect.DeepEqual(out.(map[string]any)["addresses"], value) {
			t.Fatalf("present mapper output ignored: %v", out)
		}
	}
	mapper := map[string]any{"addresses": []any{map[string]any{"city": "Mapper"}}}
	override := mapperOverride("missing")
	override.Value.SkipWhenMissing = true
	out, _ := FillDocument(template, doc, map[string]models.FieldOverride{"addresses": override}, nil, mapper, true)
	if out.(map[string]any)["addresses"].([]any)[0].(map[string]any)["city"] != "Mapper" {
		t.Fatal("skip did not fall through to mapper")
	}
	override.Value.SkipWhenMissing = false
	override.Value.DefaultValue = json.RawMessage(`[{"city":"Default","secret":true}]`)
	out, _ = FillDocument(template, doc, map[string]models.FieldOverride{"addresses": override}, nil, mapper, true)
	raw, _ := json.Marshal(out)
	if string(raw) != `{"addresses":[{"city":"Default"}]}` {
		t.Fatalf("default not projected: %s", raw)
	}
	override = mapperOverride("missing")
	out, _ = FillDocument(template, doc, map[string]models.FieldOverride{"addresses": override}, nil, mapper, true)
	if out.(map[string]any)["addresses"] != nil {
		t.Fatal("missing explicit override fell through")
	}
}

func TestMissingContainerShapeDoesNotExposeFields(t *testing.T) {
	for _, shape := range []any{nil, map[string]any{}, []any{}} {
		for _, value := range []any{map[string]any{"secret": true}, []any{map[string]any{"secret": true}}} {
			out, warnings := FillDocument(map[string]any{"data": shape}, map[string]any{}, nil, nil, map[string]any{"data": value}, true)
			raw, _ := json.Marshal(out)
			if strings.Contains(string(raw), "secret") || len(warnings) == 0 {
				t.Fatalf("unknown shape: %s %v", raw, warnings)
			}
		}
	}
	out, warnings := fillIdentity(map[string]any{"name": "Employee"}, map[string]models.FieldOverride{"data": mapperOverride("data")}, nil, map[string]any{"data": map[string]any{"secret": true}})
	if len(warnings) == 0 || out.(map[string]any)["data"] != nil {
		t.Fatal("identity copied unshaped mapper output")
	}
}

func TestSchemaSuppliesMissingExampleShape(t *testing.T) {
	for _, example := range []string{`,"example":{"addresses":[]}`, `,"example":{}`, ""} {
		spec := `{"openapi":"3.0.3","info":{"title":"test","version":"1"},"paths":{"/employees":{"get":{"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","properties":{"name":{"type":"string"},"addresses":{"type":"array","items":{"$ref":"#/components/schemas/Address"}}}}` + example + `}}}}}}},"components":{"schemas":{"Address":{"type":"object","properties":{"city":{"type":"string"},"postalCode":{"type":"string"}}}}}}`
		tmpl, err := ResolveTemplate(parser.NewParser(), spec, &models.Operation{Method: "GET", Path: "/employees"}, 200, "")
		if err != nil {
			t.Fatal(err)
		}
		out, _ := FillDocument(tmpl.Value, map[string]any{"name": "Employee"}, nil, nil, map[string]any{"addresses": []any{map[string]any{"city": "Boston", "postalCode": "02108", "secret": true}}}, true)
		raw, _ := json.Marshal(out)
		if string(raw) != `{"addresses":[{"city":"Boston","postalCode":"02108"}],"name":"Employee"}` {
			t.Fatalf("schema projection: %s", raw)
		}
	}
}

func TestMapperRenamedObjectAndArrayRoot(t *testing.T) {
	svc, _, _, _ := setupService(t)
	shape := map[string]any{"id": "example", "addrs": map[string]any{"city": "Example"}, "addresses": []any{map[string]any{"city": "Example"}}}
	cfg := &models.ResponseConfig{CollectionResponse: &models.CollectionResponseConfig{Overrides: []models.FieldOverride{{TargetPath: "addrs", Value: models.ValueBinding{Source: models.ValueSourceMapper, Key: "address"}}}}}
	match := &MatchResult{RootKind: models.RootKindArray, Template: &ResolvedTemplate{Source: TemplateSourceExample, Value: []any{shape}}, Docs: []map[string]any{{"id": "1"}, {"id": "2"}}, prepared: true, mapperOutputs: map[string]any{"address": map[string]any{"city": "Boston", "secret": true}, "addresses": []any{map[string]any{"city": "Seattle", "secret": true}}}}
	rendered, err := svc.Render(cfg, match, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"addresses":[{"city":"Seattle"}],"addrs":{"city":"Boston"},"id":"1"},{"addresses":[{"city":"Seattle"}],"addrs":{"city":"Boston"},"id":"2"}]`
	if string(rendered.Body) != want {
		t.Fatalf("array root projection: %s", rendered.Body)
	}
}

func TestRecursiveSchemaAndAllOfProjection(t *testing.T) {
	spec := `{"openapi":"3.0.3","info":{"title":"test","version":"1"},"paths":{"/employees":{"get":{"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"allOf":[{"type":"object","properties":{"name":{"type":"string"}}},{"type":"object","properties":{"address":{"$ref":"#/components/schemas/Address"}}}]}}}}}}}},"components":{"schemas":{"Address":{"type":"object","properties":{"city":{"type":"string"},"parent":{"$ref":"#/components/schemas/Address"}}}}}}`
	tmpl, err := ResolveTemplate(parser.NewParser(), spec, &models.Operation{Method: "GET", Path: "/employees"}, 200, "")
	if err != nil {
		t.Fatal(err)
	}
	out, warnings := FillDocument(tmpl.Value, map[string]any{"name": "Employee"}, nil, nil, map[string]any{"address": map[string]any{"city": "Boston", "secret": true, "parent": map[string]any{"secret": true}}}, true)
	raw, _ := json.Marshal(out)
	if strings.Contains(string(raw), "secret") || !strings.Contains(string(raw), "Boston") || len(warnings) == 0 {
		t.Fatalf("recursive projection: %s %v", raw, warnings)
	}
}
