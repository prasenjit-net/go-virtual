package template

import (
	"crypto/md5"  //nolint:gosec
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	texttmpl "text/template"
)

// buildStringFuncMap returns template functions for string transformations.
func buildStringFuncMap() texttmpl.FuncMap {
	return texttmpl.FuncMap{
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"trim":       strings.TrimSpace,
		"trimPrefix": func(prefix, s string) string { return strings.TrimPrefix(s, prefix) },
		"trimSuffix": func(suffix, s string) string { return strings.TrimSuffix(s, suffix) },
		"replace": func(old, new, s string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"truncate": func(n int, s string) string {
			runes := []rune(s)
			if n < 0 || n >= len(runes) {
				return s
			}
			return string(runes[:n])
		},
		"default": func(fallback, val string) string {
			if val == "" {
				return fallback
			}
			return val
		},
		"coalesce": func(vals ...string) string {
			for _, v := range vals {
				if v != "" {
					return v
				}
			}
			return ""
		},
		// contains returns true if s contains substr.
		"contains": func(substr, s string) bool { return strings.Contains(s, substr) },
		// hasPrefix returns true if s has the given prefix.
		"hasPrefix": func(prefix, s string) bool { return strings.HasPrefix(s, prefix) },
		// hasSuffix returns true if s has the given suffix.
		"hasSuffix": func(suffix, s string) bool { return strings.HasSuffix(s, suffix) },
		"split": func(sep, s string) []string {
			return strings.Split(s, sep)
		},
		"join": func(sep string, parts []string) string {
			return strings.Join(parts, sep)
		},
		"b64enc": func(s string) string {
			return base64.StdEncoding.EncodeToString([]byte(s))
		},
		"b64dec": func(s string) string {
			decoded, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return ""
			}
			return string(decoded)
		},
		"urlEnc": func(s string) string {
			return url.QueryEscape(s)
		},
		"urlDec": func(s string) string {
			decoded, err := url.QueryUnescape(s)
			if err != nil {
				return s
			}
			return decoded
		},
		"md5": func(s string) string {
			h := md5.Sum([]byte(s)) //nolint:gosec
			return hex.EncodeToString(h[:])
		},
		"sha256": func(s string) string {
			h := sha256.Sum256([]byte(s))
			return hex.EncodeToString(h[:])
		},
		"repeat": func(n int, s string) string {
			return strings.Repeat(s, n)
		},
		"indent": func(n int, s string) string {
			pad := strings.Repeat(" ", n)
			return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
		},
		"printf": fmt.Sprintf,
	}
}
