package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	texttmpl "text/template"
	"time"

	"github.com/google/uuid"
	"github.com/prasenjit/go-virtual/internal/models"
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
	PathParams   map[string]string
	QueryParams  map[string][]string
	Headers      map[string][]string
	Body         string
	RNG          *rand.Rand
	// ScriptOutput holds results from script bindings, keyed by outputKey.
	ScriptOutput map[string]any
	// CollectionOutput holds results from collection mappings, keyed by outputKey.
	CollectionOutput map[string]any
	// ValidationOutput holds results from validation rules, keyed by rule name.
	ValidationOutput map[string]*models.ValidationResult

	// New optional fields — all zero-value safe.
	Method    string // HTTP method, e.g. "GET"
	RequestURL string // full request URL string
	RequestID string  // stable UUID for this request

	// StoreReader reads from the session store; nil = no store access.
	StoreReader func(key string) string
	// StoreWriter increments a named counter and returns its value.
	StoreWriter func(name string) string
}

// TemplateData is the dot value (.) passed to all Go text/template execution.
// Fields are directly accessible via native Go template dot-notation.
type TemplateData struct {
	// Request sources
	Path   map[string]string // {{.Path.id}}
	Query  map[string]string // {{.Query.page}}
	Header map[string]string // {{.Header.authorization}} (lowercased keys)

	// Body is the parsed JSON body as map[string]any.
	// Enables native Go template traversal: {{.Body.user.name}}
	// For array/complex gjson paths use {{body "items.0.id"}} instead.
	Body    map[string]any // {{.Body.name}}, {{.Body.user.address.city}}
	RawBody string         // {{.RawBody}} — original body string

	// Request metadata
	Method    string // {{.Method}}
	URL       string // {{.URL}}
	RequestID string // {{.RequestID}}

	// Script holds script binding output, keyed by outputKey.
	// Access as {{.Script.pricing.total}} in templates.
	Script map[string]any
	// Collection holds collection mapping output, keyed by outputKey.
	// Access as {{.Collection.user.name}} in templates.
	Collection map[string]any
	// Validation holds validation rule results. Access as {{.Validation.auth_check.status}}
	Validation map[string]map[string]string
}

// bodyTemplateData is a legacy alias kept for internal backward compat.
type bodyTemplateData = TemplateData

