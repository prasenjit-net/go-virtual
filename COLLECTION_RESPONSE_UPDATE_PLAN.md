# Collection Response Mapper Operations Plan

Status: implemented. See [operation usage and behavior](docs/collection-response-operations.md). Browser interaction verification remains unavailable in this environment.

## Scope

Extend the main response mapper to support **Update** alongside Find One and Find Many. Its query filters still determine whether the response is selected. When an Update response wins, update the document found during selection and use the updated document as the main input for response rendering.

Extend additional mappers to support all six existing collection operations: **Insert, Find One, Find Many, Update, Upsert, and Delete**. They execute only for the selected response and expose their operation results through their existing output keys.

There is no separate `updateMappers` list or Updates tab. Updates belong to the main mapper or to an entry in the existing additional mapper list.

Example: the main mapper uses Update on `orders`, with query filters `id = path.id` and `status = pending`, and data mapping `status = confirmed`. The query selects the response from pending state; only after selection does the update run. Rendering receives the confirmed order even though it no longer matches the original filter.

## Current implementation

- `internal/models/collection_response.go` models the primary query and read-only `NamedQuery` additional mappers.
- `internal/collectionresponse/service.go` queries in `TryMatch`, caches the primary result, then runs additional reads and fills the template in `Render`.
- `internal/proxy/engine.go` selects a response before rendering it.
- `internal/collection/ops.go` already implements all six operations through session collection events. Update and Upsert can return the wrong result when changed fields invalidate the original filter; fix result tracking as part of this work.
- `internal/collection/executor.go` dispatches all six operations for existing pipeline mappings, but uses a different binding model and injects `_status` metadata. Reuse operation primitives without changing collection-response output shapes or adopting its continue-on-error behavior.

## Configuration and compatibility

Extend the existing mapper configuration in place:

| Configuration | Proposed fields and behavior |
| --- | --- |
| `primary` | Keep `collectionName` and `filterRules`; add optional `mode` and typed `dataRules` |
| Main `mode` | `find-one`, `find-many`, or `update` |
| `additionalMappers[]` | Keep `outputKey`, `collectionName`, `filterRules`, and `mode`; allow all six modes and add typed `dataRules` |
| `dataRules[]` | `targetPath` plus `value: ValueBinding`, preserving JSON types |

Replace or generalize the read-only `NamedQuery` type into a response mapper type while keeping its JSON field names stable. Keep query-mode types for selection diagnostics restricted to reads; use a separate operation type for configurable mapper modes, reusing `CollectionOpType` where appropriate.

An omitted main mode preserves current behavior: derive Find One or Find Many from the spec response shape, or from `rootKind` in identity mode. Existing additional mapper modes retain their meaning. Old configurations need no migration.

Explicit Find One requires an object response root; Find Many requires an array root. The proposed first version of main Update updates one document and requires an object root, consistent with existing `Ops.Update`. Reject Update with an array root rather than silently adding update-many semantics. The UI explains this restriction. Insert, Upsert, and Delete are supported as additional operations, not as main operations in this scope.

Example configuration fragment:

```json
{
  "primary": {
    "mode": "update",
    "collectionName": "orders",
    "filterRules": [
      {"targetPath": "_id", "value": {"source": "path", "key": "id"}},
      {"targetPath": "status", "value": {"source": "literal", "value": "pending"}}
    ],
    "dataRules": [
      {"targetPath": "status", "value": {"source": "literal", "value": "confirmed"}}
    ]
  },
  "additionalMappers": [
    {
      "outputKey": "audit",
      "mode": "insert",
      "collectionName": "order_events",
      "dataRules": [
        {"targetPath": "orderId", "value": {"source": "primary", "key": "_id"}},
        {"targetPath": "status", "value": {"source": "primary", "key": "status"}}
      ]
    }
  ]
}
```

## Operation contracts

