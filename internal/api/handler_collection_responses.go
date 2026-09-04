package api

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/prasenjit/go-virtual/internal/collection"
	"github.com/prasenjit/go-virtual/internal/models"
)

// previewRequestInput carries example request values the preview endpoint
// uses in place of a real HTTP request.
type previewRequestInput struct {
	PathParams  map[string]string   `json:"pathParams"`
	QueryParams map[string][]string `json:"queryParams"`
	Headers     map[string][]string `json:"headers"`
	Body        string              `json:"body"`
}

// collectionResponsePreviewInput is the payload for the preview endpoint: an
// unsaved (or being-edited) collection response definition plus example
// request values.
type collectionResponsePreviewInput struct {
	StatusCode         int                              `json:"statusCode"`
	CollectionResponse *models.CollectionResponseConfig `json:"collectionResponse"`
	Request            previewRequestInput              `json:"request"`
}

// collectionResponsePreviewResult reports the resolved filter, match
// outcome, matched record count, rendered body, and any fill/resolve
// warnings — without mutating collection or session state.
type collectionResponsePreviewResult struct {
	RootKind       models.RootKind `json:"rootKind"`
	TemplateSource string          `json:"templateSource"`
	Matched        bool            `json:"matched"`
	RecordCount    int             `json:"recordCount"`
	Filter         map[string]any  `json:"filter,omitempty"`
	Body           string          `json:"body,omitempty"`
	Warnings       []string        `json:"warnings,omitempty"`
}

// PreviewCollectionResponse resolves and renders a Collection Response
// definition against example request values without persisting anything or
// mutating collection/session state.
// POST /_api/operations/:id/collection-responses/preview
func (h *Handler) PreviewCollectionResponse(c *gin.Context) {
	opID := c.Param("id")
	op, err := h.store.GetOperation(opID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "operation not found"})
		return
	}

	if h.collResponseSvc == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collection responses require a collection backend to be configured"})
		return
	}

	var input collectionResponsePreviewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	statusCode := input.StatusCode
	if statusCode == 0 {
		statusCode = 200
	}

	if errs := h.validateResponseKind(op, statusCode, models.ResponseKindCollection, "", input.CollectionResponse); len(errs) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": strings.Join(errs, "; "), "fieldErrors": errs})
		return
	}

	previewCfg := &models.ResponseConfig{
		ID:                 "preview",
		OperationID:        opID,
		StatusCode:         statusCode,
		Kind:               models.ResponseKindCollection,
		CollectionResponse: input.CollectionResponse,
	}

	headers := http.Header{}
	for k, vals := range input.Request.Headers {
		for _, v := range vals {
			headers.Add(k, v)
		}
	}
	queryParams := url.Values{}
	for k, vals := range input.Request.QueryParams {
		for _, v := range vals {
			queryParams.Add(k, v)
		}
	}
	typedReq := &collection.TypedRequestContext{
		PathParams:  input.Request.PathParams,
		QueryParams: map[string][]string(queryParams),
		Headers:     headers,
		Body:        input.Request.Body,
	}

	sess := previewSession{}
	match, err := h.collResponseSvc.TryMatch(op, previewCfg, typedReq, sess)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := collectionResponsePreviewResult{
		RootKind:       match.RootKind,
		TemplateSource: string(match.Template.Source),
		Matched:        match.Matched,
		RecordCount:    match.RecordCount,
		Filter:         match.Filter,
	}

	if match.Matched {
		render, err := h.collResponseSvc.Render(previewCfg, match, typedReq, sess)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		result.Body = string(render.Body)
		result.Warnings = render.Warnings
	}

	c.JSON(http.StatusOK, result)
}

// previewSession is a minimal in-memory session.SessionState used only for
// preview: it lets the collection ops layer run reads without requiring a
// real request session, and every write it might make (none, for find-one/
// find-many) is discarded with the request.
type previewSession map[string]any

func (s previewSession) Get(key string) (any, bool)      { v, ok := s[key]; return v, ok }
func (s previewSession) Set(key string, value any) error { s[key] = value; return nil }
func (s previewSession) Has(key string) bool             { _, ok := s[key]; return ok }
func (s previewSession) Delete(key string) error         { delete(s, key); return nil }
func (s previewSession) Keys() []string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	return keys
}
func (s previewSession) Snapshot() map[string]any { return map[string]any(s) }
func (s previewSession) Info(bool) models.SessionInfo {
	return models.SessionInfo{ID: "preview"}
}
