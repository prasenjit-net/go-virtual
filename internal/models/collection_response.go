package models

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RootKind is the shape of a Collection Response's rendered body: object
// (backed by find-one) or array (backed by find-many).
type RootKind string

const (
	RootKindObject RootKind = "object"
	RootKindArray  RootKind = "array"
)

// QueryMode is the read operation used during response selection.
type QueryMode string

const (
	QueryModeFindOne  = "find-one"
	QueryModeFindMany = "find-many"
)

// ValueSource identifies where a ValueBinding's value comes from.
type ValueSource string

const (
	// ValueSourceDocument reads a path in the current result document.
	// Valid only in field overrides.
	ValueSourceDocument ValueSource = "document"
	// ValueSourceMapper reads "<outputKey>.<path>" from an additional
	// mapper's result. Valid only in field overrides.
	ValueSourceMapper ValueSource = "mapper"
	// ValueSourcePrimary reads a path in the primary result document.
	// Valid in mapper data bindings and additional-mapper filters.
	ValueSourcePrimary ValueSource = "primary"
	ValueSourcePath    ValueSource = "path"
	ValueSourceQuery   ValueSource = "query"
	ValueSourceHeader  ValueSource = "header"
	ValueSourceBody    ValueSource = "body"
	ValueSourceLiteral ValueSource = "literal"
)

// ValueBinding is a typed source for one filter or override value. Unlike
// FieldMappingRule, it preserves JSON types (number/bool/null/object/array)
// for the literal source instead of coercing everything to a string.
type ValueBinding struct {
	Source ValueSource     `json:"source"`
	Key    string          `json:"key,omitempty"`
	Value  json.RawMessage `json:"value,omitempty"` // literal only
}

// CollectionFilter binds one collection query field to a request-derived or
// literal value.
type CollectionFilter struct {
	TargetPath string       `json:"targetPath"`
	Value      ValueBinding `json:"value"`
}

// CollectionQuery configures a response mapper. An omitted primary Mode is
// inferred from the response root. DataRules are used by write operations.
type CollectionQuery struct {
	Mode           CollectionOpType   `json:"mode,omitempty"`
	DataRules      []CollectionFilter `json:"dataRules,omitempty"`
	CollectionName string             `json:"collectionName"`
	FilterRules    []CollectionFilter `json:"filterRules,omitempty"`
}

// NamedQuery is a named additional read or write operation, executed only after
// response selection. The historical type name and JSON shape remain compatible.
type NamedQuery struct {
	OutputKey string           `json:"outputKey"`
	Mode      CollectionOpType `json:"mode"`
	CollectionQuery
}

// FieldOverride replaces the convention-filled value at one template path.
type FieldOverride struct {
	TargetPath string       `json:"targetPath"`
	Value      ValueBinding `json:"value"`
}

// CollectionResponseConfig is the collection-backed rendering configuration
// for a ResponseConfig whose Kind is ResponseKindCollection.
type CollectionResponseConfig struct {
	Primary           CollectionQuery `json:"primary"`
	AdditionalMappers []NamedQuery    `json:"additionalMappers,omitempty"`
	Overrides         []FieldOverride `json:"overrides,omitempty"`
	// TemplateRef selects a named spec example when the operation's response
	// for StatusCode defines more than one. Empty selects the default.
	TemplateRef string `json:"templateRef,omitempty"`
	// RootKind is only consulted in identity mode (the spec defines no JSON
	// body for the status code); otherwise the template's own shape
	// determines the root kind.
	RootKind RootKind `json:"rootKind,omitempty"`
	// MatchOnEmpty opts out of the default data-presence match condition: an
	// empty primary result still matches and renders [] (find-many) or null
	// (find-one) instead of falling through to the next response.
	MatchOnEmpty bool `json:"matchOnEmpty,omitempty"`
	// FallbackToExample controls what fills a template leaf whose mapper
	// path is absent from the document. nil is treated as true.
	FallbackToExample *bool `json:"fallbackToExample,omitempty"`
}

// EffectiveFallbackToExample returns the configured FallbackToExample,
// defaulting to true when unset.
func (c *CollectionResponseConfig) EffectiveFallbackToExample() bool {
	if c == nil || c.FallbackToExample == nil {
		return true
	}
	return *c.FallbackToExample
}

