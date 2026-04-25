package proxy

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
)

func TestComputeSignature_NilConfig(t *testing.T) {
	pathParams := map[string]string{"id": "42"}
	queryParams := url.Values{"q": {"hello"}}
	headers := http.Header{"X-Token": {"abc"}}
	body := `{"name":"test"}`

	sig1 := ComputeSignature(pathParams, queryParams, headers, body, nil)
	sig2 := ComputeSignature(pathParams, queryParams, headers, body, nil)

	if sig1 == "" {
		t.Fatal("expected non-empty signature")
	}
	if sig1 != sig2 {
		t.Errorf("expected deterministic signature, got %q vs %q", sig1, sig2)
	}
	if len(sig1) != 16 {
		t.Errorf("expected 16-char hex signature, got %d chars: %q", len(sig1), sig1)
	}
}

func TestComputeSignature_Determinism(t *testing.T) {
	// Different ordering of map keys should produce same signature.
	pathA := map[string]string{"a": "1", "b": "2"}
	pathB := map[string]string{"b": "2", "a": "1"}
	query := url.Values{}
	headers := http.Header{}
	body := ""

	s1 := ComputeSignature(pathA, query, headers, body, nil)
	s2 := ComputeSignature(pathB, query, headers, body, nil)

	if s1 != s2 {
		t.Errorf("signatures should be order-independent: %q vs %q", s1, s2)
	}
}

func TestComputeSignature_DifferentBody(t *testing.T) {
	pathParams := map[string]string{}
	query := url.Values{}
	headers := http.Header{}

	s1 := ComputeSignature(pathParams, query, headers, `{"a":1}`, nil)
	s2 := ComputeSignature(pathParams, query, headers, `{"a":2}`, nil)

	if s1 == s2 {
		t.Error("different bodies should produce different signatures")
	}
}

func TestComputeSignature_SpecificPathParams(t *testing.T) {
	pathParams := map[string]string{"id": "42", "type": "cat"}
	query := url.Values{}
	headers := http.Header{}
	body := ""

	cfg := &models.SignatureConfig{PathParams: []string{"id"}}

	// Only "id" is included; "type" is excluded
	sig1 := ComputeSignature(pathParams, query, headers, body, cfg)

	pathParams2 := map[string]string{"id": "42", "type": "dog"}
	sig2 := ComputeSignature(pathParams2, query, headers, body, cfg)

	if sig1 != sig2 {
		t.Errorf("changing excluded path param should not change signature: %q vs %q", sig1, sig2)
	}

	pathParams3 := map[string]string{"id": "99", "type": "cat"}
	sig3 := ComputeSignature(pathParams3, query, headers, body, cfg)
	if sig1 == sig3 {
		t.Error("changing included path param should change signature")
	}
}

func TestComputeSignature_SpecificQueryParams(t *testing.T) {
	pathParams := map[string]string{}
	headers := http.Header{}
	body := ""

	cfg := &models.SignatureConfig{QueryParams: []string{"page"}}

	q1 := url.Values{"page": {"1"}, "limit": {"10"}}
	q2 := url.Values{"page": {"1"}, "limit": {"50"}}
	q3 := url.Values{"page": {"2"}, "limit": {"10"}}

	s1 := ComputeSignature(pathParams, q1, headers, body, cfg)
	s2 := ComputeSignature(pathParams, q2, headers, body, cfg)
	s3 := ComputeSignature(pathParams, q3, headers, body, cfg)

	if s1 != s2 {
		t.Errorf("changing excluded query param should not change signature: %q vs %q", s1, s2)
	}
	if s1 == s3 {
		t.Error("changing included query param should change signature")
	}
}

