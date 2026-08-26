package collection

import (
	"context"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
)

func TestExecutorInsertAndFind(t *testing.T) {
	st := storage.NewMemoryStorage()
	exec := NewExecutor(st, store.NewMemoryCollectionBackend())

	// Create a response config and mapping
	resp := &models.ResponseConfig{ID: "r1", OperationID: "op1", Enabled: true}
	st.CreateResponseConfig(resp)

	mapping := &models.CollectionMapping{
		ID:               "m1",
		ResponseConfigID: "r1",
		CollectionName:   "users",
		Name:             "Create User",
		Operation:        models.ColOpInsert,
		DataRules: []models.FieldMappingRule{
			{TargetField: "name", SourceType: "literal", SourceKey: "TestUser"},
		},
		OutputKey: "newUser",
		Order:     1,
		Enabled:   true,
	}
	st.CreateCollectionMapping(mapping)

	sess := store.NewEphemeralSession(nil)
	reqCtx := &RequestContext{}

	output, traces, err := exec.Run(context.Background(), "r1", reqCtx, sess)
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	if traces[0].Error != "" {
		t.Errorf("unexpected error: %s", traces[0].Error)
	}
	if traces[0].RecordCount != 1 {
		t.Errorf("expected record count 1, got %d", traces[0].RecordCount)
	}

	doc, ok := output["newUser"].(map[string]any)
	if !ok {
		t.Fatalf("expected newUser to be a map, got %T", output["newUser"])
	}
	if doc["name"] != "TestUser" {
		t.Errorf("expected name=TestUser, got %v", doc["name"])
	}
	if doc["_id"] == "" {
		t.Error("expected _id to be set")
	}

	// Now run a find-one mapping on the same session
	findMapping := &models.CollectionMapping{
		ID:               "m2",
		ResponseConfigID: "r2",
		CollectionName:   "users",
		Name:             "Find User",
		Operation:        models.ColOpFindOne,
		FilterRules: []models.FieldMappingRule{
			{TargetField: "name", SourceType: "literal", SourceKey: "TestUser"},
		},
		OutputKey: "user",
		Order:     1,
		Enabled:   true,
	}
	resp2 := &models.ResponseConfig{ID: "r2", OperationID: "op1", Enabled: true}
	st.CreateResponseConfig(resp2)
	st.CreateCollectionMapping(findMapping)

	output2, _, err := exec.Run(context.Background(), "r2", reqCtx, sess)
	if err != nil {
		t.Fatal(err)
	}
	found, ok := output2["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user to be a map, got %T", output2["user"])
	}
	if found["name"] != "TestUser" {
		t.Errorf("expected name=TestUser, got %v", found["name"])
	}
}

func TestExecutorRunForSpec(t *testing.T) {
	st := storage.NewMemoryStorage()
	exec := NewExecutor(st, store.NewMemoryCollectionBackend())

	mapping := &models.CollectionMapping{
		ID:             "m1",
		SpecID:         "spec1",
		CollectionName: "items",
		Name:           "Spec Insert",
		Operation:      models.ColOpInsert,
		DataRules:      []models.FieldMappingRule{{TargetField: "src", SourceType: "literal", SourceKey: "spec"}},
		OutputKey:      "specItem",
		Order:          1,
		Enabled:        true,
	}
	st.CreateCollectionMapping(mapping)

	sess := store.NewEphemeralSession(nil)
	output, traces, err := exec.RunForSpec(context.Background(), "spec1", &RequestContext{}, sess)
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	if traces[0].Error != "" {
		t.Errorf("unexpected error: %s", traces[0].Error)
	}
	doc, ok := output["specItem"].(map[string]any)
	if !ok {
		t.Fatalf("expected specItem map, got %T", output["specItem"])
	}
	if doc["src"] != "spec" {
		t.Errorf("expected src=spec, got %v", doc["src"])
	}
}

