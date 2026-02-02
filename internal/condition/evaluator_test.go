package condition

import (
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
)

func TestNewEvaluator(t *testing.T) {
	e := NewEvaluator()
	if e == nil {
		t.Fatal("NewEvaluator returned nil")
	}
}

func TestEvaluate_PathParameter(t *testing.T) {
	e := NewEvaluator()

	tests := []struct {
		name     string
		cond     models.Condition
		data     *RequestData
		expected bool
	}{
		{
			name: "path param equals",
			cond: models.Condition{
				Source:   models.SourcePath,
				Key:      "id",
				Operator: models.OpEquals,
				Value:    "123",
			},
			data: &RequestData{
				PathParams: map[string]string{"id": "123"},
			},
			expected: true,
		},
		{
			name: "path param not equals",
			cond: models.Condition{
				Source:   models.SourcePath,
				Key:      "id",
				Operator: models.OpEquals,
				Value:    "123",
			},
			data: &RequestData{
				PathParams: map[string]string{"id": "456"},
			},
			expected: false,
		},
		{
			name: "path param missing",
			cond: models.Condition{
				Source:   models.SourcePath,
				Key:      "id",
				Operator: models.OpEquals,
				Value:    "123",
			},
			data: &RequestData{
				PathParams: map[string]string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(tt.cond, tt.data)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluate_QueryParameter(t *testing.T) {
	e := NewEvaluator()

	tests := []struct {
		name     string
		cond     models.Condition
		data     *RequestData
		expected bool
	}{
		{
			name: "query param equals",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "status",
				Operator: models.OpEquals,
				Value:    "active",
			},
			data: &RequestData{
				QueryParams: map[string][]string{"status": {"active"}},
			},
			expected: true,
		},
		{
			name: "query param contains",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "name",
				Operator: models.OpContains,
				Value:    "john",
			},
			data: &RequestData{
				QueryParams: map[string][]string{"name": {"john doe"}},
			},
			expected: true,
		},
		{
			name: "query param multiple values uses first",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "tag",
				Operator: models.OpEquals,
				Value:    "first",
			},
			data: &RequestData{
				QueryParams: map[string][]string{"tag": {"first", "second"}},
			},
			expected: true,
		},
		{
			name: "query param missing",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "missing",
				Operator: models.OpExists,
				Value:    "",
			},
			data: &RequestData{
				QueryParams: map[string][]string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(tt.cond, tt.data)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluate_Header(t *testing.T) {
	e := NewEvaluator()

	tests := []struct {
		name     string
		cond     models.Condition
		data     *RequestData
		expected bool
	}{
		{
			name: "header equals case-insensitive",
			cond: models.Condition{
				Source:   models.SourceHeader,
				Key:      "Content-Type",
				Operator: models.OpEquals,
				Value:    "application/json",
			},
			data: &RequestData{
				Headers: map[string][]string{"content-type": {"application/json"}},
			},
			expected: true,
		},
		{
			name: "header contains",
			cond: models.Condition{
				Source:   models.SourceHeader,
				Key:      "Authorization",
				Operator: models.OpStartsWith,
				Value:    "Bearer ",
			},
			data: &RequestData{
				Headers: map[string][]string{"Authorization": {"Bearer token123"}},
			},
			expected: true,
		},
		{
			name: "header exists",
			cond: models.Condition{
				Source:   models.SourceHeader,
				Key:      "X-Custom",
				Operator: models.OpExists,
				Value:    "",
			},
			data: &RequestData{
				Headers: map[string][]string{"X-Custom": {"value"}},
			},
			expected: true,
		},
		{
			name: "header not exists",
			cond: models.Condition{
				Source:   models.SourceHeader,
				Key:      "X-Missing",
				Operator: models.OpNotExists,
				Value:    "",
			},
			data: &RequestData{
				Headers: map[string][]string{},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(tt.cond, tt.data)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluate_Body(t *testing.T) {
	e := NewEvaluator()

	jsonBody := `{"user": {"name": "John", "age": 30, "email": "john@example.com"}, "items": [1, 2, 3]}`

	tests := []struct {
		name     string
		cond     models.Condition
		data     *RequestData
		expected bool
	}{
		{
			name: "body jsonpath equals",
			cond: models.Condition{
				Source:   models.SourceBody,
				Key:      "user.name",
				Operator: models.OpEquals,
				Value:    "John",
			},
			data: &RequestData{
				Body: jsonBody,
			},
			expected: true,
		},
		{
			name: "body jsonpath nested",
			cond: models.Condition{
				Source:   models.SourceBody,
				Key:      "user.age",
				Operator: models.OpEquals,
				Value:    "30",
			},
			data: &RequestData{
				Body: jsonBody,
			},
			expected: true,
		},
		{
			name: "body jsonpath array",
			cond: models.Condition{
				Source:   models.SourceBody,
				Key:      "items.#",
				Operator: models.OpEquals,
				Value:    "3",
			},
			data: &RequestData{
				Body: jsonBody,
			},
			expected: true,
		},
		{
			name: "body jsonpath email contains",
			cond: models.Condition{
				Source:   models.SourceBody,
				Key:      "user.email",
				Operator: models.OpContains,
				Value:    "@example.com",
			},
			data: &RequestData{
				Body: jsonBody,
			},
			expected: true,
		},
		{
			name: "body jsonpath missing key",
			cond: models.Condition{
				Source:   models.SourceBody,
				Key:      "nonexistent.path",
				Operator: models.OpExists,
				Value:    "",
			},
			data: &RequestData{
				Body: jsonBody,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(tt.cond, tt.data)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluate_Operators(t *testing.T) {
	e := NewEvaluator()

	tests := []struct {
		name     string
		cond     models.Condition
		data     *RequestData
		expected bool
	}{
		{
			name: "not equals",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "status",
				Operator: models.OpNotEquals,
				Value:    "inactive",
			},
			data: &RequestData{
				QueryParams: map[string][]string{"status": {"active"}},
			},
			expected: true,
		},
		{
			name: "not contains",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "name",
				Operator: models.OpNotContains,
				Value:    "admin",
			},
			data: &RequestData{
				QueryParams: map[string][]string{"name": {"user123"}},
			},
			expected: true,
		},
		{
			name: "starts with",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "path",
				Operator: models.OpStartsWith,
				Value:    "/api/",
			},
			data: &RequestData{
				QueryParams: map[string][]string{"path": {"/api/users"}},
			},
			expected: true,
		},
		{
			name: "ends with",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "file",
				Operator: models.OpEndsWith,
				Value:    ".json",
			},
			data: &RequestData{
				QueryParams: map[string][]string{"file": {"config.json"}},
			},
			expected: true,
		},
		{
			name: "regex match",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "email",
				Operator: models.OpRegex,
				Value:    `^[a-z]+@example\.com$`,
			},
			data: &RequestData{
				QueryParams: map[string][]string{"email": {"john@example.com"}},
			},
			expected: true,
		},
		{
			name: "regex no match",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "email",
				Operator: models.OpRegex,
				Value:    `^[0-9]+$`,
			},
			data: &RequestData{
				QueryParams: map[string][]string{"email": {"john@example.com"}},
			},
			expected: false,
		},
		{
			name: "invalid regex",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "value",
				Operator: models.OpRegex,
				Value:    `[invalid`,
			},
			data: &RequestData{
				QueryParams: map[string][]string{"value": {"test"}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(tt.cond, tt.data)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluate_NumericComparison(t *testing.T) {
	e := NewEvaluator()

	tests := []struct {
		name     string
		cond     models.Condition
		data     *RequestData
		expected bool
	}{
		{
			name: "greater than",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "age",
				Operator: models.OpGreaterThan,
				Value:    "18",
			},
			data: &RequestData{
				QueryParams: map[string][]string{"age": {"25"}},
			},
			expected: true,
		},
		{
			name: "greater than equal",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "age",
				Operator: models.OpGTE,
				Value:    "18",
			},
			data: &RequestData{
				QueryParams: map[string][]string{"age": {"18"}},
			},
			expected: true,
		},
		{
			name: "less than",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "price",
				Operator: models.OpLessThan,
				Value:    "100",
			},
			data: &RequestData{
				QueryParams: map[string][]string{"price": {"50.5"}},
			},
			expected: true,
		},
		{
			name: "less than equal",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "count",
				Operator: models.OpLTE,
				Value:    "10",
			},
			data: &RequestData{
				QueryParams: map[string][]string{"count": {"10"}},
			},
			expected: true,
		},
		{
			name: "numeric comparison with non-numeric falls back to string",
			cond: models.Condition{
				Source:   models.SourceQuery,
				Key:      "value",
				Operator: models.OpGreaterThan,
				Value:    "abc",
			},
			data: &RequestData{
				QueryParams: map[string][]string{"value": {"xyz"}},
			},
			expected: true, // string comparison: "xyz" > "abc"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(tt.cond, tt.data)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluateAll(t *testing.T) {
	e := NewEvaluator()

	tests := []struct {
		name       string
		conditions []models.Condition
		data       *RequestData
		expected   bool
	}{
		{
			name:       "empty conditions",
			conditions: []models.Condition{},
			data:       &RequestData{},
			expected:   true,
		},
		{
			name: "all conditions match",
			conditions: []models.Condition{
				{
					Source:   models.SourceQuery,
					Key:      "status",
					Operator: models.OpEquals,
					Value:    "active",
				},
				{
					Source:   models.SourceHeader,
					Key:      "X-Version",
					Operator: models.OpEquals,
					Value:    "2.0",
				},
			},
			data: &RequestData{
				QueryParams: map[string][]string{"status": {"active"}},
				Headers:     map[string][]string{"X-Version": {"2.0"}},
			},
			expected: true,
		},
		{
			name: "one condition fails",
			conditions: []models.Condition{
				{
					Source:   models.SourceQuery,
					Key:      "status",
					Operator: models.OpEquals,
					Value:    "active",
				},
				{
					Source:   models.SourceQuery,
					Key:      "role",
					Operator: models.OpEquals,
					Value:    "admin",
				},
			},
			data: &RequestData{
				QueryParams: map[string][]string{
					"status": {"active"},
					"role":   {"user"},
				},
			},
			expected: false,
		},
		{
			name: "all conditions fail",
			conditions: []models.Condition{
				{
					Source:   models.SourceQuery,
					Key:      "status",
					Operator: models.OpEquals,
					Value:    "active",
				},
				{
					Source:   models.SourceQuery,
					Key:      "role",
					Operator: models.OpEquals,
					Value:    "admin",
				},
			},
			data: &RequestData{
				QueryParams: map[string][]string{
					"status": {"inactive"},
					"role":   {"user"},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.EvaluateAll(tt.conditions, tt.data)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHasValue(t *testing.T) {
	e := NewEvaluator()

	data := &RequestData{
		PathParams:  map[string]string{"id": "123"},
		QueryParams: map[string][]string{"status": {"active"}},
		Headers:     map[string][]string{"Authorization": {"Bearer token"}},
		Body:        `{"name": "John"}`,
	}

	tests := []struct {
		name     string
		source   string
		key      string
		expected bool
	}{
		{"path exists", models.SourcePath, "id", true},
		{"path not exists", models.SourcePath, "missing", false},
		{"query exists", models.SourceQuery, "status", true},
		{"query not exists", models.SourceQuery, "missing", false},
		{"header exists", models.SourceHeader, "Authorization", true},
		{"header not exists", models.SourceHeader, "Missing", false},
		{"body exists", models.SourceBody, "name", true},
		{"body not exists", models.SourceBody, "missing", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.HasValue(tt.source, tt.key, data)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetValue(t *testing.T) {
	e := NewEvaluator()

	data := &RequestData{
		PathParams:  map[string]string{"id": "123"},
		QueryParams: map[string][]string{"status": {"active"}},
		Headers:     map[string][]string{"Authorization": {"Bearer token"}},
		Body:        `{"name": "John", "age": 30}`,
	}

	tests := []struct {
		name     string
		source   string
		key      string
		expected string
	}{
		{"path value", models.SourcePath, "id", "123"},
		{"query value", models.SourceQuery, "status", "active"},
		{"header value", models.SourceHeader, "Authorization", "Bearer token"},
		{"body value", models.SourceBody, "name", "John"},
		{"body nested", models.SourceBody, "age", "30"},
		{"unknown source", "unknown", "key", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.GetValue(tt.source, tt.key, data)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCompareNumeric(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected int
	}{
		{"equal numbers", "10", "10", 0},
		{"a greater", "20", "10", 1},
		{"a less", "5", "10", -1},
		{"float comparison", "10.5", "10.1", 1},
		{"string fallback equal", "abc", "abc", 0},
		{"string fallback greater", "xyz", "abc", 1},
		{"string fallback less", "abc", "xyz", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareNumeric(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}
