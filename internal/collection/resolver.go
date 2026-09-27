package collection

import (
	"fmt"
	"net/http"
	"strings"

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
// current request context. An absent source uses its configured default, or "".
func Resolve(rule models.FieldMappingRule, req *RequestContext) string {
	// Preserve the historical nil-request behavior when no default is configured.
	if req == nil && rule.DefaultValue == nil {
		return rule.SourceKey
	}
	value, found := resolveSource(rule, req)
	if !found && rule.DefaultValue != nil {
		return *rule.DefaultValue
	}
	return value
}

func resolveSource(rule models.FieldMappingRule, req *RequestContext) (string, bool) {
	if rule.SourceType == "literal" {
		return rule.SourceKey, true
	}
	if req == nil {
		return "", false
	}
	switch rule.SourceType {
	case "path":
		value, found := req.PathParams[rule.SourceKey]
		return value, found
	case "query":
		values := req.QueryParams[rule.SourceKey]
		if len(values) > 0 {
			return values[0], true
		}
	case "header":
		return headerValue(req.Headers, rule.SourceKey)
	case "body":
		if req.Body != "" {
			if rule.SourceKey == "" {
				return req.Body, true
			}
			result := gjson.Get(req.Body, rule.SourceKey)
			if result.Exists() {
				return result.String(), true
			}
		}
	case "session":
		if req.Session != nil {
			value, found := req.Session.Get(rule.SourceKey)
			if found {
				if value == nil {
					return "", true
				}
				return stringify(value), true
			}
		}
	case "store":
		if req.GlobalStore != nil {
			value, found := req.GlobalStore.Get(rule.SourceKey)
			if found {
				if value == nil {
					return "", true
				}
				return stringify(value), true
			}
		}
	}
	return "", false
}

// headerValue distinguishes a present empty header from an absent value.
func headerValue(headers http.Header, key string) (string, bool) {
	if values, ok := headers[http.CanonicalHeaderKey(key)]; ok {
		if len(values) > 0 {
			return values[0], true
		}
		return "", false
	}
	for name, values := range headers {
		if strings.EqualFold(name, key) && len(values) > 0 {
			return values[0], true
		}
	}
	return "", false
}

// ResolveMap builds a map[string]any from a slice of rules.
func ResolveMap(rules []models.FieldMappingRule, req *RequestContext) map[string]any {
	m := make(map[string]any, len(rules))
	for _, rule := range rules {
		if rule.SkipWhenMissing {
			if _, found := resolveSource(rule, req); !found {
				continue
			}
		}
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
