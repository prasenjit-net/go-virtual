# Visual Template Editor Implementation Plan

## Goal

Add a response-body-only visual template editor for JSON response templates. The editor should show the current response body as a tree, let users configure where each leaf value comes from, and compile the edited tree back into the existing Go `text/template` response body string.

The existing text editor remains the source of truth for saved responses. The visual editor is an alternate editing mode for the same `ResponseConfig.body` field.

## Current Architecture Notes

- Response bodies are stored as strings on `ResponseConfig.body`.
- The backend already validates response body templates with `POST /_api/templates/validate`.
- Response body editing currently exists in two UI surfaces:
  - `ui/src/components/ResponseDesigner/ResponseConfigEditor.tsx`
  - `ui/src/components/ResponseDesigner/ResponseConfigIDE.tsx`
- Both surfaces use Monaco directly, so the implementation should extract shared body editing behavior into reusable components instead of duplicating the visual editor twice.
- Operation metadata already exposes declared request inputs:
  - path params
  - query params
  - header params
  - body fields
  - request body presence
- Existing APIs expose script bindings, collection mappings, and validation rules. These can be used to populate visual source choices.

## Proposed User Experience

### Body Mode Switch

Add a compact segmented control in the response body area:

- `Text`
- `Visual`

This switch applies only to the response body editor. Metadata, conditions, headers, and pipeline screens stay unchanged.

In the IDE page editor, the existing `Body` tab should contain the `Text | Visual` mode switch. In the modal editor, place the switch above the body editor next to `Examples` and `Prettify`.

### Visual Tree

When the body is JSON-like, display it as an expandable tree:

- object nodes display property names and child counts
- array nodes display indexes and item counts
- leaf nodes display key/index, inferred JSON type, current literal/template value, and source status

Each leaf node can be configured from a side panel or inline popover:

- literal value
- request path parameter
- request query parameter
- request header
- request body field
- raw request body
- method, URL, request ID
- session store key
- counter key
- random value
- faker value
- timestamp value
- script binding output
- validation rule output
- collection mapping output
- custom template expression

The visual editor compiles leaf choices back to valid template snippets such as:

- `{{.Path.id}}`
- `{{.Query.status}}`
- `{{body "items.0.id"}}`
- `{{.Script.pricing.total}}`
- `{{.Collection.user._id}}`
- `{{faker "email"}}`
- `{{timestamp "iso"}}`

### Source Picker

Use the current operation and response context to populate choices:

- `operationsApi.get(operationId)` for declared path/query/header/body fields
- `scriptBindingsApi.listByOperation(operationId)` and `responseScriptBindingsApi.listByResponse(...)`
- `collectionMappingsApi.listByOperation(...)` and `collectionMappingsApi.listByResponse(...)`
- `validationRulesApi.listByOperation(...)`
- static helpers for random/faker/timestamp/template functions

For response-scoped scripts and collection mappings, the response must exist before those source choices can be fully listed. For new unsaved responses, show operation-scoped sources plus a disabled message for response-scoped pipeline outputs.

## Implementation Strategy

### 1. Introduce Shared Body Editor Components

Create a shared body editor layer under:

`ui/src/components/ResponseDesigner/BodyEditor/`

Suggested components/files:

- `ResponseBodyEditor.tsx`
  - owns `Text | Visual` mode
  - receives `body`, `onBodyChange`, `operationId`, optional `responseConfigId`, `readOnly`, validation state, and editor callbacks
- `TemplateTextEditor.tsx`
  - wraps current Monaco behavior
  - preserves syntax highlighting, validation markers, and completions
- `VisualTemplateEditor.tsx`
  - renders JSON tree and source configuration UI
- `templateTree.ts`
  - parses body to a tree model
  - compiles tree model back to template string
  - handles stable path IDs and value typing
- `templateSources.ts`
  - builds available source options from operation metadata, scripts, collections, validations, and static helpers
- `templateSnippets.ts`
  - centralizes template snippet generation

Then replace the body editor blocks in both current response editor surfaces with `ResponseBodyEditor`.

### 2. Define an Internal Tree Model

Use a client-only normalized structure:

```ts
type VisualTemplateNode =
  | { kind: 'object'; path: string; key?: string; children: VisualTemplateNode[] }
  | { kind: 'array'; path: string; key?: string; children: VisualTemplateNode[]; itemMode?: 'fixed' | 'templateRange' }
  | { kind: 'leaf'; path: string; key?: string; valueType: 'string' | 'number' | 'boolean' | 'null'; binding: LeafBinding }
```

`LeafBinding` should support:

- literal JSON values
- generated template expression
- custom raw expression
- unsupported/mixed template value

Keep this model local to the UI. Do not persist it separately unless future requirements need visual metadata that cannot be recovered from the template string.

### 3. Parsing Policy

The visual editor should support a practical first version:

- Parse valid JSON bodies directly.
- Parse JSON bodies whose leaf values are simple template strings, for example `"{{.Path.id}}"`.
- If the body is empty, initialize to `{}` with actions to switch root to object or array.
- If the body is not valid JSON, show a recoverable state:
  - keep text editor available
  - show why visual mode cannot parse it
  - offer "Start from JSON object", "Start from JSON array", and "Load example" actions

Do not attempt to fully parse arbitrary Go template control flow in the first version. Complex bodies remain editable in text mode.

### 4. Root Node Behavior

Support both JSON object and JSON array roots:

- object root: users can add/remove/rename properties
- array root: users can add/remove/reorder fixed items
- scalar root: treat as a single leaf editor, but visually mark it as limited

For arrays, separate two cases:

- fixed arrays, compiled as normal JSON arrays
- future dynamic arrays using `range`, which should be a second phase because it needs block-level template editing

