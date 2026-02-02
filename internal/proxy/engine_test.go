package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

func setupTestEngine(t *testing.T) (*Engine, storage.Storage) {
	store := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)
	
	engine := NewEngine(store, collector, tracingSvc)
	return engine, store
}

func TestNewEngine(t *testing.T) {
	engine, _ := setupTestEngine(t)
	
	if engine == nil {
		t.Fatal("Expected engine to be created")
	}
	
	if engine.condEvaluator == nil {
		t.Error("Expected condition evaluator to be initialized")
	}
	
	if engine.templateEngine == nil {
		t.Error("Expected template engine to be initialized")
	}
	
	if engine.routes == nil {
		t.Error("Expected routes map to be initialized")
	}
}

func TestBuildPathPattern(t *testing.T) {
	tests := []struct {
		name        string
		basePath    string
		pathPattern string
		testPath    string
		shouldMatch bool
		expectedParams map[string]string
	}{
		{
			name:        "simple path",
			basePath:    "",
			pathPattern: "/users",
			testPath:    "/users",
			shouldMatch: true,
			expectedParams: map[string]string{},
		},
		{
			name:        "path with single param",
			basePath:    "",
			pathPattern: "/users/{id}",
			testPath:    "/users/123",
			shouldMatch: true,
			expectedParams: map[string]string{"id": "123"},
		},
		{
			name:        "path with multiple params",
			basePath:    "",
			pathPattern: "/users/{userId}/posts/{postId}",
			testPath:    "/users/42/posts/99",
			shouldMatch: true,
			expectedParams: map[string]string{"userId": "42", "postId": "99"},
		},
		{
			name:        "path with base path",
			basePath:    "/api/v1",
			pathPattern: "/users",
			testPath:    "/api/v1/users",
			shouldMatch: true,
			expectedParams: map[string]string{},
		},
		{
			name:        "path with base path and param",
			basePath:    "/api/v1",
			pathPattern: "/users/{id}",
			testPath:    "/api/v1/users/abc",
			shouldMatch: true,
			expectedParams: map[string]string{"id": "abc"},
		},
		{
			name:        "no match - wrong path",
			basePath:    "",
			pathPattern: "/users",
			testPath:    "/posts",
			shouldMatch: false,
			expectedParams: nil,
		},
		{
			name:        "no match - extra segments",
			basePath:    "",
			pathPattern: "/users",
			testPath:    "/users/extra",
			shouldMatch: false,
			expectedParams: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, paramKeys := buildPathPattern(tt.basePath, tt.pathPattern)
			
			matches := pattern.FindStringSubmatch(tt.testPath)
			
			if tt.shouldMatch {
				if matches == nil {
					t.Errorf("Expected path %q to match pattern", tt.testPath)
					return
				}
				
				// Extract params
				params := make(map[string]string)
				for i, key := range paramKeys {
					if i+1 < len(matches) {
						params[key] = matches[i+1]
					}
				}
				
				for key, expected := range tt.expectedParams {
					if actual, ok := params[key]; !ok || actual != expected {
						t.Errorf("Expected param %q = %q, got %q", key, expected, actual)
					}
				}
			} else {
				if matches != nil {
					t.Errorf("Expected path %q not to match pattern", tt.testPath)
				}
			}
		})
	}
}

func TestSortRoutes(t *testing.T) {
	routes := []*route{
		{
			operation: &models.Operation{Path: "/users/{id}"},
			paramKeys: []string{"id"},
		},
		{
			operation: &models.Operation{Path: "/users"},
			paramKeys: []string{},
		},
		{
			operation: &models.Operation{Path: "/users/{id}/posts/{postId}"},
			paramKeys: []string{"id", "postId"},
		},
	}
	
	sortRoutes(routes)
	
	// Routes with fewer params should come first
	if len(routes[0].paramKeys) != 0 {
		t.Errorf("Expected first route to have 0 params, got %d", len(routes[0].paramKeys))
	}
	if len(routes[1].paramKeys) != 1 {
		t.Errorf("Expected second route to have 1 param, got %d", len(routes[1].paramKeys))
	}
	if len(routes[2].paramKeys) != 2 {
		t.Errorf("Expected third route to have 2 params, got %d", len(routes[2].paramKeys))
	}
}

