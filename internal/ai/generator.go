package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
)

const defaultModel = "gpt-4o-mini"
const openAIEndpoint = "https://api.openai.com/v1/chat/completions"

// Config holds the AI generator configuration.
type Config struct {
	APIKey string
	Model  string
	// Endpoint overrides the OpenAI API URL. Used in tests to point at a
	// local mock server. Defaults to openAIEndpoint when empty.
	Endpoint string
}

// Generator uses OpenAI to generate mock response configurations.
type Generator struct {
	cfg      Config
	client   *http.Client
	endpoint string
}

// NewGenerator creates a new Generator with the given config.
func NewGenerator(cfg Config) *Generator {
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}
	ep := cfg.Endpoint
	if ep == "" {
		ep = openAIEndpoint
	}
	return &Generator{
		cfg:      cfg,
		client:   &http.Client{Timeout: 30 * time.Second},
		endpoint: ep,
	}
}

// IsConfigured returns true if the generator has a valid API key.
func (g *Generator) IsConfigured() bool {
	return strings.TrimSpace(g.cfg.APIKey) != ""
}

// OperationContext provides the operation metadata used to build the prompt.
type OperationContext struct {
	Method          string
	Path            string
	Summary         string
	Description     string
	ExampleResponse *models.ExampleResponse
	// SpecResponses holds every response definition from the OpenAPI spec, keyed by status code.
	// StatusCode 0 means the spec's "default" response.
	SpecResponses []SpecResponseDef
	// Inputs describes the available path params, query params, and request body fields.
	Inputs *OperationInputs
}

// SpecResponseDef mirrors parser.SpecResponseDef but lives in the ai package
// to avoid an import cycle.
type SpecResponseDef struct {
	StatusCode  int
	Description string
	BodyExample string // JSON string (from example) or schema-derived placeholder
	SchemaHint  string // e.g. "object with fields: id, name, email"
}

// OperationInputs mirrors parser.OperationInputs.
type OperationInputs struct {
	PathParams  []ParamDef
	QueryParams []ParamDef
	BodyFields  []BodyFieldDef
}

// ParamDef describes a single path or query parameter.
type ParamDef struct {
	Name        string
	In          string
	Required    bool
	Type        string
	Description string
}

// BodyFieldDef describes one field in the request body with its gjson path.
type BodyFieldDef struct {
	GjsonPath   string // dot-notation gjson path, e.g. "user.id"
	Type        string
	Description string
}

// RuntimeRequestContext captures the live request data used for runtime AI generation.
type RuntimeRequestContext struct {
	PathParams  map[string]string
	QueryParams map[string][]string
	Headers     map[string][]string
	Body        string
	Signature   string
	Scenario    *RuntimeScenario
}

// RuntimeResponse is the concrete HTTP response shape returned by runtime AI generation.
type RuntimeResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       string
}

// RuntimeScenario carries structured scenario hints for runtime generation.
type RuntimeScenario struct {
	Name                    string
	Description             string
	ResponseKind            string
	StatusCode              int
	Count                   int
	Instructions            string
	UseDefaultSuccessStatus bool
}

// GenerateResponse calls the OpenAI API and returns a ResponseConfigInput
// populated with realistic fake data. userPrompt may be empty.
func (g *Generator) GenerateResponse(ctx context.Context, op OperationContext, userPrompt string) (*models.ResponseConfigInput, error) {
	if !g.IsConfigured() {
		return nil, fmt.Errorf("OpenAI API key not configured — set ai.openaiApiKey in config.yaml or the GOVIRTUAL_AI_OPENAIAPIKEY environment variable")
	}

	systemPrompt := buildSystemPrompt(op)
	userMsg := buildUserMessage(op, userPrompt)

	reqBody := map[string]any{
		"model": g.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMsg},
		},
		"temperature":     0.7,
		"response_format": map[string]string{"type": "json_object"},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.cfg.APIKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode OpenAI response: %w", err)
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("OpenAI error: %s", apiResp.Error.Message)
	}
	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	content := apiResp.Choices[0].Message.Content

	// Parse the JSON returned by the model into a ResponseConfigInput.
	var input models.ResponseConfigInput
	if err := json.Unmarshal([]byte(content), &input); err != nil {
		return nil, fmt.Errorf("model returned invalid JSON: %w — raw: %s", err, content)
	}

	// Validate and sanitise conditions returned by the model.
	if err := validateConditions(input.Conditions); err != nil {
		return nil, fmt.Errorf("model generated invalid conditions: %w", err)
	}

	// Apply sensible defaults in case the model omitted optional fields.
	if input.StatusCode == 0 {
		input.StatusCode = 200
	}
	if input.Headers == nil {
		input.Headers = map[string]string{"Content-Type": "application/json"}
	}
	if input.Conditions == nil {
		input.Conditions = []models.Condition{}
	}
	if input.Priority == 0 {
		input.Priority = 10
	}
	input.Enabled = true

	return &input, nil
}

