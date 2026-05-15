package condition

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/tidwall/gjson"
)

// Evaluator evaluates conditions against request data
type Evaluator struct{}

// NewEvaluator creates a new condition evaluator
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// RequestData contains all request data for condition evaluation
type RequestData struct {
	PathParams  map[string]string
	QueryParams map[string][]string
	Headers     map[string][]string
	Body        string
	Signature   string // Pre-computed request signature for signature conditions
}

// EvaluateAll evaluates all conditions against request data
// All conditions must match (AND logic)
func (e *Evaluator) EvaluateAll(conditions []models.Condition, data *RequestData) bool {
	if len(conditions) == 0 {
		return true
	}

	for _, cond := range conditions {
		if !e.Evaluate(cond, data) {
			return false
		}
	}

	return true
}

// Evaluate evaluates a single condition against request data
func (e *Evaluator) Evaluate(cond models.Condition, data *RequestData) bool {
	// Normalise deprecated negative operators before evaluating.
	cond = models.NormaliseDeprecatedOperator(cond)
	value := e.extractValue(cond.Source, cond.Key, data)
	result := e.compare(value, cond.Operator, cond.Value, cond.Format)
	if cond.Negate {
		return !result
	}
	return result
}

// extractValue extracts a value from request data based on source and key
func (e *Evaluator) extractValue(source, key string, data *RequestData) string {
	switch source {
	case models.SourcePath:
		return data.PathParams[key]
	case models.SourceQuery:
		if vals, ok := data.QueryParams[key]; ok && len(vals) > 0 {
			return vals[0]
		}
		return ""
	case models.SourceHeader:
		// Headers are case-insensitive
		for k, vals := range data.Headers {
			if strings.EqualFold(k, key) && len(vals) > 0 {
				return vals[0]
			}
		}
		return ""
	case models.SourceBody:
		// Use JSONPath to extract value from body
		result := gjson.Get(data.Body, key)
		if result.Exists() {
			return result.String()
		}
		return ""
	case models.SourceSignature:
		// Return the pre-computed request signature
		return data.Signature
	default:
		return ""
	}
}

// compare compares a value against an expected value using the specified operator.
// format is an optional Go time layout hint used only by date operators.
func (e *Evaluator) compare(actual, operator, expected, format string) bool {
	switch operator {
	case models.OpEquals:
		return actual == expected
	case models.OpNotEquals:
		return actual != expected
	case models.OpContains:
		return strings.Contains(actual, expected)
	case models.OpNotContains:
		return !strings.Contains(actual, expected)
	case models.OpStartsWith:
		return strings.HasPrefix(actual, expected)
	case models.OpEndsWith:
		return strings.HasSuffix(actual, expected)
	case models.OpRegex:
		re, err := regexp.Compile(Expand(expected))
		if err != nil {
			return false
		}
		return re.MatchString(actual)
	case models.OpExists:
		return actual != ""
	case models.OpNotExists:
		return actual == ""
	case models.OpGreaterThan:
		return compareNumeric(actual, expected) > 0
	case models.OpLessThan:
		return compareNumeric(actual, expected) < 0
	case models.OpGTE:
		return compareNumeric(actual, expected) >= 0
	case models.OpLTE:
		return compareNumeric(actual, expected) <= 0
	case models.OpDateEquals, models.OpDateBefore, models.OpDateAfter,
		models.OpDateLte, models.OpDateGte,
		models.OpDateInPast, models.OpDateInFuture, models.OpDateToday,
		models.OpDateBetween:
		return compareDateOp(actual, operator, expected, format)
	default:
		return false
	}
}

// compareNumeric compares two values numerically
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareNumeric(a, b string) int {
	aFloat, aErr := strconv.ParseFloat(a, 64)
	bFloat, bErr := strconv.ParseFloat(b, 64)

	if aErr != nil || bErr != nil {
		// Fall back to string comparison
		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0
	}

	if aFloat < bFloat {
		return -1
	} else if aFloat > bFloat {
		return 1
	}
	return 0
}

// HasValue checks if a value exists for the given source and key
func (e *Evaluator) HasValue(source, key string, data *RequestData) bool {
	value := e.extractValue(source, key, data)
	return value != ""
}

