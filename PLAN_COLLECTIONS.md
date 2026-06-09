# Feature Plan: Collections — No-Code Data Persistence & Rendering

> v3 — mapper reuses the existing `store.collection("name")` Starlark built-in;
> no new storage layer; response-level attachment; visual mapper in response editor.

---

## Problem Statement

go-virtual is stateless between requests. There is no way to simulate a POST that creates a
resource and a GET that returns it without writing Starlark scripts. This plan introduces
**Collection Mappings**: a visual, no-code way to attach collection read/write operations to a
response config, so incoming request fields can be mapped to collection operations and the results
rendered in the response template.

---

## Reusing the Existing Collection Feature

`store.collection("name")` already exists in go-virtual as a Starlark built-in
(`internal/store/collection_builtin.go`). It is accessible from scripts today:

```python
users = store.collection("users")
users.insert({"name": req.body("name"), "email": req.body("email")})
user  = users.findOne({"id": req.path("id", "")})
```

The built-in provides: `insert(doc)`, `findAll([filter])`, `findOne(filter)`,
`update(filter, changes)`, `remove(filter)`, `count([filter])`, `clear()`.

Collection data is stored in the **session's private store** under the key
`"__col__" + name` (via `models.CollectionKeyPrefix`). Sessions are seeded from the
`GlobalStore.Snapshot()` at creation, but writes stay session-local
(see `Session.Set` — "session-local, never propagates to global").

**This feature does not create a new storage layer or a new Starlark built-in.** The
`CollectionMapping` executor stores data in the exact same place — the active request's
`SessionState` with the same `__col__` key prefix. This guarantees:

- Data written by a mapper in request N is readable by scripts in the same request (same session).
- Data persists between requests **as long as the client uses the same session**
  (via the `X-Virtual-Session` request header).
- Scripts using `store.collection("users")` and a mapper targeting `"users"` see the same records
  within a session.

### Storage summary

| Where | What stores there | Visible to |
|---|---|---|
| `Session` (`__col__<name>`) | mapper executor, scripts (`store.collection`) | scripts and mappers in the same session |
| `GlobalStore` | `store.get/set` keys | all sessions on creation (snapshot-seeded) |

For a full CRUD simulation the client must use a consistent `X-Virtual-Session` header so that
the session — and the collection data inside it — survives across requests.

---

## Core Concept

A **CollectionMapping** is attached to a response config (not an operation). When that response
config is matched for a request, its mappings execute in order. Each mapping:

1. Resolves source values from the request (path params, query params, headers, body fields,
   session store, global store, or literals)
2. Executes one collection operation (`insert`, `update`, `upsert`, `delete`, `find-one`,
   `find-many`) against the named collection in the request's session state
3. Exposes the result under `{{.Collection.<outputKey>.*}}` in the response template

Multiple mappings on the same response config can each target a **different** collection.

### End-to-end example

```
POST /users         (with header: X-Virtual-Session: client-abc)
  ResponseConfig 201 — "Created"
    Mapper #1  insert → "users"    data: body.name→name, body.email→email
               outputKey: newUser
    Body: {"id":"{{.Collection.newUser._id}}","name":"{{.Collection.newUser.name}}"}

GET /users/:id      (with header: X-Virtual-Session: client-abc)
  ResponseConfig 200 — "Found"
    Mapper #1  find-one → "users"  filter: path.id→_id
               outputKey: user
    Body: {"id":"{{.Collection.user._id}}","name":"{{.Collection.user.name}}","email":"{{.Collection.user.email}}"}

DELETE /users/:id
  ResponseConfig 204 — "Deleted"
    Mapper #1  delete → "users"    filter: path.id→_id
               outputKey: deleted
    Body: (empty — deleted record accessible as {{.Collection.deleted._id}} if needed)
```

The same session (`client-abc`) sees the data across all three requests because the collection
lives in that session's store.

---

## New Data Models

Only models specific to the mapping configuration. No new `Collection` or `CollectionRecord`
models are needed — collections are implicit entries in the existing session store.

All new types live in `internal/models/collection.go`.

### `CollectionMapping`

Attaches to a `ResponseConfig` (mirrors the response-level `ScriptBinding` pattern).

