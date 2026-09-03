package collectionresponse

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/prasenjit/go-virtual/internal/collection"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
)

func setupService(t *testing.T) (*Service, *models.Operation, *models.Operation, *store.MemoryCollectionBackend) {
	t.Helper()
	s := storage.NewMemoryStorage()
	spec := &models.Spec{ID: "spec1", Name: "t", Content: testSpecContent}
	if err := s.CreateSpec(spec); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	getUser := &models.Operation{ID: "op-get-user", SpecID: spec.ID, Method: "GET", Path: "/users/{id}"}
	listUsers := &models.Operation{ID: "op-list-users", SpecID: spec.ID, Method: "GET", Path: "/users"}
	if err := s.CreateOperation(getUser); err != nil {
		t.Fatalf("CreateOperation: %v", err)
	}
	if err := s.CreateOperation(listUsers); err != nil {
		t.Fatalf("CreateOperation: %v", err)
	}

	backend := store.NewMemoryCollectionBackend()
	svc := NewService(s, backend)
	return svc, getUser, listUsers, backend
}

func TestTryMatch_ObjectRoot_FindOneMatches(t *testing.T) {
	svc, getUser, _, backend := setupService(t)
	if _, err := backend.SeedInsert("users", map[string]any{"_id": "u1", "id": "u1", "name": "Alice"}); err != nil {
		t.Fatal(err)
	}

	cfg := &models.ResponseConfig{
		ID:         "r1",
		StatusCode: 200,
		Kind:       models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{
				CollectionName: "users",
				FilterRules: []models.CollectionFilter{
					{TargetPath: "_id", Value: models.ValueBinding{Source: models.ValueSourcePath, Key: "id"}},
				},
			},
		},
	}
	req := &collection.TypedRequestContext{PathParams: map[string]string{"id": "u1"}}
	sess := store.NewEphemeralSession(nil)

	match, err := svc.TryMatch(getUser, cfg, req, sess)
	if err != nil {
		t.Fatalf("TryMatch error: %v", err)
	}
	if !match.Matched {
		t.Fatal("expected a match")
	}
	if match.RootKind != models.RootKindObject {
		t.Fatalf("rootKind = %v, want object", match.RootKind)
	}
	if match.Doc["name"] != "Alice" {
		t.Fatalf("matched doc = %#v", match.Doc)
	}

	render, err := svc.Render(cfg, match, req, sess)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(render.Body, &body); err != nil {
		t.Fatalf("invalid JSON body: %s", render.Body)
	}
	if body["id"] != "u1" || body["name"] != "Alice" {
		t.Fatalf("rendered body = %#v", body)
	}
	// profile.nickname is missing from the document; template example is "anon".
	profile := body["profile"].(map[string]any)
	if profile["nickname"] != "anon" {
		t.Fatalf("expected fallback to template example, got %#v", profile)
	}
}

func TestTryMatch_ObjectRoot_NoMatchFallsThrough(t *testing.T) {
	svc, getUser, _, _ := setupService(t)
	cfg := &models.ResponseConfig{
		ID:         "r1",
		StatusCode: 200,
		Kind:       models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{
				CollectionName: "users",
				FilterRules: []models.CollectionFilter{
					{TargetPath: "_id", Value: models.ValueBinding{Source: models.ValueSourcePath, Key: "id"}},
				},
			},
		},
	}
	req := &collection.TypedRequestContext{PathParams: map[string]string{"id": "missing"}}
	sess := store.NewEphemeralSession(nil)

	match, err := svc.TryMatch(getUser, cfg, req, sess)
	if err != nil {
		t.Fatalf("TryMatch error: %v", err)
	}
	if match.Matched {
		t.Fatal("expected no match for an empty find-one result")
	}
	if match.RecordCount != 0 {
		t.Fatalf("recordCount = %d, want 0", match.RecordCount)
	}
}

func TestTryMatch_ObjectRoot_MatchOnEmptyRendersNull(t *testing.T) {
	svc, getUser, _, _ := setupService(t)
	cfg := &models.ResponseConfig{
		ID:         "r1",
		StatusCode: 200,
		Kind:       models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary:      models.CollectionQuery{CollectionName: "users"},
			MatchOnEmpty: true,
		},
	}
	req := &collection.TypedRequestContext{}
	sess := store.NewEphemeralSession(nil)

	match, err := svc.TryMatch(getUser, cfg, req, sess)
	if err != nil {
		t.Fatalf("TryMatch error: %v", err)
	}
	if !match.Matched {
		t.Fatal("matchOnEmpty should match even with no documents")
	}
	render, err := svc.Render(cfg, match, req, sess)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if string(render.Body) != "null" {
		t.Fatalf("body = %s, want null", render.Body)
	}
}

