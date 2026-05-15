package scripting

import (
	"fmt"
	"regexp"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/prasenjit/go-virtual/internal/condition"
)

// newValidateModule returns the "validate" module exposing pattern-matching
// helpers backed by the condition token registry.
//
//   validate.matches(value, "uuid")       → bool   token or raw regex
//   validate.regex(value, pattern)        → bool   raw regex (shorthand)
//   validate.pattern_names()              → list   all known token names
//   validate.is_uuid(value)               → bool   (and is_email, is_url, …)
func newValidateModule() *starlarkstruct.Module {
	members := starlark.StringDict{
		"matches":       starlark.NewBuiltin("matches", validateMatches),
		"regex":         starlark.NewBuiltin("regex", validateRegex),
		"pattern_names": starlark.NewBuiltin("pattern_names", validatePatternNames),
	}

	// Add is_<token> convenience helpers for every registered token.
	for _, entry := range condition.PatternCatalogue() {
		token := entry.Token
		pattern := entry.Pattern
		re := regexp.MustCompile(pattern)
		name := "is_" + slugToIdent(token)
		members[name] = starlark.NewBuiltin(name, func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var s string
			if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &s); err != nil {
				return nil, err
			}
			return starlark.Bool(re.MatchString(s)), nil
		})
	}

	return &starlarkstruct.Module{
		Name:    "validate",
		Members: members,
	}
}

// validateMatches implements validate.matches(value, token_or_pattern).
// If token_or_pattern is a registered token name the corresponding pattern is
// used; otherwise it is treated as a raw regular expression.
func validateMatches(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value, pattern string
	if err := starlark.UnpackPositionalArgs("matches", args, kwargs, 2, &value, &pattern); err != nil {
		return nil, err
	}
	expanded := condition.Expand(pattern) // expands token names; returns unchanged for raw regex
	re, err := regexp.Compile(expanded)
	if err != nil {
		return nil, fmt.Errorf("validate.matches: invalid pattern %q: %w", expanded, err)
	}
	return starlark.Bool(re.MatchString(value)), nil
}

// validateRegex implements validate.regex(value, pattern) — raw regex, no token expansion.
func validateRegex(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value, pattern string
	if err := starlark.UnpackPositionalArgs("regex", args, kwargs, 2, &value, &pattern); err != nil {
		return nil, err
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("validate.regex: invalid pattern %q: %w", pattern, err)
	}
	return starlark.Bool(re.MatchString(value)), nil
}

// validatePatternNames implements validate.pattern_names() → list of token names.
func validatePatternNames(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs("pattern_names", args, kwargs, 0); err != nil {
		return nil, err
	}
	entries := condition.PatternCatalogue()
	elems := make([]starlark.Value, len(entries))
	for i, e := range entries {
		elems[i] = starlark.String(e.Token)
	}
	return starlark.NewList(elems), nil
}

// slugToIdent converts a hyphenated token name like "us-phone" to a valid
// Starlark identifier "us_phone".
func slugToIdent(s string) string {
	out := make([]byte, len(s))
	for i := range len(s) {
		if s[i] == '-' {
			out[i] = '_'
		} else {
			out[i] = s[i]
		}
	}
	return string(out)
}
