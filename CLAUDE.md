# go-virtual — Claude Code guidance

## What this project is

go-virtual is an OpenAPI 3 API virtualisation server. It lets developers load OpenAPI specs, configure mock responses with conditions and templates, and optionally proxy requests to a real backend. Key capabilities:

- **Mock mode** — return configured responses matched by priority + conditions
- **Proxy mode** — forward to a real backend and optionally record responses
- **AI mode** — generate responses on-the-fly via OpenAI, Claude, or Copilot
- **Scripting** — Starlark scripts attached to specs, operations, or responses for dynamic logic
- **Sessions** — per-client session state via a header, backed by memory or Redis
- **Global store** — shared key/value store accessible from scripts and templates
- **Tracing** — full request/response capture, streamed live over WebSocket
- **Admin UI** — React/Vite SPA embedded in the binary, served at `/_ui/`

---

## Package map

```
cmd/server/           — CLI entry point (Cobra). Commands: serve, init, healthcheck
internal/
  api/                — Admin REST API handlers + Gin router (all routes at /_api/*)
  archive/            — Snapshot export/import (backup/restore)
  ai/                 — AI provider abstraction (OpenAI, Claude, Copilot)
  condition/          — Condition evaluator (eq, contains, regex, exists, gt, lt, AND/OR/NOT)
  config/             — Config struct + loader (YAML + env vars via Viper)
  logging/            — Structured logger setup (slog)
  metrics/            — Prometheus metrics
  models/             — All data models (Spec, Operation, ResponseConfig, Script, …)
  parser/             — OpenAPI 3 parser (extracts operations, params, examples)
  proxy/              — Core proxy engine: request routing, response matching, recording
  scripting/          — Starlark script engine
  stats/              — In-memory request stats collector
  storage/            — Storage interface + 3 backends (file, memory, mongo)
  store/              — Global key/value store + session management
  sync/               — Multi-instance sync (MongoDB change stream or polling)
  template/           — Go-template engine for response bodies/headers
  tlsutil/            — TLS cert generation helpers
  tracing/            — Trace capture + WebSocket streaming
  version/            — Version string (injected at build time via ldflags)
ui/                   — React + Vite frontend (TypeScript, Tailwind, Zustand, React Query)
```

---

## Key data models

| Model | Key fields | Notes |
|---|---|---|
| `Spec` | `ID`, `Content` (raw YAML/JSON), `BasePath`, `Enabled`, `ModePolicy` | Owns Operations. `ModePolicy` replaces the old `Mode`/`ProxyMode` fields (kept for compat). |
| `Operation` | `ID`, `SpecID`, `Method`, `Path`, `FullPath` | Parsed from spec. Owns ResponseConfigs. |
| `ResponseConfig` | `ID`, `OperationID`, `Priority`, `Conditions`, `StatusCode`, `Body`, `Headers` | Lower `Priority` = matched first. Body/headers are Go templates. |
| `Script` | `ID`, `Name`, `Source`, `Enabled` | `Source` is excluded from JSON (`json:"-"`) — see storage note below. |
| `ScriptBinding` | `ID`, `SpecID`/`OperationID`/`ResponseConfigID`, `ScriptID`, `OutputKey`, `Order` | Exactly one of the three target IDs is set. |
| `AIScenario` | `ID`, `Name`, `Instructions`, `StatusCode`, `Enabled` | Global (not per-spec). Selected via `X-Virtual-AI-Scenario` request header. |
| `Tag` | `Name` | Simple label applied to ResponseConfigs for filtering. |

Model files live in `internal/models/`. The `Storage` interface in `internal/storage/interface.go` is the single source of truth for what operations each backend must support.

---

## Storage backends

There are **three backends** that implement `storage.Storage` and must all behave identically:

| Backend | File | Query strategy |
|---|---|---|
| `memory` | `internal/storage/memory.go` | Go maps + loops over struct fields |
| `file` | `internal/storage/file.go` | Loads everything into the memory backend; persists to JSON files on disk |
| `mongo` | `internal/storage/mongo.go` | Server-side BSON queries; uses `!unit` build tag |

**Whenever you add or change a storage feature, verify all three backends work.**

File and memory storage never surface field-promotion bugs because they query Go struct fields directly. MongoDB queries filter against top-level BSON fields and cannot reach inside the JSON blob stored in `genericDoc.Data`.

### The MongoDB field-promotion rule

Every entity is wrapped in `genericDoc` with the entity JSON in `data` (opaque to MongoDB). Any field used in a `bson.M{...}` filter **must be explicitly promoted** to a named top-level BSON field on `genericDoc` and populated on every write.

Checklist when writing a new Mongo query:
1. Is the filter field declared with a `bson:"..."` tag in `genericDoc`?
2. Is it set in the `marshalDoc` call or explicitly after it for the relevant entity type?
3. Is there an index for it in `(*MongoStorage).EnsureIndexes`?

Current promoted fields in `genericDoc`:

