# Field mapping suggestions

Field inputs across validation rules, pipeline and collection mappings, response conditions, and collection responses provide an editable suggestion list. Type to filter, select a suggestion, or enter a custom value. Suggestions include the field's type and source when selected. Existing date tokens remain available in condition values.

Suggestions come from the OpenAPI request contract, bounded observations from recent traces, enabled configured producers, the selected collection's documents, session/global-store keys, and the selected response schema/example. Array paths use a representative `0` segment for source traversal. Collection response target paths follow the response shape, including nested fields. Type, required, enum, default, and example information appears where declared.

Pipeline suggestions respect scope and execution order. Disabled, later, and self-referencing producers are hidden. Conditional producer outputs are labelled as possibly absent. Response conditions can use upstream pipeline outputs; response rendering suggestions can also include the main and additional mapper outputs for the selected response configuration. A spec-level editor starts with fields common to its operations and lets the user select an operation context.

The suggestion panel can be opened to choose an operation context or pin a trace/session. Refresh reloads available metadata. Suggestions do not execute mappings, scripts, or collection writes; they read saved metadata, traces, and existing data. Unknown and custom keys remain editable, and server validation remains authoritative.

The API is read-only: `GET /_api/operations/{id}/mapping-hints` returns operation-scoped suggestions and `GET /_api/specs/{id}/mapping-hints` returns common spec suggestions. Optional query parameters include `operationId`, `collectionName`, `traceId`, `sessionId`, `statusCode`, `templateRef`, and `responseId`.

Collection sampling currently calls the storage interface's `GetAll` and inspects at most 50 documents; the response is bounded, but the storage read itself is not. Trace history is also bounded to 50 matching traces. Field metadata is capped at 500 entries and shape traversal at depth 8. Sensitive fields such as authorization, cookies, passwords, tokens, email, phone, and SSN never include observed value samples. Unexpressible dotted/punctuation object keys are omitted because current source resolvers cannot address them safely.
