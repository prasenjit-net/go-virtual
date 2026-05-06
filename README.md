<p align="center">
  <img src="assets/logo-banner.svg" alt="go-virtual logo" width="520" />
</p>

<p align="center">
  <strong>API Mock, AI, and Proxy Virtualization for OpenAPI 3</strong><br/>
  Run manual mocks, AI-generated fallbacks, upstream proxy recording, tracing, scripting, sessions, and admin tooling from one Go service.
</p>

<p align="center">
  <a href="#quick-start">Quick Start</a> ·
  <a href="#execution-modes">Execution Modes</a> ·
  <a href="#features">Features</a> ·
  <a href="#build-and-development">Build & Development</a> ·
  <a href="#api-surface">API Surface</a>
</p>

---

## What it does

Go-Virtual virtualizes OpenAPI 3 specifications and serves them under configurable base paths. For each operation, it can:

- serve manually configured responses
- replay previously generated responses
- fall back to OpenAPI examples
- generate a structured response with AI
- proxy to a real backend and record the result

The admin UI lives under `/_ui/`, the admin API under `/_api/`, docs under `/_docs/`, and Prometheus metrics under `/_prometheus`.

## Execution Modes

Each spec runs in one of three modes:

| Mode | Behavior after saved responses miss |
| --- | --- |
| `standard` | Uses OpenAPI example/default fallback when enabled |
| `ai` | Generates a structured response from request + schema, then records it |
| `proxy` | Forwards to the configured upstream backend, then records it |

In **all modes**, existing response configs are checked first. Recorded/generated responses are stored as normal replayable responses with an origin of:

- `manual`
- `ai`
- `proxy`

## Features

- **OpenAPI 3 virtualization** with dynamic route mounting
- **Manual response design** with conditions, priorities, delays, headers, and body templates
- **Generated response management** with operation-scoped pages for recorded AI/proxy responses
- **AI fallback mode** for runtime structured generation from the request and response schema
- **Global AI scenarios** shared across specs and managed from a sidebar-linked admin page
- **Proxy fallback mode** with upstream forwarding and response capture
- **Tracing** with response source and mode awareness
- **Starlark scripting** with per-operation ordered bindings
- **Session-aware store** with a global store and per-request session snapshots
- **Prometheus metrics** and statistics dashboards
- **Archive import/export** for instance state backup and restore
- **Embedded React admin UI** served from `/_ui/`

## Quick Start

### Prerequisites

- Go 1.21+
- Node.js 18+
- npm

### Build and run

```bash
make install-deps
make build
make run
```

Open:

- Admin UI: `http://localhost:8080/_ui/`
- Admin API: `http://localhost:8080/_api/`
- Docs: `http://localhost:8080/_docs/`
- Metrics: `http://localhost:8080/_prometheus`

### Try the sample spec

Upload `test/petstore.yaml` from the admin UI, or post it through the admin API with any JSON-capable client.

Then call the virtualized endpoint:

```bash
curl http://localhost:8080/pets
```

## Request Matching Model

For a matched operation, Go-Virtual evaluates responses in this order:

1. enabled response configs by priority
2. mode-specific fallback (`standard`, `ai`, or `proxy`)

Conditions are ANDed. Supported condition sources include:

- `path`
- `query`
- `header`
- `body`
- `signature` for replayable recorded/generated responses

Supported operators include:

- `eq`, `ne`
- `contains`, `notContains`
- `startsWith`, `endsWith`
- `regex`
- `exists`, `notExists`
- `gt`, `gte`, `lt`, `lte`

## Template and Scripting

Response bodies use Go `text/template` helpers. Current template style supports helpers like:

- `{{path "id"}}`
- `{{query "status"}}`
- `{{header "authorization"}}`
- `{{body "user.name"}}`
- `{{random "uuid"}}`
- `{{faker "email"}}`
- `{{timestamp "iso"}}`
- `{{script "binding.output"}}`

The editor also supports legacy token forms and rewrites them internally.

Scripts are written in **Starlark** and attached to operations through ordered script bindings. A script:

- must expose `def run(req):`
- receives `path`, `query`, `header`, and `body`
- can use `store` for session-scoped state
- can use `log(...)` for trace-visible diagnostics

## Session and Store Model

- **Global store**: persistent application-wide key/value store
- **Session store**: private per-session copy seeded from the global store
- **Session backend**: in-memory by default, or Redis when multiple instances need shared sessions
- **Session header**: configurable, default `X-Virtual-Session-Id`
- **Script testing**: runs against an ephemeral session snapshot and does not persist mutations

## Tracing and Recording

Tracing is enabled per spec. Trace records include:

