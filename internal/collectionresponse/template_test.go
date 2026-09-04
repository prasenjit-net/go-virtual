package collectionresponse

import (
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/parser"
)

const testSpecContent = `{
  "openapi": "3.0.0",
  "info": {"title": "t", "version": "1.0"},
  "paths": {
    "/users/{id}": {
      "get": {
        "operationId": "getUser",
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {
                "example": {"id": "placeholder", "name": "placeholder", "profile": {"nickname": "anon"}}
              }
            }
          }
        }
      }
    },
    "/users": {
      "get": {
        "operationId": "listUsers",
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {
                "example": [{"id": "placeholder", "name": "placeholder"}]
              }
            }
          },
          "404": {
            "description": "not found",
            "content": {
              "application/json": {
                "examples": {
                  "empty": {"value": {"error": "not found"}},
                  "detailed": {"value": {"error": "not found", "code": 404}}
                }
              }
            }
          }
        }
      }
    },
    "/ping": {
      "get": {
        "operationId": "ping",
        "responses": {
          "204": {"description": "no content"}
        }
      }
    }
  }
}`

func TestResolveTemplate_ObjectRoot(t *testing.T) {
	p := parser.NewParser()
	op := &models.Operation{Method: "GET", Path: "/users/{id}"}
	tmpl, err := ResolveTemplate(p, testSpecContent, op, 200, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Root != models.RootKindObject {
		t.Fatalf("root = %v, want object", tmpl.Root)
	}
	if tmpl.Source != TemplateSourceExample {
		t.Fatalf("source = %v, want example", tmpl.Source)
	}
	m, ok := tmpl.Value.(map[string]any)
	if !ok || m["id"] != "placeholder" {
		t.Fatalf("unexpected template value: %#v", tmpl.Value)
	}
}

func TestResolveTemplate_ArrayRootAndItemTemplate(t *testing.T) {
	p := parser.NewParser()
	op := &models.Operation{Method: "GET", Path: "/users"}
	tmpl, err := ResolveTemplate(p, testSpecContent, op, 200, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Root != models.RootKindArray {
		t.Fatalf("root = %v, want array", tmpl.Root)
	}
	item := tmpl.ItemTemplate()
	m, ok := item.(map[string]any)
	if !ok || m["id"] != "placeholder" {
		t.Fatalf("unexpected item template: %#v", item)
	}
}

func TestResolveTemplate_NamedExample(t *testing.T) {
	p := parser.NewParser()
	op := &models.Operation{Method: "GET", Path: "/users"}
	tmpl, err := ResolveTemplate(p, testSpecContent, op, 404, "detailed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := tmpl.Value.(map[string]any)
	if m["code"] != float64(404) {
		t.Fatalf("expected the 'detailed' named example, got %#v", m)
	}
}

func TestResolveTemplate_IdentityMode(t *testing.T) {
	p := parser.NewParser()
	op := &models.Operation{Method: "GET", Path: "/ping"}
	tmpl, err := ResolveTemplate(p, testSpecContent, op, 204, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Source != TemplateSourceIdentity {
		t.Fatalf("source = %v, want identity", tmpl.Source)
	}
	if tmpl.Value != nil {
		t.Fatalf("expected nil template value in identity mode, got %#v", tmpl.Value)
	}
}

func TestResolveTemplate_UnknownStatusCodeIsIdentity(t *testing.T) {
	p := parser.NewParser()
	op := &models.Operation{Method: "GET", Path: "/users/{id}"}
	tmpl, err := ResolveTemplate(p, testSpecContent, op, 500, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Source != TemplateSourceIdentity {
		t.Fatalf("source = %v, want identity for an undefined status code", tmpl.Source)
	}
}
