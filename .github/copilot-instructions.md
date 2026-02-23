# Go-Virtual Project Copilot Instructions

## Project Overview

Go-Virtual is an API proxy/mock service that virtualizes OpenAPI 3 specifications. It allows configuring custom responses based on request conditions, with support for Starlark scripting, session-aware store, templating, tracing, metrics, and TLS. Current version: **v1.0.0**.

## Tech Stack

### Backend (Go 1.21+)
- **HTTP Framework**: Gin (`github.com/gin-gonic/gin`)
- **OpenAPI Parser**: kin-openapi (`github.com/getkin/kin-openapi/openapi3`)
- **WebSocket**: Gorilla WebSocket (`github.com/gorilla/websocket`)
- **Starlark Engine**: `go.starlark.net/starlark`
- **JSON Path**: gjson (`github.com/tidwall/gjson`)
- **UUID**: Google UUID (`github.com/google/uuid`)
- **YAML**: `gopkg.in/yaml.v3`
- **Metrics**: Prometheus (`github.com/prometheus/client_golang`)

### Frontend (React 18 + TypeScript)
- **Build Tool**: Vite
- **Styling**: TailwindCSS
- **State Management**: Zustand
- **Data Fetching**: @tanstack/react-query
- **Charts**: Recharts
- **Code Editor**: Monaco Editor (@monaco-editor/react)
- **Icons**: Lucide React
- **Drag & Drop**: @dnd-kit

## Project Structure

```
go-virtual/
├── cmd/server/          # CLI entry point (cobra commands: serve, version)
├── internal/
│   ├── api/             # HTTP handlers and routing
│   ├── condition/       # Request condition evaluation
│   ├── config/          # Configuration loading
│   ├── metrics/         # Prometheus metrics
│   ├── models/          # Data models
│   ├── parser/          # OpenAPI 3 spec parser
│   ├── proxy/           # Dynamic proxy/mock engine
│   ├── scripting/       # Starlark scripting engine + bindings
│   ├── stats/           # Statistics collector
│   ├── storage/         # Data persistence (memory/file)
│   ├── store/           # GlobalStore + SessionManager + StoreBuiltin
│   ├── template/        # Response templating engine
│   ├── tlsutil/         # TLS certificate management
│   ├── tracing/         # Request/response tracing + WebSocket stream
│   └── version/         # Version info
├── ui/                  # React frontend
│   └── src/
│       ├── components/
│       │   ├── ScriptManager/    # Script CRUD, bindings, editor, test panel
│       │   ├── SessionManager/   # Session inspector UI
│       │   ├── SpecManager/      # Spec upload/list/detail
│       │   ├── StoreManager/     # Global store CRUD UI
│       │   ├── ResponseDesigner/ # Response config editor
│       │   ├── Dashboard.tsx
│       │   ├── Layout.tsx
│       │   ├── TagManager.tsx
│       │   └── TraceViewer.tsx
│       ├── services/    # API client (api.ts)
│       └── types/       # TypeScript interfaces (index.ts)
├── assets/              # Logo SVGs (logo.svg, logo-banner.svg)
├── test/                # Test specs and data
├── ui.go                # Embedded UI filesystem (//go:embed)
├── Makefile             # Build automation
└── config.yaml          # Default configuration
```

## Coding Conventions

### Go
- Use standard Go project layout with `cmd/` and `internal/`
- Keep packages focused and minimal
- Use interfaces for dependencies (storage, services)
- Error handling: return errors, don't panic
- Use `context.Context` for cancellation where appropriate
- Mutex naming: `mu` for single mutex, descriptive names for multiple
- Comments: GoDoc style for exported functions
- Test coverage target: ≥ 80%

### TypeScript/React
- Functional components with hooks only
- TypeScript strict mode enabled
- Use React Query for all server state
- Use Zustand for client-only state if needed
- TailwindCSS for all styling (no CSS modules)
- Lucide icons exclusively (no other icon libraries)

## Key Patterns

### Template Variables
Response bodies and headers support these template variables:
- `{{.path.<param>}}` — Path parameters
- `{{.query.<param>}}` — Query parameters
- `{{.header.<name>}}` — Request headers
- `{{.body}}` — Full request body (raw)
- `{{.body.<jsonpath>}}` — JSON path extraction via gjson
- `{{.random.uuid}}`, `{{.random.int}}`, `{{.random.string}}`
- `{{.timestamp}}`, `{{.timestamp.unix}}`
- `{{.script.<outputKey>.<field>}}` — Script binding output

