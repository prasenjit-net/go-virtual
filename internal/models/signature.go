package models

import (
	"net/textproto"
	"sort"
	"strings"
)

const virtualControlHeaderPrefix = "x-virtual-"

// SignatureConfig defines what parts of a request are used to compute
// the request signature for an operation. Signatures are used in proxy
// recording mode to deduplicate responses.
type SignatureConfig struct {
	// PathParams lists which path parameters to include.
	// Empty slice means include ALL declared path parameters.
	PathParams []string `json:"pathParams"`

	// QueryParams lists which query parameters to include.
	// Empty slice means include ALL declared query parameters.
	QueryParams []string `json:"queryParams"`

	// HeadersConfigured distinguishes "use default headers" from an explicit
	// empty header list. When false, declared header params plus spec-scoped
	// signature headers are used as defaults.
	HeadersConfigured bool `json:"headersConfigured,omitempty"`

	// Headers lists specific request headers to include when HeadersConfigured
	// is true. An empty slice then means "include no headers".
	Headers []string `json:"headers"`

	// IncludeBody controls whether the request body is part of the signature.
	// Nil means use the operation default (include full body when request body
	// exists). False disables body inclusion entirely.
	IncludeBody *bool `json:"includeBody,omitempty"`

	// BodyJsonPaths lists specific gjson paths to extract from the body.
	// When empty and IncludeBody resolves to true, the entire raw body is used.
	BodyJsonPaths []string `json:"bodyJsonPaths"`
}

// Normalize sorts and deduplicates signature configuration fields while
// preserving override semantics.
func (c *SignatureConfig) Normalize() {
	if c == nil {
		return
	}
	c.PathParams = normalizeIdentifierList(c.PathParams)
	c.QueryParams = normalizeIdentifierList(c.QueryParams)
	c.Headers = NormalizeSignatureHeaderNames(c.Headers)
	c.BodyJsonPaths = normalizeIdentifierList(c.BodyJsonPaths)
	if c.IncludeBody != nil && !*c.IncludeBody {
		c.BodyJsonPaths = nil
	}
}

// NormalizeSignatureHeaderNames trims, canonicalizes, filters control headers,
// and deduplicates header names case-insensitively.
func NormalizeSignatureHeaderNames(headers []string) []string {
	if len(headers) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(headers))
	normalized := make([]string, 0, len(headers))
	for _, header := range headers {
		name := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(header))
		if name == "" || IsIgnoredSignatureHeader(name) {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, name)
	}
	sort.Strings(normalized)
	return normalized
}

// IsIgnoredSignatureHeader returns true for internal control headers that
// should never contribute to recorded-response signatures.
func IsIgnoredSignatureHeader(name string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(name)), virtualControlHeaderPrefix)
}

func normalizeIdentifierList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	sort.Strings(normalized)
	return normalized
}
