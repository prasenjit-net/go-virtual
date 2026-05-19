package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
)

// mockOpenAI starts a test HTTP server that simulates the OpenAI chat completions
// endpoint. respBody is the raw JSON to return; statusCode defaults to 200.
func mockOpenAI(t *testing.T, statusCode int, respBody string) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if statusCode != 0 {
			w.WriteHeader(statusCode)
		}
		w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv, srv.URL
}

// openAISuccessResponse builds a fake OpenAI chat completions response.
func openAISuccessResponse(content string) string {
	data, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]string{"content": content}},
		},
	})
	return string(data)
}

func mockClaude(t *testing.T, statusCode int, respBody string) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if statusCode != 0 {
			w.WriteHeader(statusCode)
		}
		w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv, srv.URL
}

func claudeSuccessResponse(content string) string {
	data, _ := json.Marshal(map[string]any{
		"content": []map[string]string{
			{"type": "text", "text": content},
		},
	})
	return string(data)
}

// ── NewGenerator / IsConfigured ───────────────────────────────────────────────

func TestNewGenerator_Defaults(t *testing.T) {
	g := NewGenerator(Config{APIKey: "key-123"})
	if g == nil {
		t.Fatal("NewGenerator returned nil")
	}
	if g.cfg.Model != defaultModel {
		t.Errorf("expected default model %q, got %q", defaultModel, g.cfg.Model)
	}
}

func TestNewGenerator_CustomModel(t *testing.T) {
	g := NewGenerator(Config{APIKey: "k", Model: "gpt-4"})
	if g.cfg.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %q", g.cfg.Model)
	}
}

func TestIsConfigured_True(t *testing.T) {
	g := NewGenerator(Config{APIKey: "sk-test"})
	if !g.IsConfigured() {
		t.Error("expected IsConfigured true")
	}
}

func TestIsConfigured_False_EmptyKey(t *testing.T) {
	g := NewGenerator(Config{})
	if g.IsConfigured() {
		t.Error("expected IsConfigured false for empty key")
	}
}

func TestIsConfigured_False_WhitespaceKey(t *testing.T) {
	g := NewGenerator(Config{APIKey: "   "})
	if g.IsConfigured() {
		t.Error("expected IsConfigured false for whitespace-only key")
	}
}

// ── validateConditions ────────────────────────────────────────────────────────

