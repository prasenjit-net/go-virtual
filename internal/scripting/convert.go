package scripting

import (
	"fmt"

	"go.starlark.net/starlark"
)

// StarToGo recursively converts a Starlark value to a native Go value.
// The returned types are: nil, bool, int64, float64, string, []any, map[string]any.
func StarToGo(v starlark.Value) any {
	if v == nil || v == starlark.None {
		return nil
	}

	switch val := v.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(val)
	case starlark.Int:
		// Try int64 first; fall back to big.Int string
		if i, ok := val.Int64(); ok {
			return i
		}
		return val.String()
	case starlark.Float:
		return float64(val)
	case starlark.String:
		return string(val)
	case *starlark.List:
		result := make([]any, val.Len())
		for i := 0; i < val.Len(); i++ {
			result[i] = StarToGo(val.Index(i))
		}
		return result
	case starlark.Tuple:
		result := make([]any, val.Len())
		for i := 0; i < val.Len(); i++ {
			result[i] = StarToGo(val.Index(i))
		}
		return result
	case *starlark.Dict:
		result := make(map[string]any, val.Len())
		for _, item := range val.Items() {
			key := item[0]
			value := item[1]
			// Keys are stringified
			result[fmt.Sprintf("%s", starKeyToString(key))] = StarToGo(value)
		}
		return result
	default:
		// Fallback: use Starlark's string representation
		return v.String()
	}
}

// starKeyToString converts a starlark key to a Go string for use in maps.
func starKeyToString(v starlark.Value) string {
	if s, ok := v.(starlark.String); ok {
		return string(s)
	}
	return v.String()
}

// GoToStar converts a native Go value to the corresponding Starlark value.
// Supports: nil, bool, int/int64, float64, string, []any, map[string]any.
func GoToStar(v any) starlark.Value {
	if v == nil {
		return starlark.None
	}

	switch val := v.(type) {
	case bool:
		return starlark.Bool(val)
	case int:
		return starlark.MakeInt(val)
	case int32:
		return starlark.MakeInt(int(val))
	case int64:
		return starlark.MakeInt64(val)
	case float32:
		return starlark.Float(float64(val))
	case float64:
		return starlark.Float(val)
	case string:
		return starlark.String(val)
	case []any:
		elems := make([]starlark.Value, len(val))
		for i, item := range val {
			elems[i] = GoToStar(item)
		}
		return starlark.NewList(elems)
	case map[string]any:
		d := new(starlark.Dict)
		for k, item := range val {
			_ = d.SetKey(starlark.String(k), GoToStar(item))
		}
		return d
	default:
		return starlark.String(fmt.Sprintf("%v", val))
	}
}
