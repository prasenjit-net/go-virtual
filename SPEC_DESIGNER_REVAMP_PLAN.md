# Spec Designer revamp plan

## Goal

Open an uploaded specification in one full-screen, editor-style workspace. The workspace keeps a local draft while the user navigates between the spec, operations, response configurations, validation rules, pipelines, and collection response mappers. A single Save command submits the workspace snapshot. Saving updates the server while keeping the designer open; Close leaves the editor and handles unsaved changes explicitly.

The central user promise is simple: moving between items never saves or loses edits, and Save represents one deliberate workspace save rather than a series of hidden panel mutations.

## Product experience

### Entry and route

- Clicking a spec in the spec list opens its designer route, for example `/specs/:specId/designer`.
- The route loads one workspace bundle before presenting an editable canvas. It shows a useful load/error state and keeps the current route stable when selecting items.
- Browser refresh/deep links restore the selected item and subview. The navigation tree can encode selection in the URL without serializing the draft into it.
- Existing spec detail and operation pages can remain available during rollout. The spec list should use the designer by default once the new flow is proven; provide a temporary route back during staged adoption if needed.

### Layout

Use the full viewport below the application shell:

1. **Top command bar**: spec name and dirty marker; Save; Close; undo/redo; workspace validation/error indicator; save progress/result; optional command/search palette. Save stays visible at all widths. Support Cmd/Ctrl+S.
2. **Left navigation rail/tree**: collapsible to a narrow icon rail. Search and filter items. Group spec settings, operations, and each operation's responses, validations, pipeline/mappers, signature, plus spec-level pipeline/mappers. Show method/path, response kind/status/condition summary, disabled state, and unsaved/error badges. Add, duplicate, reorder where currently supported, and delete actions live here or in the command palette.
3. **Middle editor canvas**: displays the selected item. Selecting a different tree item switches the canvas without a network write. Use focused editors for operation details, response conditions/body/headers, collection response selection/query/mappers/overrides, validation rules, scripts/pipeline order, collection mappings, and signature settings. Reuse current controls where they can become controlled editors.
4. **Right context panel**: collapsible and optional. Shows details useful for the active editor: OpenAPI request/response schemas and examples, operation path/query/header/body inputs, response output preview/shape, mapping suggestions, execution traces, validation output, and diagnostics. It must resize around the center editor rather than cover required controls. User can pin/hide it; remember the preference locally.

At narrow widths, show one main pane at a time with explicit navigation to the tree or context panel. Preserve the same draft and command bar behavior.

### Navigation tree and selection behavior

- Selecting an operation opens its overview by default; children navigate directly to response, validation, pipeline/mapping, or signature editors.
- Responses show kind, status, priority, enabled state, and condition summary. A collection response expands to primary query/write, additional mappers, output mapping, headers, and conditions.
- Spec-level and operation-level script bindings, validations, and collection mappings remain visibly scoped. Response-level steps appear under their response.
- Tree search filters without changing or deleting draft data. Keyboard navigation and accessible names are required.
- Support unsaved-item indicators and validation error counts at item and parent levels.
- Navigating within a spec never prompts to save. Closing the designer, navigating to another spec, or discarding a local draft prompts only when necessary.

### Command bar behavior

- **Save** validates the draft, shows errors in-place, then submits a complete snapshot. It stays in the designer on success, records the new baseline/revision, and clears the dirty state.
- **Close** returns to the previous spec list/context. If dirty, offer Save, Discard, and Cancel; Discard clears the recoverable local draft only after the user chooses it.
- **Undo/Redo** operate on workspace draft changes, not independent form controls. Initial implementation may use a bounded edit history; every editor must emit structured changes for this to work.
- **Save status** distinguishes saved, unsaved, saving, save failed, and conflict. Never clear dirty state on failed save.
- Optional **command/search palette** can locate operations, responses, mappers, and commands by name/path. It is additive and must not replace the left tree.

## Workspace model and save contract

### Client draft

Create a normalized `SpecWorkspace` draft composed of:

- spec metadata and runtime settings, including base path, mode policy, tags/fallback/tracing settings, backend URI, and signature headers;
- original OpenAPI source document and derived operations, retaining stable operation IDs and an explicit distinction between imported contract fields and editable simulator configuration;
- operation-specific editable settings such as signature configuration;
- all spec-, operation-, and response-scoped response configs, validation rules, script bindings, collection mappings, and collection response definitions;
- referenced shared resources as read-only catalog details initially, unless the user explicitly enters their owning editor.

