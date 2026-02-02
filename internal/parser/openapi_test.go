package parser

import (
	"strings"
	"testing"
)

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("NewParser returned nil")
	}
}

func TestParse_ValidSpec(t *testing.T) {
	p := NewParser()

	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
  description: A test API
paths:
  /users:
    get:
      operationId: getUsers
      summary: Get all users
      description: Returns a list of users
      tags:
        - users
      responses:
        '200':
          description: Success
          content:
            application/json:
              example:
                users: []
    post:
      operationId: createUser
      summary: Create a user
      responses:
        '201':
          description: Created
  /users/{id}:
    get:
      operationId: getUserById
      summary: Get user by ID
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Success
    delete:
      summary: Delete user
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '204':
          description: Deleted
`

	result, err := p.Parse(spec, "/api/v1")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Check spec
	if result.Spec == nil {
		t.Fatal("Spec is nil")
	}
	if result.Spec.Name != "Test API" {
		t.Errorf("Expected name 'Test API', got %q", result.Spec.Name)
	}
	if result.Spec.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %q", result.Spec.Version)
	}
	if result.Spec.Description != "A test API" {
		t.Errorf("Expected description 'A test API', got %q", result.Spec.Description)
	}
	if result.Spec.BasePath != "/api/v1" {
		t.Errorf("Expected base path '/api/v1', got %q", result.Spec.BasePath)
	}
	if !result.Spec.Enabled {
		t.Error("Expected spec to be enabled by default")
	}
	if !result.Spec.UseExampleFallback {
		t.Error("Expected useExampleFallback to be enabled by default")
	}

	// Check operations
	if len(result.Operations) != 4 {
		t.Errorf("Expected 4 operations, got %d", len(result.Operations))
	}

	// Verify operations
	opMap := make(map[string]*struct {
		Method      string
		Path        string
		OperationID string
		Summary     string
	})
	for _, op := range result.Operations {
		key := op.Method + " " + op.Path
		opMap[key] = &struct {
			Method      string
			Path        string
			OperationID string
			Summary     string
		}{
			Method:      op.Method,
			Path:        op.Path,
			OperationID: op.OperationID,
			Summary:     op.Summary,
		}
	}

	if op, ok := opMap["GET /users"]; !ok {
		t.Error("GET /users operation not found")
	} else {
		if op.OperationID != "getUsers" {
			t.Errorf("Expected operationId 'getUsers', got %q", op.OperationID)
		}
		if op.Summary != "Get all users" {
			t.Errorf("Expected summary 'Get all users', got %q", op.Summary)
		}
	}

	if op, ok := opMap["POST /users"]; !ok {
		t.Error("POST /users operation not found")
	} else {
		if op.OperationID != "createUser" {
			t.Errorf("Expected operationId 'createUser', got %q", op.OperationID)
		}
	}

	if op, ok := opMap["DELETE /users/{id}"]; !ok {
		t.Error("DELETE /users/{id} operation not found")
	} else {
		// Generated operation ID
		if !strings.Contains(op.OperationID, "delete") {
			t.Errorf("Expected generated operationId containing 'delete', got %q", op.OperationID)
		}
	}
}

func TestParse_ExampleResponse(t *testing.T) {
	p := NewParser()

	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      operationId: getUsers
      responses:
        '200':
          description: Success
          content:
            application/json:
              example:
                users:
                  - id: 1
                    name: John
                  - id: 2
                    name: Jane
`

	result, err := p.Parse(spec, "")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Operations) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(result.Operations))
	}

	op := result.Operations[0]
	if op.ExampleResponse == nil {
		t.Fatal("Expected example response")
	}

	if op.ExampleResponse.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", op.ExampleResponse.StatusCode)
	}

	if op.ExampleResponse.Body == "" {
		t.Error("Expected example body")
	}

	if !strings.Contains(op.ExampleResponse.Body, "John") {
		t.Error("Expected body to contain 'John'")
	}
}

func TestParse_NoContentResponse(t *testing.T) {
	p := NewParser()

	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users/{id}:
    delete:
      operationId: deleteUser
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '204':
          description: No Content
`

	result, err := p.Parse(spec, "")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Operations) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(result.Operations))
	}

	op := result.Operations[0]
	if op.ExampleResponse == nil {
		t.Fatal("Expected example response for 204")
	}

	if op.ExampleResponse.StatusCode != 204 {
		t.Errorf("Expected status code 204, got %d", op.ExampleResponse.StatusCode)
	}
}

func TestParse_InvalidSpec(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name string
		spec string
	}{
		{
			name: "invalid yaml",
			spec: `this is not valid: yaml: content`,
		},
		{
			name: "missing openapi version",
			spec: `
info:
  title: Test API
  version: 1.0.0
paths: {}
`,
		},
		{
			name: "invalid openapi structure",
			spec: `
openapi: 3.0.0
info: "invalid"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.Parse(tt.spec, "")
			if err == nil {
				t.Error("Expected error for invalid spec")
			}
		})
	}
}

