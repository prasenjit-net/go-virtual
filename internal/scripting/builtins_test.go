package scripting

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/prasenjit/go-virtual/internal/store"
)

// helpers ─────────────────────────────────────────────────────────────────────

func mustCompileAndRun(t *testing.T, id, src string, sess *store.Session) (any, error) {
	t.Helper()
	runner := &StarlarkRunner{}
	cs, err := runner.Compile(id, src)
	if err != nil {
		t.Fatalf("Compile %s: %v", id, err)
	}
	return cs.Execute(context.Background(), &ScriptInput{}, 500, sess, nil, nil, nil)
}

func mustCompile(t *testing.T, id, src string) (any, error) {
	return mustCompileAndRun(t, id, src, nil)
}

// ── uuid() ────────────────────────────────────────────────────────────────────

func TestBuiltin_UUID(t *testing.T) {
	src := `
def run(req):
    u = uuid()
    return u
`
	v, err := mustCompile(t, "uuid1", src)
	if err != nil {
		t.Fatalf("uuid: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("uuid: expected string, got %T", v)
	}
	// Quick UUID v4 sanity check
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		t.Errorf("uuid: bad format %q", s)
	}
}

func TestBuiltin_UUID_TwoCalls(t *testing.T) {
	src := `
def run(req):
    return uuid() != uuid()
`
	v, err := mustCompile(t, "uuid2", src)
	if err != nil {
		t.Fatalf("uuid uniqueness: %v", err)
	}
	if v != true {
		t.Error("Expected two uuid() calls to produce different values")
	}
}

// ── now() ─────────────────────────────────────────────────────────────────────

func TestBuiltin_Now_Unix(t *testing.T) {
	src := `
def run(req):
    return now()
`
	v, err := mustCompile(t, "now1", src)
	if err != nil {
		t.Fatalf("now: %v", err)
	}
	ts, ok := v.(int64)
	if !ok {
		t.Fatalf("now unix: expected int64, got %T (%v)", v, v)
	}
	if ts <= 0 {
		t.Errorf("now unix: got non-positive value %d", ts)
	}
}

func TestBuiltin_Now_UnixMs(t *testing.T) {
	src := `
def run(req):
    return now("unix_ms")
`
	v, err := mustCompile(t, "now2", src)
	if err != nil {
		t.Fatalf("now unix_ms: %v", err)
	}
	ts, ok := v.(int64)
	if !ok {
		t.Fatalf("now unix_ms: expected int64, got %T", v)
	}
	if ts <= 0 {
		t.Errorf("now unix_ms: non-positive %d", ts)
	}
}

