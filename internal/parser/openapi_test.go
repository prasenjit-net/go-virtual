package parser

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
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

func TestParseOperations(t *testing.T) {
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
  /users/{id}:
    delete:
      responses:
        '204':
          description: No Content
`

	ops, err := p.ParseOperations(spec, "spec-1", "/api")
	if err != nil {
		t.Fatalf("ParseOperations error: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(ops))
	}

	if ops[0].SpecID != "spec-1" {
		t.Fatalf("expected spec ID to be set")
	}
}

func TestGenerateOperationID(t *testing.T) {
	first := generateOperationID("spec-1", "GET", "/users")
	second := generateOperationID("spec-1", "GET", "/users")
	third := generateOperationID("spec-1", "POST", "/users")

	if first != second {
		t.Fatalf("expected deterministic operation IDs, got %q and %q", first, second)
	}
	if first == third {
		t.Fatalf("expected different operation IDs for different methods")
	}
	if len(first) != 32 {
		t.Fatalf("expected operation ID length 32, got %d", len(first))
	}
}

func TestGenerateExampleFromSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   *openapi3.Schema
		expected string
	}{
		{
			name:     "string",
			schema:   &openapi3.Schema{Type: &openapi3.Types{"string"}},
			expected: `"string"`,
		},
		{
			name:     "object",
			schema:   &openapi3.Schema{Type: &openapi3.Types{"object"}},
			expected: `{}`,
		},
		{
			name:     "array",
			schema:   &openapi3.Schema{Type: &openapi3.Types{"array"}},
			expected: `[]`,
		},
		{
			name:     "integer",
			schema:   &openapi3.Schema{Type: &openapi3.Types{"integer"}},
			expected: `0`,
		},
		{
			name:     "number",
			schema:   &openapi3.Schema{Type: &openapi3.Types{"number"}},
			expected: `0.0`,
		},
		{
			name:     "boolean",
			schema:   &openapi3.Schema{Type: &openapi3.Types{"boolean"}},
			expected: `false`,
		},
		{
			name:     "no type (oneOf/allOf/anyOf)",
			schema:   &openapi3.Schema{},
			expected: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateExampleFromSchema(tt.schema); got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestExtractExampleResponse(t *testing.T) {
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
          headers:
            X-Rate-Limit:
              schema:
                type: string
              example: "100"
          content:
            application/json:
              example:
                ok: true
`

	headers, body, err := p.ExtractExampleResponse(spec, "GET", "/users", 200)
	if err != nil {
		t.Fatalf("ExtractExampleResponse error: %v", err)
	}
	if headers["X-Rate-Limit"] != "100" {
		t.Fatalf("expected header X-Rate-Limit=100, got %q", headers["X-Rate-Limit"])
	}
	if headers["Content-Type"] != "application/json" {
		t.Fatalf("expected content type header")
	}
	if !strings.Contains(body, "ok") {
		t.Fatalf("expected body to include example data")
	}

	if _, _, err := p.ExtractExampleResponse(spec, "GET", "/missing", 200); err == nil {
		t.Fatalf("expected error for missing path")
	}
	if _, _, err := p.ExtractExampleResponse(spec, "TRACE", "/users", 200); err == nil {
		t.Fatalf("expected error for unsupported method")
	}
}

// ── Shared fixture spec used by the new function tests ────────────────────────

const fixtureSpec = `
openapi: 3.0.0
info:
  title: Fixture API
  version: 1.0.0
paths:
  /pets/{petId}:
    get:
      operationId: getPetById
      summary: Get a pet
      parameters:
        - name: petId
          in: path
          required: true
          description: The pet ID
          schema:
            type: string
        - name: format
          in: query
          required: false
          description: Response format
          schema:
            type: string
      responses:
        '200':
          description: Pet found
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: string
                  name:
                    type: string
              example:
                id: "pet-1"
                name: "Fluffy"
        '404':
          description: Not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
              example:
                code: 404
                message: "Pet not found"
        '500':
          description: Internal error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
              example:
                code: 500
                message: "Internal server error"
  /pets:
    post:
      operationId: createPet
      summary: Create a pet
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                  description: Pet name
                category:
                  type: string
                tags:
                  type: array
                  items:
                    type: string
                owner:
                  type: object
                  properties:
                    id:
                      type: integer
                    email:
                      type: string
      responses:
        '201':
          description: Created
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: string
                  name:
                    type: string
        '400':
          description: Bad request
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
              example:
                code: 400
                message: "Validation error"
components:
  schemas:
    Error:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
`

