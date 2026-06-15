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
