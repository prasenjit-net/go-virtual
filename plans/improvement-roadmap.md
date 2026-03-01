# Go-Virtual: Improvement Roadmap

**Generated:** post-v1.2.0  
**Status:** Proposed  
**Scope:** All layers — backend (Go), frontend (React/TypeScript), documentation

---

## Overview

This document catalogues all identified improvement areas discovered during a systematic codebase review. Items are grouped by theme, each rated by **Priority** (P1 = critical, P2 = important, P3 = nice-to-have) and **Effort** (S = small <1 day, M = medium 1–3 days, L = large 3–7 days, XL = extra-large >1 week).

---

## 1. Code Quality & Maintainability

### 1.1 Split the monolithic `handler.go` — P1 / M

**File:** `internal/api/handler.go` — currently **1849 lines**, containing every single API handler.

**Problem:** A file of this size is hard to navigate, review, and test in isolation. Related handlers (e.g. spec handlers, script handlers) are scattered across a flat namespace.

**Proposed split (by domain):**
```
internal/api/
├── handler.go          // Handler struct + constructor + shared helpers only (~80 lines)
├── handler_specs.go    // ListSpecs, CreateSpec, GetSpec, UpdateSpec, DeleteSpec,
│                       //   Enable/Disable, ToggleTracing, ExampleFallback,
│                       //   Backend/ProxyMode, Tags (~350 lines)
├── handler_responses.go // ListResponseConfigs, CreateResponseConfig, GetResponseConfig,
│                       //   UpdateResponseConfig, DeleteResponseConfig, UpdatePriority (~250 lines)
├── handler_scripts.go  // ListScripts, CreateScript, ValidateScript, GetScript,
│                       //   UpdateScript, DeleteScript, TestScript,
│                       //   ListScriptBindings, CreateScriptBinding, etc. (~300 lines)
├── handler_store.go    // ListStoreEntries, GetStoreEntry, UpsertStoreEntry,
│                       //   DeleteStoreEntry, ClearStore (~120 lines)
├── handler_sessions.go // ListSessions, GetSession, Invalidate/InvalidateAll (~120 lines)
├── handler_archives.go // ListArchives, CreateArchive, UploadArchive, GetArchive,
│                       //   DeleteArchive, DownloadArchive, RestoreArchive (~200 lines)
├── handler_system.go   // HealthCheck, Version, GetBranding, GetRoutes,
│                       //   Stats, ResetStats, Traces, ClearTraces,
│                       //   ValidateTemplate, Tags (~200 lines)
```

**Detail plan:** See [`handler-refactor.md`](handler-refactor.md).

---

### 1.2 Clean up dependency injection in `NewHandler` / `NewRouter` — P2 / S

**Problem:**
- `NewHandler` takes only 4 of 11 dependencies; the rest are injected via ad-hoc setters (`SetArchiveManager`, `SetStoreManager`, `SetBranding`).
- `NewRouter` takes a variadic `...config.BrandingConfig` — a code smell covering for a missing field.

**Proposed fix:**
```go
type HandlerConfig struct {
    Store          storage.Storage
    StatsCollector *stats.Collector
    TracingService *tracing.Service
    ProxyEngine    *proxy.Engine
    GlobalStore    *store.GlobalStore
    SessionManager *store.SessionManager
    ArchiveManager *archive.ArchiveManager
    Branding       config.BrandingConfig
    ScriptTimeout  int
}

func NewHandler(cfg HandlerConfig) *Handler { ... }
```

---

### 1.3 Cache compiled regex patterns in condition evaluator — P2 / S

**File:** `internal/condition/evaluator.go`  
**Problem:** Every call to `compare()` with operator `regex` calls `regexp.Compile(expected)` from scratch. A response with 5 conditions hitting at 100 req/s compiles the same pattern 500 times per second.

**Fix:** Add a simple `sync.Map`-backed LRU cache of `map[string]*regexp.Regexp` inside `Evaluator`. Patterns are immutable once created, so a read-through cache is safe.

---

### 1.4 Fix `rand.Rand` concurrency in template engine — P2 / S

**File:** `internal/template/engine.go`  
**Problem:** `engine.rng` is a single `*rand.Rand` shared across all goroutines. `rand.Rand` is **not** goroutine-safe. Concurrent requests calling `{{.random.int}}` / `{{.random.string}}` will race.

