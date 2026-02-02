package template

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

// Engine processes template strings with variable substitution
type Engine struct {
	rng *rand.Rand
}

// NewEngine creates a new template engine
func NewEngine() *Engine {
	return &Engine{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Context contains all data available for template rendering
type Context struct {
	PathParams  map[string]string
	QueryParams map[string][]string
	Headers     map[string][]string
	Body        string
}

// templateVarPattern matches template variables like {{variable}}
var templateVarPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// Process processes a template string and replaces all variables
func (e *Engine) Process(template string, ctx *Context) string {
	return templateVarPattern.ReplaceAllStringFunc(template, func(match string) string {
		// Extract variable name (remove {{ and }})
		varName := strings.TrimSpace(match[2 : len(match)-2])
		return e.resolveVariable(varName, ctx)
	})
}

// ProcessHeaders processes all headers and replaces template variables
func (e *Engine) ProcessHeaders(headers map[string]string, ctx *Context) map[string]string {
	result := make(map[string]string)
	for key, value := range headers {
		result[key] = e.Process(value, ctx)
	}
	return result
}

// resolveVariable resolves a single variable to its value
func (e *Engine) resolveVariable(varName string, ctx *Context) string {
	// Handle optional leading dot (e.g., both "path.id" and ".path.id" are valid)
	varName = strings.TrimPrefix(varName, ".")
	
	parts := strings.SplitN(varName, ".", 2)
	if len(parts) < 1 {
		return ""
	}

	source := parts[0]
	var key string
	if len(parts) > 1 {
		key = parts[1]
	}

	switch source {
	case "path":
		if key != "" && ctx.PathParams != nil {
			if val, ok := ctx.PathParams[key]; ok {
				return val
			}
		}
	case "query":
		if key != "" && ctx.QueryParams != nil {
			if vals, ok := ctx.QueryParams[key]; ok && len(vals) > 0 {
				return vals[0]
			}
		}
	case "header":
		if key != "" && ctx.Headers != nil {
			// Headers are case-insensitive
			for k, vals := range ctx.Headers {
				if strings.EqualFold(k, key) && len(vals) > 0 {
					return vals[0]
				}
			}
		}
	case "body":
		if key != "" && ctx.Body != "" {
			result := gjson.Get(ctx.Body, key)
			if result.Exists() {
				return result.String()
			}
		}
	case "random":
		return e.resolveRandom(key)
	case "timestamp":
		return e.resolveTimestamp(key)
	case "env":
		// Environment variables could be added here if needed
		return ""
	}

	return ""
}

// resolveRandom resolves random value generators
func (e *Engine) resolveRandom(key string) string {
	switch {
	case key == "uuid":
		return uuid.New().String()
	case key == "int":
		return strconv.Itoa(e.rng.Intn(1000000))
	case strings.HasPrefix(key, "int("):
		// Parse int(min,max)
		params := parseParams(key, "int")
		if len(params) == 2 {
			min, _ := strconv.Atoi(params[0])
			max, _ := strconv.Atoi(params[1])
			if max > min {
				return strconv.Itoa(min + e.rng.Intn(max-min+1))
			}
		}
		return strconv.Itoa(e.rng.Intn(1000000))
	case key == "float":
		return fmt.Sprintf("%.2f", e.rng.Float64()*1000)
	case strings.HasPrefix(key, "float("):
		params := parseParams(key, "float")
		if len(params) == 2 {
			min, _ := strconv.ParseFloat(params[0], 64)
			max, _ := strconv.ParseFloat(params[1], 64)
			if max > min {
				return fmt.Sprintf("%.2f", min+e.rng.Float64()*(max-min))
			}
		}
		return fmt.Sprintf("%.2f", e.rng.Float64()*1000)
	case key == "string":
		return randomString(e.rng, 10)
	case strings.HasPrefix(key, "string("):
		params := parseParams(key, "string")
		if len(params) == 1 {
			length, _ := strconv.Atoi(params[0])
			if length > 0 {
				return randomString(e.rng, length)
			}
		}
		return randomString(e.rng, 10)
	case key == "bool":
		if e.rng.Intn(2) == 0 {
			return "false"
		}
		return "true"
	case key == "email":
		return fmt.Sprintf("%s@example.com", randomString(e.rng, 8))
	case key == "name":
		names := []string{"John", "Jane", "Bob", "Alice", "Charlie", "Diana", "Eve", "Frank"}
		return names[e.rng.Intn(len(names))]
	case key == "phone":
		return fmt.Sprintf("+1-%03d-%03d-%04d", e.rng.Intn(1000), e.rng.Intn(1000), e.rng.Intn(10000))
	}

	return ""
}

// resolveTimestamp resolves timestamp generators
func (e *Engine) resolveTimestamp(key string) string {
	now := time.Now()

	switch {
	case key == "" || key == "unix":
		return strconv.FormatInt(now.Unix(), 10)
	case key == "unixMilli":
		return strconv.FormatInt(now.UnixMilli(), 10)
	case key == "unixNano":
		return strconv.FormatInt(now.UnixNano(), 10)
	case key == "iso":
		return now.Format(time.RFC3339)
	case key == "date":
		return now.Format("2006-01-02")
	case key == "time":
		return now.Format("15:04:05")
	case key == "datetime":
		return now.Format("2006-01-02 15:04:05")
	case strings.HasPrefix(key, "format("):
		params := parseParams(key, "format")
		if len(params) == 1 {
			return now.Format(params[0])
		}
	case strings.HasPrefix(key, "add("):
		// Add duration to current time: timestamp.add(1h)
		params := parseParams(key, "add")
		if len(params) == 1 {
			duration, err := time.ParseDuration(params[0])
			if err == nil {
				return now.Add(duration).Format(time.RFC3339)
			}
		}
	}

	return strconv.FormatInt(now.Unix(), 10)
}

// parseParams extracts parameters from a function call like "func(param1,param2)"
func parseParams(key, funcName string) []string {
	prefix := funcName + "("
	if !strings.HasPrefix(key, prefix) {
		return nil
	}

	// Remove prefix and trailing )
	paramsStr := strings.TrimPrefix(key, prefix)
	paramsStr = strings.TrimSuffix(paramsStr, ")")

	if paramsStr == "" {
		return nil
	}

	// Split by comma
	return strings.Split(paramsStr, ",")
}

// randomString generates a random alphanumeric string
func randomString(rng *rand.Rand, length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rng.Intn(len(charset))]
	}
	return string(result)
}