// templateVarPattern matches template variables like {{variable}}
var templateVarPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// Process processes a template string by running it through the full Go template engine.
// This ensures headers and simple string templates have access to all helpers.
func (e *Engine) Process(tmplStr string, ctx *Context) string {
	result, err := e.RenderBodyTemplate(tmplStr, ctx)
	if err != nil {
		// Fallback: simple variable replacement for templates that fail to parse
		return templateVarPattern.ReplaceAllStringFunc(tmplStr, func(match string) string {
			varName := strings.TrimSpace(match[2 : len(match)-2])
			return e.resolveVariable(varName, ctx)
		})
	}
	return result
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

func (e *Engine) buildBodyTemplateContext(ctx *Context) (TemplateData, texttmpl.FuncMap) {
	if ctx == nil {
		ctx = &Context{}
	}

	rng := e.rngForContext(ctx)
	query := normalizeFirstValues(ctx.QueryParams)
	headers := normalizeFirstValues(ctx.Headers)

	scriptOutput := ctx.ScriptOutput
	collectionOutput := ctx.CollectionOutput

	// Parse JSON body into a map for native dot-traversal.
	var parsedBody map[string]any
	if ctx.Body != "" {
		_ = json.Unmarshal([]byte(ctx.Body), &parsedBody)
	}

	// Build validation data map: name -> {status: "pass"|"fail", ...properties}
	validationData := make(map[string]map[string]string)
	for name, result := range ctx.ValidationOutput {
		entry := map[string]string{"status": result.Status}
		for k, v := range result.Properties {
			entry[k] = v
		}
		validationData[name] = entry
	}

	data := TemplateData{
		Path:       ctx.PathParams,
		Query:      query,
		Header:     headers,
		Body:       parsedBody,
		RawBody:    ctx.Body,
		Script:     scriptOutput,
		Collection: collectionOutput,
		Validation: validationData,
		Method:     ctx.Method,
		URL:        ctx.RequestURL,
		RequestID:  ctx.RequestID,
	}

	funcMap := texttmpl.FuncMap{
		// ── Source functions (legacy function-call style) ──────────────────────
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
		"rawBody": func() string { return ctx.Body },
		"random": func(args ...interface{}) string {
			return e.resolveRandom(buildKeyFromArgs(args), rng)
		},
		"faker": func(args ...interface{}) string {
			return e.resolveFaker(buildDotKeyFromArgs(args), rng)
		},
		"timestamp": func(args ...interface{}) string {
			return e.resolveTimestamp(buildKeyFromArgs(args))
		},
		"env": func(string) string { return "" },
		// script resolves a dot-path into the script output map.
		"script": func(path string) string {
			if path == "" || scriptOutput == nil {
				return ""
			}
			return resolveScriptOutputPath(scriptOutput, path)
		},
		// store reads a key from the session store (empty if not available).
		"store": func(key string) string {
			if ctx.StoreReader == nil {
				return ""
			}
			return ctx.StoreReader(key)
		},
		// counter increments a named session counter and returns its value.
		"counter": func(name string) string {
			if ctx.StoreWriter == nil {
				return "0"
			}
			return ctx.StoreWriter(name)
		},
		// ── now / dateFormat ───────────────────────────────────────────────────
		"now": func() time.Time { return time.Now().UTC() },
		"dateFormat": func(layout string, t time.Time) string {
			return t.Format(layout)
		},
		// ── toJSON ────────────────────────────────────────────────────────────
		"toJSON": func(v interface{}) string {
			b, err := json.Marshal(v)
			if err != nil {
				return ""
			}
			return string(b)
		},
		// jsonGet extracts a gjson path from a JSON string field.
		"jsonGet": func(path, jsonStr string) string {
			result := gjson.Get(jsonStr, path)
			if result.Exists() {
				return result.String()
			}
			return ""
		},
	}

	// Merge string, math, iter, json formatting funcMaps
	for k, v := range buildStringFuncMap() {
		funcMap[k] = v
	}
	for k, v := range buildMathFuncMap() {
		funcMap[k] = v
	}
	for k, v := range buildIterFuncMap() {
		funcMap[k] = v
	}
	for k, v := range buildFormatFuncMap() {
		funcMap[k] = v
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
	case "script":
		if key != "" && ctx != nil && ctx.ScriptOutput != nil {
			return resolveScriptOutputPath(ctx.ScriptOutput, key)
		}
	case "env":
		// Environment variables could be added here if needed
		return ""
	}

	return ""
}

// resolveScriptOutputPath walks a dot-separated path through a script output map,
// returning the string representation of the leaf value (or "" if not found).
func resolveScriptOutputPath(output map[string]any, path string) string {
	if output == nil || path == "" {
		return ""
	}

	parts := strings.SplitN(path, ".", 2)
	v, ok := output[parts[0]]
	if !ok || v == nil {
		return ""
	}

	if len(parts) == 1 {
		// Leaf: convert to string
		return fmt.Sprintf("%v", v)
	}

	// Recurse into nested map
	if nested, ok := v.(map[string]any); ok {
		return resolveScriptOutputPath(nested, parts[1])
	}
	return ""
}

// resolveRandom resolves random value generators
func (e *Engine) resolveRandom(key string, rng *rand.Rand) string {
	if rng == nil {
		rng = e.rng
	}

	switch {
	case key == "uuid" || key == "uuid4":
		return uuid.New().String()
	case key == "int":
		return strconv.Itoa(rng.Intn(1000000))
	case strings.HasPrefix(key, "int("):
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
	case key == "string" || key == "alphanumeric":
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
	case strings.HasPrefix(key, "alphanumeric("):
		params := parseParams(key, "alphanumeric")
		if len(params) == 1 {
			length, _ := strconv.Atoi(params[0])
			if length > 0 {
				return randomString(rng, length)
			}
		}
		return randomString(rng, 10)
	case key == "alpha":
		return randomAlpha(rng, 10, false)
	case strings.HasPrefix(key, "alpha("):
		params := parseParams(key, "alpha")
		if len(params) == 1 {
			length, _ := strconv.Atoi(params[0])
			if length > 0 {
				return randomAlpha(rng, length, false)
			}
		}
		return randomAlpha(rng, 10, false)
	case key == "ALPHA":
		return randomAlpha(rng, 10, true)
	case strings.HasPrefix(key, "ALPHA("):
		params := parseParams(key, "ALPHA")
		if len(params) == 1 {
			length, _ := strconv.Atoi(params[0])
			if length > 0 {
				return randomAlpha(rng, length, true)
			}
		}
		return randomAlpha(rng, 10, true)
	case key == "numeric":
		return randomNumeric(rng, 6)
	case strings.HasPrefix(key, "numeric("):
		params := parseParams(key, "numeric")
		if len(params) == 1 {
			length, _ := strconv.Atoi(params[0])
			if length > 0 {
				return randomNumeric(rng, length)
			}
		}
		return randomNumeric(rng, 6)
	case key == "hex":
		return randomHex(rng, 8)
	case strings.HasPrefix(key, "hex("):
		params := parseParams(key, "hex")
		if len(params) == 1 {
			length, _ := strconv.Atoi(params[0])
			if length > 0 {
				return randomHex(rng, length)
			}
		}
		return randomHex(rng, 8)
	case key == "bool":
		if rng.Intn(2) == 0 {
			return "false"
		}
		return "true"
	case key == "email":
		return fmt.Sprintf("%s@example.com", randomAlpha(rng, 8, false))
	case key == "name":
		names := []string{"John", "Jane", "Bob", "Alice", "Charlie", "Diana", "Eve", "Frank"}
		return names[rng.Intn(len(names))]
	case key == "phone":
		return fmt.Sprintf("+1-%03d-%03d-%04d", rng.Intn(1000), rng.Intn(1000), rng.Intn(10000))
	}

	return ""
}

// randomAlpha generates a random alpha string (lower or upper case).
func randomAlpha(rng *rand.Rand, length int, upper bool) string {
	const lower = "abcdefghijklmnopqrstuvwxyz"
	const upperC = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	chars := lower
	if upper {
		chars = upperC
	}
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rng.Intn(len(chars))]
	}
	return string(b)
}