func TestTryMatch_ArrayRoot_FindManyAndRenderEachItem(t *testing.T) {
	svc, _, listUsers, backend := setupService(t)
	if _, err := backend.SeedInsert("users", map[string]any{"id": "u1", "name": "Alice"}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.SeedInsert("users", map[string]any{"id": "u2", "name": "Bob"}); err != nil {
		t.Fatal(err)
	}

	cfg := &models.ResponseConfig{
		ID:         "r2",
		StatusCode: 200,
		Kind:       models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{CollectionName: "users"},
		},
	}
	req := &collection.TypedRequestContext{}
	sess := store.NewEphemeralSession(nil)

	match, err := svc.TryMatch(listUsers, cfg, req, sess)
	if err != nil {
		t.Fatalf("TryMatch error: %v", err)
	}
	if !match.Matched || match.RecordCount != 2 {
		t.Fatalf("match = %#v", match)
	}

	render, err := svc.Render(cfg, match, req, sess)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	var body []map[string]any
	if err := json.Unmarshal(render.Body, &body); err != nil {
		t.Fatalf("invalid JSON array body: %s", render.Body)
	}
	if len(body) != 2 || body[0]["name"] != "Alice" || body[1]["name"] != "Bob" {
		t.Fatalf("rendered body = %#v", body)
	}
}

func TestTryMatch_ArrayRoot_EmptyFallsThroughByDefault(t *testing.T) {
	svc, _, listUsers, _ := setupService(t)
	cfg := &models.ResponseConfig{
		ID:         "r2",
		StatusCode: 200,
		Kind:       models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{CollectionName: "users"},
		},
	}
	req := &collection.TypedRequestContext{}
	sess := store.NewEphemeralSession(nil)

	match, err := svc.TryMatch(listUsers, cfg, req, sess)
	if err != nil {
		t.Fatalf("TryMatch error: %v", err)
	}
	if match.Matched {
		t.Fatal("expected no match for an empty find-many result")
	}
}

func TestTryMatch_ArrayRoot_MatchOnEmptyRendersEmptyArray(t *testing.T) {
	svc, _, listUsers, _ := setupService(t)
	cfg := &models.ResponseConfig{
		ID:         "r2",
		StatusCode: 200,
		Kind:       models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary:      models.CollectionQuery{CollectionName: "users"},
			MatchOnEmpty: true,
		},
	}
	req := &collection.TypedRequestContext{}
	sess := store.NewEphemeralSession(nil)

	match, err := svc.TryMatch(listUsers, cfg, req, sess)
	if err != nil {
		t.Fatalf("TryMatch error: %v", err)
	}
	if !match.Matched {
		t.Fatal("matchOnEmpty should match even with no documents")
	}
	render, err := svc.Render(cfg, match, req, sess)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if string(render.Body) != "[]" {
		t.Fatalf("body = %s, want []", render.Body)
	}
}

func TestTryMatch_NoSessionNeverMatches(t *testing.T) {
	svc, getUser, _, backend := setupService(t)
	if _, err := backend.SeedInsert("users", map[string]any{"_id": "u1", "id": "u1", "name": "Alice"}); err != nil {
		t.Fatal(err)
	}
	cfg := &models.ResponseConfig{
		ID:         "r1",
		StatusCode: 200,
		Kind:       models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{CollectionName: "users"},
		},
	}
	match, err := svc.TryMatch(getUser, cfg, &collection.TypedRequestContext{}, nil)
	if err != nil {
		t.Fatalf("TryMatch error: %v", err)
	}
	if match.Matched {
		t.Fatal("expected no match without a session")
	}
}

