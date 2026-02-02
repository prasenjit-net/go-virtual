package template

import (
	"regexp"
	"strings"
	"testing"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.rng == nil {
		t.Fatal("Engine rng is nil")
	}
}

func TestProcess_PathParams(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name     string
		template string
		ctx      *Context
		expected string
	}{
		{
			name:     "single path param",
			template: "User ID: {{path.id}}",
			ctx: &Context{
				PathParams: map[string]string{"id": "123"},
			},
			expected: "User ID: 123",
		},
		{
			name:     "multiple path params",
			template: "User {{path.userId}} in org {{path.orgId}}",
			ctx: &Context{
				PathParams: map[string]string{"userId": "456", "orgId": "789"},
			},
			expected: "User 456 in org 789",
		},
		{
			name:     "path param with spaces in template",
			template: "ID: {{ path.id }}",
			ctx: &Context{
				PathParams: map[string]string{"id": "123"},
			},
			expected: "ID: 123",
		},
		{
			name:     "missing path param",
			template: "ID: {{path.missing}}",
			ctx: &Context{
				PathParams: map[string]string{"id": "123"},
			},
			expected: "ID: ",
		},
		{
			name:     "nil path params",
			template: "ID: {{path.id}}",
			ctx: &Context{
				PathParams: nil,
			},
			expected: "ID: ",
		},
		{
			name:     "dotted path param format",
			template: "User ID: {{.path.id}}",
			ctx: &Context{
				PathParams: map[string]string{"id": "123"},
			},
			expected: "User ID: 123",
		},
		{
			name:     "dotted path param with spaces",
			template: "ID: {{ .path.id }}",
			ctx: &Context{
				PathParams: map[string]string{"id": "456"},
			},
			expected: "ID: 456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Process(tt.template, tt.ctx)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestProcess_QueryParams(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name     string
		template string
		ctx      *Context
		expected string
	}{
		{
			name:     "single query param",
			template: "Status: {{query.status}}",
			ctx: &Context{
				QueryParams: map[string][]string{"status": {"active"}},
			},
			expected: "Status: active",
		},
		{
			name:     "query param uses first value",
			template: "Tag: {{query.tag}}",
			ctx: &Context{
				QueryParams: map[string][]string{"tag": {"first", "second", "third"}},
			},
			expected: "Tag: first",
		},
		{
			name:     "missing query param",
			template: "Status: {{query.missing}}",
			ctx: &Context{
				QueryParams: map[string][]string{"status": {"active"}},
			},
			expected: "Status: ",
		},
		{
			name:     "empty query params",
			template: "Status: {{query.status}}",
			ctx: &Context{
				QueryParams: map[string][]string{"status": {}},
			},
			expected: "Status: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Process(tt.template, tt.ctx)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestProcess_Headers(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name     string
		template string
		ctx      *Context
		expected string
	}{
		{
			name:     "header value",
			template: "Auth: {{header.Authorization}}",
			ctx: &Context{
				Headers: map[string][]string{"Authorization": {"Bearer token123"}},
			},
			expected: "Auth: Bearer token123",
		},
		{
			name:     "header case insensitive",
			template: "Type: {{header.content-type}}",
			ctx: &Context{
				Headers: map[string][]string{"Content-Type": {"application/json"}},
			},
			expected: "Type: application/json",
		},
		{
			name:     "missing header",
			template: "Value: {{header.X-Missing}}",
			ctx: &Context{
				Headers: map[string][]string{},
			},
			expected: "Value: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Process(tt.template, tt.ctx)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestProcess_Body(t *testing.T) {
	e := NewEngine()

	jsonBody := `{"user": {"name": "John", "age": 30}, "items": ["a", "b", "c"]}`

	tests := []struct {
		name     string
		template string
		ctx      *Context
		expected string
	}{
		{
			name:     "body simple path",
			template: "Name: {{body.user.name}}",
			ctx: &Context{
				Body: jsonBody,
			},
			expected: "Name: John",
		},
		{
			name:     "body nested numeric",
			template: "Age: {{body.user.age}}",
			ctx: &Context{
				Body: jsonBody,
			},
			expected: "Age: 30",
		},
		{
			name:     "body array index",
			template: "First: {{body.items.0}}",
			ctx: &Context{
				Body: jsonBody,
			},
			expected: "First: a",
		},
		{
			name:     "body array count",
			template: "Count: {{body.items.#}}",
			ctx: &Context{
				Body: jsonBody,
			},
			expected: "Count: 3",
		},
		{
			name:     "body missing path",
			template: "Missing: {{body.nonexistent}}",
			ctx: &Context{
				Body: jsonBody,
			},
			expected: "Missing: ",
		},
		{
			name:     "body empty",
			template: "Value: {{body.key}}",
			ctx: &Context{
				Body: "",
			},
			expected: "Value: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Process(tt.template, tt.ctx)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestProcess_RandomValues(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	t.Run("random uuid", func(t *testing.T) {
		result := e.Process("{{random.uuid}}", ctx)
		uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
		if !uuidPattern.MatchString(result) {
			t.Errorf("expected UUID format, got %q", result)
		}
	})

	t.Run("random int", func(t *testing.T) {
		result := e.Process("{{random.int}}", ctx)
		if result == "" {
			t.Error("expected non-empty result")
		}
		// Verify it's a number
		for _, c := range result {
			if c < '0' || c > '9' {
				t.Errorf("expected numeric result, got %q", result)
				break
			}
		}
	})

	t.Run("random int with range", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			result := e.Process("{{random.int(1,10)}}", ctx)
			if result == "" {
				t.Error("expected non-empty result")
			}
		}
	})

	t.Run("random float", func(t *testing.T) {
		result := e.Process("{{random.float}}", ctx)
		if !strings.Contains(result, ".") {
			t.Errorf("expected float format, got %q", result)
		}
	})

	t.Run("random float with range", func(t *testing.T) {
		result := e.Process("{{random.float(1.0,10.0)}}", ctx)
		if !strings.Contains(result, ".") {
			t.Errorf("expected float format, got %q", result)
		}
	})

	t.Run("random string", func(t *testing.T) {
		result := e.Process("{{random.string}}", ctx)
		if len(result) != 10 {
			t.Errorf("expected 10 char string, got %d chars", len(result))
		}
	})

	t.Run("random string with length", func(t *testing.T) {
		result := e.Process("{{random.string(20)}}", ctx)
		if len(result) != 20 {
			t.Errorf("expected 20 char string, got %d chars", len(result))
		}
	})

	t.Run("random bool", func(t *testing.T) {
		result := e.Process("{{random.bool}}", ctx)
		if result != "true" && result != "false" {
			t.Errorf("expected true or false, got %q", result)
		}
	})

	t.Run("random email", func(t *testing.T) {
		result := e.Process("{{random.email}}", ctx)
		if !strings.Contains(result, "@example.com") {
			t.Errorf("expected email format, got %q", result)
		}
	})

	t.Run("random name", func(t *testing.T) {
		result := e.Process("{{random.name}}", ctx)
		names := []string{"John", "Jane", "Bob", "Alice", "Charlie", "Diana", "Eve", "Frank"}
		found := false
		for _, name := range names {
			if result == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected a name, got %q", result)
		}
	})

	t.Run("random phone", func(t *testing.T) {
		result := e.Process("{{random.phone}}", ctx)
		if !strings.HasPrefix(result, "+1-") {
			t.Errorf("expected phone format, got %q", result)
		}
	})
}

func TestProcess_Timestamp(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	t.Run("timestamp unix", func(t *testing.T) {
		result := e.Process("{{timestamp.unix}}", ctx)
		if len(result) != 10 {
			t.Errorf("expected 10 digit unix timestamp, got %q", result)
		}
	})

	t.Run("timestamp default is unix", func(t *testing.T) {
		result := e.Process("{{timestamp}}", ctx)
		if len(result) != 10 {
			t.Errorf("expected 10 digit unix timestamp, got %q", result)
		}
	})

	t.Run("timestamp unixMilli", func(t *testing.T) {
		result := e.Process("{{timestamp.unixMilli}}", ctx)
		if len(result) != 13 {
			t.Errorf("expected 13 digit unix milli timestamp, got %q", result)
		}
	})

	t.Run("timestamp unixNano", func(t *testing.T) {
		result := e.Process("{{timestamp.unixNano}}", ctx)
		if len(result) < 18 {
			t.Errorf("expected 19 digit unix nano timestamp, got %q", result)
		}
	})

	t.Run("timestamp iso", func(t *testing.T) {
		result := e.Process("{{timestamp.iso}}", ctx)
		if !strings.Contains(result, "T") {
			t.Errorf("expected ISO format, got %q", result)
		}
	})

	t.Run("timestamp date", func(t *testing.T) {
		result := e.Process("{{timestamp.date}}", ctx)
		datePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
		if !datePattern.MatchString(result) {
			t.Errorf("expected date format YYYY-MM-DD, got %q", result)
		}
	})

	t.Run("timestamp time", func(t *testing.T) {
		result := e.Process("{{timestamp.time}}", ctx)
		timePattern := regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)
		if !timePattern.MatchString(result) {
			t.Errorf("expected time format HH:MM:SS, got %q", result)
		}
	})

	t.Run("timestamp datetime", func(t *testing.T) {
		result := e.Process("{{timestamp.datetime}}", ctx)
		datetimePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`)
		if !datetimePattern.MatchString(result) {
			t.Errorf("expected datetime format, got %q", result)
		}
	})

	t.Run("timestamp add", func(t *testing.T) {
		result := e.Process("{{timestamp.add(1h)}}", ctx)
		if !strings.Contains(result, "T") {
			t.Errorf("expected ISO format, got %q", result)
		}
	})
}

func TestProcess_MixedTemplate(t *testing.T) {
	e := NewEngine()

	ctx := &Context{
		PathParams:  map[string]string{"id": "123"},
		QueryParams: map[string][]string{"format": {"json"}},
		Headers:     map[string][]string{"User-Agent": {"TestClient"}},
		Body:        `{"name": "Test"}`,
	}

	template := `{
		"userId": "{{path.id}}",
		"format": "{{query.format}}",
		"client": "{{header.User-Agent}}",
		"name": "{{body.name}}"
	}`

	result := e.Process(template, ctx)

	if !strings.Contains(result, `"userId": "123"`) {
		t.Error("path param not substituted correctly")
	}
	if !strings.Contains(result, `"format": "json"`) {
		t.Error("query param not substituted correctly")
	}
	if !strings.Contains(result, `"client": "TestClient"`) {
		t.Error("header not substituted correctly")
	}
	if !strings.Contains(result, `"name": "Test"`) {
		t.Error("body value not substituted correctly")
	}
}

func TestProcessHeaders(t *testing.T) {
	e := NewEngine()

	ctx := &Context{
		PathParams: map[string]string{"id": "123"},
	}

	headers := map[string]string{
		"X-Request-ID":   "{{random.uuid}}",
		"X-User-ID":      "{{path.id}}",
		"Content-Type":   "application/json",
		"X-Timestamp":    "{{timestamp.unix}}",
	}

	result := e.ProcessHeaders(headers, ctx)

	if result["X-User-ID"] != "123" {
		t.Errorf("expected X-User-ID to be 123, got %q", result["X-User-ID"])
	}

	if result["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type to be application/json, got %q", result["Content-Type"])
	}

	// UUID should be 36 characters
	if len(result["X-Request-ID"]) != 36 {
		t.Errorf("expected UUID length 36, got %d", len(result["X-Request-ID"]))
	}

	// Timestamp should be 10 digits
	if len(result["X-Timestamp"]) != 10 {
		t.Errorf("expected timestamp length 10, got %d", len(result["X-Timestamp"]))
	}
}

func TestProcess_NoVariables(t *testing.T) {
	e := NewEngine()

	template := "This is a plain string with no variables"
	ctx := &Context{}

	result := e.Process(template, ctx)
	if result != template {
		t.Errorf("expected unchanged template, got %q", result)
	}
}

func TestProcess_UnknownSource(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "unknown source",
			template: "{{unknown.value}}",
			expected: "",
		},
		{
			name:     "env source returns empty",
			template: "{{env.HOME}}",
			expected: "",
		},
		{
			name:     "empty_variable_not_matched",
			template: "{{}}",
			expected: "{{}}", // {{}} doesn't match the pattern (requires at least one char)
		},
	}

	ctx := &Context{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Process(tt.template, ctx)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseParams(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		funcName string
		expected []string
	}{
		{
			name:     "single param",
			key:      "int(100)",
			funcName: "int",
			expected: []string{"100"},
		},
		{
			name:     "multiple params",
			key:      "int(1,100)",
			funcName: "int",
			expected: []string{"1", "100"},
		},
		{
			name:     "wrong func name",
			key:      "int(1,100)",
			funcName: "float",
			expected: nil,
		},
		{
			name:     "empty params",
			key:      "int()",
			funcName: "int",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseParams(tt.key, tt.funcName)
			if tt.expected == nil && result != nil {
				t.Errorf("expected nil, got %v", result)
			} else if tt.expected != nil && result == nil {
				t.Errorf("expected %v, got nil", tt.expected)
			} else if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRandomString(t *testing.T) {
	e := NewEngine()

	t.Run("correct length", func(t *testing.T) {
		result := randomString(e.rng, 15)
		if len(result) != 15 {
			t.Errorf("expected length 15, got %d", len(result))
		}
	})

	t.Run("alphanumeric only", func(t *testing.T) {
		result := randomString(e.rng, 100)
		for _, c := range result {
			isAlphanumeric := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
			if !isAlphanumeric {
				t.Errorf("expected alphanumeric, got character %c", c)
			}
		}
	})

	t.Run("zero length", func(t *testing.T) {
		result := randomString(e.rng, 0)
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})
}