func TestReloadRoutes(t *testing.T) {
	engine, store := setupTestEngine(t)
	
	// Add a spec and operations
	spec := &models.Spec{
		ID:       "spec-1",
		Name:     "Test API",
		BasePath: "/api",
		Enabled:  true,
	}
	store.CreateSpec(spec)
	
	op1 := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users",
		FullPath: "/api/users",
	}
	op2 := &models.Operation{
		ID:       "op-2",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users/{id}",
		FullPath: "/api/users/{id}",
	}
	store.CreateOperation(op1)
	store.CreateOperation(op2)
	
	// Reload routes
	err := engine.ReloadRoutes()
	if err != nil {
		t.Fatalf("ReloadRoutes failed: %v", err)
	}
	
	// Check routes were loaded
	routes := engine.GetRegisteredRoutes()
	if len(routes["GET"]) != 2 {
		t.Errorf("Expected 2 GET routes, got %d", len(routes["GET"]))
	}
}

func TestReloadRoutes_DisabledSpec(t *testing.T) {
	engine, store := setupTestEngine(t)
	
	// Add a disabled spec
	spec := &models.Spec{
		ID:       "spec-1",
		Name:     "Test API",
		BasePath: "/api",
		Enabled:  false, // Disabled
	}
	store.CreateSpec(spec)
	
	op := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users",
		FullPath: "/api/users",
	}
	store.CreateOperation(op)
	
	// Reload routes
	err := engine.ReloadRoutes()
	if err != nil {
		t.Fatalf("ReloadRoutes failed: %v", err)
	}
	
	// Check no routes were loaded (spec is disabled)
	routes := engine.GetRegisteredRoutes()
	if len(routes["GET"]) != 0 {
		t.Errorf("Expected 0 GET routes for disabled spec, got %d", len(routes["GET"]))
	}
}

func TestMatchRoute(t *testing.T) {
	engine, store := setupTestEngine(t)
	
	// Add a spec and operations
	spec := &models.Spec{
		ID:       "spec-1",
		Name:     "Test API",
		BasePath: "/api",
		Enabled:  true,
	}
	store.CreateSpec(spec)
	
	op1 := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users",
		FullPath: "/api/users",
	}
	op2 := &models.Operation{
		ID:       "op-2",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users/{id}",
		FullPath: "/api/users/{id}",
	}
	op3 := &models.Operation{
		ID:       "op-3",
		SpecID:   "spec-1",
		Method:   "POST",
		Path:     "/users",
		FullPath: "/api/users",
	}
	store.CreateOperation(op1)
	store.CreateOperation(op2)
	store.CreateOperation(op3)
	
	engine.ReloadRoutes()
	
	tests := []struct {
		name           string
		method         string
		path           string
		expectedOp     string
		expectedParams map[string]string
	}{
		{
			name:           "match simple path",
			method:         "GET",
			path:           "/api/users",
			expectedOp:     "op-1",
			expectedParams: map[string]string{},
		},
		{
			name:           "match path with param",
			method:         "GET",
			path:           "/api/users/123",
			expectedOp:     "op-2",
			expectedParams: map[string]string{"id": "123"},
		},
		{
			name:           "match POST method",
			method:         "POST",
			path:           "/api/users",
			expectedOp:     "op-3",
			expectedParams: map[string]string{},
		},
		{
			name:           "no match - wrong method",
			method:         "DELETE",
			path:           "/api/users",
			expectedOp:     "",
			expectedParams: nil,
		},
		{
			name:           "no match - wrong path",
			method:         "GET",
			path:           "/api/posts",
			expectedOp:     "",
			expectedParams: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, params, err := engine.MatchRoute(tt.method, tt.path)
			if err != nil {
				t.Fatalf("MatchRoute failed: %v", err)
			}
			
			if tt.expectedOp == "" {
				if op != nil {
					t.Errorf("Expected no match, got operation %s", op.ID)
				}
				return
			}
			
			if op == nil {
				t.Fatalf("Expected operation %s, got nil", tt.expectedOp)
			}
			
			if op.ID != tt.expectedOp {
				t.Errorf("Expected operation %s, got %s", tt.expectedOp, op.ID)
			}
			
			for key, expected := range tt.expectedParams {
				if actual, ok := params[key]; !ok || actual != expected {
					t.Errorf("Expected param %q = %q, got %q", key, expected, actual)
				}
			}
		})
	}
}