```go
type CollectionMapping struct {
    ID               string             `json:"id"`
    ResponseConfigID string             `json:"responseConfigId"`
    CollectionName   string             `json:"collectionName"`  // name passed to store.collection()
    Name             string             `json:"name"`            // display name
    Operation        CollectionOpType   `json:"operation"`
    FilterRules      []FieldMappingRule `json:"filterRules"` // locate record(s)
    DataRules        []FieldMappingRule `json:"dataRules"`   // fields to write
    OutputKey        string             `json:"outputKey"`   // {{.Collection.<outputKey>}}
    Order            int                `json:"order"`
    Enabled          bool               `json:"enabled"`
}

type CollectionOpType string

const (
    ColOpInsert   CollectionOpType = "insert"
    ColOpUpdate   CollectionOpType = "update"
    ColOpUpsert   CollectionOpType = "upsert"
    ColOpDelete   CollectionOpType = "delete"
    ColOpFindOne  CollectionOpType = "find-one"
    ColOpFindMany CollectionOpType = "find-many"
)

// FieldMappingRule maps one collection field to a value sourced from the request.
type FieldMappingRule struct {
    TargetField string `json:"targetField"` // field name in the collection document
    SourceType  string `json:"sourceType"`  // path|query|header|body|session|store|literal
    SourceKey   string `json:"sourceKey"`   // param name, header name, body dot-path, or value
}
```

**`SourceType` values:**

| SourceType | SourceKey meaning | Example |
|---|---|---|
| `path` | URL path parameter name | `id` → `/users/:id` |
| `query` | Query string parameter name | `page` → `?page=2` |
| `header` | Header name (lowercased) | `x-tenant-id` |
| `body` | Dot-notation JSON path | `user.email`, `items.0.sku` |
| `session` | Session store key | `currentUserId` |
| `store` | Global key/value store key | `featureFlag` |
| `literal` | Hardcoded string value | `"active"` |

### `CollectionTrace`

```go
type CollectionTrace struct {
    MappingID      string           `json:"mappingId"`
    MappingName    string           `json:"mappingName"`
    CollectionName string           `json:"collectionName"`
    Operation      CollectionOpType `json:"operation"`
    OutputKey      string           `json:"outputKey"`
    DurationMs     float64          `json:"durationMs"`
    RecordCount    int              `json:"recordCount"`
    Error          string           `json:"error,omitempty"`
}
```

---

## Uniform Document Return

Every operation returns the full post-operation document, regardless of type. This differs from
the Starlark `CollectionBuiltin` (which returns `None` for `insert` and a count for `update` /
`remove`) — the Go-level executor handles this internally.

| Operation | Returned value under `{{.Collection.<key>}}` |
|---|---|
| `find-one` | The matched document (or nil if no match) |
| `find-many` | Slice of matched documents (may be empty) |
| `insert` | The inserted document, with `_id` auto-assigned if absent |
| `update` | The first post-update document |
| `upsert` | The post-upsert document (created or updated) |
| `delete` | The deleted document as it existed before removal |

Nil / empty results: `{{if .Collection.user}}` guards work as expected.

Template examples:

```
{{.Collection.newUser._id}}
{{.Collection.user.name}}
{{range .Collection.results}}{{._id}}: {{.name}}
{{end}}
{{if .Collection.user}}Found: {{.Collection.user.email}}{{end}}
```

---

## Storage Interface Additions

Only mapping CRUD is new. Collection data itself lives in the existing session store — no new
storage methods for documents are needed.

```go
// Mappings — added to internal/storage/interface.go
GetCollectionMappingsByResponse(responseConfigID string) ([]*models.CollectionMapping, error)
GetCollectionMapping(id string) (*models.CollectionMapping, error)
CreateCollectionMapping(m *models.CollectionMapping) error
UpdateCollectionMapping(m *models.CollectionMapping) error
DeleteCollectionMapping(id string) error
DeleteCollectionMappingsByResponse(responseConfigID string) error
```

### Backend Notes

**Memory** (`memory.go`): in-process map, keyed by ID; filtered by `ResponseConfigID` in a loop.

**File** (`file.go`): delegates to memory; persists to `collection_mappings.json` on disk.