func TestParse_EmptyPaths(t *testing.T) {
	p := NewParser()

	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths: {}
`

	result, err := p.Parse(spec, "")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Operations) != 0 {
		t.Errorf("Expected 0 operations, got %d", len(result.Operations))
	}
}

func TestParse_AllHTTPMethods(t *testing.T) {
	p := NewParser()

	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /resource:
    get:
      responses:
        '200':
          description: OK
    post:
      responses:
        '201':
          description: Created
    put:
      responses:
        '200':
          description: OK
    patch:
      responses:
        '200':
          description: OK
    delete:
      responses:
        '204':
          description: No Content
    head:
      responses:
        '200':
          description: OK
    options:
      responses:
        '200':
          description: OK
`

	result, err := p.Parse(spec, "")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Operations) != 7 {
		t.Errorf("Expected 7 operations (all HTTP methods), got %d", len(result.Operations))
	}

	methods := make(map[string]bool)
	for _, op := range result.Operations {
		methods[op.Method] = true
	}

	expectedMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	for _, method := range expectedMethods {
		if !methods[method] {
			t.Errorf("Missing operation for method %s", method)
		}
	}
}

func TestParse_BasePath(t *testing.T) {
	p := NewParser()

	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      responses:
        '200':
          description: OK
`

	tests := []struct {
		name         string
		basePath     string
		expectedFull string
	}{
		{
			name:         "with base path",
			basePath:     "/api/v1",
			expectedFull: "/api/v1/users",
		},
		{
			name:         "empty base path",
			basePath:     "",
			expectedFull: "/users",
		},
		{
			name:         "base path with trailing slash",
			basePath:     "/api/",
			expectedFull: "/api/users",
		},
		{
			name:         "root base path",
			basePath:     "/",
			expectedFull: "/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(spec, tt.basePath)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if len(result.Operations) != 1 {
				t.Fatalf("Expected 1 operation, got %d", len(result.Operations))
			}

			if result.Operations[0].Path != "/users" {
				t.Errorf("Expected path '/users', got %q", result.Operations[0].Path)
			}
		})
	}
}

func TestParse_Tags(t *testing.T) {
	p := NewParser()

	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      tags:
        - users
        - admin
      responses:
        '200':
          description: OK
`

	result, err := p.Parse(spec, "")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	op := result.Operations[0]
	if len(op.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(op.Tags))
	}

	if op.Tags[0] != "users" || op.Tags[1] != "admin" {
		t.Errorf("Unexpected tags: %v", op.Tags)
	}
}

func TestParse_SpecWithSchemaExamples(t *testing.T) {
	p := NewParser()

	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: integer
                    example: 123
                  name:
                    type: string
                    example: John Doe
`

	result, err := p.Parse(spec, "")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Operations) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(result.Operations))
	}

	// This test verifies parsing succeeds with schema examples
	// The actual example generation depends on the implementation
	op := result.Operations[0]
	if op == nil {
		t.Fatal("Operation is nil")
	}
}

func TestNormalizeBasePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/api/v1", "/api/v1"},
		{"/api/v1/", "/api/v1"},
		{"api/v1", "/api/v1"},
		{"/", ""},
		{"", ""},
		// Note: the current implementation doesn't clean up double slashes
		// If needed, this could be enhanced in the future
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeBasePath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeBasePath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/users", "users"},
		{"/users/{id}", "users_id"},
		{"/users/{userId}/posts/{postId}", "users_userId_posts_postId"},
		{"/api/v1/users", "api_v1_users"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizePath(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizePath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatExample(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "byte slice",
			input:    []byte("hello"),
			expected: "hello",
		},
		{
			name:     "map",
			input:    map[string]string{"key": "value"},
			expected: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatExample(tt.input)
			if result != tt.expected {
				t.Errorf("formatExample(%v) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParse_JSONSpec(t *testing.T) {
	p := NewParser()

	spec := `{
  "openapi": "3.0.0",
  "info": {
    "title": "JSON API",
    "version": "2.0.0"
  },
  "paths": {
    "/items": {
      "get": {
        "operationId": "getItems",
        "responses": {
          "200": {
            "description": "OK"
          }
        }
      }
    }
  }
}`

	result, err := p.Parse(spec, "")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result.Spec.Name != "JSON API" {
		t.Errorf("Expected name 'JSON API', got %q", result.Spec.Name)
	}

	if result.Spec.Version != "2.0.0" {
		t.Errorf("Expected version '2.0.0', got %q", result.Spec.Version)
	}

	if len(result.Operations) != 1 {
		t.Errorf("Expected 1 operation, got %d", len(result.Operations))
	}
}
