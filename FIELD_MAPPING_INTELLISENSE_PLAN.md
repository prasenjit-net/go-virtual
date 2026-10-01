# Field mapping IntelliSense plan

Status: implemented. See [docs/field-mapping-intellisense.md](docs/field-mapping-intellisense.md) for user-facing behavior and current sampling limits.

## 1. Goal and user experience

Offer inline field-name, path, and value suggestions while configuring validation rules, collection mappings, and response conditions. Include collection response filters, write fields, additional mappers, and output overrides so mapping assistance is consistent throughout the product.

Use the same interaction as the existing relative-date value input: a normal editable text input with a native `datalist`. Focusing/typing displays matching choices; selecting a choice fills the input. Users can always type a custom value. Suggestions are assistance, not a restriction or automatic configuration change.

Each option displays the insertable key and a short description, for example:

- `employeeId` — string · required · request path
- `addresses.0.postalCode` — string · request schema
- `employeeCheck.status` — string · earlier validation
- `addresses` — array of Address · additional mapper output
- `addresses.0.city` — string · observed in a selected trace

Provide a small helper below the field for the selected suggestion's type, origin, and availability. Native datalist rendering differs by browser, so essential information must also be visible in this helper. Match the existing date input's size, placement, light/dark theme, and free-text behavior. Do not introduce a code editor or modal for field selection.

Keep existing relative-date tokens and operators working. Use React `useId()` rather than depth/index alone for suggestion-list IDs; multiple condition rows currently can share `date-tokens-${depth}`.

## 2. Existing foundations and gaps

Relevant implementation points:

- `ui/src/components/shared/ConditionEditor.tsx`: shared condition rows and relative-date datalist; used by validations and response conditions.
- `ui/src/components/CollectionMapper/CollectionMappingsPanel.tsx`: already suggests declared operation path/query/header/body keys using `OperationHints` and datalists.
- `ui/src/components/Pipeline/PipelinePanel.tsx`: another mapping editor that needs the same provider and rendering component.
- `ui/src/components/ValidationManager/ValidationRulesPanel.tsx`: condition trees and literal success/failure properties.
- `ui/src/components/ResponseDesigner/CollectionResponseEditor.tsx`: typed filters/data, main and additional mapper context, output overrides, and missing-source policies.
- `internal/parser/openapi.go`: `ExtractOperationInputs`, `ParamDef`, and `BodyFieldDef` expose some schema information, but not the complete type/enum/format/example metadata needed here.
- `internal/models/operation.go` and `internal/api/handler_operations.go`: declared input name lists already reach the client.
- `internal/condition/evaluator.go`, `internal/collection/resolver.go`, and `internal/collection/typed_resolver.go`: authoritative source capabilities and key syntax.
- Trace models contain requests, script outputs, validation results, and collection response mapper results. Legacy collection traces do not currently populate their result field, so runtime inference cannot assume all collection outputs are already available.

First audit actual pipeline execution/context propagation before implementing availability rules. A step appearing earlier in the editor is insufficient unless the evaluator receives its result. In particular, do not assume that standalone validation execution exposes preceding validation outputs during the same run.

## 3. Coverage by editor and field

| Editor/site | Source-key suggestions | Target/value suggestions |
| --- | --- | --- |
| Validation condition | Request inputs and only evaluator-supported outputs available before that rule | Schema enums, booleans, observed non-sensitive scalar values, existing relative-date tokens |
| Response condition | Request inputs and spec/operation pipeline outputs available before response selection | Same type-aware values; signature source remains key-disabled |
| Collection mapping filter | Path/query/header/body/session/store according to existing resolver capabilities | Known collection field names for filter targets |
| Collection mapping data | Same supported sources | Existing collection fields plus manually entered new fields; literal/default values preserve string semantics |
| Pipeline collection step | Same as standalone collection mapping, scoped to this step | Same target/value assistance |
| Collection response primary filters/data | Supported request sources; primary-document bindings only where execution makes them available | Chosen collection fields; typed literal/default JSON suggestions |
| Additional mapper filters/data | Request inputs and primary output where supported | Additional mapper's collection fields |
| Collection response override | Document fields, named mapper outputs, supported request sources | Target paths from selected response schema/example; whole-object/array output keys and nested paths |

