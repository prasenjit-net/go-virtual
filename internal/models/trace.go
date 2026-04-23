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

	// Session identifies the session that was active during this request.
	// Populated in Phase 2 when session management is enabled.
	Session *SessionTrace `json:"session,omitempty"`
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
