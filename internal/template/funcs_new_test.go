//go:build unit

package template

import (
	"strings"
	"testing"
)

func TestNewRandomTypes_UUID4(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{random "uuid4"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 36 {
		t.Errorf("expected UUID (36 chars), got %q", result)
	}
}

func TestNewRandomTypes_Alpha(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	r1, _ := e.RenderBodyTemplate(`{{random "alpha"}}`, ctx)
	if len(r1) != 10 {
		t.Errorf("expected 10-char alpha, got %q", r1)
	}
	if r1 != strings.ToLower(r1) {
		t.Errorf("expected lowercase, got %q", r1)
	}

	r2, _ := e.RenderBodyTemplate(`{{random "alpha(5)"}}`, ctx)
	if len(r2) != 5 {
		t.Errorf("expected 5-char alpha, got %q", r2)
	}
}

func TestNewRandomTypes_AlphaUpper(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	r1, _ := e.RenderBodyTemplate(`{{random "ALPHA"}}`, ctx)
	if len(r1) != 10 {
		t.Errorf("expected 10-char ALPHA, got %q", r1)
	}
	if r1 != strings.ToUpper(r1) {
		t.Errorf("expected uppercase, got %q", r1)
	}

	r2, _ := e.RenderBodyTemplate(`{{random "ALPHA(6)"}}`, ctx)
	if len(r2) != 6 {
		t.Errorf("expected 6-char ALPHA, got %q", r2)
	}
}

func TestNewRandomTypes_Numeric(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	r1, _ := e.RenderBodyTemplate(`{{random "numeric"}}`, ctx)
	if len(r1) != 6 {
		t.Errorf("expected 6 digits, got %q", r1)
	}
	for _, c := range r1 {
		if c < '0' || c > '9' {
			t.Errorf("expected only digits in %q", r1)
			break
		}
	}

	r2, _ := e.RenderBodyTemplate(`{{random "numeric(4)"}}`, ctx)
	if len(r2) != 4 {
		t.Errorf("expected 4 digits, got %q", r2)
	}
}

func TestNewRandomTypes_Hex(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	r1, _ := e.RenderBodyTemplate(`{{random "hex"}}`, ctx)
	if len(r1) != 8 {
		t.Errorf("expected 8-char hex, got %q", r1)
	}
	for _, c := range r1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("expected hex chars in %q", r1)
			break
		}
	}

	r2, _ := e.RenderBodyTemplate(`{{random "hex(12)"}}`, ctx)
	if len(r2) != 12 {
		t.Errorf("expected 12-char hex, got %q", r2)
	}
}

func TestNewRandomTypes_Alphanumeric(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	r1, _ := e.RenderBodyTemplate(`{{random "alphanumeric"}}`, ctx)
	if len(r1) != 10 {
		t.Errorf("expected 10-char alphanumeric, got %q", r1)
	}
	r2, _ := e.RenderBodyTemplate(`{{random "alphanumeric(8)"}}`, ctx)
	if len(r2) != 8 {
		t.Errorf("expected 8-char alphanumeric, got %q", r2)
	}
}

func TestNewRandomTypes_AlphaInvalidLen(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	// Invalid length falls back to default
	r, _ := e.RenderBodyTemplate(`{{random "alpha(abc)"}}`, ctx)
	if len(r) != 10 {
		t.Errorf("expected fallback to 10, got %q", r)
	}
}

func TestNewRandomTypes_HexInvalidLen(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	r, _ := e.RenderBodyTemplate(`{{random "hex(bad)"}}`, ctx)
	if len(r) != 8 {
		t.Errorf("expected fallback to 8, got %q", r)
	}
}

func TestNewRandomTypes_NumericInvalidLen(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	r, _ := e.RenderBodyTemplate(`{{random "numeric(x)"}}`, ctx)
	if len(r) != 6 {
		t.Errorf("expected fallback to 6, got %q", r)
	}
}

func TestNewTimestamp_Variants(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	tests := []struct {
		key     string
		minLen  int
		maxLen  int
	}{
		{"unix_ms", 10, 20},
		{"unix_ns", 10, 25},
		{"utc", 20, 30},
		{"year", 4, 4},
		{"month", 2, 2},
		{"day", 2, 2},
	}

	for _, tt := range tests {
		result, err := e.RenderBodyTemplate(`{{timestamp "`+tt.key+`"}}`, ctx)
		if err != nil {
			t.Fatalf("timestamp %s: %v", tt.key, err)
		}
		if len(result) < tt.minLen || len(result) > tt.maxLen {
			t.Errorf("timestamp %s: len=%d out of [%d,%d] result=%q",
				tt.key, len(result), tt.minLen, tt.maxLen, result)
		}
	}
}