func TestExecutorRunForOperation(t *testing.T) {
	st := storage.NewMemoryStorage()
	exec := NewExecutor(st, store.NewMemoryCollectionBackend())

	mapping := &models.CollectionMapping{
		ID:             "m1",
		OperationID:    "op1",
		CollectionName: "items",
		Name:           "Op Insert",
		Operation:      models.ColOpInsert,
		DataRules:      []models.FieldMappingRule{{TargetField: "src", SourceType: "literal", SourceKey: "op"}},
		OutputKey:      "opItem",
		Order:          1,
		Enabled:        true,
	}
	st.CreateCollectionMapping(mapping)

	sess := store.NewEphemeralSession(nil)
	output, traces, err := exec.RunForOperation(context.Background(), "op1", &RequestContext{}, sess)
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	doc, ok := output["opItem"].(map[string]any)
	if !ok {
		t.Fatalf("expected opItem map, got %T", output["opItem"])
	}
	if doc["src"] != "op" {
		t.Errorf("expected src=op, got %v", doc["src"])
	}
}

func TestExecutorRunOneMapping(t *testing.T) {
	exec := NewExecutor(storage.NewMemoryStorage(), store.NewMemoryCollectionBackend())
	sess := store.NewEphemeralSession(nil)

	mapping := &models.CollectionMapping{
		ID:             "m1",
		CollectionName: "items",
		Name:           "Create Item",
		Operation:      models.ColOpInsert,
		DataRules:      []models.FieldMappingRule{{TargetField: "src", SourceType: "literal", SourceKey: "single"}},
		OutputKey:      "item",
		Enabled:        true,
	}

	output, trace, err := exec.RunOneMapping(context.Background(), mapping, &RequestContext{}, sess)
	if err != nil {
		t.Fatal(err)
	}
	if trace.MappingID != "m1" || trace.MappingName != "Create Item" || trace.RecordCount != 1 {
		t.Fatalf("unexpected trace: %+v", trace)
	}
	doc, ok := output["item"].(map[string]any)
	if !ok {
		t.Fatalf("item: got %T, want map[string]any", output["item"])
	}
	if doc["src"] != "single" || doc["_status"] != "success" {
		t.Fatalf("unexpected item output: %+v", doc)
	}
}

func TestExecutorRunOneMappingReturnsNotFoundStatus(t *testing.T) {
	exec := NewExecutor(storage.NewMemoryStorage(), store.NewMemoryCollectionBackend())
	sess := store.NewEphemeralSession(nil)

	mapping := &models.CollectionMapping{
		ID:             "m1",
		CollectionName: "items",
		Operation:      models.ColOpFindOne,
		FilterRules:    []models.FieldMappingRule{{TargetField: "_id", SourceType: "literal", SourceKey: "missing"}},
		OutputKey:      "item",
		Enabled:        true,
	}

	output, trace, err := exec.RunOneMapping(context.Background(), mapping, &RequestContext{}, sess)
	if err != nil {
		t.Fatal(err)
	}
	if trace.RecordCount != 0 || trace.Error != "" {
		t.Fatalf("unexpected trace: %+v", trace)
	}
	status, ok := output["item"].(map[string]any)
	if !ok {
		t.Fatalf("item: got %T, want map[string]any", output["item"])
	}
	if status["_status"] != "not_found" {
		t.Fatalf("unexpected status output: %+v", status)
	}
}

func TestExecutorSkipsDisabledMappings(t *testing.T) {
	st := storage.NewMemoryStorage()
	exec := NewExecutor(st, store.NewMemoryCollectionBackend())

	resp := &models.ResponseConfig{ID: "r1", OperationID: "op1"}
	st.CreateResponseConfig(resp)

	mapping := &models.CollectionMapping{
		ID:               "m1",
		ResponseConfigID: "r1",
		CollectionName:   "things",
		Operation:        models.ColOpInsert,
		DataRules:        []models.FieldMappingRule{{TargetField: "x", SourceType: "literal", SourceKey: "1"}},
		OutputKey:        "result",
		Enabled:          false, // disabled
	}
	st.CreateCollectionMapping(mapping)

	sess := store.NewEphemeralSession(nil)
	output, traces, err := exec.Run(context.Background(), "r1", &RequestContext{}, sess)
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) != 0 {
		t.Errorf("expected 0 traces for disabled mapping, got %d", len(traces))
	}
	if len(output) != 0 {
		t.Errorf("expected empty output for disabled mapping")
	}
}
