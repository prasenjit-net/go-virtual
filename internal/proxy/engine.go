package proxy

import (
	"io"
	"net/http"
	"path"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/prasenjit/go-virtual/internal/condition"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/template"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

// Engine handles proxying requests to virtual API endpoints
type Engine struct {
	store           storage.Storage
	statsCollector  *stats.Collector
	tracingService  *tracing.Service
	condEvaluator   *condition.Evaluator
	templateEngine  *template.Engine
	mu              sync.RWMutex
	routes          map[string][]*route // method -> routes
}

// route represents a registered route
type route struct {
	spec      *models.Spec
	operation *models.Operation
	pattern   *regexp.Regexp
	paramKeys []string
}

// NewEngine creates a new proxy engine
func NewEngine(store storage.Storage, statsCollector *stats.Collector, tracingService *tracing.Service) *Engine {
	e := &Engine{
		store:          store,
		statsCollector: statsCollector,
		tracingService: tracingService,
		condEvaluator:  condition.NewEvaluator(),
		templateEngine: template.NewEngine(),
		routes:         make(map[string][]*route),
	}

	// Load initial routes
	e.ReloadRoutes()

	return e
}

// ReloadRoutes reloads all routes from enabled specs
func (e *Engine) ReloadRoutes() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear existing routes
	e.routes = make(map[string][]*route)

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

	// Find matching route
	e.mu.RLock()
	matchedRoute, pathParams := e.matchRoute(r.Method, r.URL.Path)
	e.mu.RUnlock()

	if matchedRoute == nil {
		http.NotFound(w, r)
		return
	}

	// Read request body
	var requestBody string
	if r.Body != nil {
		bodyBytes, _ := io.ReadAll(r.Body)
		requestBody = string(bodyBytes)
	}

	// Build request data for condition evaluation
	reqData := &condition.RequestData{
		PathParams:  pathParams,
		QueryParams: r.URL.Query(),
		Headers:     r.Header,
		Body:        requestBody,
	}

	// Get response configs for the operation
	responseConfigs, err := e.store.GetResponseConfigsByOperation(matchedRoute.operation.ID)
	
	// Find matching response config by priority (only if configs exist)
	var matchedConfig *models.ResponseConfig
	if err == nil && len(responseConfigs) > 0 {
		for _, cfg := range responseConfigs {
			if !cfg.Enabled {
				continue
			}
			if e.condEvaluator.EvaluateAll(cfg.Conditions, reqData) {
				matchedConfig = cfg
				break
			}
		}
	}

	// If no matching config found, try to use example response from OpenAPI spec
	if matchedConfig == nil && matchedRoute.operation.ExampleResponse != nil {
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
		
		// Record trace if enabled
		if matchedRoute.spec.Tracing {
			trace := &models.Trace{
				SpecID:        matchedRoute.spec.ID,
				SpecName:      matchedRoute.spec.Name,
				OperationID:   matchedRoute.operation.ID,
				OperationPath: matchedRoute.operation.Path,
				Timestamp:     startTime,
				Duration:      duration.Nanoseconds(),
				MatchedConfig: "spec-example",
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
		w.Write([]byte(`{"error": "No matching response configuration and no example in spec"}`))
		return
	}

	// Apply delay if configured
	if matchedConfig.Delay > 0 {
		time.Sleep(time.Duration(matchedConfig.Delay) * time.Millisecond)
	}

	// Build template context
	templateCtx := &template.Context{
		PathParams:  pathParams,
		QueryParams: r.URL.Query(),
		Headers:     r.Header,
		Body:        requestBody,
	}

	// Process headers
	responseHeaders := e.templateEngine.ProcessHeaders(matchedConfig.Headers, templateCtx)
	for key, value := range responseHeaders {
		w.Header().Set(key, value)
	}

	// Set default content-type if not set
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}

	// Process body
	responseBody := e.templateEngine.Process(matchedConfig.Body, templateCtx)

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

	// Record trace if tracing is enabled
	if matchedRoute.spec.Tracing {
		trace := &models.Trace{
			SpecID:          matchedRoute.spec.ID,
			SpecName:        matchedRoute.spec.Name,
			OperationID:     matchedRoute.operation.ID,
			OperationPath:   matchedRoute.operation.Path,
			Timestamp:       startTime,
			Duration:        duration.Nanoseconds(),
			MatchedConfigID: matchedConfig.ID,
			MatchedConfig:   matchedConfig.Name,
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
