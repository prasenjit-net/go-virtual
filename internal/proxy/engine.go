package proxy

import (
	"context"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prasenjit/go-virtual/internal/ai"
	"github.com/prasenjit/go-virtual/internal/condition"
	"github.com/prasenjit/go-virtual/internal/logging"
	"github.com/prasenjit/go-virtual/internal/metrics"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/parser"
	"github.com/prasenjit/go-virtual/internal/scripting"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
	"github.com/prasenjit/go-virtual/internal/template"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

// Engine handles proxying requests to virtual API endpoints
type Engine struct {
	store             storage.Storage
	statsCollector    *stats.Collector
	tracingService    *tracing.Service
	condEvaluator     *condition.Evaluator
	templateEngine    *template.Engine
	scriptEngine      *scripting.ScriptEngine
	sessionManager    store.SessionRegistry // nil when Phase 2 is not configured
	sessionHeaderName string
	recorder          *Recorder
	aiGenerator       *ai.Generator
	mu                sync.RWMutex
	warningMu         sync.Mutex
	runtimeWarnings   map[string]struct{}
	routes            map[string][]*route // method -> routes
}

// route represents a registered route
type route struct {
	spec      *models.Spec
	operation *models.Operation
	pattern   *regexp.Regexp
	paramKeys []string
}

// NewEngine creates a new proxy engine.
// An optional scriptTimeoutMs parameter overrides the default script execution timeout (100ms).
func NewEngine(store storage.Storage, statsCollector *stats.Collector, tracingService *tracing.Service, scriptTimeoutMs ...int) *Engine {
	timeoutMs := 100
	if len(scriptTimeoutMs) > 0 && scriptTimeoutMs[0] > 0 {
		timeoutMs = scriptTimeoutMs[0]
	}

	e := &Engine{
		store:           store,
		statsCollector:  statsCollector,
		tracingService:  tracingService,
		condEvaluator:   condition.NewEvaluator(),
		templateEngine:  template.NewEngine(),
		scriptEngine:    scripting.NewScriptEngine(store, timeoutMs),
		recorder:        NewRecorder(store),
		runtimeWarnings: make(map[string]struct{}),
		routes:          make(map[string][]*route),
	}

	// Load initial routes
	e.ReloadRoutes()

	return e
}

// SetSessionManager attaches a SessionManager to the engine, enabling Phase 2
// session tracking. headerName is the HTTP header used to identify sessions.
func (e *Engine) SetSessionManager(sm store.SessionRegistry, headerName string) {
	e.sessionManager = sm
	e.sessionHeaderName = headerName
}

// SetProxyHTTPClient replaces the HTTP client used by the proxy recorder for
// backend requests. Use proxy.BuildClient to construct a client with mTLS or
// custom CA settings from a ClientConfig.
func (e *Engine) SetProxyHTTPClient(client *http.Client) {
	e.recorder.SetHTTPClient(client)
}

// SetAIGenerator attaches the runtime AI generator used for AI fallback mode.
func (e *Engine) SetAIGenerator(generator *ai.Generator) {
	e.aiGenerator = generator
	e.resetRuntimeWarnings()
}

// StartRouteSync starts a background goroutine that listens on notifyCh for
// route-change signals and calls ReloadRoutes() with a debounce period to
// collapse bursts (e.g. uploading a spec with many operations) into a single
// reload.  The goroutine stops when ctx is cancelled.
//
// Call this once after the engine has been fully configured (session manager,
// AI generator, proxy client, etc.).  Typical usage in the Mongo multi-
// instance path:
//
//	notifyCh := make(chan struct{}, 8)
//	engine.StartRouteSync(serverCtx, notifyCh)
//	// ... wire notifyCh to the ChangeWatcher handler
func (e *Engine) StartRouteSync(ctx context.Context, notifyCh <-chan struct{}) {
	go e.routeSyncLoop(ctx, notifyCh)
}

const routeReloadDebounce = 200 * time.Millisecond

// routeSyncLoop drains notifyCh and calls ReloadRoutes after a quiet period.
func (e *Engine) routeSyncLoop(ctx context.Context, notifyCh <-chan struct{}) {
	logger := logging.Logger("proxy.route_sync")
	var timer *time.Timer

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return

		case _, ok := <-notifyCh:
			if !ok {
				return
			}
			// Reset (or start) the debounce timer.
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(routeReloadDebounce, func() {
				if ctx.Err() != nil {
					return
				}
				if err := e.ReloadRoutes(); err != nil {
					logger.Error("Route sync reload failed",
						"event", "route_sync_reload_error",
						"error", err,
					)
				} else {
					logger.Info("Routes reloaded via sync notification",
						"event", "route_sync_reloaded",
					)
				}
			})
		}
	}
}