- request/response payloads
- matched response config
- spec mode
- final response source (`config`, `example`, `ai`, `proxy`)
- matched config origin (`manual`, `ai`, `proxy`)
- proxy/backend details when applicable
- script traces and session activity when present

AI-generated and proxy-recorded responses are shown together on the operation’s generated responses page and remain editable like manual responses.

## AI Scenarios

AI runtime generation can be steered with the `X-Virtual-AI-Scenario` request header.

- scenarios are **application-scoped** and shared across all specs
- the admin UI manages them from the **AI Scenarios** page in the sidebar
- default scenarios are seeded automatically:
  - `success`
  - `client_error`
  - `server_error`

When a request reaches AI fallback mode, the runtime looks up the named scenario and applies its
kind, optional status code, optional count hint, and free-form instructions. If no enabled scenario
matches, AI generation continues without scenario overrides.

All `X-Virtual-*` headers are excluded from recorded-response signature hashing, so control headers
like `X-Virtual-AI-Scenario` and `X-Virtual-Session-Id` do not fragment replayed recordings.

## Build and Development

### Common commands

```bash
make build          # build UI + Go binary
make build-ui       # build UI only
make build-go       # rebuild UI, then build Go binary
make dev            # run Go server in dev mode
make dev-ui         # run Vite dev server
make dev-all        # run Go server + Vite together
make test           # run Go tests
make clean          # remove build artifacts and node_modules
```

### Notes

- `make build-go` rebuilds the UI first so embedded assets stay current.
- In dev mode (`make dev`), the UI is served from `./ui/dist`.
- In headless mode, the admin API and admin UI are disabled; only proxy routing and metrics remain available.

## Configuration

The default config file is `config.yaml`. Important sections include:

```yaml
server:
  host: "127.0.0.1"
  port: 8080

storage:
  type: "file"
  path: "./data"

logging:
  level: "info"   # debug, info, warn, error
  format: "json"  # json or text

session:
  storeType: "memory"  # memory or redis
  headerName: "X-Virtual-Session-Id"
  inactivityTimeout: "30m"
  maxSessions: 10000
  redis:
    addr: "127.0.0.1:6379"
    username: ""
    password: ""
    db: 0
    keyPrefix: "go-virtual:sessions"

proxy:
  timeoutSeconds: 30
  insecureSkipVerify: false

scripting:
  defaultTimeoutMs: 100

ai:
  provider: "openai"  # or "claude"
  openai:
    apiKey: ""
    model: "gpt-4o-mini"
    baseUrl: ""
  claude:
    apiKey: ""
    model: "claude-sonnet-4-6"
    baseUrl: ""
    apiVersion: "2023-06-01"
```

Legacy OpenAI aliases (`ai.openaiApiKey`, `ai.openaiModel`, `ai.openaiBaseUrl`) are still supported for backward compatibility. Use environment variables or local config overrides for secrets and provider-specific AI settings.

Use `logging.level: "debug"` during troubleshooting to include debug-level request noise such as health, metrics, and static asset access logs. For production, `info` with `json` is the recommended default.

Use `session.storeType: "redis"` only when you need shared session state across multiple Go-Virtual instances. Specs, responses, archives, and the global store still use the normal `storage.*` configuration; Redis support is scoped to sessions only.

## Storage Backends

Go-Virtual supports three storage backends configured via `storage.type`:

| Type | Description |
|------|-------------|
| `file` | (default) Persists specs, responses, scripts, and the global store to the local filesystem under `storage.path`. |
| `memory` | Keeps all data in RAM. Data is lost on restart. Useful for testing or ephemeral environments. |
| `mongo` | Uses MongoDB for all persistent data and the global store. Suitable for multi-instance deployments. |

### File storage (default)

```yaml
storage:
  type: file
  path: ./data
```

### Memory storage

```yaml
storage:
  type: memory
```

### MongoDB storage

```yaml
storage:
  type: mongo
  mongo:
    uri: "mongodb://localhost:27017"
    database: "go-virtual"          # default
    collectionPrefix: "gv_"         # default; prefix for all collection names
    connectTimeoutSeconds: 10        # default
    sync:
      mode: "auto"                  # auto | change_stream | polling | off
      pollIntervalSeconds: 10       # used by polling and auto-fallback
```

When `storage.type` is `mongo`, both the entity storage (specs, operations, responses, scripts, bindings, tags, AI scenarios) and the global key-value store are backed by MongoDB. Each entity type has its own prefixed collection (e.g., `gv_specs`, `gv_responses`, `gv_global_store`).

