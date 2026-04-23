package proxy

import (
	"hash"
	"hash/fnv"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/prasenjit/go-virtual/internal/ai"
	"github.com/prasenjit/go-virtual/internal/condition"
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
	sessionManager    *store.SessionManager // nil when Phase 2 is not configured
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
func (e *Engine) SetSessionManager(sm *store.SessionManager, headerName string) {
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

// ReloadRoutes reloads all routes from enabled specs
func (e *Engine) ReloadRoutes() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear existing routes
	e.routes = make(map[string][]*route)
	e.resetRuntimeWarnings()

	// Get all enabled specs
	specs, err := e.store.GetEnabledSpecs()
	if err != nil {
		return err
	}

	for _, spec := range specs {
		ops, err := e.store.GetOperationsBySpec(spec.ID)
		if err != nil {
			continue
		}

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
		// Record trace for unmatched request if any spec has tracing enabled
		e.recordUnmatchedTrace(r, requestBody, startTime)
		metrics.UnmatchedRequestsTotal.Inc()
		http.NotFound(w, r)
		return
	}

	// Compute the request signature (needed for both proxy recording and condition evaluation)
	signature := ComputeSignature(
		pathParams,
		r.URL.Query(),
		r.Header,
		requestBody,
		matchedRoute.operation.SignatureConfig,
	)

	// ---- Virtual response mode ----
	// Build request data for condition evaluation (include pre-computed signature)
	reqData := &condition.RequestData{
		PathParams:  pathParams,
		QueryParams: r.URL.Query(),
		Headers:     r.Header,
		Body:        requestBody,
		Signature:   signature,
	}

	// Get response configs for the operation
	responseConfigs, err := e.store.GetResponseConfigsByOperation(matchedRoute.operation.ID)
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
		} else {
			matchedConfig = e.findMatchingResponseConfig(responseConfigs, reqData, enabledTags, true)
			if matchedConfig != nil {
				responseTier = models.TraceResponseTierRecorded
			}
		}
	}

	modeSelection := e.selectMode(matchedRoute.spec, reqData)
	specMode := modeSelection.Mode

	if matchedConfig == nil {
		switch specMode {
		case models.SpecModeAI:
			opCtx := e.buildAIOperationContext(matchedRoute.operation)
			aiResp, aiErr := e.aiGenerator.GenerateRuntimeResponse(r.Context(), opCtx, ai.RuntimeRequestContext{
				PathParams:  pathParams,
				QueryParams: r.URL.Query(),
				Headers:     headersToMap(r.Header),
				Body:        requestBody,
				Signature:   signature,
			})
			if aiErr != nil {
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
						SpecID:             matchedRoute.spec.ID,
						SpecName:           matchedRoute.spec.Name,
						OperationID:        matchedRoute.operation.ID,
						OperationPath:      matchedRoute.operation.Path,
						Timestamp:          startTime,
						Duration:           duration.Nanoseconds(),
						Mode:               specMode,
						ResponseSource:     models.TraceResponseSourceAI,
						ResponseTier:       models.TraceResponseTierFallback,
						Signature:          signature,
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
					})
				}
				return
			}

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
					SpecID:             matchedRoute.spec.ID,
					SpecName:           matchedRoute.spec.Name,
					OperationID:        matchedRoute.operation.ID,
					OperationPath:      matchedRoute.operation.Path,
					Timestamp:          startTime,
					Duration:           duration.Nanoseconds(),
					MatchedConfig:      "[ai-generated]",
					Mode:               specMode,
					ResponseSource:     models.TraceResponseSourceAI,
					ResponseTier:       models.TraceResponseTierFallback,
					Signature:          signature,
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
						StatusCode: aiResp.StatusCode,
						Headers:    headersToMap(w.Header()),
						Body:       aiResp.Body,
					},
				})
			}
			return
		case models.SpecModeProxy:
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
				statusCode = http.StatusBadGateway
				respBody = `{"error":"proxy backend unavailable: ` + proxyErr.Error() + `"}`
				http.Error(w, respBody, statusCode)
			} else {
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
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "No matching response configuration for this request"}`))
		return
	}

	// Resolve session and run scripts only once we know we are serving a virtual response.
	var sess *store.Session
	var sessionIsNew bool
	if e.sessionManager != nil {
		rawSessionID := r.Header.Get(e.sessionHeaderName)
		sess, sessionIsNew = e.sessionManager.GetOrCreate(rawSessionID)
		w.Header().Set(e.sessionHeaderName, sess.ID)
	}

	var scriptOutput map[string]any
	var scriptTraces []models.ScriptTrace
	scriptInput := scripting.BuildInput(pathParams, r, requestBody)
	scriptOutput, scriptTraces = e.scriptEngine.RunBindings(r.Context(), matchedRoute.operation.ID, scriptInput, sess)

	// Apply delay if configured
	if matchedConfig.Delay > 0 {
		time.Sleep(time.Duration(matchedConfig.Delay) * time.Millisecond)
	}

	// Build template context
	seed := buildTemplateSeed(r, pathParams)
	templateCtx := &template.Context{
		PathParams:   pathParams,
		QueryParams:  r.URL.Query(),
		Headers:      r.Header,
		Body:         requestBody,
		RNG:          rand.New(rand.NewSource(seed)),
		ScriptOutput: scriptOutput,
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

	// Record trace if tracing is enabled
	if matchedRoute.spec.Tracing {
		var sessionTrace *models.SessionTrace
		if sess != nil {
			sessionTrace = &models.SessionTrace{
				ID:    sess.ID,
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

	log.Printf("proxy: %s fallback enabled for spec %q (%s) but skipped: %s", mode, spec.Name, spec.ID, reason)
}

func (e *Engine) resetRuntimeWarnings() {
	e.warningMu.Lock()
	defer e.warningMu.Unlock()
	e.runtimeWarnings = make(map[string]struct{})
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