func TestValidateConditions_Valid(t *testing.T) {
	conditions := []models.Condition{
		{Source: "path", Key: "id", Operator: "eq", Value: "42"},
		{Source: "query", Key: "status", Operator: "ne", Value: "inactive"},
		{Source: "header", Key: "Authorization", Operator: "exists", Value: ""},
		{Source: "body", Key: "user.role", Operator: "eq", Value: "admin"},
	}
	if err := validateConditions(conditions); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConditions_Empty(t *testing.T) {
	if err := validateConditions(nil); err != nil {
		t.Errorf("nil conditions should be valid, got: %v", err)
	}
	if err := validateConditions([]models.Condition{}); err != nil {
		t.Errorf("empty conditions should be valid, got: %v", err)
	}
}

func TestValidateConditions_InvalidSource(t *testing.T) {
	err := validateConditions([]models.Condition{
		{Source: "cookie", Key: "token", Operator: "eq", Value: "x"},
	})
	if err == nil {
		t.Error("expected error for invalid source")
	}
	if !strings.Contains(err.Error(), "source") {
		t.Errorf("error should mention 'source', got: %v", err)
	}
}

func TestValidateConditions_InvalidOperator(t *testing.T) {
	err := validateConditions([]models.Condition{
		{Source: "query", Key: "q", Operator: "like", Value: "x"},
	})
	if err == nil {
		t.Error("expected error for invalid operator")
	}
	if !strings.Contains(err.Error(), "operator") {
		t.Errorf("error should mention 'operator', got: %v", err)
	}
}

func TestValidateConditions_EmptyKey(t *testing.T) {
	err := validateConditions([]models.Condition{
		{Source: "query", Key: "", Operator: "eq", Value: "x"},
	})
	if err == nil {
		t.Error("expected error for empty key")
	}
	if !strings.Contains(err.Error(), "key") {
		t.Errorf("error should mention 'key', got: %v", err)
	}
}

func TestValidateConditions_AllOperators(t *testing.T) {
	ops := []string{
		"eq", "ne", "contains", "notContains", "regex",
		"exists", "notExists", "gt", "lt", "gte", "lte",
		"startsWith", "endsWith",
	}
	for _, op := range ops {
		err := validateConditions([]models.Condition{
			{Source: "query", Key: "x", Operator: op, Value: ""},
		})
		if err != nil {
			t.Errorf("operator %q should be valid, got: %v", op, err)
		}
	}
}

// ── buildSystemPrompt ─────────────────────────────────────────────────────────

func TestBuildSystemPrompt_Basic(t *testing.T) {
	op := OperationContext{Method: "GET", Path: "/pets"}
	prompt := buildSystemPrompt(op)

	for _, want := range []string{"statusCode", "conditions", "body", "priority", "CONDITION SCHEMA"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
}

func TestBuildSystemPrompt_WithSpecResponses(t *testing.T) {
	op := OperationContext{
		Method: "GET",
		Path:   "/pets",
		SpecResponses: []SpecResponseDef{
			{StatusCode: 200, Description: "Success", BodyExample: `{"id":"1"}`},
			{StatusCode: 404, Description: "Not found", SchemaHint: "object with fields: code, message"},
			{StatusCode: 0, Description: "Default error"},
		},
	}
	prompt := buildSystemPrompt(op)

	if !strings.Contains(prompt, "SPEC-DEFINED RESPONSES") {
		t.Error("prompt missing SPEC-DEFINED RESPONSES section")
	}
	if !strings.Contains(prompt, "[200]") {
		t.Error("prompt missing [200] status code")
	}
	if !strings.Contains(prompt, "[404]") {
		t.Error("prompt missing [404] status code")
	}
	if !strings.Contains(prompt, "[default]") {
		t.Error("prompt missing [default] status code")
	}
	if !strings.Contains(prompt, `{"id":"1"}`) {
		t.Error("prompt missing body example")
	}
}

func TestBuildSystemPrompt_WithInputs(t *testing.T) {
	op := OperationContext{
		Method: "POST",
		Path:   "/pets",
		Inputs: &OperationInputs{
			PathParams:  []ParamDef{{Name: "petId", In: "path", Type: "string", Required: true}},
			QueryParams: []ParamDef{{Name: "status", In: "query", Type: "string"}},
			BodyFields:  []BodyFieldDef{{GjsonPath: "name", Type: "string", Description: "pet name"}},
		},
	}
	prompt := buildSystemPrompt(op)

	if !strings.Contains(prompt, "AVAILABLE REQUEST INPUTS") {
		t.Error("prompt missing AVAILABLE REQUEST INPUTS section")
	}
	if !strings.Contains(prompt, "petId") {
		t.Error("prompt missing path param petId")
	}
	if !strings.Contains(prompt, "status") {
		t.Error("prompt missing query param status")
	}
	if !strings.Contains(prompt, "name") {
		t.Error("prompt missing body field name")
	}
}

func TestBuildSystemPrompt_WithExampleResponse(t *testing.T) {
	op := OperationContext{
		Method: "GET",
		Path:   "/pets",
		ExampleResponse: &models.ExampleResponse{
			StatusCode: 200,
			Body:       `{"id":"1","name":"Fluffy"}`,
		},
	}
	prompt := buildSystemPrompt(op)
	if !strings.Contains(prompt, "Fluffy") {
		t.Error("prompt missing example response body content")
	}
}

// ── buildUserMessage ──────────────────────────────────────────────────────────

func TestBuildUserMessage_WithPrompt(t *testing.T) {
	op := OperationContext{Method: "POST", Path: "/pets", Summary: "Create a pet"}
	msg := buildUserMessage(op, "generate an error response")

	if !strings.Contains(msg, "POST") {
		t.Error("message missing method")
	}
	if !strings.Contains(msg, "/pets") {
		t.Error("message missing path")
	}
	if !strings.Contains(msg, "generate an error response") {
		t.Error("message missing user prompt")
	}
}

func TestBuildUserMessage_NoPrompt(t *testing.T) {
	op := OperationContext{Method: "GET", Path: "/pets"}
	msg := buildUserMessage(op, "")
	if !strings.Contains(msg, "realistic fake data") {
		t.Error("empty prompt should fall back to default message")
	}
}

func TestBuildRuntimeSystemPrompt(t *testing.T) {
	op := OperationContext{
		Method: "GET",
		Path:   "/pets",
		SpecResponses: []SpecResponseDef{
			{StatusCode: 200, Description: "Success", BodyExample: `{"id":"1"}`},
			{StatusCode: 500, Description: "Error", SchemaHint: "object with fields: error"},
		},
	}

	prompt := buildRuntimeSystemPrompt(op, nil)
	if !strings.Contains(prompt, "Spec-defined responses") {
		t.Fatal("expected spec-defined responses section")
	}
	if !strings.Contains(prompt, `{"id":"1"}`) || !strings.Contains(prompt, "Schema: object with fields: error") {
		t.Fatalf("unexpected runtime system prompt: %s", prompt)
	}
}

func TestBuildRuntimeSystemPrompt_WithScenario(t *testing.T) {
	op := OperationContext{Method: "GET", Path: "/pets"}

	prompt := buildRuntimeSystemPrompt(op, &RuntimeScenario{
		Name:         "client_error",
		ResponseKind: "error",
		StatusCode:   400,
		Count:        5,
		Instructions: "Return validation details",
	})

	for _, want := range []string{"Scenario name: client_error", "Status code: 400", "Item count: return exactly 5", "Return validation details"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("runtime system prompt missing %q: %s", want, prompt)
		}
	}
}

func TestBuildRuntimeUserMessage(t *testing.T) {
	op := OperationContext{
		Method:      "POST",
		Path:        "/pets/{id}",
		Summary:     "Create pet",
		Description: "Creates a pet resource",
		Inputs: &OperationInputs{
			PathParams:  []ParamDef{{Name: "id", Type: "string"}},
			QueryParams: []ParamDef{{Name: "status", Type: "string"}},
			BodyFields:  []BodyFieldDef{{GjsonPath: "pet.name", Type: "string"}},
		},
	}
	reqCtx := RuntimeRequestContext{
		PathParams:  map[string]string{"id": "123"},
		QueryParams: map[string][]string{"status": {"active"}},
		Headers:     map[string][]string{"X-Test": {"1"}},
		Body:        `{"pet":{"name":"Fido"}}`,
		Signature:   "sig-123",
	}

	msg := buildRuntimeUserMessage(op, reqCtx)
	for _, want := range []string{"Request signature: sig-123", "Path params:", "pet.name", "Create pet", `{"pet":{"name":"Fido"}}`} {
		if !strings.Contains(msg, want) {
			t.Fatalf("runtime user message missing %q: %s", want, msg)
		}
	}
}

func TestStringifyRuntimeBody(t *testing.T) {
	tests := []struct {
		name string
		body any
		want string
	}{
		{name: "nil", body: nil, want: ""},
		{name: "string", body: "hello", want: "hello"},
		{name: "object", body: map[string]any{"id": 1}, want: `{"id":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stringifyRuntimeBody(tt.body)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("stringifyRuntimeBody(%v) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestDefaultRuntimeStatusCode(t *testing.T) {
	if got := defaultRuntimeStatusCode(OperationContext{
		SpecResponses: []SpecResponseDef{{StatusCode: 201}, {StatusCode: 200}},
	}); got != 201 {
		t.Fatalf("expected first spec response status, got %d", got)
	}

	if got := defaultRuntimeStatusCode(OperationContext{
		ExampleResponse: &models.ExampleResponse{StatusCode: 202},
	}); got != 202 {
		t.Fatalf("expected example response status, got %d", got)
	}

	if got := defaultRuntimeStatusCode(OperationContext{}); got != 200 {
		t.Fatalf("expected default status 200, got %d", got)
	}
}

func TestGenerateRuntimeResponse(t *testing.T) {
	srv, endpoint := mockOpenAI(t, 200, openAISuccessResponse(`{"statusCode":0,"headers":{},"body":{"id":"pet-1"}}`))
	_ = srv

	g := NewGenerator(Config{APIKey: "sk-test", Endpoint: endpoint})
	resp, err := g.GenerateRuntimeResponse(context.Background(), OperationContext{
		Method:        "GET",
		Path:          "/pets/{id}",
		SpecResponses: []SpecResponseDef{{StatusCode: 201}},
	}, RuntimeRequestContext{
		PathParams: map[string]string{"id": "pet-1"},
	})
	if err != nil {
		t.Fatalf("GenerateRuntimeResponse returned error: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("expected defaulted status 201, got %d", resp.StatusCode)
	}
	if resp.Headers["Content-Type"] != "application/json" {
		t.Fatalf("expected default json content type, got %q", resp.Headers["Content-Type"])
	}
	if resp.Body != `{"id":"pet-1"}` {
		t.Fatalf("unexpected body %q", resp.Body)
	}
}

func TestGenerateRuntimeResponse_UsesScenarioStatusCode(t *testing.T) {
	srv, endpoint := mockOpenAI(t, 200, openAISuccessResponse(`{"statusCode":0,"headers":{},"body":{"error":"unauthorized"}}`))
	_ = srv

	g := NewGenerator(Config{APIKey: "sk-test", Endpoint: endpoint})
	resp, err := g.GenerateRuntimeResponse(context.Background(), OperationContext{
		Method:        "GET",
		Path:          "/pets/{id}",
		SpecResponses: []SpecResponseDef{{StatusCode: 200}},
	}, RuntimeRequestContext{
		Scenario: &RuntimeScenario{
			Name:         "unauthorized",
			ResponseKind: "error",
			StatusCode:   401,
		},
	})
	if err != nil {
		t.Fatalf("GenerateRuntimeResponse returned error: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("expected scenario status 401, got %d", resp.StatusCode)
	}
}

func TestGenerateRuntimeResponse_InvalidJSON(t *testing.T) {
	srv, endpoint := mockOpenAI(t, 200, openAISuccessResponse(`not-json`))
	_ = srv

	g := NewGenerator(Config{APIKey: "sk-test", Endpoint: endpoint})
	_, err := g.GenerateRuntimeResponse(context.Background(), OperationContext{Method: "GET", Path: "/pets"}, RuntimeRequestContext{})
	if err == nil || !strings.Contains(err.Error(), "model returned invalid JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
}

// ── buildScriptSystemPrompt ───────────────────────────────────────────────────

func TestBuildScriptSystemPrompt_Basic(t *testing.T) {
	sctx := ScriptContext{}
	prompt := buildScriptSystemPrompt(sctx)

	for _, want := range []string{
		"def run(req):",
		"store.get",
		"store.set",
		"log(",
		"OUTPUT FORMAT",
		"STARLARK LANGUAGE CONSTRAINTS",
		"ENTRY POINT",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("script system prompt missing %q", want)
		}
	}
}

func TestBuildScriptSystemPrompt_WithOperationInputs(t *testing.T) {
	sctx := ScriptContext{
		OperationMethod:  "GET",
		OperationPath:    "/pets/{petId}",
		OperationSummary: "Get pet by ID",
		Inputs: &OperationInputs{
			PathParams:  []ParamDef{{Name: "petId", In: "path", Type: "string"}},
			QueryParams: []ParamDef{{Name: "format", In: "query", Type: "string", Required: false}},
			BodyFields:  []BodyFieldDef{{GjsonPath: "tags.0", Type: "string"}},
		},
	}
	prompt := buildScriptSystemPrompt(sctx)

	if !strings.Contains(prompt, "AVAILABLE REQUEST INPUTS FOR THIS OPERATION") {
		t.Error("prompt missing operation inputs section")
	}
	if !strings.Contains(prompt, `req.path("petId")`) {
		t.Error("prompt missing path param")
	}
	if !strings.Contains(prompt, `req.query("format")`) {
		t.Error("prompt missing query param")
	}
	if !strings.Contains(prompt, "tags.0") {
		t.Error("prompt missing body field")
	}
}

func TestBuildScriptSystemPrompt_CommentBlockRules(t *testing.T) {
	prompt := buildScriptSystemPrompt(ScriptContext{})
	for _, want := range []string{"REWRITE", "Store:", "Returns:", "Inputs:"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("script system prompt missing comment block rule keyword %q", want)
		}
	}
}

// ── buildScriptUserMessage ────────────────────────────────────────────────────

func TestBuildScriptUserMessage_FirstTurn(t *testing.T) {
	sctx := ScriptContext{OperationMethod: "POST", OperationPath: "/orders", OperationSummary: "Place order"}
	msg := buildScriptUserMessage(sctx, "", "track order count")

	if !strings.Contains(msg, "POST") {
		t.Error("message missing method")
	}
	if !strings.Contains(msg, "track order count") {
		t.Error("message missing task")
	}
	// No current source on first turn — should not include "Current script" block.
	if strings.Contains(msg, "Current script") {
		t.Error("first-turn message should not include Current script block")
	}
}

func TestBuildScriptUserMessage_SubsequentTurn(t *testing.T) {
	sctx := ScriptContext{OperationMethod: "GET", OperationPath: "/pets"}
	existingSource := `def run(req):
    return {"count": 1}
`
	msg := buildScriptUserMessage(sctx, existingSource, "also log the request")

	if !strings.Contains(msg, "Current script") {
		t.Error("subsequent turn should include current script")
	}
	if !strings.Contains(msg, "also log the request") {
		t.Error("message missing updated task")
	}
	if !strings.Contains(msg, existingSource[:20]) {
		t.Error("message should embed the current source")
	}
}

func TestBuildScriptUserMessage_NoOperation(t *testing.T) {
	msg := buildScriptUserMessage(ScriptContext{}, "", "just do something")
	if !strings.Contains(msg, "just do something") {
		t.Error("message missing task")
	}
	// No operation prefix when method is empty.
	if strings.Contains(msg, "API operation:") {
		t.Error("message should not include API operation prefix when context is empty")
	}
}

// ── GenerateResponse / GenerateScript — unconfigured ─────────────────────────

func TestGenerateResponse_NotConfigured(t *testing.T) {
	g := NewGenerator(Config{})
	_, err := g.GenerateResponse(nil, OperationContext{Method: "GET", Path: "/"}, "")
	if err == nil {
		t.Error("expected error when not configured")
	}
	if !strings.Contains(err.Error(), "ai.openai.apiKey") {
		t.Errorf("error should mention provider config key, got: %v", err)
	}
}

func TestGenerateScript_NotConfigured(t *testing.T) {
	g := NewGenerator(Config{})
	_, err := g.GenerateScript(nil, ScriptContext{}, nil, "", "do something")
	if err == nil {
		t.Error("expected error when not configured")
	}
	if !strings.Contains(err.Error(), "ai.openai.apiKey") {
		t.Errorf("error should mention provider config key, got: %v", err)
	}
}

// ── GenerateResponse — mock OpenAI server ─────────────────────────────────────

func TestGenerateResponse_Success(t *testing.T) {
	responseBody := `{
"statusCode": 200,
"name": "Success",
"description": "Returns a list of pets",
"headers": {"Content-Type": "application/json"},
"body": "{\"pets\":[]}",
"priority": 10,
"enabled": true,
"conditions": [],
"delay": 0
}`
	_, url := mockOpenAI(t, 200, openAISuccessResponse(responseBody))

	g := NewGenerator(Config{APIKey: "test-key", Endpoint: url})
	result, err := g.GenerateResponse(context.Background(), OperationContext{Method: "GET", Path: "/pets"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("expected StatusCode 200, got %d", result.StatusCode)
	}
	if !result.Enabled {
		t.Error("expected Enabled true")
	}
}

func TestGenerateResponse_OpenAIError(t *testing.T) {
	errBody := `{"error":{"message":"invalid api key"}}`
	_, url := mockOpenAI(t, 401, errBody)

	g := NewGenerator(Config{APIKey: "bad-key", Endpoint: url})
	_, err := g.GenerateResponse(context.Background(), OperationContext{Method: "GET", Path: "/pets"}, "")
	if err == nil {
		t.Error("expected error from OpenAI error response")
	}
}

func TestGenerateResponse_InvalidJSON(t *testing.T) {
	_, url := mockOpenAI(t, 200, openAISuccessResponse("not valid json"))

	g := NewGenerator(Config{APIKey: "test-key", Endpoint: url})
	_, err := g.GenerateResponse(context.Background(), OperationContext{Method: "GET", Path: "/pets"}, "")
	if err == nil {
		t.Error("expected error for invalid JSON from model")
	}
}

func TestGenerateResponse_InvalidConditions(t *testing.T) {
	responseBody := `{
"statusCode": 200,
"name": "Bad",
"headers": {},
"body": "{}",
"priority": 5,
"enabled": true,
"conditions": [{"source":"cookie","key":"x","operator":"eq","value":"1"}],
"delay": 0
}`
	_, url := mockOpenAI(t, 200, openAISuccessResponse(responseBody))

	g := NewGenerator(Config{APIKey: "test-key", Endpoint: url})
	_, err := g.GenerateResponse(context.Background(), OperationContext{Method: "GET", Path: "/pets"}, "")
	if err == nil {
		t.Error("expected error for invalid condition source")
	}
}

func TestGenerateResponse_NoChoices(t *testing.T) {
	_, url := mockOpenAI(t, 200, `{"choices":[]}`)

	g := NewGenerator(Config{APIKey: "test-key", Endpoint: url})
	_, err := g.GenerateResponse(context.Background(), OperationContext{Method: "GET", Path: "/pets"}, "")
	if err == nil {
		t.Error("expected error when OpenAI returns no choices")
	}
}

func TestGenerateResponse_DefaultsApplied(t *testing.T) {
	// Model omits optional fields — defaults should be applied.
	responseBody := `{
"statusCode": 0,
"name": "Minimal",
"headers": null,
"body": "{}",
"priority": 0,
"enabled": false,
"conditions": null,
"delay": 0
}`
	_, url := mockOpenAI(t, 200, openAISuccessResponse(responseBody))

	g := NewGenerator(Config{APIKey: "test-key", Endpoint: url})
	result, err := g.GenerateResponse(context.Background(), OperationContext{Method: "GET", Path: "/pets"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("expected default StatusCode 200, got %d", result.StatusCode)
	}
	if result.Headers == nil {
		t.Error("expected default headers to be applied")
	}
	if result.Conditions == nil {
		t.Error("expected default conditions slice (not nil)")
	}
	if result.Priority != 10 {
		t.Errorf("expected default priority 10, got %d", result.Priority)
	}
	if !result.Enabled {
		t.Error("expected Enabled to be forced true")
	}
}

func TestGenerateResponse_ClaudeProvider(t *testing.T) {
	responseBody := `{
"statusCode": 201,
"name": "Created",
"description": "Creates a pet",
"headers": {"Content-Type": "application/json"},
"body": "{\"id\":\"pet-2\"}",
"priority": 10,
"enabled": true,
"conditions": [],
"delay": 0
}`
	_, url := mockClaude(t, 200, claudeSuccessResponse(responseBody))

	g := NewGenerator(Config{
		Provider: "claude",
		Claude: ClaudeProviderConfig{
			APIKey:   "claude-test-key",
			Model:    "claude-3-7-sonnet-latest",
			Endpoint: url,
		},
	})
	result, err := g.GenerateResponse(context.Background(), OperationContext{Method: "POST", Path: "/pets"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 201 {
		t.Errorf("expected StatusCode 201, got %d", result.StatusCode)
	}
	if got := g.Status(); got.Provider != "claude" || !got.Configured {
		t.Fatalf("expected configured Claude status, got %+v", got)
	}
}

// ── GenerateScript — mock OpenAI server ──────────────────────────────────────

func TestGenerateScript_Success(t *testing.T) {
	scriptSrc := "# Count requests\n#\n# Inputs:  none\n# Store:   reads/writes \"count\"\n# Returns: {\"count\"}\n\ndef run(req):\n    count = store.get(\"count\", 0)\n    store.set(\"count\", count + 1)\n    return {\"count\": count + 1}\n"
	wrapper, _ := json.Marshal(map[string]string{"source": scriptSrc})
	_, url := mockOpenAI(t, 200, openAISuccessResponse(string(wrapper)))

	g := NewGenerator(Config{APIKey: "test-key", Endpoint: url})
	source, err := g.GenerateScript(context.Background(), ScriptContext{}, nil, "", "count requests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(source, "def run(req):") {
		t.Errorf("generated source should contain run(req), got: %s", source)
	}
}

func TestGenerateScript_WithHistory(t *testing.T) {
	scriptSrc := "def run(req):\n    return {\"ok\": True}\n"
	wrapper, _ := json.Marshal(map[string]string{"source": scriptSrc})
	_, url := mockOpenAI(t, 200, openAISuccessResponse(string(wrapper)))

	history := []ChatMessage{
		{Role: "user", Content: "initial prompt"},
		{Role: "assistant", Content: "{\"source\": \"def run(req): return {}\"}"},
	}
	g := NewGenerator(Config{APIKey: "test-key", Endpoint: url})
	source, err := g.GenerateScript(context.Background(), ScriptContext{}, history, "def run(req): return {}", "add ok flag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source == "" {
		t.Error("expected non-empty source")
	}
}

func TestGenerateScript_EmptySource(t *testing.T) {
	wrapper, _ := json.Marshal(map[string]string{"source": "   "})
	_, url := mockOpenAI(t, 200, openAISuccessResponse(string(wrapper)))

	g := NewGenerator(Config{APIKey: "test-key", Endpoint: url})
	_, err := g.GenerateScript(context.Background(), ScriptContext{}, nil, "", "something")
	if err == nil {
		t.Error("expected error for empty generated source")
	}
}

func TestGenerateScript_OpenAIError(t *testing.T) {
	_, url := mockOpenAI(t, 200, `{"error":{"message":"quota exceeded"}}`)

	g := NewGenerator(Config{APIKey: "test-key", Endpoint: url})
	_, err := g.GenerateScript(context.Background(), ScriptContext{}, nil, "", "something")
	if err == nil {
		t.Error("expected error from OpenAI error response")
	}
}

func TestGenerateScript_ClaudeProvider(t *testing.T) {
	scriptSrc := "# Echo request\n#\n# Inputs:  req[\"query\"][\"name\"] — optional name\n# Store:   none\n# Returns: {\"hello\"}\n\ndef run(req):\n    return {\"hello\": req[\"query\"].get(\"name\", \"world\")}\n"
	wrapper, _ := json.Marshal(map[string]string{"source": scriptSrc})
	_, url := mockClaude(t, 200, claudeSuccessResponse(string(wrapper)))

	g := NewGenerator(Config{
		Provider: "claude",
		Claude: ClaudeProviderConfig{
			APIKey:   "claude-test-key",
			Model:    "claude-3-7-sonnet-latest",
			Endpoint: url,
		},
	})
	source, err := g.GenerateScript(context.Background(), ScriptContext{}, nil, "", "echo the name query param")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(source, "def run(req):") {
		t.Fatalf("expected Claude-generated script source, got %q", source)
	}
}

func TestGenerateResponse_ClaudeModelErrorIncludesConfigHint(t *testing.T) {
	_, url := mockClaude(t, 400, `{"error":{"type":"invalid_request_error","message":"model: invalid-model"}}`)

	g := NewGenerator(Config{
		Provider: "claude",
		Claude: ClaudeProviderConfig{
			APIKey:   "claude-test-key",
			Model:    "invalid-model",
			Endpoint: url,
		},
	})
	_, err := g.GenerateResponse(context.Background(), OperationContext{Method: "GET", Path: "/pets"}, "")
	if err == nil {
		t.Fatal("expected Claude model error")
	}
	if !strings.Contains(err.Error(), "ai.claude.model") || !strings.Contains(err.Error(), "invalid-model") {
		t.Fatalf("expected Claude model hint in error, got %v", err)
	}
}
