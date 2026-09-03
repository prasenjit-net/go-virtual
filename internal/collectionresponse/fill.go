package collectionresponse

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/prasenjit/go-virtual/internal/collection"
	"github.com/prasenjit/go-virtual/internal/models"
)

// fillContext carries the inputs shared across one document's fill pass.
type fillContext struct {
	rootDoc           map[string]any
	overrides         map[string]models.FieldOverride
	request           *collection.TypedRequestContext
	mappers           map[string]any
	fallbackToExample bool
	warnings          []string
}

// FillDocument fills template using doc as the current result document,
// applying overrides and falling back to the template's own example values
// (or null) for paths the document does not have. It returns the filled
// value and any fill warnings (missing paths, shape mismatches, unresolved
// overrides).
func FillDocument(template any, doc map[string]any, overrides map[string]models.FieldOverride, request *collection.TypedRequestContext, mappers map[string]any, fallbackToExample bool) (any, []string) {
	ctx := &fillContext{
		rootDoc:           doc,
		overrides:         overrides,
		request:           request,
		mappers:           mappers,
		fallbackToExample: fallbackToExample,
	}
	var sub any
	found := false
	if doc != nil {
		sub = doc
		found = true
	}
	result := ctx.fillNode(template, sub, found, "")
	return result, ctx.warnings
}

func (ctx *fillContext) fillNode(template any, sub any, subFound bool, fullPath string) any {
	if ov, ok := ctx.overrides[fullPath]; ok {
		bctx := &collection.BindingContext{Request: ctx.request, Document: ctx.rootDoc, Mappers: ctx.mappers}
		v, found, err := collection.ResolveValueBinding(ov.Value, bctx)
		if err != nil {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: override error: %s", labelFor(fullPath), err))
			return nil
		}
		if !found {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: override source has no value", labelFor(fullPath)))
			return nil
		}
		return v
	}

	switch t := template.(type) {
	case map[string]any:
		var subMap map[string]any
		if subFound {
			m, ok := sub.(map[string]any)
			if !ok {
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: expected an object in the document but found %T", labelFor(fullPath), sub))
			} else {
				subMap = m
			}
		}
		result := make(map[string]any, len(t))
		for k, v := range t {
			childPath := joinPath(fullPath, k)
			var childVal any
			childFound := false
			if subMap != nil {
				if cv, ok := subMap[k]; ok {
					childVal, childFound = cv, true
				}
			}
			result[k] = ctx.fillNode(v, childVal, childFound, childPath)
		}
		return result

	case []any:
		if len(t) == 0 {
			if arr, ok := sub.([]any); ok {
				return arr
			}
			return []any{}
		}
		item := t[0]
		arr, ok := sub.([]any)
		if !ok {
			if subFound {
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: expected an array in the document but found %T", labelFor(fullPath), sub))
			}
			return []any{}
		}
		result := make([]any, 0, len(arr))
		for _, elem := range arr {
			result = append(result, ctx.fillNode(item, elem, true, fullPath))
		}
		return result

	default:
		if subFound {
			return sub
		}
		if ctx.fallbackToExample {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: no document value; using template example", labelFor(fullPath)))
			return template
		}
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: no document value; rendered as null", labelFor(fullPath)))
		return nil
	}
}

func labelFor(path string) string {
	if path == "" {
		return "(root)"
	}
	return path
}

func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

// fillIdentity is used in identity mode (no spec-defined template): the
// document is echoed as-is with overrides applied by absolute path.
func fillIdentity(doc map[string]any, overrides map[string]models.FieldOverride, request *collection.TypedRequestContext, mappers map[string]any) (any, []string) {
	copied := deepCopyJSON(doc)
	var warnings []string
	m, ok := copied.(map[string]any)
	if !ok {
		return copied, warnings
	}
	for path, ov := range overrides {
		bctx := &collection.BindingContext{Request: request, Document: doc, Mappers: mappers}
		v, found, err := collection.ResolveValueBinding(ov.Value, bctx)
		if err != nil || !found {
			warnings = append(warnings, fmt.Sprintf("%s: override could not be resolved", labelFor(path)))
			continue
		}
		setPath(m, path, v)
	}
	return m, warnings
}

// setPath writes value at a dot-separated path into root, creating
// intermediate map nodes as needed. Numeric segments are not treated as
// array indices — identity-mode overrides only create/replace object keys.
func setPath(root map[string]any, path string, value any) {
	segs := strings.Split(path, ".")
	cur := root
	for i, seg := range segs {
		if i == len(segs)-1 {
			cur[seg] = value
			return
		}
		next, ok := cur[seg].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[seg] = next
		}
		cur = next
	}
}

func deepCopyJSON(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}
