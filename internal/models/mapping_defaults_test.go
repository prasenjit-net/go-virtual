package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDefaultValueJSONRoundTrip(t *testing.T) {
	for _, value := range []string{`null`, `false`, `0`, `""`, `{}`, `[]`} {
		var binding ValueBinding
		raw := `{"source":"body","key":"value","defaultValue":` + value + `}`
		if err := json.Unmarshal([]byte(raw), &binding); err != nil {
			t.Fatal(err)
		}
		if errs := validateValueBinding("binding", binding, filterSourcesBase); len(errs) > 0 {
			t.Fatal(errs)
		}
		encoded, err := json.Marshal(binding)
		if err != nil || !strings.Contains(string(encoded), `"defaultValue":`+value) {
			t.Fatalf("lost default: %s %v", encoded, err)
		}
	}
	var rule FieldMappingRule
	if err := json.Unmarshal([]byte(`{"sourceType":"query","sourceKey":"q","defaultValue":""}`), &rule); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(rule)
	if rule.DefaultValue == nil || *rule.DefaultValue != "" || !strings.Contains(string(encoded), `"defaultValue":""`) {
		t.Fatal("lost empty legacy default")
	}
	if err := json.Unmarshal([]byte(`{"sourceType":"query","sourceKey":"q"}`), &rule); err != nil {
		t.Fatal(err)
	}
	if rule.DefaultValue != nil {
		t.Fatal("omitted default should clear reused struct")
	}
}

func TestInvalidDefaultsRejected(t *testing.T) {
	for _, raw := range []string{
		`{"sourceType":"query","sourceKey":"q","defaultValue":null}`,
		`{"sourceType":"query","sourceKey":"q","defaultValue":42}`,
		`{"sourceType":"literal","sourceKey":"q","defaultValue":"fallback"}`,
		`{"sourceType":"bogus","sourceKey":"q","defaultValue":"fallback"}`,
	} {
		var rule FieldMappingRule
		if err := json.Unmarshal([]byte(raw), &rule); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	var binding ValueBinding
	if err := json.Unmarshal([]byte(`{"source":"literal","value":null,"defaultValue":null}`), &binding); err == nil {
		t.Fatal("literal default accepted")
	}
	if errs := validateValueBinding("binding", ValueBinding{Source: ValueSourceBody, Key: "x", DefaultValue: json.RawMessage(`bad`)}, filterSourcesBase); len(errs) == 0 {
		t.Fatal("malformed default accepted")
	}
}
