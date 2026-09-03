# Collection-Backed Response Plan

## Refinement Summary

This revision changes two core semantics from the first draft and adds one capability:

1. **The collection query is part of response matching.** A Collection Response carries an implicit condition: *the primary mapper returned data*. An empty result means the response does not match, and matching falls through to the next response in priority order. The previous draft's empty-result policy (404/null) is replaced by fall-through.
2. **The template is the spec's own response schema, not a sample-derived projection tree.** Rendering uses a schema-fill engine: the operation's OpenAPI response body (example or schema-generated) is the structural template, and mapper output plus request context fill it **by naming convention**. Users only configure exceptions, not the whole tree.
3. **Additional mappers.** Beyond the primary query, a response can configure extra named collection queries whose outputs fill specific fields, and every mapper's query filters can be bound to request data.

## Product Idea

Add a new configured response kind called **Collection Response**.

A Collection Response appears in the same response list as a manually authored response, participates in the same priority ordering, and uses the same enabled state, tags, explicit conditions, status code, headers, and delay. It differs in two ways:

- **Selection**: it wins only if its primary collection query returns data.
- **Rendering**: its body is produced by filling the operation's spec-defined response shape with collection data, instead of rendering a user-authored Go template.

The authoring model is intentionally minimal:

1. Choose a collection and map request values into its query filters.
2. The response template comes from the OpenAPI spec automatically.
3. Convention fills every template field that shares a name with a document field.
4. Optionally add extra mappers or per-field overrides for the fields convention cannot fill.

Users never write a collection mapping record, Go template, or `range` block for this response kind.

## Why This Should Be a Response Kind

- It is itself a selectable response, not a pipeline step.
- Its query result *is* the matching signal and the response body, not an intermediate `.Collection.<outputKey>` value.
- The query, mappers, and overrides save atomically with the response.
- Modeling it as a `ResponseConfig` variant preserves the current response list, ordering, and drag-and-drop behavior. Internally it reuses `collection.Ops.FindOne`/`FindMany` and the filter resolver without creating hidden `CollectionMapping` records.

## Terminology

- **Explicit conditions** are the normal user-configured response conditions.
- **Data-presence condition** is the implicit condition that the primary mapper returned data.
- **Query filters** select document(s) inside the collection; they are built from request bindings.
- **Primary mapper** is the query that gates matching and provides the main fill data.
- **Additional mapper** is a named secondary query used only to fill specific fields.
- **Template** is the JSON tree derived from the operation's spec response for the configured status code.
- **Convention fill** is the automatic mapping of a template path to the same document path.
- **Override** is an explicit per-field source that replaces convention for one template path.

## Response Selection Semantics

Matching walks the ordered response list exactly as today, with one addition for Collection Responses:

```text
for each enabled response, by priority:
    evaluate tags + explicit conditions          (no I/O, unchanged)
    if manual and conditions pass        -> match
    if collection and conditions pass:
        resolve primary filters from request
        execute primary query (find-one / find-many)
        data returned                    -> match (result cached for render)
        empty result                     -> not a match; continue to next response
```

Rules:

1. Explicit conditions run first, so the query executes only for Collection Responses that already pass their cheap checks. Most requests run zero or one collection query during matching.
2. The primary query executes **at most once per request per response** — the matched result is carried into rendering, never re-queried.
3. "Data returned" means: `find-one` found a document, or `find-many` returned at least one document.
4. A `matchOnEmpty` flag (default `false`) opts a response out of the data-presence condition: with it set, an empty `find-many` still matches and renders `[]`, and an empty `find-one` matches and renders `null`. This covers legitimate empty-list endpoints without weakening the default convention.
5. If no response matches at all (including every Collection Response falling through), the existing no-match behavior of the operation applies unchanged (example fallback, proxy, AI, or 404 per current mode policy).
6. A collection backend error during matching does **not** fall through — it stops matching and returns `500` with the standard runtime error body, recorded in the trace. Falling through on infrastructure errors would silently mask outages.

Tracing must record each Collection Response that was *attempted* during matching, its resolved filter, and whether it matched, fell through empty, or errored.

## Rendering: Schema-Fill Engine

Rendering a Collection Response does not use the Go-template engine. A dedicated schema-fill engine combines three inputs:

- **Template** — the spec-derived response body for the response's status code.
- **Data** — the primary mapper result plus any additional mapper results.
- **Request context** — path, query, header, and GJSON body values.

### Template Source

The parser already extracts per-status-code response bodies from the spec (direct example, named examples, or `generateExampleFromSchema`), surfaced as `SpecResponseDef` entries. The engine uses that JSON as the template tree:

- The template for the response is the spec response body matching the configured status code; when multiple named examples exist, the user picks one (`templateRef` stores the choice).
- The template's root shape derives the query mode: **object root → `find-one`, array root → `find-many`**. The operation is displayed but not independently editable, so invalid combinations cannot be configured.
- For an array root, the first item of the template array is the **item template**, applied independently to every document returned by `find-many`.
- If the spec defines no JSON body for the status code, the engine uses **identity mode**: the document (or array of documents) is serialized as-is, with overrides still applied by target path. Root shape must then be chosen manually.

Because the template is read from the spec at render time, the response body automatically tracks spec updates. Overrides pointing at template paths that no longer exist are skipped with a trace/preview warning, never an error.

### Convention Fill

For each leaf path in the template, in order of precedence:

1. **Override** — if an override targets this path, resolve its binding.
2. **Convention** — read the same dot path from the current document (`customer.name` template field ← `customer.name` document field), matched case-insensitively on the last segment when an exact match is absent.
3. **Template fallback** — if the document lacks the path, keep the template's example value when `fallbackToExample` is set (default `true`); otherwise emit `null`. Either way the preview flags it.

Structural rules:

- Nested template objects recurse with the corresponding document subtree as the current document.
- A template array whose items are objects, backed by a document array, applies the item template per element. A shape mismatch (template expects array, document holds a scalar) emits the document value as-is with a warning; v1 performs no type coercion to schema types.
- Document fields with no matching template path are **not** emitted — the spec shape is authoritative. (An `includeExtraFields` escape hatch is deferred.)
- The engine builds native Go values and calls `json.Marshal`. It never generates template text, so escaping, JSON types, and array punctuation are always correct, and stored collection documents are never mutated.

## Mappers

### Primary Mapper

- One collection name plus a list of query filters.
- Each filter binds a collection field path to a request value or literal:

| Collection field | Source | Key |
| --- | --- | --- |
| `_id` | Path | `id` |
| `status` | Query | `status` |
| `tenantId` | Header | `X-Tenant-Id` |
| `customer.email` | JSON body | `customer.email` |
| `active` | Literal | `true` |

- Supported sources: path parameter, query parameter, request header (case-insensitive), request JSON body via existing GJSON syntax, JSON literal. Pickers prefer the operation's declared inputs while allowing custom keys. Body and literal values preserve JSON types (number, boolean, null, object, array).
- No filters means "first document" for `find-one` and "all documents" for `find-many`.
- A filter whose bound request value is missing resolves to JSON `null` for the query and is flagged in preview; it does not error.

### Additional Mappers

Optional named queries used purely for filling fields — they never affect matching:

- Each has an `outputKey`, collection name, mode (`find-one`/`find-many`), and its own filter list.
- Filter sources: everything the primary mapper supports, **plus `primary`** — a field of the primary result document, enabling lookups like *orders where `customerId` = primary doc's `id`*. The `primary` source is only valid when the primary mode is `find-one` (single current document).
- Additional mappers execute once per request, after the response wins and before rendering, in declared order. Per-item execution for array roots (true joins) is deferred.
- An empty additional result does not unmatch the response; fields bound to it fall back per the convention-fill rules.
- Their results are addressable in overrides as `mapper:<outputKey>.<path>`.

### Overrides

A sparse list, not a full tree — convention handles everything else:

- `targetPath` — dot path in the template (relative to the item template for array roots).
- Value binding, one of:
  - `document` — a different path in the current primary document (rename/re-map)
  - `mapper` — `<outputKey>.<path>` into an additional mapper result
  - `path` / `query` / `header` / `body` — request context
  - `literal` — typed JSON literal
- Context and literal bindings evaluate once per request and repeat identically on every array item; `document` bindings vary per item.

Example configuration:

```text
Collection: users            (primary)
Template:   GET /users 200 spec response (array root -> Find Many)
Filter:     tenantId <- Header[X-Tenant-Id]

Additional mapper "plan": collection plans, find-one, filter _id <- primary[planId]

Overrides
|- displayName <- document[profile.name]      (rename)
|- planLabel   <- mapper:plan.label           (lookup)
|- requested   <- query[include]              (request context)
`- active      <- literal true
```

Everything else in the template (`id`, `email`, `orders`, …) fills by convention from each user document.

## Proposed Data Model

```go
type ResponseKind string

const (
    ResponseKindManual     ResponseKind = "manual"
    ResponseKindCollection ResponseKind = "collection"
)

type ResponseConfig struct {
    // Existing fields remain unchanged.
    Kind               ResponseKind              `json:"kind,omitempty"`
    CollectionResponse *CollectionResponseConfig `json:"collectionResponse,omitempty"`
}

type CollectionResponseConfig struct {
    Primary           CollectionQuery  `json:"primary"`
    AdditionalMappers []NamedQuery     `json:"additionalMappers,omitempty"`
    Overrides         []FieldOverride  `json:"overrides,omitempty"`
    TemplateRef       string           `json:"templateRef,omitempty"`   // named spec example; empty = default for status code
    RootKind          RootKind         `json:"rootKind,omitempty"`      // auto (default) | object | array; only meaningful in identity mode
    MatchOnEmpty      bool             `json:"matchOnEmpty,omitempty"`
    FallbackToExample *bool            `json:"fallbackToExample,omitempty"` // nil = true
}

type CollectionQuery struct {
    CollectionName string             `json:"collectionName"`
    FilterRules    []CollectionFilter `json:"filterRules,omitempty"`
}

type NamedQuery struct {
    OutputKey string    `json:"outputKey"`
    Mode      QueryMode `json:"mode"` // find-one | find-many
    CollectionQuery
}

type CollectionFilter struct {
    TargetPath string       `json:"targetPath"`
    Value      ValueBinding `json:"value"`
}

type FieldOverride struct {
    TargetPath string       `json:"targetPath"`
    Value      ValueBinding `json:"value"`
}

type ValueBinding struct {
    Source ValueSource     `json:"source"` // document | mapper | primary | path | query | header | body | literal
    Key    string          `json:"key,omitempty"`
    Value  json.RawMessage `json:"value,omitempty"` // literal only
}
```

Notes:

- An omitted or empty `kind` normalizes to `manual`, preserving every existing response file and MongoDB document without migration.
- `ValueBinding` is typed (raw JSON literals) because the current collection resolver returns strings and would corrupt `42`, `true`, and `null`. One typed request-value resolver serves query filters, `primary` bindings, and overrides; it may later replace the string-only resolver used by ordinary collection mappings, but that migration is not required for v1.
- `primary` as a source is valid only inside additional-mapper filters; `mapper` is valid only inside overrides; `document` is valid only inside overrides.

## Runtime Semantics

```text
spec/operation pipeline (unchanged)
        |
ordered response matching (manual + collection together)
        |   collection responses run their primary query here;
        |   empty result = fall through to next response
        |
winning response kind
        |-- manual:     existing pipeline -> Go-template render (byte-for-byte unchanged)
        `-- collection: cached primary result
                        -> run additional mappers
                        -> schema-fill (template x data x request context x overrides)
                        -> json.Marshal
                        -> status, headers, delay
```

1. Spec- and operation-scope pipeline steps run as today; their outputs remain usable in explicit conditions.
2. Matching follows the Response Selection Semantics above.
3. On a Collection Response win: additional mappers run, then the schema-fill engine renders the body.
4. Configured status code, headers, and delay apply exactly as for manual responses. Header values may keep using the existing template context; the body never does.
5. Traces record: response kind, template source, per-mapper resolved filters and record counts, matched/fell-through/errored outcome per attempted response, and fill warnings.
6. Errors after matching (additional mapper failure, fill error, marshal error) return `500` with the standard runtime error body and a trace entry.

### Response-Level Pipelines

Do not expose response-level script or collection pipeline attachments for this kind in v1; additional mappers cover the data-lookup need declaratively. The backend rejects creating response-scoped bindings for a Collection Response. Existing manual responses and their pipelines are untouched.

## User Experience

### Creating a Response

The add-response action offers **Manual Response** or **Collection Response**. Both kinds live in one sortable list; a compact `Manual`/`Collection` badge identifies the kind, and drag-and-drop keeps updating the shared `priority` field. Kind is immutable after first save in v1; cloning preserves it.

