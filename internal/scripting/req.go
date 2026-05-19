package scripting

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"go.starlark.net/starlark"
)

// reqValue is the Starlark object passed to run(req).
// Each attribute is a callable that reads from the corresponding request section.
//
// Usage in scripts:
//
//	req.path("id")                   → string or error if missing
//	req.path("id", "")               → string or default if missing
//	req.query("page", "1")           → string or default
//	req.header("authorization", "")  → string or default (key auto-lowercased)
//	req.body()                       → whole body as dict/list/None
//	req.body("user.name", "")        → gjson-style nested path with default
type reqValue struct {
	input *ScriptInput
}

var (
	_ starlark.Value    = (*reqValue)(nil)
	_ starlark.HasAttrs = (*reqValue)(nil)
)

func (r *reqValue) String() string        { return "<req>" }
func (r *reqValue) Type() string          { return "req" }
func (r *reqValue) Freeze()               {}
func (r *reqValue) Truth() starlark.Bool  { return true }
func (r *reqValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: req") }

func (r *reqValue) AttrNames() []string {
	return []string{"path", "query", "header", "body"}
}

func (r *reqValue) Attr(name string) (starlark.Value, error) {
	switch name {
	case "path":
		return starlark.NewBuiltin("path", r.makeLookup("path", r.input.Path)), nil
	case "query":
		return starlark.NewBuiltin("query", r.makeLookup("query", r.input.Query)), nil
	case "header":
		return starlark.NewBuiltin("header", r.makeHeaderLookup()), nil
	case "body":
		return starlark.NewBuiltin("body", r.makeBodyLookup()), nil
	}
	return nil, nil
}

// makeLookup returns a builtin that looks up key in m.
//
//	fn(key)           → value or error if missing
//	fn(key, default)  → value or default if missing
func (r *reqValue) makeLookup(section string, m map[string]string) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var key starlark.String
		var def starlark.Value
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &key, &def); err != nil {
			return nil, err
		}
		k := string(key)
		if v, ok := m[k]; ok {
			return starlark.String(v), nil
		}
		if def != nil {
			return def, nil
		}
		return nil, fmt.Errorf("req.%s: key %q not found (use a default: req.%s(%q, \"\"))", section, k, section, k)
	}
}

// makeHeaderLookup is like makeLookup but auto-lowercases the key.
func (r *reqValue) makeHeaderLookup() func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var key starlark.String
		var def starlark.Value
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &key, &def); err != nil {
			return nil, err
		}
		k := strings.ToLower(string(key))
		if v, ok := r.input.Header[k]; ok {
			return starlark.String(v), nil
		}
		if def != nil {
			return def, nil
		}
		return nil, fmt.Errorf("req.header: key %q not found (use a default: req.header(%q, \"\"))", k, k)
	}
}

// makeBodyLookup returns a builtin for body access.
//
//	body()                  → whole body (dict/list/None)
//	body("field")           → gjson path, None if missing
//	body("field", default)  → gjson path with explicit default
func (r *reqValue) makeBodyLookup() func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	// Pre-compute JSON once for path navigation
	var bodyJSON string
	if r.input.Body != nil {
		if b, err := json.Marshal(r.input.Body); err == nil {
			bodyJSON = string(b)
		}
	}

	return func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// No arguments → return whole body
		if len(args) == 0 && len(kwargs) == 0 {
			return GoToStar(r.input.Body), nil
		}

		var path starlark.String
		var def starlark.Value
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &path, &def); err != nil {
			return nil, err
		}

		if bodyJSON == "" {
			if def != nil {
				return def, nil
			}
			return starlark.None, nil
		}

		res := gjson.Get(bodyJSON, string(path))
		if !res.Exists() {
			if def != nil {
				return def, nil
			}
			return starlark.None, nil
		}
		return GoToStar(res.Value()), nil
	}
}

// buildReqValue constructs the reqValue passed to run(req).
func buildReqValue(input *ScriptInput) *reqValue {
	return &reqValue{input: input}
}