func TestServeHTTP_NotFound(t *testing.T) {
	engine, _ := setupTestEngine(t)
	
	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	
	engine.ServeHTTP(w, req)
	
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestServeHTTP_ExampleFallback(t *testing.T) {
	engine, store := setupTestEngine(t)
	
	// Add a spec with example fallback enabled
	spec := &models.Spec{
		ID:                 "spec-1",
		Name:               "Test API",
		BasePath:           "/api",
		Enabled:            true,
		UseExampleFallback: true,
	}
	store.CreateSpec(spec)
	
	op := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users",
		FullPath: "/api/users",
		ExampleResponse: &models.ExampleResponse{
			StatusCode: 200,
			Headers:    map[string]string{"X-Custom": "header"},
			Body:       `{"users": []}`,
		},
	}
	store.CreateOperation(op)
	
	engine.ReloadRoutes()
	
	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()
	
	engine.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	body := w.Body.String()
	if body != `{"users": []}` {
		t.Errorf("Unexpected body: %s", body)
	}
	
	if w.Header().Get("X-Custom") != "header" {
		t.Errorf("Expected X-Custom header to be 'header', got %q", w.Header().Get("X-Custom"))
	}
}

func TestServeHTTP_ResponseConfig(t *testing.T) {
	engine, store := setupTestEngine(t)
	
	// Add a spec
	spec := &models.Spec{
		ID:                 "spec-1",
		Name:               "Test API",
		BasePath:           "/api",
		Enabled:            true,
		UseExampleFallback: false,
	}
	store.CreateSpec(spec)
	
	op := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users/{id}",
		FullPath: "/api/users/{id}",
	}
	store.CreateOperation(op)
	
	// Add a response config
	config := &models.ResponseConfig{
		ID:          "config-1",
		OperationID: "op-1",
		Name:        "Default",
		StatusCode:  200,
		Headers:     map[string]string{"Content-Type": "application/json"},
		Body:        `{"id": "{{.path.id}}", "name": "User"}`,
		Priority:    1,
		Enabled:     true,
		Conditions:  []models.Condition{},
	}
	store.CreateResponseConfig(config)
	
	engine.ReloadRoutes()
	
	req := httptest.NewRequest("GET", "/api/users/42", nil)
	w := httptest.NewRecorder()
	
	engine.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	body := w.Body.String()
	if !strings.Contains(body, `"id": "42"`) {
		t.Errorf("Expected body to contain templated id, got: %s", body)
	}
}