// GenerateRuntimeResponse calls the OpenAI API to generate a concrete response
// for a live request. The response is not a reusable config template; it is the
// final HTTP payload that should be returned to the caller and optionally saved
// for replay.
func (g *Generator) GenerateRuntimeResponse(ctx context.Context, op OperationContext, reqCtx RuntimeRequestContext) (*RuntimeResponse, error) {
	if !g.IsConfigured() {
		return nil, fmt.Errorf("OpenAI API key not configured — set ai.openaiApiKey in config.yaml or the GOVIRTUAL_AI_OPENAIAPIKEY environment variable")
	}

	systemPrompt := buildRuntimeSystemPrompt(op, reqCtx.Scenario)
	userMsg := buildRuntimeUserMessage(op, reqCtx)

	reqBody := map[string]any{
		"model": g.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMsg},
		},
		"temperature":     0.3,
		"response_format": map[string]string{"type": "json_object"},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.cfg.APIKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode OpenAI response: %w", err)
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("OpenAI error: %s", apiResp.Error.Message)
	}
	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	content := apiResp.Choices[0].Message.Content

	var wrapper struct {
		StatusCode int               `json:"statusCode"`
		Headers    map[string]string `json:"headers"`
		Body       any               `json:"body"`
	}
	if err := json.Unmarshal([]byte(content), &wrapper); err != nil {
		return nil, fmt.Errorf("model returned invalid JSON: %w — raw: %s", err, content)
	}

	body, err := stringifyRuntimeBody(wrapper.Body)
	if err != nil {
		return nil, err
	}

	if reqCtx.Scenario != nil && reqCtx.Scenario.StatusCode > 0 {
		wrapper.StatusCode = reqCtx.Scenario.StatusCode
	} else if wrapper.StatusCode == 0 {
		wrapper.StatusCode = defaultRuntimeStatusCode(op)
	}
	if wrapper.Headers == nil {
		wrapper.Headers = map[string]string{}
	}
	if strings.TrimSpace(body) != "" && wrapper.Headers["Content-Type"] == "" {
		wrapper.Headers["Content-Type"] = "application/json"
	}

	return &RuntimeResponse{
		StatusCode: wrapper.StatusCode,
		Headers:    wrapper.Headers,
		Body:       body,
	}, nil
}

// ChatMessage represents a single turn in a conversation with the model.
type ChatMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"` // message text
}

// ScriptContext provides context for Starlark script generation.
type ScriptContext struct {
	// Optional: describe what the script should do in the context of an operation.
	OperationMethod  string
	OperationPath    string
	OperationSummary string
	// Inputs from the operation spec (path/query params, body fields).
	Inputs *OperationInputs
}

// GenerateScript calls the OpenAI API and returns Starlark source code for a
// script. priorMessages is the conversation history from previous turns (may be
// nil for the first call). currentSource is the script that is currently in the
// editor (empty on the first call); the model uses it as a starting point for
// modifications. userPrompt describes what the script should do.
func (g *Generator) GenerateScript(ctx context.Context, sctx ScriptContext, priorMessages []ChatMessage, currentSource, userPrompt string) (string, error) {
	if !g.IsConfigured() {
		return "", fmt.Errorf("OpenAI API key not configured — set ai.openaiApiKey in config.yaml or the GOVIRTUAL_AI_OPENAIAPIKEY environment variable")
	}

	systemPrompt := buildScriptSystemPrompt(sctx)
	userMsg := buildScriptUserMessage(sctx, currentSource, userPrompt)

	// Build the messages array: system + conversation history + new user turn.
	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}
	for _, m := range priorMessages {
		messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": userMsg})

	reqBody := map[string]any{
		"model":           g.cfg.Model,
		"messages":        messages,
		"temperature":     0.5,
		"response_format": map[string]string{"type": "json_object"},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.cfg.APIKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("failed to decode OpenAI response: %w", err)
	}
	if apiResp.Error != nil {
		return "", fmt.Errorf("OpenAI error: %s", apiResp.Error.Message)
	}
	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI returned no choices")
	}

	content := apiResp.Choices[0].Message.Content

	// The model wraps the script in {"source": "..."} per the system prompt.
	var wrapper struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal([]byte(content), &wrapper); err != nil {
		return "", fmt.Errorf("model returned invalid JSON: %w — raw: %s", err, content)
	}
	if strings.TrimSpace(wrapper.Source) == "" {
		return "", fmt.Errorf("model returned an empty script")
	}
	return wrapper.Source, nil
}

