package collection

import (
	"fmt"
	"net/http"

	"github.com/tidwall/gjson"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/store"
)

// RequestContext carries the request data needed to resolve FieldMappingRules.
// It mirrors scripting.Input but is decoupled from that package.
type RequestContext struct {
	PathParams  map[string]string
	QueryParams map[string][]string
	Headers     http.Header
	Body        string // raw request body
	Session     store.SessionState
	GlobalStore store.GlobalStoreBackend
}

// Resolve returns the concrete string value for a FieldMappingRule given the
// current request context. Returns "" when a source cannot be resolved.
func Resolve(rule models.FieldMappingRule, req *RequestContext) string {
	if req == nil {
		return rule.SourceKey
	}
	switch rule.SourceType {
	case "path":
		if req.PathParams != nil {
			return req.PathParams[rule.SourceKey]
		}
	case "query":
		if req.QueryParams != nil {
			vals := req.QueryParams[rule.SourceKey]
			if len(vals) > 0 {
				return vals[0]
			}
		}
	case "header":
		if req.Headers != nil {
			return req.Headers.Get(rule.SourceKey)
		}
	case "body":
		if req.Body != "" {
			if rule.SourceKey == "" {
				return req.Body
			}
			res := gjson.Get(req.Body, rule.SourceKey)
			if res.Exists() {
				return res.String()
			}
		}
	case "session":
		if req.Session != nil {
			v, ok := req.Session.Get(rule.SourceKey)
			if ok && v != nil {
				return stringify(v)
			}
		}
	case "store":
		if req.GlobalStore != nil {
			v, ok := req.GlobalStore.Get(rule.SourceKey)
			if ok && v != nil {
				return stringify(v)
			}
		}
	case "literal":
		return rule.SourceKey
	}
	return ""
}

// ResolveMap builds a map[string]any from a slice of rules.
func ResolveMap(rules []models.FieldMappingRule, req *RequestContext) map[string]any {
	m := make(map[string]any, len(rules))
	for _, rule := range rules {
		m[rule.TargetField] = Resolve(rule, req)
	}
	return m
}

func stringify(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