func TestNewTimestamp_Sub(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{timestamp "sub(24h)"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) < 20 {
		t.Errorf("expected RFC3339 from sub, got %q", result)
	}
}

func TestNewContext_Method(t *testing.T) {
	e := NewEngine()
	ctx := &Context{Method: "POST"}
	result, err := e.RenderBodyTemplate(`{{.Method}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "POST" {
		t.Errorf("expected POST, got %q", result)
	}
}

func TestNewContext_URL(t *testing.T) {
	e := NewEngine()
	ctx := &Context{RequestURL: "https://example.com/api/v1"}
	result, err := e.RenderBodyTemplate(`{{.URL}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "https://example.com/api/v1" {
		t.Errorf("unexpected URL, got %q", result)
	}
}

func TestNewContext_RequestID(t *testing.T) {
	e := NewEngine()
	ctx := &Context{RequestID: "test-req-id-123"}
	result, err := e.RenderBodyTemplate(`{{.RequestID}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "test-req-id-123" {
		t.Errorf("unexpected RequestID, got %q", result)
	}
}

func TestNewContext_RawBody(t *testing.T) {
	e := NewEngine()
	ctx := &Context{Body: `{"name":"alice"}`}
	result, err := e.RenderBodyTemplate(`{{rawBody}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != `{"name":"alice"}` {
		t.Errorf("unexpected rawBody, got %q", result)
	}
}

func TestNewContext_DotBodyNative(t *testing.T) {
	e := NewEngine()
	ctx := &Context{Body: `{"user":{"name":"alice"}}`}
	result, err := e.RenderBodyTemplate(`{{.Body.user.name}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "alice" {
		t.Errorf("expected alice from .Body.user.name, got %q", result)
	}
}

func TestNewContext_DotBodyTopLevel(t *testing.T) {
	e := NewEngine()
	ctx := &Context{Body: `{"name":"bob"}`}
	result, err := e.RenderBodyTemplate(`{{.Body.name}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "bob" {
		t.Errorf("expected bob, got %q", result)
	}
}

func TestNewContext_DotBodyInvalidJSON(t *testing.T) {
	e := NewEngine()
	ctx := &Context{Body: `not json`}
	result, err := e.RenderBodyTemplate(`{{.RawBody}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "not json" {
		t.Errorf("expected raw body passthrough, got %q", result)
	}
}

func TestNewContext_Store(t *testing.T) {
	e := NewEngine()
	store := map[string]string{"userId": "abc-123"}
	ctx := &Context{
		StoreReader: func(key string) string { return store[key] },
	}
	result, err := e.RenderBodyTemplate(`{{store "userId"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "abc-123" {
		t.Errorf("expected abc-123, got %q", result)
	}
}

func TestNewContext_StoreNil(t *testing.T) {
	e := NewEngine()
	ctx := &Context{} // no StoreReader
	result, err := e.RenderBodyTemplate(`{{store "key"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestNewContext_Counter(t *testing.T) {
	e := NewEngine()
	calls := 0
	ctx := &Context{
		StoreWriter: func(name string) string {
			calls++
			return strings.Repeat("1", calls)
		},
	}
	result, err := e.RenderBodyTemplate(`{{counter "hits"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "1" {
		t.Errorf("expected 1, got %q", result)
	}
}

func TestNewContext_CounterNil(t *testing.T) {
	e := NewEngine()
	ctx := &Context{} // no StoreWriter
	result, err := e.RenderBodyTemplate(`{{counter "hits"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "0" {
		t.Errorf("expected 0, got %q", result)
	}
}

func TestNewContext_ToJSON(t *testing.T) {
	e := NewEngine()
	ctx := &Context{
		ScriptOutput: map[string]any{
			"result": map[string]any{"status": "ok"},
		},
	}
	result, err := e.RenderBodyTemplate(`{{toJSON .Script.result}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"status"`) {
		t.Errorf("expected JSON with status, got %q", result)
	}
}

func TestNewContext_JSONGet(t *testing.T) {
	e := NewEngine()
	ctx := &Context{Body: `{"user":{"name":"carol"}}`}
	result, err := e.RenderBodyTemplate(`{{jsonGet "user.name" .RawBody}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "carol" {
		t.Errorf("expected carol, got %q", result)
	}
}

func TestNewContext_JSONGetMissing(t *testing.T) {
	e := NewEngine()
	ctx := &Context{Body: `{"a":1}`}
	result, err := e.RenderBodyTemplate(`{{jsonGet "b.c" .RawBody}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestProcess_DelegatesNow(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{now | dateFormat "2006"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 4 {
		t.Errorf("expected year (4 chars), got %q", result)
	}
}

func TestProcess_DelegatesDatFmt(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{now | dateFmt "2006"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 4 {
		t.Errorf("expected year (4 chars), got %q", result)
	}
}
