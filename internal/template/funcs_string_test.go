//go:build unit

package template

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestStringFuncs_Upper(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"name": "hello"}}
	result, err := e.RenderBodyTemplate(`{{upper (path "name")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "HELLO" {
		t.Errorf("expected HELLO, got %q", result)
	}
}

func TestStringFuncs_Lower(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"name": "WORLD"}}
	result, err := e.RenderBodyTemplate(`{{lower (path "name")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "world" {
		t.Errorf("expected world, got %q", result)
	}
}

func TestStringFuncs_Trim(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"v": "  hello  "}}
	result, err := e.RenderBodyTemplate(`{{trim (path "v")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Errorf("expected hello, got %q", result)
	}
}

func TestStringFuncs_TrimPrefix(t *testing.T) {
	e := NewEngine()
	ctx := &Context{Headers: map[string][]string{"authorization": {"Bearer token123"}}}
	result, err := e.RenderBodyTemplate(`{{trimPrefix "Bearer " (header "authorization")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "token123" {
		t.Errorf("expected token123, got %q", result)
	}
}

func TestStringFuncs_TrimSuffix(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"f": "file.json"}}
	result, err := e.RenderBodyTemplate(`{{trimSuffix ".json" (path "f")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "file" {
		t.Errorf("expected file, got %q", result)
	}
}

func TestStringFuncs_Replace(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"id": "hello-world"}}
	result, err := e.RenderBodyTemplate(`{{replace "-" "_" (path "id")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello_world" {
		t.Errorf("expected hello_world, got %q", result)
	}
}

func TestStringFuncs_Truncate(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"desc": "hello world"}}
	result, err := e.RenderBodyTemplate(`{{truncate 5 (path "desc")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Errorf("expected hello, got %q", result)
	}
}

func TestStringFuncs_Default(t *testing.T) {
	e := NewEngine()
	ctx := &Context{QueryParams: map[string][]string{}}
	result, err := e.RenderBodyTemplate(`{{default "guest" (query "name")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "guest" {
		t.Errorf("expected guest, got %q", result)
	}
}

func TestStringFuncs_DefaultNonEmpty(t *testing.T) {
	e := NewEngine()
	ctx := &Context{QueryParams: map[string][]string{"name": {"alice"}}}
	result, err := e.RenderBodyTemplate(`{{default "guest" (query "name")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "alice" {
		t.Errorf("expected alice, got %q", result)
	}
}

func TestStringFuncs_Coalesce(t *testing.T) {
	e := NewEngine()
	ctx := &Context{QueryParams: map[string][]string{"b": {"found"}}}
	result, err := e.RenderBodyTemplate(`{{coalesce (query "a") (query "b") "fallback"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "found" {
		t.Errorf("expected found, got %q", result)
	}
}

func TestStringFuncs_Contains(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"roles": "admin,user"}}
	result, err := e.RenderBodyTemplate(`{{if contains "admin" (path "roles")}}yes{{else}}no{{end}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "yes" {
		t.Errorf("expected yes, got %q", result)
	}
}

func TestStringFuncs_HasPrefix(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"v": "Bearer xyz"}}
	result, err := e.RenderBodyTemplate(`{{if hasPrefix "Bearer" (path "v")}}yes{{else}}no{{end}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "yes" {
		t.Errorf("expected yes, got %q", result)
	}
}

func TestStringFuncs_HasSuffix(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"f": "photo.jpg"}}
	result, err := e.RenderBodyTemplate(`{{if hasSuffix ".jpg" (path "f")}}yes{{else}}no{{end}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "yes" {
		t.Errorf("expected yes, got %q", result)
	}
}

func TestStringFuncs_SplitJoin(t *testing.T) {
	e := NewEngine()
	ctx := &Context{QueryParams: map[string][]string{"tags": {"a,b,c"}}}
	result, err := e.RenderBodyTemplate(`{{join "|" (split "," (query "tags"))}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "a|b|c" {
		t.Errorf("expected a|b|c, got %q", result)
	}
}

func TestStringFuncs_B64EncDec(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"v": "hello"}}
	encoded := base64.StdEncoding.EncodeToString([]byte("hello"))
	result, err := e.RenderBodyTemplate(`{{b64enc (path "v")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != encoded {
		t.Errorf("expected %q, got %q", encoded, result)
	}

	ctx2 := &Context{PathParams: map[string]string{"v": encoded}}
	result2, err := e.RenderBodyTemplate(`{{b64dec (path "v")}}`, ctx2)
	if err != nil {
		t.Fatal(err)
	}
	if result2 != "hello" {
		t.Errorf("expected hello, got %q", result2)
	}
}

func TestStringFuncs_B64DecInvalid(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"v": "not-valid-base64!!!"}}
	result, err := e.RenderBodyTemplate(`{{b64dec (path "v")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestStringFuncs_URLEncDec(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"v": "hello world"}}
	result, err := e.RenderBodyTemplate(`{{urlEnc (path "v")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello+world" {
		t.Errorf("expected hello+world, got %q", result)
	}

	ctx2 := &Context{PathParams: map[string]string{"v": "hello+world"}}
	result2, err := e.RenderBodyTemplate(`{{urlDec (path "v")}}`, ctx2)
	if err != nil {
		t.Fatal(err)
	}
	if result2 != "hello world" {
		t.Errorf("expected 'hello world', got %q", result2)
	}
}

func TestStringFuncs_URLDecInvalid(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"v": "hello%ZZworld"}}
	result, err := e.RenderBodyTemplate(`{{urlDec (path "v")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	// returns original on error
	if result != "hello%ZZworld" {
		t.Errorf("expected original, got %q", result)
	}
}

func TestStringFuncs_MD5(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"v": "hello"}}
	result, err := e.RenderBodyTemplate(`{{md5 (path "v")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 32 {
		t.Errorf("expected 32-char MD5, got %d chars: %q", len(result), result)
	}
}

func TestStringFuncs_SHA256(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"v": "hello"}}
	result, err := e.RenderBodyTemplate(`{{sha256 (path "v")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 64 {
		t.Errorf("expected 64-char SHA256, got %d chars: %q", len(result), result)
	}
}

func TestStringFuncs_Repeat(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{repeat 3 "ab"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "ababab" {
		t.Errorf("expected ababab, got %q", result)
	}
}

func TestStringFuncs_Printf(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"n": "42"}}
	result, err := e.RenderBodyTemplate(`{{printf "ID-%05d" (toInt (path "n"))}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "ID-00042" {
		t.Errorf("expected ID-00042, got %q", result)
	}
}

func TestStringFuncs_TruncateOverLen(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"v": "hi"}}
	result, err := e.RenderBodyTemplate(`{{truncate 10 (path "v")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hi" {
		t.Errorf("expected hi, got %q", result)
	}
}

func TestStringFuncs_TruncateNegative(t *testing.T) {
	e := NewEngine()
	ctx := &Context{PathParams: map[string]string{"v": "hello"}}
	result, err := e.RenderBodyTemplate(`{{truncate -1 (path "v")}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Errorf("expected hello, got %q", result)
	}
}

func TestStringFuncs_Indent(t *testing.T) {
	fm := buildStringFuncMap()
	indent := fm["indent"].(func(int, string) string)
	result := indent(2, "hello")
	if !strings.HasPrefix(result, "  hello") {
		t.Errorf("expected indented, got %q", result)
	}
}
