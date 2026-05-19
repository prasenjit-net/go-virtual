package models

import "testing"

func TestScriptBindingScopeHelpers(t *testing.T) {
	spec := &ScriptBinding{SpecID: "spec-1"}
	if !spec.IsSpecBinding() {
		t.Fatal("expected spec binding to report true")
	}
	if spec.IsResponseBinding() {
		t.Fatal("expected spec binding to report false for response")
	}

	resp := &ScriptBinding{ResponseConfigID: "resp-1"}
	if !resp.IsResponseBinding() {
		t.Fatal("expected response binding to report true")
	}
	if resp.IsSpecBinding() {
		t.Fatal("expected response binding to report false for spec")
	}
}