**Fix:** Either protect with a mutex, or replace with `rand.Int63()` (uses the global source, which is safe since Go 1.20), or create a new `rand.Rand` per render call seeded with `time.Now().UnixNano() ^ goroutine-id`.

---

### 1.5 Use the `TracingConfig.Retention` setting — P2 / S

**Files:** `internal/config/config.go`, `internal/tracing/service.go`  
**Problem:** `TracingConfig.Retention` (default 24h) is defined in config and documented, but `tracing.Service` only stores up to `maxTraces` records and never evicts by age. The retention field is wired nowhere after being parsed from YAML.

**Fix:** In `RecordTrace`, after appending, sweep entries older than `retention` from the front of the slice (they are recorded in order). Also wire the retention duration in `NewService(maxTraces, retention)`.

---

## 2. API Design & Storage

### 2.1 Add pagination to the Storage interface — P2 / L

**File:** `internal/storage/interface.go`  
**Problem:** `GetAllSpecs()`, `GetAllOperations()`, `GetAllScripts()` return every record unconditionally. With hundreds of specs and thousands of operations this will load everything into memory on each request.

**Proposed approach:** Introduce a `Page` type and paginated variants:
```go
type Page struct {
    Offset int
    Limit  int // 0 = all (backwards-compat default)
}

type PagedResult[T any] struct {
    Items  []T
    Total  int
    Offset int
    Limit  int
}

// New paginated methods (old methods stay for internal use)
ListSpecs(p Page) (*PagedResult[*models.Spec], error)
ListScripts(p Page) (*PagedResult[*models.Script], error)
```

API endpoints get `?page=0&limit=50` query params. The admin UI paginates lists.

**Detail plan:** See [`storage-pagination.md`](storage-pagination.md).

---

### 2.2 Add condition logic groups (AND / OR / NOT) — P2 / M

**File:** `internal/condition/evaluator.go`, `internal/models/condition.go`  
**Problem:** All conditions are evaluated with strict AND — "all conditions must match". There is no way to express "match if (A AND B) OR (C AND D)".

**Proposed model:**
```go
type ConditionGroup struct {
    Logic      string      `json:"logic"` // "and" | "or"
    Conditions []Condition `json:"conditions"`
    Groups     []ConditionGroup `json:"groups,omitempty"` // nested
}
```

Response configs would carry a `ConditionGroup` (defaulting to `{logic:"and", conditions:[...]}`) while remaining backwards-compatible with the flat slice via migration on first load.

**Detail plan:** See [`condition-logic-groups.md`](condition-logic-groups.md).

---

### 2.3 Add `ContentType` field to `ResponseConfig` — P2 / S

**File:** `internal/models/response.go`  
**Problem:** There is no explicit `ContentType` field; users must set `Content-Type` inside the headers map. This is error-prone (case sensitivity), and there is no first-class way to render the content type in the UI.

**Fix:**
```go
type ResponseConfig struct {
    ...
    ContentType string            `json:"contentType,omitempty"` // shorthand; populates Content-Type header
    Headers     map[string]string `json:"headers,omitempty"`
}
```

When rendering: if `ContentType` is set, it takes precedence over the `Content-Type` entry in `Headers`.

---

### 2.4 Add a `POST /_api/specs/:id/reload` endpoint — P3 / S

**Problem:** Currently re-uploading a spec means deleting and recreating it (losing all response configs). An in-place reload endpoint would re-parse the YAML/JSON content, reconcile operations (add new, remove deleted), and keep existing response configs on unchanged operations.

---

## 3. Security

### 3.1 Admin API authentication middleware — P2 / M

**File:** `internal/api/router.go`  
**Problem:** The entire `/_api/*` surface is completely open. Anyone who can reach the server can read/modify all specs, scripts, store entries, and sessions.

**Proposed approach:** Optional API-key authentication — disabled by default for backwards compatibility:
```yaml
# config.yaml
auth:
  enabled: true
  apiKeys:
    - name: "ci-key"
      key: "sk_xxxxxxxxxxxx"
      scopes: ["read", "write"]   # future: fine-grained
```

Middleware checks `Authorization: Bearer <key>` or `X-Api-Key: <key>` on all `/_api/*` requests. The UI stores the key in `localStorage` and sends it on every request.

**Detail plan:** See [`auth-middleware.md`](auth-middleware.md).

---

### 3.2 Configure CORS properly — P2 / S