**MongoDB** (`mongo.go`): requires `response_config_id` BSON promotion. That field is already
promoted for response-level `ScriptBinding` — verify it is also set on `CollectionMapping` write
paths and that the existing index covers it.

---

## New Package: `internal/collection`

```
internal/collection/
  executor.go      — Executor: runs mappings against SessionState, returns output + traces
  executor_test.go
  resolver.go      — SourceValue: extracts a concrete value from RequestContext by SourceType
  resolver_test.go
  ops.go           — Go-level collection operator wrapping SessionState (__col__ key format)
  ops_test.go
```

### `ops.go` — Go-Level Collection Operator

The Starlark `CollectionBuiltin` methods `load()` and `save()` are unexported. Rather than
exposing them, `ops.go` implements an equivalent Go-level operator that works directly on
`SessionState` using the same `models.CollectionKeyPrefix + name` key and the same JSON-array
document format. This ensures byte-level compatibility: data written by the mapper is readable
by Starlark scripts via `store.collection("name")` and vice versa.

```go
// CollectionOps performs Go-level collection operations against a SessionState.
// The storage format matches store.CollectionBuiltin exactly.
type CollectionOps struct {
    sess store.SessionState
    name string
}

func NewCollectionOps(name string, sess store.SessionState) *CollectionOps

func (c *CollectionOps) FindOne(filter map[string]any) (map[string]any, error)
func (c *CollectionOps) FindMany(filter map[string]any) ([]map[string]any, error)
func (c *CollectionOps) Insert(doc map[string]any) (map[string]any, error) // returns doc with _id
func (c *CollectionOps) Update(filter, changes map[string]any) (map[string]any, error) // returns post-update
func (c *CollectionOps) Upsert(filter, data map[string]any) (map[string]any, error)
func (c *CollectionOps) Delete(filter map[string]any) (map[string]any, error)       // returns deleted doc
```

### `Executor`

```go
type Executor struct {
    store storage.Storage // for loading CollectionMapping configs
}

// Run executes all enabled mappings for a response config in Order order.
// sess is the active request session — the same one passed to the script engine.
func (e *Executor) Run(
    ctx context.Context,
    responseConfigID string,
    req *RequestContext,
    sess store.SessionState,
) (map[string]any, []models.CollectionTrace, error)
```

`RequestContext` mirrors `scripting.Input`: resolved path params, query params, headers,
raw body string, parsed body map, session reader, global store reader.

---

## Template Context Extension

Add `Collection map[string]any` to `TemplateData` in `internal/template/engine.go` alongside
the existing `Script map[string]any`:

```go
type TemplateData struct {
    // ... existing fields unchanged ...
    Script     map[string]any // {{.Script.pricing.total}}
    Collection map[string]any // {{.Collection.user.name}}
}
```

`template.Context` gains `CollectionOutput map[string]any`. `buildBodyTemplateContext` maps it
to `TemplateData.Collection` using the same path-walk helper as `Script`.

---

## Proxy Engine Integration

Mappers run **after** response-level scripts and **before** template rendering:

```
proxy/engine.go — after RunResponseBindings (~line 802):

collOut, collTraces := e.collectionExecutor.Run(ctx, matchedConfig.ID, reqCtx, sess)
templateCtx.CollectionOutput = collOut
traceRecord.Collections = collTraces
```

The active `sess` (already used by script bindings) is passed directly to the executor,
ensuring scripts and mappers share the same session state within a request.

---

## Admin API Endpoints

New handler file: `internal/api/mappings.go`

| Method | Path | Description |
|---|---|---|
| GET | `/_api/responses/:id/mappings` | List mappings for a response config |
| POST | `/_api/responses/:id/mappings` | Create mapping |
| GET | `/_api/mappings/:id` | Get mapping |
| PUT | `/_api/mappings/:id` | Update mapping |
| DELETE | `/_api/mappings/:id` | Delete mapping |

No new endpoints for collection documents — existing sessions/store APIs already expose the raw
session data (including `__col__*` keys). A convenience filter can be added to the store API
(`GET /_api/store?prefix=__col__`) to list all active collection names.

---

## UI Design

### Updated: Response Config Editor — "Collection Mappings" Tab