// buildScriptSystemPrompt builds the system prompt for script generation.
func buildScriptSystemPrompt(sctx ScriptContext) string {
	var sb strings.Builder
	sb.WriteString(`You are an expert Starlark script writer for go-virtual, an API virtualization service.

go-virtual executes Starlark scripts during request handling. Each script must define a top-level
function "run(req)" that is called once per matching request.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
STARLARK LANGUAGE CONSTRAINTS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
- Starlark is a Python-like deterministic scripting language.
- NO import statements — there is no standard library.
- NO classes (no "class" keyword).
- NO global mutable state (use "store" builtin for persistence).
- Supported types: bool, int, float, string, list, dict, None.
- String methods: .upper(), .lower(), .strip(), .split(), .startswith(), .endswith(), .replace(), .format()
- Math: standard +, -, *, /, //, %, ** operators; abs(), min(), max(), len()
- Dict methods: .get(key, default), .keys(), .values(), .items(), .update(), .pop()
- List methods: .append(), .extend(), .insert(), .remove(), .pop(), .index(), .count(), .sort(), .reverse()
- Type conversion: str(), int(), float(), bool()
- The "in" operator works for strings, lists, and dicts.
- Conditionals and loops are standard Python syntax.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
ENTRY POINT
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Every script MUST define:

    def run(req):
        # ... your logic here ...
        return result

The return value is stored under the binding's outputKey and is accessible in response
templates as {{.script.<outputKey>.<field>}} or {{.script.<outputKey>}} for scalar values.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
REQUEST OBJECT — req
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
req is a dict with these keys:

  req["path"]    → dict of path parameters    e.g. req["path"]["id"]
  req["query"]   → dict of query parameters   e.g. req["query"]["status"]
  req["header"]  → dict of request headers (all keys lowercased)
                   e.g. req["header"]["authorization"]
  req["body"]    → parsed JSON body as a Starlark dict/list, or None if no body

Examples:
  pet_id  = req["path"].get("petId", "")
  status  = req["query"].get("status", "available")
  token   = req["header"].get("authorization", "")
  name    = req["body"].get("name", "") if req["body"] != None else ""

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
STORE BUILTIN — store
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
"store" is a session-scoped key-value store. Data written here persists across
requests within the same session (identified by X-Virtual-Session-Id header).

  store.get("key")            → value, or None
  store.get("key", default)   → value, or default if not found
  store.set("key", value)     → None  (value can be any Starlark type)
  store.has("key")            → bool
  store.delete("key")         → None
  store.keys()                → list of all keys in this session

Example — simple counter:
  count = store.get("visit_count", 0)
  store.set("visit_count", count + 1)
  return {"visits": count + 1}

Example — accumulate list:
  items = store.get("cart", [])
  body = req["body"]
  if body != None:
      items.append(body.get("item"))
      store.set("cart", items)
  return {"cart": items}

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
LOG BUILTIN — log(...)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
log() appends a message to the request trace log. Accepts any number of arguments.

  log("processing request for", req["path"].get("id"))
  log("cart size:", len(items))

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
COMPLETE EXAMPLES
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Example 1 — return data based on path param:
  def run(req):
      pet_id = req["path"].get("petId", "unknown")
      log("fetching pet", pet_id)
      return {
          "id": pet_id,
          "name": "Fluffy",
          "status": "available",
      }

Example 2 — use store to track request count:
  def run(req):
      count = store.get("count", 0)
      store.set("count", count + 1)
      return {"requestNumber": count + 1}

Example 3 — conditional logic based on query param:
  def run(req):
      status = req["query"].get("status", "available")
      if status == "sold":
          return {"found": False, "message": "No sold pets available"}
      return {"found": True, "status": status}

Example 4 — read and validate request body:
  def run(req):
      body = req["body"]
      if body == None:
          return {"error": "missing body"}
      name = body.get("name", "")
      if name == "":
          return {"error": "name is required"}
      id = store.get("next_id", 1)
      store.set("next_id", id + 1)
      store.set("pet_" + str(id), {"id": id, "name": name})
      return {"id": id, "name": name, "status": "available"}

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
OUTPUT FORMAT
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Return ONLY a JSON object with a single "source" key containing the complete Starlark script as a string.
Do NOT include any explanation, markdown fences, or extra fields.

COMMENT BLOCK RULES (mandatory):
Every script MUST start with a comment block that explains the CURRENT version's logic.
On every generation or refinement, REWRITE this comment block from scratch to accurately
describe what the script does right now — do NOT keep stale comments from a previous version.
The comment block must cover:
  1. One-line summary (what the script does)
  2. Inputs used: which req fields are read and why
  3. Store usage: any keys read or written (or "No store usage" if none)
  4. Return value: what the dict/value represents

Format:
  # <one-line summary>
  #
  # Inputs:  <e.g. req["path"]["petId"] — the pet to look up>
  # Store:   <e.g. reads "pet_{id}", writes "next_id">  or  # Store:   none
  # Returns: <e.g. {"id", "name", "status"} — the matched pet>

Example output format:
{"source": "# Return a pet by ID from the session store.\n#\n# Inputs:  req[\"path\"][\"petId\"] — ID of the pet\n# Store:   reads \"pet_{id}\"\n# Returns: {\"id\", \"name\", \"status\"} or {\"error\"} if not found\n\ndef run(req):\n    pet_id = req[\"path\"].get(\"petId\", \"\")\n    pet = store.get(\"pet_\" + pet_id)\n    if pet == None:\n        return {\"error\": \"not found\"}\n    return pet\n"}`)

	// Include operation inputs if available.
	if sctx.Inputs != nil {
		hasInputs := len(sctx.Inputs.PathParams) > 0 || len(sctx.Inputs.QueryParams) > 0 || len(sctx.Inputs.BodyFields) > 0
		if hasInputs {
			sb.WriteString("\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			sb.WriteString("AVAILABLE REQUEST INPUTS FOR THIS OPERATION\n")
			sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			for _, p := range sctx.Inputs.PathParams {
				fmt.Fprintf(&sb, "\n  req[\"path\"][\"%s\"]    type=%s", p.Name, p.Type)
				if p.Description != "" {
					fmt.Fprintf(&sb, "  // %s", p.Description)
				}
			}
			for _, p := range sctx.Inputs.QueryParams {
				req := ""
				if p.Required {
					req = " (required)"
				}
				fmt.Fprintf(&sb, "\n  req[\"query\"][\"%s\"]   type=%s%s", p.Name, p.Type, req)
				if p.Description != "" {
					fmt.Fprintf(&sb, "  // %s", p.Description)
				}
			}
			if len(sctx.Inputs.BodyFields) > 0 {
				sb.WriteString("\n  Request body fields (access via req[\"body\"].get(...)):")
				for _, f := range sctx.Inputs.BodyFields {
					fmt.Fprintf(&sb, "\n    %-30s type=%s", f.GjsonPath, f.Type)
					if f.Description != "" {
						fmt.Fprintf(&sb, "  // %s", f.Description)
					}
				}
			}
		}
	}

	return sb.String()
}

// buildScriptUserMessage builds the user message for script generation.
// On the first turn currentSource is empty. On subsequent turns it contains
// the script currently in the editor so the model refines it rather than
// starting from scratch.
func buildScriptUserMessage(sctx ScriptContext, currentSource, userPrompt string) string {
	var sb strings.Builder
	if sctx.OperationMethod != "" {
		fmt.Fprintf(&sb, "API operation: %s %s", sctx.OperationMethod, sctx.OperationPath)
		if sctx.OperationSummary != "" {
			fmt.Fprintf(&sb, " — %s", sctx.OperationSummary)
		}
		sb.WriteString("\n\n")
	}
	if strings.TrimSpace(currentSource) != "" {
		sb.WriteString("Current script (modify/extend this unless the task requires a complete rewrite):\n```\n")
		sb.WriteString(strings.TrimSpace(currentSource))
		sb.WriteString("\n```\n\n")
	}
	sb.WriteString("Task: ")
	sb.WriteString(userPrompt)
	return sb.String()
}

// buildSystemPrompt creates the fixed system prompt for the model.
func buildSystemPrompt(op OperationContext) string {
	var sb strings.Builder
	sb.WriteString(`You are an expert API mock-response generator for go-virtual, an API virtualization service.

Your task is to generate a single realistic mock response configuration for the API operation described by the user.

You MUST return a JSON object with EXACTLY these fields:
{
  "name":        string  — concise label, e.g. "Success", "Created", "Not Found", "Invalid Input",
  "description": string  — one sentence explaining when this response is returned,
  "statusCode":  number  — appropriate HTTP status code (200, 201, 400, 404, 422, 500, …),
  "headers":     object  — at minimum {"Content-Type": "application/json"},
  "body":        string  — the response body as a JSON string (the JSON object serialised to a string),
  "priority":    number  — use 10 unless conditions require a higher priority (lower number = higher priority),
  "enabled":     boolean — use true,
  "conditions":  array   — list of condition objects (see schema below); use [] if no conditions are needed,
  "delay":       number  — use 0
}

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
CONDITION SCHEMA
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Each element of "conditions" must be:
{
  "source":   one of: "path" | "query" | "header" | "body"
  "key":      parameter name, header name, or gjson path (for body)
  "operator": "eq" | "ne" | "contains" | "notContains" | "regex" |
              "exists" | "notExists" | "gt" | "lt" | "gte" | "lte" |
              "startsWith" | "endsWith"
  "value":    comparison value as string (leave "" for exists/notExists)
}

GJSON PATH SYNTAX (used ONLY for source="body"):
  - Uses dot notation. NO leading "$" or "@" — this is NOT JSONPath RFC 9535.
  - Simple field:          "id"           → body.id
  - Nested field:          "user.id"      → body.user.id
  - Array element:         "items.0"      → first element of items array
  - Nested in array item:  "items.0.id"   → id field of first item
  - WRONG (never use):     "$.id"  "body.id"  "/id"  "$['id']"

CONDITION EXAMPLES:
  Path param {id} equals "42":
    {"source":"path","key":"id","operator":"eq","value":"42"}
  Query param ?status=active:
    {"source":"query","key":"status","operator":"eq","value":"active"}
  Header Authorization exists:
    {"source":"header","key":"Authorization","operator":"exists","value":""}
  Body field "id" equals 100:
    {"source":"body","key":"id","operator":"eq","value":"100"}
  Nested body field "user.role" equals "admin":
    {"source":"body","key":"user.role","operator":"eq","value":"admin"}

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
RULES
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
- "body" field in the response MUST be a string (stringify your JSON payload).
- Use realistic fake data (names, UUIDs, ISO-8601 dates, URLs, numbers).
- ALWAYS honour the spec-defined response structure provided below.
- For body conditions use ONLY the gjson paths listed in the "Request body fields" section.
- Add conditions ONLY when the user asks for conditional behaviour.
- When conditions are present, lower the priority (e.g. 5) so they match before unconditional responses.
- Return ONLY the raw JSON object — no markdown fences, no explanation.`)

	// ── Spec-defined responses (body shapes per status code) ──────────────────
	if len(op.SpecResponses) > 0 {
		sb.WriteString("\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString("SPEC-DEFINED RESPONSES (match the body structure for the chosen status code)\n")
		sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, r := range op.SpecResponses {
			label := fmt.Sprintf("%d", r.StatusCode)
			if r.StatusCode == 0 {
				label = "default"
			}
			fmt.Fprintf(&sb, "\n  [%s]", label)
			if r.Description != "" {
				fmt.Fprintf(&sb, " — %s", r.Description)
			}
			if r.BodyExample != "" {
				fmt.Fprintf(&sb, "\n    Example body: %s", r.BodyExample)
			} else if r.SchemaHint != "" {
				fmt.Fprintf(&sb, "\n    Schema: %s", r.SchemaHint)
			}
		}
	} else if op.ExampleResponse != nil && strings.TrimSpace(op.ExampleResponse.Body) != "" {
		sb.WriteString("\n\nSpec example response body (use as data-shape reference):\n")
		sb.WriteString(op.ExampleResponse.Body)
	}

	// ── Request inputs (path params, query params, body fields) ───────────────
	if op.Inputs != nil {
		hasInputs := len(op.Inputs.PathParams) > 0 || len(op.Inputs.QueryParams) > 0 || len(op.Inputs.BodyFields) > 0
		if hasInputs {
			sb.WriteString("\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			sb.WriteString("AVAILABLE REQUEST INPUTS (use these to build conditions)\n")
			sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			for _, p := range op.Inputs.PathParams {
				fmt.Fprintf(&sb, "\n  path   %-20s type=%s", p.Name, p.Type)
				if p.Description != "" {
					fmt.Fprintf(&sb, "  // %s", p.Description)
				}
			}
			for _, p := range op.Inputs.QueryParams {
				req := ""
				if p.Required {
					req = " (required)"
				}
				fmt.Fprintf(&sb, "\n  query  %-20s type=%s%s", p.Name, p.Type, req)
				if p.Description != "" {
					fmt.Fprintf(&sb, "  // %s", p.Description)
				}
			}
			if len(op.Inputs.BodyFields) > 0 {
				sb.WriteString("\n  Request body fields (use gjson dot-path as key for source=\"body\"):")
				for _, f := range op.Inputs.BodyFields {
					fmt.Fprintf(&sb, "\n    %-30s type=%s", f.GjsonPath, f.Type)
					if f.Description != "" {
						fmt.Fprintf(&sb, "  // %s", f.Description)
					}
				}
			}
		}
	}

	return sb.String()
}

// buildUserMessage constructs the per-request user message.
func buildUserMessage(op OperationContext, userPrompt string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "API operation:\n  Method: %s\n  Path:   %s\n", op.Method, op.Path)
	if op.Summary != "" {
		fmt.Fprintf(&sb, "  Summary: %s\n", op.Summary)
	}
	if op.Description != "" {
		fmt.Fprintf(&sb, "  Description: %s\n", op.Description)
	}
	if userPrompt != "" {
		fmt.Fprintf(&sb, "\nUser instructions:\n%s", userPrompt)
	} else {
		sb.WriteString("\nGenerate a successful response with realistic fake data.")
	}
	return sb.String()
}

func buildRuntimeSystemPrompt(op OperationContext, scenario *RuntimeScenario) string {
	var sb strings.Builder
	sb.WriteString(`You are an expert runtime response generator for go-virtual, an API virtualization service.

You are generating the final HTTP response for a live incoming request.

Return ONLY a JSON object with EXACTLY these fields:
{
  "statusCode": number,
  "headers": object,
  "body": object | array | string
}

Rules:
- Prefer a successful status code unless the request context clearly implies an error response.
- The body MUST match the spec-defined response schema or example for the chosen status code.
- Use the incoming request as context so the response feels consistent with the request inputs.
- When a JSON response is appropriate, return "body" as a JSON object/array, not a stringified JSON blob.
- Keep headers minimal; include Content-Type: application/json for JSON responses.
- Return raw JSON only. No markdown. No explanations.`)

	if scenario != nil {
		sb.WriteString("\n\nRuntime scenario requirements:")
		fmt.Fprintf(&sb, "\n- Scenario name: %s", scenario.Name)
		fmt.Fprintf(&sb, "\n- Response kind: %s", scenario.ResponseKind)
		if scenario.UseDefaultSuccessStatus {
			sb.WriteString("\n- Status code: use the operation's default success status")
		} else if scenario.StatusCode > 0 {
			fmt.Fprintf(&sb, "\n- Status code: %d", scenario.StatusCode)
		}
		if scenario.Count > 0 {
			fmt.Fprintf(&sb, "\n- Item count: return exactly %d top-level entries when generating a list response", scenario.Count)
		}
		if scenario.Instructions != "" {
			fmt.Fprintf(&sb, "\n- Additional instructions: %s", scenario.Instructions)
		}
	}

	if len(op.SpecResponses) > 0 {
		sb.WriteString("\n\nSpec-defined responses:")
		for _, r := range op.SpecResponses {
			label := fmt.Sprintf("%d", r.StatusCode)
			if r.StatusCode == 0 {
				label = "default"
			}
			fmt.Fprintf(&sb, "\n  [%s]", label)
			if r.Description != "" {
				fmt.Fprintf(&sb, " — %s", r.Description)
			}
			if r.BodyExample != "" {
				fmt.Fprintf(&sb, "\n    Example body: %s", r.BodyExample)
			} else if r.SchemaHint != "" {
				fmt.Fprintf(&sb, "\n    Schema: %s", r.SchemaHint)
			}
		}
	} else if op.ExampleResponse != nil && strings.TrimSpace(op.ExampleResponse.Body) != "" {
		sb.WriteString("\n\nFallback example response body:\n")
		sb.WriteString(op.ExampleResponse.Body)
	}

	return sb.String()
}

func buildRuntimeUserMessage(op OperationContext, reqCtx RuntimeRequestContext) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "API operation:\n  Method: %s\n  Path:   %s\n", op.Method, op.Path)
	if op.Summary != "" {
		fmt.Fprintf(&sb, "  Summary: %s\n", op.Summary)
	}
	if op.Description != "" {
		fmt.Fprintf(&sb, "  Description: %s\n", op.Description)
	}
	if reqCtx.Signature != "" {
		fmt.Fprintf(&sb, "\nRequest signature: %s\n", reqCtx.Signature)
	}
	if reqCtx.Scenario != nil {
		fmt.Fprintf(&sb, "\nRequested AI scenario: %s\n", reqCtx.Scenario.Name)
		if reqCtx.Scenario.Description != "" {
			fmt.Fprintf(&sb, "  Description: %s\n", reqCtx.Scenario.Description)
		}
		fmt.Fprintf(&sb, "  Response kind: %s\n", reqCtx.Scenario.ResponseKind)
		if reqCtx.Scenario.UseDefaultSuccessStatus {
			sb.WriteString("  Status code: use default success status\n")
		} else if reqCtx.Scenario.StatusCode > 0 {
			fmt.Fprintf(&sb, "  Status code: %d\n", reqCtx.Scenario.StatusCode)
		}
		if reqCtx.Scenario.Count > 0 {
			fmt.Fprintf(&sb, "  Count: %d\n", reqCtx.Scenario.Count)
		}
		if reqCtx.Scenario.Instructions != "" {
			fmt.Fprintf(&sb, "  Instructions: %s\n", reqCtx.Scenario.Instructions)
		}
	}
	fmt.Fprintf(&sb, "\nIncoming request context:\n  Path params:  %#v\n  Query params: %#v\n  Headers:      %#v\n", reqCtx.PathParams, reqCtx.QueryParams, reqCtx.Headers)
	if strings.TrimSpace(reqCtx.Body) != "" {
		fmt.Fprintf(&sb, "  Body:         %s\n", reqCtx.Body)
	} else {
		sb.WriteString("  Body:         <empty>\n")
	}
	if op.Inputs != nil {
		sb.WriteString("\nKnown request inputs from spec:")
		for _, p := range op.Inputs.PathParams {
			fmt.Fprintf(&sb, "\n  path   %s (%s)", p.Name, p.Type)
		}
		for _, p := range op.Inputs.QueryParams {
			fmt.Fprintf(&sb, "\n  query  %s (%s)", p.Name, p.Type)
		}
		for _, f := range op.Inputs.BodyFields {
			fmt.Fprintf(&sb, "\n  body   %s (%s)", f.GjsonPath, f.Type)
		}
	}
	return sb.String()
}