Keep child collections keyed by stable server IDs. Newly created draft items receive client temporary IDs; the server maps these to durable IDs in its save response. Deleting an item from the tree marks it deleted in the draft and does not call DELETE immediately. Preserve unsupported/unknown OpenAPI fields and extension properties byte-for-byte or through a lossless parse/serialize representation.

Use explicit operations/reducers to modify the draft; do not keep independent panel states that can diverge from the workspace snapshot. Normalize ordering and nested ownership when constructing the request. Calculate dirty state against the last server-loaded/successfully-saved baseline, not against whether an editor was opened.

### Local recovery

- Persist the working draft in IndexedDB (or an equivalent browser store) under spec ID and base server revision. Write changes with a short debounce and retain a schema version for future migrations.
- Reopening the same spec offers Restore draft, Compare/Discard draft, or Load latest server version when a local draft exists. Never silently overwrite a newer server copy with stale local state.
- Store only editor configuration already available to the user. Do not persist trace payloads, secrets, tokens, or generated runtime data in the workspace draft. Treat the browser as untrusted: local data is convenience, not authority.
- Provide an explicit Discard Draft action and clear the local snapshot after successful Save or deliberate discard. Closing a tab may leave recovery state.

### Server bundle API

Add a workspace-oriented API, for example:

- `GET /_api/specs/:id/workspace`: return the spec, operations, and every scoped editable configuration needed by the designer, plus a monotonic revision/ETag.
- `PUT /_api/specs/:id/workspace`: accept a full workspace snapshot, expected revision, client temporary-ID map/correlation IDs, and an idempotency key. Validate ownership and references for every nested entity, then persist it as one logical unit.

Do not implement Save by calling today's per-entity create/update/delete endpoints in a loop; a failure halfway through would leave the server different from the draft and from the prior state. Introduce a domain-level workspace apply operation and storage capability that provide all-or-none behavior. During architecture work, verify how to provide this guarantee for every supported backend (memory, file, Mongo). Use storage transactions where supported and atomic snapshot/rename or a journal-and-rollback strategy for file storage. If a backend cannot give the promised guarantee, either implement its safe strategy or clearly gate workspace Save for it; do not report a partial multi-entity write as a successful save.

The handler must:

- verify the spec exists and the submitted revision still matches;
- ensure every operation, response, validation, binding, and mapping belongs to this spec and the correct parent scope;
- distinguish updates, creates, and deletions by stable IDs and explicit draft deletion/complete-snapshot semantics;
- reject duplicate IDs, cross-spec references, invalid output keys/orders, invalid condition trees, invalid collection response/query/mapping combinations, and malformed OpenAPI content before applying anything;
- preserve shared script/library entities unless the payload explicitly includes an authorized edit to their owning resource;
- persist a consistent updated revision, update timestamps, reload runtime routes once after commit, and return the canonical saved workspace with resolved IDs and revision;
- return field-addressable validation errors that the tree and editors can navigate to.

Use optimistic concurrency with a workspace revision or ETag. On conflict, keep the local draft intact and offer reload, compare, or an explicit merge workflow; never automatically overwrite the server's newer workspace. Repeated requests with the same idempotency key must not duplicate newly created entities.

### OpenAPI contract editing boundary

The uploaded OpenAPI document is the source of truth for imported paths, parameters, schemas, examples, and generated operation identities. Initially, show this contract in a read-only schema/example panel and allow edits only to simulator-owned configuration and safe spec metadata. This avoids an ambiguous split between raw document content and normalized operation rows, since the current update endpoint does not regenerate operations from edited OpenAPI content.

If editing the OpenAPI document itself is a requirement, treat it as a separate explicit capability: parse and validate the whole document; reconcile operations by stable `operationId` or method/path; show additions, removals, and changed contracts before applying; define what happens to response configurations, traces, signatures, and mappings for removed operations; preserve extensions and examples; and apply document plus dependent configuration atomically. Do not silently re-import or delete config when the user edits metadata.

## Editor architecture

