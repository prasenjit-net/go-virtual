package proxy

import (
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/tidwall/gjson"
)

// ComputeSignature generates a deterministic hash of the request that can be
// used to uniquely identify a request for the purpose of proxy recording.
//
// When cfg is nil the defaults are used:
//   - all path parameters
//   - all query parameters
//   - no request headers
//   - full request body
func ComputeSignature(
	pathParams map[string]string,
	queryParams url.Values,
	headers http.Header,
	body string,
	cfg *models.SignatureConfig,
) string {
	h := fnv.New64a()

	// Resolve effective config
	includeAllPath := true
	includeAllQuery := true
	includeBody := true
	var pathParamFilter []string
	var queryParamFilter []string
	var headerFilter []string
	var bodyJsonPaths []string

	if cfg != nil {
		if len(cfg.PathParams) > 0 {
			includeAllPath = false
			pathParamFilter = cfg.PathParams
		}
		if len(cfg.QueryParams) > 0 {
			includeAllQuery = false
			queryParamFilter = cfg.QueryParams
		}
		headerFilter = cfg.Headers
		if cfg.IncludeBody != nil {
			includeBody = *cfg.IncludeBody
		}
		bodyJsonPaths = cfg.BodyJsonPaths
	}

	// --- path params ---
	_, _ = io.WriteString(h, "path:")
	if includeAllPath {
		writeAllPathParams(h, pathParams)
	} else {
		keys := sortedCopy(pathParamFilter)
		for _, key := range keys {
			if val, ok := pathParams[key]; ok {
				_, _ = io.WriteString(h, key+"="+val+"&")
			}
		}
	}
	_, _ = io.WriteString(h, "|")

	// --- query params ---
	_, _ = io.WriteString(h, "query:")
	if includeAllQuery {
		writeAllQueryParams(h, queryParams)
	} else {
		keys := sortedCopy(queryParamFilter)
		for _, key := range keys {
			if vals, ok := queryParams[key]; ok {
				sortedVals := sortedCopy(vals)
				for _, val := range sortedVals {
					_, _ = io.WriteString(h, key+"="+val+"&")
				}
			}
		}
	}
	_, _ = io.WriteString(h, "|")

	// --- headers ---
	_, _ = io.WriteString(h, "headers:")
	headerKeys := sortedCopy(headerFilter)
	for _, key := range headerKeys {
		if models.IsIgnoredSignatureHeader(key) {
			continue
		}
		for k, vals := range headers {
			if models.IsIgnoredSignatureHeader(k) {
				continue
			}
			if strings.EqualFold(k, key) {
				sortedVals := sortedCopy(vals)
				for _, val := range sortedVals {
					_, _ = io.WriteString(h, strings.ToLower(key)+"="+val+"&")
				}
			}
		}
	}
	_, _ = io.WriteString(h, "|")

	// --- body ---
	_, _ = io.WriteString(h, "body:")
	if includeBody {
		if len(bodyJsonPaths) == 0 {
			_, _ = io.WriteString(h, body)
		} else {
			jpaths := sortedCopy(bodyJsonPaths)
			for _, jp := range jpaths {
				result := gjson.Get(body, jp)
				_, _ = io.WriteString(h, jp+"="+result.String()+"&")
			}
		}
	}

	return fmt.Sprintf("%016x", h.Sum64())
}

// ResolveSignatureConfig derives the effective runtime signature configuration
// for an operation by combining declared defaults from the spec/operation with
// any persisted operation override.
func ResolveSignatureConfig(spec *models.Spec, op *models.Operation) *models.SignatureConfig {
	if op == nil {
		return &models.SignatureConfig{}
	}
	specHeaders := []string(nil)
	if spec != nil {
		specHeaders = spec.SignatureHeaders
	}
	defaults := &models.SignatureConfig{
		PathParams:  append([]string{}, op.DeclaredPathParams...),
		QueryParams: append([]string{}, op.DeclaredQueryParams...),
		Headers:     models.NormalizeSignatureHeaderNames(append(append([]string{}, op.DeclaredHeaderParams...), specHeaders...)),
	}
	includeBody := op.HasRequestBody
	defaults.IncludeBody = &includeBody
	if op.SignatureConfig == nil {
		defaults.Normalize()
		return defaults
	}

	effective := &models.SignatureConfig{
		PathParams:  defaults.PathParams,
		QueryParams: defaults.QueryParams,
		Headers:     defaults.Headers,
		IncludeBody: defaults.IncludeBody,
	}
	if len(op.SignatureConfig.PathParams) > 0 {
		effective.PathParams = append([]string{}, op.SignatureConfig.PathParams...)
	}
	if len(op.SignatureConfig.QueryParams) > 0 {
		effective.QueryParams = append([]string{}, op.SignatureConfig.QueryParams...)
	}
	if op.SignatureConfig.HeadersConfigured {
		effective.Headers = append([]string{}, op.SignatureConfig.Headers...)
	}
	if op.SignatureConfig.IncludeBody != nil {
		includeBody := *op.SignatureConfig.IncludeBody
		effective.IncludeBody = &includeBody
	}
	if effective.IncludeBody != nil && *effective.IncludeBody && len(op.SignatureConfig.BodyJsonPaths) > 0 {
		effective.BodyJsonPaths = append([]string{}, op.SignatureConfig.BodyJsonPaths...)
	}
	effective.Normalize()
	return effective
}

// writeAllPathParams writes all path params sorted deterministically.
func writeAllPathParams(h io.Writer, params map[string]string) {
	if len(params) == 0 {
		return
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, _ = io.WriteString(h, k+"="+params[k]+"&")
	}
}

// writeAllQueryParams writes all query params sorted deterministically.
func writeAllQueryParams(h io.Writer, query url.Values) {
	if len(query) == 0 {
		return
	}
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vals := sortedCopy(query[k])
		for _, v := range vals {
			_, _ = io.WriteString(h, k+"="+v+"&")
		}
	}
}

// sortedCopy returns a sorted copy of the given slice.
func sortedCopy(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	c := make([]string, len(s))
	copy(c, s)
	sort.Strings(c)
	return c
}
