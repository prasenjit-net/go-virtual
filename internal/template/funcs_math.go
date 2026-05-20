package template

import (
	"fmt"
	"math"
	"strconv"
	texttmpl "text/template"
)

// toNumber coerces any numeric-ish value to float64.
func toNumber(v interface{}) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case float32:
		return float64(n)
	case float64:
		return n
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}

// buildMathFuncMap returns template functions for math operations.
func buildMathFuncMap() texttmpl.FuncMap {
	return texttmpl.FuncMap{
		"add": func(a, b interface{}) float64 {
			return toNumber(a) + toNumber(b)
		},
		"sub": func(a, b interface{}) float64 {
			return toNumber(a) - toNumber(b)
		},
		"mul": func(a, b interface{}) float64 {
			return toNumber(a) * toNumber(b)
		},
		"div": func(a, b interface{}) float64 {
			bv := toNumber(b)
			if bv == 0 {
				return 0
			}
			return toNumber(a) / bv
		},
		"mod": func(a, b interface{}) int {
			bv := int(toNumber(b))
			if bv == 0 {
				return 0
			}
			return int(toNumber(a)) % bv
		},
		"abs": func(a interface{}) float64 {
			return math.Abs(toNumber(a))
		},
		"max": func(a, b interface{}) float64 {
			av, bv := toNumber(a), toNumber(b)
			if av > bv {
				return av
			}
			return bv
		},
		"min": func(a, b interface{}) float64 {
			av, bv := toNumber(a), toNumber(b)
			if av < bv {
				return av
			}
			return bv
		},
		"ceil": func(a interface{}) float64 {
			return math.Ceil(toNumber(a))
		},
		"floor": func(a interface{}) float64 {
			return math.Floor(toNumber(a))
		},
		"round": func(a interface{}) float64 {
			return math.Round(toNumber(a))
		},
		"toInt": func(v interface{}) int {
			return int(toNumber(v))
		},
		"toFloat": func(v interface{}) float64 {
			return toNumber(v)
		},
		"toString": func(v interface{}) string {
			return fmt.Sprintf("%v", v)
		},
	}
}
