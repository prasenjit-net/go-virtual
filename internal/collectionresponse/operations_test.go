package collectionresponse

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/prasenjit/go-virtual/internal/collection"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/store"
)

func literalRule(field string, value any) models.CollectionFilter {
	raw, _ := json.Marshal(value)
	return models.CollectionFilter{TargetPath: field, Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: raw}}
}

func updateConfig() *models.ResponseConfig {
	return &models.ResponseConfig{StatusCode: 200, Kind: models.ResponseKindCollection, CollectionResponse: &models.CollectionResponseConfig{
		Primary: models.CollectionQuery{Mode: models.ColOpUpdate, CollectionName: "users", FilterRules: []models.CollectionFilter{literalRule("name", "Alice")}, DataRules: []models.CollectionFilter{literalRule("name", "Confirmed")}},
	}}
}

func TestMainUpdateSelectionExecutionAndPreview(t *testing.T) {
	svc, op, _, backend := setupService(t)
	backend.SeedInsert("users", map[string]any{"_id": "1", "id": "1", "name": "Alice"})
	backend.SeedInsert("users", map[string]any{"_id": "2", "id": "2", "name": "Alice"})
	sess := store.NewEphemeralSession(nil)
	cfg := updateConfig()
	cfg.CollectionResponse.AdditionalMappers = []models.NamedQuery{{OutputKey: "audit", Mode: models.ColOpInsert, CollectionQuery: models.CollectionQuery{CollectionName: "audit", DataRules: []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourcePrimary, Key: "name"}}}}}}
	match, err := svc.TryMatch(op, cfg, nil, sess)
	if err != nil || !match.Matched {
		t.Fatalf("match: %v %v", match, err)
	}
	preview, err := svc.Render(cfg, match, nil, sess)
	if err != nil || !strings.Contains(string(preview.Body), "Alice") || len(preview.Warnings) < 2 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if len(store.LoadEvents(sess, "users")) != 0 || len(store.LoadEvents(sess, "audit")) != 0 {
		t.Fatal("selection/preview wrote events")
	}
	prepared, err := svc.ExecuteSelected(cfg, match, nil, sess)
	if err != nil {
		t.Fatal(err)
	}
	if match.Doc["name"] != "Alice" || prepared.Doc["name"] != "Confirmed" {
		t.Fatal("incorrect original/updated snapshots")
	}
	if prepared.mapperOutputs["audit"].(map[string]any)["name"] != "Confirmed" {
		t.Fatal("additional mapper did not see updated primary")
	}
	for i := 0; i < 2; i++ {
		rendered, err := svc.Render(cfg, prepared, nil, sess)
		if err != nil || !strings.Contains(string(rendered.Body), "Confirmed") {
			t.Fatalf("render: %+v %v", rendered, err)
		}
	}
	if _, err := svc.ExecuteSelected(cfg, match, nil, sess); err == nil {
		t.Fatal("repeat execution allowed")
	}
	docs, _ := collection.NewOps("users", backend, sess).FindMany(nil)
	if docs[0]["name"] != "Confirmed" || docs[1]["name"] != "Alice" {
		t.Fatalf("wrong update target: %v", docs)
	}
	if len(store.LoadEvents(sess, "users")) != 1 || len(store.LoadEvents(sess, "audit")) != 1 {
		t.Fatal("operations repeated")
	}
	base, _ := backend.GetAll("users")
	if base[0]["name"] != "Alice" {
		t.Fatal("base mutated")
	}
	other, _ := collection.NewOps("users", backend, store.NewEphemeralSession(nil)).FindOne(nil)
	if other["name"] != "Alice" {
		t.Fatal("session isolation broken")
	}
}

