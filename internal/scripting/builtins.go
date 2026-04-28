package scripting

import (
	"crypto/md5"  //nolint:gosec
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"math/rand"
	"reflect"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.starlark.net/starlark"

	"github.com/prasenjit/go-virtual/internal/store"
)

// regexpCache caches compiled regular expressions to avoid re-compiling on every call.
var (
	regexpCacheMu sync.Mutex
	regexpCache   = map[string]*regexp.Regexp{}
)

func getCachedRegexp(pattern string) (*regexp.Regexp, error) {
	regexpCacheMu.Lock()
	defer regexpCacheMu.Unlock()
	if re, ok := regexpCache[pattern]; ok {
		return re, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexpCache[pattern] = re
	return re, nil
}

// injectBuiltins adds all standard builtins to the predeclared dict.
// sess is used by the counter() builtin (nil → local fallback map).
// rng is a per-execution random source.
func injectBuiltins(predeclared starlark.StringDict, rng *rand.Rand, sess store.SessionState) {
	// ── uuid() ──────────────────────────────────────────────────────────────
	// uuid() → string   Generate a random UUID v4.
	predeclared["uuid"] = starlark.NewBuiltin("uuid", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if err := starlark.UnpackPositionalArgs("uuid", args, kwargs, 0); err != nil {
			return nil, err
		}
		return starlark.String(uuid.New().String()), nil
	})

	// ── now(format?) ────────────────────────────────────────────────────────
	// now()            → Unix timestamp (int, seconds)
	// now("unix_ms")   → Unix timestamp in milliseconds
	// now("iso")       → "2006-01-02T15:04:05Z"
	// now("date")      → "2006-01-02"
	predeclared["now"] = starlark.NewBuiltin("now", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		format := ""
		if err := starlark.UnpackPositionalArgs("now", args, kwargs, 0, &format); err != nil {
			return nil, err
		}
		t := time.Now().UTC()
		switch format {
		case "", "unix":
			return starlark.MakeInt64(t.Unix()), nil
		case "unix_ms":
			return starlark.MakeInt64(t.UnixMilli()), nil
		case "iso":
			return starlark.String(t.Format(time.RFC3339)), nil
		case "date":
			return starlark.String(t.Format("2006-01-02")), nil
		default:
			return nil, fmt.Errorf("now: unknown format %q; valid: unix, unix_ms, iso, date", format)
		}
	})

	// ── rand_int(min, max) ──────────────────────────────────────────────────
	// rand_int(100)       → random int in [0, 100]
	// rand_int(1, 100)    → random int in [1, 100] inclusive
	predeclared["rand_int"] = starlark.NewBuiltin("rand_int", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var a, b starlark.Value
		if err := starlark.UnpackPositionalArgs("rand_int", args, kwargs, 1, &a, &b); err != nil {
			return nil, err
		}
		aInt, ok := a.(starlark.Int)
		if !ok {
			return nil, fmt.Errorf("rand_int: argument must be int, got %s", a.Type())
		}
		lo, _ := aInt.Int64()
		var hi int64
		if b == nil {
			hi = lo
			lo = 0
		} else {
			bInt, ok := b.(starlark.Int)
			if !ok {
				return nil, fmt.Errorf("rand_int: argument must be int, got %s", b.Type())
			}
			hi, _ = bInt.Int64()
		}
		if hi < lo {
			return nil, fmt.Errorf("rand_int: max (%d) must be >= min (%d)", hi, lo)
		}
		v := lo + rng.Int63n(hi-lo+1)
		return starlark.MakeInt64(v), nil
	})

	// ── rand_choice(list) ───────────────────────────────────────────────────
	// rand_choice(["a", "b", "c"]) → one element picked at random
	predeclared["rand_choice"] = starlark.NewBuiltin("rand_choice", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var lst *starlark.List
		if err := starlark.UnpackPositionalArgs("rand_choice", args, kwargs, 1, &lst); err != nil {
			return nil, err
		}
		if lst.Len() == 0 {
			return nil, fmt.Errorf("rand_choice: list is empty")
		}
		idx := rng.Intn(lst.Len())
		return lst.Index(idx), nil
	})

	// ── counter(name, delta?) ────────────────────────────────────────────────
	// counter("hits")       → increments by 1, returns new value
	// counter("hits", 5)    → increments by 5, returns new value
	// counter("hits", 0)    → returns current value without mutating
	// Backed by session store under key "__counter__:<name>".
	// When sess is nil (no session), falls back to an in-memory map (ephemeral).
	localCounters := map[string]int64{}
	predeclared["counter"] = starlark.NewBuiltin("counter", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name string
		var deltaVal starlark.Value
		if err := starlark.UnpackPositionalArgs("counter", args, kwargs, 1, &name, &deltaVal); err != nil {
			return nil, err
		}
		delta := int64(1)
		if deltaVal != nil {
			di, ok := deltaVal.(starlark.Int)
			if !ok {
				return nil, fmt.Errorf("counter: delta must be int, got %s", deltaVal.Type())
			}
			d, _ := di.Int64()
			delta = d
		}
		key := "__counter__:" + name
		var cur int64
		if hasSession(sess) {
			existing, ok := sess.Get(key)
			if ok && existing != nil {
				switch v := existing.(type) {
				case int64:
					cur = v
				case float64:
					cur = int64(v)
				case int:
					cur = int64(v)
				}
			}
			cur += delta
			if err := sess.Set(key, cur); err != nil {
				return nil, err
			}
		} else {
			cur = localCounters[key] + delta
			localCounters[key] = cur
		}
		return starlark.MakeInt64(cur), nil
	})

	// ── base64_encode(str) ──────────────────────────────────────────────────
	predeclared["base64_encode"] = starlark.NewBuiltin("base64_encode", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var s string
		if err := starlark.UnpackPositionalArgs("base64_encode", args, kwargs, 1, &s); err != nil {
			return nil, err
		}
		return starlark.String(base64.StdEncoding.EncodeToString([]byte(s))), nil
	})

	// ── base64_decode(str) ──────────────────────────────────────────────────
	predeclared["base64_decode"] = starlark.NewBuiltin("base64_decode", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var s string
		if err := starlark.UnpackPositionalArgs("base64_decode", args, kwargs, 1, &s); err != nil {
			return nil, err
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("base64_decode: %w", err)
		}
		return starlark.String(b), nil
	})

	// ── hash(algo, str) ─────────────────────────────────────────────────────
	// hash("sha256", "input") → hex-encoded digest string
	// Supported: md5, sha1, sha256, sha512
	predeclared["hash"] = starlark.NewBuiltin("hash", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var algo, input string
		if err := starlark.UnpackPositionalArgs("hash", args, kwargs, 2, &algo, &input); err != nil {
			return nil, err
		}
		var h hash.Hash
		switch algo {
		case "md5":
			h = md5.New() //nolint:gosec
		case "sha1":
			h = sha1.New() //nolint:gosec
		case "sha256":
			h = sha256.New()
		case "sha512":
			h = sha512.New()
		default:
			return nil, fmt.Errorf("hash: unknown algorithm %q; valid: md5, sha1, sha256, sha512", algo)
		}
		h.Write([]byte(input))
		return starlark.String(hex.EncodeToString(h.Sum(nil))), nil
	})

	// ── sleep(ms) ───────────────────────────────────────────────────────────
	// sleep(50) — pause for up to 50 ms.
	// The sleep is broken into small ticks so the Starlark step-counter can
	// cancel it if the overall script timeout fires.
	predeclared["sleep"] = starlark.NewBuiltin("sleep", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var ms starlark.Int
		if err := starlark.UnpackPositionalArgs("sleep", args, kwargs, 1, &ms); err != nil {
			return nil, err
		}
		msVal, _ := ms.Int64()
		if msVal > 0 {
			time.Sleep(time.Duration(msVal) * time.Millisecond)
		}
		return starlark.None, nil
	})

	// ── json_parse(str) ─────────────────────────────────────────────────────
	predeclared["json_parse"] = starlark.NewBuiltin("json_parse", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var s string
		if err := starlark.UnpackPositionalArgs("json_parse", args, kwargs, 1, &s); err != nil {
			return nil, err
		}
		var v any
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return nil, fmt.Errorf("json_parse: %w", err)
		}
		return GoToStar(v), nil
	})

	// ── json_stringify(val) ─────────────────────────────────────────────────
	predeclared["json_stringify"] = starlark.NewBuiltin("json_stringify", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var val starlark.Value
		if err := starlark.UnpackPositionalArgs("json_stringify", args, kwargs, 1, &val); err != nil {
			return nil, err
		}
		b, err := json.Marshal(StarToGo(val))
		if err != nil {
			return nil, fmt.Errorf("json_stringify: %w", err)
		}
		return starlark.String(b), nil
	})

	// ── regex_match(pattern, str) ────────────────────────────────────────────
	// Returns True if the pattern matches anywhere in str.
	predeclared["regex_match"] = starlark.NewBuiltin("regex_match", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var pattern, s string
		if err := starlark.UnpackPositionalArgs("regex_match", args, kwargs, 2, &pattern, &s); err != nil {
			return nil, err
		}
		re, err := getCachedRegexp(pattern)
		if err != nil {
			return nil, fmt.Errorf("regex_match: %w", err)
		}
		return starlark.Bool(re.MatchString(s)), nil
	})

	// ── regex_find(pattern, str) ─────────────────────────────────────────────
	// Returns the first match string, or None if no match.
	predeclared["regex_find"] = starlark.NewBuiltin("regex_find", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var pattern, s string
		if err := starlark.UnpackPositionalArgs("regex_find", args, kwargs, 2, &pattern, &s); err != nil {
			return nil, err
		}
		re, err := getCachedRegexp(pattern)
		if err != nil {
			return nil, fmt.Errorf("regex_find: %w", err)
		}
		match := re.FindString(s)
		if match == "" && !re.MatchString(s) {
			return starlark.None, nil
		}
		return starlark.String(match), nil
	})

	// ── regex_find_all(pattern, str) ─────────────────────────────────────────
	// Returns a list of all non-overlapping matches (empty list if none).
	predeclared["regex_find_all"] = starlark.NewBuiltin("regex_find_all", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var pattern, s string
		if err := starlark.UnpackPositionalArgs("regex_find_all", args, kwargs, 2, &pattern, &s); err != nil {
			return nil, err
		}
		re, err := getCachedRegexp(pattern)
		if err != nil {
			return nil, fmt.Errorf("regex_find_all: %w", err)
		}
		matches := re.FindAllString(s, -1)
		elems := make([]starlark.Value, len(matches))
		for i, m := range matches {
			elems[i] = starlark.String(m)
		}
		return starlark.NewList(elems), nil
	})
}

func hasSession(sess store.SessionState) bool {
	if sess == nil {
		return false
	}
	v := reflect.ValueOf(sess)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !v.IsNil()
	default:
		return true
	}
}

// builtinNames lists all names that injectBuiltins injects, used by Compile's
// predeclared predicate so the compiler does not flag them as undefined.
var builtinNames = map[string]bool{
	"uuid":           true,
	"now":            true,
	"rand_int":       true,
	"rand_choice":    true,
	"counter":        true,
	"base64_encode":  true,
	"base64_decode":  true,
	"hash":           true,
	"sleep":          true,
	"json_parse":     true,
	"json_stringify": true,
	"regex_match":    true,
	"regex_find":     true,
	"regex_find_all": true,
}