func TestBuiltin_Now_ISO(t *testing.T) {
	src := `
def run(req):
    return now("iso")
`
	v, err := mustCompile(t, "now3", src)
	if err != nil {
		t.Fatalf("now iso: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("now iso: expected string, got %T", v)
	}
	if !strings.Contains(s, "T") {
		t.Errorf("now iso: not an ISO timestamp: %q", s)
	}
}

func TestBuiltin_Now_Date(t *testing.T) {
	src := `
def run(req):
    return now("date")
`
	v, err := mustCompile(t, "now4", src)
	if err != nil {
		t.Fatalf("now date: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("now date: expected string, got %T", v)
	}
	parts := strings.Split(s, "-")
	if len(parts) != 3 {
		t.Errorf("now date: unexpected format %q", s)
	}
}

func TestBuiltin_Now_UnknownFormat(t *testing.T) {
	src := `
def run(req):
    return now("badformat")
`
	_, err := mustCompile(t, "now5", src)
	if err == nil {
		t.Error("now: expected error for unknown format, got nil")
	}
}

// ── rand_int() ────────────────────────────────────────────────────────────────

func TestBuiltin_RandInt_OneArg(t *testing.T) {
	src := `
def run(req):
    v = rand_int(10)
    return v >= 0 and v <= 10
`
	v, err := mustCompile(t, "ri1", src)
	if err != nil {
		t.Fatalf("rand_int one-arg: %v", err)
	}
	if v != true {
		t.Errorf("rand_int one-arg: value out of range")
	}
}

func TestBuiltin_RandInt_TwoArgs(t *testing.T) {
	src := `
def run(req):
    v = rand_int(5, 10)
    return v >= 5 and v <= 10
`
	v, err := mustCompile(t, "ri2", src)
	if err != nil {
		t.Fatalf("rand_int two-arg: %v", err)
	}
	if v != true {
		t.Errorf("rand_int two-arg: value out of range")
	}
}

func TestBuiltin_RandInt_MinGtMax(t *testing.T) {
	src := `
def run(req):
    return rand_int(10, 1)
`
	_, err := mustCompile(t, "ri3", src)
	if err == nil {
		t.Error("rand_int: expected error when min > max, got nil")
	}
}

// ── rand_choice() ─────────────────────────────────────────────────────────────

func TestBuiltin_RandChoice(t *testing.T) {
	src := `
def run(req):
    items = ["a", "b", "c"]
    choice = rand_choice(items)
    return choice in items
`
	v, err := mustCompile(t, "rc1", src)
	if err != nil {
		t.Fatalf("rand_choice: %v", err)
	}
	if v != true {
		t.Error("rand_choice: returned element not in list")
	}
}

func TestBuiltin_RandChoice_EmptyList(t *testing.T) {
	src := `
def run(req):
    return rand_choice([])
`
	_, err := mustCompile(t, "rc2", src)
	if err == nil {
		t.Error("rand_choice: expected error for empty list, got nil")
	}
}

// ── counter() ─────────────────────────────────────────────────────────────────

func TestBuiltin_Counter_DefaultDelta(t *testing.T) {
	src := `
def run(req):
    counter("hits")
    counter("hits")
    return counter("hits")
`
	sess := store.NewEphemeralSession(nil)
	v, err := mustCompileAndRun(t, "ctr1", src, sess)
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	n, ok := v.(int64)
	if !ok {
		t.Fatalf("counter: expected int64, got %T", v)
	}
	if n != 3 {
		t.Errorf("counter: expected 3, got %d", n)
	}
}

func TestBuiltin_Counter_CustomDelta(t *testing.T) {
	src := `
def run(req):
    return counter("score", 10)
`
	sess := store.NewEphemeralSession(nil)
	v, err := mustCompileAndRun(t, "ctr2", src, sess)
	if err != nil {
		t.Fatalf("counter custom delta: %v", err)
	}
	n, ok := v.(int64)
	if !ok {
		t.Fatalf("counter custom delta: expected int64, got %T", v)
	}
	if n != 10 {
		t.Errorf("counter custom delta: expected 10, got %d", n)
	}
}

func TestBuiltin_Counter_Peek(t *testing.T) {
	src := `
def run(req):
    counter("x", 5)
    return counter("x", 0)
`
	sess := store.NewEphemeralSession(nil)
	v, err := mustCompileAndRun(t, "ctr3", src, sess)
	if err != nil {
		t.Fatalf("counter peek: %v", err)
	}
	if v.(int64) != 5 {
		t.Errorf("counter peek: expected 5, got %v", v)
	}
}

func TestBuiltin_Counter_NoSession(t *testing.T) {
	src := `
def run(req):
    counter("c")
    counter("c")
    return counter("c")
`
	v, err := mustCompile(t, "ctr4", src)
	if err != nil {
		t.Fatalf("counter no-sess: %v", err)
	}
	if v.(int64) != 3 {
		t.Errorf("counter no-sess: expected 3, got %v", v)
	}
}

// ── base64 ────────────────────────────────────────────────────────────────────

func TestBuiltin_Base64RoundTrip(t *testing.T) {
	input := "Hello, Starlark!"
	src := `
def run(req):
    enc = base64_encode(req.body())
    dec = base64_decode(enc)
    return dec
`
	runner := &StarlarkRunner{}
	cs, err := runner.Compile("b64", src)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	v, err := cs.Execute(context.Background(), &ScriptInput{Body: input}, 200, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if v != input {
		t.Errorf("base64 round-trip: got %q, want %q", v, input)
	}
}

func TestBuiltin_Base64Encode_KnownValue(t *testing.T) {
	src := `
def run(req):
    return base64_encode("hello")
`
	v, err := mustCompile(t, "b64e", src)
	if err != nil {
		t.Fatalf("base64_encode: %v", err)
	}
	want := base64.StdEncoding.EncodeToString([]byte("hello"))
	if v != want {
		t.Errorf("base64_encode: got %q, want %q", v, want)
	}
}

func TestBuiltin_Base64Decode_Invalid(t *testing.T) {
	src := `
def run(req):
    return base64_decode("!!!notbase64!!!")
`
	_, err := mustCompile(t, "b64d", src)
	if err == nil {
		t.Error("base64_decode: expected error for invalid input, got nil")
	}
}

// ── hash() ────────────────────────────────────────────────────────────────────

func TestBuiltin_Hash_SHA256(t *testing.T) {
	src := `
def run(req):
    return hash("sha256", "hello")
`
	v, err := mustCompile(t, "h1", src)
	if err != nil {
		t.Fatalf("hash sha256: %v", err)
	}
	// Well-known SHA-256 of "hello"
	const want = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if v != want {
		t.Errorf("hash sha256: got %q, want %q", v, want)
	}
}

func TestBuiltin_Hash_MD5(t *testing.T) {
	src := `
def run(req):
    return hash("md5", "hello")
`
	v, err := mustCompile(t, "h2", src)
	if err != nil {
		t.Fatalf("hash md5: %v", err)
	}
	const want = "5d41402abc4b2a76b9719d911017c592"
	if v != want {
		t.Errorf("hash md5: got %q, want %q", v, want)
	}
}

func TestBuiltin_Hash_UnknownAlgo(t *testing.T) {
	src := `
def run(req):
    return hash("md6", "hello")
`
	_, err := mustCompile(t, "h3", src)
	if err == nil {
		t.Error("hash: expected error for unknown algo")
	}
}

// ── sleep() ───────────────────────────────────────────────────────────────────

func TestBuiltin_Sleep(t *testing.T) {
	src := `
def run(req):
    sleep(5)
    return "ok"
`
	v, err := mustCompile(t, "sl1", src)
	if err != nil {
		t.Fatalf("sleep: %v", err)
	}
	if v != "ok" {
		t.Errorf("sleep: expected \"ok\", got %v", v)
	}
}

func TestBuiltin_Sleep_Negative(t *testing.T) {
	src := `
def run(req):
    sleep(-1)
    return "ok"
`
	v, err := mustCompile(t, "sl2", src)
	if err != nil {
		t.Fatalf("sleep negative: %v", err)
	}
	if v != "ok" {
		t.Errorf("sleep negative: expected ok, got %v", v)
	}
}

// ── json_parse / json_stringify ───────────────────────────────────────────────

func TestBuiltin_JSONRoundTrip(t *testing.T) {
	src := `
def run(req):
    obj = json_parse('{"x":1,"y":true}')
    return json_stringify(obj)
`
	v, err := mustCompile(t, "json1", src)
	if err != nil {
		t.Fatalf("json round-trip: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("json round-trip: expected string, got %T", v)
	}
	if !strings.Contains(s, `"x"`) || !strings.Contains(s, `"y"`) {
		t.Errorf("json round-trip: unexpected output %q", s)
	}
}

func TestBuiltin_JSONParse_Invalid(t *testing.T) {
	src := `
def run(req):
    return json_parse("{bad json")
`
	_, err := mustCompile(t, "json2", src)
	if err == nil {
		t.Error("json_parse: expected error for invalid JSON, got nil")
	}
}

// ── regex ─────────────────────────────────────────────────────────────────────

func TestBuiltin_RegexMatch_True(t *testing.T) {
	src := `
def run(req):
    return regex_match(r"^\d+$", "12345")
`
	v, err := mustCompile(t, "rx1", src)
	if err != nil {
		t.Fatalf("regex_match true: %v", err)
	}
	if v != true {
		t.Error("regex_match: expected True for matching pattern")
	}
}

func TestBuiltin_RegexMatch_False(t *testing.T) {
	src := `
def run(req):
    return regex_match(r"^\d+$", "abc")
`
	v, err := mustCompile(t, "rx2", src)
	if err != nil {
		t.Fatalf("regex_match false: %v", err)
	}
	if v != false {
		t.Error("regex_match: expected False for non-matching pattern")
	}
}

func TestBuiltin_RegexFind(t *testing.T) {
	src := `
def run(req):
    return regex_find(r"\d+", "abc123def456")
`
	v, err := mustCompile(t, "rx3", src)
	if err != nil {
		t.Fatalf("regex_find: %v", err)
	}
	if v != "123" {
		t.Errorf("regex_find: expected \"123\", got %v", v)
	}
}

func TestBuiltin_RegexFind_NoMatch(t *testing.T) {
	src := `
def run(req):
    return regex_find(r"\d+", "nope") == None
`
	v, err := mustCompile(t, "rx4", src)
	if err != nil {
		t.Fatalf("regex_find no-match: %v", err)
	}
	if v != true {
		t.Error("regex_find: expected None for no match")
	}
}

func TestBuiltin_RegexFindAll(t *testing.T) {
	src := `
def run(req):
    return regex_find_all(r"\d+", "a1b2c3")
`
	v, err := mustCompile(t, "rx5", src)
	if err != nil {
		t.Fatalf("regex_find_all: %v", err)
	}
	lst, ok := v.([]any)
	if !ok {
		t.Fatalf("regex_find_all: expected []any, got %T", v)
	}
	if len(lst) != 3 {
		t.Errorf("regex_find_all: expected 3 matches, got %d", len(lst))
	}
}

func TestBuiltin_RegexFindAll_NoMatch(t *testing.T) {
	src := `
def run(req):
    return len(regex_find_all(r"\d+", "nope"))
`
	v, err := mustCompile(t, "rx6", src)
	if err != nil {
		t.Fatalf("regex_find_all no-match: %v", err)
	}
	n, ok := v.(int64)
	if !ok {
		t.Fatalf("regex_find_all no-match: expected int64, got %T", v)
	}
	if n != 0 {
		t.Errorf("regex_find_all no-match: expected 0, got %d", n)
	}
}

func TestBuiltin_Regex_BadPattern(t *testing.T) {
	src := `
def run(req):
    return regex_match("[unclosed", "test")
`
	_, err := mustCompile(t, "rx7", src)
	if err == nil {
		t.Error("regex_match: expected error for invalid pattern, got nil")
	}
}