**File:** `internal/api/router.go` (look for `corsMiddleware`)  
**Problem:** The current CORS middleware is permissive (allow-all origins). This is fine for local dev but is a security risk for shared / cloud deployments.

**Fix:** Add a `cors.allowedOrigins` config key (default `["*"]`). The middleware reads from config.

---

### 3.3 Request body size limit — P2 / S

**Problem:** Incoming proxy requests and admin API payloads are read without size limit:
- `io.ReadAll(r.Body)` in `proxy/engine.go` — could exhaust RAM with a large upload.
- `c.ShouldBindJSON(...)` in handlers — no max size set.

**Fix:** In `runServe`, wrap the HTTP server with `http.MaxBytesReader`. Add a config key `server.maxRequestBodyBytes` (default 10 MB for proxy, 5 MB for admin payloads).

---

## 4. Observability

### 4.1 Persistent trace storage — P3 / L

**File:** `internal/tracing/service.go`  
**Problem:** All traces are in-memory. Restarting the server loses all trace history. High-throughput specs fill the ring buffer quickly.

**Proposed approach:** Introduce a `TraceStore` interface:
```go
type TraceStore interface {
    Append(trace *models.Trace) error
    Query(filter *models.TraceFilter) ([]*models.Trace, error)
    Count(filter *models.TraceFilter) (int, error)
    Delete(filter *models.TraceFilter) error
    Close() error
}
```

Implementations:
- `MemoryTraceStore` — current in-memory ring buffer (default).
- `SQLiteTraceStore` — persistent SQLite-backed store (opt-in via config).

Config:
```yaml
tracing:
  storage: "memory"  # or "sqlite"
  sqlitePath: "./data/traces.db"
  maxTraces: 10000
  retention: 72h
```

**Detail plan:** See [`trace-persistence.md`](trace-persistence.md).

---

### 4.2 Structured request logging — P3 / S

**Problem:** The server currently uses `gin.Logger()` which outputs to stdout in a human-readable format. There's a `LoggingConfig` struct with `level` and `format` fields but they are parsed and then ignored — no structured logger is wired.

**Fix:** Wire `slog` (stdlib since Go 1.21) or `zerolog` as the global logger, reading level/format from `LoggingConfig`. Replace all `log.Println` calls throughout `cmd/server/`.

---

### 4.3 Expose `/health` with detailed component checks — P3 / S

**Problem:** `GET /_api/health` returns `{"status":"ok"}` unconditionally. It doesn't check whether storage is readable, sessions are functional, or scripts can compile.

**Proposed response:**
```json
{
  "status": "ok",
  "version": "1.2.0",
  "components": {
    "storage": "ok",
    "tracing": "ok",
    "scripting": "ok"
  },
  "uptime": "4h32m"
}
```

---

## 5. Scripting & Templates

### 5.1 Additional Starlark builtins — P2 / M

**File:** `internal/scripting/builtins.go`  
**Problem:** Scripts currently have access to `store` and `log`. Several common use-cases require extra builtins that are safe, deterministic, and sandboxed.

**Proposed additions:**

| Builtin | Signature | Purpose |
|---|---|---|
| `http.get(url)` | `(url: str) → {status, body, headers}` | Read-only external HTTP call (timeout-bounded) |
| `json.encode(v)` | `(v) → str` | Starlark value → JSON string |
| `json.decode(s)` | `(s: str) → value` | JSON string → Starlark value |
| `base64.encode(s)` | `(s: str) → str` | Base64-encode a string |
| `base64.decode(s)` | `(s: str) → str` | Base64-decode |
| `hash.md5(s)` / `hash.sha256(s)` | `(s: str) → str` | Digest strings |
| `env(key)` | `(key: str) → str` | Read environment variable (whitelist-only) |
| `uuid()` | `() → str` | Generate a UUID v4 |
| `now()` | `() → int` | Unix timestamp (seconds) |

Each builtin should be independently togglable via config (`scripting.disabledBuiltins: ["http.get"]`).

**Detail plan:** See [`scripting-builtins-roadmap.md`](../docs/scripting-builtins-roadmap.md) (existing).

---

### 5.2 Script output schema / type hints — P3 / S

**Problem:** Scripts return arbitrary Starlark values. Template authors have no visibility into what keys are available under `{{.Script.myBinding.*}}` without running the script.