// ReloadRoutes reloads all routes from enabled specs
func (e *Engine) ReloadRoutes() error {
	logger := logging.Logger("proxy.routes")
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear existing routes
	e.routes = make(map[string][]*route)
	e.resetRuntimeWarnings()

	// Get all enabled specs
	specs, err := e.store.GetEnabledSpecs()
	if err != nil {
		logger.Error("Failed to load enabled specs while reloading routes", "event", "routes_reload_specs_failed", "error", err)
		return err
	}

	for _, spec := range specs {
		ops, err := e.store.GetOperationsBySpec(spec.ID)
		if err != nil {
			logger.Error("Failed to load operations for enabled spec during route reload",
				"event", "routes_reload_operations_failed",
				"spec_id", spec.ID,
				"spec_name", spec.Name,
				"error", err,
			)
			continue
		}
		logger.Debug("Loaded operations for enabled spec",
			"event", "routes_reload_spec_loaded",
			"spec_id", spec.ID,
			"spec_name", spec.Name,
			"operation_count", len(ops),
		)

		for _, op := range ops {
			r := &route{
				spec:      spec,
				operation: op,
			}

			// Build regex pattern from path
			r.pattern, r.paramKeys = buildPathPattern(spec.BasePath, op.Path)

			e.routes[op.Method] = append(e.routes[op.Method], r)
		}
	}

	// Sort routes by specificity (more specific patterns first)
	for method := range e.routes {
		sortRoutes(e.routes[method])
	}

	// Update active-specs gauge
	metrics.ActiveSpecsTotal.Set(float64(len(specs)))
	logger.Info("Reloaded proxy routes",
		"event", "routes_reloaded",
		"enabled_specs", len(specs),
		"registered_methods", len(e.routes),
	)

	return nil
}

// buildPathPattern converts an OpenAPI path pattern to a regex
func buildPathPattern(basePath, pathPattern string) (*regexp.Regexp, []string) {
	fullPath := path.Join(basePath, pathPattern)

	var paramKeys []string

	// Escape special regex characters except for path parameters
	escaped := regexp.QuoteMeta(fullPath)

	// Replace escaped path parameters {param} with capture groups
	paramPattern := regexp.MustCompile(`\\{([^}]+)\\}`)
	result := paramPattern.ReplaceAllStringFunc(escaped, func(match string) string {
		// Extract parameter name
		paramName := match[2 : len(match)-2] // Remove \{ and \}
		paramKeys = append(paramKeys, paramName)
		return `([^/]+)`
	})

	// Anchor the pattern
	result = "^" + result + "$"

	pattern, _ := regexp.Compile(result)
	return pattern, paramKeys
}

// sortRoutes sorts routes by specificity (routes without parameters come first)
func sortRoutes(routes []*route) {
	sort.Slice(routes, func(i, j int) bool {
		// Count parameters in each route
		iParams := len(routes[i].paramKeys)
		jParams := len(routes[j].paramKeys)

		// Fewer parameters = more specific
		if iParams != jParams {
			return iParams < jParams
		}

		// Same number of params, sort by path length (longer = more specific)
		return len(routes[i].operation.Path) > len(routes[j].operation.Path)
	})
}

// Handler returns an http.Handler for the proxy engine
func (e *Engine) Handler() http.Handler {
	return http.HandlerFunc(e.ServeHTTP)
}

