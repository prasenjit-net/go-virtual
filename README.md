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

session:
  headerName: "X-Virtual-Session-Id"
  inactivityTimeout: "30m"
  maxSessions: 10000

proxy:
  timeoutSeconds: 30
  insecureSkipVerify: false

scripting:
  defaultTimeoutMs: 100

ai:
  openaiApiKey: ""
  openaiModel: ""
  openaiBaseUrl: ""
```

Use environment variables or local config overrides for secrets and provider-specific AI settings.

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

- `GET /_api/ai/status`
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
