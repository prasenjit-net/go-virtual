# Scripting Support — Phase 1: Core Starlark Engine

**Date:** 2026-02-22  
**Target Version:** v0.8.0  
**Status:** Draft / Planning  
**See also:** [scripting-phase2-plan.md](scripting-phase2-plan.md) — Session Store

---

## 1. Overview

Scripting support allows users to attach lightweight, sandboxed **Starlark** scripts to any API operation. When a matching request arrives, the attached scripts are executed with the full request context and their output becomes available as additional template variables in the response body and headers.

This bridges the gap between static templating (which can only echo back request data or generate random values) and fully dynamic behaviour — without requiring users to write a real backend.

### Motivating Use Cases

| Scenario | How scripting helps |
|---|---|
| Compute a derived field (e.g. total from quantity × price) | Script reads body fields, returns computed value |
| Conditional field masking | Script inspects auth header, strips fields from cloned body |
| Custom ID generation (prefixed, formatted) | Script builds `USR-0042` from path/body data |
| Complex conditional logic | Script expresses `if/elif/else` chains too complex for conditions |
| Header-driven content shaping | Script inspects `Accept` / `X-*` headers and returns different shapes |

> **State-dependent responses** (call counters, sequential IDs, per-session data) are covered by the Session Store in [Phase 2](scripting-phase2-plan.md). Phase 1 scripts are stateless.

---

## 2. Script Resource

Scripts are a **first-class managed resource** in go-virtual, alongside Specs, Operations, and Response Configs.

### 2.1 Data Model

Each script is stored as two files (see §7). The `Source` field is loaded from the companion `.star` file and is intentionally excluded from the JSON metadata:

```go
// Script represents a user-defined Starlark script resource.
type Script struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Timeout     int       `json:"timeout"`   // Max execution time in ms (default: 100)
    Enabled     bool      `json:"enabled"`
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
    // Source is loaded from <id>.star; omitted from JSON serialisation.
    Source      string    `json:"-"`
}

// ScriptInput is used for create/update API calls.
// Source is written to the companion .star file on save.
type ScriptInput struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Source      string `json:"source"`  // Starlark source code
    Timeout     int    `json:"timeout"`
    Enabled     bool   `json:"enabled"`
}
```

### 2.2 Script–Operation Binding

Scripts are attached to operations via a binding resource, which allows ordering and independent enable/disable:

```go
// ScriptBinding attaches a Script to an Operation.
type ScriptBinding struct {
    ID          string `json:"id"`
    OperationID string `json:"operationId"`
    ScriptID    string `json:"scriptId"`
    ScriptName  string `json:"scriptName"`  // Denormalised for display
    OutputKey   string `json:"outputKey"`   // Namespace under {{.script.*}}
    Order       int    `json:"order"`       // Execution order, ascending
    Enabled     bool   `json:"enabled"`
}
```

One operation can have **multiple bindings**, executed in ascending `Order`. Each binding's output is namespaced under `.script.<outputKey>` in the template context. Scripts are **global resources** — the same script can be bound to many operations.

---

## 3. Scripting Language — Starlark

