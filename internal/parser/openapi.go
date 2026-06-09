package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
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

	// Extract default backend URI from the first non-templated server entry
	defaultBackendURI := ""
	for _, srv := range doc.Servers {
		if srv != nil && srv.URL != "" && !strings.Contains(srv.URL, "{") {
			defaultBackendURI = strings.TrimRight(srv.URL, "/")
			break
		}
	}

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
		Mode:               models.SpecModeStandard,
		BackendURI:         defaultBackendURI,
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

			declaredInputs := extractOperationInputs(pathItem, op)
			operation := &models.Operation{
				ID:                   opID,
				SpecID:               specID,
				Method:               method,
				Path:                 pathPattern,
				FullPath:             path.Join(basePath, pathPattern),
				OperationID:          operationID,
				Summary:              op.Summary,
				Description:          op.Description,
				Tags:                 op.Tags,
				DeclaredPathParams:   paramNames(declaredInputs.PathParams),
				DeclaredQueryParams:  paramNames(declaredInputs.QueryParams),
				DeclaredHeaderParams: paramNames(declaredInputs.HeaderParams),
				DeclaredBodyFields:   bodyFieldPaths(declaredInputs.BodyFields),
				HasRequestBody:       declaredInputs.HasBody,
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

// SpecResponseDef holds spec-defined response information for a single status code.
// When a response has multiple named examples, one SpecResponseDef is emitted per example.
type SpecResponseDef struct {
	StatusCode     int    `json:"statusCode"`
	Description    string `json:"description"`
	ContentType    string `json:"contentType,omitempty"`    // e.g. "application/json"
	BodyExample    string `json:"bodyExample,omitempty"`    // JSON string or schema-derived example
	SchemaHint     string `json:"schemaHint,omitempty"`     // Human-readable schema summary
	ExampleName    string `json:"exampleName,omitempty"`    // Named example key from the spec
	ExampleSummary string `json:"exampleSummary,omitempty"` // Human-readable summary of the named example
}

// ParamDef describes a single path or query parameter.
type ParamDef struct {
	Name        string
	In          string // "path", "query", or "header"
	Required    bool
	Type        string // "string", "integer", "boolean", etc.
	Description string
}

// BodyFieldDef describes one (potentially nested) field in the request body JSON.
type BodyFieldDef struct {
	GjsonPath   string // dot-notation gjson path, e.g. "user.id", "items.0.name"
	Type        string // "string", "integer", "array", "object", etc.
	Description string
}

// OperationInputs aggregates all input metadata for an operation.
type OperationInputs struct {
	PathParams   []ParamDef
	QueryParams  []ParamDef
	HeaderParams []ParamDef
	BodyFields   []BodyFieldDef // nil when no request body or no JSON fields can be flattened
	HasBody      bool
}

// ExtractOperationInputs extracts path params, query params, and request body
// field definitions for the given operation.
func (p *Parser) ExtractOperationInputs(content string, method, pathPattern string) (*OperationInputs, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse spec: %w", err)
	}

	pathItem := doc.Paths.Find(pathPattern)
	if pathItem == nil {
		return nil, fmt.Errorf("path not found: %s", pathPattern)
	}

	op := operationByMethod(pathItem, method)
	if op == nil {
		return nil, nil
	}

	return extractOperationInputs(pathItem, op), nil
}

