package template

import texttmpl "text/template"

// buildIterFuncMap returns template functions for iteration helpers.
func buildIterFuncMap() texttmpl.FuncMap {
	return texttmpl.FuncMap{
		// times returns []int{0, 1, ..., n-1} — use with {{range $i, $_ := times 5}}.
		"times": func(n int) []int {
			if n <= 0 {
				return []int{}
			}
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		// seq returns []int{start, start+1, ..., end} inclusive.
		"seq": func(start, end int) []int {
			if end < start {
				return []int{}
			}
			s := make([]int, end-start+1)
			for i := range s {
				s[i] = start + i
			}
			return s
		},
		// list creates a slice from the provided values.
		"list": func(vals ...interface{}) []interface{} {
			return vals
		},
		// first returns the first element of a slice or empty string.
		"first": func(s []interface{}) interface{} {
			if len(s) == 0 {
				return ""
			}
			return s[0]
		},
		// last returns the last element of a slice or empty string.
		"last": func(s []interface{}) interface{} {
			if len(s) == 0 {
				return ""
			}
			return s[len(s)-1]
		},
		// len returns the length of a slice.
		"len": func(s []interface{}) int {
			return len(s)
		},
	}
}
