package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/prasenjit/go-virtual/internal/models"
)

// Parser handles OpenAPI 3 specification parsing
type Parser struct{}

// NewParser creates a new OpenAPI parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseResult contains the parsed spec and operations
type ParseResult struct {
	Spec       *models.Spec
	Operations []*models.Operation
}

// Parse parses an OpenAPI 3 specification
func (p *Parser) Parse(content string, basePath string) (*ParseResult, error) {
	// Load the OpenAPI document
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromData([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Validate the document
	if err := doc.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
	}

	// Extract spec info
	specID := uuid.New().String()
	now := time.Now()

	spec := &models.Spec{
		ID:                 specID,
		Name:               doc.Info.Title,
		Version:            doc.Info.Version,
		Description:        doc.Info.Description,
		Content:            content,
		BasePath:           normalizeBasePath(basePath),
		Enabled:            true,
		Tracing:            false,
		UseExampleFallback: true, // Enable example fallback by default
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	// Extract operations
	operations := p.extractOperations(doc, specID, spec.BasePath)

	return &ParseResult{
		Spec:       spec,
		Operations: operations,
	}, nil
}

// ParseOperations parses operations from spec content for an existing spec
// This is used when regenerating operations from stored specs
func (p *Parser) ParseOperations(content string, specID string, basePath string) ([]*models.Operation, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromData([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	return p.extractOperations(doc, specID, normalizeBasePath(basePath)), nil
}

// extractOperations extracts all operations from the OpenAPI document
func (p *Parser) extractOperations(doc *openapi3.T, specID, basePath string) []*models.Operation {
	var operations []*models.Operation

	for pathPattern, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}

		// Process each HTTP method
		methods := map[string]*openapi3.Operation{
			"GET":     pathItem.Get,
			"POST":    pathItem.Post,
			"PUT":     pathItem.Put,
			"DELETE":  pathItem.Delete,
			"PATCH":   pathItem.Patch,
			"HEAD":    pathItem.Head,
			"OPTIONS": pathItem.Options,
		}

		for method, op := range methods {
			if op == nil {
				continue
			}

			// Generate deterministic operation ID based on spec, method, and path
			// This allows operations to be regenerated from spec while maintaining stable IDs
			opID := generateOperationID(specID, method, pathPattern)
			operationID := op.OperationID
			if operationID == "" {
				// Generate operation ID if not provided
				operationID = fmt.Sprintf("%s_%s", strings.ToLower(method), sanitizePath(pathPattern))
			}

			operation := &models.Operation{
				ID:          opID,
				SpecID:      specID,
				Method:      method,
				Path:        pathPattern,
				FullPath:    path.Join(basePath, pathPattern),
				OperationID: operationID,
				Summary:     op.Summary,
				Description: op.Description,
				Tags:        op.Tags,
			}

			// Extract example response from spec (try 200, 201, then default)
			operation.ExampleResponse = extractExampleResponseFromOp(op)

			operations = append(operations, operation)
		}
	}

	return operations
}

// extractExampleResponseFromOp extracts an example success response from an OpenAPI operation
func extractExampleResponseFromOp(op *openapi3.Operation) *models.ExampleResponse {
	if op.Responses == nil {
		return nil
	}

	// Try success status codes in order of preference
	successCodes := []int{200, 201, 202, 204}
	
	for _, statusCode := range successCodes {
		response := op.Responses.Status(statusCode)
		if response == nil || response.Value == nil {
			continue
		}

		example := &models.ExampleResponse{
			StatusCode: statusCode,
			Headers:    make(map[string]string),
		}

		// Extract headers
		for name, header := range response.Value.Headers {
			if header.Value != nil && header.Value.Example != nil {
				example.Headers[name] = fmt.Sprintf("%v", header.Value.Example)
			}
		}

		// Extract body example from JSON content
		for mediaType, content := range response.Value.Content {
			if strings.Contains(mediaType, "json") {
				example.Headers["Content-Type"] = mediaType
				
				if content.Example != nil {
					// Direct example
					example.Body = formatExample(content.Example)
				} else if len(content.Examples) > 0 {
					// Named examples - use first one
					for _, ex := range content.Examples {
						if ex.Value != nil && ex.Value.Value != nil {
							example.Body = formatExample(ex.Value.Value)
							break
						}
					}
				} else if content.Schema != nil && content.Schema.Value != nil {
					// Generate from schema
					example.Body = generateExampleFromSchema(content.Schema.Value)
				}
				
				if example.Body != "" {
					return example
				}
				break
			}
		}

		// Even without body, return if we have a valid status (e.g., 204 No Content)
		if statusCode == 204 {
			return example
		}
	}

	return nil
}

// formatExample converts an example value to a JSON string
func formatExample(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		// Try to marshal as JSON
		if data, err := json.Marshal(val); err == nil {
			return string(data)
		}
		return fmt.Sprintf("%v", val)
	}
}

// normalizeBasePath ensures the base path is properly formatted
func normalizeBasePath(basePath string) string {
	if basePath == "" {
		return ""
	}

	// Ensure it starts with /
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}

	// Remove trailing /
	basePath = strings.TrimSuffix(basePath, "/")

	return basePath
}