- Add a designer shell with a workspace query, draft store/reducer, local recovery service, navigation tree, command bar, middle editor registry, and context panel registry.
- Keep the shell responsible for loading/saving and draft lifecycle. Feature editors receive selected draft values and `onChange` callbacks; they must not directly mutate server state.
- Refactor current editors incrementally into controlled/presentational form sections. Existing standalone pages may keep a thin adapter that invokes their current endpoint until migrated, but designer mode must use the workspace draft exclusively.
- Introduce a capability/context object for editors (`specId`, operation/response IDs, selected contract/example, trace context, scope). Existing IntelliSense and mapping panels can consume draft-aware local data while retaining current API suggestions where useful.
- Context-panel preview should render from the local draft. Collection response preview must not persist or execute writes; use the existing preview semantics only after checking it is side-effect free, otherwise add a draft preview endpoint that runs against an isolated/read-only session and clearly marks external dependencies.
- Keep template/script code editors as specialized subeditors. Shared script definitions and reusable templates are linked resources; if editing them is enabled in the workspace, include their contents and ownership rules in the same save contract or keep their save action explicitly separate and outside the “one workspace save” promise.
- Preserve dark mode, existing responsive behavior, focus management, keyboard navigation, and accessible labels. Avoid rendering all heavy editors at once; mount the selected editor and lazily load the context panel.

## Migration of current flows

1. Inventory each current editor's reads, mutations, ownership scope, validation rules, and side effects. Cover `SpecDetail`, `OperationDetail`, `ResponseConfigIDE` / `ResponseConfigEditor`, `CollectionResponseEditor`, `PipelinePanel`, `CollectionMappingsPanel`, signature configuration, spec/operation/response scripts, and validation rules.
2. Define the workspace DTO and revision protocol from that inventory. Keep current endpoints for other pages and compatibility; add workspace endpoints rather than changing every legacy client at once.
3. Build the shell and draft lifecycle with spec settings and operation navigation. No persistence mutations except the workspace Save endpoint.
4. Convert operation settings, response editors, collection response, signature, validations, scripts/pipeline, and collection mappings into controlled subeditors. After each conversion, verify its edits are reflected in one draft and survive navigation within the designer.
5. Add context panels and local recovery. Verify draft restoration across reload, Save/Discard/Cancel, conflict handling, and browser storage migration.
6. Add workspace service/storage atomic application and tests for each backend; test failures at each child write boundary and confirm the previously saved workspace remains intact.
7. Roll out behind a feature flag or route switch, compare the new save output with existing endpoints, then make spec-list clicks open the designer by default. Remove duplicated old screens only after parity and migration are established.

## Validation and acceptance criteria

- Clicking a spec opens a full-viewport workspace with a collapsible left navigation tree, selected-item editor in the middle, optional contextual right panel, and persistent command bar.
- The user can navigate from spec to any operation, response config, validation, pipeline/mapping, and collection response mapper without saving or losing changes.
- Every supported editable field changes one local workspace draft. The network makes no persistence request while navigating or editing; only Save submits mutations.
- Save sends the complete logical workspace once, validates all entities, handles temporary IDs, and either commits all changes or leaves the prior server state unchanged. The designer stays open with a clean draft after success.
- Save failure, validation failure, offline operation, and server revision conflict preserve the user's draft and provide actionable messages.
- Close has correct Save/Discard/Cancel behavior. Refresh can restore a local draft and detects stale server revisions.
- Existing IDs and references remain stable across edits; deletes and reorders are represented correctly; no unrelated shared scripts/templates are changed.
- Changes to the OpenAPI contract follow the explicit read-only boundary or the separately designed reconcile flow; they never silently destroy simulator configuration.
- Runtime routing reflects committed workspace changes after one reload, and failed workspace writes do not partially reload configuration.
- Tests cover serialization round trips, nested ownership validation, optimistic concurrency, idempotency, atomic rollback for every storage backend, draft reducers/dirty state, local recovery, save lifecycle, keyboard commands, navigation, responsive layout, and regression parity with existing editors.

## Out of scope for the first release

- Collaborative simultaneous editing and live remote cursors.
- Automatic merging of conflicting edits from two browsers; show a conflict and preserve both server and local snapshots first.
- Editing shared/global script or template libraries under the same save unless their ownership and transaction semantics are explicitly included.
- Silent OpenAPI re-import, destructive operation reconciliation, or automatic rewrites of user references.
- Replacing every administration page in one release; the spec designer can ship incrementally while legacy routes remain available.