The sole scripting language is **Starlark** ([go.starlark.net/starlark](https://pkg.go.dev/go.starlark.net/starlark)), a Python-like deterministic language designed for embedding:

- **Pure Go** — no CGo, no subprocess; embeds as a single dependency.
- **Sandboxed** — no file I/O, no network, no OS access by design.
- **Deterministic** — no global mutable state in the language itself.
- **Familiar** — Python-like syntax: `def`, `if/elif/else`, `for`, list/dict comprehensions.
- **Safe termination** — supports execution step limits to bound infinite loops.

### 3.1 Script Contract

Every script must define a top-level function named `run` that accepts one dict argument (the request context) and returns any JSON-serialisable value (string, int, float, bool, list, or dict):

```python
# Minimal script
def run(req):
    return "Hello from Starlark"
```

```python
# Compute order totals from request body
def run(req):
    body       = req["body"]
    qty        = body.get("quantity", 0)
    unit_price = body.get("unitPrice", 0.0)
    total      = qty * unit_price
    return {
        "total":        total,
        "taxed":        total * 1.2,
        "freeShipping": total > 50,
        "summary":      "Order for " + body.get("productName", "unknown"),
    }
```

```python
# Shape response based on headers
def run(req):
    auth  = req["header"].get("authorization", "")
    token = auth.removeprefix("Bearer ").strip()
    return {
        "hasToken": len(token) > 0,
        "userId":   req["path"].get("id", "anonymous"),
        "greeting": "Hello, " + req["header"].get("x-user-name", "guest") + "!",
    }
```

```python
# Validate and transform list data
def run(req):
    items = req["body"].get("items", [])
    valid = [i for i in items if i.get("price", 0) > 0]
    return {
        "itemCount":    len(valid),
        "total":        sum(i["price"] * i.get("qty", 1) for i in valid),
        "invalidCount": len(items) - len(valid),
    }
```

### 3.2 Why Starlark

| Property | Detail |
|---|---|
| **Safety** | No I/O builtins; cannot access host filesystem or network |
| **Termination** | `starlark.Thread.SetMaxSteps` bounds CPU; goroutine timeout bounds wall time |
| **Embeddability** | Pure Go, zero CGo, single `go.starlark.net` module |
| **Expressiveness** | Full imperative scripting — conditionals, loops, helper functions, list/dict ops |
| **Thread safety** | Each execution creates a new `*starlark.Thread`; safe for concurrent requests |

---

## 4. Execution Context

Each script execution receives a **read-only `req` dict** populated from the current request:

```python
req = {
    "path":   { "userId": "42" },
    "query":  { "format": "json", "page": "2" },
    "header": { "authorization": "Bearer abc", "content-type": "application/json" },
    "body":   { "name": "Alice", "age": 30 }
}
```

| Field | Type | Description |
|---|---|---|
| `path` | `dict[str, str]` | Path parameters |
| `query` | `dict[str, str]` | First value of each query parameter |
| `header` | `dict[str, str]` | First value of each header, lower-cased keys |
| `body` | `dict` or `str` | Parsed JSON body as dict; falls back to raw string if not valid JSON |

Scripts **cannot**:
- Access the file system, network, or OS
- Spawn goroutines or threads
- Import Go packages
- Access global Go state or other requests' data

Scripts **can**:
- Read all `req` fields
- Perform arithmetic, string operations, list/dict manipulation
- Call helper functions defined in the same script
- Return any JSON-serialisable value

### 4.1 Execution Pipeline per Request

```
Incoming Request
      │
      ▼
 Route Match  (existing)
      │
      ▼
 Script Engine
  ┌─────────────────────────────────────────────┐
  │  For each enabled ScriptBinding (by Order): │
  │                                             │
  │  1. Check compiled-script cache             │
  │     └─ miss → compile + cache               │
  │  2. Build req dict from request data        │
  │  3. Execute run(req) with step + time limit │
  │  4. Convert Starlark value → Go any         │
  │  5. Store result under binding.OutputKey    │
  └─────────────────────────────────────────────┘
      │
      ▼
 Script Output  { "pricing": {...}, "auth": {...} }
      │
      ▼
 Condition Evaluation  (existing — script output NOT available here)
      │
      ▼
 Template Engine  (existing + {{.script.*}} injected)
      │
      ▼
 Response to Client
```

### 4.2 Error Handling

| Situation | Behaviour |
|---|---|
| Execution succeeds | Output stored under `outputKey`, available to template engine |
| Script returns an error value | Treated as execution error (see below) |
| Runtime error / panic | `recover()` catches it; `{{.script.<key>}}` resolves to empty; error recorded in trace |
| Step limit / timeout exceeded | Treated as execution error; response still returned normally |
| Compile error at save time | API returns `422 Unprocessable Entity` with line/column details; script not saved |

Scripts never block the response. A failing script degrades gracefully to empty output rather than a 500.

---

## 5. Template Integration

Script outputs are available under the `.script` namespace in the existing template engine. Given two bindings with `outputKey` values `pricing` and `auth`:

```
{{.script.pricing.total}}           → number as string
{{.script.pricing.freeShipping}}    → bool as string
{{.script.auth.userId}}             → string
{{.script.auth.hasToken}}           → bool as string
```

Deep dot-path access works for arbitrarily nested dict outputs:

```json
{
  "orderId":    "{{.random.uuid}}",
  "total":      "{{.script.pricing.total}}",
  "taxed":      "{{.script.pricing.taxed}}",
  "user":       "{{.script.auth.userId}}",
  "greeting":   "{{.script.greeting}}",
  "eligible":   "{{.script.pricing.freeShipping}}"
}
```

**Return type behaviour:**

| Script return type | Template access |
|---|---|
| `str` / `int` / `float` / `bool` | `{{.script.key}}` → value as string |
| `dict` | `{{.script.key.field}}` → nested field |
| `list` | `{{.script.key}}` → JSON array string, e.g. `["a","b"]` |
| `None` / error | `{{.script.key}}` → empty string |

---

## 6. API Endpoints

All endpoints are under the existing `/_api` group.

### 6.1 Script CRUD

| Method | Path | Description |
|---|---|---|
| `GET` | `/_api/scripts` | List all scripts (metadata only, no source) |
| `POST` | `/_api/scripts` | Create a new script |
| `GET` | `/_api/scripts/:id` | Get a script (includes source) |
| `PUT` | `/_api/scripts/:id` | Update a script (metadata and/or source) |
| `DELETE` | `/_api/scripts/:id` | Delete script + all its bindings |
| `POST` | `/_api/scripts/validate` | Validate Starlark source without saving |
| `POST` | `/_api/scripts/:id/test` | Execute script with a sample input and return output |

### 6.2 Script–Operation Bindings

| Method | Path | Description |
|---|---|---|
| `GET` | `/_api/operations/:id/scripts` | List bindings for an operation |
| `POST` | `/_api/operations/:id/scripts` | Attach a script to an operation |
| `PUT` | `/_api/operations/:id/scripts/:bindingId` | Update binding (outputKey, order, enabled) |
| `DELETE` | `/_api/operations/:id/scripts/:bindingId` | Detach script from operation |
| `PUT` | `/_api/operations/:id/scripts/reorder` | Bulk reorder bindings |

### 6.3 Validate & Test Payloads

**POST `/_api/scripts/validate`** — validates source without creating a script:

```json
// Request
{ "source": "def run(req):\n  return req[\"path\"][\"id\"]" }

// Response (success)
{ "valid": true, "error": null }

// Response (error)
{ "valid": false, "error": "line 2: undefined: req2" }
```

**POST `/_api/scripts/:id/test`** — executes the saved script with a mock input:

```json
// Request
{
  "input": {
    "path":   { "id": "42" },
    "query":  { "format": "json" },
    "header": { "x-user-name": "Alice" },
    "body":   { "quantity": 3, "unitPrice": 12.50 }
  }
}

// Response
{
  "output":     { "total": 37.50, "taxed": 45.0, "summary": "Order for Widget" },
  "durationMs": 0.4,
  "error":      null
}
```

---

## 7. Storage

Scripts follow the same storage pattern as existing resources, with one addition: source code is stored in a separate file.

### 7.1 File Layout

Each script is stored as **two files** — metadata and source are intentionally separated so the source can be edited directly on disk, diffed in version control, and to avoid embedding large text inside JSON:

```
data/
  scripts/
    <script-id>.json            # Metadata: name, description, timeout, enabled, timestamps
    <script-id>.star            # Starlark source code (plain text, UTF-8)
  operations/
    <operation-id>.json         # Existing: operation customisations
    <operation-id>.scripts.json # New: ordered list of ScriptBindings for this operation
```

**Load semantics:** the `.json` file is decoded first; then the companion `.star` file is read and assigned to `Script.Source`. Both files are written atomically on create/update. Deleting a script removes both files. A missing `.star` file at load time is treated as empty source with a warning log.

### 7.2 Storage Interface Extension

```go
// New methods on the storage.Storage interface:

// Script CRUD
CreateScript(script *models.Script) error
GetScript(id string) (*models.Script, error)
GetAllScripts() ([]*models.Script, error)
UpdateScript(script *models.Script) error
DeleteScript(id string) error

// Script–Operation bindings
GetScriptBindings(operationID string) ([]*models.ScriptBinding, error)
CreateScriptBinding(binding *models.ScriptBinding) error
UpdateScriptBinding(binding *models.ScriptBinding) error
DeleteScriptBinding(id string) error
DeleteScriptBindingsByScript(scriptID string) error  // Cascade on script delete
```

---

## 8. Admin UI

### 8.1 Scripts Page — `/scripts`

New top-level sidebar page.

**Script List View**
- Table: Name, Description, Timeout, Enabled toggle, Updated, Actions (Edit / Delete)
- Toolbar: "New Script" button, search by name
- Badge showing number of operations each script is bound to
- Empty state with example prompt

**Script Create / Edit View**
- Fields: Name, Description, Timeout (ms), Enabled toggle
- Full-height **Monaco Editor** — language mode `python` (best available match for Starlark syntax highlighting)
- **Validate** button — calls `POST /_api/scripts/validate` with current source; shows inline error with line number
- **Test** panel (collapsible, below editor):
  - JSON editor for mock `path`, `query`, `header`, `body`
  - **Run** button — calls `POST /_api/scripts/:id/test`
  - Output panel: formatted JSON result or error message with execution time
- Save / Cancel

### 8.2 Operation Detail — Script Bindings Section

The existing Operation Detail page gains a **Scripts** section below Response Configs.

**Binding List**
- Drag-to-reorder (via `@dnd-kit`, consistent with response priority ordering)
- Each row: Order badge, Script name (link to edit), Output Key (`{{.script.key}}`), Enabled toggle, Remove button
- "Attach Script" button

**Attach Script Modal**
- Searchable dropdown of all scripts (name + description)
- Output Key input — defaults to slugified script name; shown as `{{.script.<outputKey>}}`
- Preview of template variables that will be available once bound
- Attach / Cancel

### 8.3 Response Designer — Script Variables Reference

The Response Config editor's template helper sidebar gains a **Script Variables** section listing the output keys of scripts bound to the current operation:

```
Available script variables for this operation:

  {{.script.pricing}}           full output dict
  {{.script.pricing.total}}     nested field
  {{.script.auth}}              full output dict
  {{.script.auth.userId}}       nested field
```

---

## 9. Internal Package Structure

```
internal/
  scripting/
    engine.go    # ScriptEngine: resolve bindings, run all scripts for an operation
    runner.go    # Starlark Runner: Compile() → CompiledScript, Execute()
    cache.go     # Compiled-script cache (keyed by scriptID + updatedAt)
    context.go   # Build Starlark req dict from pathParams + *http.Request
    convert.go   # starlark.Value → Go any (for template injection)
```

### 9.1 Core Interfaces

```go
// Runner compiles Starlark source into reusable programs.
type Runner interface {
    Compile(source string) (CompiledScript, error)
}

// CompiledScript is an immutable compiled Starlark program.
// It is safe to cache and Execute concurrently from multiple goroutines.
type CompiledScript interface {
    // Execute calls run(req) with the given input, bounded by timeoutMs.
    // Returns a Go-native value (string, float64, bool, map[string]any, []any)
    // or an error.
    Execute(input ScriptInput, timeoutMs int) (any, error)
}

// ScriptInput is the read-only request context passed to each script as req.
type ScriptInput struct {
    Path   map[string]string // Path parameters
    Query  map[string]string // First value of each query parameter
    Header map[string]string // First value of each header, lowercased
    Body   any               // Parsed JSON (map[string]any) or raw string
}
```

### 9.2 Compilation Cache

Compiling Starlark source on every request is wasteful. An in-memory cache avoids recompilation while staying correct across updates:

```go
// compiledCache maps scriptID → { updatedAt, CompiledScript }.
// A script update changes updatedAt → cache miss → recompile automatically.
type compiledCache struct {
    mu    sync.RWMutex
    store map[string]cacheEntry
}

type cacheEntry struct {
    updatedAt time.Time
    compiled  CompiledScript
}
```

`ScriptEngine.RunBindings` checks the cache before calling `runner.Compile`. Cache hits are the common path for every request once a script has been executed at least once.

The cache interface is designed to accommodate Phase 2's `store` builtin injection without cache structure changes — only the `Execute` signature will gain an optional session store parameter.

### 9.3 Integration Point in Proxy Engine

```go
// In engine.go ServeHTTP, after route match, before response selection:

scriptOutput, scriptTraces := e.scriptEngine.RunBindings(
    r.Context(),
    matchedRoute.operation.ID,
    &scripting.ScriptInput{
        Path:   pathParams,
        Query:  firstValues(r.URL.Query()),
        Header: firstValues(r.Header),
        Body:   parseBody(requestBody),
    },
)
// scriptOutput: map[string]any keyed by outputKey
// scriptTraces: []ScriptTrace for trace record

tmplCtx := &template.Context{
    PathParams:   pathParams,
    QueryParams:  r.URL.Query(),
    Headers:      r.Header,
    Body:         requestBody,
    ScriptOutput: scriptOutput,  // NEW field
}
```

---

## 10. Security Considerations

| Concern | Mitigation |
|---|---|
| Infinite loops / CPU exhaustion | `starlark.Thread.SetMaxSteps(N)` — hard step limit per execution |
| Wall-time overrun | Goroutine + `context.WithTimeout` cancels execution at `script.Timeout` ms |
| Memory exhaustion | Starlark value allocations are bounded by step limit; output size capped at 64 KB |
| Panic in script | `recover()` in `Execute()`; panic converted to error, never propagates to request handler |
| Access to Go internals | Starlark has no `import` for host packages; no `reflect` / `unsafe` exposure |
| Script source injection | Source stored verbatim in `.star` file; never interpolated or passed through `eval` |

---

## 11. Trace Integration

When tracing is enabled for a spec, each trace record is extended with a `scripts` array:

```json
"scripts": [
  {
    "bindingId":  "abc123",
    "scriptId":   "def456",
    "scriptName": "Pricing Calculator",
    "outputKey":  "pricing",
    "durationMs": 0.38,
    "output":     { "total": 37.50, "taxed": 45.0 },
    "error":      null
  },
  {
    "bindingId":  "bcd234",
    "scriptId":   "efa567",
    "scriptName": "Auth Inspector",
    "outputKey":  "auth",
    "durationMs": 0.12,
    "output":     { "hasToken": true, "userId": "42" },
    "error":      null
  }
]
```

The Trace Viewer in the Admin UI shows this alongside request/response data, making it easy to debug what each script produced for a given request.

---

## 12. Implementation Checklist (v0.8.0)

1. `models.Script`, `models.ScriptBinding` — source excluded from JSON
2. `storage.Storage` interface: script CRUD + binding CRUD methods
3. `internal/storage/file.go` — two-file (`.json` + `.star`) read/write/delete
4. `internal/storage/memory.go` — in-memory implementations for tests
5. `internal/scripting` — `Runner`, `CompiledScript`, `compiledCache`, `ScriptInput`, context builder, value converter
6. `internal/api/handler.go` — script CRUD handlers, binding handlers, `/validate`, `/test`
7. `internal/api/router.go` — register script + binding routes
8. `internal/proxy/engine.go` — call `scriptEngine.RunBindings` after route match
9. `internal/template/engine.go` — add `ScriptOutput` to `Context`, resolve `{{.script.*}}`
10. `internal/models/trace.go` — add `Scripts []ScriptTrace` to `Trace`
11. Admin UI — Scripts list page, Monaco editor, Test panel, Operation binding section
12. Admin UI — Response Designer script variable hints
13. `config.yaml` — `scripting.defaultTimeoutMs` (default: 100)
14. Tests — unit: runner, cache, value converter; integration: binding execution, handler tests

---

## 13. Decisions

| # | Decision |
|---|---|
| Language | **Starlark only** — single language, no selector in UI |
| Script scope | **App-wide** — scripts are global resources, reused across operations via bindings |
| Script output as conditions | **No** — output is only available to the template engine after condition evaluation |
| Compilation cache | **Yes** — in-memory cache keyed by `scriptID + updatedAt`; invalidated on update |
| Timeout | **Per-script with global default** — `config.yaml` `scripting.defaultTimeoutMs`; each script can override |
| Stateful store | **Phase 2** — see [scripting-phase2-plan.md](scripting-phase2-plan.md) |