// sanitizePath converts a path to a valid identifier
func sanitizePath(pathPattern string) string {
	// Replace path parameters
	result := strings.ReplaceAll(pathPattern, "{", "")
	result = strings.ReplaceAll(result, "}", "")
	result = strings.ReplaceAll(result, "/", "_")
	result = strings.TrimPrefix(result, "_")
	result = strings.TrimSuffix(result, "_")
	return result
}

// ExtractExampleResponse extracts an example response from the OpenAPI spec for an operation
func (p *Parser) ExtractExampleResponse(content string, method, pathPattern string, statusCode int) (map[string]string, string, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(content))
	if err != nil {
		return nil, "", err
	}

	pathItem := doc.Paths.Find(pathPattern)
	if pathItem == nil {
		return nil, "", fmt.Errorf("path not found: %s", pathPattern)
	}

	var op *openapi3.Operation
	switch strings.ToUpper(method) {
	case "GET":
		op = pathItem.Get
	case "POST":
		op = pathItem.Post
	case "PUT":
		op = pathItem.Put
	case "DELETE":
		op = pathItem.Delete
	case "PATCH":
		op = pathItem.Patch
	default:
		return nil, "", fmt.Errorf("unsupported method: %s", method)
	}

	if op == nil {
		return nil, "", fmt.Errorf("operation not found for %s %s", method, pathPattern)
	}

	// Find response for status code
	statusStr := fmt.Sprintf("%d", statusCode)
	response := op.Responses.Status(statusCode)
	if response == nil {
		// Try default response
		response = op.Responses.Default()
		if response == nil {
			return nil, "", fmt.Errorf("no response defined for status %s", statusStr)
		}
	}

	headers := make(map[string]string)
	var body string

	// Extract headers
	if response.Value != nil {
		for name, header := range response.Value.Headers {
			if header.Value != nil && header.Value.Schema != nil {
				// Use example if available
				if header.Value.Example != nil {
					headers[name] = fmt.Sprintf("%v", header.Value.Example)
				}
			}
		}

		// Extract body example
		for mediaType, content := range response.Value.Content {
			if strings.Contains(mediaType, "json") {
				if content.Example != nil {
					body = fmt.Sprintf("%v", content.Example)
				} else if content.Schema != nil && content.Schema.Value != nil {
					// Try to generate example from schema
					body = generateExampleFromSchema(content.Schema.Value)
				}
				headers["Content-Type"] = mediaType
				break
			}
		}
	}

	return headers, body, nil
}

// generateOperationID generates a deterministic operation ID based on spec, method, and path
// This allows operations to be regenerated from spec while maintaining stable IDs
// that response configs can reference
func generateOperationID(specID, method, path string) string {
	// Create a deterministic hash from spec ID + method + path
	data := fmt.Sprintf("%s:%s:%s", specID, method, path)
	hash := sha256.Sum256([]byte(data))
	// Use first 16 bytes to create a UUID-like string
	return hex.EncodeToString(hash[:16])
}

// generateExampleFromSchema generates a basic example from an OpenAPI schema
func generateExampleFromSchema(schema *openapi3.Schema) string {
	if schema.Example != nil {
		return fmt.Sprintf("%v", schema.Example)
	}

	switch schema.Type.Slice()[0] {
	case "object":
		return "{}"
	case "array":
		return "[]"
	case "string":
		return `"string"`
	case "integer":
		return "0"
	case "number":
		return "0.0"
	case "boolean":
		return "false"
	default:
		return "null"
	}
}
