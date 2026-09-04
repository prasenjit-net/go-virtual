package collection

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/prasenjit/go-virtual/internal/models"
)

// TypedRequestContext carries the request data a typed ValueBinding may read
// from. It is the typed counterpart of RequestContext: body/literal values
// preserve their native JSON type instead of being coerced to strings.
type TypedRequestContext struct {
	PathParams  map[string]string
	QueryParams map[string][]string
	Headers     http.Header
	Body        string
}

// BindingContext supplies every data source a ValueBinding may reference
// beyond the request itself. Callers set only the fields relevant to the
// binding sites they resolve — e.g. filter resolution never needs Document
// or Mappers.
type BindingContext struct {
	Request *TypedRequestContext
	// Primary is the primary mapper's result document. Used by
	// ValueSourcePrimary (valid only in additional-mapper filters).
	Primary map[string]any
	// Document is the "current" result document during projection fill.
	// Used by ValueSourceDocument (valid only in field overrides).
	Document map[string]any
	// Mappers holds named additional-mapper results keyed by OutputKey.
	// Used by ValueSourceMapper ("<outputKey>.<path>", valid only in field
	// overrides).
	Mappers map[string]any
}

// ResolveValueBinding resolves one ValueBinding to a native JSON-compatible
// value (string, float64, bool, nil, map[string]any, or []any).
//
// found is false when the referenced source has no value for this request —
// this is distinguishable from an explicit JSON null, which resolves to
// (nil, true, nil). err is non-nil only for a malformed binding, such as
// invalid literal JSON.
func ResolveValueBinding(b models.ValueBinding, ctx *BindingContext) (any, bool, error) {
	switch b.Source {
	case models.ValueSourceLiteral:
		if len(b.Value) == 0 {
			return nil, false, nil
		}
		var v any
		if err := json.Unmarshal(b.Value, &v); err != nil {
			return nil, false, fmt.Errorf("invalid literal JSON: %w", err)
		}
		return v, true, nil

	case models.ValueSourcePath:
		if ctx == nil || ctx.Request == nil || ctx.Request.PathParams == nil {
			return nil, false, nil
		}
		v, ok := ctx.Request.PathParams[b.Key]
		if !ok {
			return nil, false, nil
		}
		return v, true, nil

	case models.ValueSourceQuery:
		if ctx == nil || ctx.Request == nil || ctx.Request.QueryParams == nil {
			return nil, false, nil
		}
		vals, ok := ctx.Request.QueryParams[b.Key]
		if !ok || len(vals) == 0 {
			return nil, false, nil
		}
		return vals[0], true, nil

	case models.ValueSourceHeader:
		if ctx == nil || ctx.Request == nil || ctx.Request.Headers == nil {
			return nil, false, nil
		}
		v := ctx.Request.Headers.Get(b.Key)
		if v == "" {
			return nil, false, nil
		}
		return v, true, nil

	case models.ValueSourceBody:
		if ctx == nil || ctx.Request == nil || ctx.Request.Body == "" {
			return nil, false, nil
		}
		res := gjson.Get(ctx.Request.Body, b.Key)
		if !res.Exists() {
			return nil, false, nil
		}
		return res.Value(), true, nil

	case models.ValueSourcePrimary:
		if ctx == nil || ctx.Primary == nil {
			return nil, false, nil
		}
		v, ok := GetPath(ctx.Primary, b.Key)
		return v, ok, nil

	case models.ValueSourceDocument:
		if ctx == nil || ctx.Document == nil {
			return nil, false, nil
		}
		v, ok := GetPath(ctx.Document, b.Key)
		return v, ok, nil

	case models.ValueSourceMapper:
		if ctx == nil || ctx.Mappers == nil {
			return nil, false, nil
		}
		outputKey, path := splitMapperKey(b.Key)
		result, ok := ctx.Mappers[outputKey]
		if !ok {
			return nil, false, nil
		}
		if path == "" {
			return result, true, nil
		}
		v, ok := GetPath(result, path)
		return v, ok, nil

	default:
		return nil, false, nil
	}
}

// ResolveFilterMap resolves a slice of CollectionFilter into a filter map
// suitable for Ops.FindOne/FindMany. A filter whose value cannot be resolved
// contributes an explicit nil (matches documents where the field is
// present-and-null, same as every other collection-mapping filter).
func ResolveFilterMap(filters []models.CollectionFilter, ctx *BindingContext) (map[string]any, error) {
	out := make(map[string]any, len(filters))
	for _, f := range filters {
		v, _, err := ResolveValueBinding(f.Value, ctx)
		if err != nil {
			return nil, fmt.Errorf("filter %q: %w", f.TargetPath, err)
		}
		out[f.TargetPath] = v
	}
	return out, nil
}

func splitMapperKey(key string) (outputKey, path string) {
	idx := strings.Index(key, ".")
	if idx < 0 {
		return key, ""
	}
	return key[:idx], key[idx+1:]
}

// GetPath reads a dot-separated path out of a value tree built from
// map[string]any / []any (as produced by collection documents or
// encoding/json unmarshalling into any). Numeric segments index into arrays.
//
// found is false when the path cannot be resolved because an intermediate
// segment is missing or not a container. A path that resolves to an explicit
// JSON null returns (nil, true) — the null was found, it just has no value.
func GetPath(doc any, path string) (any, bool) {
	if path == "" {
		return doc, true
	}
	cur := doc
	for _, seg := range strings.Split(path, ".") {
		if cur == nil {
			return nil, false
		}
		switch v := cur.(type) {
		case map[string]any:
			val, ok := v[seg]
			if !ok {
				return nil, false
			}
			cur = val
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			cur = v[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}
