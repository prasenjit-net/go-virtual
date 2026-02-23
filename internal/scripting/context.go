package scripting

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ScriptInput holds the request data passed to each Starlark script as the `req` dict.
type ScriptInput struct {
	Path   map[string]string
	Query  map[string]string
	Header map[string]string
	Body   any // map[string]any if JSON, raw string otherwise
}

// BuildInput constructs a ScriptInput from an incoming request and the already-read body string.
func BuildInput(pathParams map[string]string, r *http.Request, body string) *ScriptInput {
	input := &ScriptInput{
		Path:   make(map[string]string),
		Query:  make(map[string]string),
		Header: make(map[string]string),
	}

	// Path parameters
	for k, v := range pathParams {
		input.Path[k] = v
	}

	// Query parameters – first value per key
	for k, vals := range r.URL.Query() {
		if len(vals) > 0 {
			input.Query[k] = vals[0]
		}
	}

	// Headers – lower-cased keys, first value per key
	for k, vals := range r.Header {
		if len(vals) > 0 {
			input.Header[strings.ToLower(k)] = vals[0]
		}
	}

	// Body – try to parse as JSON; fall back to raw string
	if body != "" {
		var parsed any
		if err := json.Unmarshal([]byte(body), &parsed); err == nil {
			input.Body = parsed
		} else {
			input.Body = body
		}
	} else {
		input.Body = nil
	}

	return input
}
