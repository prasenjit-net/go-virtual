package validation

import (
	"testing"

	"github.com/prasenjit/go-virtual/internal/condition"
	"github.com/prasenjit/go-virtual/internal/models"
)

// makeData returns a minimal RequestData for tests that don't need specific values.
func makeData() *condition.RequestData {
	return &condition.RequestData{
		QueryParams: map[string][]string{"status": {"active"}},
	}
}

// passCond is a condition that always evaluates to true (status == "active").
func passCond() *models.ConditionNode {
	return &models.ConditionNode{
		Condition: &models.Condition{
			Source:   models.SourceQuery,
			Key:      "status",
			Operator: models.OpEquals,
			Value:    "active",
		},
	}
}

// failCond is a condition that always evaluates to false.
func failCond() *models.ConditionNode {
	return &models.ConditionNode{
		Condition: &models.Condition{
			Source:   models.SourceQuery,
			Key:      "status",
			Operator: models.OpEquals,
			Value:    "inactive",
		},
	}
}

func TestRunRules_Empty(t *testing.T) {
	output, traces := RunRules(nil, makeData())
	if len(output) != 0 {
		t.Errorf("expected empty output map, got %d entries", len(output))
	}
	if len(traces) != 0 {
		t.Errorf("expected 0 traces, got %d", len(traces))
	}
}

func TestRunRules_DisabledSkipped(t *testing.T) {
	rules := []*models.ValidationRule{
		{
			ID:            "r1",
			Name:          "myRule",
			Enabled:       false,
			ConditionTree: passCond(),
			OnSuccess:     map[string]string{"k": "v"},
		},
	}

	output, traces := RunRules(rules, makeData())
	if len(output) != 0 {
		t.Errorf("expected no output for disabled rule, got %d", len(output))
	}
	if len(traces) != 0 {
		t.Errorf("expected 0 traces for disabled rule, got %d", len(traces))
	}
}

func TestRunRules_Pass(t *testing.T) {
	rules := []*models.ValidationRule{
		{
			ID:            "r1",
			Name:          "checkStatus",
			OperationID:   "op-1",
			Enabled:       true,
			ConditionTree: passCond(),
			OnSuccess:     map[string]string{"level": "ok"},
			OnFailure:     map[string]string{"level": "error"},
		},
	}

	output, traces := RunRules(rules, makeData())

	result, ok := output["checkStatus"]
	if !ok {
		t.Fatal("expected 'checkStatus' in output")
	}
	if result.Status != "pass" {
		t.Errorf("expected status=pass, got %q", result.Status)
	}
	if result.Properties["level"] != "ok" {
		t.Errorf("expected level=ok, got %q", result.Properties["level"])
	}

	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	if traces[0].Status != "pass" {
		t.Errorf("trace status: expected pass, got %q", traces[0].Status)
	}
	if traces[0].RuleID != "r1" {
		t.Errorf("trace RuleID: expected r1, got %q", traces[0].RuleID)
	}
	if traces[0].RuleName != "checkStatus" {
		t.Errorf("trace RuleName: expected checkStatus, got %q", traces[0].RuleName)
	}
}

func TestRunRules_Fail(t *testing.T) {
	rules := []*models.ValidationRule{
		{
			ID:            "r1",
			Name:          "checkStatus",
			OperationID:   "op-1",
			Enabled:       true,
			ConditionTree: failCond(),
			OnSuccess:     map[string]string{"level": "ok"},
			OnFailure:     map[string]string{"level": "error"},
		},
	}

	output, traces := RunRules(rules, makeData())

	result, ok := output["checkStatus"]
	if !ok {
		t.Fatal("expected 'checkStatus' in output")
	}
	if result.Status != "fail" {
		t.Errorf("expected status=fail, got %q", result.Status)
	}
	if result.Properties["level"] != "error" {
		t.Errorf("expected level=error, got %q", result.Properties["level"])
	}

	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	if traces[0].Status != "fail" {
		t.Errorf("trace status: expected fail, got %q", traces[0].Status)
	}
}

func TestRunRules_SpecScope(t *testing.T) {
	rules := []*models.ValidationRule{
		{
			ID:            "r1",
			Name:          "specRule",
			SpecID:        "spec-1",
			Enabled:       true,
			ConditionTree: passCond(),
		},
	}

	_, traces := RunRules(rules, makeData())
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	if traces[0].Scope != "spec" {
		t.Errorf("expected scope=spec for rule with SpecID, got %q", traces[0].Scope)
	}
}

func TestRunRules_OperationScope(t *testing.T) {
	rules := []*models.ValidationRule{
		{
			ID:          "r1",
			Name:        "opRule",
			OperationID: "op-1",
			Enabled:     true,
		},
	}

	_, traces := RunRules(rules, makeData())
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	if traces[0].Scope != "operation" {
		t.Errorf("expected scope=operation for rule with OperationID, got %q", traces[0].Scope)
	}
}

func TestRunRules_TraceCount(t *testing.T) {
	rules := []*models.ValidationRule{
		{ID: "r1", Name: "a", Enabled: true, ConditionTree: passCond()},
		{ID: "r2", Name: "b", Enabled: true, ConditionTree: failCond()},
		{ID: "r3", Name: "c", Enabled: false, ConditionTree: passCond()},
		{ID: "r4", Name: "d", Enabled: true, ConditionTree: passCond()},
	}

	output, traces := RunRules(rules, makeData())
	// Only 3 enabled rules should produce output + traces
	if len(output) != 3 {
		t.Errorf("expected 3 output entries, got %d", len(output))
	}
	if len(traces) != 3 {
		t.Errorf("expected 3 traces, got %d", len(traces))
	}
}

func TestRunRules_NilConditionTree(t *testing.T) {
	// nil ConditionTree should always pass (EvaluateTree(nil) == true)
	rules := []*models.ValidationRule{
		{
			ID:            "r1",
			Name:          "alwaysPass",
			Enabled:       true,
			ConditionTree: nil,
			OnSuccess:     map[string]string{"ok": "yes"},
			OnFailure:     map[string]string{"ok": "no"},
		},
	}

	output, _ := RunRules(rules, makeData())
	result, ok := output["alwaysPass"]
	if !ok {
		t.Fatal("expected 'alwaysPass' in output")
	}
	if result.Status != "pass" {
		t.Errorf("nil condition tree should pass, got status=%q", result.Status)
	}
	if result.Properties["ok"] != "yes" {
		t.Errorf("expected ok=yes from OnSuccess, got %q", result.Properties["ok"])
	}
}
