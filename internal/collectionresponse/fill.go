package collectionresponse

import (
	"encoding/json"
	"fmt"
	"strconv"
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
	mappings          []models.ResponseFieldMappingTrace
}

// FillDocument fills template using doc as the current result document,
// applying overrides and falling back to the template's own example values
// (or null) for paths the document does not have. It returns the filled
// value and any fill warnings (missing paths, shape mismatches, unresolved
// overrides).
func FillDocument(template any, doc map[string]any, overrides map[string]models.FieldOverride, request *collection.TypedRequestContext, mappers map[string]any, fallbackToExample bool) (any, []string) {
	value, warnings, _ := fillDocumentWithTrace(template, doc, overrides, request, mappers, fallbackToExample)
	return value, warnings
}

func fillDocumentWithTrace(template any, doc map[string]any, overrides map[string]models.FieldOverride, request *collection.TypedRequestContext, mappers map[string]any, fallbackToExample bool) (any, []string, []models.ResponseFieldMappingTrace) {
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
	result := ctx.fillNode(template, sub, found, "", "", true)
	return result, ctx.warnings, ctx.mappings
}

func (ctx *fillContext) fillNode(template any, sub any, subFound bool, fullPath, concretePath string, resolveSource bool) any {
	if resolveSource {
		selected := false
		ov, hasOverride := ctx.overrides[concretePath]
		if !hasOverride {
			ov, hasOverride = ctx.overrides[fullPath]
		}
		if hasOverride {
			bctx := &collection.BindingContext{Request: ctx.request, Document: ctx.rootDoc, Mappers: ctx.mappers}
			v, found, err := collection.ResolveValueBinding(ov.Value, bctx)
			if err != nil {
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: override error: %s", labelFor(concretePath), err))
				return nil
			}
			if found {
				sub, subFound, selected = v, true, true
				ctx.mappings = append(ctx.mappings, models.ResponseFieldMappingTrace{TargetPath: concretePath, Source: "override", Key: string(ov.Value.Source) + ":" + ov.Value.Key})
			} else if !ov.Value.SkipWhenMissing {
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: override source has no value", labelFor(concretePath)))
				return nil
			}
		}
		// Automatic additional-mapper matching applies only to root fields.
		if !selected && fullPath != "" && !strings.Contains(fullPath, ".") && fullPath == concretePath {
			if value, exists := ctx.mappers[fullPath]; exists {
				sub, subFound = value, true
				ctx.mappings = append(ctx.mappings, models.ResponseFieldMappingTrace{TargetPath: concretePath, Source: "mapper", Key: fullPath})
			} else {
				ctx.mappings = append(ctx.mappings, models.ResponseFieldMappingTrace{TargetPath: concretePath, Source: "document", Key: fullPath})
			}
		}
	}
	if subFound && sub == nil {
		return nil
	}
	switch t := template.(type) {
	case map[string]any:
		subMap, ok := sub.(map[string]any)
		if subFound && !ok {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: expected an object but found %T", labelFor(concretePath), sub))
		}
		if len(t) == 0 && len(subMap) > 0 {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: no object fields defined by response schema/example; omitted source fields", labelFor(concretePath)))
		}
		result := make(map[string]any, len(t))
		for k, childTemplate := range t {
			child, found := subMap[k]
			result[k] = ctx.fillNode(childTemplate, child, found, joinPath(fullPath, k), joinPath(concretePath, k), true)
		}
		return result
	case []any:
		arr, ok := sub.([]any)
		if !ok {
			if subFound {
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: expected an array but found %T", labelFor(concretePath), sub))
			}
			return []any{}
		}
		if len(t) == 0 {
			if len(arr) > 0 {
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: no array item shape defined by response schema/example; omitted source items", labelFor(concretePath)))
			}
			return []any{}
		}
		result := make([]any, 0, len(arr))
		for i, elem := range arr {
			// Do not reapply the parent array override to each individual item.
			result = append(result, ctx.fillNode(t[0], elem, true, fullPath, joinPath(concretePath, strconv.Itoa(i)), false))
		}
		return result
	default:
		if subFound {
			switch sub.(type) {
			case map[string]any, []any:
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: object/array source has no matching response shape; rendered as null", labelFor(concretePath)))
				return nil
			}
			return sub
		}
		if ctx.fallbackToExample {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: no document value; using template example", labelFor(concretePath)))
			return template
		}
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: no document value; rendered as null", labelFor(concretePath)))
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
		if err == nil && !found && ov.Value.SkipWhenMissing {
			continue
		}
		if err != nil || !found {
			warnings = append(warnings, fmt.Sprintf("%s: override could not be resolved", labelFor(path)))
			continue
		}
		switch v.(type) {
		case map[string]any, []any:
			warnings = append(warnings, fmt.Sprintf("%s: object/array mapping requires a response schema/example; override skipped", labelFor(path)))
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
