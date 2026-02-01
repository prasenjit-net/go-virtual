package condition

import (
	"regexp"
	"strconv"
	"strings"

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
	value := e.extractValue(cond.Source, cond.Key, data)
	return e.compare(value, cond.Operator, cond.Value)
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
	default:
		return ""
	}
}

// compare compares a value against an expected value using the specified operator
func (e *Evaluator) compare(actual, operator, expected string) bool {
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
		re, err := regexp.Compile(expected)
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