### Collection Response Editor

Four focused sections:

**General** — reuse name/description, tag, enabled, priority, explicit conditions, status code, headers, delay. Beside the conditions, show a fixed non-removable row: *"Primary query returns data"* with the `matchOnEmpty` toggle — making the implicit condition visible where users look for matching logic.

**Data Source** — primary collection selector; filter rows (collection field ← source picker ← key, sources preferring declared operation inputs); derived operation label (`Find One`/`Find Many`) read from the template root; template picker when the status code has multiple named spec examples; identity-mode notice + manual root control when the spec has no body.

**Additional Mappers** — compact list of named queries: output key, collection, mode, filters (including the `primary[...]` source). Reorderable; icon-driven add/remove like other pipeline rows.

**Output** — a read-only tree of the spec template with a per-field source chip: `Auto` (convention), or the override source. Clicking a field opens the override picker (document path / mapper output / request context / literal). No add/remove/rename of tree nodes — the spec owns the shape, which is what keeps this editor small.

### Preview

- Editable example values for every request input referenced by filters or overrides.
- Shows, per mapper: resolved filter, operation, record count; and the final rendered JSON.
- Per-field diagnostics: filled by convention, overridden, fell back to example, or `null` (missing document path).
- A matching banner: "with these inputs, this response would match / fall through (empty result)".
- Backed by a read-only endpoint that must not mutate collection or session state.

Collection document sampling is no longer used to build the output tree (the spec is authoritative); it remains useful in preview to warn when sampled documents' types disagree with the template shape.

## API and Validation

Extend the existing response create, get, update, list, clone, export, import, and archive payloads — no second CRUD API.

Server validation for a Collection Response requires:

- known response kind; non-null `collectionResponse`; empty manual `body`
- non-empty primary collection name; valid filter target paths
- allowed sources per binding site (`primary` only in additional-mapper filters, etc.) and complete keys/values
- unique, non-empty additional-mapper output keys; `primary` source rejected when primary mode is `find-many`
- valid GJSON body paths and JSON-decodable literals
- override target paths that are syntactically valid (existence against the template is a warning, not an error, since specs evolve)
- `rootKind` only when the operation lacks a spec body for the status code

Manual responses reject a non-null `collectionResponse`.

Preview endpoint:

```text
POST /_api/operations/:id/collection-responses/preview
```

Accepts an unsaved definition plus example request context; returns match outcome, per-mapper resolved filters and counts, rendered body, and per-field diagnostics.

## Backend Implementation Plan

1. **Models and normalization** — kind constants and `EffectiveKind()` on `ResponseConfig`; new model file for `CollectionResponseConfig` and friends; normalize absent kind to manual at API/storage boundaries; field-specific validation errors for the UI.
2. **Typed request resolver** — alongside `internal/collection/resolver.go`; preserves native JSON values from body paths and literals; path/query/header values are JSON strings in v1; returns `(value, found, error)`; shared by filters, `primary` bindings, and overrides.
3. **Template extraction** — reuse the parser's spec-response extraction to resolve the template JSON for (operation, status code, templateRef) with caching; expose the same resolution to the preview endpoint and the UI's output tree.
4. **Schema-fill service** — new `internal/collectionresponse/` package: match-phase query execution (`TryMatch` returning cached result / fall-through / error), additional-mapper execution, recursive convention fill with overrides, structured result (value, counts, resolved filters, warnings, trace data). Unit-tested independently of HTTP.
5. **Proxy integration** — extend response matching in the proxy engine to call `TryMatch` for collection kinds and carry the cached result to `serveMatchedConfig`; branch on `EffectiveKind()` for rendering; keep manual rendering byte-for-byte compatible; wire delay/headers/trace/metrics.
6. **API, clone, archive, import** — kind-specific validation in create/update; clone preserves nested config; file and Mongo serialization include it (it rides inside the response JSON blob, so no new Mongo field promotion is needed — verify against the field-promotion checklist anyway); archive/import round-trip; reject response-level bindings for this kind.

## Frontend Implementation Plan