// ── ExtractAllResponses ───────────────────────────────────────────────────────

func TestExtractAllResponses_ReturnsAllStatusCodes(t *testing.T) {
p := NewParser()
defs, err := p.ExtractAllResponses(fixtureSpec, "GET", "/pets/{petId}")
if err != nil {
t.Fatalf("ExtractAllResponses error: %v", err)
}
if len(defs) != 3 {
t.Fatalf("expected 3 response defs (200, 404, 500), got %d", len(defs))
}

codes := make(map[int]SpecResponseDef)
for _, d := range defs {
codes[d.StatusCode] = d
}

if _, ok := codes[200]; !ok {
t.Error("missing 200 response")
}
if _, ok := codes[404]; !ok {
t.Error("missing 404 response")
}
if _, ok := codes[500]; !ok {
t.Error("missing 500 response")
}
}

func TestExtractAllResponses_PopulatesDescription(t *testing.T) {
p := NewParser()
defs, err := p.ExtractAllResponses(fixtureSpec, "GET", "/pets/{petId}")
if err != nil {
t.Fatalf("ExtractAllResponses error: %v", err)
}
for _, d := range defs {
if d.Description == "" {
t.Errorf("response %d has empty description", d.StatusCode)
}
}
}

func TestExtractAllResponses_PopulatesBodyExample(t *testing.T) {
p := NewParser()
defs, err := p.ExtractAllResponses(fixtureSpec, "GET", "/pets/{petId}")
if err != nil {
t.Fatalf("ExtractAllResponses error: %v", err)
}

found := false
for _, d := range defs {
if d.StatusCode == 200 && d.BodyExample != "" {
found = true
}
}
if !found {
t.Error("expected 200 response to have a body example")
}
}

func TestExtractAllResponses_PostOperation(t *testing.T) {
p := NewParser()
defs, err := p.ExtractAllResponses(fixtureSpec, "POST", "/pets")
if err != nil {
t.Fatalf("ExtractAllResponses error: %v", err)
}
if len(defs) != 2 {
t.Fatalf("expected 2 responses (201, 400) for POST /pets, got %d", len(defs))
}
}

func TestExtractAllResponses_UnknownPath(t *testing.T) {
p := NewParser()
_, err := p.ExtractAllResponses(fixtureSpec, "GET", "/nonexistent")
if err == nil {
t.Error("expected error for unknown path")
}
}

func TestExtractAllResponses_UnknownMethod(t *testing.T) {
	p := NewParser()
	// DELETE is not defined on /pets/{petId} in fixtureSpec — operationByMethod
	// returns nil, so ExtractAllResponses returns (nil, nil) — not an error.
	defs, err := p.ExtractAllResponses(fixtureSpec, "DELETE", "/pets/{petId}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if defs != nil {
		t.Errorf("expected nil defs for undefined method, got %v", defs)
	}
}

func TestExtractAllResponses_InvalidSpec(t *testing.T) {
p := NewParser()
_, err := p.ExtractAllResponses("not: valid: yaml: [", "GET", "/pets")
if err == nil {
t.Error("expected error for invalid spec")
}
}

// ── ExtractOperationInputs ────────────────────────────────────────────────────