func TestAdditionalOperationsOrderedResults(t *testing.T) {
	svc, op, _, backend := setupService(t)
	backend.SeedInsert("users", map[string]any{"_id": "1", "name": "Alice"})
	cfg := updateConfig()
	cfg.CollectionResponse.Primary.Mode = ""
	cfg.CollectionResponse.Primary.DataRules = nil
	mapper := func(key string, mode models.CollectionOpType, filters, data []models.CollectionFilter) models.NamedQuery {
		return models.NamedQuery{OutputKey: key, Mode: mode, CollectionQuery: models.CollectionQuery{CollectionName: "items", FilterRules: filters, DataRules: data}}
	}
	cfg.CollectionResponse.AdditionalMappers = []models.NamedQuery{
		mapper("inserted", models.ColOpInsert, nil, []models.CollectionFilter{literalRule("name", "old"), literalRule("enabled", false), literalRule("count", 0), literalRule("nullable", nil)}),
		mapper("found", models.ColOpFindOne, []models.CollectionFilter{literalRule("name", "old")}, nil),
		mapper("updated", models.ColOpUpdate, []models.CollectionFilter{literalRule("name", "old")}, []models.CollectionFilter{literalRule("name", "new")}),
		mapper("many", models.ColOpFindMany, []models.CollectionFilter{literalRule("name", "new")}, nil),
		mapper("upserted", models.ColOpUpsert, []models.CollectionFilter{literalRule("name", "new")}, []models.CollectionFilter{literalRule("name", "final")}),
		mapper("created", models.ColOpUpsert, []models.CollectionFilter{literalRule("name", "another")}, []models.CollectionFilter{literalRule("count", 2)}),
		mapper("deleted", models.ColOpDelete, []models.CollectionFilter{literalRule("name", "final")}, nil),
		mapper("missing", models.ColOpFindOne, []models.CollectionFilter{literalRule("name", "final")}, nil),
		mapper("noUpdate", models.ColOpUpdate, []models.CollectionFilter{literalRule("name", "absent")}, []models.CollectionFilter{literalRule("count", 1)}),
		mapper("noDelete", models.ColOpDelete, []models.CollectionFilter{literalRule("name", "absent")}, nil),
	}
	if errs := cfg.CollectionResponse.Validate(); len(errs) > 0 {
		t.Fatal(errs)
	}
	sess := store.NewEphemeralSession(nil)
	match, err := svc.TryMatch(op, cfg, nil, sess)
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.ExecuteSelected(cfg, match, nil, sess)
	if err != nil {
		t.Fatal(err)
	}
	for _, trace := range result.mapperTraces {
		if trace.Error == "" && trace.RecordCount > 0 && trace.Result == nil {
			t.Fatalf("mapper output missing from trace: %+v", trace)
		}
		if trace.Operation == models.ColOpInsert && len(trace.Data) == 0 {
			t.Fatalf("write data missing from trace: %+v", trace)
		}
	}
	outputs := result.mapperOutputs
	inserted := outputs["inserted"].(map[string]any)
	if inserted["_id"] == nil || inserted["enabled"] != false || inserted["count"] != float64(0) || inserted["nullable"] != nil {
		t.Fatalf("typed insert: %v", inserted)
	}
	for key, want := range map[string]string{"found": "old", "updated": "new", "upserted": "final", "created": "another", "deleted": "final"} {
		if outputs[key].(map[string]any)["name"] != want {
			t.Fatalf("%s: %v", key, outputs[key])
		}
	}
	if len(outputs["many"].([]any)) != 1 || outputs["missing"] != nil || outputs["noUpdate"] != nil || outputs["noDelete"] != nil {
		t.Fatalf("unexpected results: %v", outputs)
	}
	if result.Doc["name"] != "Alice" || len(result.mapperTraces) != 10 {
		t.Fatal("primary snapshot or traces incorrect")
	}
	// Generated identities survive event replay and do not change on subsequent reads.
	stored, _ := collection.NewOps("items", backend, sess).FindOne(nil)
	if stored["_id"] != outputs["created"].(map[string]any)["_id"] {
		t.Fatal("unstable inserted identity")
	}
}

