package template

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestRenderBodyTemplate_LegacyTokensAndHelpers(t *testing.T) {
	e := NewEngine()
	ctx := &Context{
		PathParams:  map[string]string{"id": "123"},
		QueryParams: map[string][]string{"status": {"active"}},
		Headers:     map[string][]string{"Authorization": {"Bearer token"}},
		Body:        `{"user":{"name":"Ada"}}`,
		RNG:         rand.New(rand.NewSource(1)),
	}

	input := `id={{path.id}};status={{query.STATUS}};auth={{header.authorization}};name={{body.user.name}};raw={{body ""}};uuid={{random.uuid}};ts={{timestamp.unix}}`

	result, err := e.RenderBodyTemplate(input, ctx)
	if err != nil {
		t.Fatalf("RenderBodyTemplate error: %v", err)
	}

	payload := make(map[string]string)
	for _, part := range strings.Split(result, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			payload[kv[0]] = kv[1]
		}
	}

	if payload["id"] != "123" {
		t.Fatalf("expected id to be 123, got %q", payload["id"])
	}
	if payload["status"] != "active" {
		t.Fatalf("expected status to be active, got %q", payload["status"])
	}
	if payload["auth"] != "Bearer token" {
		t.Fatalf("expected auth header to match, got %q", payload["auth"])
	}
	if payload["name"] != "Ada" {
		t.Fatalf("expected name to be Ada, got %q", payload["name"])
	}
	if payload["raw"] != `{"user":{"name":"Ada"}}` {
		t.Fatalf("expected raw body to be preserved, got %q", payload["raw"])
	}

	uuidPattern := regexp.MustCompile(`^[a-f0-9-]{36}$`)
	if !uuidPattern.MatchString(payload["uuid"]) {
		t.Fatalf("expected uuid to match pattern, got %q", payload["uuid"])
	}

	if payload["ts"] == "" || !regexp.MustCompile(`^\d+$`).MatchString(payload["ts"]) {
		t.Fatalf("expected unix timestamp digits, got %q", payload["ts"])
	}
}

func TestValidateBodyTemplate_Invalid(t *testing.T) {
	e := NewEngine()

	if err := e.ValidateBodyTemplate("{{if}}"); err == nil {
		t.Fatal("expected validation error for invalid template")
	}

	if err := e.ValidateBodyTemplate("Hello {{path \"id\"}}"); err != nil {
		t.Fatalf("expected valid template, got error: %v", err)
	}
}

func TestPreprocessLegacyBodyTemplate(t *testing.T) {
	input := "Hello {{path.id}} {{.query.status}} {{header.Authorization}} {{random.uuid}} {{timestamp.iso}} {{if .Path}}{{end}}"
	expected := "Hello {{path \"id\"}} {{query \"status\"}} {{header \"Authorization\"}} {{random \"uuid\"}} {{timestamp \"iso\"}} {{if .Path}}{{end}}"

	if got := preprocessLegacyBodyTemplate(input); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestNormalizeAndBuildHelpers(t *testing.T) {
	normalized := normalizeFirstValues(map[string][]string{
		"X-Test": {"a", "b"},
		"Empty":  {},
	})

	if normalized["x-test"] != "a" {
		t.Fatalf("expected normalized x-test to be a, got %q", normalized["x-test"])
	}
	if _, ok := normalized["empty"]; ok {
		t.Fatal("expected empty key to be omitted")
	}

	if got := buildKeyFromArgs([]interface{}{"int", 1, 5}); got != "int(1,5)" {
		t.Fatalf("expected int(1,5), got %q", got)
	}
	if got := buildKeyFromArgs([]interface{}{"uuid"}); got != "uuid" {
		t.Fatalf("expected uuid, got %q", got)
	}
	if got := buildKeyFromArgs(nil); got != "" {
		t.Fatalf("expected empty key, got %q", got)
	}

	if got := buildDotKeyFromArgs([]interface{}{"name", "first"}); got != "name.first" {
		t.Fatalf("expected name.first, got %q", got)
	}
	if got := buildDotKeyFromArgs([]interface{}{"username"}); got != "username" {
		t.Fatalf("expected username, got %q", got)
	}
	if got := buildDotKeyFromArgs(nil); got != "" {
		t.Fatalf("expected empty dot key, got %q", got)
	}
}

func TestResolveTimestampAndRandom(t *testing.T) {
	e := NewEngine()

	formatted := e.resolveTimestamp("format(2006-01-02)")
	if _, err := time.Parse("2006-01-02", formatted); err != nil {
		t.Fatalf("expected formatted date, got %q", formatted)
	}

	added := e.resolveTimestamp("add(1h)")
	if _, err := time.Parse(time.RFC3339, added); err != nil {
		t.Fatalf("expected RFC3339 timestamp, got %q", added)
	}

	rng := rand.New(rand.NewSource(1))
	if got := e.resolveRandom("string(5)", rng); len(got) != 5 {
		t.Fatalf("expected string length 5, got %q", got)
	}

	rng = rand.New(rand.NewSource(2))
	intVal := e.resolveRandom("int(5,10)", rng)
	if !regexp.MustCompile(`^\d+$`).MatchString(intVal) {
		t.Fatalf("expected int value, got %q", intVal)
	}

	rng = rand.New(rand.NewSource(3))
	floatVal := e.resolveRandom("float(1,2)", rng)
	if !regexp.MustCompile(`^\d+\.\d{2}$`).MatchString(floatVal) {
		t.Fatalf("expected float value, got %q", floatVal)
	}

	rng = rand.New(rand.NewSource(4))
	boolVal := e.resolveRandom("bool", rng)
	if boolVal != "true" && boolVal != "false" {
		t.Fatalf("expected bool value, got %q", boolVal)
	}

	rng = rand.New(rand.NewSource(5))
	emailVal := e.resolveRandom("email", rng)
	if !strings.HasSuffix(emailVal, "@example.com") {
		t.Fatalf("expected example.com email, got %q", emailVal)
	}
}

func TestResolveFaker(t *testing.T) {
	e := NewEngine()

	rng := rand.New(rand.NewSource(1))
	firstName := e.resolveFaker("name.first", rng)
	allowedFirst := map[string]struct{}{"Liam": {}, "Noah": {}, "Olivia": {}, "Emma": {}, "Ava": {}, "Sophia": {}, "Mason": {}, "Logan": {}, "Mia": {}, "Lucas": {}}
	if _, ok := allowedFirst[firstName]; !ok {
		t.Fatalf("unexpected first name %q", firstName)
	}

	rng = rand.New(rand.NewSource(2))
	email := e.resolveFaker("email", rng)
	if !strings.Contains(email, "@") {
		t.Fatalf("expected email to contain @, got %q", email)
	}

	rng = rand.New(rand.NewSource(3))
	company := e.resolveFaker("company.name", rng)
	if company == "" {
		t.Fatal("expected company name to be populated")
	}

	rng = rand.New(rand.NewSource(4))
	url := e.resolveFaker("internet.url", rng)
	if !strings.HasPrefix(url, "https://") {
		t.Fatalf("expected url to start with https://, got %q", url)
	}

	rng = rand.New(rand.NewSource(5))
	word := e.resolveFaker("lorem.word", rng)
	if word == "" {
		t.Fatal("expected lorem word to be populated")
	}
}
