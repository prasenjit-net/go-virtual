package ai

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/prasenjit/go-virtual/internal/models"
)

//go:embed prompts/script_system.txt
var scriptSystemPromptBase string

//go:embed prompts/response_system.txt
var responseSystemPromptBase string

//go:embed prompts/runtime_system.txt
var runtimeSystemPromptBase string

// Generator uses the configured AI provider to generate mock response configurations.
type Generator struct {
	cfg      Config
	provider completionProvider
}

// NewGenerator creates a new Generator with the given config.
func NewGenerator(cfg Config) *Generator {
	cfg = cfg.Normalize()
	client := newHTTPClient()
	return &Generator{
		cfg:      cfg,
		provider: newCompletionProvider(cfg, client),
	}
}

// IsConfigured returns true if the generator has a valid API key.
func (g *Generator) IsConfigured() bool {
	return g.provider != nil && g.provider.IsConfigured()
}

// Status returns the selected provider and whether it is configured.
func (g *Generator) Status() Status {
	if g == nil || g.provider == nil {
		return Status{Configured: false, Provider: openAIProviderName}
	}
	return Status{
		Configured: g.provider.IsConfigured(),
		Provider:   g.provider.Name(),
		Model:      g.provider.Model(),
	}
}

// MissingConfigMessage reports how to configure the currently selected provider.
func (g *Generator) MissingConfigMessage() string {
	if g == nil || g.provider == nil {
		return `AI generation is not configured — set ai.provider and the selected provider credentials in config.yaml`
	}
	return g.provider.MissingConfigMessage()
}

// ProviderDisplayName returns the selected provider name for UI and validation messages.
func (g *Generator) ProviderDisplayName() string {
	return titleProvider(g.Status().Provider)
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

// GenerateResponse calls the configured AI provider and returns a ResponseConfigInput
// populated with realistic fake data. userPrompt may be empty.
func (g *Generator) GenerateResponse(ctx context.Context, op OperationContext, userPrompt string) (*models.ResponseConfigInput, error) {
	if !g.IsConfigured() {
		return nil, fmt.Errorf("%s", g.MissingConfigMessage())
	}

	systemPrompt := buildSystemPrompt(op)
	userMsg := buildUserMessage(op, userPrompt)
	content, err := g.provider.Complete(ctx, providerRequest{
		SystemPrompt: systemPrompt,
		Messages:     []ChatMessage{{Role: "user", Content: userMsg}},
		Temperature:  0.7,
	})
	if err != nil {
		return nil, err
	}

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

// GenerateRuntimeResponse calls the configured AI provider to generate a concrete response
// for a live request. The response is not a reusable config template; it is the
// final HTTP payload that should be returned to the caller and optionally saved
// for replay.
func (g *Generator) GenerateRuntimeResponse(ctx context.Context, op OperationContext, reqCtx RuntimeRequestContext) (*RuntimeResponse, error) {
	if !g.IsConfigured() {
		return nil, fmt.Errorf("%s", g.MissingConfigMessage())
	}

	systemPrompt := buildRuntimeSystemPrompt(op, reqCtx.Scenario)
	userMsg := buildRuntimeUserMessage(op, reqCtx)
	content, err := g.provider.Complete(ctx, providerRequest{
		SystemPrompt: systemPrompt,
		Messages:     []ChatMessage{{Role: "user", Content: userMsg}},
		Temperature:  0.3,
	})
	if err != nil {
		return nil, err
	}

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

// GenerateScript calls the configured AI provider and returns Starlark source code for a
// script. priorMessages is the conversation history from previous turns (may be
// nil for the first call). currentSource is the script that is currently in the
// editor (empty on the first call); the model uses it as a starting point for
// modifications. userPrompt describes what the script should do.
func (g *Generator) GenerateScript(ctx context.Context, sctx ScriptContext, priorMessages []ChatMessage, currentSource, userPrompt string) (string, error) {
	if !g.IsConfigured() {
		return "", fmt.Errorf("%s", g.MissingConfigMessage())
	}

	systemPrompt := buildScriptSystemPrompt(sctx)
	userMsg := buildScriptUserMessage(sctx, currentSource, userPrompt)
	messages := append([]ChatMessage{}, priorMessages...)
	messages = append(messages, ChatMessage{Role: "user", Content: userMsg})
	content, err := g.provider.Complete(ctx, providerRequest{
		SystemPrompt: systemPrompt,
		Messages:     messages,
		Temperature:  0.5,
	})
	if err != nil {
		return "", err
	}

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
	sb.WriteString(scriptSystemPromptBase)

	// Include operation inputs if available.
	if sctx.Inputs != nil {
		hasInputs := len(sctx.Inputs.PathParams) > 0 || len(sctx.Inputs.QueryParams) > 0 || len(sctx.Inputs.BodyFields) > 0
		if hasInputs {
			sb.WriteString("\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			sb.WriteString("AVAILABLE REQUEST INPUTS FOR THIS OPERATION\n")
			sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			for _, p := range sctx.Inputs.PathParams {
				fmt.Fprintf(&sb, "\n  req.path(%q)    type=%s", p.Name, p.Type)
				if p.Description != "" {
					fmt.Fprintf(&sb, "  // %s", p.Description)
				}
			}
			for _, p := range sctx.Inputs.QueryParams {
				req := ""
				if p.Required {
					req = " (required)"
				}
				fmt.Fprintf(&sb, "\n  req.query(%q)   type=%s%s", p.Name, p.Type, req)
				if p.Description != "" {
					fmt.Fprintf(&sb, "  // %s", p.Description)
				}
			}
			if len(sctx.Inputs.BodyFields) > 0 {
				sb.WriteString("\n  Request body fields (access via req.body(\"path\", default)):")
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
	sb.WriteString(responseSystemPromptBase)

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
	sb.WriteString(runtimeSystemPromptBase)

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
	"path": true, "query": true, "header": true, "body": true, "signature": true, "script": true,
}

var validOperators = map[string]bool{
	"eq": true, "ne": true, "contains": true, "notContains": true,
	"regex": true, "exists": true, "notExists": true,
	"gt": true, "lt": true, "gte": true, "lte": true,
	"startsWith": true, "endsWith": true,
	// date operators
	"dateEq": true, "dateBefore": true, "dateAfter": true,
	"dateLte": true, "dateGte": true,
	"dateInPast": true, "dateInFuture": true,
	"dateToday": true, "dateBetween": true,
}

// validateConditions returns an error if any condition has an invalid source or operator.
func validateConditions(conditions []models.Condition) error {
	for i, c := range conditions {
		if !validSources[c.Source] {
			return fmt.Errorf("condition[%d]: invalid source %q (valid: path, query, header, body, signature, script)", i, c.Source)
		}
		if !validOperators[c.Operator] {
			return fmt.Errorf("condition[%d]: invalid operator %q", i, c.Operator)
		}
		if c.Key == "" && c.Source != "signature" {
			return fmt.Errorf("condition[%d]: key must not be empty", i)
		}
	}
	return nil
}