func TestComputeSignature_Headers(t *testing.T) {
	pathParams := map[string]string{}
	query := url.Values{}
	body := ""

	cfg := &models.SignatureConfig{HeadersConfigured: true, Headers: []string{"X-Tenant"}}

	h1 := http.Header{"X-Tenant": {"acme"}, "Authorization": {"Bearer x"}}
	h2 := http.Header{"X-Tenant": {"acme"}, "Authorization": {"Bearer y"}}
	h3 := http.Header{"X-Tenant": {"globex"}, "Authorization": {"Bearer x"}}

	s1 := ComputeSignature(pathParams, query, h1, body, cfg)
	s2 := ComputeSignature(pathParams, query, h2, body, cfg)
	s3 := ComputeSignature(pathParams, query, h3, body, cfg)

	if s1 != s2 {
		t.Errorf("changing excluded header should not change signature: %q vs %q", s1, s2)
	}
	if s1 == s3 {
		t.Error("changing included header should change signature")
	}
}

func TestComputeSignature_HeadersCaseInsensitive(t *testing.T) {
	pathParams := map[string]string{}
	query := url.Values{}
	body := ""

	cfg := &models.SignatureConfig{HeadersConfigured: true, Headers: []string{"x-tenant"}}

	// Header name canonicalized vs lowercase key in config
	h := http.Header{"X-Tenant": {"acme"}}

	sig := ComputeSignature(pathParams, query, h, body, cfg)
	if sig == "" {
		t.Fatal("expected non-empty signature with header config")
	}
}

func TestComputeSignature_ExcludeBody(t *testing.T) {
	pathParams := map[string]string{}
	query := url.Values{}
	headers := http.Header{}

	includeBody := false
	cfg := &models.SignatureConfig{IncludeBody: &includeBody}

	s1 := ComputeSignature(pathParams, query, headers, `{"a":1}`, cfg)
	s2 := ComputeSignature(pathParams, query, headers, `{"a":2}`, cfg)

	if s1 != s2 {
		t.Errorf("with IncludeBody=false bodies should not affect signature: %q vs %q", s1, s2)
	}
}

func TestComputeSignature_BodyJsonPaths(t *testing.T) {
	pathParams := map[string]string{}
	query := url.Values{}
	headers := http.Header{}

	includeBody := true
	cfg := &models.SignatureConfig{IncludeBody: &includeBody, BodyJsonPaths: []string{"user.id"}}

	body1 := `{"user":{"id":"u1","name":"Alice"},"ts":1}`
	body2 := `{"user":{"id":"u1","name":"Bob"},"ts":99}`
	body3 := `{"user":{"id":"u2","name":"Alice"},"ts":1}`

	s1 := ComputeSignature(pathParams, query, headers, body1, cfg)
	s2 := ComputeSignature(pathParams, query, headers, body2, cfg)
	s3 := ComputeSignature(pathParams, query, headers, body3, cfg)

	if s1 != s2 {
		t.Errorf("changing non-extracted field should not change signature: %q vs %q", s1, s2)
	}
	if s1 == s3 {
		t.Error("changing extracted JSON path value should change signature")
	}
}

func TestComputeSignature_EmptyInputs(t *testing.T) {
	sig := ComputeSignature(nil, nil, nil, "", nil)
	if len(sig) != 16 {
		t.Errorf("expected 16-char signature for empty inputs, got %q", sig)
	}
}

func TestComputeSignature_IgnoresVirtualHeaders(t *testing.T) {
	cfg := &models.SignatureConfig{HeadersConfigured: true, Headers: []string{"X-Virtual-AI-Scenario", "X-Tenant"}}

	headersA := map[string][]string{
		"X-Virtual-AI-Scenario": {"success"},
		"X-Tenant":              {"tenant-a"},
	}
	headersB := map[string][]string{
		"X-Virtual-AI-Scenario": {"client_error"},
		"X-Tenant":              {"tenant-a"},
	}

	sigA := ComputeSignature(nil, nil, headersA, "", cfg)
	sigB := ComputeSignature(nil, nil, headersB, "", cfg)
	if sigA != sigB {
		t.Fatalf("expected virtual headers to be ignored, got %q != %q", sigA, sigB)
	}
}

func TestComputeSignature_IgnoresVirtualHeadersCaseInsensitive(t *testing.T) {
	cfg := &models.SignatureConfig{HeadersConfigured: true, Headers: []string{"x-virtual-trace", "X-Test"}}

	headersA := map[string][]string{
		"X-VIRTUAL-TRACE": {"one"},
		"X-Test":          {"same"},
	}
	headersB := map[string][]string{
		"x-virtual-trace": {"two"},
		"X-Test":          {"same"},
	}

	sigA := ComputeSignature(nil, nil, headersA, "", cfg)
	sigB := ComputeSignature(nil, nil, headersB, "", cfg)
	if sigA != sigB {
		t.Fatalf("expected case-insensitive virtual headers to be ignored, got %q != %q", sigA, sigB)
	}
}

