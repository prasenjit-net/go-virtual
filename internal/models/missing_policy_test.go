package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMissingPolicyRoundTripAndValidation(t *testing.T) {
	for _, typed := range []bool{false, true} {
		base := `"sourceType":"body","sourceKey":"name"`
		var target any = &FieldMappingRule{}
		if typed {
			base = `"source":"body","key":"name"`
			target = &ValueBinding{}
		}
		if err := json.Unmarshal([]byte("{"+base+`,"skipWhenMissing":true}`), target); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(target)
		if err != nil || !strings.Contains(string(raw), `"skipWhenMissing":true`) {
			t.Fatalf("lost policy: %s %v", raw, err)
		}
		if err := json.Unmarshal([]byte("{"+base+`,"skipWhenMissing":true,"defaultValue":"fallback"}`), target); err == nil {
			t.Fatal("conflicting policies accepted")
		}
		if err := json.Unmarshal([]byte("{"+base+"}"), target); err != nil {
			t.Fatal(err)
		}
		raw, _ = json.Marshal(target)
		if strings.Contains(string(raw), "skipWhenMissing") {
			t.Fatal("omitted policy not cleared")
		}
	}
	for _, raw := range []string{`{"source":"literal","value":1,"skipWhenMissing":true}`, `{"source":"invalid","key":"name","skipWhenMissing":true}`} {
		var target ValueBinding
		if err := json.Unmarshal([]byte(raw), &target); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
