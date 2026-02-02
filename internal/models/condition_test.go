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
		{OpNotEquals, "ne"},
		{OpContains, "contains"},
		{OpNotContains, "notContains"},
		{OpRegex, "regex"},
		{OpExists, "exists"},
		{OpNotExists, "notExists"},
		{OpGreaterThan, "gt"},
		{OpLessThan, "lt"},
		{OpGTE, "gte"},
		{OpLTE, "lte"},
		{OpStartsWith, "startsWith"},
		{OpEndsWith, "endsWith"},
	}

	for _, op := range operators {
		if op.constant != op.expected {
			t.Errorf("Expected %q, got %q", op.expected, op.constant)
		}
	}
}

func TestValidSources(t *testing.T) {
	sources := ValidSources()

	expected := []string{"path", "query", "header", "body"}
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

	if len(operators) != 13 {
		t.Errorf("Expected 13 operators, got %d", len(operators))
	}

	// Check that key operators are included
	expectedOps := map[string]bool{
		"eq": true, "ne": true, "contains": true, "regex": true,
		"exists": true, "gt": true, "lt": true,
	}

	for _, op := range operators {
		delete(expectedOps, op)
	}

	if len(expectedOps) > 0 {
		t.Errorf("Missing operators: %v", expectedOps)
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
}
