package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/ai"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/parser"
)

// GenerateAIResponse calls the OpenAI API to generate a response config for an
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
			"error": "AI generation is not configured — set ai.openaiApiKey in config.yaml or the GOVIRTUAL_AI_OPENAIAPIKEY environment variable",
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
		ID:          generateID(),
		OperationID: opID,
		Name:        input.Name,
		Description: input.Description,
		Tag:         normalizeTag(input.Tag),
		Priority:    input.Priority,
		Conditions:  input.Conditions,
		StatusCode:  input.StatusCode,
		Headers:     input.Headers,
		Body:        input.Body,
		Delay:       input.Delay,
		Enabled:     input.Enabled,
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

// extractSpecResponses retrieves all response definitions from the spec for the
// operation's path+method. Returns nil (non-fatal) on any error.
func extractSpecResponses(h *Handler, op *models.Operation) []ai.SpecResponseDef {
	spec, err := h.store.GetSpec(op.SpecID)
	if err != nil || spec.Content == "" {
		return nil
	}

	p := parser.NewParser()
	defs, err := p.ExtractAllResponses(spec.Content, op.Method, op.Path)
	if err != nil {
		log.Printf("ai: failed to extract spec responses for %s %s: %v", op.Method, op.Path, err)
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
		log.Printf("ai: failed to extract operation inputs for %s %s: %v", op.Method, op.Path, err)
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