**Fix:** Add an optional `outputSchema` JSON field to `Script` model:
```json
{ "outputSchema": { "total": "number", "items": "array" } }
```
This is for documentation purposes — displayed in the template editor's autocomplete.

---

### 5.3 Unify template syntax — P3 / M

**File:** `internal/template/engine.go`  
**Problem:** Two overlapping syntaxes exist:
1. Legacy `{{varName}}` — simple regex-replace approach.
2. Go `text/template` — full Go template language (conditionals, loops, pipes).

Both are active simultaneously (legacy patterns are preprocessed before being passed to `text/template`). This creates ambiguity and the preprocessor can corrupt legitimate Go template syntax.

**Fix:** Deprecate the legacy `{{varName}}` syntax with a migration guide and auto-migrator. All templates should use the Go `text/template` syntax exclusively (`{{.path.id}}`, `{{.query.limit}}`). Provide a one-time migration tool: `go-virtual migrate templates`.

---

## 6. Proxy & Recording

### 6.1 Selective proxy recording (match on conditions) — P3 / M

**Problem:** Recording mode captures every response from the backend. There is no way to record only responses that match certain conditions (e.g. only record 2xx responses, or only specific paths).

**Fix:** Add a `RecordingFilter` to `Spec`:
```go
type RecordingFilter struct {
    StatusCodes   []int  `json:"statusCodes"`   // e.g. [200, 201]
    PathPattern   string `json:"pathPattern"`   // regex
    MinDurationMs int    `json:"minDurationMs"` // only slow responses
}
```

---

### 6.2 Response chaining — P3 / L

**Problem:** There is no way to make a virtualised response reference the output of another operation (e.g. create returns an ID, then get uses that ID).

**Proposed approach:** Allow response body / headers to reference `{{.session.store.<key>}}` directly (already possible via Starlark scripts, but has no first-class UI). Document this pattern clearly and add a UI hint.

This is partially addressed by the session store — the main gap is discoverability and documentation.

---

### 6.3 Multi-spec request fallthrough — P3 / M

**Problem:** When two specs define overlapping paths, the first match wins with no fallthrough. There is no way to try the next spec on a miss.

**Fix:** Add a `spec.Priority` field (int, lower = higher precedence). When `fallthrough: true` is set on a spec, a 404 from matching causes the engine to try the next-lower-priority spec.

---

## 7. Archive & Portability

### 7.1 Partial archive restore — P2 / S

**Problem:** Restoring an archive is all-or-nothing. There is no way to restore only selected specs, scripts, or tags from a larger archive.

**Fix:** Extend the `POST /_api/archives/:id/restore` body:
```json
{
  "specIds": ["abc123"],
  "scriptIds": ["def456"],
  "restoreTags": true,
  "restoreStore": false,
  "conflictStrategy": "skip" | "overwrite" | "rename"
}
```

A `conflictStrategy` parameter controls what happens when a spec with the same name already exists: skip (default), overwrite, or rename with a `(copy)` suffix.

---

### 7.2 Archive format versioning — P2 / S

**File:** `internal/archive/manifest.go`  
**Problem:** The archive manifest tracks `appVersion` but not a dedicated `formatVersion`. If the archive format changes (e.g. new fields added) there is no clean migration path.

**Fix:** Add `FormatVersion int` to `Manifest`. Start at `1`. Reader checks format version and applies migrations before loading. Bumping format version should happen when the binary layout changes, independent of app release.

---

## 8. Developer Experience

### 8.1 OpenAPI spec validation against configured responses — P3 / M

**Problem:** When users configure response bodies, there is no check that the body is valid according to the spec's schema for that operation/status code. Invalid JSON or mismatched schemas produce confusing behaviour at runtime.

**Fix:** After a response config is saved, optionally validate the static body against the OpenAPI schema using `kin-openapi`'s response validator. Surface warnings in the UI without blocking save.

---

### 8.2 Config hot-reload — P3 / M

**Problem:** Changing `config.yaml` requires a server restart. Branding, tracing settings, session timeout, and scripting timeout are all static after startup.

**Fix:** Watch `config.yaml` with `fsnotify`. On change, reload non-structural settings (branding, tracing limits, session timeout, scripting timeout, logging level). Structural settings that require a restart (port, storage backend, TLS) are flagged with a warning log but not re-applied.

---

### 8.3 CLI `validate` sub-command — P3 / S

**Problem:** There is no way to validate a config file or OpenAPI spec without starting the server.