func TestUpdateEmptyAndMissingBindings(t *testing.T) {
	svc, op, _, backend := setupService(t)
	cfg := updateConfig()
	sess := store.NewEphemeralSession(nil)
	match, err := svc.TryMatch(op, cfg, nil, sess)
	if err != nil || match.Matched {
		t.Fatalf("empty: %v %v", match, err)
	}
	if _, err := svc.ExecuteSelected(cfg, match, nil, sess); err == nil {
		t.Fatal("unselected response executed")
	}
	cfg.CollectionResponse.MatchOnEmpty = true
	match, err = svc.TryMatch(op, cfg, nil, sess)
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.ExecuteSelected(cfg, match, nil, sess)
	if err != nil || result.Doc != nil || len(store.LoadEvents(sess, "users")) != 0 {
		t.Fatalf("empty update: %v %v", result, err)
	}
	backend.SeedInsert("users", map[string]any{"_id": "1", "name": "Alice"})
	cfg.CollectionResponse.Primary.DataRules = []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceBody, Key: "name"}}}
	match, _ = svc.TryMatch(op, cfg, nil, sess)
	result, err = svc.ExecuteSelected(cfg, match, nil, sess)
	if err == nil || result.primaryTrace.Error == "" || len(store.LoadEvents(sess, "users")) != 0 {
		t.Fatal("missing write binding was not rejected before mutation")
	}
	cfg.CollectionResponse.Primary.FilterRules = []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourcePath, Key: "missing"}}}
	if _, err = svc.TryMatch(op, cfg, nil, sess); err == nil {
		t.Fatal("missing update filter allowed")
	}
}

func TestUpdatePartialFailureAndSnapshot(t *testing.T) {
	svc, op, _, backend := setupService(t)
	backend.SeedInsert("users", map[string]any{"_id": "1", "name": "Alice"})
	cfg := updateConfig()
	cfg.CollectionResponse.AdditionalMappers = []models.NamedQuery{
		{OutputKey: "deleted", Mode: models.ColOpDelete, CollectionQuery: models.CollectionQuery{CollectionName: "users", FilterRules: []models.CollectionFilter{literalRule("_id", "1")}}},
		{OutputKey: "created", Mode: models.ColOpInsert, CollectionQuery: models.CollectionQuery{CollectionName: "items", DataRules: []models.CollectionFilter{literalRule("_id", "same")}}},
		{OutputKey: "duplicate", Mode: models.ColOpInsert, CollectionQuery: models.CollectionQuery{CollectionName: "items", DataRules: []models.CollectionFilter{literalRule("_id", "same")}}},
	}
	sess := store.NewEphemeralSession(nil)
	match, _ := svc.TryMatch(op, cfg, nil, sess)
	result, err := svc.ExecuteSelected(cfg, match, nil, sess)
	if err == nil || result.Doc["name"] != "Confirmed" || len(result.mapperTraces) != 3 || result.mapperTraces[2].Error == "" {
		t.Fatalf("partial failure: %v %v", result, err)
	}
	if len(store.LoadEvents(sess, "users")) != 2 || len(store.LoadEvents(sess, "items")) != 1 {
		t.Fatal("partial writes not preserved")
	}
	if result.mapperOutputs["deleted"].(map[string]any)["name"] != "Confirmed" {
		t.Fatal("delete should return updated pre-removal document")
	}
}

func TestMainUpdateShapeValidation(t *testing.T) {
	svc, _, arrayOp, _ := setupService(t)
	errs, err := svc.ValidateAgainstOperation(arrayOp, updateConfig())
	if err != nil || len(errs) == 0 {
		t.Fatalf("array Update accepted: %v %v", errs, err)
	}
}

