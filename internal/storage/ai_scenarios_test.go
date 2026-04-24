package storage

import (
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
)

func TestMemoryStorageAIScenarioCRUD(t *testing.T) {
	store := NewMemoryStorage()

	initial, err := store.ListAIScenarios()
	if err != nil {
		t.Fatalf("ListAIScenarios error: %v", err)
	}
	if len(initial) < 3 {
		t.Fatalf("expected default scenarios, got %d", len(initial))
	}

	scenario := &models.AIScenario{
		ID:           "custom",
		Name:         "unauthorized",
		ResponseKind: models.AIScenarioKindError,
		StatusCode:   401,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := store.CreateAIScenario(scenario); err != nil {
		t.Fatalf("CreateAIScenario error: %v", err)
	}

	got, err := store.GetAIScenario("custom")
	if err != nil {
		t.Fatalf("GetAIScenario error: %v", err)
	}
	if got.Name != "unauthorized" {
		t.Fatalf("unexpected scenario name %q", got.Name)
	}

	got.StatusCode = 403
	if err := store.UpdateAIScenario(got); err != nil {
		t.Fatalf("UpdateAIScenario error: %v", err)
	}

	updated, err := store.GetAIScenario("custom")
	if err != nil {
		t.Fatalf("GetAIScenario after update error: %v", err)
	}
	if updated.StatusCode != 403 {
		t.Fatalf("expected updated status code, got %d", updated.StatusCode)
	}

	if err := store.DeleteAIScenario("custom"); err != nil {
		t.Fatalf("DeleteAIScenario error: %v", err)
	}
	if _, err := store.GetAIScenario("custom"); err == nil {
		t.Fatal("expected deleted scenario lookup to fail")
	}
}

func TestFileStorageAIScenarioPersistenceAndMigration(t *testing.T) {
	baseDir := t.TempDir()

	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage error: %v", err)
	}

	scenario := &models.AIScenario{
		ID:           "shared",
		Name:         "five_items",
		ResponseKind: models.AIScenarioKindSuccess,
		Count:        5,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := fs.CreateAIScenario(scenario); err != nil {
		t.Fatalf("CreateAIScenario error: %v", err)
	}

	scenario.StatusCode = 202
	if err := fs.UpdateAIScenario(scenario); err != nil {
		t.Fatalf("UpdateAIScenario error: %v", err)
	}

	reloaded, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("reload NewFileStorage error: %v", err)
	}

	loaded, err := reloaded.GetAIScenario("shared")
	if err != nil {
		t.Fatalf("GetAIScenario error: %v", err)
	}
	if loaded.Count != 5 || loaded.StatusCode != 202 {
		t.Fatalf("unexpected persisted scenario: %#v", loaded)
	}

	if err := reloaded.DeleteAIScenario("shared"); err != nil {
		t.Fatalf("DeleteAIScenario error: %v", err)
	}
	if _, err := reloaded.GetAIScenario("shared"); err == nil {
		t.Fatal("expected deleted scenario lookup to fail")
	}

	legacyDir := t.TempDir()
	legacy, err := NewFileStorage(legacyDir)
	if err != nil {
		t.Fatalf("legacy NewFileStorage error: %v", err)
	}

	spec := &models.Spec{
		ID:          "spec-1",
		Name:        "Legacy",
		AIScenarios: []models.AIScenario{{ID: "legacy-scenario", Name: "legacy_error", ResponseKind: models.AIScenarioKindError, StatusCode: 418, Enabled: true}},
	}
	spec.NormalizeMode()
	if err := legacy.CreateSpec(spec); err != nil {
		t.Fatalf("CreateSpec error: %v", err)
	}

	reloadedLegacy, err := NewFileStorage(legacyDir)
	if err != nil {
		t.Fatalf("reload legacy storage error: %v", err)
	}

	migrated, err := reloadedLegacy.ListAIScenarios()
	if err != nil {
		t.Fatalf("ListAIScenarios error: %v", err)
	}
	if models.FindAIScenario(derefAIScenarios(migrated), "legacy_error") == nil {
		t.Fatal("expected legacy spec scenario to migrate into global AI scenarios")
	}
}

func derefAIScenarios(items []*models.AIScenario) []models.AIScenario {
	result := make([]models.AIScenario, 0, len(items))
	for _, item := range items {
		if item != nil {
			result = append(result, *item)
		}
	}
	return result
}