// Validate performs structural validation that does not require the owning
// operation's spec. It rejects malformed bindings, unknown sources, and
// unresolvable mapper-output references. Cross-checks that need the spec's
// response schema (e.g. whether "primary" filters are legal given the
// derived root kind) are performed separately — see
// internal/collectionresponse.ValidateAgainstOperation.
func (c *CollectionResponseConfig) Validate() []string {
	if c == nil {
		return []string{"collectionResponse is required for a collection response"}
	}

	var errs []string

	if strings.TrimSpace(c.Primary.CollectionName) == "" {
		errs = append(errs, "primary.collectionName is required")
	}
	errs = append(errs, validateFilterBindings("primary.filterRules", c.Primary.FilterRules, false)...)
	if c.Primary.Mode != "" && c.Primary.Mode != ColOpFindOne && c.Primary.Mode != ColOpFindMany && c.Primary.Mode != ColOpUpdate {
		errs = append(errs, "primary.mode must be find-one, find-many, or update")
	}
	errs = append(errs, validateOperationFields("primary", c.Primary.Mode, c.Primary.FilterRules, c.Primary.DataRules)...)

	outputKeys := make(map[string]bool, len(c.AdditionalMappers))
	for i, m := range c.AdditionalMappers {
		p := fmt.Sprintf("additionalMappers[%d]", i)
		key := strings.TrimSpace(m.OutputKey)
		if key == "" {
			errs = append(errs, p+".outputKey is required")
		} else if outputKeys[key] {
			errs = append(errs, fmt.Sprintf("%s.outputKey %q is duplicated", p, key))
		} else {
			outputKeys[key] = true
		}
		switch m.Mode {
		case ColOpFindOne, ColOpFindMany, ColOpInsert, ColOpUpdate, ColOpUpsert, ColOpDelete:
		default:
			errs = append(errs, p+".mode is invalid")
		}
		errs = append(errs, validateOperationFields(p, m.Mode, m.FilterRules, m.DataRules)...)
		if m.CollectionQuery.Mode != "" {
			errs = append(errs, p+" has conflicting embedded mode")
		}
		if strings.TrimSpace(m.CollectionName) == "" {
			errs = append(errs, p+".collectionName is required")
		}
		errs = append(errs, validateFilterBindings(p+".filterRules", m.FilterRules, true)...)
	}

	for i, o := range c.Overrides {
		p := fmt.Sprintf("overrides[%d]", i)
		if strings.TrimSpace(o.TargetPath) == "" {
			errs = append(errs, p+".targetPath is required")
		}
		errs = append(errs, validateValueBinding(p+".value", o.Value, overrideSources)...)
		if o.Value.Source == ValueSourceMapper {
			key := o.Value.Key
			if idx := strings.Index(key, "."); idx > 0 {
				key = key[:idx]
			}
			if !outputKeys[key] {
				errs = append(errs, fmt.Sprintf("%s.value: mapper output key %q is not declared in additionalMappers", p, key))
			}
		}
	}

	if c.RootKind != "" && c.RootKind != RootKindObject && c.RootKind != RootKindArray {
		errs = append(errs, `rootKind must be "object" or "array"`)
	}

	return errs
}

var filterSourcesBase = map[ValueSource]bool{
	ValueSourcePath:    true,
	ValueSourceQuery:   true,
	ValueSourceHeader:  true,
	ValueSourceBody:    true,
	ValueSourceLiteral: true,
}

var overrideSources = map[ValueSource]bool{
	ValueSourceDocument: true,
	ValueSourceMapper:   true,
	ValueSourcePath:     true,
	ValueSourceQuery:    true,
	ValueSourceHeader:   true,
	ValueSourceBody:     true,
	ValueSourceLiteral:  true,
}

func validateFilterBindings(prefix string, filters []CollectionFilter, allowPrimarySource bool) []string {
	var errs []string
	allowed := filterSourcesBase
	if allowPrimarySource {
		allowed = map[ValueSource]bool{
			ValueSourcePath:    true,
			ValueSourceQuery:   true,
			ValueSourceHeader:  true,
			ValueSourceBody:    true,
			ValueSourceLiteral: true,
			ValueSourcePrimary: true,
		}
	}
	for i, f := range filters {
		if strings.TrimSpace(f.TargetPath) == "" {
			errs = append(errs, fmt.Sprintf("%s[%d].targetPath is required", prefix, i))
		}
		errs = append(errs, validateValueBinding(fmt.Sprintf("%s[%d].value", prefix, i), f.Value, allowed)...)
	}
	return errs
}

func validateValueBinding(label string, v ValueBinding, allowed map[ValueSource]bool) []string {
	if v.Source == "" {
		return []string{label + ": source is required"}
	}
	if !allowed[v.Source] {
		return []string{fmt.Sprintf("%s: source %q is not allowed here", label, v.Source)}
	}
	switch v.Source {
	case ValueSourceLiteral:
		if len(v.Value) == 0 || !json.Valid(v.Value) {
			return []string{label + ": literal value must be valid JSON"}
		}
	case ValueSourceMapper:
		if v.Key == "" || !strings.Contains(v.Key, ".") {
			return []string{label + `: mapper key must be "<outputKey>.<path>"`}
		}
	default: // document, primary, path, query, header, body
		if strings.TrimSpace(v.Key) == "" {
			return []string{label + ": key is required for source " + string(v.Source)}
		}
	}
	return nil
}

// validateOperationFields validates typed write bindings without altering read behavior.
func validateOperationFields(prefix string, mode CollectionOpType, filters, data []CollectionFilter) []string {
	var errs []string
	writesData := mode == ColOpInsert || mode == ColOpUpdate || mode == ColOpUpsert
	if writesData && len(data) == 0 {
		errs = append(errs, prefix+".dataRules is required")
	}
	if !writesData && len(data) > 0 {
		errs = append(errs, prefix+".dataRules is not allowed for this operation")
	}
	if mode == ColOpInsert && len(filters) > 0 {
		errs = append(errs, prefix+".filterRules is not allowed for insert")
	}
	if (mode == ColOpUpdate || mode == ColOpUpsert || mode == ColOpDelete) && len(filters) == 0 {
		errs = append(errs, prefix+".filterRules is required")
	}
	errs = append(errs, validateFilterBindings(prefix+".dataRules", data, true)...)
	seen := map[string]bool{}
	for _, rule := range data {
		if strings.Contains(rule.TargetPath, ".") {
			errs = append(errs, prefix+".dataRules: dotted targets are not supported")
		}
		if rule.TargetPath == "_id" && mode != ColOpInsert {
			errs = append(errs, prefix+".dataRules: _id cannot be changed")
		}
		if seen[rule.TargetPath] {
			errs = append(errs, prefix+".dataRules: duplicate target "+rule.TargetPath)
		}
		seen[rule.TargetPath] = true
	}
	return errs
}