func TestMainInsertUpsertExecution(t *testing.T) {
	for _, mode := range []models.CollectionOpType{models.ColOpInsert, models.ColOpUpsert} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/existing=%v", mode, existing), func(t *testing.T) {
				svc, op, arrayOp, backend := setupService(t)
				if existing {
					backend.SeedInsert("users", map[string]any{"_id": "old", "name": "Alice"})
				}
				cfg := updateConfig()
				cfg.CollectionResponse.Primary.Mode = mode
				if mode == models.ColOpInsert {
					cfg.CollectionResponse.Primary.FilterRules = nil
				}
				cfg.CollectionResponse.Primary.DataRules = append(cfg.CollectionResponse.Primary.DataRules, literalRule("secret", "hidden"))
				cfg.CollectionResponse.AdditionalMappers = []models.NamedQuery{{OutputKey: "audit", Mode: models.ColOpInsert, CollectionQuery: models.CollectionQuery{CollectionName: "audit", DataRules: []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourcePrimary, Key: "name"}}}}}}
				cfg.CollectionResponse.Overrides = []models.FieldOverride{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceMapper, Key: "audit.name"}}}
				if errs := cfg.CollectionResponse.Validate(); len(errs) != 0 {
					t.Fatal(errs)
				}
				if errs, err := svc.ValidateAgainstOperation(arrayOp, cfg); err != nil || len(errs) == 0 {
					t.Fatalf("array accepted: %v %v", errs, err)
				}
				sess := store.NewEphemeralSession(nil)
				match, err := svc.TryMatch(op, cfg, nil, sess)
				if err != nil || !match.Matched || match.Doc != nil || match.Filter != nil {
					t.Fatalf("selection: %+v %v", match, err)
				}
				preview, err := svc.Render(cfg, match, nil, sess)
				if err != nil || len(preview.Warnings) < 2 || len(store.LoadEvents(sess, "users")) != 0 {
					t.Fatalf("preview: %+v %v", preview, err)
				}
				result, err := svc.ExecuteSelected(cfg, match, nil, sess)
				if err != nil {
					t.Fatal(err)
				}
				if result.primaryTrace.Operation != mode || result.primaryTrace.RecordCount != 1 {
					t.Fatalf("trace: %+v", result.primaryTrace)
				}
				if mode == models.ColOpUpsert && existing && result.Doc["_id"] != "old" {
					t.Fatal("upsert did not retain existing identity")
				}
				for i := 0; i < 2; i++ {
					rendered, err := svc.Render(cfg, result, nil, sess)
					if err != nil || !strings.Contains(string(rendered.Body), "Confirmed") || strings.Contains(string(rendered.Body), "secret") || strings.Contains(string(rendered.Body), "_id") {
						t.Fatalf("render: %+v %v", rendered, err)
					}
				}
				if _, err := svc.ExecuteSelected(cfg, match, nil, sess); err == nil {
					t.Fatal("repeat execution accepted")
				}
				docs, _ := collection.NewOps("users", backend, sess).FindMany(nil)
				want := 1
				if mode == models.ColOpInsert && existing {
					want = 2
				}
				if len(docs) != want || len(store.LoadEvents(sess, "users")) != 1 || len(store.LoadEvents(sess, "audit")) != 1 {
					t.Fatalf("unexpected writes: %v", docs)
				}
			})
		}
	}
}

func TestMainUpsertMissingFilterFailsOnlyAfterSelection(t *testing.T) {
	svc, op, _, _ := setupService(t)
	cfg := updateConfig()
	cfg.CollectionResponse.Primary.Mode = models.ColOpUpsert
	cfg.CollectionResponse.Primary.FilterRules = []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceBody, Key: "missing"}}}
	sess := store.NewEphemeralSession(nil)
	match, err := svc.TryMatch(op, cfg, nil, sess)
	if err != nil || !match.Matched {
		t.Fatalf("filter affected selection: %v %v", match, err)
	}
	result, err := svc.ExecuteSelected(cfg, match, nil, sess)
	if err == nil || result.primaryTrace.Error == "" || len(store.LoadEvents(sess, "users")) != 0 {
		t.Fatalf("missing filter wrote: %+v %v", result, err)
	}
}

func TestMainInsertUpsertValidation(t *testing.T) {
	for _, mode := range []models.CollectionOpType{models.ColOpInsert, models.ColOpUpsert} {
		cfg := updateConfig().CollectionResponse
		cfg.Primary.Mode = mode
		if mode == models.ColOpInsert {
			cfg.Primary.FilterRules = nil
		}
		cfg.Primary.DataRules = nil
		if len(cfg.Validate()) == 0 {
			t.Fatal("missing data accepted")
		}
		cfg.Primary.DataRules = []models.CollectionFilter{{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourcePrimary, Key: "name"}}}
		if len(cfg.Validate()) == 0 {
			t.Fatal("unavailable primary source accepted")
		}
		cfg.Primary.DataRules = []models.CollectionFilter{literalRule("name", "Alice")}
		if mode == models.ColOpInsert {
			cfg.Primary.FilterRules = []models.CollectionFilter{literalRule("name", "Alice")}
		} else {
			cfg.Primary.FilterRules = nil
		}
		if len(cfg.Validate()) == 0 {
			t.Fatal("invalid filters accepted")
		}
	}
}
