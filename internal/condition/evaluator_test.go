package condition

import (
	"testing"
	"time"

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

// ── Date operator tests ──────────────────────────────────────────────────────

func TestEvaluate_DateOperators(t *testing.T) {
	e := NewEvaluator()

	// Use a fixed past date and a fixed future date relative to now.
	pastDate := "1999-06-15"
	futureDate := "2099-06-15"
	todayDate := time.Now().UTC().Format("2006-01-02")
	// A datetime that is clearly in the past
	pastDateTime := "1999-06-15T12:00:00Z"

	tests := []struct {
		name     string
		cond     models.Condition
		data     *RequestData
		expected bool
	}{
		// dateInPast
		{
			name: "dateInPast - past date",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateInPast},
			data: &RequestData{QueryParams: map[string][]string{"d": {pastDate}}},
			expected: true,
		},
		{
			name: "dateInPast - future date",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateInPast},
			data: &RequestData{QueryParams: map[string][]string{"d": {futureDate}}},
			expected: false,
		},

		// dateInFuture
		{
			name: "dateInFuture - future date",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateInFuture},
			data: &RequestData{QueryParams: map[string][]string{"d": {futureDate}}},
			expected: true,
		},
		{
			name: "dateInFuture - past date",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateInFuture},
			data: &RequestData{QueryParams: map[string][]string{"d": {pastDate}}},
			expected: false,
		},

		// dateToday
		{
			name: "dateToday - today",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateToday},
			data: &RequestData{QueryParams: map[string][]string{"d": {todayDate}}},
			expected: true,
		},
		{
			name: "dateToday - past",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateToday},
			data: &RequestData{QueryParams: map[string][]string{"d": {pastDate}}},
			expected: false,
		},

		// dateBefore
		{
			name: "dateBefore - past before now",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateBefore, Value: "now"},
			data: &RequestData{QueryParams: map[string][]string{"d": {pastDate}}},
			expected: true,
		},
		{
			name: "dateBefore - future not before now",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateBefore, Value: "now"},
			data: &RequestData{QueryParams: map[string][]string{"d": {futureDate}}},
			expected: false,
		},
		{
			name: "dateBefore - before literal date",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateBefore, Value: "2000-01-01"},
			data: &RequestData{QueryParams: map[string][]string{"d": {pastDate}}},
			expected: true,
		},

		// dateAfter
		{
			name: "dateAfter - future after now",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateAfter, Value: "now"},
			data: &RequestData{QueryParams: map[string][]string{"d": {futureDate}}},
			expected: true,
		},
		{
			name: "dateAfter - past not after now",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateAfter, Value: "now"},
			data: &RequestData{QueryParams: map[string][]string{"d": {pastDate}}},
			expected: false,
		},
		{
			name: "dateAfter - after yesterday token",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateAfter, Value: "yesterday"},
			data: &RequestData{QueryParams: map[string][]string{"d": {futureDate}}},
			expected: true,
		},

		// dateGte
		{
			name: "dateGte - today >= today",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateGte, Value: "today"},
			data: &RequestData{QueryParams: map[string][]string{"d": {todayDate}}},
			expected: true,
		},
		{
			name: "dateGte - past not >= today",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateGte, Value: "today"},
			data: &RequestData{QueryParams: map[string][]string{"d": {pastDate}}},
			expected: false,
		},

		// dateLte
		{
			name: "dateLte - today <= today",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateLte, Value: "today"},
			data: &RequestData{QueryParams: map[string][]string{"d": {todayDate}}},
			expected: true,
		},
		{
			name: "dateLte - future not <= today",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateLte, Value: "today"},
			data: &RequestData{QueryParams: map[string][]string{"d": {futureDate}}},
			expected: false,
		},

		// dateEq
		{
			name: "dateEq - exact match",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateEquals, Value: "1999-06-15"},
			data: &RequestData{QueryParams: map[string][]string{"d": {pastDate}}},
			expected: true,
		},
		{
			name: "dateEq - no match",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateEquals, Value: "1999-06-16"},
			data: &RequestData{QueryParams: map[string][]string{"d": {pastDate}}},
			expected: false,
		},

		// dateBetween
		{
			name: "dateBetween - in range",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateBetween, Value: "yesterday,tomorrow"},
			data: &RequestData{QueryParams: map[string][]string{"d": {todayDate}}},
			expected: true,
		},
		{
			name: "dateBetween - outside range",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateBetween, Value: "yesterday,tomorrow"},
			data: &RequestData{QueryParams: map[string][]string{"d": {pastDate}}},
			expected: false,
		},
		{
			name: "dateBetween - literal range",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateBetween, Value: "1999-01-01,1999-12-31"},
			data: &RequestData{QueryParams: map[string][]string{"d": {pastDate}}},
			expected: true,
		},

		// now±N tokens
		{
			name: "dateBefore - now+7d token",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateBefore, Value: "now+7d"},
			data: &RequestData{QueryParams: map[string][]string{"d": {todayDate}}},
			expected: true,
		},
		{
			name: "dateAfter - now-7d token",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateAfter, Value: "now-7d"},
			data: &RequestData{QueryParams: map[string][]string{"d": {futureDate}}},
			expected: true,
		},

		// Format hint
		{
			name: "dateBefore - US slash format",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateBefore, Value: "now", Format: "01/02/2006"},
			data: &RequestData{QueryParams: map[string][]string{"d": {"06/15/1999"}}},
			expected: true,
		},

		// Unix timestamp
		{
			name: "dateInPast - unix timestamp",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateInPast},
			data: &RequestData{QueryParams: map[string][]string{"d": {"929390400"}}}, // 1999-06-15
			expected: true,
		},

		// datetime RFC3339
		{
			name: "dateInPast - RFC3339 datetime",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateInPast},
			data: &RequestData{QueryParams: map[string][]string{"d": {pastDateTime}}},
			expected: true,
		},

		// body JSONPath
		{
			name: "dateBefore from body",
			cond: models.Condition{Source: models.SourceBody, Key: "date", Operator: models.OpDateBefore, Value: "now"},
			data: &RequestData{Body: `{"date":"1999-06-15"}`},
			expected: true,
		},

		// invalid date
		{
			name: "invalid date value returns false",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateInPast},
			data: &RequestData{QueryParams: map[string][]string{"d": {"not-a-date"}}},
			expected: false,
		},

		// dateBetween - malformed value
		{
			name: "dateBetween - missing comma returns false",
			cond: models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateBetween, Value: "yesterday"},
			data: &RequestData{QueryParams: map[string][]string{"d": {todayDate}}},
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