// ServeHTTP handles incoming requests
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	reqLogger := logging.Logger("proxy.request").With(
		"method", r.Method,
		"path", r.URL.Path,
		"query", r.URL.RawQuery,
	)
	reqLogger.Debug("Received proxy request", "event", "proxy_request_received")

	// Read request body early for tracing (we need it even for unmatched requests)
	var requestBody string
	if r.Body != nil {
		bodyBytes, _ := io.ReadAll(r.Body)
		requestBody = string(bodyBytes)
	}

	// Find matching route
	e.mu.RLock()
	matchedRoute, pathParams := e.matchRoute(r.Method, r.URL.Path)
	e.mu.RUnlock()

	if matchedRoute == nil {
		reqLogger.Info("No route matched incoming request", "event", "proxy_route_unmatched")
		// Record trace for unmatched request if any spec has tracing enabled
		e.recordUnmatchedTrace(r, requestBody, startTime)
		metrics.UnmatchedRequestsTotal.Inc()
		http.NotFound(w, r)
		return
	}
	reqLogger = reqLogger.With(
		"spec_id", matchedRoute.spec.ID,
		"spec_name", matchedRoute.spec.Name,
		"operation_id", matchedRoute.operation.ID,
		"operation_path", matchedRoute.operation.Path,
	)
	reqLogger.Debug("Matched proxy route",
		"event", "proxy_route_matched",
		"path_params", pathParams,
	)

	// Compute the request signature (needed for both proxy recording and condition evaluation)
	effectiveSignatureConfig := ResolveSignatureConfig(matchedRoute.spec, matchedRoute.operation)
	signature := ComputeSignature(
		pathParams,
		r.URL.Query(),
		r.Header,
		requestBody,
		effectiveSignatureConfig,
	)
	reqLogger = reqLogger.With("signature", signature)

	// ---- Virtual response mode ----
	// Resolve session and run operation-level scripts BEFORE response matching so
	// that script output can be used as a condition source (source=script).
	//
	// Session creation rules:
	//   1. Session header sent + session exists        → reuse, echo ID in response
	//   2. Session header sent + session not found     → create with that ID, echo in response
	//   3. No session header + store op in script      → create new UUID session, echo in response
	//   4. No session header + no store op             → no session, no header sent
	var sess store.SessionState
	var sessionIsNew bool
	var lazySession *store.LazySession

	if e.sessionManager != nil {
		rawSessionID := r.Header.Get(e.sessionHeaderName)
		if rawSessionID != "" {
			// Case 1 or 2: explicit ID provided
			var sessErr error
			sess, sessionIsNew, sessErr = e.sessionManager.GetOrCreate(rawSessionID)
			if sessErr != nil {
				reqLogger.Error("Failed to resolve request session",
					"event", "session_resolve_failed",
					"header_name", e.sessionHeaderName,
					"error", sessErr,
				)
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error": "Failed to resolve session"}`))
				return
			}
			sessionInfo := sess.Info(false)
			w.Header().Set(e.sessionHeaderName, sessionInfo.ID)
			reqLogger.Debug("Resolved request session",
				"event", "session_resolved",
				"session_id", sessionInfo.ID,
				"session_is_new", sessionIsNew,
			)
		} else {
			// Case 3 or 4: no ID — create a lazy session that materialises on first store op
			lazySession = store.NewLazySession(e.sessionManager)
			sess = lazySession
		}
	}

	scriptInput := scripting.BuildInput(pathParams, r, requestBody)

	// Run spec-level script bindings first, then operation-level bindings.
	// Spec scripts provide a shared base; operation scripts can override by key.
	scriptOutput, scriptTraces := e.scriptEngine.RunSpecBindings(r.Context(), matchedRoute.spec.ID, scriptInput, sess)
	opOutput, opTraces := e.scriptEngine.RunBindings(r.Context(), matchedRoute.operation.ID, scriptInput, sess)
	for k, v := range opOutput {
		scriptOutput[k] = v
	}
	scriptTraces = append(scriptTraces, opTraces...)
	reqLogger.Debug("Executed script bindings",
		"event", "script_bindings_executed",
		"binding_count", len(scriptTraces),
	)

	// Build request data for condition evaluation (include pre-computed signature and script output)
	reqData := &condition.RequestData{
		PathParams:   pathParams,
		QueryParams:  r.URL.Query(),
		Headers:      r.Header,
		Body:         requestBody,
		Signature:    signature,
		ScriptOutput: scriptOutput,
	}

	// Get response configs for the operation
	responseConfigs, err := e.store.GetResponseConfigsByOperation(matchedRoute.operation.ID)
	if err != nil {
		reqLogger.Error("Failed to load response configs for operation",
			"event", "response_configs_load_failed",
			"error", err,
		)
	} else {
		reqLogger.Debug("Loaded response configs for operation",
			"event", "response_configs_loaded",
			"config_count", len(responseConfigs),
		)
	}
	enabledTags := make(map[string]struct{})
	enabledTags[models.DefaultTagName] = struct{}{}
	for _, tag := range matchedRoute.spec.EnabledTags {
		enabledTags[tag] = struct{}{}
	}

	var matchedConfig *models.ResponseConfig
	responseTier := ""
	if err == nil && len(responseConfigs) > 0 {
		matchedConfig = e.findMatchingResponseConfig(responseConfigs, reqData, enabledTags, false)
		if matchedConfig != nil {
			responseTier = models.TraceResponseTierConfigured
			reqLogger.Info("Selected configured response",
				"event", "response_config_selected",
				"response_config_id", matchedConfig.ID,
				"response_config_name", matchedConfig.Name,
				"response_origin", matchedConfig.EffectiveOrigin(),
				"response_tier", responseTier,
			)
		} else {
			matchedConfig = e.findMatchingResponseConfig(responseConfigs, reqData, enabledTags, true)
			if matchedConfig != nil {
				responseTier = models.TraceResponseTierRecorded
				reqLogger.Info("Selected recorded response",
					"event", "response_config_selected",
					"response_config_id", matchedConfig.ID,
					"response_config_name", matchedConfig.Name,
					"response_origin", matchedConfig.EffectiveOrigin(),
					"response_tier", responseTier,
				)
			}
		}
	}

	modeSelection := e.selectMode(matchedRoute.spec, reqData)
	specMode := modeSelection.Mode
	requestedScenarioName := strings.TrimSpace(r.Header.Get("X-Virtual-AI-Scenario"))
	appliedScenario := e.resolveAIScenario(requestedScenarioName)
	reqLogger.Debug("Resolved fallback mode for request",
		"event", "fallback_mode_resolved",
		"mode", specMode,
		"ai_skipped_reason", modeSelection.AISkippedReason,
		"proxy_skipped_reason", modeSelection.ProxySkippedReason,
		"ai_scenario_requested", requestedScenarioName,
		"ai_scenario_applied", aiScenarioName(appliedScenario),
	)

	if matchedConfig == nil {
		switch specMode {
		case models.SpecModeAI:
			reqLogger.Info("Using AI fallback for unmatched request", "event", "ai_fallback_selected")
			opCtx := e.buildAIOperationContext(matchedRoute.operation)
			reqLogger.Debug("Starting AI runtime response generation", "event", "ai_runtime_generation_started")
			aiResp, aiErr := e.aiGenerator.GenerateRuntimeResponse(r.Context(), opCtx, ai.RuntimeRequestContext{
				PathParams:  pathParams,
				QueryParams: r.URL.Query(),
				Headers:     headersToMap(r.Header),
				Body:        requestBody,
				Signature:   signature,
				Scenario:    appliedScenario,
			})
			if aiErr != nil {
				reqLogger.Error("AI runtime response generation failed",
					"event", "ai_runtime_generation_failed",
					"error", aiErr,
				)
				statusCode := http.StatusBadGateway
				respBody := `{"error":"AI response generation failed: ` + aiErr.Error() + `"}`
				http.Error(w, respBody, statusCode)
				duration := time.Since(startTime)
				e.statsCollector.RecordRequest(
					matchedRoute.spec.ID,
					matchedRoute.operation.ID,
					matchedRoute.operation.Method,
					matchedRoute.operation.Path,
					duration,
					true,
				)
				metrics.RequestsTotal.WithLabelValues(
					matchedRoute.spec.ID,
					matchedRoute.operation.Method,
					matchedRoute.operation.Path,
					metrics.StatusLabel(statusCode),
				).Inc()
				metrics.RequestDurationSeconds.WithLabelValues(
					matchedRoute.spec.ID,
					matchedRoute.operation.Method,
					matchedRoute.operation.Path,
				).Observe(duration.Seconds())
				if matchedRoute.spec.Tracing {
					e.tracingService.RecordTrace(&models.Trace{
						SpecID:              matchedRoute.spec.ID,
						SpecName:            matchedRoute.spec.Name,
						OperationID:         matchedRoute.operation.ID,
						OperationPath:       matchedRoute.operation.Path,
						Timestamp:           startTime,
						Duration:            duration.Nanoseconds(),
						Mode:                specMode,
						ResponseSource:      models.TraceResponseSourceAI,
						ResponseTier:        models.TraceResponseTierFallback,
						Signature:           signature,
						AISkippedReason:     modeSelection.AISkippedReason,
						ProxySkippedReason:  modeSelection.ProxySkippedReason,
						AIScenarioRequested: requestedScenarioName,
						AIScenarioApplied:   aiScenarioName(appliedScenario),
						Request: models.TraceRequest{
							Method:  r.Method,
							URL:     r.URL.String(),
							Path:    r.URL.Path,
							Query:   r.URL.Query(),
							Headers: r.Header,
							Body:    requestBody,
						},
						Response: models.TraceResponse{
							StatusCode: statusCode,
							Headers:    headersToMap(w.Header()),
							Body:       respBody,
						},
					})
				}
				return
			}
			reqLogger.Info("AI runtime response generated",
				"event", "ai_runtime_generation_succeeded",
				"status_code", aiResp.StatusCode,
				"headers_count", len(aiResp.Headers),
			)

			for key, value := range aiResp.Headers {
				if skipRecordedResponseHeaders[http.CanonicalHeaderKey(key)] {
					continue
				}
				w.Header().Set(key, value)
			}
			if w.Header().Get("Content-Type") == "" && aiResp.Body != "" {
				w.Header().Set("Content-Type", "application/json")
			}
			w.WriteHeader(aiResp.StatusCode)
			if aiResp.Body != "" {
				_, _ = w.Write([]byte(aiResp.Body))
			}
			go e.recorder.SaveResponse(matchedRoute.operation, signature, aiResp.StatusCode, aiResp.Headers, aiResp.Body, models.ResponseOriginAI)

			duration := time.Since(startTime)
			isError := aiResp.StatusCode >= 400
			e.statsCollector.RecordRequest(
				matchedRoute.spec.ID,
				matchedRoute.operation.ID,
				matchedRoute.operation.Method,
				matchedRoute.operation.Path,
				duration,
				isError,
			)
			metrics.RequestsTotal.WithLabelValues(
				matchedRoute.spec.ID,
				matchedRoute.operation.Method,
				matchedRoute.operation.Path,
				metrics.StatusLabel(aiResp.StatusCode),
			).Inc()
			metrics.RequestDurationSeconds.WithLabelValues(
				matchedRoute.spec.ID,
				matchedRoute.operation.Method,
				matchedRoute.operation.Path,
			).Observe(duration.Seconds())
			if matchedRoute.spec.Tracing {
				e.tracingService.RecordTrace(&models.Trace{
					SpecID:              matchedRoute.spec.ID,
					SpecName:            matchedRoute.spec.Name,
					OperationID:         matchedRoute.operation.ID,
					OperationPath:       matchedRoute.operation.Path,
					Timestamp:           startTime,
					Duration:            duration.Nanoseconds(),
					MatchedConfig:       "[ai-generated]",
					Mode:                specMode,
					ResponseSource:      models.TraceResponseSourceAI,
					ResponseTier:        models.TraceResponseTierFallback,
					Signature:           signature,
					AISkippedReason:     modeSelection.AISkippedReason,
					ProxySkippedReason:  modeSelection.ProxySkippedReason,
					AIScenarioRequested: requestedScenarioName,
					AIScenarioApplied:   aiScenarioName(appliedScenario),
					Request: models.TraceRequest{
						Method:  r.Method,
						URL:     r.URL.String(),
						Path:    r.URL.Path,
						Query:   r.URL.Query(),
						Headers: r.Header,
						Body:    requestBody,
					},
					Response: models.TraceResponse{
						StatusCode: aiResp.StatusCode,
						Headers:    headersToMap(w.Header()),
						Body:       aiResp.Body,
					},
				})
			}
			return
		case models.SpecModeProxy:
			reqLogger.Info("Using proxy fallback for unmatched request",
				"event", "proxy_fallback_selected",
				"backend_uri", matchedRoute.spec.BackendURI,
			)
			statusCode, respHeaders, respBody, proxyErr := e.recorder.ProxyAndRecord(
				r.Method,
				r.URL.Path,
				r.URL.RawQuery,
				r.Header,
				requestBody,
				matchedRoute.operation,
				matchedRoute.spec,
				signature,
			)
			if proxyErr != nil {
				reqLogger.Error("Proxy fallback request failed",
					"event", "proxy_fallback_failed",
					"backend_uri", matchedRoute.spec.BackendURI,
					"error", proxyErr,
				)
				statusCode = http.StatusBadGateway
				respBody = `{"error":"proxy backend unavailable: ` + proxyErr.Error() + `"}`
				http.Error(w, respBody, statusCode)
			} else {
				reqLogger.Info("Proxy fallback request succeeded",
					"event", "proxy_fallback_succeeded",
					"backend_uri", matchedRoute.spec.BackendURI,
					"status_code", statusCode,
				)
				for key, val := range respHeaders {
					w.Header().Set(key, val)
				}
				w.WriteHeader(statusCode)
				_, _ = w.Write([]byte(respBody))
			}

			duration := time.Since(startTime)
			isError := statusCode >= 400
			e.statsCollector.RecordRequest(
				matchedRoute.spec.ID,
				matchedRoute.operation.ID,
				matchedRoute.operation.Method,
				matchedRoute.operation.Path,
				duration,
				isError,
			)
			metrics.RequestsTotal.WithLabelValues(
				matchedRoute.spec.ID,
				matchedRoute.operation.Method,
				matchedRoute.operation.Path,
				metrics.StatusLabel(statusCode),
			).Inc()
			metrics.RequestDurationSeconds.WithLabelValues(
				matchedRoute.spec.ID,
				matchedRoute.operation.Method,
				matchedRoute.operation.Path,
			).Observe(duration.Seconds())
			metrics.ProxyRequestsTotal.WithLabelValues(
				matchedRoute.spec.ID,
				matchedRoute.spec.BackendURI,
			).Inc()
			if matchedRoute.spec.Tracing {
				trace := &models.Trace{
					SpecID:             matchedRoute.spec.ID,
					SpecName:           matchedRoute.spec.Name,
					OperationID:        matchedRoute.operation.ID,
					OperationPath:      matchedRoute.operation.Path,
					Timestamp:          startTime,
					Duration:           duration.Nanoseconds(),
					MatchedConfig:      "[proxy-recorded]",
					Mode:               specMode,
					ResponseSource:     models.TraceResponseSourceProxy,
					ResponseTier:       models.TraceResponseTierFallback,
					ProxyMode:          proxyErr == nil,
					Signature:          signature,
					BackendURI:         matchedRoute.spec.BackendURI,
					AISkippedReason:    modeSelection.AISkippedReason,
					ProxySkippedReason: modeSelection.ProxySkippedReason,
					Request: models.TraceRequest{
						Method:  r.Method,
						URL:     r.URL.String(),
						Path:    r.URL.Path,
						Query:   r.URL.Query(),
						Headers: r.Header,
						Body:    requestBody,
					},
					Response: models.TraceResponse{
						StatusCode: statusCode,
						Headers:    headersToMap(w.Header()),
						Body:       respBody,
					},
				}
				e.tracingService.RecordTrace(trace)
			}
			return
		}
	}

	// If no matching config found, try to use example response from OpenAPI spec.
	// Only standard mode falls back to spec examples/defaults.
	if matchedConfig == nil && specMode == models.SpecModeStandard && matchedRoute.spec.UseExampleFallback && matchedRoute.operation.ExampleResponse != nil {
		example := matchedRoute.operation.ExampleResponse
		reqLogger.Info("Using OpenAPI example fallback",
			"event", "example_fallback_selected",
			"status_code", example.StatusCode,
		)

		// Set headers from example
		for key, value := range example.Headers {
			w.Header().Set(key, value)
		}

		// Set default content-type if not set
		if w.Header().Get("Content-Type") == "" && example.Body != "" {
			w.Header().Set("Content-Type", "application/json")
		}

		// Write response
		w.WriteHeader(example.StatusCode)
		if example.Body != "" {
			w.Write([]byte(example.Body))
		}

		// Calculate duration and record stats
		duration := time.Since(startTime)
		isError := example.StatusCode >= 400
		e.statsCollector.RecordRequest(
			matchedRoute.spec.ID,
			matchedRoute.operation.ID,
			matchedRoute.operation.Method,
			matchedRoute.operation.Path,
			duration,
			isError,
		)
		metrics.RequestsTotal.WithLabelValues(
			matchedRoute.spec.ID,
			matchedRoute.operation.Method,
			matchedRoute.operation.Path,
			metrics.StatusLabel(example.StatusCode),
		).Inc()
		metrics.RequestDurationSeconds.WithLabelValues(
			matchedRoute.spec.ID,
			matchedRoute.operation.Method,
			matchedRoute.operation.Path,
		).Observe(duration.Seconds())

		// Record trace if enabled
		if matchedRoute.spec.Tracing {
			trace := &models.Trace{
				SpecID:             matchedRoute.spec.ID,
				SpecName:           matchedRoute.spec.Name,
				OperationID:        matchedRoute.operation.ID,
				OperationPath:      matchedRoute.operation.Path,
				Timestamp:          startTime,
				Duration:           duration.Nanoseconds(),
				MatchedConfig:      "spec-example",
				Mode:               specMode,
				ResponseSource:     models.TraceResponseSourceExample,
				ResponseTier:       models.TraceResponseTierFallback,
				AISkippedReason:    modeSelection.AISkippedReason,
				ProxySkippedReason: modeSelection.ProxySkippedReason,
				Request: models.TraceRequest{
					Method:  r.Method,
					URL:     r.URL.String(),
					Path:    r.URL.Path,
					Query:   r.URL.Query(),
					Headers: r.Header,
					Body:    requestBody,
				},
				Response: models.TraceResponse{
					StatusCode: example.StatusCode,
					Headers:    headersToMap(w.Header()),
					Body:       example.Body,
				},
			}
			e.tracingService.RecordTrace(trace)
		}
		return
	}

	// If still no match and no example, return error
	if matchedConfig == nil {
		reqLogger.Info("No response configuration matched request",
			"event", "response_config_not_found",
			"mode", specMode,
		)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "No matching response configuration for this request"}`))
		return
	}

	// Run response-level script bindings (if any) and merge their output into scriptOutput.
	// Response-level scripts run after the config is selected so they have access to
	// operation-level output while adding response-specific computed values.
	if respScriptOutput, respScriptTraces := e.scriptEngine.RunResponseBindings(r.Context(), matchedConfig.ID, scriptInput, sess); len(respScriptOutput) > 0 {
		for k, v := range respScriptOutput {
			scriptOutput[k] = v
		}
		scriptTraces = append(scriptTraces, respScriptTraces...)
		reqLogger.Debug("Executed response script bindings",
			"event", "response_script_bindings_executed",
			"binding_count", len(respScriptTraces),
		)
	}

	// Case 3 of session-creation rules: if a lazy session was used (no header
	// sent by the client) and any script performed a store operation, the session
	// will now be materialised — echo its generated ID back to the caller.
	// If nothing materialised, clear sess so downstream code treats it as absent.
	if lazySession != nil {
		if inner := lazySession.Materialized(); inner != nil {
			info := inner.Info(false)
			w.Header().Set(e.sessionHeaderName, info.ID)
			sess = inner
			sessionIsNew = true
			reqLogger.Debug("Lazy session materialised by store operation",
				"event", "lazy_session_created",
				"session_id", info.ID,
			)
		} else {
			sess = nil
		}
	}

	// Apply delay if configured
	if matchedConfig.Delay > 0 {
		time.Sleep(time.Duration(matchedConfig.Delay) * time.Millisecond)
	}

	// Build template context
	seed := buildTemplateSeed(r, pathParams)
	requestID := uuid.New().String()
	templateCtx := &template.Context{
		PathParams:   pathParams,
		QueryParams:  r.URL.Query(),
		Headers:      r.Header,
		Body:         requestBody,
		RNG:          rand.New(rand.NewSource(seed)),
		ScriptOutput: scriptOutput,
		Method:       r.Method,
		RequestURL:   r.URL.String(),
		RequestID:    requestID,
	}
	if sess != nil {
		templateCtx.StoreReader = func(key string) string {
			v, _ := sess.Get(key)
			if v == nil {
				return ""
			}
			return fmt.Sprintf("%v", v)
		}
		templateCtx.StoreWriter = func(name string) string {
			counterKey := "__counter__" + name
			v, _ := sess.Get(counterKey)
			var n int64
			if v != nil {
				switch cv := v.(type) {
				case int64:
					n = cv
				case float64:
					n = int64(cv)
				}
			}
			n++
			_ = sess.Set(counterKey, n)
			return fmt.Sprintf("%d", n)
		}
	}

	// Process headers – skip any that Go's ResponseWriter manages automatically
	// or that are stale/misleading for virtual responses (see skipRecordedResponseHeaders).
	responseHeaders := e.templateEngine.ProcessHeaders(matchedConfig.Headers, templateCtx)
	for key, value := range responseHeaders {
		if skipRecordedResponseHeaders[http.CanonicalHeaderKey(key)] {
			continue
		}
		w.Header().Set(key, value)
	}

	// Set default content-type if not set
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}

	// Process body with advanced templating
	responseBody, err := e.templateEngine.RenderBodyTemplate(matchedConfig.Body, templateCtx)
	if err != nil {
		reqLogger.Error("Failed to render response template; using raw body",
			"event", "response_template_render_failed",
			"response_config_id", matchedConfig.ID,
			"response_config_name", matchedConfig.Name,
			"error", err,
		)
		responseBody = matchedConfig.Body
	}

	// Write response
	w.WriteHeader(matchedConfig.StatusCode)
	w.Write([]byte(responseBody))

	// Calculate duration
	duration := time.Since(startTime)

	// Record statistics
	isError := matchedConfig.StatusCode >= 400
	e.statsCollector.RecordRequest(
		matchedRoute.spec.ID,
		matchedRoute.operation.ID,
		matchedRoute.operation.Method,
		matchedRoute.operation.Path,
		duration,
		isError,
	)
	metrics.RequestsTotal.WithLabelValues(
		matchedRoute.spec.ID,
		matchedRoute.operation.Method,
		matchedRoute.operation.Path,
		metrics.StatusLabel(matchedConfig.StatusCode),
	).Inc()
	metrics.RequestDurationSeconds.WithLabelValues(
		matchedRoute.spec.ID,
		matchedRoute.operation.Method,
		matchedRoute.operation.Path,
	).Observe(duration.Seconds())
	metrics.ResponseConfigMatchesTotal.WithLabelValues(
		matchedRoute.operation.ID,
		matchedConfig.Name,
	).Inc()
	reqLogger.Info("Served configured response",
		"event", "configured_response_served",
		"response_config_id", matchedConfig.ID,
		"response_config_name", matchedConfig.Name,
		"response_origin", matchedConfig.EffectiveOrigin(),
		"status_code", matchedConfig.StatusCode,
		"duration_ms", duration.Milliseconds(),
	)

	// Record trace if tracing is enabled
	if matchedRoute.spec.Tracing {
		var sessionTrace *models.SessionTrace
		if sess != nil {
			sessionInfo := sess.Info(false)
			sessionTrace = &models.SessionTrace{
				ID:    sessionInfo.ID,
				IsNew: sessionIsNew,
			}
		}
		trace := &models.Trace{
			SpecID:              matchedRoute.spec.ID,
			SpecName:            matchedRoute.spec.Name,
			OperationID:         matchedRoute.operation.ID,
			OperationPath:       matchedRoute.operation.Path,
			Timestamp:           startTime,
			Duration:            duration.Nanoseconds(),
			MatchedConfigID:     matchedConfig.ID,
			MatchedConfig:       matchedConfig.Name,
			MatchedConfigOrigin: matchedConfig.EffectiveOrigin(),
			Mode:                specMode,
			ResponseSource:      models.TraceResponseSourceConfig,
			ResponseTier:        responseTier,
			Signature:           signature,
			AISkippedReason:     modeSelection.AISkippedReason,
			ProxySkippedReason:  modeSelection.ProxySkippedReason,
			Scripts:             scriptTraces,
			Session:             sessionTrace,
			Request: models.TraceRequest{
				Method:  r.Method,
				URL:     r.URL.String(),
				Path:    r.URL.Path,
				Query:   r.URL.Query(),
				Headers: r.Header,
				Body:    requestBody,
			},
			Response: models.TraceResponse{
				StatusCode: matchedConfig.StatusCode,
				Headers:    headersToMap(w.Header()),
				Body:       responseBody,
			},
		}
		e.tracingService.RecordTrace(trace)
	}
}

type modeSelection struct {
	Mode               string
	AISkippedReason    string
	ProxySkippedReason string
}

func (e *Engine) findMatchingResponseConfig(configs []*models.ResponseConfig, reqData *condition.RequestData, enabledTags map[string]struct{}, recorded bool) *models.ResponseConfig {
	for _, cfg := range configs {
		if !cfg.Enabled || cfg.Recorded != recorded {
			continue
		}
		tag := cfg.Tag
		if tag == "" {
			tag = models.DefaultTagName
		}
		if _, ok := enabledTags[tag]; !ok {
			continue
		}
		if e.condEvaluator.EvaluateAll(cfg.Conditions, reqData) {
			return cfg
		}
	}
	return nil
}

func (e *Engine) selectMode(spec *models.Spec, reqData *condition.RequestData) modeSelection {
	selection := modeSelection{Mode: models.SpecModeStandard}
	policy := spec.EffectiveModePolicy()

	if !policy.AI.Enabled {
		selection.AISkippedReason = "disabled"
	} else if e.aiGenerator == nil || !e.aiGenerator.IsConfigured() {
		selection.AISkippedReason = "not-configured"
		e.warnModeUnavailable(spec, "AI", "AI generator is not configured")
	} else if e.condEvaluator.EvaluateAll(policy.AI.Conditions, reqData) {
		selection.Mode = models.SpecModeAI
		return selection
	} else {
		selection.AISkippedReason = "conditions-not-matched"
	}

	if !policy.Proxy.Enabled {
		selection.ProxySkippedReason = "disabled"
		return selection
	}
	if spec == nil || spec.BackendURI == "" {
		selection.ProxySkippedReason = "no-backend"
		e.warnModeUnavailable(spec, "proxy", "backend URI is not configured")
		return selection
	}
	if e.condEvaluator.EvaluateAll(policy.Proxy.Conditions, reqData) {
		selection.Mode = models.SpecModeProxy
		return selection
	}
	selection.ProxySkippedReason = "conditions-not-matched"
	return selection
}

func (e *Engine) warnModeUnavailable(spec *models.Spec, mode, reason string) {
	if spec == nil {
		return
	}

	key := spec.ID + "|" + mode + "|" + reason
	e.warningMu.Lock()
	if _, exists := e.runtimeWarnings[key]; exists {
		e.warningMu.Unlock()
		return
	}
	e.runtimeWarnings[key] = struct{}{}
	e.warningMu.Unlock()

	logging.Logger("proxy").Warn("Fallback mode skipped",
		"event", "fallback_mode_skipped",
		"mode", strings.ToLower(mode),
		"spec_id", spec.ID,
		"spec_name", spec.Name,
		"reason", reason,
	)
}

func (e *Engine) resetRuntimeWarnings() {
	e.warningMu.Lock()
	defer e.warningMu.Unlock()
	e.runtimeWarnings = make(map[string]struct{})
}

func (e *Engine) resolveAIScenario(requested string) *ai.RuntimeScenario {
	scenarios, err := e.store.ListAIScenarios()
	if err != nil || len(scenarios) == 0 {
		return nil
	}

	normalized := make([]models.AIScenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		if scenario != nil {
			normalized = append(normalized, *scenario)
		}
	}

	scenario := models.FindAIScenario(normalized, requested)
	if scenario == nil || !scenario.Enabled {
		return nil
	}
	return &ai.RuntimeScenario{
		Name:                    scenario.Name,
		Description:             scenario.Description,
		ResponseKind:            scenario.ResponseKind,
		StatusCode:              scenario.StatusCode,
		Count:                   scenario.Count,
		Instructions:            scenario.Instructions,
		UseDefaultSuccessStatus: scenario.ResponseKind == models.AIScenarioKindSuccess && scenario.StatusCode == 0,
	}
}

func aiScenarioName(scenario *ai.RuntimeScenario) string {
	if scenario == nil {
		return ""
	}
	return scenario.Name
}

func (e *Engine) buildAIOperationContext(op *models.Operation) ai.OperationContext {
	opCtx := ai.OperationContext{
		Method:          op.Method,
		Path:            op.Path,
		Summary:         op.Summary,
		Description:     op.Description,
		ExampleResponse: op.ExampleResponse,
	}

	spec, err := e.store.GetSpec(op.SpecID)
	if err != nil || spec.Content == "" {
		return opCtx
	}

	p := parser.NewParser()
	if defs, err := p.ExtractAllResponses(spec.Content, op.Method, op.Path); err == nil {
		opCtx.SpecResponses = make([]ai.SpecResponseDef, 0, len(defs))
		for _, d := range defs {
			opCtx.SpecResponses = append(opCtx.SpecResponses, ai.SpecResponseDef{
				StatusCode:  d.StatusCode,
				Description: d.Description,
				BodyExample: d.BodyExample,
				SchemaHint:  d.SchemaHint,
			})
		}
	}
	if inputs, err := p.ExtractOperationInputs(spec.Content, op.Method, op.Path); err == nil && inputs != nil {
		opCtx.Inputs = &ai.OperationInputs{}
		for _, pp := range inputs.PathParams {
			opCtx.Inputs.PathParams = append(opCtx.Inputs.PathParams, ai.ParamDef{
				Name:        pp.Name,
				In:          pp.In,
				Required:    pp.Required,
				Type:        pp.Type,
				Description: pp.Description,
			})
		}
		for _, qp := range inputs.QueryParams {
			opCtx.Inputs.QueryParams = append(opCtx.Inputs.QueryParams, ai.ParamDef{
				Name:        qp.Name,
				In:          qp.In,
				Required:    qp.Required,
				Type:        qp.Type,
				Description: qp.Description,
			})
		}
		for _, bf := range inputs.BodyFields {
			opCtx.Inputs.BodyFields = append(opCtx.Inputs.BodyFields, ai.BodyFieldDef{
				GjsonPath:   bf.GjsonPath,
				Type:        bf.Type,
				Description: bf.Description,
			})
		}
	}

	return opCtx
}

// matchRoute finds a matching route for the given method and path
func (e *Engine) matchRoute(method, requestPath string) (*route, map[string]string) {
	routes, ok := e.routes[method]
	if !ok {
		return nil, nil
	}

	for _, r := range routes {
		if r.pattern == nil {
			continue
		}

		matches := r.pattern.FindStringSubmatch(requestPath)
		if matches == nil {
			continue
		}

		// Extract path parameters
		pathParams := make(map[string]string)
		for i, key := range r.paramKeys {
			if i+1 < len(matches) {
				pathParams[key] = matches[i+1]
			}
		}

		return r, pathParams
	}

	return nil, nil
}

// headersToMap converts http.Header to map[string][]string
func headersToMap(h http.Header) map[string][]string {
	result := make(map[string][]string)
	for key, values := range h {
		result[key] = values
	}
	return result
}

func buildTemplateSeed(r *http.Request, pathParams map[string]string) int64 {
	h := fnv.New64a()
	_, _ = io.WriteString(h, r.Method)
	_, _ = io.WriteString(h, "|")
	_, _ = io.WriteString(h, r.URL.Path)
	_, _ = io.WriteString(h, "|")
	writeSortedPathParams(h, pathParams)
	_, _ = io.WriteString(h, "|")
	writeSortedQueryParams(h, r.URL.Query())

	return int64(h.Sum64())
}

func writeSortedPathParams(h hash.Hash64, params map[string]string) {
	if len(params) == 0 {
		return
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		_, _ = io.WriteString(h, key)
		_, _ = io.WriteString(h, "=")
		_, _ = io.WriteString(h, params[key])
		_, _ = io.WriteString(h, "&")
	}
}

func writeSortedQueryParams(h hash.Hash64, query url.Values) {
	if len(query) == 0 {
		return
	}

	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		vals := append([]string(nil), query[key]...)
		sort.Strings(vals)
		for _, val := range vals {
			_, _ = io.WriteString(h, key)
			_, _ = io.WriteString(h, "=")
			_, _ = io.WriteString(h, val)
			_, _ = io.WriteString(h, "&")
		}
	}
}

// MatchRoute is exported for testing purposes
func (e *Engine) MatchRoute(method, path string) (*models.Operation, map[string]string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	matchedRoute, pathParams := e.matchRoute(method, path)
	if matchedRoute == nil {
		return nil, nil, nil
	}

	return matchedRoute.operation, pathParams, nil
}

// GetRegisteredRoutes returns information about registered routes
func (e *Engine) GetRegisteredRoutes() map[string][]string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string][]string)
	for method, routes := range e.routes {
		for _, r := range routes {
			result[method] = append(result[method], r.operation.FullPath)
		}
	}
	return result
}

// recordUnmatchedTrace records a trace for requests that don't match any operation
// This helps debug requests that are failing to match
func (e *Engine) recordUnmatchedTrace(r *http.Request, requestBody string, startTime time.Time) {
	// Check if any spec has tracing enabled
	specs, err := e.store.GetEnabledSpecs()
	if err != nil {
		return
	}

	var tracingEnabled bool
	for _, spec := range specs {
		if spec.Tracing {
			tracingEnabled = true
			break
		}
	}

	if !tracingEnabled {
		return
	}

	duration := time.Since(startTime)

	trace := &models.Trace{
		SpecID:        "",
		SpecName:      "[Unmatched]",
		OperationID:   "",
		OperationPath: "",
		Timestamp:     startTime,
		Duration:      duration.Nanoseconds(),
		MatchedConfig: "no-match",
		Request: models.TraceRequest{
			Method:  r.Method,
			URL:     r.URL.String(),
			Path:    r.URL.Path,
			Query:   r.URL.Query(),
			Headers: r.Header,
			Body:    requestBody,
		},
		Response: models.TraceResponse{
			StatusCode: http.StatusNotFound,
			Headers:    map[string][]string{"Content-Type": {"text/plain; charset=utf-8"}},
			Body:       "404 page not found\n",
		},
	}
	e.tracingService.RecordTrace(trace)
}