func TestRender_AdditionalMapperWithPrimarySourceAndOverride(t *testing.T) {
	svc, getUser, _, backend := setupService(t)
	if _, err := backend.SeedInsert("users", map[string]any{"_id": "u1", "id": "u1", "name": "Alice", "planId": "p1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.SeedInsert("plans", map[string]any{"_id": "p1", "label": "Gold"}); err != nil {
		t.Fatal(err)
	}

	cfg := &models.ResponseConfig{
		ID:         "r1",
		StatusCode: 200,
		Kind:       models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{
				CollectionName: "users",
				FilterRules: []models.CollectionFilter{
					{TargetPath: "_id", Value: models.ValueBinding{Source: models.ValueSourcePath, Key: "id"}},
				},
			},
			AdditionalMappers: []models.NamedQuery{
				{
					OutputKey: "plan",
					Mode:      models.QueryModeFindOne,
					CollectionQuery: models.CollectionQuery{
						CollectionName: "plans",
						FilterRules: []models.CollectionFilter{
							{TargetPath: "_id", Value: models.ValueBinding{Source: models.ValueSourcePrimary, Key: "planId"}},
						},
					},
				},
			},
			Overrides: []models.FieldOverride{
				{TargetPath: "name", Value: models.ValueBinding{Source: models.ValueSourceMapper, Key: "plan.label"}},
			},
		},
	}
	req := &collection.TypedRequestContext{PathParams: map[string]string{"id": "u1"}}
	sess := store.NewEphemeralSession(nil)

	match, err := svc.TryMatch(getUser, cfg, req, sess)
	if err != nil || !match.Matched {
		t.Fatalf("TryMatch: match=%v err=%v", match, err)
	}
	render, err := svc.Render(cfg, match, req, sess)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if len(render.AdditionalMapperTraces) != 1 || render.AdditionalMapperTraces[0].RecordCount != 1 {
		t.Fatalf("additional mapper trace = %#v", render.AdditionalMapperTraces)
	}
	var body map[string]any
	if err := json.Unmarshal(render.Body, &body); err != nil {
		t.Fatalf("invalid JSON: %s", render.Body)
	}
	if body["name"] != "Gold" {
		t.Fatalf("expected name overridden from mapper output, got %#v", body)
	}
	if body["id"] != "u1" {
		t.Fatalf("expected id to still fill by convention, got %#v", body)
	}
}

func TestValidateAgainstOperation_PrimarySourceRejectedForArrayRoot(t *testing.T) {
	svc, _, listUsers, _ := setupService(t)
	cfg := &models.ResponseConfig{
		StatusCode: 200,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{CollectionName: "users"},
			AdditionalMappers: []models.NamedQuery{
				{
					OutputKey: "plan",
					Mode:      models.QueryModeFindOne,
					CollectionQuery: models.CollectionQuery{
						CollectionName: "plans",
						FilterRules: []models.CollectionFilter{
							{TargetPath: "_id", Value: models.ValueBinding{Source: models.ValueSourcePrimary, Key: "planId"}},
						},
					},
				},
			},
		},
	}
	errs, err := svc.ValidateAgainstOperation(listUsers, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) == 0 {
		t.Fatal("expected an error: primary source requires an object-rooted (find-one) primary query")
	}
}

func TestValidateAgainstOperation_IdentityModeRequiresRootKind(t *testing.T) {
	svc, _, _, _ := setupService(t)
	pingOp := &models.Operation{ID: "op-ping", SpecID: "spec1", Method: "GET", Path: "/ping"}
	cfg := &models.ResponseConfig{
		StatusCode: 204,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{CollectionName: "events"},
		},
	}
	errs, err := svc.ValidateAgainstOperation(pingOp, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) == 0 {
		t.Fatal("expected an error: rootKind is required in identity mode")
	}
}

func TestRender_IdentityMode_OverridesRenameAndFallThrough(t *testing.T) {
	svc, _, _, backend := setupService(t)
	pingOp := &models.Operation{ID: "op-ping", SpecID: "spec1", Method: "GET", Path: "/ping"}
	if _, err := backend.SeedInsert("events", map[string]any{"_id": "e1", "type": "click", "meta": map[string]any{"x": 1}}); err != nil {
		t.Fatal(err)
	}

	cfg := &models.ResponseConfig{
		ID:         "r-ping",
		StatusCode: 204,
		Kind:       models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary:  models.CollectionQuery{CollectionName: "events"},
			RootKind: models.RootKindObject,
			Overrides: []models.FieldOverride{
				// Renames an existing top-level field.
				{TargetPath: "type", Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: json.RawMessage(`"renamed"`)}},
				// Creates a new nested path that doesn't exist in the document.
				{TargetPath: "extra.info", Value: models.ValueBinding{Source: models.ValueSourceLiteral, Value: json.RawMessage(`"x"`)}},
				// References a request value that isn't present — should warn, not fail.
				{TargetPath: "missing", Value: models.ValueBinding{Source: models.ValueSourceQuery, Key: "absent"}},
			},
		},
	}
	req := &collection.TypedRequestContext{}
	sess := store.NewEphemeralSession(nil)

	match, err := svc.TryMatch(pingOp, cfg, req, sess)
	if err != nil || !match.Matched {
		t.Fatalf("TryMatch: match=%v err=%v", match, err)
	}
	if match.Template.Source != TemplateSourceIdentity {
		t.Fatalf("expected identity mode, got %v", match.Template.Source)
	}

	render, err := svc.Render(cfg, match, req, sess)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(render.Body, &body); err != nil {
		t.Fatalf("invalid JSON: %s", render.Body)
	}
	if body["type"] != "renamed" {
		t.Fatalf("expected override applied, got %#v", body)
	}
	if body["_id"] != "e1" {
		t.Fatalf("expected passthrough field, got %#v", body)
	}
	extra, ok := body["extra"].(map[string]any)
	if !ok || extra["info"] != "x" {
		t.Fatalf("expected a newly created nested path, got %#v", body["extra"])
	}
	if _, ok := body["missing"]; ok {
		t.Fatalf("unresolved override should not set the field, got %#v", body["missing"])
	}
	if len(render.Warnings) == 0 {
		t.Fatal("expected a warning for the unresolved override")
	}
}