func extractOperationInputs(pathItem *openapi3.PathItem, op *openapi3.Operation) *OperationInputs {
	inputs := &OperationInputs{}

	// Path, query, and header parameters (merge path-level + operation-level)
	allParams := make(openapi3.Parameters, 0)
	allParams = append(allParams, pathItem.Parameters...)
	allParams = append(allParams, op.Parameters...)

	for _, pRef := range allParams {
		if pRef == nil || pRef.Value == nil {
			continue
		}
		pv := pRef.Value
		pd := ParamDef{
			Name:        pv.Name,
			In:          pv.In,
			Required:    pv.Required,
			Description: pv.Description,
		}
		if pv.Schema != nil && pv.Schema.Value != nil {
			ts := pv.Schema.Value.Type.Slice()
			if len(ts) > 0 {
				pd.Type = ts[0]
			}
		}
		switch pv.In {
		case "path":
			inputs.PathParams = append(inputs.PathParams, pd)
		case "query":
			inputs.QueryParams = append(inputs.QueryParams, pd)
		case "header":
			inputs.HeaderParams = append(inputs.HeaderParams, pd)
		}
	}

	// Request body
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		inputs.HasBody = true
		for mediaType, mt := range op.RequestBody.Value.Content {
			if !strings.Contains(mediaType, "json") || mt == nil || mt.Schema == nil || mt.Schema.Value == nil {
				continue
			}
			inputs.BodyFields = flattenSchema(mt.Schema.Value, "", 0)
			break
		}
	}

	return inputs
}

// flattenSchema recursively walks a JSON schema and returns gjson-path field defs.
// maxDepth prevents runaway recursion on self-referential or deeply nested schemas.
func flattenSchema(s *openapi3.Schema, prefix string, depth int) []BodyFieldDef {
	if s == nil || depth > 4 {
		return nil
	}

	var fields []BodyFieldDef
	types := s.Type.Slice()
	typeName := ""
	if len(types) > 0 {
		typeName = types[0]
	}

	switch typeName {
	case "object", "":
		for name, propRef := range s.Properties {
			if propRef == nil || propRef.Value == nil {
				continue
			}
			pv := propRef.Value
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			pts := pv.Type.Slice()
			pt := ""
			if len(pts) > 0 {
				pt = pts[0]
			}
			fields = append(fields, BodyFieldDef{
				GjsonPath:   path,
				Type:        pt,
				Description: pv.Description,
			})
			// Recurse into nested objects
			if pt == "object" || (pt == "" && len(pv.Properties) > 0) {
				fields = append(fields, flattenSchema(pv, path, depth+1)...)
			}
		}
		// allOf / oneOf / anyOf — include from first sub-schema for hints
		for _, sub := range append(append(s.AllOf, s.OneOf...), s.AnyOf...) {
			if sub != nil && sub.Value != nil {
				fields = append(fields, flattenSchema(sub.Value, prefix, depth+1)...)
			}
		}
	case "array":
		if s.Items != nil && s.Items.Value != nil {
			// Represent array items with .0 gjson index hint
			itemPrefix := "0"
			if prefix != "" {
				itemPrefix = prefix + ".0"
			}
			its := s.Items.Value.Type.Slice()
			it := ""
			if len(its) > 0 {
				it = its[0]
			}
			fields = append(fields, BodyFieldDef{GjsonPath: itemPrefix, Type: it})
			fields = append(fields, flattenSchema(s.Items.Value, itemPrefix, depth+1)...)
		}
	}

	return fields
}

func paramNames(params []ParamDef) []string {
	names := make([]string, 0, len(params))
	for _, param := range params {
		if param.Name != "" {
			names = append(names, param.Name)
		}
	}
	sort.Strings(names)
	return names
}

func bodyFieldPaths(fields []BodyFieldDef) []string {
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.GjsonPath != "" {
			paths = append(paths, field.GjsonPath)
		}
	}
	sort.Strings(paths)
	return paths
}