func stringifyRuntimeBody(body any) (string, error) {
	switch v := body.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("model returned an invalid body: %w", err)
		}
		return string(data), nil
	}
}

func defaultRuntimeStatusCode(op OperationContext) int {
	for _, resp := range op.SpecResponses {
		if resp.StatusCode > 0 {
			return resp.StatusCode
		}
	}
	if op.ExampleResponse != nil && op.ExampleResponse.StatusCode > 0 {
		return op.ExampleResponse.StatusCode
	}
	return 200
}

var validSources = map[string]bool{
	"path": true, "query": true, "header": true, "body": true,
}

var validOperators = map[string]bool{
	"eq": true, "ne": true, "contains": true, "notContains": true,
	"regex": true, "exists": true, "notExists": true,
	"gt": true, "lt": true, "gte": true, "lte": true,
	"startsWith": true, "endsWith": true,
}

// validateConditions returns an error if any condition has an invalid source or operator.
func validateConditions(conditions []models.Condition) error {
	for i, c := range conditions {
		if !validSources[c.Source] {
			return fmt.Errorf("condition[%d]: invalid source %q (valid: path, query, header, body)", i, c.Source)
		}
		if !validOperators[c.Operator] {
			return fmt.Errorf("condition[%d]: invalid operator %q", i, c.Operator)
		}
		if c.Key == "" {
			return fmt.Errorf("condition[%d]: key must not be empty", i)
		}
	}
	return nil
}