### Condition Operators
- `eq`, `ne` — Equality
- `contains`, `not_contains` — String contains
- `regex` — Regular expression match
- `exists`, `not_exists` — Field existence
- `gt`, `gte`, `lt`, `lte` — Numeric comparison
- `in`, `not_in` — Value in list

### Starlark Scripting
Scripts are attached to operations via **ScriptBindings** (ordered). Each script:
- Must define `def run(req):` as the top-level entry point
- Receives a `req` dict with `path`, `query`, `header`, `body` keys
- Has access to `store` builtin (session-scoped key-value store)
- Has access to `log(...)` builtin (messages collected into trace logs)
- Returns any value; stored under `binding.OutputKey` in template context
- Compiled once, cached by `(scriptID, updatedAt)`; thread-safe
- Timeout configurable per-script (default 100 ms)

#### Store Builtin (`store`)
- `store.get("key")` / `store.get("key", default)` — read value
- `store.set("key", value)` — write value (session-local)
- `store.has("key")` — existence check
- `store.delete("key")` — remove key
- `store.keys()` — list all keys

#### Session Store Architecture
- **GlobalStore** — application-wide persistent KV store (JSON file on disk)
- **Session** — per-request private copy seeded from GlobalStore snapshot at session creation; mutations never propagate back to GlobalStore
- Sessions identified by `X-Virtual-Session-Id` header (configurable)
- Unknown/missing IDs → new session created, UUID echoed back in response header
- Sessions expire after configurable inactivity timeout (default 30 min)
- **TestScript endpoint** — creates an ephemeral `Session` seeded from the live GlobalStore snapshot; discarded after execution so test mutations never persist

### API Endpoints
- Admin API: `/_api/*` (specs, operations, responses, stats, traces, scripts, store, sessions)
- Metrics: `/_prometheus` (Prometheus exposition format)
- Admin UI: `/_ui/*` (embedded React SPA)
- Proxy: All other paths (matched against registered specs)

### Proxy Mode vs Mock Mode
- **Mock mode** (default): engine matches request → evaluates conditions → runs script bindings → renders template response
- **Proxy mode** (`spec.ProxyMode = true`): engine forwards request to upstream; script bindings and session resolution are **skipped entirely**
- **Recording mode**: proxy records real backend responses as saved response configs

## Build Commands

```bash
make build          # Full production build (UI + Go binary)
make build-ui       # Build UI only
make build-go       # Build Go binary only
make dev-server     # Run Go server in dev mode
make dev-ui         # Run Vite dev server
make install-deps   # Install all dependencies
make clean          # Clean build artifacts
```

## Important Notes

1. **UI Embedding**: The UI is embedded in the Go binary via `//go:embed` in `ui.go`. Always run `make build-ui` before `make build-go` for a production build.

2. **Dev Mode**: Pass `-dev` flag to serve UI from `./ui/dist` on the filesystem instead of the embedded copy.

3. **Route Reloading**: Call `proxyEngine.ReloadRoutes()` after any spec or operation change.

4. **Response Priority**: Lower priority number = higher precedence. Conditions are evaluated in priority order; first match wins.

5. **Tracing**: Enable per-spec via the API. Live traces streamed via WebSocket at `/_api/traces/stream`. `ScriptTrace` records include `logs`, `output`, `durationMs`, and `error`. `SessionTrace` records capture the session ID, whether it was newly created, and store access events.

6. **Metrics**: Prometheus metrics exposed at `/_prometheus`. Covers request counts, latency histograms, error rates, and spec-level breakdowns.

7. **TLS**: Self-signed certificate auto-generated via `tlsutil` if no cert/key files are provided. Certs are stored under the data directory.

8. **Headless Mode**: Pass `--headless` flag (or `server.headless: true` in config) to disable the admin UI and serve only the proxy + API.

9. **Script Test Isolation**: `ScriptEngine.TestScript` always creates an ephemeral `Session` seeded from the current `GlobalStore` snapshot so `store.*` calls work correctly. The session is discarded after the call — no side effects.

10. **`ScriptEngine.SetGlobalStore`**: Must be called after wiring Phase 2 (done automatically in `handler.SetStoreManager`). Without it, `TestScript` falls back to an empty store snapshot.


## Testing

Test the proxy with the sample petstore spec:
```bash
# Upload spec
curl -X POST http://localhost:8080/_api/specs \
  -H "Content-Type: application/json" \
  -d '{"content": "$(cat test/petstore.yaml)", "name": "Pet Store"}'

# Test endpoint
curl http://localhost:8080/pets
```