| Operation | Filter | Data | Result available for rendering |
| --- | --- | --- | --- |
| Find One | Optional, as today | None | First document or null |
| Find Many | Optional, as today | None | Array of matching documents, possibly empty |
| Insert | None | Required | Inserted document including generated identity |
| Update | Required | Required | First matching document after changes, or null |
| Upsert | Required | Required | Updated document, or inserted document if no match |
| Delete | Required | None | Deleted document before removal, or null |

Additional mapper results remain plain documents/arrays/null under `outputKey`; existing overrides such as `mapper.audit._id` can consume them. Do not inject pipeline `_status` fields. Zero matches for an additional Update or Delete is a successful no-op, not a reason to select a different response.

Initial writes use existing top-level merge semantics. Reject dotted data targets rather than treating them as nested writes; complete typed nested objects may be assigned. Prevent Update/Upsert data from changing `_id`. Insert may retain the existing supported explicit-ID behavior; validate identity uniqueness before relying on identity-based mutation results.

## Selection and execution order

1. Apply priority, enabled state, tags, and explicit conditions as today.
2. In `TryMatch`, execute only the main mapper's selection query. Find One and Find Many behave as today. Update performs a read-only Find One using its query filters, caching the selected document and stable identity.
3. Apply existing data-presence and `matchOnEmpty` rules and select the winning response. No mutation, including an additional mapper mutation, happens during candidate evaluation.
4. In an explicit selected-response execution phase, resolve the main Update data against the request and pre-update primary snapshot. Update exactly the selected document by immutable identity; never perform a new broad search that could update another document.
5. Use the returned updated document as the main render input. For a read operation, retain the cached query result. Keep original matching diagnostics unchanged.
6. Execute additional mappers in their configured array order, dispatching all six operations. Each sees session state produced by earlier operations, including the main Update. Store each result under its output key.
7. Fill the spec template or identity body from the main result and additional outputs, using existing overrides and fallback settings.

Do not re-run the original query or reselect the response after a write. If Update changes a field used in selection, the updated document is still the main render input.

Additional mappers run once per response, not once per item in a Find Many result. Their `primary` bindings refer to the main operation's result, so they see post-update values. Additional mutations do not implicitly replace the main result snapshot: even if one later modifies or deletes the same stored document, its own output describes that operation; use its output in overrides when desired. This keeps the main mapper's result and every additional mapper's result well-defined.

With `matchOnEmpty`, a main Update can be selected with no document: it performs no write and produces null, never an implicit upsert. Additional request/literal-based operations may still run; a required primary binding without a primary document fails explicitly.

## Bindings and validation

- Preserve typed `ValueBinding` values, including false, zero, null, arrays, and objects.
- Main query filters retain request/literal sources. Main Update data also permits `primary`, referring to the pre-update selected document.
- Additional filter/data bindings support request/literal sources and `primary`, referring to the main result. Reject `primary` for array-root responses, as with existing additional query filters.
- Additional-to-additional binding dependencies are outside this change; existing `mapper` references remain available in rendering overrides.
- Validate operation-specific fields, required collection and unique output keys, allowed sources, nonempty write filters/data, duplicate targets, root shape, and forbidden identity changes.
- Reject filters for Insert and data for read/Delete operations; the editor clears obsolete operation-specific fields when a mode changes.
- Missing values in mutation filters/data must fail rather than silently dropping a filter or field. Distinguish an absent value from explicit null.
- Resolve and validate main Update bindings before writing. Resolve additional bindings against the main result and preflight them before additional writes where possible. A failure after the main update can still leave that update committed.

## Implementation steps

1. **Models and API**
   - Update `internal/models/collection_response.go` and `ui/src/types/index.ts` with main mode, generalized additional operations, and typed data rules.
   - Extend structural validation and `ValidateAgainstOperation`; preserve omitted-mode inference.
   - Verify create/update, cloning, storage, and archive import/export round trips without losing new fields.
