package models

import (
	"testing"
)

func TestConditionConstants(t *testing.T) {
	// Test source constants
	if SourcePath != "path" {
		t.Errorf("Expected SourcePath to be 'path', got %q", SourcePath)
	}
	if SourceQuery != "query" {
		t.Errorf("Expected SourceQuery to be 'query', got %q", SourceQuery)
	}
	if SourceHeader != "header" {
		t.Errorf("Expected SourceHeader to be 'header', got %q", SourceHeader)
	}
	if SourceBody != "body" {
		t.Errorf("Expected SourceBody to be 'body', got %q", SourceBody)
	}
}

func TestOperatorConstants(t *testing.T) {
	operators := []struct {
		constant string
		expected string
	}{
		{OpEquals, "eq"},
		{OpContains, "contains"},
		{OpRegex, "regex"},
		{OpExists, "exists"},
		{OpGreaterThan, "gt"},
		{OpLessThan, "lt"},
		{OpGTE, "gte"},
		{OpLTE, "lte"},
		{OpStartsWith, "startsWith"},
		{OpEndsWith, "endsWith"},
		{OpDateEquals, "dateEq"},
		{OpDateBefore, "dateBefore"},
		{OpDateAfter, "dateAfter"},
		{OpDateLte, "dateLte"},
		{OpDateGte, "dateGte"},
		{OpDateInPast, "dateInPast"},
		{OpDateInFuture, "dateInFuture"},
		{OpDateToday, "dateToday"},
		{OpDateBetween, "dateBetween"},
		// Deprecated — keep constants
		{OpNotEquals, "ne"},
		{OpNotContains, "notContains"},
		{OpNotExists, "notExists"},
	}

	for _, op := range operators {
		if op.constant != op.expected {
			t.Errorf("Expected %q, got %q", op.expected, op.constant)
		}
	}
}

func TestValidSources(t *testing.T) {
	sources := ValidSources()

	expected := []string{"path", "query", "header", "body", "signature", "script", "validation", "collection"}
	if len(sources) != len(expected) {
		t.Errorf("Expected %d sources, got %d", len(expected), len(sources))
	}

	for i, src := range expected {
		if sources[i] != src {
			t.Errorf("Expected source %q at index %d, got %q", src, i, sources[i])
		}
	}
}

func TestValidOperators(t *testing.T) {
	operators := ValidOperators()

	// 10 standard + 9 date = 19 current operators (deprecated excluded)
	if len(operators) != 19 {
		t.Errorf("Expected 19 operators, got %d", len(operators))
	}

	// Deprecated operators must NOT appear in ValidOperators
	deprecated := map[string]bool{"ne": true, "notContains": true, "notExists": true}
	for _, op := range operators {
		if deprecated[op] {
			t.Errorf("Deprecated operator %q should not appear in ValidOperators()", op)
		}
	}

	// Key operators must be present
	required := map[string]bool{
		"eq": true, "contains": true, "regex": true, "exists": true,
		"gt": true, "lt": true,
		"dateEq": true, "dateBefore": true, "dateInPast": true, "dateBetween": true,
	}
	for _, op := range operators {
		delete(required, op)
	}
	if len(required) > 0 {
		t.Errorf("Missing operators: %v", required)
	}
}

func TestDeprecatedOperators(t *testing.T) {
	deps := DeprecatedOperators()
	if len(deps) != 3 {
		t.Errorf("Expected 3 deprecated operators, got %d", len(deps))
	}
	set := map[string]bool{}
	for _, d := range deps {
		set[d] = true
	}
	for _, expected := range []string{"ne", "notContains", "notExists"} {
		if !set[expected] {
			t.Errorf("Expected %q in DeprecatedOperators()", expected)
		}
	}
}

func TestConditionStruct(t *testing.T) {
	cond := Condition{
		Source:   SourcePath,
		Key:      "id",
		Operator: OpEquals,
		Value:    "123",
	}

	if cond.Source != "path" {
		t.Errorf("Expected source 'path', got %q", cond.Source)
	}
	if cond.Key != "id" {
		t.Errorf("Expected key 'id', got %q", cond.Key)
	}
	if cond.Operator != "eq" {
		t.Errorf("Expected operator 'eq', got %q", cond.Operator)
	}
	if cond.Value != "123" {
		t.Errorf("Expected value '123', got %q", cond.Value)
	}
	if cond.Negate {
		t.Error("Expected Negate to default to false")
	}
}

func TestNormaliseDeprecatedOperator(t *testing.T) {
	tests := []struct {
		name        string
		input       Condition
		wantOp      string
		wantNegate  bool
	}{
		{
			name:       "ne → eq + negate",
			input:      Condition{Operator: OpNotEquals, Value: "x"},
			wantOp:     OpEquals,
			wantNegate: true,
		},
		{
			name:       "notContains → contains + negate",
			input:      Condition{Operator: OpNotContains, Value: "x"},
			wantOp:     OpContains,
			wantNegate: true,
		},
		{
			name:       "notExists → exists + negate",
			input:      Condition{Operator: OpNotExists},
			wantOp:     OpExists,
			wantNegate: true,
		},
		{
			name:       "ne with negate already true → eq + negate false (double negate)",
			input:      Condition{Operator: OpNotEquals, Negate: true},
			wantOp:     OpEquals,
			wantNegate: false,
		},
		{
			name:       "current operator unchanged",
			input:      Condition{Operator: OpEquals, Value: "x"},
			wantOp:     OpEquals,
			wantNegate: false,
		},
		{
			name:       "regex unchanged",
			input:      Condition{Operator: OpRegex, Value: "uuid"},
			wantOp:     OpRegex,
			wantNegate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormaliseDeprecatedOperator(tt.input)
			if got.Operator != tt.wantOp {
				t.Errorf("operator: got %q, want %q", got.Operator, tt.wantOp)
			}
			if got.Negate != tt.wantNegate {
				t.Errorf("negate: got %v, want %v", got.Negate, tt.wantNegate)
			}
		})
	}
}