// GetValue gets a value from request data
func (e *Evaluator) GetValue(source, key string, data *RequestData) string {
	return e.extractValue(source, key, data)
}

// ── Date helpers ────────────────────────────────────────────────────────────

// autoDateLayouts is the ordered list of formats tried when no Format hint is set.
var autoDateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02",
	"01/02/2006",
	"02/01/2006",
	"20060102",
	time.RFC1123,
	time.RFC850,
}

// tokenRe matches dynamic tokens like now+7d, now-30m, now+2h.
var tokenRe = regexp.MustCompile(`^now([+-])(\d+)([dhm])$`)

// resolveToken converts a date token string to a time.Time.
// Recognised tokens: now, today, yesterday, tomorrow,
// now±Nd (days), now±Nh (hours), now±Nm (minutes).
// Falls back to parseDate when no token pattern matches.
func resolveToken(token, format string) (time.Time, error) {
	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)

	switch token {
	case "now":
		return now, nil
	case "today":
		return today, nil
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), nil
	}

	if m := tokenRe.FindStringSubmatch(token); m != nil {
		sign := m[1]
		n, _ := strconv.Atoi(m[2])
		unit := m[3]
		if sign == "-" {
			n = -n
		}
		switch unit {
		case "d":
			return now.AddDate(0, 0, n), nil
		case "h":
			return now.Add(time.Duration(n) * time.Hour), nil
		case "m":
			return now.Add(time.Duration(n) * time.Minute), nil
		}
	}

	// Not a token — treat as a date literal.
	return parseDate(token, format)
}

// parseDate parses a date string using the provided layout hint, or tries all
// autoDateLayouts when hint is empty. Unix second/millisecond integers are
// also accepted.
func parseDate(value, format string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty date value")
	}

	// Try Unix timestamp (seconds or millis) when the string is all digits.
	if isAllDigits(value) {
		n, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			if n >= 1e12 { // milliseconds
				return time.UnixMilli(n).UTC(), nil
			}
			return time.Unix(n, 0).UTC(), nil
		}
	}

	if format != "" {
		t, err := time.Parse(format, value)
		if err != nil {
			return time.Time{}, fmt.Errorf("date %q does not match format %q: %w", value, format, err)
		}
		return t.UTC(), nil
	}

	for _, layout := range autoDateLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date %q", value)
}

// isAllDigits reports whether s contains only ASCII digits.
func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// compareDateOp evaluates a date-specific operator.
func compareDateOp(actual, operator, expected, format string) bool {
	t, err := parseDate(actual, format)
	if err != nil {
		return false
	}
	now := time.Now().UTC()

	switch operator {
	case models.OpDateInPast:
		return t.Before(now)
	case models.OpDateInFuture:
		return t.After(now)
	case models.OpDateToday:
		today := now.Truncate(24 * time.Hour)
		return !t.Truncate(24*time.Hour).Before(today) && t.Truncate(24*time.Hour).Before(today.Add(24*time.Hour))
	case models.OpDateEquals:
		ref, err := resolveToken(expected, format)
		if err != nil {
			return false
		}
		return t.Equal(ref)
	case models.OpDateBefore:
		ref, err := resolveToken(expected, format)
		if err != nil {
			return false
		}
		return t.Before(ref)
	case models.OpDateAfter:
		ref, err := resolveToken(expected, format)
		if err != nil {
			return false
		}
		return t.After(ref)
	case models.OpDateLte:
		ref, err := resolveToken(expected, format)
		if err != nil {
			return false
		}
		return !t.After(ref)
	case models.OpDateGte:
		ref, err := resolveToken(expected, format)
		if err != nil {
			return false
		}
		return !t.Before(ref)
	case models.OpDateBetween:
		parts := strings.SplitN(expected, ",", 2)
		if len(parts) != 2 {
			return false
		}
		from, err1 := resolveToken(strings.TrimSpace(parts[0]), format)
		to, err2 := resolveToken(strings.TrimSpace(parts[1]), format)
		if err1 != nil || err2 != nil {
			return false
		}
		return !t.Before(from) && !t.After(to)
	}
	return false
}
