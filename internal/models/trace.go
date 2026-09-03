package models

import (
	"time"
)

const (
	TraceResponseSourceConfig  = "config"
	TraceResponseSourceExample = "example"
	TraceResponseSourceAI      = "ai"
	TraceResponseSourceProxy   = "proxy"

	TraceResponseTierConfigured = "configured"
	TraceResponseTierRecorded   = "recorded"
	TraceResponseTierFallback   = "fallback"
)

// Trace represents a captured request/response trace
type Trace struct {
	ID                  string        `json:"id"`
	SpecID              string        `json:"specId"`
	SpecName            string        `json:"specName"`
	OperationID         string        `json:"operationId"`
	OperationPath       string        `json:"operationPath"`
	Timestamp           time.Time     `json:"timestamp"`
	Duration            int64         `json:"duration"` // Duration in nanoseconds
	Request             TraceRequest  `json:"request"`
	Response            TraceResponse `json:"response"`
	MatchedConfigID     string        `json:"matchedConfigId,omitempty"`
	MatchedConfig       string        `json:"matchedConfig,omitempty"` // Name of matched response config
	MatchedConfigOrigin string        `json:"matchedConfigOrigin,omitempty"`
	Mode                string        `json:"mode,omitempty"`
	ResponseSource      string        `json:"responseSource,omitempty"`
	ResponseTier        string        `json:"responseTier,omitempty"`
	AISkippedReason     string        `json:"aiSkippedReason,omitempty"`
	ProxySkippedReason  string        `json:"proxySkippedReason,omitempty"`
	AIScenarioRequested string        `json:"aiScenarioRequested,omitempty"`
	AIScenarioApplied   string        `json:"aiScenarioApplied,omitempty"`

	// Proxy recording fields – populated when the trace is recorded in proxy mode
	ProxyMode  bool   `json:"proxyMode,omitempty"`  // true when the request was forwarded to a real backend
	Signature  string `json:"signature,omitempty"`  // deterministic hash of the request used for deduplication
	BackendURI string `json:"backendUri,omitempty"` // backend URL that handled the request

	// Script execution traces — one entry per executed binding
	Scripts []ScriptTrace `json:"scripts,omitempty"`
	// Collection execution traces — one entry per executed mapping
	Collections []CollectionTrace `json:"collections,omitempty"`
	// Validation traces — one entry per evaluated validation rule
	Validations []ValidationTrace `json:"validations,omitempty"`

	// Pipeline is the unified execution timeline in step order across all scopes
	// (spec → operation → response). Each item carries one of Script/Validation/Collection.
	// Aborted is set on the validation step that caused a scope abort.
	Pipeline []PipelineTraceItem `json:"pipeline,omitempty"`

	// Session identifies the session that was active during this request.
	// Populated in Phase 2 when session management is enabled.
	Session *SessionTrace `json:"session,omitempty"`

	// CollectionResponseAttempts records every Collection Response evaluated
	// during matching, in priority order, including ones that fell through
	// because their primary query returned no data.
	CollectionResponseAttempts []CollectionResponseAttempt `json:"collectionResponseAttempts,omitempty"`
	// CollectionResponseRender describes how the winning Collection
	// Response's body was produced. Nil for manual responses.
	CollectionResponseRender *CollectionResponseRenderTrace `json:"collectionResponseRender,omitempty"`
}

// CollectionResponseAttempt captures one Collection Response's primary query
// during matching.
type CollectionResponseAttempt struct {
	ResponseConfigID   string         `json:"responseConfigId"`
	ResponseConfigName string         `json:"responseConfigName"`
	CollectionName     string         `json:"collectionName"`
	Mode               QueryMode      `json:"mode"`
	Filter             map[string]any `json:"filter,omitempty"`
	Matched            bool           `json:"matched"`
	RecordCount        int            `json:"recordCount"`
	Error              string         `json:"error,omitempty"`
}

// CollectionResponseRenderTrace describes how a Collection Response's body
// was rendered after it won matching.
type CollectionResponseRenderTrace struct {
	TemplateStatusCode int               `json:"templateStatusCode,omitempty"`
	TemplateSource     string            `json:"templateSource,omitempty"` // "example" | "schema" | "identity"
	AdditionalMappers  []CollectionTrace `json:"additionalMappers,omitempty"`
	Warnings           []string          `json:"warnings,omitempty"`
}

// PipelineTraceItem is one step in the unified pipeline execution timeline.
type PipelineTraceItem struct {
	Type       PipelineStepType `json:"type"`
	Script     *ScriptTrace     `json:"script,omitempty"`
	Validation *ValidationTrace `json:"validation,omitempty"`
	Collection *CollectionTrace `json:"collection,omitempty"`
	// Aborted is true when this validation step caused the remaining scope steps to be skipped.
	Aborted bool `json:"aborted,omitempty"`
}

// TraceRequest represents the captured request
type TraceRequest struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Path    string              `json:"path"`
	Query   map[string][]string `json:"query"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

// TraceResponse represents the captured response
type TraceResponse struct {
	StatusCode int                 `json:"statusCode"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
}

// TraceFilter represents filters for querying traces
type TraceFilter struct {
	SpecID      string    `json:"specId,omitempty"`
	OperationID string    `json:"operationId,omitempty"`
	Method      string    `json:"method,omitempty"`
	Path        string    `json:"path,omitempty"`
	StatusCode  int       `json:"statusCode,omitempty"`
	StartTime   time.Time `json:"startTime,omitempty"`
	EndTime     time.Time `json:"endTime,omitempty"`
	Limit       int       `json:"limit,omitempty"`
	Offset      int       `json:"offset,omitempty"`
}
