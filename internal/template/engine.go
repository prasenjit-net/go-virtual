package template

import (
	"bytes"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	texttmpl "text/template"
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
	RNG         *rand.Rand
}

type bodyTemplateData struct {
	Path   map[string]string
	Query  map[string]string
	Header map[string]string
	Body   string
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

// RenderBodyTemplate renders the body using text/template with advanced features
func (e *Engine) RenderBodyTemplate(body string, ctx *Context) (string, error) {
	if body == "" {
		return "", nil
	}

	preprocessed := preprocessLegacyBodyTemplate(body)
	data, funcMap := e.buildBodyTemplateContext(ctx)

	tmpl, err := texttmpl.New("body").Option("missingkey=zero").Funcs(funcMap).Parse(preprocessed)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ValidateBodyTemplate checks whether a body template can be parsed with current helpers.
func (e *Engine) ValidateBodyTemplate(body string) error {
	if body == "" {
		return nil
	}

	preprocessed := preprocessLegacyBodyTemplate(body)
	_, funcMap := e.buildBodyTemplateContext(nil)
	_, err := texttmpl.New("body").Option("missingkey=zero").Funcs(funcMap).Parse(preprocessed)
	return err
}

func (e *Engine) buildBodyTemplateContext(ctx *Context) (bodyTemplateData, texttmpl.FuncMap) {
	if ctx == nil {
		ctx = &Context{}
	}

	rng := e.rngForContext(ctx)
	query := normalizeFirstValues(ctx.QueryParams)
	headers := normalizeFirstValues(ctx.Headers)

	data := bodyTemplateData{
		Path:   ctx.PathParams,
		Query:  query,
		Header: headers,
		Body:   ctx.Body,
	}

	funcMap := texttmpl.FuncMap{
		"path": func(key string) string {
			if key == "" || ctx.PathParams == nil {
				return ""
			}
			return ctx.PathParams[key]
		},
		"query": func(key string) string {
			if key == "" {
				return ""
			}
			val, ok := query[strings.ToLower(key)]
			if !ok {
				return ""
			}
			return val
		},
		"header": func(key string) string {
			if key == "" {
				return ""
			}
			val, ok := headers[strings.ToLower(key)]
			if !ok {
				return ""
			}
			return val
		},
		"body": func(path string) string {
			if path == "" {
				return ctx.Body
			}
			if ctx.Body == "" {
				return ""
			}
			result := gjson.Get(ctx.Body, path)
			if result.Exists() {
				return result.String()
			}
			return ""
		},
		"random": func(args ...interface{}) string {
			return e.resolveRandom(buildKeyFromArgs(args), rng)
		},
		"faker": func(args ...interface{}) string {
			return e.resolveFaker(buildDotKeyFromArgs(args), rng)
		},
		"timestamp": func(args ...interface{}) string {
			return e.resolveTimestamp(buildKeyFromArgs(args))
		},
		"env": func(string) string {
			return ""
		},
	}

	return data, funcMap
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

	rng := e.rngForContext(ctx)

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
		return e.resolveRandom(key, rng)
	case "faker":
		return e.resolveFaker(key, rng)
	case "timestamp":
		return e.resolveTimestamp(key)
	case "env":
		// Environment variables could be added here if needed
		return ""
	}

	return ""
}

// resolveRandom resolves random value generators
func (e *Engine) resolveRandom(key string, rng *rand.Rand) string {
	if rng == nil {
		rng = e.rng
	}

	switch {
	case key == "uuid":
		return uuid.New().String()
	case key == "int":
		return strconv.Itoa(rng.Intn(1000000))
	case strings.HasPrefix(key, "int("):
		// Parse int(min,max)
		params := parseParams(key, "int")
		if len(params) == 2 {
			min, _ := strconv.Atoi(params[0])
			max, _ := strconv.Atoi(params[1])
			if max > min {
				return strconv.Itoa(min + rng.Intn(max-min+1))
			}
		}
		return strconv.Itoa(rng.Intn(1000000))
	case key == "float":
		return fmt.Sprintf("%.2f", rng.Float64()*1000)
	case strings.HasPrefix(key, "float("):
		params := parseParams(key, "float")
		if len(params) == 2 {
			min, _ := strconv.ParseFloat(params[0], 64)
			max, _ := strconv.ParseFloat(params[1], 64)
			if max > min {
				return fmt.Sprintf("%.2f", min+rng.Float64()*(max-min))
			}
		}
		return fmt.Sprintf("%.2f", rng.Float64()*1000)
	case key == "string":
		return randomString(rng, 10)
	case strings.HasPrefix(key, "string("):
		params := parseParams(key, "string")
		if len(params) == 1 {
			length, _ := strconv.Atoi(params[0])
			if length > 0 {
				return randomString(rng, length)
			}
		}
		return randomString(rng, 10)
	case key == "bool":
		if rng.Intn(2) == 0 {
			return "false"
		}
		return "true"
	case key == "email":
		return fmt.Sprintf("%s@example.com", randomString(rng, 8))
	case key == "name":
		names := []string{"John", "Jane", "Bob", "Alice", "Charlie", "Diana", "Eve", "Frank"}
		return names[rng.Intn(len(names))]
	case key == "phone":
		return fmt.Sprintf("+1-%03d-%03d-%04d", rng.Intn(1000), rng.Intn(1000), rng.Intn(10000))
	}

	return ""
}

func (e *Engine) rngForContext(ctx *Context) *rand.Rand {
	if ctx != nil && ctx.RNG != nil {
		return ctx.RNG
	}
	return e.rng
}

// resolveFaker resolves faker-style generators
func (e *Engine) resolveFaker(key string, rng *rand.Rand) string {
	if rng == nil {
		rng = e.rng
	}
	if key == "" {
		return ""
	}

	parts := strings.Split(key, ".")
	category := parts[0]
	field := ""
	if len(parts) > 1 {
		field = parts[1]
	}

	switch category {
	case "name":
		return fakerName(field, rng)
	case "email":
		return fakerEmail(rng)
	case "phone":
		return fakerPhone(rng)
	case "company":
		return fakerCompany(field, rng)
	case "address":
		return fakerAddress(field, rng)
	case "internet":
		return fakerInternet(field, rng)
	case "lorem":
		return fakerLorem(field, rng)
	case "uuid":
		return uuid.New().String()
	case "username":
		return fakerInternet("username", rng)
	case "domain":
		return fakerInternet("domain", rng)
	case "url":
		return fakerInternet("url", rng)
	case "firstName":
		return fakerName("first", rng)
	case "lastName":
		return fakerName("last", rng)
	case "fullName":
		return fakerName("full", rng)
	}

	return ""
}

func fakerName(field string, rng *rand.Rand) string {
	firstNames := []string{"Liam", "Noah", "Olivia", "Emma", "Ava", "Sophia", "Mason", "Logan", "Mia", "Lucas"}
	lastNames := []string{"Smith", "Johnson", "Brown", "Taylor", "Anderson", "Thomas", "Jackson", "White", "Harris", "Martin"}
	first := firstNames[rng.Intn(len(firstNames))]
	last := lastNames[rng.Intn(len(lastNames))]

	switch field {
	case "first":
		return first
	case "last":
		return last
	case "full", "":
		return first + " " + last
	}

	return first + " " + last
}

func fakerEmail(rng *rand.Rand) string {
	providers := []string{"example.com", "mail.test", "dev.local", "sample.net"}
	return fmt.Sprintf("%s@%s", randomString(rng, 8), providers[rng.Intn(len(providers))])
}

func fakerPhone(rng *rand.Rand) string {
	return fmt.Sprintf("+1-%03d-%03d-%04d", rng.Intn(1000), rng.Intn(1000), rng.Intn(10000))
}

func fakerCompany(field string, rng *rand.Rand) string {
	companies := []string{"Acme", "Globex", "Initech", "Umbrella", "Hooli", "Vandelay", "Stark", "Wayne", "Wonka", "Aperture"}
	suffixes := []string{"Inc", "LLC", "Ltd", "Group", "Corp"}
	name := companies[rng.Intn(len(companies))]

	switch field {
	case "name", "":
		return name + " " + suffixes[rng.Intn(len(suffixes))]
	case "suffix":
		return suffixes[rng.Intn(len(suffixes))]
	}

	return name + " " + suffixes[rng.Intn(len(suffixes))]
}

func fakerAddress(field string, rng *rand.Rand) string {
	streets := []string{"Main", "Oak", "Pine", "Maple", "Cedar", "Elm", "Sunset", "Hillcrest", "Park", "Lake"}
	cities := []string{"Springfield", "Riverton", "Fairview", "Greenville", "Madison", "Georgetown", "Ashland", "Clinton"}
	states := []string{"CA", "TX", "NY", "FL", "WA", "IL", "CO", "MA"}
	street := fmt.Sprintf("%d %s St", 100+rng.Intn(900), streets[rng.Intn(len(streets))])
	city := cities[rng.Intn(len(cities))]
	state := states[rng.Intn(len(states))]
	zip := fmt.Sprintf("%05d", rng.Intn(100000))

	switch field {
	case "street", "streetAddress":
		return street
	case "city":
		return city
	case "state":
		return state
	case "zip", "postalCode":
		return zip
	case "full", "":
		return fmt.Sprintf("%s, %s, %s %s", street, city, state, zip)
	}

	return fmt.Sprintf("%s, %s, %s %s", street, city, state, zip)
}

func fakerInternet(field string, rng *rand.Rand) string {
	domains := []string{"example.com", "demo.dev", "mock.io", "test.net"}
	username := randomString(rng, 10)
	domain := domains[rng.Intn(len(domains))]

	switch field {
	case "username", "user":
		return username
	case "domain":
		return domain
	case "url", "":
		return fmt.Sprintf("https://%s/%s", domain, randomString(rng, 6))
	}

	return fmt.Sprintf("https://%s/%s", domain, randomString(rng, 6))
}

func fakerLorem(field string, rng *rand.Rand) string {
	words := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliet", "kilo", "lima"}
	word := words[rng.Intn(len(words))]

	switch field {
	case "word", "":
		return word
	case "sentence":
		return fmt.Sprintf("%s %s %s.", words[rng.Intn(len(words))], words[rng.Intn(len(words))], words[rng.Intn(len(words))])
	case "paragraph":
		return fmt.Sprintf("%s %s %s. %s %s %s.", words[rng.Intn(len(words))], words[rng.Intn(len(words))], words[rng.Intn(len(words))], words[rng.Intn(len(words))], words[rng.Intn(len(words))], words[rng.Intn(len(words))])
	}

	return word
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
	if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, ")") {
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
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rng.Intn(len(chars))]
	}
	return string(b)
}