func TestServeHTTP_ConditionalResponse(t *testing.T) {
	engine, store := setupTestEngine(t)
	
	// Add a spec
	spec := &models.Spec{
		ID:       "spec-1",
		Name:     "Test API",
		BasePath: "/api",
		Enabled:  true,
	}
	store.CreateSpec(spec)
	
	op := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users/{id}",
		FullPath: "/api/users/{id}",
	}
	store.CreateOperation(op)
	
	// Add two response configs with conditions
	configNotFound := &models.ResponseConfig{
		ID:          "config-notfound",
		OperationID: "op-1",
		Name:        "Not Found",
		StatusCode:  404,
		Body:        `{"error": "User not found"}`,
		Priority:    1,
		Enabled:     true,
		Conditions: []models.Condition{
			{Source: "path", Key: "id", Operator: "eq", Value: "999"},
		},
	}
	configDefault := &models.ResponseConfig{
		ID:          "config-default",
		OperationID: "op-1",
		Name:        "Default",
		StatusCode:  200,
		Body:        `{"id": "{{.path.id}}"}`,
		Priority:    2,
		Enabled:     true,
		Conditions:  []models.Condition{},
	}
	store.CreateResponseConfig(configNotFound)
	store.CreateResponseConfig(configDefault)
	
	engine.ReloadRoutes()
	
	// Test matching condition (404)
	req := httptest.NewRequest("GET", "/api/users/999", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
	
	// Test non-matching condition (200)
	req = httptest.NewRequest("GET", "/api/users/123", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestServeHTTP_ResponseDelay(t *testing.T) {
	engine, store := setupTestEngine(t)
	
	// Add a spec
	spec := &models.Spec{
		ID:       "spec-1",
		Name:     "Test API",
		BasePath: "/api",
		Enabled:  true,
	}
	store.CreateSpec(spec)
	
	op := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users",
		FullPath: "/api/users",
	}
	store.CreateOperation(op)
	
	// Add a response config with delay
	config := &models.ResponseConfig{
		ID:          "config-1",
		OperationID: "op-1",
		Name:        "Delayed",
		StatusCode:  200,
		Body:        `{"ok": true}`,
		Priority:    1,
		Enabled:     true,
		Delay:       100, // 100ms delay
	}
	store.CreateResponseConfig(config)
	
	engine.ReloadRoutes()
	
	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()
	
	start := time.Now()
	engine.ServeHTTP(w, req)
	elapsed := time.Since(start)
	
	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected delay of at least 100ms, got %v", elapsed)
	}
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestServeHTTP_TracingEnabled(t *testing.T) {
	store := storage.NewMemoryStorage()
	collector := stats.NewCollector()
	tracingSvc := tracing.NewService(100)
	engine := NewEngine(store, collector, tracingSvc)
	
	// Add a spec with tracing enabled
	spec := &models.Spec{
		ID:                 "spec-1",
		Name:               "Test API",
		BasePath:           "/api",
		Enabled:            true,
		Tracing:            true, // Enable tracing
		UseExampleFallback: true,
	}
	store.CreateSpec(spec)
	
	op := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users",
		FullPath: "/api/users",
		ExampleResponse: &models.ExampleResponse{
			StatusCode: 200,
			Body:       `{"users": []}`,
		},
	}
	store.CreateOperation(op)
	
	engine.ReloadRoutes()
	
	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	
	// Check that trace was recorded
	traces := tracingSvc.GetTraces(nil)
	if len(traces) != 1 {
		t.Errorf("Expected 1 trace, got %d", len(traces))
	}
	
	if len(traces) > 0 {
		trace := traces[0]
		if trace.SpecID != "spec-1" {
			t.Errorf("Expected trace spec ID 'spec-1', got %q", trace.SpecID)
		}
		if trace.Request.Method != "GET" {
			t.Errorf("Expected trace method 'GET', got %q", trace.Request.Method)
		}
	}
}

func TestServeHTTP_RequestBody(t *testing.T) {
	engine, store := setupTestEngine(t)
	
	// Add a spec
	spec := &models.Spec{
		ID:       "spec-1",
		Name:     "Test API",
		BasePath: "/api",
		Enabled:  true,
	}
	store.CreateSpec(spec)
	
	op := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "POST",
		Path:     "/users",
		FullPath: "/api/users",
	}
	store.CreateOperation(op)
	
	// Add a response config that uses body
	config := &models.ResponseConfig{
		ID:          "config-1",
		OperationID: "op-1",
		Name:        "Echo Name",
		StatusCode:  201,
		Body:        `{"created": "{{.body.name}}"}`,
		Priority:    1,
		Enabled:     true,
	}
	store.CreateResponseConfig(config)
	
	engine.ReloadRoutes()
	
	reqBody := `{"name": "John Doe", "email": "john@example.com"}`
	req := httptest.NewRequest("POST", "/api/users", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	engine.ServeHTTP(w, req)
	
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
	
	body := w.Body.String()
	if !strings.Contains(body, `"created": "John Doe"`) {
		t.Errorf("Expected body to contain 'John Doe', got: %s", body)
	}
}

func TestServeHTTP_QueryParams(t *testing.T) {
	engine, store := setupTestEngine(t)
	
	// Add a spec
	spec := &models.Spec{
		ID:       "spec-1",
		Name:     "Test API",
		BasePath: "/api",
		Enabled:  true,
	}
	store.CreateSpec(spec)
	
	op := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/search",
		FullPath: "/api/search",
	}
	store.CreateOperation(op)
	
	// Add a response config that uses query params
	config := &models.ResponseConfig{
		ID:          "config-1",
		OperationID: "op-1",
		Name:        "Search",
		StatusCode:  200,
		Body:        `{"query": "{{.query.q}}"}`,
		Priority:    1,
		Enabled:     true,
	}
	store.CreateResponseConfig(config)
	
	engine.ReloadRoutes()
	
	req := httptest.NewRequest("GET", "/api/search?q=test+term", nil)
	w := httptest.NewRecorder()
	
	engine.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	body := w.Body.String()
	if !strings.Contains(body, `"query": "test term"`) {
		t.Errorf("Expected body to contain query param, got: %s", body)
	}
}

