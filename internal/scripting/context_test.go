package scripting

import (
	"net/http/httptest"
	"testing"
)

func TestBuildInput_PathAndQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/pets/42?status=active&sort=asc", nil)
	input := BuildInput(map[string]string{"id": "42"}, req, "")

	if input.Path["id"] != "42" {
		t.Errorf("path.id: got %q", input.Path["id"])
	}
	if input.Query["status"] != "active" {
		t.Errorf("query.status: got %q", input.Query["status"])
	}
	if input.Query["sort"] != "asc" {
		t.Errorf("query.sort: got %q", input.Query["sort"])
	}
	if input.Body != nil {
		t.Error("Expected nil body for empty string input")
	}
}

func TestBuildInput_Headers(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "abc123")

	input := BuildInput(nil, req, "")

	if input.Header["content-type"] != "application/json" {
		t.Errorf("header content-type: got %q", input.Header["content-type"])
	}
	if input.Header["x-request-id"] != "abc123" {
		t.Errorf("header x-request-id: got %q", input.Header["x-request-id"])
	}
}

func TestBuildInput_JSONBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	input := BuildInput(nil, req, `{"name":"alice","age":30}`)

	m, ok := input.Body.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map body, got %T", input.Body)
	}
	if m["name"] != "alice" {
		t.Errorf("body.name: got %v", m["name"])
	}
}

func TestBuildInput_RawStringBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	input := BuildInput(nil, req, "not-valid-json")

	if input.Body != "not-valid-json" {
		t.Errorf("Expected raw string body, got %v", input.Body)
	}
}

func TestBuildInput_NilPathParams(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	input := BuildInput(nil, req, "")

	if input.Path == nil {
		t.Error("Expected non-nil Path map")
	}
	if len(input.Path) != 0 {
		t.Errorf("Expected empty Path map, got %v", input.Path)
	}
}

func TestBuildInput_MultipleQueryValues(t *testing.T) {
	req := httptest.NewRequest("GET", "/?tag=a&tag=b&tag=c", nil)
	input := BuildInput(nil, req, "")

	// Only first value per key
	if input.Query["tag"] != "a" {
		t.Errorf("Expected first query value 'a', got %q", input.Query["tag"])
	}
}