**Fix:**
```bash
go-virtual validate config.yaml
go-virtual validate spec --file petstore.yaml
```

Exits 0 on success, 1 with errors printed.

---

### 8.4 Webhook / event notifications — P3 / L

**Problem:** There is no way for external systems to react to events (request received, script error, session created, spec enabled/disabled).

**Proposed:** Add an optional `webhooks` config section:
```yaml
webhooks:
  - url: "https://ci-server/hooks/virtual"
    events: ["request.matched", "script.error", "spec.enabled"]
    secret: "hmac-secret"
    timeoutSeconds: 5
```

Events are dispatched asynchronously. Delivery failures are logged and do not affect request handling.

---

## 9. Testing

### 9.1 Integration test suite — P2 / L

**Problem:** Tests are unit-level only. There are no end-to-end tests that start the server, upload a spec, configure responses, and make requests.

**Fix:** Add `test/integration/` with table-driven integration tests using `httptest.NewServer`. Cover at minimum:
- Upload spec → list operations → configure response → make request → check response.
- Condition matching (eq, regex, header).
- Script execution with store access.
- Proxy recording mode.
- Archive create/restore round-trip.

---

### 9.2 Benchmark suite for condition evaluator and template engine — P3 / S

**Problem:** No benchmarks exist. Changes to hot-path code (condition evaluator, template engine, route matching) cannot be measured for regression.

**Fix:** Add `_test.go` benchmark functions in `internal/condition/`, `internal/template/`, `internal/proxy/` using `testing.B`.

---

## Priority Summary

| # | Item | Priority | Effort |
|---|---|---|---|
| 1.1 | Split handler.go | P1 | M |
| 1.3 | Cache regex in condition evaluator | P2 | S |
| 1.4 | Fix rand.Rand race in template engine | P2 | S |
| 1.5 | Wire TracingConfig.Retention | P2 | S |
| 1.2 | Clean up DI in NewHandler | P2 | S |
| 2.1 | Storage pagination | P2 | L |
| 2.2 | Condition AND/OR/NOT groups | P2 | M |
| 2.3 | ContentType field on ResponseConfig | P2 | S |
| 3.1 | Admin API auth middleware | P2 | M |
| 3.2 | Configurable CORS | P2 | S |
| 3.3 | Request body size limit | P2 | S |
| 5.1 | Additional Starlark builtins | P2 | M |
| 7.1 | Partial archive restore | P2 | S |
| 7.2 | Archive format versioning | P2 | S |
| 9.1 | Integration test suite | P2 | L |
| 2.4 | Spec in-place reload endpoint | P3 | S |
| 3.3 | Detailed health check | P3 | S |
| 4.1 | Persistent trace storage | P3 | L |
| 4.2 | Structured request logging | P3 | S |
| 5.2 | Script output schema hints | P3 | S |
| 5.3 | Unify template syntax | P3 | M |
| 6.1 | Selective proxy recording | P3 | M |
| 6.2 | Response chaining (docs) | P3 | S |
| 6.3 | Multi-spec fallthrough | P3 | M |
| 8.1 | OpenAPI response validation | P3 | M |
| 8.2 | Config hot-reload | P3 | M |
| 8.3 | CLI validate sub-command | P3 | S |
| 8.4 | Webhook event notifications | P3 | L |
| 9.2 | Benchmark suite | P3 | S |

---

## Suggested Release Plan

### v1.3.0 — Code health & security
- 1.1 Split handler.go
- 1.2 DI cleanup
- 1.3 Cache regex patterns
- 1.4 Fix rand.Rand race
- 1.5 Wire retention config
- 2.3 ContentType field
- 3.2 Configurable CORS
- 3.3 Request body size limit
- 7.2 Archive format versioning

### v1.4.0 — Auth & pagination
- 3.1 Admin API auth middleware
- 2.1 Storage pagination (admin list endpoints)
- 4.2 Structured logging

### v1.5.0 — Richer conditions & scripting
- 2.2 Condition AND/OR/NOT groups
- 5.1 Additional Starlark builtins (json, base64, uuid, now)
- 7.1 Partial archive restore

### v2.0.0 — Schema & persistence (breaking changes)
- 5.3 Unify template syntax (remove legacy `{{varName}}`)
- 4.1 Persistent trace storage
- 2.1 Full pagination (breaking API change if not done via query params)
- 9.1 Integration test suite