`ResponseConfigEditor.tsx` and `ResponseConfigIDE.tsx` gain a **Collection Mappings** tab
(same placement as the Scripts tab).

```
Response: 201 Created
Tabs: [Body / Headers] [Conditions] [Scripts] [Collection Mappings]

Collection Mappings tab:
┌──────────────────────────────────────────────────────────────────────────┐
│  [+ Add Mapper]                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  #1  insert  →  users     outputKey: newUser  ☑   [Edit]  [Delete]      │
│  #2  find-one → roles     outputKey: role     ☑   [Edit]  [Delete]      │
└──────────────────────────────────────────────────────────────────────────┘
```

---

### Visual Mapper Editor

Opens as a slide-over panel. The "visual" experience is the direct left→right rule rows and the
live Output Preview — no full drag canvas required.

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│  Mapper: [Lookup User                    ]                                      │
│  Collection: [users            ]  (type name or pick from known collections)   │
│  Operation:  [find-one ▼]    Output Key: [user    ]    Order: [1]   ☑ Enabled  │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌── FILTER — which record(s) to locate ────────────────────────────────────┐  │
│  │                                                                           │  │
│  │  SOURCE                              →   COLLECTION FIELD                │  │
│  │  ┌─────────────────────────────┐     →   ┌──────────────────────────┐   │  │
│  │  │ path  ▼ │ id                │ ──────▶  │ _id                  [×] │   │  │
│  │  └─────────────────────────────┘          └──────────────────────────┘   │  │
│  │                                                                           │  │
│  │  [+ Add filter rule]                                                      │  │
│  └───────────────────────────────────────────────────────────────────────────┘  │
│                                                                                 │
│  ┌── DATA — fields to write ─────────────────────────────────────────────────┐  │
│  │  (hidden for find-one / find-many / delete)                               │  │
│  └───────────────────────────────────────────────────────────────────────────┘  │
│                                                                                 │
│  ┌── OUTPUT PREVIEW — copy into your response template ─────────────────────┐  │
│  │  {{.Collection.user._id}}      {{.Collection.user.name}}                 │  │
│  │  {{.Collection.user.email}}    {{.Collection.user.status}}               │  │
│  │                                                                           │  │
│  │  Fields inferred from existing records in this collection.               │  │
│  └───────────────────────────────────────────────────────────────────────────┘  │
│                                                          [Cancel] [Save Mapper] │
└─────────────────────────────────────────────────────────────────────────────────┘
```

**Collection name input**: free-text (collections are created lazily on first insert).
Auto-suggests known names by scanning `__col__*` keys from the store API.

**Source dropdown (left side of each rule row):**
- Type: `path | query | header | body | session | store | literal`
- Key: autocompletes from OpenAPI spec parameters and request body schema fields

**Collection field input (right side of each rule row):**
- Free-text; field names are suggested by scanning documents already in the collection
  (fetch `store.get("__col__<name>")` from the store API and collect all field names)

**Section visibility by operation:**

| Operation | Filter Rules | Data Rules |
|---|---|---|
| `find-one` | shown | hidden |
| `find-many` | shown | hidden |
| `insert` | hidden | shown |
| `update` | shown | shown |
| `upsert` | shown | shown |
| `delete` | shown | hidden |

**Output Preview**: regenerates live as output key or collection changes. Shows `_id` plus
all field names inferred from existing records. Each token is click-to-copy.

---

### Collection Data Visibility in Existing UIs

No separate Collections Manager is needed. Collection data is visible through existing UIs:

- **Store Manager** — already shows all session store keys. `__col__*` entries appear here as
  raw JSON arrays. Optional: add a "Collections" sub-filter tab that parses these entries and
  displays them as a record table.
- **Trace Viewer** — gains a **Collection** section showing `CollectionTrace` entries
  (mapper name, operation badge, collection name, record count, duration, error).

---

## Implementation Phases

### Phase 1 — Models & Storage (est. 2 days)

- [ ] `internal/models/collection.go` — CollectionMapping, FieldMappingRule,
      CollectionOpType constants, CollectionTrace
- [ ] `internal/storage/interface.go` — add mapping CRUD methods only
- [ ] `internal/storage/memory.go` — implement mapping CRUD
- [ ] `internal/storage/file.go` — delegate to memory + persist to `collection_mappings.json`
- [ ] `internal/storage/mongo.go` — implement; verify `response_config_id` is promoted + indexed
- [ ] Unit tests for all three backends

### Phase 2 — Collection Executor (est. 2 days)

- [ ] `internal/collection/ops.go` — Go-level `CollectionOps` wrapping `SessionState`
      using `models.CollectionKeyPrefix`; same document format as `store.CollectionBuiltin`
- [ ] `internal/collection/resolver.go` — `FieldMappingRule` + `RequestContext` → concrete value
- [ ] `internal/collection/executor.go` — runs mappings, returns `map[string]any` output + traces
- [ ] Unit tests (ops, resolver, executor)

### Phase 3 — Proxy Engine Integration (est. 1 day)

- [ ] Add `CollectionOutput map[string]any` to `template.Context`
- [ ] Add `Collection map[string]any` to `template.TemplateData`
- [ ] Update `buildBodyTemplateContext` to populate `.Collection`
- [ ] Wire `collection.Executor.Run` into `proxy/engine.go` (after `RunResponseBindings`,
      pass the existing `sess`)
- [ ] Append `[]CollectionTrace` to `models.Trace`

### Phase 4 — Admin API (est. 1 day)

- [ ] `internal/api/mappings.go` — mapping CRUD handlers
- [ ] Register routes in `internal/api/router.go`
- [ ] Optional: `GET /_api/store?prefix=__col__` filter on existing store API
- [ ] Handler tests

### Phase 5 — UI: Visual Mapper in Response Editor (est. 4–5 days)

- [ ] TypeScript types: `CollectionMapping`, `FieldMappingRule`
- [ ] API service functions in `ui/src/services/api.ts` for mapping CRUD
- [ ] `ui/src/components/CollectionMapper/` — mapper list + visual editor slide-over
- [ ] Integrate "Collection Mappings" tab into `ResponseConfigEditor.tsx`
- [ ] Integrate same tab into `ResponseConfigIDE.tsx`
- [ ] Source-key autocomplete from OpenAPI spec parameters + request body schema
- [ ] Collection name autocomplete from store API (`__col__*` keys)
- [ ] Collection field suggestions from existing records in the collection
- [ ] Output Preview panel with click-to-copy tokens
- [ ] Section visibility per operation type (filter / data)

### Phase 6 — Polish & Integration (est. 1–2 days)

- [ ] `{{.Collection.*}}` variable hints in response body editor
- [ ] Collection section in `TraceViewer.tsx`
- [ ] Archive export/import — include `collection_mappings` in snapshots
- [ ] End-to-end smoke tests: insert via POST, find via GET (same session)

---

## Open Questions

1. **`_id` auto-assignment**: `store.CollectionBuiltin.insert` does not assign `_id`
   automatically. The `CollectionOps.Insert` in the executor must assign one (e.g. `uuid.New()`)
   if the document does not already contain an `_id` field. Should this be consistent with how
   scripts call `store.collection().insert()`, or should auto-ID remain executor-only?

2. **Cross-session data**: Collection data is session-scoped. Users who want truly global,
   cross-session collections (e.g. a shared counter) would need to use the global store directly
   via scripts. Should we document this limitation clearly, or add a future `GlobalCollectionOps`
   variant backed by `GlobalStoreBackend`?

3. **Field name inference**: The Output Preview and field autocomplete depend on scanning
   existing records from the store API. For a brand-new collection (no records yet), the field
   picker is empty. Should we allow users to manually declare expected fields in the mapper UI
   (lightweight schema hint, not persisted to a model)?

4. **Transactional semantics**: Multiple mappers execute sequentially. If mapper #2 fails,
   mapper #1's write already committed to the session. Best-effort (no rollback) is consistent
   with script binding behavior today.

---

## Non-Goals (this iteration)

- A separate Collection entity with a schema model and dedicated storage
- A new Starlark built-in — the existing `store.collection("name")` is sufficient
- Cross-session / globally-shared collection data (use global store + scripts for that)
- Full query operators beyond equality filtering (use Starlark scripts for complex queries)
- CSV / JSON bulk record import