The `sync` subsection controls cross-instance synchronisation. See [Horizontal Scaling](#horizontal-scaling) below for details.

The MongoDB URI, database name, collection prefix, and connection timeout can also be provided via environment variables using the `GOVIRTUAL_` prefix with underscores replacing dots:

```
GOVIRTUAL_STORAGE_MONGO_URI=mongodb://host:27017
GOVIRTUAL_STORAGE_MONGO_DATABASE=myapp
GOVIRTUAL_STORAGE_MONGO_SYNC_MODE=auto
```

## Horizontal Scaling

When multiple go-virtual instances share a MongoDB backend, they must stay in sync. Two in-memory caches are kept by each instance:

- **Route table** (`proxy.Engine.routes`) — built from specs/operations at startup. Without sync, uploading a spec to Instance A leaves Instance B serving 404 for the new routes.
- **Global store cache** (`MongoGlobalStore.cache`) — write-through in-memory map. Without sync, values written by scripts on Instance A are invisible on Instance B.

The `storage.mongo.sync` subsystem solves both by watching MongoDB for changes:

| Sync mode | Mechanism | Latency | MongoDB requirement |
|-----------|-----------|---------|---------------------|
| `change_stream` | MongoDB change stream cursors | Sub-second | Replica set or Atlas |
| `polling` | SHA-256 fingerprint scan | Up to `pollIntervalSeconds` | Any deployment |
| `auto` | Change streams, fallback to polling | Sub-second (or poll interval) | Any deployment |
| `off` | No sync | — | Single-instance only |

**Session state** is not synced through MongoDB. Use `session.storeType: redis` for shared sessions across instances.

**Tracing and metrics** are per-instance by design. Point Prometheus at all instances to aggregate metrics.

See the full [Clustering guide](docs/clustering.html) for architecture details, Docker Compose and Kubernetes examples, and a deployment checklist.

## API Surface

### Specs

- `GET /_api/specs`
- `POST /_api/specs`
- `GET /_api/specs/:id`
- `PUT /_api/specs/:id`
- `DELETE /_api/specs/:id`
- `PUT /_api/specs/:id/enable`
- `PUT /_api/specs/:id/disable`
- `PUT /_api/specs/:id/tracing`
- `PUT /_api/specs/:id/example-fallback`
- `PUT /_api/specs/:id/backend`
- `PUT /_api/specs/:id/mode`
- `PUT /_api/specs/:id/proxy-mode` (legacy compatibility)
- `GET /_api/specs/:id/tags`
- `PUT /_api/specs/:id/tags`

### Operations and responses

- `GET /_api/specs/:id/operations`
- `GET /_api/operations/:id`
- `GET /_api/operations/:id/signature`
- `PUT /_api/operations/:id/signature`
- `GET /_api/operations/:id/responses`
- `POST /_api/operations/:id/responses`
- `POST /_api/operations/:id/ai-response`
- `GET /_api/responses/:id`
- `PUT /_api/responses/:id`
- `DELETE /_api/responses/:id`
- `PUT /_api/responses/:id/priority`

### AI, scripts, store, sessions, and archives

- `GET /_api/ai/status` — returns the selected provider plus whether AI generation is configured
- `GET /_api/ai-scenarios`
- `POST /_api/ai-scenarios`
- `PUT /_api/ai-scenarios/:scenarioId`
- `DELETE /_api/ai-scenarios/:scenarioId`
- `POST /_api/scripts/ai-generate`
- `GET/POST/PUT/DELETE /_api/scripts...`
- `GET/PUT /_api/operations/:id/scripts...`
- `GET/PUT/DELETE /_api/store...`
- `GET/DELETE /_api/sessions...`
- `GET/POST/DELETE /_api/archives...`

### Observability

- `GET /_api/stats`
- `GET /_api/stats/specs/:id`
- `GET /_api/stats/operations/:id`
- `POST /_api/stats/reset`
- `GET /_api/traces`
- `GET /_api/traces/:id`
- `DELETE /_api/traces`
- `WS /_api/traces/stream`
- `GET /_prometheus`

## Project Structure

```text
go-virtual/
├── cmd/server/          # CLI entry point
├── internal/api/        # Admin API handlers and router
├── internal/proxy/      # Runtime request engine, recorder, signature logic
├── internal/scripting/  # Starlark engine and script bindings
├── internal/store/      # Global store and session management
├── internal/template/   # Response template engine
├── internal/tracing/    # Trace capture and WebSocket streaming
├── internal/storage/    # Memory/file persistence
├── ui/                  # React/Vite admin UI
├── test/                # Sample specs and test data
├── config.yaml          # Default configuration
└── Makefile
```

## License

MIT