// ── Negate field tests ───────────────────────────────────────────────────────

func TestEvaluate_Negate(t *testing.T) {
e := NewEvaluator()

tests := []struct {
name     string
cond     models.Condition
data     *RequestData
expected bool
}{
{
name:     "eq match + negate = false",
cond:     models.Condition{Source: models.SourceQuery, Key: "s", Operator: models.OpEquals, Value: "x", Negate: true},
data:     &RequestData{QueryParams: map[string][]string{"s": {"x"}}},
expected: false,
},
{
name:     "eq no match + negate = true",
cond:     models.Condition{Source: models.SourceQuery, Key: "s", Operator: models.OpEquals, Value: "x", Negate: true},
data:     &RequestData{QueryParams: map[string][]string{"s": {"y"}}},
expected: true,
},
{
name:     "contains match + negate = false",
cond:     models.Condition{Source: models.SourceQuery, Key: "s", Operator: models.OpContains, Value: "foo", Negate: true},
data:     &RequestData{QueryParams: map[string][]string{"s": {"foobar"}}},
expected: false,
},
{
name:     "contains no match + negate = true",
cond:     models.Condition{Source: models.SourceQuery, Key: "s", Operator: models.OpContains, Value: "foo", Negate: true},
data:     &RequestData{QueryParams: map[string][]string{"s": {"hello"}}},
expected: true,
},
{
name:     "exists + negate on missing key = true",
cond:     models.Condition{Source: models.SourceQuery, Key: "missing", Operator: models.OpExists, Negate: true},
data:     &RequestData{QueryParams: map[string][]string{}},
expected: true,
},
{
name:     "exists + negate on present key = false",
cond:     models.Condition{Source: models.SourceQuery, Key: "s", Operator: models.OpExists, Negate: true},
data:     &RequestData{QueryParams: map[string][]string{"s": {"v"}}},
expected: false,
},
{
name:     "regex match + negate = false",
cond:     models.Condition{Source: models.SourceQuery, Key: "s", Operator: models.OpRegex, Value: `^\d+$`, Negate: true},
data:     &RequestData{QueryParams: map[string][]string{"s": {"123"}}},
expected: false,
},
{
name:     "regex no match + negate = true",
cond:     models.Condition{Source: models.SourceQuery, Key: "s", Operator: models.OpRegex, Value: `^\d+$`, Negate: true},
data:     &RequestData{QueryParams: map[string][]string{"s": {"abc"}}},
expected: true,
},
{
name:     "dateInPast match + negate = false",
cond:     models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateInPast, Negate: true},
data:     &RequestData{QueryParams: map[string][]string{"d": {"1999-01-01"}}},
expected: false,
},
{
name:     "dateInPast no match (future) + negate = true",
cond:     models.Condition{Source: models.SourceQuery, Key: "d", Operator: models.OpDateInPast, Negate: true},
data:     &RequestData{QueryParams: map[string][]string{"d": {"2099-01-01"}}},
expected: true,
},
{
name:     "deprecated ne still works",
cond:     models.Condition{Source: models.SourceQuery, Key: "s", Operator: "ne", Value: "x"},
data:     &RequestData{QueryParams: map[string][]string{"s": {"y"}}},
expected: true,
},
{
name:     "deprecated notExists still works",
cond:     models.Condition{Source: models.SourceQuery, Key: "missing", Operator: "notExists"},
data:     &RequestData{QueryParams: map[string][]string{}},
expected: true,
},
{
name:     "deprecated notContains still works",
cond:     models.Condition{Source: models.SourceQuery, Key: "s", Operator: "notContains", Value: "foo"},
data:     &RequestData{QueryParams: map[string][]string{"s": {"hello"}}},
expected: true,
},
{
name:     "negate false does not change result",
cond:     models.Condition{Source: models.SourceQuery, Key: "s", Operator: models.OpEquals, Value: "x", Negate: false},
data:     &RequestData{QueryParams: map[string][]string{"s": {"x"}}},
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

// ── Regex token tests ────────────────────────────────────────────────────────

func TestEvaluate_RegexTokens(t *testing.T) {
e := NewEvaluator()

tests := []struct {
name     string
cond     models.Condition
data     *RequestData
expected bool
}{
{
name:     "uuid token matches UUID",
cond:     models.Condition{Source: models.SourceQuery, Key: "id", Operator: models.OpRegex, Value: "uuid"},
data:     &RequestData{QueryParams: map[string][]string{"id": {"550e8400-e29b-41d4-a716-446655440000"}}},
expected: true,
},
{
name:     "uuid token does not match non-UUID",
cond:     models.Condition{Source: models.SourceQuery, Key: "id", Operator: models.OpRegex, Value: "uuid"},
data:     &RequestData{QueryParams: map[string][]string{"id": {"not-a-uuid"}}},
expected: false,
},
{
name:     "UUID token uppercase lookup",
cond:     models.Condition{Source: models.SourceQuery, Key: "id", Operator: models.OpRegex, Value: "UUID"},
data:     &RequestData{QueryParams: map[string][]string{"id": {"550e8400-e29b-41d4-a716-446655440000"}}},
expected: true,
},
{
name:     "email token matches email",
cond:     models.Condition{Source: models.SourceBody, Key: "email", Operator: models.OpRegex, Value: "email"},
data:     &RequestData{Body: `{"email":"user@example.com"}`},
expected: true,
},
{
name:     "email token rejects invalid email",
cond:     models.Condition{Source: models.SourceBody, Key: "email", Operator: models.OpRegex, Value: "email"},
data:     &RequestData{Body: `{"email":"not-an-email"}`},
expected: false,
},
{
name:     "ssn token matches SSN",
cond:     models.Condition{Source: models.SourceQuery, Key: "ssn", Operator: models.OpRegex, Value: "ssn"},
data:     &RequestData{QueryParams: map[string][]string{"ssn": {"123-45-6789"}}},
expected: true,
},
{
name:     "us-phone token matches US phone",
cond:     models.Condition{Source: models.SourceQuery, Key: "phone", Operator: models.OpRegex, Value: "us-phone"},
data:     &RequestData{QueryParams: map[string][]string{"phone": {"555-867-5309"}}},
expected: true,
},
{
name:     "semver token matches semantic version",
cond:     models.Condition{Source: models.SourceHeader, Key: "x-version", Operator: models.OpRegex, Value: "semver"},
data:     &RequestData{Headers: map[string][]string{"x-version": {"1.2.3-beta.1"}}},
expected: true,
},
{
name:     "uuid token + negate rejects UUID",
cond:     models.Condition{Source: models.SourceQuery, Key: "id", Operator: models.OpRegex, Value: "uuid", Negate: true},
data:     &RequestData{QueryParams: map[string][]string{"id": {"550e8400-e29b-41d4-a716-446655440000"}}},
expected: false,
},
{
name:     "uuid token + negate passes non-UUID",
cond:     models.Condition{Source: models.SourceQuery, Key: "id", Operator: models.OpRegex, Value: "uuid", Negate: true},
data:     &RequestData{QueryParams: map[string][]string{"id": {"hello"}}},
expected: true,
},
{
name:     "raw regex still works",
cond:     models.Condition{Source: models.SourceQuery, Key: "n", Operator: models.OpRegex, Value: `^\d{3}$`},
data:     &RequestData{QueryParams: map[string][]string{"n": {"123"}}},
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
