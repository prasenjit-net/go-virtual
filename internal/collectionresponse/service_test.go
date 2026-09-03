package collectionresponse

import (
	"encoding/json"
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
