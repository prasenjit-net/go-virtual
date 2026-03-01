# Plan: Split `handler.go` into Domain-Focused Files

**Priority:** P1  
**Effort:** M (1–2 days)  
**Target release:** v1.3.0  
**Related:** [improvement-roadmap.md § 1.1](improvement-roadmap.md)

---

## Problem

`internal/api/handler.go` is **1849 lines** containing all API handler methods for every domain (specs, operations, responses, scripts, bindings, tags, store, sessions, archives, stats, tracing, system). This makes it:

- **Hard to navigate** — finding any single handler requires scrolling or searching.
- **Hard to review** — PRs touching one domain produce large diffs.
- **Hard to test in isolation** — test file mirrors the same problem (`handler_test.go`).
- **A merge conflict magnet** — multiple features touching the same file simultaneously.

The `Handler` struct itself is also partially constructed — four dependencies are in `NewHandler`, but three more are injected later via setters (`SetBranding`, `SetArchiveManager`, `SetStoreManager`). This makes the dependency graph hard to reason about.

---

## Goal

1. Split handler methods into **8 focused files**, grouped by API domain.
2. Consolidate dependency injection into a single `HandlerConfig` struct passed to `NewHandler`.
3. Split `handler_test.go` to mirror the new file structure.
4. Zero behaviour changes — all routes, status codes, and response shapes stay the same.

---

## File Layout After Refactor

```
internal/api/
├── handler.go              // Handler struct, HandlerConfig, NewHandler, shared helpers
├── handler_specs.go        // Spec CRUD + Enable/Disable/Tracing/ExampleFallback/Backend/ProxyMode/Tags
├── handler_operations.go   // ListOperations, GetOperation, GetSignatureConfig, UpdateSignatureConfig
├── handler_responses.go    // Response config CRUD + UpdatePriority
├── handler_scripts.go      // Script CRUD + ValidateScript + TestScript
├── handler_bindings.go     // Script binding CRUD + ReorderScriptBindings
├── handler_store.go        // Global store endpoints
├── handler_sessions.go     // Session list + get + invalidate
├── handler_archives.go     // Archive CRUD + upload + download + restore
├── handler_system.go       // Health, Version, Branding, Routes, Stats, Traces, Tags, Templates
```

Corresponding test files:
```
handler_test.go             // shared test helpers, test server setup
handler_specs_test.go
handler_responses_test.go
handler_scripts_test.go
handler_store_test.go
handler_sessions_test.go
handler_archives_test.go
handler_system_test.go
```

---

## Struct & Constructor Changes

### Current

```go
type Handler struct {
    store          storage.Storage
    statsCollector *stats.Collector
    tracingService *tracing.Service
    proxyEngine    *proxy.Engine
    parser         *parser.Parser         // created internally
    templateEngine *template.Engine       // created internally
    scriptEngine   *scripting.ScriptEngine // created internally
    globalStore    *store.GlobalStore      // set via SetStoreManager
    sessionManager *store.SessionManager   // set via SetStoreManager
    archiveManager *archive.ArchiveManager // set via SetArchiveManager
    branding       config.BrandingConfig   // set via SetBranding
}

func NewHandler(store, statsCollector, tracingService, proxyEngine) *Handler
func (h *Handler) SetBranding(b config.BrandingConfig)
func (h *Handler) SetArchiveManager(am *archive.ArchiveManager)
func (h *Handler) SetStoreManager(gs *store.GlobalStore, sm *store.SessionManager)
```

### Proposed

```go
// HandlerConfig holds all dependencies for the API handler.
type HandlerConfig struct {
    Store          storage.Storage
    StatsCollector *stats.Collector
    TracingService *tracing.Service
    ProxyEngine    *proxy.Engine
    GlobalStore    *store.GlobalStore       // optional; nil = Phase 1 mode
    SessionManager *store.SessionManager    // optional; nil = Phase 1 mode
    ArchiveManager *archive.ArchiveManager  // optional; nil disables archive endpoints
    Branding       config.BrandingConfig
    ScriptTimeout  int                      // ms; 0 = use default (100)
}

// NewHandler creates a fully-initialised Handler.
func NewHandler(cfg HandlerConfig) *Handler {
    return &Handler{
        store:          cfg.Store,
        statsCollector: cfg.StatsCollector,
        tracingService: cfg.TracingService,
        proxyEngine:    cfg.ProxyEngine,
        globalStore:    cfg.GlobalStore,
        sessionManager: cfg.SessionManager,
        archiveManager: cfg.ArchiveManager,
        branding:       cfg.Branding,
        parser:         parser.NewParser(),
        templateEngine: template.NewEngine(),
        scriptEngine:   scripting.NewScriptEngine(cfg.Store, cfg.ScriptTimeout),
    }
}
```

