package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/ai"
	"github.com/prasenjit/go-virtual/internal/logging"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/parser"
)

// GetAIStatus returns whether the AI generator is configured.
func (h *Handler) GetAIStatus(c *gin.Context) {
	status := ai.Status{Configured: false, Provider: "openai"}
	if h.aiGenerator != nil {
		status = h.aiGenerator.Status()
	}
	c.JSON(http.StatusOK, status)
}

// GenerateAIResponse calls the configured AI provider to generate a response config for an
// operation and stores it, returning the created ResponseConfig.
func (h *Handler) GenerateAIResponse(c *gin.Context) {
	opID := c.Param("id")

	op, err := h.store.GetOperation(opID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Operation not found"})
		return
	}

	if h.aiGenerator == nil || !h.aiGenerator.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": h.aiUnavailableMessage(),
		})
		return
	}

	var req struct {
		UserPrompt string `json:"userPrompt"`
	}
	// Body is optional; ignore bind errors.
	_ = c.ShouldBindJSON(&req)

	// Extract all spec-defined responses (every status code) so the AI has
	// the correct body shape for whichever status code it generates.
	specResponses := extractSpecResponses(h, op)
	inputs := extractOperationInputs(h, op)

	opCtx := ai.OperationContext{
		Method:          op.Method,
		Path:            op.Path,
		Summary:         op.Summary,
		Description:     op.Description,
		ExampleResponse: op.ExampleResponse,
		SpecResponses:   specResponses,
		Inputs:          inputs,
	}

	input, err := h.aiGenerator.GenerateResponse(c.Request.Context(), opCtx, req.UserPrompt)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	// Persist using the same path as CreateResponseConfig.
	cfg := &models.ResponseConfig{
		ID:            generateID(),
		OperationID:   opID,
		Name:          input.Name,
		Description:   input.Description,
		Tag:           normalizeTag(input.Tag),
		Priority:      input.Priority,
		Conditions:    input.Conditions,
		ConditionTree: input.ConditionTree,
		StatusCode:    input.StatusCode,
		Headers:       input.Headers,
		Body:          input.Body,
		Delay:         input.Delay,
		Enabled:       input.Enabled,
		Origin:        models.ResponseOriginAI,
	}
	if cfg.Tag == "" {
		cfg.Tag = models.DefaultTagName
	}
	if err := h.ensureTagExists(cfg.Tag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if cfg.StatusCode == 0 {
		cfg.StatusCode = 200
	}
	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}
	if cfg.Conditions == nil {
		cfg.Conditions = make([]models.Condition, 0)
	}

	if err := h.store.CreateResponseConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, cfg)
}

// GenerateAIScript calls the configured AI provider to generate a Starlark script and
// returns the source code. The caller can optionally pass an operationId in
// the request body to provide operation context for the AI.
func (h *Handler) GenerateAIScript(c *gin.Context) {
	if h.aiGenerator == nil || !h.aiGenerator.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": h.aiUnavailableMessage(),
		})
		return
	}

	var req struct {
		UserPrompt    string           `json:"userPrompt"`
		OperationID   string           `json:"operationId"`
		CurrentSource string           `json:"currentSource"`
		History       []ai.ChatMessage `json:"history"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserPrompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userPrompt is required"})
		return
	}

	sctx := ai.ScriptContext{}

	// Enrich context if an operation is specified.
	if req.OperationID != "" {
		if op, err := h.store.GetOperation(req.OperationID); err == nil {
			sctx.OperationMethod = op.Method
			sctx.OperationPath = op.Path
			sctx.OperationSummary = op.Summary
			sctx.Inputs = extractOperationInputs(h, op)
		}
	}

	source, err := h.aiGenerator.GenerateScript(c.Request.Context(), sctx, req.History, req.CurrentSource, req.UserPrompt,
		func(src string) error { return h.scriptEngine.CompileAndValidate("ai-gen", src) })
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"source": source})
}

func (h *Handler) aiUnavailableMessage() string {
	if h.aiGenerator == nil {
		return "AI generation is not configured — set ai.provider and the selected provider credentials in config.yaml"
	}
	return h.aiGenerator.MissingConfigMessage()
}

func (h *Handler) aiModeConfigurationError() error {
	if h.aiGenerator == nil {
		return fmt.Errorf("the selected AI provider must be configured before enabling AI mode")
	}
	return fmt.Errorf("%s must be configured before enabling AI mode", h.aiGenerator.ProviderDisplayName())
}

// operation's path+method. Returns nil (non-fatal) on any error.
func extractSpecResponses(h *Handler, op *models.Operation) []ai.SpecResponseDef {
	spec, err := h.store.GetSpec(op.SpecID)
	if err != nil || spec.Content == "" {
		return nil
	}

	p := parser.NewParser()
	defs, err := p.ExtractAllResponses(spec.Content, op.Method, op.Path)
	if err != nil {
		logging.Logger("api.ai").Warn("Failed to extract spec responses for AI context",
			"event", "ai_spec_responses_extract_failed",
			"operation_method", op.Method,
			"operation_path", op.Path,
			"spec_id", op.SpecID,
			"error", err,
		)
		return nil
	}

	result := make([]ai.SpecResponseDef, 0, len(defs))
	for _, d := range defs {
		result = append(result, ai.SpecResponseDef{
			StatusCode:  d.StatusCode,
			Description: d.Description,
			BodyExample: d.BodyExample,
			SchemaHint:  d.SchemaHint,
		})
	}
	return result
}

// extractOperationInputs retrieves path params, query params, and request body
// field definitions from the spec. Returns nil (non-fatal) on any error.
func extractOperationInputs(h *Handler, op *models.Operation) *ai.OperationInputs {
	spec, err := h.store.GetSpec(op.SpecID)
	if err != nil || spec.Content == "" {
		return nil
	}

	p := parser.NewParser()
	inputs, err := p.ExtractOperationInputs(spec.Content, op.Method, op.Path)
	if err != nil {
		logging.Logger("api.ai").Warn("Failed to extract operation inputs for AI context",
			"event", "ai_operation_inputs_extract_failed",
			"operation_method", op.Method,
			"operation_path", op.Path,
			"spec_id", op.SpecID,
			"error", err,
		)
		return nil
	}
	if inputs == nil {
		return nil
	}

	result := &ai.OperationInputs{}
	for _, pp := range inputs.PathParams {
		result.PathParams = append(result.PathParams, ai.ParamDef{
			Name: pp.Name, In: pp.In, Required: pp.Required,
			Type: pp.Type, Description: pp.Description,
		})
	}
	for _, qp := range inputs.QueryParams {
		result.QueryParams = append(result.QueryParams, ai.ParamDef{
			Name: qp.Name, In: qp.In, Required: qp.Required,
			Type: qp.Type, Description: qp.Description,
		})
	}
	for _, bf := range inputs.BodyFields {
		result.BodyFields = append(result.BodyFields, ai.BodyFieldDef{
			GjsonPath: bf.GjsonPath, Type: bf.Type, Description: bf.Description,
		})
	}
	return result
}