Do not add new runtime source types as part of IntelliSense. For example, legacy collection mappings currently do not accept `script` or `validation` sources. Additional collection response mappers cannot bind to earlier additional outputs merely because their operations execute in order. Their results are available to rendering overrides.

Validation onSuccess/onFailure entries are literal properties, not source bindings. Offer existing property-name/value reuse where helpful, but do not imply that these strings evaluate expressions.

## 4. Sources of knowledge

### A. Declared request contract

Extend parser metadata to cover:

- Path-level and operation-level path/query/header parameters, with operation-level overrides resolved by `(in, name)`.
- Required/optional, JSON type, format, nullable, description, enum, default, and explicit examples.
- Request body schemas and examples for the selected media type; recurse into properties and array items.
- Resolved references and allOf composition. Preserve oneOf/anyOf alternatives as conditional suggestions rather than claiming every branch is always present.
- Cycle and depth limits; unsupported schemas should reduce suggestions, not prevent editing.

Use the operation's actual parameter names: `/employees/{employeeId}` suggests `employeeId`, not a guessed `id`. Treat header lookup case-insensitively while preserving a canonical display spelling.

No body-key inference from non-JSON bodies unless the selected resolver supports that format. Do not fetch remote schema references beyond the application's existing spec-loading policy.

### B. Saved configuration and current editor draft

Infer output namespaces and known shapes from configured producers:

- Script binding output keys; use explicit/observed output metadata where available, not speculative execution or arbitrary source-code inference.
- Validation rule names, `.status` (`pass`/`fail`), and declared success/failure property keys. Branch-specific properties remain optional/conditional.
- Collection mapping output keys and operation shape (single document, array, status wrapper). Preserve the existing `_status` representation where applicable; do not apply it to collection response outputs, which are plain results.
- Collection response primary and additional mapper output shapes, collection fields, and the selected response schema/example.
- Session/global-store key names, only through existing authorized access and a selected session for session-specific suggestions.

Merge unsaved editor drafts with the saved catalog locally. Renaming a producer, changing a source/collection/operation, or reordering a pipeline must immediately update suggestions without saving first. Never rewrite existing references automatically.

### C. Existing collection documents

Offer collection target fields from a bounded sample of the selected collection. Union fields across sampled documents instead of using only the first document. Mark them as observed, not a required collection schema. Include nested read/filter paths only when supported. For write targets, respect current top-level-only restrictions and immutable `_id` rules.

Use server-side shape extraction and return field metadata rather than downloading entire collections. A session view is optional and must be explicitly scoped to a selected session; otherwise use the base collection.

### D. Previous executions

Use recent captured traces for the same operation, optionally pinned to a specific trace/session:

- Request path/query/header/body shapes and permitted value samples.
- Script, validation, and collection output shapes at their actual execution points.
- Primary and additional collection response mapper outputs, including null/empty results.
- Successful upstream work before an abort may supply observations for earlier contexts; failed/skipped producers must not be treated as successful outputs.

Do not infer current availability from the final HTTP body. Do not mix unrelated operations or sibling response configurations. A previous result means “observed previously,” not “guaranteed on the next request.”

Suggestions must never execute a script, validation, mapper, insert, update, upsert, or delete to discover fields. No request replay is triggered by focusing or typing in an input.

## 5. Execution-aware availability

Represent each editing location with a context descriptor:

- spec ID, operation ID, response config ID when applicable;
- site (condition, filter, data, override, literal/default);
- source type, pipeline scope, current step ID/order, and current mapper ID/output key;
- selected collection, response example/status, and optional trace/session ID;
- draft configuration revision.

Build visibility from the actual runtime sequence and evaluator capabilities:

1. Request inputs are available from the start.
2. Spec steps can use only upstream data the runtime exposes in their context.
3. Operation steps may see completed spec steps and eligible earlier operation steps.
4. Response selection conditions cannot use response-scope execution or collection response mapper outputs, which happen after selection.
5. Main Update data may use its selected primary document. Main Insert/Upsert data cannot assume a pre-existing primary document.
6. Additional collection response mappers can use the completed main result where supported; array-root primary restrictions remain enforced.
7. Rendering overrides can use completed additional mapper outputs, including a bare output key for an object/array.