2. **Shared operation correctness**
   - Fix Update and Upsert to track the selected/inserted identity and return the correct post-write document even when filter fields change.
   - Pin main Update to the document selected by `TryMatch`; reject missing/ambiguous identity before writing. If a previously selected target disappears, report an execution error rather than updating a replacement target.
   - Verify Insert result identity, Delete pre-removal output, event replay, and supported backend consistency. Keep existing pipeline callers compatible.
3. **Collection-response lifecycle**
   - Add a once-per-selected-response execution phase in `internal/collectionresponse/service.go` and invoke it from the winning path in `internal/proxy/engine.go`.
   - Separate side-effect execution from template filling so validation, previews, and rendering retries cannot accidentally repeat writes.
   - Dispatch additional operations through shared `collection.Ops` with typed bindings; preserve ordered result snapshots without aliasing mutable nested data.
   - Preserve lazy session materialization and return the session header for committed writes, including error paths.
4. **Traces and failure handling**
   - Record the main selection query separately from main Update execution and ordered additional mapper traces.
   - Include actual operation, output key, record count, duration, and errors in trace models and UI.
   - Preserve completed and failed operation traces on runtime errors; do not discard partial execution diagnostics.
5. **Editor**
   - In the main mapper's Query section, offer Find One / Find Many / Update, with shape compatibility guidance and a data mapping editor for Update.
   - Explain that query filters select the response, and Update runs only after selection; its result supplies the response body.
   - Expand the existing additional mapper Mode control to all six operations. Show filters and data fields according to the operation table.
   - Add ordering controls for additional mappers because reads and writes now have meaningful execution order. Preserve unique output-key editing and override references.
   - Verify save/reload, operation switching, validation, keyboard focus, and light/dark styling. Do not add a separate Updates tab.
6. **Documentation**
   - Update the original collection-response design documentation to describe main Update and all six additional operations.
   - Include examples for Update followed by audit Insert, Upsert followed by Find One, and Delete whose returned document fills an override.

## Failure and consistency contract

Candidate matching, configuration saves, validation, and template previews never mutate collections. Only the selected response executes mapper operations, once in that request.

Execution errors return the existing runtime error response; never fall through to another response after selection or retry a mutation phase automatically. Additional zero-match Update/Delete results are null; main Update with an initially empty match follows the explicit `matchOnEmpty` behavior above.

The existing session event store has no transaction across multiple mapper operations. If a later mapper or rendering fails, earlier committed writes remain. Preserve partial traces and document this behavior. Rollback, cross-request exactly-once execution, and stronger concurrent-request isolation are outside scope. All mutations remain session scoped and leave base collection data unchanged.

## Acceptance tests

- Existing configurations with no main mode or write data retain their behavior and JSON output.
- Main Update uses its query as a selection condition; rejected candidates and lower-priority responses write nothing.
- A main query matches pending state, Update changes it to confirmed, and rendering receives confirmed state without repeating the original query.
- Main Update affects the selected identity only, even when multiple documents match, and rejects incompatible array roots.
- Cover main Update `matchOnEmpty`, missing bindings, disappeared targets, and identity validation.
- Exercise all six additional operations and their exact output shapes, including Insert IDs, Update/Upsert post-write values, and Delete pre-removal values.
- Verify Upsert both when a target exists and when it inserts; cover writes changing original filter fields.
- Verify ordered Insert → Find One, Update → Find Many, Upsert → Find One, and Delete → Find One behavior.
- Additional primary bindings see the main updated result; additional writes do not silently change the main snapshot or selection diagnostics.
- Verify zero-match additional mutations, typed values, operation-specific validation, duplicate output keys, and array-primary restrictions.
- Cover partial write failures and rendering failures: no fallback/retry, retained traces, session header continuity, and isolation from other sessions/base data.
- Verify API, persistence, import/export, read-only previews, and editor operation-switch/save/reload flows in both themes.

Run focused tests in models, collection, collectionresponse, API, store, and proxy, followed by the full Go suite and the UI production build during implementation. Implementation coverage includes model validation, collection identity/event replay, API round trips and previews, service operation ordering, and proxy request/session/trace behavior.