| BSON field | Populated from | Used by |
|---|---|---|
| `spec_id` | `op.SpecID` / `binding.SpecID` | operations, bindings |
| `operation_id` | `cfg.OperationID` / `binding.OperationID` | responses, bindings |
| `script_id` | `binding.ScriptID` | bindings |
| `response_config_id` | `binding.ResponseConfigID` | bindings |
| `source` | `script.Source` | scripts (`json:"-"` excludes it from the data blob) |

### Index management

Indexes live in `(*MongoStorage).EnsureIndexes` (`internal/storage/mongo.go`) and are created by `go-virtual init`, **not** at server startup — the runtime user in prod may lack `createIndex` privilege. Add a new index entry whenever you promote a new query field.

---

## Build & run

```bash
make build           # build UI (Vite) then Go binary → build/go-virtual
make dev             # go run in dev mode (--dev serves UI from ui/dist on disk)
make dev-ui          # start Vite dev server (proxies API calls to :8080)
make dev-all         # both above in parallel (requires 'concurrently')

make test            # go test -tags unit ./...   (no MongoDB needed)
make test-coverage   # same, produces coverage.html
make test-coverage-integration  # includes Mongo integration tests (needs $MONGO_URI)

make compose-up-build  # docker-compose build + run (file storage, :8080)
make compose-down

make lint            # golangci-lint
make fmt             # gofmt
```

Build tags:
- `-tags unit` — excludes `mongo.go` and other `!unit` files; all unit tests use this
- No tag / integration — includes MongoDB backend; requires a live instance

Version info is injected at build time:
```
-X github.com/prasenjit/go-virtual/internal/version.Version=...
-X github.com/prasenjit/go-virtual/internal/version.Commit=...
-X github.com/prasenjit/go-virtual/internal/version.BuildDate=...
```

---

## Configuration

Loaded from `config.yaml` (current directory) or `--config` flag. All keys also available as env vars with prefix `GOVIRTUAL_` and `_` separators (e.g. `GOVIRTUAL_STORAGE_TYPE`). CamelCase viper keys need explicit `BindEnv` calls — see `serve.go`.

Top-level sections: `server`, `storage`, `tracing`, `logging`, `branding`, `scripting`, `session`, `proxy`, `ai`.

Storage types: `file` (default), `memory`, `mongo`.
AI providers: `openai`, `claude`, `copilot`.
Session stores: `memory` (default), `redis`.

---

## Admin API routes (`/_api/`)

| Method | Path | Handler |
|---|---|---|
| GET/POST | `/specs` | list / create spec |
| GET/PUT/DELETE | `/specs/:id` | get / update / delete spec |
| GET | `/specs/:id/operations` | list operations for spec |
| GET | `/operations/:id` | get operation |
| GET | `/operations/:id/responses` | list response configs |
| POST | `/operations/:id/responses` | create response config |
| GET/PUT/DELETE | `/responses/:id` | get / update / delete response config |
| POST | `/responses/:id/clone` | clone response config |
| GET/POST | `/specs/:id/scripts` | list / create spec-level script bindings |
| GET/POST | `/operations/:id/scripts` | list / create operation-level script bindings |
| GET/POST | `/operations/:id/responses/:respId/scripts` | response-level script bindings |
| GET/POST | `/scripts` | list / create scripts |
| GET/PUT/DELETE | `/scripts/:id` | get / update / delete script |
| GET/POST | `/ai-scenarios` | list / create AI scenarios |
| GET/PUT/DELETE | `/ai-scenarios/:id` | manage AI scenario |
| GET | `/stats`, `/stats/specs/:id`, `/stats/operations/:id` | statistics |
| GET | `/traces`, `/traces/:id` | traces |
| GET/POST | `/store`, `/store/:key` | global key/value store |
| GET | `/sessions`, `/sessions/:id` | active sessions |
| GET/POST | `/archives` | snapshot archives |
| GET | `/health`, `/version`, `/branding` | system |

Proxy/mock traffic is served on all other paths (non-`/_api/`, non-`/_ui/`).

---

## Scripting

Scripts are written in Starlark (a deterministic Python subset). A `ScriptBinding` attaches a script to a spec, operation, or response config with an `outputKey`. The script result is available in templates as `{{.script.<outputKey>.*}}`.

`Script.Source` has `json:"-"` — intentional for the file backend which stores source in a companion `.star` file. The MongoDB backend stores it in a top-level `source` BSON field instead.

---

## Multi-instance sync

When storage is `mongo`, multiple server instances stay in sync via `internal/sync/`:
- `MongoWatcher` — uses MongoDB change streams (requires replica set)
- `PollingWatcher` — falls back to periodic collection scans

Configured via `storage.mongo.sync.mode` (`changestream` or `poll`) and `storage.mongo.sync.pollIntervalSeconds`.

---

## UI

React SPA (TypeScript, Vite, Tailwind CSS, Zustand for state, React Query for server state).
Source in `ui/src/`. Built output embedded in the binary via `go:embed`. In `--dev` mode the binary serves from `ui/dist` on disk (no rebuild needed for UI changes during Go development).

Main UI sections: Specs, Operations, Responses, Scripts, AI Scenarios, Store, Sessions, Archives, Traces.
