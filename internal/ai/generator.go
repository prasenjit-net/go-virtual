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
}

// Generator uses OpenAI to generate mock response configurations.
type Generator struct {
	cfg    Config
	client *http.Client
}

// NewGenerator creates a new Generator with the given config.
func NewGenerator(cfg Config) *Generator {
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}
	return &Generator{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIEndpoint, bytes.NewReader(data))
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