Only eligible producers appear as selectable suggestions. Conditional producers are marked “may be absent”; disabled, later, self-referencing, and out-of-scope producers are excluded. Typed custom references remain editable, with non-blocking availability feedback. Existing server-side validation remains authoritative.

Spec-scoped editors need special handling: use the common contract across applicable operations by default, with an operation context selector to inspect operation-specific suggestions. Clearly identify operation-specific keys; never label a union of keys as universally present.

## 6. Paths, types, and values

Keep canonical structural paths separate from insertable syntax:

- Display array shape as `addresses[].city`; insert a resolver-valid representative key such as `addresses.0.city` with “first item” clearly indicated.
- Whole mapper results insert `addresses`; nested values insert `addresses.0.city`.
- Offer `addresses.city` as a response target that applies to every item, and numeric indexed targets where the renderer supports them. Do not confuse target traversal rules with GJSON source traversal rules.
- Generate escaping for property names containing punctuation according to the specific resolver. If a path cannot be expressed safely, explain it rather than inserting an incorrect key.
- Preserve literal dotted session/store key names where those resolvers use exact lookup.
- Null, empty string, false, zero, empty arrays, and missing values remain distinct.

For condition values, prioritize declared enums, booleans, and date tokens. Offer bounded observed scalar choices as optional examples. Keep invalid/unknown custom values editable. Type-based operator advice is informative, not a silent operator change.

For typed literal/default fields, insert valid serialized JSON. For legacy string defaults, insert text using existing semantics. Do not turn on Default or Skip or supply a value merely because a suggestion exists.

## 7. Shared API and frontend design

Propose a read-only metadata endpoint such as `GET /_api/operations/:id/mapping-hints`, with validated context parameters (site, source, responseConfigId, stepId, collectionName, optional traceId/sessionId). Add a spec-level companion for reusable spec rules. Server-side authorization and ownership checks must verify every referenced ID.

Return a versioned catalog, for example:

```json
{
  "revision": "catalog-revision",
  "items": [{
    "source": "body",
    "path": "addresses[].postalCode",
    "insertText": "addresses.0.postalCode",
    "types": ["string"],
    "required": false,
    "description": "US ZIP or ZIP+4",
    "origins": ["schema", "observed"],
    "availability": "request",
    "producerId": null,
    "observedCount": 4
  }],
  "truncated": false,
  "warnings": []
}
```

Keep value suggestions optional and separately filtered. Include producer ID/scope, conditionality, enum/format metadata, and last-observed timestamp when relevant. The schema remains authoritative if observed types conflict; retain the conflict visibly instead of erasing either source.

Frontend pieces:

- `SuggestedFieldInput`: shared input/datalist/helper using unique IDs, free text, and existing date input styling.
- `useMappingHints(context)`: React Query-backed metadata retrieval, memoized local filtering, deduplication, and ranking.
- A shared context provider/props passed through nested condition trees, mapping rows, modals, and pipeline panels.
- A local draft overlay so unsaved configuration changes update the catalog immediately.

Deduplicate by source and exact effective key (case-insensitive for headers). Rank prefix matches before substring matches, declared/context-valid fields before observed-only fields, and recent observations before old ones. Keep the displayed list deterministic and capped. Avoid a request per keystroke: fetch a bounded context catalog, then filter locally.

Cache by spec/config revision, operation/site/producer context, collection revision, and optional trace/session selection. Invalidate on schema upload, example selection, producer rename/reorder/delete, collection changes, and user-requested runtime refresh. Hide stale-context suggestions while a changed source/context loads; never clear user input on a delayed response.

## 8. Runtime metadata, limits, and data handling

Add an opt-in shape collector at existing completed execution boundaries where trace metadata is insufficient. Record producer identity, scope/order, operation/config identity, field paths and types, and completion status. Do not rely on display names alone, which can collide.