// ExtractAllResponses extracts every response definition from the spec for the given operation.
// It returns one SpecResponseDef per status code (plus "default" mapped to 0).
func (p *Parser) ExtractAllResponses(content string, method, pathPattern string) ([]SpecResponseDef, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse spec: %w", err)
	}

	pathItem := doc.Paths.Find(pathPattern)
	if pathItem == nil {
		return nil, fmt.Errorf("path not found: %s", pathPattern)
	}

	op := operationByMethod(pathItem, method)
	if op == nil || op.Responses == nil {
		return nil, nil
	}

	var defs []SpecResponseDef
	for statusStr, respRef := range op.Responses.Map() {
		if respRef == nil || respRef.Value == nil {
			continue
		}
		rv := respRef.Value
		def := SpecResponseDef{}

		// Status code
		if statusStr == "default" {
			def.StatusCode = 0
		} else {
			fmt.Sscanf(statusStr, "%d", &def.StatusCode)
		}

		if rv.Description != nil {
			def.Description = *rv.Description
		}

		// Extract body examples from the first JSON media type.
		// When multiple named examples exist, emit one SpecResponseDef per example.
		appended := false
		for mediaType, mt := range rv.Content {
			if !strings.Contains(mediaType, "json") {
				continue
			}
			if mt == nil {
				continue
			}
			def.ContentType = mediaType

			if mt.Example != nil {
				// Single inline example
				def.BodyExample = formatExample(mt.Example)
			} else if len(mt.Examples) > 1 {
				// Multiple named examples — emit one entry per example
				// Sort keys for deterministic ordering
				names := make([]string, 0, len(mt.Examples))
				for name := range mt.Examples {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					ex := mt.Examples[name]
					if ex == nil || ex.Value == nil || ex.Value.Value == nil {
						continue
					}
					namedDef := def
					namedDef.BodyExample = formatExample(ex.Value.Value)
					namedDef.ExampleName = name
					namedDef.ExampleSummary = ex.Value.Summary
					defs = append(defs, namedDef)
					appended = true
				}
				break
			} else if len(mt.Examples) == 1 {
				// Single named example — use it but still surface the name
				for name, ex := range mt.Examples {
					if ex.Value != nil && ex.Value.Value != nil {
						def.BodyExample = formatExample(ex.Value.Value)
						def.ExampleName = name
						def.ExampleSummary = ex.Value.Summary
					}
				}
			}

			if def.BodyExample == "" && mt.Schema != nil && mt.Schema.Value != nil {
				def.BodyExample = generateExampleFromSchema(mt.Schema.Value)
				def.SchemaHint = schemaTypeHint(mt.Schema.Value)
			}
			break
		}

		if !appended {
			defs = append(defs, def)
		}
	}

	return defs, nil
}

// operationByMethod returns the openapi3.Operation for the given HTTP method.
func operationByMethod(item *openapi3.PathItem, method string) *openapi3.Operation {
	switch strings.ToUpper(method) {
	case "GET":
		return item.Get
	case "POST":
		return item.Post
	case "PUT":
		return item.Put
	case "DELETE":
		return item.Delete
	case "PATCH":
		return item.Patch
	case "HEAD":
		return item.Head
	case "OPTIONS":
		return item.Options
	}
	return nil
}

// schemaTypeHint returns a brief human-readable description of a schema's type/structure.
func schemaTypeHint(s *openapi3.Schema) string {
	if s == nil {
		return ""
	}
	types := s.Type.Slice()
	if len(types) == 0 {
		if len(s.Properties) > 0 {
			keys := make([]string, 0, len(s.Properties))
			for k := range s.Properties {
				keys = append(keys, k)
			}
			return "object with fields: " + strings.Join(keys, ", ")
		}
		return ""
	}
	switch types[0] {
	case "object":
		if len(s.Properties) > 0 {
			keys := make([]string, 0, len(s.Properties))
			for k := range s.Properties {
				keys = append(keys, k)
			}
			return "object with fields: " + strings.Join(keys, ", ")
		}
		return "object"
	case "array":
		if s.Items != nil && s.Items.Value != nil {
			return "array of " + schemaTypeHint(s.Items.Value)
		}
		return "array"
	default:
		return types[0]
	}
}

// generateOperationID generates a deterministic operation ID based on spec, method, and path
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

	types := schema.Type.Slice()
	if len(types) == 0 {
		// Schema has no explicit type (e.g. oneOf/allOf/anyOf); skip example generation
		return ""
	}
	switch types[0] {
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