The three setter methods are removed. `NewRouter` is updated to pass a `HandlerConfig` directly.

---

## Method Distribution

### `handler.go` (~80 lines)
- `Handler` struct
- `HandlerConfig` struct
- `NewHandler(cfg HandlerConfig) *Handler`
- Shared helper: `h.notFound(c)`, `h.internalError(c, err)`

### `handler_specs.go` (~360 lines)
- `ListSpecs`
- `CreateSpec`
- `GetSpec`
- `UpdateSpec`
- `DeleteSpec`
- `EnableSpec`
- `DisableSpec`
- `ToggleTracing`
- `ToggleExampleFallback`
- `SetBackendURI`
- `ToggleProxyMode`
- `GetSpecTags`
- `UpdateSpecTags`

### `handler_operations.go` (~120 lines)
- `ListOperations`
- `GetOperation`
- `GetSignatureConfig`
- `UpdateSignatureConfig`

### `handler_responses.go` (~250 lines)
- `ListResponseConfigs`
- `CreateResponseConfig`
- `GetResponseConfig`
- `UpdateResponseConfig`
- `DeleteResponseConfig`
- `UpdateResponsePriority`

### `handler_scripts.go` (~300 lines)
- `ListScripts`
- `CreateScript`
- `ValidateScript`
- `GetScript`
- `UpdateScript`
- `DeleteScript`
- `TestScript`

### `handler_bindings.go` (~180 lines)
- `ListScriptBindings`
- `CreateScriptBinding`
- `ReorderScriptBindings`
- `UpdateScriptBinding`
- `DeleteScriptBinding`

### `handler_store.go` (~120 lines)
- `ListStoreEntries`
- `GetStoreEntry`
- `UpsertStoreEntry`
- `DeleteStoreEntry`
- `ClearStore`

### `handler_sessions.go` (~120 lines)
- `ListSessions`
- `GetSession`
- `InvalidateSession`
- `InvalidateAllSessions`

### `handler_archives.go` (~200 lines)
- `ListArchives`
- `CreateArchive`
- `UploadArchive`
- `GetArchive`
- `DeleteArchive`
- `DownloadArchive`
- `RestoreArchive`

### `handler_system.go` (~200 lines)
- `HealthCheck`
- `Version`
- `GetBranding`
- `GetRoutes`
- `GetGlobalStats`
- `GetSpecStats`
- `GetOperationStats`
- `ResetStats`
- `ListTraces`
- `GetTrace`
- `ClearTraces`
- `ListTags`
- `CreateTag`
- `UpdateTag`
- `DeleteTag`
- `ValidateTemplate`

---

## Migration Steps

1. **Create the new files** with handler methods moved verbatim (no logic changes).
2. **Update `handler.go`** to contain only the struct, config, constructor, and shared helpers.
3. **Update `NewRouter`** in `router.go` to use `HandlerConfig` instead of positional args + setters.
4. **Update `serve.go`** wiring accordingly.
5. **Run tests** — `go test ./internal/api/...` must pass unchanged.
6. **Split test file** — move test functions into matching `_test.go` files; keep shared helpers in `handler_test.go`.

---

## Acceptance Criteria

- [ ] `handler.go` is ≤ 100 lines
- [ ] No handler method appears in `handler.go` (all in domain files)
- [ ] `NewHandler` takes a `HandlerConfig`; no setter methods remain
- [ ] All existing tests pass
- [ ] New test files mirror domain files
- [ ] `go vet ./...` passes
- [ ] `golint ./internal/api/...` passes
