//go:build unit

package template

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIterFuncs_Times(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{range $i, $_ := times 3}}{{$i}}{{end}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "012" {
		t.Errorf("expected 012, got %q", result)
	}
}

func TestIterFuncs_TimesZero(t *testing.T) {
	fm := buildIterFuncMap()
	timesFn := fm["times"].(func(int) []int)
	result := timesFn(0)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestIterFuncs_TimesNegative(t *testing.T) {
	fm := buildIterFuncMap()
	timesFn := fm["times"].(func(int) []int)
	result := timesFn(-3)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestIterFuncs_Seq(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{range seq 1 3}}{{.}} {{end}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result) != "1 2 3" {
		t.Errorf("expected '1 2 3', got %q", result)
	}
}

func TestIterFuncs_SeqReversed(t *testing.T) {
	fm := buildIterFuncMap()
	seqFn := fm["seq"].(func(int, int) []int)
	result := seqFn(5, 3)
	if len(result) != 0 {
		t.Errorf("expected empty slice for reversed seq, got %v", result)
	}
}

func TestIterFuncs_List(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{range list "a" "b" "c"}}{{.}}{{end}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "abc" {
		t.Errorf("expected abc, got %q", result)
	}
}

func TestRenderBodyTemplate_RangeCollectionWithJSONStrings(t *testing.T) {
	e := NewEngine()
	body := `[
  {{range $i, $item := .Collection.u}}{{if $i}},{{end}}
  {
    "age": {{index $item ` + "`age`" + `}},
    "email": "{{index $item ` + "`email`" + `}}",
    "id": "{{index $item ` + "`id`" + `}}",
    "name": "{{index $item ` + "`name`" + `}}"
  }
  {{end}}
]`
	ctx := &Context{
		CollectionOutput: map[string]any{
			"u": []any{
				map[string]any{"age": 32, "email": "alice@example.com", "id": "1", "name": "Alice Johnson"},
				map[string]any{"age": 45, "email": "bob@example.com", "id": "2", "name": "Bob Smith"},
			},
		},
	}

	result, err := e.RenderBodyTemplate(body, ctx)
	if err != nil {
		t.Fatalf("RenderBodyTemplate error: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(result), &rows); err != nil {
		t.Fatalf("rendered body is not JSON: %v\n%s", err, result)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["email"] != "alice@example.com" || rows[1]["name"] != "Bob Smith" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestRenderBodyTemplate_ObjectWithNestedCollectionRange(t *testing.T) {
	e := NewEngine()
	body := `{
  "id": "{{index .Collection.user ` + "`id`" + `}}",
  "name": "{{index .Collection.user ` + "`name`" + `}}",
  "orders": [
    {{range $i, $item := .Collection.orders}}{{if $i}},{{end}}
    {
      "id": "{{index $item ` + "`id`" + `}}",
      "total": {{index $item ` + "`total`" + `}}
    }
    {{end}}
  ]
}`
	ctx := &Context{
		CollectionOutput: map[string]any{
			"user": map[string]any{"id": "u1", "name": "Alice"},
			"orders": []any{
				map[string]any{"id": "o1", "total": 10},
				map[string]any{"id": "o2", "total": 20},
			},
		},
	}

	result, err := e.RenderBodyTemplate(body, ctx)
	if err != nil {
		t.Fatalf("RenderBodyTemplate error: %v", err)
	}

	var rendered struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Orders []struct {
			ID    string  `json:"id"`
			Total float64 `json:"total"`
		} `json:"orders"`
	}
	if err := json.Unmarshal([]byte(result), &rendered); err != nil {
		t.Fatalf("rendered body is not JSON: %v\n%s", err, result)
	}
	if rendered.ID != "u1" || rendered.Name != "Alice" {
		t.Fatalf("unexpected user: %+v", rendered)
	}
	if len(rendered.Orders) != 2 || rendered.Orders[1].ID != "o2" || rendered.Orders[1].Total != 20 {
		t.Fatalf("unexpected orders: %+v", rendered.Orders)
	}
}

func TestRenderBodyTemplate_RangeCollectionWithNestedItemRange(t *testing.T) {
	e := NewEngine()
	body := `[
  {{range $i, $item := .Collection.u}}{{if $i}},{{end}}
  {
    "age": {{index $item ` + "`age`" + `}},
    "email": "{{index $item ` + "`email`" + `}}",
    "id": "{{index $item ` + "`id`" + `}}",
    "name": "{{index $item ` + "`name`" + `}}",
    "orders": [
      {{range $i, $item := index $item ` + "`orders`" + `}}{{if $i}},{{end}}
      {
        "id": "{{index $item ` + "`id`" + `}}",
        "amount": {{index $item ` + "`amount`" + `}}
      }
      {{end}}
    ]
  }
  {{end}}
]`
	ctx := &Context{
		CollectionOutput: map[string]any{
			"u": []any{
				map[string]any{
					"age":   32,
					"email": "alice@example.com",
					"id":    "1",
					"name":  "Alice Johnson",
					"orders": []any{
						map[string]any{"id": "o1", "amount": 10},
						map[string]any{"id": "o2", "amount": 20},
					},
				},
				map[string]any{
					"age":    45,
					"email":  "bob@example.com",
					"id":     "2",
					"name":   "Bob Smith",
					"orders": []any{map[string]any{"id": "o3", "amount": 30}},
				},
			},
		},
	}

	result, err := e.RenderBodyTemplate(body, ctx)
	if err != nil {
		t.Fatalf("RenderBodyTemplate error: %v", err)
	}

	var rows []struct {
		ID     string `json:"id"`
		Orders []struct {
			ID     string  `json:"id"`
			Amount float64 `json:"amount"`
		} `json:"orders"`
	}
	if err := json.Unmarshal([]byte(result), &rows); err != nil {
		t.Fatalf("rendered body is not JSON: %v\n%s", err, result)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 users, got %d", len(rows))
	}
	if len(rows[0].Orders) != 2 || rows[0].Orders[1].ID != "o2" || rows[0].Orders[1].Amount != 20 {
		t.Fatalf("unexpected first user orders: %+v", rows[0].Orders)
	}
	if len(rows[1].Orders) != 1 || rows[1].Orders[0].ID != "o3" || rows[1].Orders[0].Amount != 30 {
		t.Fatalf("unexpected second user orders: %+v", rows[1].Orders)
	}
}

func TestIterFuncs_FirstLast(t *testing.T) {
	fm := buildIterFuncMap()
	listFn := fm["list"].(func(...interface{}) []interface{})
	firstFn := fm["first"].(func([]interface{}) interface{})
	lastFn := fm["last"].(func([]interface{}) interface{})

	items := listFn("x", "y", "z")
	if firstFn(items) != "x" {
		t.Errorf("expected x, got %v", firstFn(items))
	}
	if lastFn(items) != "z" {
		t.Errorf("expected z, got %v", lastFn(items))
	}
}

func TestIterFuncs_FirstLastEmpty(t *testing.T) {
	fm := buildIterFuncMap()
	firstFn := fm["first"].(func([]interface{}) interface{})
	lastFn := fm["last"].(func([]interface{}) interface{})

	empty := []interface{}{}
	if firstFn(empty) != "" {
		t.Errorf("expected empty, got %v", firstFn(empty))
	}
	if lastFn(empty) != "" {
		t.Errorf("expected empty, got %v", lastFn(empty))
	}
}

func TestIterFuncs_Len(t *testing.T) {
	fm := buildIterFuncMap()
	listFn := fm["list"].(func(...interface{}) []interface{})
	lenFn := fm["len"].(func([]interface{}) int)

	items := listFn("a", "b", "c", "d")
	if lenFn(items) != 4 {
		t.Errorf("expected 4, got %d", lenFn(items))
	}
}
