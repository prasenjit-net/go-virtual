package template

import (
	"fmt"
	"math"
	"strings"
	texttmpl "text/template"
	"time"
)

// buildFormatFuncMap returns template functions for JSON and formatting helpers.
func buildFormatFuncMap() texttmpl.FuncMap {
	return texttmpl.FuncMap{
		// numberFormat formats a number with thousands separators.
		"numberFormat": func(v interface{}) string {
			f := toNumber(v)
			intPart := int64(math.Abs(f))
			fracPart := math.Abs(f) - float64(intPart)

			// Format integer part with commas
			s := fmt.Sprintf("%d", intPart)
			if len(s) > 3 {
				var result []byte
				for i, c := range s {
					if i > 0 && (len(s)-i)%3 == 0 {
						result = append(result, ',')
					}
					result = append(result, byte(c))
				}
				s = string(result)
			}
			if f < 0 {
				s = "-" + s
			}
			// Append fractional part if non-zero
			if fracPart > 0 {
				s += fmt.Sprintf("%s", strings.TrimPrefix(fmt.Sprintf("%.2f", fracPart), "0"))
			} else {
				s += ".00"
			}
			return s
		},
		// currency formats a number with a currency symbol prefix.
		"currency": func(v interface{}, symbol string) string {
			f := toNumber(v)
			return fmt.Sprintf("%s%.2f", symbol, f)
		},
		// dateFmt formats a time.Time with the given layout (pipeable: {{now | dateFmt "Jan 2006"}}).
		"dateFmt": func(layout string, t time.Time) string {
			return t.Format(layout)
		},
		// durationHuman formats a duration in a human-readable way.
		"durationHuman": func(d interface{}) string {
			dur := time.Duration(int64(toNumber(d)))
			if dur < time.Minute {
				return fmt.Sprintf("%ds", int(dur.Seconds()))
			} else if dur < time.Hour {
				return fmt.Sprintf("%dm%ds", int(dur.Minutes()), int(dur.Seconds())%60)
			}
			return fmt.Sprintf("%dh%dm", int(dur.Hours()), int(dur.Minutes())%60)
		},
	}
}