func preprocessLegacyBodyTemplate(input string) string {
	if input == "" {
		return input
	}

	keywords := map[string]struct{}{
		"if": {}, "range": {}, "with": {}, "end": {}, "else": {}, "define": {}, "template": {}, "block": {},
	}

	return templateVarPattern.ReplaceAllStringFunc(input, func(match string) string {
		value := strings.TrimSpace(match[2 : len(match)-2])
		if value == "" {
			return match
		}
		if strings.ContainsAny(value, " \t\n") {
			first := strings.Fields(value)
			if len(first) > 0 {
				if _, ok := keywords[first[0]]; ok {
					return match
				}
			}
			return match
		}
		if strings.HasPrefix(value, ".") {
			trimmed := strings.TrimPrefix(value, ".")
			if converted := legacyTokenToTemplate(trimmed); converted != "{{"+trimmed+"}}" {
				return converted
			}
			return match
		}
		if _, ok := keywords[value]; ok {
			return match
		}

		return legacyTokenToTemplate(value)
	})
}


func legacyTokenToTemplate(value string) string {
	switch {
	case strings.HasPrefix(value, "path."):
		return fmt.Sprintf("{{path %q}}", strings.TrimPrefix(value, "path."))
	case strings.HasPrefix(value, "query."):
		return fmt.Sprintf("{{query %q}}", strings.TrimPrefix(value, "query."))
	case strings.HasPrefix(value, "header."):
		return fmt.Sprintf("{{header %q}}", strings.TrimPrefix(value, "header."))
	case strings.HasPrefix(value, "body."):
		return fmt.Sprintf("{{body %q}}", strings.TrimPrefix(value, "body."))
	case strings.HasPrefix(value, "random."):
		return fmt.Sprintf("{{random %q}}", strings.TrimPrefix(value, "random."))
	case strings.HasPrefix(value, "faker."):
		return fmt.Sprintf("{{faker %q}}", strings.TrimPrefix(value, "faker."))
	case strings.HasPrefix(value, "timestamp."):
		return fmt.Sprintf("{{timestamp %q}}", strings.TrimPrefix(value, "timestamp."))
	case strings.HasPrefix(value, "env."):
		return fmt.Sprintf("{{env %q}}", strings.TrimPrefix(value, "env."))
	default:
		return "{{" + value + "}}"
	}
}

func normalizeFirstValues(values map[string][]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}

	normalized := make(map[string]string, len(values))
	for key, vals := range values {
		if len(vals) == 0 {
			continue
		}
		lower := strings.ToLower(key)
		normalized[lower] = vals[0]
	}

	return normalized
}

func buildKeyFromArgs(args []interface{}) string {
	if len(args) == 0 {
		return ""
	}
	base := fmt.Sprint(args[0])
	if len(args) == 1 {
		return base
	}
	params := make([]string, 0, len(args)-1)
	for _, arg := range args[1:] {
		params = append(params, fmt.Sprint(arg))
	}
	return fmt.Sprintf("%s(%s)", base, strings.Join(params, ","))
}

func buildDotKeyFromArgs(args []interface{}) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return fmt.Sprint(args[0])
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, fmt.Sprint(arg))
	}
	return strings.Join(parts, ".")
}
