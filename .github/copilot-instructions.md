# Go-Virtual Project Copilot Instructions

## Project Overview

Go-Virtual virtualizes OpenAPI 3 specifications and can serve traffic in three spec modes:

- **standard**: saved/manual responses first, then example/default fallback
- **ai**: saved/manual responses first, then runtime AI generation, with the generated response recorded for replay
- **proxy**: saved/manual responses first, then upstream forwarding, with the backend response recorded for replay

Recorded/generated responses are stored as normal `ResponseConfig` entries with an origin of `manual`, `ai`, or `proxy`. The admin UI is mounted under `/_ui/`, the admin API under `/_api/`, docs under `/_docs/`, and metrics under `/_prometheus`.

## Tech Stack

### Backend

- Go 1.21+
- Gin
- kin-openapi
- Gorilla WebSocket
- Starlark (`go.starlark.net/starlark`)
- gjson
- Prometheus client

### Frontend

- React 18 + TypeScript
- Vite
- TailwindCSS
- React Query
- Zustand
- Monaco Editor
- Lucide React
- dnd-kit

## Project Structure

```text
go-virtual/
├── cmd/server/          # Cobra CLI entry point
├── internal/api/        # Admin handlers and router
├── internal/condition/  # Condition evaluation
├── internal/models/     # Spec, response, trace, archive, script, store models
├── internal/parser/     # OpenAPI parsing
├── internal/proxy/      # Runtime engine, recorder, signature calculation
├── internal/scripting/  # Starlark execution + bindings
├── internal/storage/    # Memory/file persistence
├── internal/store/      # Global store and session manager
├── internal/template/   # Go text/template rendering helpers
├── internal/tracing/    # Trace service and live stream
├── ui/src/components/   # Admin UI pages
├── ui/src/services/     # API client
├── ui/src/types/        # Shared TS types
├── assets/              # Branding assets
├── test/                # Sample specs and fixtures
├── config.yaml          # Default config
└── Makefile
```

## Runtime Model

### Matching order

For a matched operation, the runtime flow is:

1. compute request signature
2. evaluate saved response configs by priority
3. if no match, apply the spec-mode fallback

Mode-specific fallback:

- `standard` -> OpenAPI example/default response fallback when enabled
- `ai` -> runtime AI generation
- `proxy` -> upstream proxy request

### Response origins

`ResponseConfig.Origin` can be:

- `manual`
- `ai`
- `proxy`

Recorded/generated responses usually use a `signature` condition so they can replay exact request shapes.

### Tracing

Traces are mode-aware and source-aware. Important fields:

- `mode`
- `responseSource` (`config`, `example`, `ai`, `proxy`)
- `matchedConfigOrigin`
- `signature`
- `backendUri` when proxy fallback is used
- `scripts`
- `session`

## Important UI Behavior

- The main operation page focuses on manually configured responses.
- Generated/recorded responses are shown on an operation-scoped subpage.
- AI-generated and proxy-recorded responses share that page and are distinguished by origin badges.
- Editing a recorded response must preserve support for `signature` conditions.

## Scripting and Store

Scripts are attached by ordered `ScriptBindings` and must expose `run(req)`.

Available request data:

- `path`
- `query`
- `header`
- `body`

Builtins:

- `store.get/set/has/delete/keys`
- `log(...)`

Session model:

- `GlobalStore` is persistent and shared
- sessions are per-request snapshots keyed by the configured session header
- script test execution uses an ephemeral seeded session and must not persist mutations

## Templates

Response bodies are rendered with Go `text/template` helpers. Prefer the current helper style:

- `{{path "id"}}`
- `{{query "status"}}`
- `{{header "authorization"}}`
- `{{body "user.name"}}`
- `{{random "uuid"}}`
- `{{faker "email"}}`
- `{{timestamp "iso"}}`
- `{{script "binding.output"}}`

Legacy token syntax is still accepted and normalized internally.

## Conditions

Valid sources:

- `path`
- `query`
- `header`
- `body`
- `signature`

Valid operators:

- `eq`, `ne`
- `contains`, `notContains`
- `startsWith`, `endsWith`
- `regex`
- `exists`, `notExists`
- `gt`, `gte`, `lt`, `lte`

## API Surface Highlights

Common admin routes:

- `/_api/specs/*`
- `/_api/operations/:id`
- `/_api/operations/:id/signature`
- `/_api/operations/:id/responses`
- `/_api/operations/:id/ai-response`
- `/_api/operations/:id/scripts`
- `/_api/scripts/*`
- `/_api/store/*`
- `/_api/sessions/*`
- `/_api/archives/*`
- `/_api/traces/*`
- `/_api/stats/*`
- `/_api/ai/status`

## Build and Dev Commands

Use these commands:

```bash
make build
make build-ui
make build-go
make dev
make dev-ui
make dev-all
make test
```

Important:

- `make build-go` already rebuilds the UI first.
- In dev mode, UI assets are served from `./ui/dist`.
- In headless mode, the admin UI and admin API are disabled; proxy routing still works and metrics stay available.

## Implementation Notes

- Keep changes compatible with existing stored specs that may still use legacy `proxyMode` or `recorded` fields.
- Prefer updating normalization or compatibility logic at storage/API boundaries rather than scattering special cases.
- Do not treat proxy mode as an early exit before matching saved responses.
- When changing recorded response behavior, keep deduplication keyed by request signature and origin.
- After changing specs or operations in ways that affect routing, ensure routes are reloaded.
