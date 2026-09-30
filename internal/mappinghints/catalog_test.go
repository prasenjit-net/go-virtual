package mappinghints

import (
	"encoding/json"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"testing"
)

func TestObservedShapesAndSensitiveValues(t *testing.T) {
	c := New()
	var body any
	json.Unmarshal([]byte(`{"addresses":[{"city":"Boston"},{"postalCode":"02108"}],"email":"private@example.com","empty":"","nil":null,"flag":false,"zero":0,"dot.key":1}`), &body)
	c.Observe(body, Item{Source: "body", Origin: "observed"}, 0)
	fields := map[string]Item{}
	for _, i := range c.Items {
		fields[i.Key] = i
	}
	for _, key := range []string{"addresses", "addresses.0.city", "addresses.0.postalCode", "empty", "nil", "flag", "zero"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}
	if len(fields["email"].Values) != 0 {
		t.Fatal("sensitive example exposed")
	}
	if _, ok := fields["dot.key"]; ok {
		t.Fatal("unsafe path suggested")
	}
	if fields["nil"].Type != "null" || fields["empty"].Values[0] != `""` || fields["flag"].Values[0] != "false" || fields["zero"].Values[0] != "0" {
		t.Fatal("falsey values lost")
	}
}
func TestSchemaPrecedenceAndLimits(t *testing.T) {
	c := New()
	c.Observe(42.0, Item{Source: "body", Key: "name", Origin: "observed"}, 0)
	schema := openapi3.NewStringSchema().WithEnum("Alice", "Bob")
	schema.Nullable = true
	c.Schema(schema, Item{Source: "body", Key: "name", Required: true}, map[*openapi3.Schema]bool{}, 0)
	c.Observe(3.0, Item{Source: "body", Key: "name", Origin: "observed"}, 0)
	if c.Items[0].Type != "string" || c.Items[0].Origin != "schema" || !c.Items[0].Conditional || !c.Items[0].Required || !c.Items[0].Nullable {
		t.Fatalf("schema: %+v", c.Items[0])
	}
	for i := 0; i < 600; i++ {
		c.Add(Item{Source: "body", Key: fmt.Sprint(i)})
	}
	if len(c.Items) != MaxFields || !c.Truncated {
		t.Fatal("unbounded catalog")
	}
}
func TestHeaderNormalizationAndSchemaCycles(t *testing.T) {
	c := New()
	c.Add(Item{Source: "header", Key: "X-Request-ID", Type: "string", Origin: "schema"})
	c.Observe("id", Item{Source: "header", Key: "x-request-id", Origin: "observed"}, 0)
	if len(c.Items) != 1 {
		t.Fatal("duplicate header")
	}
	s := openapi3.NewObjectSchema()
	s.Properties["child"] = &openapi3.SchemaRef{Value: s}
	c.Schema(s, Item{Source: "body"}, map[*openapi3.Schema]bool{}, 0)
	if !c.Truncated {
		t.Fatal("cycle not bounded")
	}
}
