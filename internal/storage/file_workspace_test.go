package storage

import (
	"path/filepath"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
)

func TestFileWorkspaceAtomicSaveAndFailureRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	fs, err := NewFileStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.CreateSpec(&models.Spec{ID: "workspace-spec", Name: "Before", Content: "openapi: 3.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := fs.CreateScript(&models.Script{ID: "workspace-script", Name: "Script", Source: "def run(req):\n  return {}\n", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	spec, err := fs.GetSpec("workspace-spec")
	if err != nil {
		t.Fatal(err)
	}
	spec.Name = "After"
	workspace := &models.SpecWorkspace{Spec: spec, SpecScripts: []*models.ScriptBinding{}, SpecValidations: []*models.ValidationRule{}, SpecMappings: []*models.CollectionMapping{}, Operations: []models.SpecWorkspaceOperation{}}
	if err := fs.ApplySpecWorkspace(workspace); err != nil {
		t.Fatalf("apply workspace: %v", err)
	}
	stored, err := fs.GetSpec("workspace-spec")
	if err != nil || stored.Name != "After" {
		t.Fatalf("successful save not visible: %+v, %v", stored, err)
	}

	failed := *workspace
	failed.Spec = &models.Spec{ID: spec.ID, Name: "Must roll back", Content: spec.Content, Version: spec.Version, CreatedAt: spec.CreatedAt, ModePolicy: spec.ModePolicy}
	failed.SpecScripts = []*models.ScriptBinding{
		{ID: "duplicate", ScriptID: "workspace-script", SpecID: spec.ID, OutputKey: "one", Enabled: true},
		{ID: "duplicate", ScriptID: "workspace-script", SpecID: spec.ID, OutputKey: "two", Enabled: true},
	}
	if err := fs.ApplySpecWorkspace(&failed); err == nil {
		t.Fatal("expected staged duplicate binding to fail")
	}
	stored, err = fs.GetSpec("workspace-spec")
	if err != nil || stored.Name != "After" {
		t.Fatalf("failed save changed live workspace: %+v, %v", stored, err)
	}
	bindings, err := fs.GetSpecScriptBindings(spec.ID)
	if err != nil || len(bindings) != 0 {
		t.Fatalf("failed save leaked staged bindings: %+v, %v", bindings, err)
	}
	if err := fs.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewFileStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	stored, err = reopened.GetSpec("workspace-spec")
	if err != nil || stored.Name != "After" {
		t.Fatalf("saved workspace did not survive restart: %+v, %v", stored, err)
	}
}
