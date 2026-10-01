// Package mappinghints builds bounded, read-only field metadata for editors.
package mappinghints

import (
	"encoding/json"
	"github.com/getkin/kin-openapi/openapi3"
	"sort"
	"strings"
)

const MaxFields = 500

type Item struct {
	Source      string   `json:"source"`
	Key         string   `json:"key"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Origin      string   `json:"origin"`
	Required    bool     `json:"required,omitempty"`
	Nullable    bool     `json:"nullable,omitempty"`
	Conditional bool     `json:"conditional,omitempty"`
	Values      []string `json:"values,omitempty"`
	ProducerID  string   `json:"producerId,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	Order       int      `json:"order,omitempty"`
	Collection  string   `json:"collection,omitempty"`
}
type Catalog struct {
	Items     []Item   `json:"items"`
	Truncated bool     `json:"truncated"`
	Warnings  []string `json:"warnings,omitempty"`
}

func New() *Catalog { return &Catalog{Items: []Item{}} }
func Sensitive(key string) bool {
	key = strings.ToLower(key)
	for _, part := range []string{"authorization", "cookie", "password", "secret", "token", "api-key", "apikey", "email", "phone", "ssn"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}
func (c *Catalog) Add(item Item) {
	if item.Key == "" {
		return
	}
	if Sensitive(item.Key) {
		item.Values = nil
	}
	for i := range c.Items {
		prev := &c.Items[i]
		same := prev.Key == item.Key || item.Source == "header" && strings.EqualFold(prev.Key, item.Key)
		if same && prev.Source == item.Source && prev.ProducerID == item.ProducerID && prev.Collection == item.Collection {
			if prev.Type != item.Type && item.Type != "unknown" {
				prev.Conditional = true
			}
			if item.Origin == "schema" && prev.Origin != "schema" {
				*prev = item
				return
			}
			for _, value := range item.Values {
				if len(prev.Values) < 5 && !contains(prev.Values, value) {
					prev.Values = append(prev.Values, value)
				}
			}
			return
		}
	}
	if len(c.Items) >= MaxFields {
		c.Truncated = true
		return
	}
	if len(item.Values) > 5 {
		item.Values = item.Values[:5]
	}
	c.Items = append(c.Items, item)
}
func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
func SafePart(key string) bool { return key != "" && !strings.ContainsAny(key, ".\\#*?[]|!@:") }
func Join(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}
func JSONType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, int, int64, json.Number:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "unknown"
}
func (c *Catalog) Observe(v any, base Item, depth int) {
	if depth > 8 || len(c.Items) >= MaxFields {
		c.Truncated = true
		return
	}
	base.Type = JSONType(v)
	base.Values = nil
	if base.Type != "array" && base.Type != "object" && base.Type != "unknown" && !Sensitive(base.Key) {
		raw, err := json.Marshal(v)
		if err == nil && len(raw) <= 120 {
			base.Values = []string{string(raw)}
		}
	}
	c.Add(base)
	switch value := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for k := range value {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if SafePart(k) {
				child := base
				child.Key = Join(base.Key, k)
				c.Observe(value[k], child, depth+1)
			}
		}
	case []any:
		for i, child := range value {
			if i >= 5 {
				break
			}
			next := base
			next.Key = Join(base.Key, "0")
			next.Description = "Array item (representative index 0)"
			c.Observe(child, next, depth+1)
		}
	}
}
func (c *Catalog) Schema(s *openapi3.Schema, base Item, seen map[*openapi3.Schema]bool, depth int) {
	if s == nil {
		return
	}
	if depth > 8 || seen[s] {
		c.Truncated = true
		return
	}
	seen[s] = true
	defer delete(seen, s)
	base.Origin = "schema"
	base.Description = s.Description
	base.Nullable = s.Nullable
	base.Type = "unknown"
	base.Values = nil
	if s.Type != nil && len(*s.Type) > 0 {
		base.Type = (*s.Type)[0]
	}
	if len(s.Properties) > 0 {
		base.Type = "object"
	}
	if s.Items != nil {
		base.Type = "array"
	}
	if s.Format != "" {
		base.Description += " (" + s.Format + ")"
	}
	for _, v := range s.Enum {
		b, _ := json.Marshal(v)
		base.Values = append(base.Values, string(b))
	}
	if s.Example != nil {
		b, _ := json.Marshal(s.Example)
		if len(b) <= 120 {
			base.Values = append(base.Values, string(b))
		}
	}
	if s.Default != nil {
		b, _ := json.Marshal(s.Default)
		if len(b) <= 120 {
			base.Values = append(base.Values, string(b))
		}
	}
	if base.Type == "boolean" && len(base.Values) == 0 {
		base.Values = []string{"true", "false"}
	}
	c.Add(base)
	keys := make([]string, 0, len(s.Properties))
	for k := range s.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ref := s.Properties[k]
		if ref != nil && SafePart(k) {
			child := base
			child.Key = Join(base.Key, k)
			child.Required = base.Required && contains(s.Required, k)
			c.Schema(ref.Value, child, seen, depth+1)
		}
	}
	if s.Items != nil {
		child := base
		child.Key = Join(base.Key, "0")
		child.Required = false
		c.Schema(s.Items.Value, child, seen, depth+1)
	}
	for _, ref := range s.AllOf {
		if ref != nil {
			c.Schema(ref.Value, base, seen, depth+1)
		}
	}
	for _, refs := range [][]*openapi3.SchemaRef{s.OneOf, s.AnyOf} {
		for _, ref := range refs {
			if ref != nil {
				child := base
				child.Conditional = true
				c.Schema(ref.Value, child, seen, depth+1)
			}
		}
	}
}
