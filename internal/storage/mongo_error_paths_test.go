package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/models"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func newUnavailableMongoStorage(t *testing.T) *MongoStorage {
	t.Helper()
	client, err := mongo.Connect(
		options.Client().
			ApplyURI("mongodb://127.0.0.1:1/?directConnection=true").
			SetConnectTimeout(50 * time.Millisecond).
			SetServerSelectionTimeout(50 * time.Millisecond),
	)
	if err != nil {
		t.Fatalf("mongo.Connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(t.Context()) })
	return &MongoStorage{client: client, db: client.Database("go_virtual_test"), prefix: "gv_"}
}

func TestMongoStorage_Helpers(t *testing.T) {
	if _, err := NewMongoStorage(config.MongoConfig{}); err == nil {
		t.Fatal("expected empty URI to be rejected")
	}

	doc, err := marshalDoc("id-1", "spec-1", "op-1", "script-1", &models.ScriptBinding{ID: "id-1", OutputKey: "result"})
	if err != nil {
		t.Fatalf("marshalDoc: %v", err)
	}
	if doc.ID != "id-1" || doc.SpecID != "spec-1" || doc.OperationID != "op-1" || doc.ScriptID != "script-1" {
		t.Fatalf("unexpected genericDoc: %#v", doc)
	}

	var binding models.ScriptBinding
	if err := unmarshalDoc(doc, &binding); err != nil {
		t.Fatalf("unmarshalDoc: %v", err)
	}
	if binding.ID != "id-1" || binding.OutputKey != "result" {
		t.Fatalf("unexpected binding: %#v", binding)
	}

	ctx, cancel := ctxTimeout()
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("expected ctxTimeout to set a deadline")
	}

	m := newUnavailableMongoStorage(t)
	if got := m.col(colSpecs).Name(); got != "gv_"+colSpecs {
		t.Fatalf("unexpected collection name: %q", got)
	}
}

func TestMongoStorage_ErrorPaths(t *testing.T) {
	m := newUnavailableMongoStorage(t)
	ops := []struct {
		name string
		fn   func() error
	}{
		{"CreateSpec", func() error { return m.CreateSpec(&models.Spec{ID: "spec-1"}) }},
		{"GetSpec", func() error { _, err := m.GetSpec("spec-1"); return err }},
		{"GetAllSpecs", func() error { _, err := m.GetAllSpecs(); return err }},
		{"GetEnabledSpecs", func() error { _, err := m.GetEnabledSpecs(); return err }},
		{"UpdateSpec", func() error { return m.UpdateSpec(&models.Spec{ID: "spec-1"}) }},
		{"DeleteSpec", func() error { return m.DeleteSpec("spec-1") }},
		{"ListTags", func() error { _, err := m.ListTags(); return err }},
		{"GetTag", func() error { _, err := m.GetTag("blue"); return err }},
		{"CreateTag", func() error { return m.CreateTag(&models.Tag{Name: "blue"}) }},
		{"UpdateTag", func() error { return m.UpdateTag("blue", &models.Tag{Name: "blue"}) }},
		{"DeleteTag", func() error { return m.DeleteTag("blue") }},
		{"CreateOperation", func() error { return m.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"}) }},
		{"GetOperation", func() error { _, err := m.GetOperation("op-1"); return err }},
		{"GetOperationsBySpec", func() error { _, err := m.GetOperationsBySpec("spec-1"); return err }},
		{"GetAllOperations", func() error { _, err := m.GetAllOperations(); return err }},
		{"UpdateOperation", func() error { return m.UpdateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"}) }},
		{"DeleteOperation", func() error { return m.DeleteOperation("op-1") }},
		{"DeleteOperationsBySpec", func() error { return m.DeleteOperationsBySpec("spec-1") }},
		{"CreateResponseConfig", func() error {
			return m.CreateResponseConfig(&models.ResponseConfig{ID: "resp-1", OperationID: "op-1", Name: "resp", StatusCode: 200})
		}},
		{"GetResponseConfig", func() error { _, err := m.GetResponseConfig("resp-1"); return err }},
		{"GetResponseConfigsByOperation", func() error { _, err := m.GetResponseConfigsByOperation("op-1"); return err }},
		{"UpdateResponseConfig", func() error {
			return m.UpdateResponseConfig(&models.ResponseConfig{ID: "resp-1", OperationID: "op-1", Name: "resp", StatusCode: 200})
		}},
		{"DeleteResponseConfig", func() error { return m.DeleteResponseConfig("resp-1") }},
		{"DeleteResponseConfigsByOperation", func() error { return m.DeleteResponseConfigsByOperation("op-1") }},
		{"CreateScript", func() error { return m.CreateScript(&models.Script{ID: "script-1", Name: "script"}) }},
		{"GetScript", func() error { _, err := m.GetScript("script-1"); return err }},
		{"GetAllScripts", func() error { _, err := m.GetAllScripts(); return err }},
		{"UpdateScript", func() error { return m.UpdateScript(&models.Script{ID: "script-1", Name: "script"}) }},
		{"DeleteScript", func() error { return m.DeleteScript("script-1") }},
		{"ListAIScenarios", func() error { _, err := m.ListAIScenarios(); return err }},
		{"GetAIScenario", func() error { _, err := m.GetAIScenario("scenario-1"); return err }},
		{"CreateAIScenario", func() error { return m.CreateAIScenario(&models.AIScenario{ID: "scenario-1", Name: "happy"}) }},
		{"UpdateAIScenario", func() error { return m.UpdateAIScenario(&models.AIScenario{ID: "scenario-1", Name: "happy"}) }},
		{"DeleteAIScenario", func() error { return m.DeleteAIScenario("scenario-1") }},
		{"GetScriptBindings", func() error { _, err := m.GetScriptBindings("op-1"); return err }},
		{"GetSpecScriptBindings", func() error { _, err := m.GetSpecScriptBindings("spec-1"); return err }},
		{"GetResponseScriptBindings", func() error { _, err := m.GetResponseScriptBindings("resp-1"); return err }},
		{"CreateScriptBinding", func() error {
			return m.CreateScriptBinding(&models.ScriptBinding{ID: "bind-1", OperationID: "op-1", ScriptID: "script-1", OutputKey: "result"})
		}},
		{"UpdateScriptBinding", func() error {
			return m.UpdateScriptBinding(&models.ScriptBinding{ID: "bind-1", OperationID: "op-1", ScriptID: "script-1", OutputKey: "result"})
		}},
		{"DeleteScriptBinding", func() error { return m.DeleteScriptBinding("bind-1") }},
		{"DeleteScriptBindingsByScript", func() error { return m.DeleteScriptBindingsByScript("script-1") }},
		{"DeleteScriptBindingsBySpec", func() error { return m.DeleteScriptBindingsBySpec("spec-1") }},
		{"DeleteScriptBindingsByResponse", func() error { return m.DeleteScriptBindingsByResponse("resp-1") }},
	}

	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			err := op.fn()
			if err == nil {
				t.Fatalf("expected %s to fail against unavailable MongoDB", op.name)
			}
			if !strings.Contains(strings.ToLower(err.Error()), "mongo") && !strings.Contains(strings.ToLower(err.Error()), "server selection") {
				t.Fatalf("expected mongo-related error, got %v", err)
			}
		})
	}

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