func TestResolveSignatureConfig_UsesDeclaredDefaults(t *testing.T) {
	spec := &models.Spec{SignatureHeaders: []string{"X-Tenant"}}
	op := &models.Operation{
		DeclaredPathParams:   []string{"id"},
		DeclaredQueryParams:  []string{"status"},
		DeclaredHeaderParams: []string{"X-Request-Id"},
		HasRequestBody:       true,
	}

	cfg := ResolveSignatureConfig(spec, op)
	if len(cfg.PathParams) != 1 || cfg.PathParams[0] != "id" {
		t.Fatalf("unexpected path defaults: %#v", cfg.PathParams)
	}
	if len(cfg.QueryParams) != 1 || cfg.QueryParams[0] != "status" {
		t.Fatalf("unexpected query defaults: %#v", cfg.QueryParams)
	}
	if len(cfg.Headers) != 2 {
		t.Fatalf("unexpected header defaults: %#v", cfg.Headers)
	}
	if cfg.IncludeBody == nil || !*cfg.IncludeBody {
		t.Fatalf("expected body to default to included, got %#v", cfg.IncludeBody)
	}
}

func TestResolveSignatureConfig_ExplicitHeaderNoneAndBodyOverride(t *testing.T) {
	includeBody := false
	spec := &models.Spec{SignatureHeaders: []string{"X-Tenant"}}
	op := &models.Operation{
		DeclaredPathParams:   []string{"id"},
		DeclaredQueryParams:  []string{"status"},
		DeclaredHeaderParams: []string{"X-Request-Id"},
		HasRequestBody:       true,
		SignatureConfig: &models.SignatureConfig{
			HeadersConfigured: true,
			Headers:           []string{},
			IncludeBody:       &includeBody,
		},
	}

	cfg := ResolveSignatureConfig(spec, op)
	if len(cfg.Headers) != 0 {
		t.Fatalf("expected explicit empty headers to disable header inclusion, got %#v", cfg.Headers)
	}
	if cfg.IncludeBody == nil || *cfg.IncludeBody {
		t.Fatalf("expected body inclusion override to disable body, got %#v", cfg.IncludeBody)
	}
}

func TestResolveSignatureConfig_UsesExplicitSelections(t *testing.T) {
	includeBody := true
	spec := &models.Spec{SignatureHeaders: []string{"X-Tenant"}}
	op := &models.Operation{
		DeclaredPathParams:   []string{"id", "accountId"},
		DeclaredQueryParams:  []string{"status", "expand"},
		DeclaredHeaderParams: []string{"X-Request-Id"},
		HasRequestBody:       true,
		SignatureConfig: &models.SignatureConfig{
			PathParams:        []string{"id"},
			QueryParams:       []string{"status"},
			HeadersConfigured: true,
			Headers:           []string{"X-Custom"},
			IncludeBody:       &includeBody,
			BodyJsonPaths:     []string{"user.id"},
		},
	}

	cfg := ResolveSignatureConfig(spec, op)
	if len(cfg.PathParams) != 1 || cfg.PathParams[0] != "id" {
		t.Fatalf("unexpected explicit path params: %#v", cfg.PathParams)
	}
	if len(cfg.QueryParams) != 1 || cfg.QueryParams[0] != "status" {
		t.Fatalf("unexpected explicit query params: %#v", cfg.QueryParams)
	}
	if len(cfg.Headers) != 1 || cfg.Headers[0] != "X-Custom" {
		t.Fatalf("unexpected explicit headers: %#v", cfg.Headers)
	}
	if len(cfg.BodyJsonPaths) != 1 || cfg.BodyJsonPaths[0] != "user.id" {
		t.Fatalf("unexpected explicit body paths: %#v", cfg.BodyJsonPaths)
	}
}