func TestExtractOperationInputs_PathAndQueryParams(t *testing.T) {
p := NewParser()
inputs, err := p.ExtractOperationInputs(fixtureSpec, "GET", "/pets/{petId}")
if err != nil {
t.Fatalf("ExtractOperationInputs error: %v", err)
}
if inputs == nil {
t.Fatal("expected non-nil inputs")
}

if len(inputs.PathParams) != 1 {
t.Errorf("expected 1 path param, got %d", len(inputs.PathParams))
}
if inputs.PathParams[0].Name != "petId" {
t.Errorf("expected path param 'petId', got %q", inputs.PathParams[0].Name)
}
if !inputs.PathParams[0].Required {
t.Error("petId should be required")
}

if len(inputs.QueryParams) != 1 {
t.Errorf("expected 1 query param, got %d", len(inputs.QueryParams))
}
if inputs.QueryParams[0].Name != "format" {
t.Errorf("expected query param 'format', got %q", inputs.QueryParams[0].Name)
}
}

func TestExtractOperationInputs_BodyFields(t *testing.T) {
p := NewParser()
inputs, err := p.ExtractOperationInputs(fixtureSpec, "POST", "/pets")
if err != nil {
t.Fatalf("ExtractOperationInputs error: %v", err)
}
if inputs == nil {
t.Fatal("expected non-nil inputs")
}

// Should have flattened body fields: name, category, tags, tags.0, owner.id, owner.email
fieldPaths := make(map[string]bool)
for _, f := range inputs.BodyFields {
fieldPaths[f.GjsonPath] = true
}

for _, want := range []string{"name", "category"} {
if !fieldPaths[want] {
t.Errorf("expected body field %q to be present", want)
}
}
// Nested field
if !fieldPaths["owner.id"] && !fieldPaths["owner.email"] {
t.Error("expected nested owner fields to be flattened")
}
}

func TestExtractOperationInputs_NoBody(t *testing.T) {
p := NewParser()
inputs, err := p.ExtractOperationInputs(fixtureSpec, "GET", "/pets/{petId}")
if err != nil {
t.Fatalf("ExtractOperationInputs error: %v", err)
}
if len(inputs.BodyFields) != 0 {
t.Errorf("GET operation should have no body fields, got %d", len(inputs.BodyFields))
}
}

func TestExtractOperationInputs_UnknownPath(t *testing.T) {
p := NewParser()
_, err := p.ExtractOperationInputs(fixtureSpec, "GET", "/missing")
if err == nil {
t.Error("expected error for unknown path")
}
}

func TestExtractOperationInputs_InvalidSpec(t *testing.T) {
p := NewParser()
_, err := p.ExtractOperationInputs("!!bad yaml!!", "GET", "/pets")
if err == nil {
t.Error("expected error for invalid spec")
}
}

// ── schemaTypeHint ────────────────────────────────────────────────────────────

func TestSchemaTypeHint_Primitive(t *testing.T) {
cases := []struct {
typ  string
want string
}{
{"string", "string"},
{"integer", "integer"},
{"number", "number"},
{"boolean", "boolean"},
{"array", "array"},
}
for _, tc := range cases {
schema := &openapi3.Schema{Type: &openapi3.Types{tc.typ}}
got := schemaTypeHint(schema)
if !strings.Contains(got, tc.want) {
t.Errorf("schemaTypeHint(%q): expected %q in %q", tc.typ, tc.want, got)
}
}
}

func TestSchemaTypeHint_Object(t *testing.T) {
schema := &openapi3.Schema{
Type: &openapi3.Types{"object"},
Properties: openapi3.Schemas{
"id":   {Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
"name": {Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
},
}
hint := schemaTypeHint(schema)
if !strings.Contains(hint, "object") {
t.Errorf("expected 'object' in hint, got %q", hint)
}
}

func TestSchemaTypeHint_Nil(t *testing.T) {
	// schemaTypeHint(nil) returns "" by design — no schema, no hint.
	hint := schemaTypeHint(nil)
	if hint != "" {
		t.Errorf("schemaTypeHint(nil) should return empty string, got %q", hint)
	}
}