// randomNumeric generates a random string of digits.
func randomNumeric(rng *rand.Rand, length int) string {
	const digits = "0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = digits[rng.Intn(len(digits))]
	}
	return string(b)
}

// randomHex generates a random lowercase hex string.
func randomHex(rng *rand.Rand, length int) string {
	const hexChars = "0123456789abcdef"
	b := make([]byte, length)
	for i := range b {
		b[i] = hexChars[rng.Intn(len(hexChars))]
	}
	return string(b)
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
	case "date":
		return fakerDate(field, rng)
	case "finance":
		return fakerFinance(field, rng)
	case "product":
		return fakerProduct(field, rng)
	case "location":
		return fakerLocation(field, rng)
	case "id":
		return fakerID(field, rng)
	case "color":
		return fakerColor(field, rng)
	case "number":
		return fakerNumber(field, rng)
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
	username := randomAlpha(rng, 10, false)
	domain := domains[rng.Intn(len(domains))]

	switch field {
	case "username", "user":
		return username
	case "domain":
		return domain
	case "url", "":
		return fmt.Sprintf("https://%s/%s", domain, randomAlpha(rng, 6, false))
	case "ip":
		return fmt.Sprintf("%d.%d.%d.%d", rng.Intn(256), rng.Intn(256), rng.Intn(256), rng.Intn(256))
	case "ipv6":
		return fmt.Sprintf("2001:%04x:%04x:%04x::%04x", rng.Intn(0x10000), rng.Intn(0x10000), rng.Intn(0x10000), rng.Intn(0x10000))
	case "mac":
		return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
			rng.Intn(256), rng.Intn(256), rng.Intn(256), rng.Intn(256), rng.Intn(256), rng.Intn(256))
	case "port":
		return strconv.Itoa(1024 + rng.Intn(64511))
	}

	return fmt.Sprintf("https://%s/%s", domain, randomAlpha(rng, 6, false))
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
	case key == "unixMilli" || key == "unix_ms":
		return strconv.FormatInt(now.UnixMilli(), 10)
	case key == "unixNano" || key == "unix_ns":
		return strconv.FormatInt(now.UnixNano(), 10)
	case key == "iso" || key == "utc":
		return now.UTC().Format(time.RFC3339)
	case key == "date":
		return now.Format("2006-01-02")
	case key == "time":
		return now.Format("15:04:05")
	case key == "datetime":
		return now.Format("2006-01-02 15:04:05")
	case key == "year":
		return now.Format("2006")
	case key == "month":
		return now.Format("01")
	case key == "day":
		return now.Format("02")
	case strings.HasPrefix(key, "format("):
		params := parseParams(key, "format")
		if len(params) == 1 {
			return now.Format(params[0])
		}
	case strings.HasPrefix(key, "add("):
		params := parseParams(key, "add")
		if len(params) == 1 {
			duration, err := time.ParseDuration(params[0])
			if err == nil {
				return now.Add(duration).Format(time.RFC3339)
			}
		}
	case strings.HasPrefix(key, "sub("):
		params := parseParams(key, "sub")
		if len(params) == 1 {
			duration, err := time.ParseDuration(params[0])
			if err == nil {
				return now.Add(-duration).Format(time.RFC3339)
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
	case strings.HasPrefix(value, "script."):
		return fmt.Sprintf("{{script %q}}", strings.TrimPrefix(value, "script."))
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

// fakerDate generates date-related fake values.
func fakerDate(field string, rng *rand.Rand) string {
	now := time.Now()
	switch field {
	case "past":
		days := rng.Intn(365) + 1
		return now.AddDate(0, 0, -days).Format("2006-01-02")
	case "future":
		days := rng.Intn(365) + 1
		return now.AddDate(0, 0, days).Format("2006-01-02")
	case "recent":
		days := rng.Intn(30) + 1
		return now.AddDate(0, 0, -days).Format("2006-01-02")
	case "birthdate":
		years := 18 + rng.Intn(62)
		return now.AddDate(-years, -rng.Intn(12), -rng.Intn(28)).Format("2006-01-02")
	}
	return now.Format("2006-01-02")
}

// fakerFinance generates finance-related fake values.
func fakerFinance(field string, rng *rand.Rand) string {
	currencies := []string{"USD", "EUR", "GBP", "JPY", "CAD", "AUD", "CHF"}
	symbols := []string{"$", "€", "£", "¥", "CA$", "A$", "Fr"}
	switch field {
	case "amount":
		return fmt.Sprintf("%.2f", float64(rng.Intn(100000))/100.0)
	case "currency":
		return currencies[rng.Intn(len(currencies))]
	case "currencySymbol":
		return symbols[rng.Intn(len(symbols))]
	case "iban":
		return fmt.Sprintf("GB%02d MOCK %04d %04d %04d %02d",
			rng.Intn(100), rng.Intn(10000), rng.Intn(10000), rng.Intn(10000), rng.Intn(100))
	case "creditCard":
		return fmt.Sprintf("4%03d-%04d-%04d-%04d",
			rng.Intn(1000), rng.Intn(10000), rng.Intn(10000), rng.Intn(10000))
	}
	return fmt.Sprintf("%.2f", float64(rng.Intn(100000))/100.0)
}

// fakerProduct generates product/commerce fake values.
func fakerProduct(field string, rng *rand.Rand) string {
	adjectives := []string{"Ergonomic", "Sleek", "Robust", "Premium", "Ultra", "Smart", "Compact"}
	materials := []string{"Steel", "Plastic", "Wooden", "Aluminum", "Carbon", "Titanium"}
	types := []string{"Chair", "Desk", "Lamp", "Monitor", "Keyboard", "Mouse", "Headset"}
	categories := []string{"Electronics", "Furniture", "Clothing", "Tools", "Sports", "Books"}
	switch field {
	case "name":
		return fmt.Sprintf("%s %s %s",
			adjectives[rng.Intn(len(adjectives))],
			materials[rng.Intn(len(materials))],
			types[rng.Intn(len(types))])
	case "category":
		return categories[rng.Intn(len(categories))]
	case "price":
		return fmt.Sprintf("%.2f", float64(rng.Intn(99900)+100)/100.0)
	case "sku":
		return fmt.Sprintf("SKU-%s-%04d", randomAlpha(rng, 3, true), rng.Intn(10000))
	}
	return fmt.Sprintf("%s %s %s",
		adjectives[rng.Intn(len(adjectives))],
		materials[rng.Intn(len(materials))],
		types[rng.Intn(len(types))])
}

// fakerLocation generates geo/location fake values.
func fakerLocation(field string, rng *rand.Rand) string {
	countries := []string{"United States", "Germany", "France", "Japan", "Brazil", "India", "Canada", "Australia"}
	codes := []string{"US", "DE", "FR", "JP", "BR", "IN", "CA", "AU"}
	timezones := []string{"America/New_York", "Europe/Berlin", "Europe/Paris", "Asia/Tokyo", "America/Sao_Paulo", "Asia/Kolkata", "America/Toronto", "Australia/Sydney"}
	idx := rng.Intn(len(countries))
	switch field {
	case "country":
		return countries[idx]
	case "countryCode":
		return codes[idx]
	case "timezone":
		return timezones[idx]
	case "latitude":
		return fmt.Sprintf("%.4f", (rng.Float64()*180.0)-90.0)
	case "longitude":
		return fmt.Sprintf("%.4f", (rng.Float64()*360.0)-180.0)
	}
	return countries[idx]
}

// fakerID generates various ID formats.
func fakerID(field string, rng *rand.Rand) string {
	switch field {
	case "objectId":
		return randomHex(rng, 24)
	case "nanoid":
		const nanoidChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"
		b := make([]byte, 21)
		for i := range b {
			b[i] = nanoidChars[rng.Intn(len(nanoidChars))]
		}
		return string(b)
	case "shortId":
		return randomString(rng, 8)
	}
	return uuid.New().String()
}

// fakerColor generates color fake values.
func fakerColor(field string, rng *rand.Rand) string {
	names := []string{"coral", "teal", "slate", "amber", "violet", "indigo", "rose", "emerald", "sky", "fuchsia"}
	switch field {
	case "hex":
		return fmt.Sprintf("#%s", randomHex(rng, 6))
	case "name":
		return names[rng.Intn(len(names))]
	case "rgb":
		return fmt.Sprintf("rgb(%d, %d, %d)", rng.Intn(256), rng.Intn(256), rng.Intn(256))
	}
	return fmt.Sprintf("#%s", randomHex(rng, 6))
}

// fakerNumber generates number fake values.
func fakerNumber(field string, rng *rand.Rand) string {
	switch field {
	case "int":
		return strconv.Itoa(rng.Intn(1000000))
	case "float":
		return fmt.Sprintf("%.2f", rng.Float64()*1000)
	}
	return strconv.Itoa(rng.Intn(1000000))
}

// fakerInternet extended with ip/ipv6/mac/port fields.
func fakerInternetExtended(field string, rng *rand.Rand) string {
	switch field {
	case "ip":
		return fmt.Sprintf("%d.%d.%d.%d", rng.Intn(256), rng.Intn(256), rng.Intn(256), rng.Intn(256))
	case "ipv6":
		return fmt.Sprintf("2001:%04x:%04x:%04x::%04x", rng.Intn(0x10000), rng.Intn(0x10000), rng.Intn(0x10000), rng.Intn(0x10000))
	case "mac":
		return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
			rng.Intn(256), rng.Intn(256), rng.Intn(256), rng.Intn(256), rng.Intn(256), rng.Intn(256))
	case "port":
		return strconv.Itoa(1024 + rng.Intn(64511))
	}
	return fakerInternet(field, rng)
}
