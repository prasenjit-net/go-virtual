package scripting

import (
	"testing"

	"go.starlark.net/starlark"
)

// ─── validate module tests ────────────────────────────────────────────────────

func TestValidate_Matches_Token(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.matches("550e8400-e29b-41d4-a716-446655440000", "uuid")
`)
	if !mustBool(t, result) {
		t.Error("valid UUID should match 'uuid' token")
	}
}

func TestValidate_Matches_Token_Fail(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.matches("not-a-uuid", "uuid")
`)
	if mustBool(t, result) {
		t.Error("non-UUID should not match 'uuid' token")
	}
}

func TestValidate_Matches_RawRegex(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.matches("hello123", "^[a-z]+[0-9]+$")
`)
	if !mustBool(t, result) {
		t.Error("should match raw regex")
	}
}

func TestValidate_Regex(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.regex("abc123", "[a-z]+[0-9]+")
`)
	if !mustBool(t, result) {
		t.Error("regex match failed")
	}
}

func TestValidate_PatternNames(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.pattern_names()
`)
	lst, ok := result.(*starlark.List)
	if !ok {
		t.Fatalf("expected list, got %T", result)
	}
	if lst.Len() == 0 {
		t.Error("pattern_names() should return non-empty list")
	}
	// Check that "uuid" is in the list.
	found := false
	for i := range lst.Len() {
		if s, ok := lst.Index(i).(starlark.String); ok && string(s) == "uuid" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'uuid' in pattern_names()")
	}
}

func TestValidate_IsUUID(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.is_uuid("550e8400-e29b-41d4-a716-446655440000")
`)
	if !mustBool(t, result) {
		t.Error("is_uuid should return True for valid UUID")
	}
}

func TestValidate_IsEmail(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.is_email("user@example.com")
`)
	if !mustBool(t, result) {
		t.Error("is_email should return True for valid email")
	}
}

func TestValidate_IsEmail_Fail(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.is_email("not-an-email")
`)
	if mustBool(t, result) {
		t.Error("is_email should return False for non-email")
	}
}

func TestValidate_IsURL(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.is_url("https://example.com/path?q=1")
`)
	if !mustBool(t, result) {
		t.Error("is_url should match https URL")
	}
}

func TestValidate_IsSemver(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.is_semver("1.2.3-alpha.1+build.42")
`)
	if !mustBool(t, result) {
		t.Error("is_semver should match valid semver")
	}
}

func TestValidate_IsJWT(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.is_jwt("eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U")
`)
	if !mustBool(t, result) {
		t.Error("is_jwt should match valid JWT")
	}
}

func TestValidate_IsUSPhone(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.is_us_phone("(555) 123-4567")
`)
	if !mustBool(t, result) {
		t.Error("is_us_phone should match US phone format")
	}
}

func TestValidate_IsSSN(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.is_ssn("123-45-6789")
`)
	if !mustBool(t, result) {
		t.Error("is_ssn should match SSN format")
	}
}

func TestValidate_IsIPv4(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.is_ipv4("192.168.1.1")
`)
	if !mustBool(t, result) {
		t.Error("is_ipv4 should match valid IPv4")
	}
}

func TestValidate_IsDateISO(t *testing.T) {
	result := execScript(t, `
def run(req):
    return validate.is_date_iso("2025-01-15")
`)
	if !mustBool(t, result) {
		t.Error("is_date_iso should match YYYY-MM-DD")
	}
}

func TestValidate_CombineWithDatetime(t *testing.T) {
	// Integration: use validate + datetime together.
	result := execScript(t, `
def run(req):
    date_str = "2025-06-15"
    if not validate.is_date_iso(date_str):
        return {"error": "invalid date"}
    d = datetime.date.fromisoformat(date_str)
    return {"year": d.year, "valid": True}
`)
	d, ok := result.(*starlark.Dict)
	if !ok {
		t.Fatalf("expected dict, got %T", result)
	}
	valid, _, _ := d.Get(starlark.String("valid"))
	if !mustBool(t, valid) {
		t.Error("combined validate+datetime failed")
	}
}
