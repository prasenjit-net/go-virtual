// Package collectionresponse implements the Collection Response kind: a
// configured response that matches only when its primary collection query
// returns data, and renders its body by filling the operation's spec-defined
// response shape from that data by naming convention.
package collectionresponse

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/parser"
)

// TemplateSource identifies where a resolved template's shape came from.
type TemplateSource string

const (
	// TemplateSourceExample means the template is a direct or named spec example.
	TemplateSourceExample TemplateSource = "example"
	// TemplateSourceSchema means the template was generated from the spec's schema.
	TemplateSourceSchema TemplateSource = "schema"
	// TemplateSourceIdentity means the spec defines no JSON body for this
	// status code; the document renders as-is.
	TemplateSourceIdentity TemplateSource = "identity"
)

// ResolvedTemplate is an operation's spec response body for one status code,
// parsed into a tree that schema-fill rendering walks.
type ResolvedTemplate struct {
	// Value is the parsed template tree. Nil when Source is
	// TemplateSourceIdentity.
	Value any
	// Root is the template's shape: object or array. In identity mode this
	// is filled in by the caller from the response's configured/explicit
	// root kind.
	Root   models.RootKind
	Source TemplateSource
}

// ItemTemplate returns the per-item shape for an array-rooted template: the
// first element, or nil when the template array is empty (no shape known).
func (t *ResolvedTemplate) ItemTemplate() any {
	if t == nil {
		return nil
	}
	if arr, ok := t.Value.([]any); ok && len(arr) > 0 {
		return arr[0]
	}
	return nil
}

// ResolveTemplate finds the operation's spec-defined response body for
// statusCode — optionally selecting a named example via templateRef — and
// parses it into a ResolvedTemplate. A status code with no JSON body (or no
// response definition at all) resolves to TemplateSourceIdentity.
func ResolveTemplate(p *parser.Parser, specContent string, op *models.Operation, statusCode int, templateRef string) (*ResolvedTemplate, error) {
	defs, err := p.ExtractAllResponses(specContent, op.Method, op.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve collection response template: %w", err)
	}

	var chosen *parser.SpecResponseDef
	for i := range defs {
		d := &defs[i]
		if d.StatusCode != statusCode {
			continue
		}
		if templateRef != "" {
			if d.ExampleName == templateRef {
				chosen = d
				break
			}
			continue
		}
		if chosen == nil {
			chosen = d
		}
	}

	if chosen == nil || strings.TrimSpace(chosen.BodyExample) == "" {
		return &ResolvedTemplate{Source: TemplateSourceIdentity}, nil
	}

	var v any
	if err := json.Unmarshal([]byte(chosen.BodyExample), &v); err != nil {
		return nil, fmt.Errorf("collection response template for status %d is not valid JSON: %w", statusCode, err)
	}

	source := TemplateSourceExample
	if chosen.SchemaHint != "" {
		source = TemplateSourceSchema
	}

	root := models.RootKindObject
	if _, ok := v.([]any); ok {
		root = models.RootKindArray
	}

	return &ResolvedTemplate{Value: v, Root: root, Source: source}, nil
}
