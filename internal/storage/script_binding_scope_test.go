package storage

import (
	"os"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
)

func TestMemoryScopedScriptBindings(t *testing.T) {
	s := NewMemoryStorage()
	_ = s.CreateScriptBinding(&models.ScriptBinding{ID: "spec-2", SpecID: "spec-1", ScriptID: "s1", OutputKey: "b", Order: 2})
	_ = s.CreateScriptBinding(&models.ScriptBinding{ID: "spec-1", SpecID: "spec-1", ScriptID: "s1", OutputKey: "a", Order: 1})
	_ = s.CreateScriptBinding(&models.ScriptBinding{ID: "resp-2", ResponseConfigID: "resp-1", ScriptID: "s1", OutputKey: "y", Order: 2})
	_ = s.CreateScriptBinding(&models.ScriptBinding{ID: "resp-1", ResponseConfigID: "resp-1", ScriptID: "s1", OutputKey: "x", Order: 1})
	_ = s.CreateScriptBinding(&models.ScriptBinding{ID: "op-1", OperationID: "op-1", ScriptID: "s1", OutputKey: "z", Order: 0})

	specBindings, err := s.GetSpecScriptBindings("spec-1")
	if err != nil {
		t.Fatalf("GetSpecScriptBindings: %v", err)
	}
	if len(specBindings) != 2 || specBindings[0].ID != "spec-1" || specBindings[1].ID != "spec-2" {
		t.Fatalf("unexpected spec bindings: %#v", specBindings)
	}

	respBindings, err := s.GetResponseScriptBindings("resp-1")
	if err != nil {
		t.Fatalf("GetResponseScriptBindings: %v", err)
	}
	if len(respBindings) != 2 || respBindings[0].ID != "resp-1" || respBindings[1].ID != "resp-2" {
		t.Fatalf("unexpected response bindings: %#v", respBindings)
	}

	if err := s.DeleteScriptBindingsBySpec("spec-1"); err != nil {
		t.Fatalf("DeleteScriptBindingsBySpec: %v", err)
	}
	specBindings, _ = s.GetSpecScriptBindings("spec-1")
	if len(specBindings) != 0 {
		t.Fatalf("expected spec bindings to be deleted, got %#v", specBindings)
	}
	respBindings, _ = s.GetResponseScriptBindings("resp-1")
	if len(respBindings) != 2 {
		t.Fatalf("expected response bindings to remain, got %#v", respBindings)
	}

	if err := s.DeleteScriptBindingsByResponse("resp-1"); err != nil {
		t.Fatalf("DeleteScriptBindingsByResponse: %v", err)
	}
	respBindings, _ = s.GetResponseScriptBindings("resp-1")
	if len(respBindings) != 0 {
		t.Fatalf("expected response bindings to be deleted, got %#v", respBindings)
	}
}

func TestFileStorage_SpecAndResponseScriptBindings(t *testing.T) {
	baseDir := t.TempDir()
	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}

	specBinding := &models.ScriptBinding{ID: "spec-1", SpecID: "spec-1", ScriptID: "s1", OutputKey: "spec", Order: 0}
	respBinding := &models.ScriptBinding{ID: "resp-1", ResponseConfigID: "resp-1", ScriptID: "s1", OutputKey: "resp", Order: 0}
	if err := fs.CreateScriptBinding(specBinding); err != nil {
		t.Fatalf("CreateScriptBinding spec: %v", err)
	}
	if err := fs.CreateScriptBinding(respBinding); err != nil {
		t.Fatalf("CreateScriptBinding response: %v", err)
	}

	specBindings, err := fs.GetSpecScriptBindings("spec-1")
	if err != nil {
		t.Fatalf("GetSpecScriptBindings: %v", err)
	}
	if len(specBindings) != 1 || specBindings[0].ID != "spec-1" {
		t.Fatalf("unexpected spec bindings: %#v", specBindings)
	}
	respBindings, err := fs.GetResponseScriptBindings("resp-1")
	if err != nil {
		t.Fatalf("GetResponseScriptBindings: %v", err)
	}
	if len(respBindings) != 1 || respBindings[0].ID != "resp-1" {
		t.Fatalf("unexpected response bindings: %#v", respBindings)
	}

	if _, err := os.Stat(fs.specScriptBindingsPath("spec-1")); err != nil {
		t.Fatalf("expected spec binding file to exist: %v", err)
	}
	if _, err := os.Stat(fs.responseScriptBindingsPath("resp-1")); err != nil {
		t.Fatalf("expected response binding file to exist: %v", err)
	}

	if err := fs.DeleteScriptBindingsBySpec("spec-1"); err != nil {
		t.Fatalf("DeleteScriptBindingsBySpec: %v", err)
	}
	if _, err := os.Stat(fs.specScriptBindingsPath("spec-1")); !os.IsNotExist(err) {
		t.Fatalf("expected spec binding file cleanup, got %v", err)
	}

	if err := fs.DeleteScriptBindingsByResponse("resp-1"); err != nil {
		t.Fatalf("DeleteScriptBindingsByResponse: %v", err)
	}
	if _, err := os.Stat(fs.responseScriptBindingsPath("resp-1")); !os.IsNotExist(err) {
		t.Fatalf("expected response binding file cleanup, got %v", err)
	}
}