1. **Types and API client** — `ResponseKind`, config/binding/override/preview types in `ui/src/types/index.ts`; extend existing response inputs; one preview client method.
2. **Unified response list** — kind badge plus collection name/root summary in `ResponseConfigList`; existing drag ordering, enable toggle, clone, copy/export, delete all preserved; copied/exported payloads include the new fields.
3. **Creation flow** — Manual/Collection choice routing into the existing response page with a `kind` initializer; kind immutable after save.
4. **Editor** — the four sections above; template tree rendered from the operation's spec response; source chips and override picker; additional-mapper rows reusing filter-row components.
5. **Preview and diagnostics** — example request inputs, match-outcome banner, per-field fill provenance, warnings; server validation authoritative before save.

## Test Plan

### Matching

- manual and collection responses compete in one priority order
- explicit conditions gate the query: failing conditions never execute the query
- non-empty result matches; empty result falls through to the next response (manual or collection)
- several collection responses in a row: each attempted in order, first with data wins
- `matchOnEmpty` renders `[]` (find-many) and `null` (find-one) instead of falling through
- all responses fall through → existing no-match behavior fires
- backend error during matching → `500`, no fall-through, trace recorded
- primary query executes exactly once per request per response (no re-query at render)

### Template and Fill

- object template → `find-one`; array template → `find-many`; item template applied per document
- convention fill: exact path match, case-insensitive last-segment match, nested objects, nested object arrays
- precedence: override > document value > example fallback > null
- `fallbackToExample=false` yields nulls; warnings emitted either way
- document fields absent from the template are not emitted
- identity mode when spec has no body; manual root kind honored
- template tracks spec updates; stale override paths skipped with a warning
- JSON types preserved end-to-end; escaping of quotes, backslashes, newlines, Unicode
- fill never mutates stored collection documents

### Mappers, Filters, Overrides

- path, query, case-insensitive header, GJSON body, and literal filter bindings, with types preserved
- missing request input resolves to null filter value plus warning
- additional mapper with `primary[...]` filter performs the lookup; rejected for find-many primaries
- additional mappers run in order, once per request; empty results fall back cleanly
- overrides: document rename, mapper output, request context (constant across array items), typed literals

### API and Persistence

- absent kind normalizes to manual; manual/collection fields cannot be mixed
- create/update/list/get/clone round-trip the nested config on all three storage backends
- archive export/import round-trip; response-level binding APIs reject Collection Response IDs
- invalid filters, literals, sources-per-site, and duplicate output keys return field-specific `400`s

### UI

- creation choice, immutable kind, mixed-list ordering and badges
- template tree from spec, source chips, override picker round-trip
- additional-mapper editing and `primary` source availability rules
- preview: match banner, fill provenance, warnings

## Delivery Sequence

1. Models, normalization, validation, serialization tests.
2. Typed resolver and template extraction/caching.
3. Schema-fill service with match-phase query API; full unit coverage.
4. Proxy matching + rendering integration and tracing.
5. Response API extensions, clone, archive, import/export, binding guards.
6. Frontend types, creation choice, list badges.
7. Editor: data source, additional mappers, output tree with overrides.
8. Preview endpoint + UI diagnostics; end-to-end object/array/fall-through scenarios.

## V1 Acceptance Criteria

- A Collection Response is displayed beside manual responses and reorders with them.
- It participates in priority + explicit-condition selection, and additionally matches only when its primary query returns data; on empty it falls through to the next response.
- Object templates perform `find-one`; array templates perform `find-many`; the mode is derived, never hand-picked (outside identity mode).
- Query filters — for the primary and any additional mapper — can source values from path, query, header, GJSON body paths, or typed literals.
- The body is the spec's response shape, filled by naming convention from the primary document, with per-field overrides from document paths, additional mapper outputs, request context, or literals.
- Rendering emits valid typed JSON without user-authored templates and never mutates stored documents.
- Fall-through, `matchOnEmpty`, error handling, and trace visibility are documented and tested.
- Existing manual responses remain backward compatible.

## Deliberately Deferred

- per-item additional-mapper execution for array roots (true joins)
- type coercion of document values to schema types
- `includeExtraFields` (emitting document fields absent from the template)
- non-JSON content types and content-type negotiation
- query pagination, sorting, limit, and operators beyond equality
- filter/override values sourced from scripts, validation outputs, session, or global store
- per-field expressions or transforms
- converting an existing response between manual and collection kinds
- editable projection trees decoupled from the spec schema

Each can be added later without weakening the v1 mental model: *one ordered response that matches when its data exists, one spec-shaped template, filled by convention.*