func TestRender_AdditionalMapperFindMany(t *testing.T) {
	svc, getUser, _, backend := setupService(t)
	if _, err := backend.SeedInsert("users", map[string]any{"_id": "u1", "id": "u1", "name": "Alice"}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.SeedInsert("orders", map[string]any{"_id": "o1", "userId": "u1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.SeedInsert("orders", map[string]any{"_id": "o2", "userId": "u1"}); err != nil {
		t.Fatal(err)
	}

	cfg := &models.ResponseConfig{
		ID:         "r-fm",
		StatusCode: 200,
		Kind:       models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{
				CollectionName: "users",
				FilterRules: []models.CollectionFilter{
					{TargetPath: "_id", Value: models.ValueBinding{Source: models.ValueSourcePath, Key: "id"}},
				},
			},
			AdditionalMappers: []models.NamedQuery{
				{OutputKey: "orders", Mode: models.QueryModeFindMany, CollectionQuery: models.CollectionQuery{CollectionName: "orders"}},
			},
		},
	}
	req := &collection.TypedRequestContext{PathParams: map[string]string{"id": "u1"}}
	sess := store.NewEphemeralSession(nil)

	match, err := svc.TryMatch(getUser, cfg, req, sess)
	if err != nil || !match.Matched {
		t.Fatalf("TryMatch: match=%v err=%v", match, err)
	}
	render, err := svc.Render(cfg, match, req, sess)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if len(render.AdditionalMapperTraces) != 1 || render.AdditionalMapperTraces[0].RecordCount != 2 {
		t.Fatalf("mapper trace = %#v", render.AdditionalMapperTraces)
	}
}

// partialErrorBackend fails GetAll for one named collection and delegates to
// a real in-memory backend for every other one — used to exercise the
// additional-mapper error path without the primary query also failing.
type partialErrorBackend struct {
	*store.MemoryCollectionBackend
	failCollection string
}

func (b *partialErrorBackend) GetAll(name string) ([]map[string]any, error) {
	if name == b.failCollection {
		return nil, fmt.Errorf("simulated backend outage")
	}
	return b.MemoryCollectionBackend.GetAll(name)
}

func TestRender_AdditionalMapperError(t *testing.T) {
	s := storage.NewMemoryStorage()
	if err := s.CreateSpec(&models.Spec{ID: "spec-err", Content: testSpecContent}); err != nil {
		t.Fatal(err)
	}
	op := &models.Operation{ID: "op-err", SpecID: "spec-err", Method: "GET", Path: "/users/{id}"}

	backend := &partialErrorBackend{MemoryCollectionBackend: store.NewMemoryCollectionBackend(), failCollection: "orders"}
	if _, err := backend.SeedInsert("users", map[string]any{"_id": "u1", "id": "u1", "name": "Alice"}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(s, backend)
	cfg := &models.ResponseConfig{
		ID:         "r-err",
		StatusCode: 200,
		Kind:       models.ResponseKindCollection,
		CollectionResponse: &models.CollectionResponseConfig{
			Primary: models.CollectionQuery{
				CollectionName: "users",
				FilterRules: []models.CollectionFilter{
					{TargetPath: "_id", Value: models.ValueBinding{Source: models.ValueSourcePath, Key: "id"}},
				},
			},
			AdditionalMappers: []models.NamedQuery{
				{OutputKey: "orders", Mode: models.QueryModeFindOne, CollectionQuery: models.CollectionQuery{CollectionName: "orders"}},
			},
		},
	}
	req := &collection.TypedRequestContext{PathParams: map[string]string{"id": "u1"}}
	sess := store.NewEphemeralSession(nil)

	match, err := svc.TryMatch(op, cfg, req, sess)
	if err != nil || !match.Matched {
		t.Fatalf("TryMatch: match=%v err=%v", match, err)
	}
	if _, err := svc.Render(cfg, match, req, sess); err == nil {
		t.Fatal("expected an error from the failing additional mapper")
	}
}

func TestResolveTemplateFor(t *testing.T) {
	svc, getUser, _, _ := setupService(t)
	tmpl, err := svc.ResolveTemplateFor(getUser, 200, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Root != models.RootKindObject {
		t.Fatalf("root = %v, want object", tmpl.Root)
	}
}