func TestServeHTTP_Headers(t *testing.T) {
	engine, store := setupTestEngine(t)
	
	// Add a spec
	spec := &models.Spec{
		ID:       "spec-1",
		Name:     "Test API",
		BasePath: "/api",
		Enabled:  true,
	}
	store.CreateSpec(spec)
	
	op := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/whoami",
		FullPath: "/api/whoami",
	}
	store.CreateOperation(op)
	
	// Add a response config that uses headers
	config := &models.ResponseConfig{
		ID:          "config-1",
		OperationID: "op-1",
		Name:        "WhoAmI",
		StatusCode:  200,
		Body:        `{"user": "{{.header.X-User-Id}}"}`,
		Priority:    1,
		Enabled:     true,
	}
	store.CreateResponseConfig(config)
	
	engine.ReloadRoutes()
	
	req := httptest.NewRequest("GET", "/api/whoami", nil)
	req.Header.Set("X-User-Id", "user-12345")
	w := httptest.NewRecorder()
	
	engine.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	body := w.Body.String()
	if !strings.Contains(body, `"user": "user-12345"`) {
		t.Errorf("Expected body to contain header value, got: %s", body)
	}
}

func TestHandler(t *testing.T) {
	engine, _ := setupTestEngine(t)
	
	handler := engine.Handler()
	if handler == nil {
		t.Error("Expected handler to be returned")
	}
	
	// Verify it implements http.Handler
	_ = handler.(http.Handler)
}

func TestHeadersToMap(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Add("Accept", "text/html")
	h.Add("Accept", "application/json")
	
	result := headersToMap(h)
	
	if len(result) != 2 {
		t.Errorf("Expected 2 header keys, got %d", len(result))
	}
	
	if result["Content-Type"][0] != "application/json" {
		t.Errorf("Unexpected Content-Type: %v", result["Content-Type"])
	}
	
	if len(result["Accept"]) != 2 {
		t.Errorf("Expected 2 Accept values, got %d", len(result["Accept"]))
	}
}

func TestServeHTTP_NoMatchingConfig(t *testing.T) {
	engine, store := setupTestEngine(t)
	
	// Add a spec with example fallback disabled
	spec := &models.Spec{
		ID:                 "spec-1",
		Name:               "Test API",
		BasePath:           "/api",
		Enabled:            true,
		UseExampleFallback: false,
	}
	store.CreateSpec(spec)
	
	op := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users",
		FullPath: "/api/users",
		// No example response
	}
	store.CreateOperation(op)
	
	// No response configs
	
	engine.ReloadRoutes()
	
	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()
	
	engine.ServeHTTP(w, req)
	
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
	
	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), "No matching response configuration") {
		t.Errorf("Expected error message, got: %s", body)
	}
}

func TestGetRegisteredRoutes(t *testing.T) {
	engine, store := setupTestEngine(t)
	
	// Add a spec and operations
	spec := &models.Spec{
		ID:       "spec-1",
		Name:     "Test API",
		BasePath: "/api",
		Enabled:  true,
	}
	store.CreateSpec(spec)
	
	op1 := &models.Operation{
		ID:       "op-1",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/users",
		FullPath: "/api/users",
	}
	op2 := &models.Operation{
		ID:       "op-2",
		SpecID:   "spec-1",
		Method:   "POST",
		Path:     "/users",
		FullPath: "/api/users",
	}
	op3 := &models.Operation{
		ID:       "op-3",
		SpecID:   "spec-1",
		Method:   "GET",
		Path:     "/posts",
		FullPath: "/api/posts",
	}
	store.CreateOperation(op1)
	store.CreateOperation(op2)
	store.CreateOperation(op3)
	
	engine.ReloadRoutes()
	
	routes := engine.GetRegisteredRoutes()
	
	if len(routes["GET"]) != 2 {
		t.Errorf("Expected 2 GET routes, got %d", len(routes["GET"]))
	}
	if len(routes["POST"]) != 1 {
		t.Errorf("Expected 1 POST route, got %d", len(routes["POST"]))
	}
}
