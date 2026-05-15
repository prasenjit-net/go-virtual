package condition

import (
	"regexp"
	"strings"
	"testing"
)

// sampleValues maps each token to a representative string that should match.
var sampleValues = map[string]string{
	"uuid":         "550e8400-e29b-41d4-a716-446655440000",
	"uuid4":        "550e8400-e29b-41d4-a716-446655440000",
	"email":        "user@example.com",
	"url":          "https://example.com/path?q=1",
	"ipv4":         "192.168.1.1",
	"ipv6":         "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
	"ip":           "10.0.0.1",
	"us-phone":     "555-867-5309",
	"us-zip":       "90210",
	"ssn":          "123-45-6789",
	"date-iso":     "2024-06-15",
	"datetime-iso": "2024-06-15T14:30:00Z",
	"time-hms":     "14:30:00",
	"integer":      "42",
	"decimal":      "3.14",
	"alpha":        "HelloWorld",
	"alphanumeric": "Hello123",
	"slug":         "my-great-post",
	"hex-color":    "#ff6600",
	"base64":       "SGVsbG8gV29ybGQ=",
	"jwt":          "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.abc123",
	"credit-card":  "4111111111111111",
	"iban":         "GB29NWBK60161331926819",
	"semver":       "1.2.3",
}

func TestPatternCatalogueCoversAllTokens(t *testing.T) {
	catalogue := PatternCatalogue()
	if len(catalogue) != len(sampleValues) {
		t.Errorf("catalogue has %d entries, expected %d", len(catalogue), len(sampleValues))
	}
	for _, e := range catalogue {
		if _, ok := sampleValues[e.Token]; !ok {
			t.Errorf("token %q in catalogue has no sample value in test map", e.Token)
		}
	}
}

func TestExpand_KnownTokensExpand(t *testing.T) {
	for _, e := range PatternCatalogue() {
		t.Run(e.Token, func(t *testing.T) {
			expanded := Expand(e.Token)
			if expanded == e.Token {
				t.Errorf("Expand(%q) returned the token unchanged — expected pattern", e.Token)
			}
			// Must be a valid regex
			if _, err := regexp.Compile(expanded); err != nil {
				t.Errorf("Expand(%q) returned invalid regex: %v", e.Token, err)
			}
		})
	}
}

func TestExpand_SampleValuesMatch(t *testing.T) {
	for token, sample := range sampleValues {
		t.Run(token, func(t *testing.T) {
			pattern := Expand(token)
			re, err := regexp.Compile(pattern)
			if err != nil {
				t.Fatalf("token %q pattern did not compile: %v", token, err)
			}
			if !re.MatchString(sample) {
				t.Errorf("token %q pattern %q did not match sample %q", token, pattern, sample)
			}
		})
	}
}

func TestExpand_CaseInsensitiveLookup(t *testing.T) {
	variants := []string{"UUID", "Uuid", "uuid", "uUiD"}
	for _, v := range variants {
		t.Run(v, func(t *testing.T) {
			expanded := Expand(v)
			if expanded == v {
				t.Errorf("Expand(%q) should expand the uuid token regardless of case", v)
			}
		})
	}
}

func TestExpand_RawRegexPassthrough(t *testing.T) {
	raws := []string{
		`^[0-9]+$`,
		`.*foo.*`,
		`(?i)hello`,
		`[invalid`, // stays as-is (invalid but not our concern here)
	}
	for _, raw := range raws {
		t.Run(raw, func(t *testing.T) {
			if got := Expand(raw); got != raw {
				t.Errorf("Expand(%q) = %q; want passthrough", raw, got)
			}
		})
	}
}

func TestIsToken(t *testing.T) {
	if !IsToken("uuid") {
		t.Error("IsToken(uuid) should be true")
	}
	if !IsToken("UUID") {
		t.Error("IsToken(UUID) should be true (case-insensitive)")
	}
	if IsToken("not-a-token") {
		t.Error("IsToken(not-a-token) should be false")
	}
	if IsToken("^[0-9]+$") {
		t.Error("IsToken(^[0-9]+$) should be false")
	}
}

func TestPatternCatalogue_FieldsPopulated(t *testing.T) {
	for _, e := range PatternCatalogue() {
		if e.Token == "" {
			t.Error("catalogue entry has empty token")
		}
		if e.Description == "" {
			t.Errorf("token %q has empty description", e.Token)
		}
		if e.Pattern == "" {
			t.Errorf("token %q has empty pattern", e.Token)
		}
		if strings.ToLower(e.Token) != e.Token {
			t.Errorf("token %q is not lowercase", e.Token)
		}
	}
}

func TestPatternCatalogue_ReturnsACopy(t *testing.T) {
	c1 := PatternCatalogue()
	c2 := PatternCatalogue()
	c1[0].Token = "mutated"
	if c2[0].Token == "mutated" {
		t.Error("PatternCatalogue() should return a copy, not a shared slice")
	}
}
