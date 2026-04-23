package models

import "testing"

func TestNormalizeAIScenarioKind(t *testing.T) {
	if got := NormalizeAIScenarioKind(" error "); got != AIScenarioKindError {
		t.Fatalf("expected %q, got %q", AIScenarioKindError, got)
	}
	if got := NormalizeAIScenarioKind("unexpected"); got != AIScenarioKindSuccess {
		t.Fatalf("expected default %q, got %q", AIScenarioKindSuccess, got)
	}
}

func TestNormalizeAIScenariosAndFindAIScenario(t *testing.T) {
	spec := &Spec{
		AIScenarios: []AIScenario{
			{
				Name:         "  unauthorized  ",
				Description:  "  auth failure  ",
				ResponseKind: "unknown",
				Instructions: "  return details  ",
			},
		},
	}

	spec.NormalizeAIScenarios()

	if len(spec.AIScenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(spec.AIScenarios))
	}

	scenario := spec.AIScenarios[0]
	if scenario.Name != "unauthorized" {
		t.Fatalf("expected trimmed name, got %q", scenario.Name)
	}
	if scenario.Description != "auth failure" {
		t.Fatalf("expected trimmed description, got %q", scenario.Description)
	}
	if scenario.Instructions != "return details" {
		t.Fatalf("expected trimmed instructions, got %q", scenario.Instructions)
	}
	if scenario.ResponseKind != AIScenarioKindSuccess {
		t.Fatalf("expected invalid kind to normalize to %q, got %q", AIScenarioKindSuccess, scenario.ResponseKind)
	}
	if scenario.ID == "" {
		t.Fatal("expected scenario ID to be generated")
	}
	if scenario.CreatedAt.IsZero() || scenario.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps to be populated")
	}

	found := spec.FindAIScenario("UNAUTHORIZED")
	if found == nil || found.Name != "unauthorized" {
		t.Fatalf("expected case-insensitive scenario lookup, got %#v", found)
	}
	if spec.FindAIScenario(" ") != nil {
		t.Fatal("expected blank lookup to return nil")
	}
	if spec.FindAIScenario("missing") != nil {
		t.Fatal("expected missing scenario lookup to return nil")
	}
}

func TestNormalizeAIScenariosSeedsDefaults(t *testing.T) {
	spec := &Spec{}

	spec.NormalizeAIScenarios()

	if len(spec.AIScenarios) != 3 {
		t.Fatalf("expected default scenarios to be seeded, got %d", len(spec.AIScenarios))
	}
	if spec.FindAIScenario(DefaultAIScenarioSuccess) == nil {
		t.Fatal("expected success scenario to be present")
	}
}