### 5. Leaf Binding Behavior

Each leaf editor should preserve JSON type correctness:

- string leaves quote template output as JSON strings
- number leaves warn if source may not be numeric
- boolean leaves warn if source may not be boolean
- null leaves can stay null or be changed to another type

For custom template expressions, allow advanced users to enter the raw expression inside `{{ ... }}` or a full custom value. Validate with the existing backend template validator after compilation.

### 6. Compilation Rules

Compile the tree model to a pretty JSON response body string:

- object and array structure is generated from the tree
- literal leaf values are serialized with `JSON.stringify`
- template leaf values are inserted according to type:
  - string leaf: `"{{.Path.id}}"`
  - number/boolean leaf: `{{toJSON .Script.result.count}}` or custom expression if type-safe
  - object/array leaf from source: use `{{toJSON .Collection.outputKey}}` when replacing an entire subtree

After every visual edit:

- compile to body string
- update parent `body`
- mark the response dirty
- run the existing debounced template validation

### 7. Text to Visual Round-Trip

When switching from text to visual:

- attempt parse
- if successful, build the visual model
- if unsuccessful, keep the current body unchanged and show parse guidance

When switching from visual to text:

- show the compiled body string in Monaco
- keep Monaco validation active

If users edit text after using visual mode, visual mode should re-parse on the next switch rather than maintaining stale tree state.

### 8. Source Option Coverage

Minimum source groups:

- Request:
  - `.Path.<name>`
  - `.Query.<name>`
  - `.Header.<name>`
  - `body "<gjson path>"`
  - `.RawBody`
  - `.Method`
  - `.URL`
  - `.RequestID`
- Runtime helpers:
  - `random`
  - `faker`
  - `timestamp`
  - `counter`
  - `store`
- Pipeline outputs:
  - `.Script.<outputKey>.<field>`
  - `.Validation.<ruleName>.<property>`
  - `.Collection.<outputKey>.<field>`
- Advanced:
  - custom Go template expression

For script, validation, and collection outputs where field shapes are unknown, provide output-key selection plus a manual nested path input.

### 9. Validation and Warnings

Keep backend template validation as the final authority.

Add client-side warnings for:

- invalid JSON shape in visual mode
- duplicate object keys while renaming
- invalid field names when adding properties
- deleting a subtree
- source selected but required key/path missing
- response-scoped outputs unavailable for unsaved responses
- type mismatch risk
- complex Go template blocks not representable visually

### 10. Accessibility and Layout

Use the existing Tailwind/lucide UI style:

- tree expand/collapse buttons with icons
- keyboard-accessible controls
- no nested cards inside cards
- stable row heights for tree rows
- read-only mode disables all visual edits but still permits inspection
- compact layout suitable for the IDE body tab and modal editor

### 11. Testing Plan

Add focused UI unit tests if a React test setup exists; otherwise add pure TypeScript tests for `templateTree.ts` and `templateSnippets.ts` if the project accepts a test runner later.

Manual verification:

- create a response from empty body and save
- load spec example and switch to visual
- object root edit
- array root edit
- scalar root display
- bind leaves to path/query/header/body
- bind leaves to script/collection/validation output
- switch visual to text and back
- invalid JSON text body shows visual parse fallback
- complex `if/range` text template is preserved and not overwritten
- read-only recorded response displays tree without editing
- template validation error still blocks save

Run before completion:

- `npm run build` from `ui/`
- `go test ./...`

## Edge Cases

- Empty body.
- Whitespace-only body.
- Valid non-JSON body such as plain text.
- Invalid JSON.
- JSON object root.
- JSON array root.
- JSON scalar root.
- Deeply nested objects.
- Large arrays.
- Duplicate object keys in raw JSON.
- Property names containing dots, spaces, quotes, or slashes.
- Array indexes and gjson paths.
- Null values.
- Numeric values using string sources.
- Boolean values using string sources.
- Template functions that output JSON fragments.
- Full-subtree replacement from `toJSON`.
- Existing Go template control blocks.
- Existing template comments.
- Missing keys with `missingkey=zero`.
- Lowercased header/query lookup behavior.
- Response-scoped pipeline outputs unavailable before first save.
- Disabled scripts/mappings/validation rules.
- Renamed or deleted pipeline output keys leaving stale template references.
- Read-only recorded responses.
- Unsaved dirty body when switching modes.
- Validation request race conditions while visual edits are compiling.

## Suggested Phasing

### Phase 1: Safe Visual Editing for JSON Leaves

- Build shared body editor wrapper.
- Add mode switch.
- Parse valid JSON with simple leaf template strings.
- Render object/array/scalar tree.
- Configure leaf bindings.
- Compile back to body string.
- Reuse existing validation.

### Phase 2: Source Richness and Subtree Replacement

- Add response-scoped pipeline source options.
- Add full object/array source binding via `toJSON`.
- Improve source picker filtering by expected value type.
- Add warnings for stale output keys.

### Phase 3: Advanced Template Blocks

- Add visual support for repeat/range array items.
- Add conditional optional fields.
- Add preview rendering if a sample request context is available.

## Open Questions

1. Should visual mode support only JSON response bodies, or should it also have a limited tree view for non-JSON formats?
2. When you said "in case of the root node is an", did you mean the root node can be an array? This plan assumes object, array, and scalar roots should all be handled.
3. For arrays, do you need dynamic `range` support in the first release, or is fixed-array editing acceptable initially?
4. Should the visual editor persist any UI-only metadata, or is recompiling from the body string on each switch acceptable?
5. Do you want a live rendered preview using a sample request context, or is template validation enough for the first version?