Prefer deriving metadata from already captured authorized traces initially. Do not enable tracing automatically or store raw payloads in a second system. If tracing is disabled or records have expired, static/configuration suggestions still work.

Initial configurable limits: latest 50 traces per operation, 50 sampled collection documents, depth 8, 500 paths per context, 50 visible suggestions, and at most 5 scalar value examples per field. Mark truncation. Treat an empty array as an array with unknown observed item fields and fall back to schema metadata.

Suppress observed values for credentials and sensitive fields (for example Authorization, Cookie, Set-Cookie, token, password, secret); key/type suggestions may remain. Do not copy trace contents into browser local storage. Keep session-scoped metadata separated by session and expire it with its underlying data. Clearing traces invalidates derived runtime hints. No outbound calls or writes are needed to serve suggestions.

## 9. Delivery phases

1. **Contract and capability audit:** document actual source support, execution order, key syntax, and runtime output wrappers for every editor site; define the catalog and availability model.
2. **Shared UI and static inputs:** extract the date-style datalist component, fix duplicate IDs, enrich parser metadata, and wire request suggestions into validation conditions, response conditions, both collection mapping editors, and collection response rows.
3. **Configured outputs and targets:** add producer namespaces, validation properties, collection target fields, response target shapes, whole mapper references, and draft-aware filtering.
4. **Observed execution context:** expose bounded trace/collection metadata, populate missing legacy collection output-shape metadata at execution, support trace/session selection, and enforce execution-point visibility.
5. **Values and polish:** type-aware value suggestions, source provenance/helper text, cache invalidation, empty/error states, accessibility, and documentation.

Each phase must preserve saved configuration formats and runtime mapping semantics. The final feature is complete only when static and observed suggestions work across all requested editors; a request-key-only implementation is an intermediate milestone.

## 10. Verification and acceptance criteria

Backend regression coverage:

- Parameter inheritance/override, nested schemas, arrays, references, cycles, alternative schemas, enums, required/nullability, and header normalization.
- Correct context availability before/after steps, disabled steps, failures/aborts, output-name collisions, response selection vs post-selection execution, and session isolation.
- Observed-only fields, conflicting types, empty arrays, missing/null distinctions, expired/deleted traces, sampling limits, and sensitive-value suppression.
- Exact generated paths resolve through the real evaluator/resolver; no inferred syntax is accepted without a round-trip test.
- Hint requests have no write side effects or script/mapper execution.

UI coverage:

- Focus/type/select and free-text input, keyboard and screen-reader behavior, unique datalist IDs, nested condition groups, and light/dark modes.
- Source switch, operator switch, pipeline reorder, output rename, collection change, response example change, and unsaved drafts refresh the correct choices.
- Date tokens remain functional; existing Default/Skip policies and literal types are unchanged.
- Loading/errors/empty catalogs do not block editing, reset saved values, or expose another context's stale choices.

End-to-end examples using `test/employee-api.yaml`:

1. Employee conditions suggest path `employeeId` and declared query/header/body keys applicable to that operation.
2. PATCH body suggestions include `name`, `department`, `managerId`, and `addresses.0.postalCode`; omission remains allowed.
3. A selected trace supplies an observed custom request field with an observed-only label.
4. Response conditions suggest eligible earlier validation/script/collection outputs, but never post-selection `addresses` mapper results.
5. Output overrides suggest `addresses` and `addresses.0.city`; targets come from the selected response shape.
6. Collection filter/data targets suggest sampled employee/address fields while allowing new manually typed fields.
7. Existing mappings still load/save without using suggestions; absence of traces does not disable IntelliSense.

Validation before delivery: focused backend tests, UI type checking/build, appropriate interaction tests, browser checks where available, and a full regression run. Document limitations of native datalist display across supported browsers and the distinction between schema-guaranteed and previously observed fields.

## 11. Out of scope

No automatic request execution, automatic reference rewriting, new expression language, automatic mapping creation, new mapper source capabilities, arbitrary code execution for inference, or required migration of existing configurations. Advanced IDE-style completion can be considered separately; the requested UI stays consistent with relative-date suggestions.
