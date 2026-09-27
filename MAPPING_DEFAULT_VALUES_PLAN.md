# Optional default values for mappings

Status: implemented. Standalone collection mappings are absent from the existing archive format; their defaults persist through normal mapper save/load, while collection response defaults also round-trip through archives.

## Feature

Add an optional default to each mapping binding. Resolve its configured source and key first. If the source is unavailable or the key/path does not exist, use the configured default.

Examples:

- Map request query `status` into a collection filter, defaulting to `active` when the query parameter is absent.
- Map request body `enabled` into Update data, defaulting to JSON `true` when that field is absent.
- Render `planLabel` from additional mapper output `plan.label`, defaulting to `Free` when no plan or label exists.

This applies to collection mapper filter/data rules at spec, operation, and response scopes, plus collection response primary filters, primary Update data, additional mapper filters/data, and output overrides. It does not introduce defaults for scripts, response conditions, Go templates, or automatic convention-filled fields without an explicit binding.

## Resolution contract

| Source outcome | Result |
| --- | --- |
| Source and key exist | Use the mapped value |
| Source unavailable or key/path absent, default configured | Use the default |
| Source unavailable or key/path absent, no default configured | Retain existing missing-value behavior at that binding site |
| Invalid binding, malformed literal/default, or operation failure | Report the error; a default does not recover from errors |

Defaults activate only for absence. Present `null`, `false`, `0`, `""`, `{}`, and `[]` are not replaced. For example, `body.profile` set to null exists; `body.profile.city` does not exist because traversal cannot reach that child. An out-of-range array index is absent.

A missing additional mapper result or a null result has no child fields, so `plan.label` can use its default. Defaults do not suppress mapper execution errors or allow references to undeclared mapper output keys.

Distinguish the absence of a configured default from a default value that is empty or null. The UI must not use truthiness to decide whether a default is enabled.

## Configuration

Add the optional JSON property `defaultValue` to the two existing binding models, preserving their current value semantics:

| Model | Go representation | Default value semantics |
| --- | --- | --- |
| `FieldMappingRule` in `internal/models/collection.go` | `DefaultValue *string` with `omitempty` | String, like the existing collection mapper resolver; an empty string is valid |
| `ValueBinding` in `internal/models/collection_response.go` | `DefaultValue json.RawMessage` with `omitempty` | Any valid JSON value, including explicit null; an absent byte slice means unconfigured |

In TypeScript use `defaultValue?: string` on legacy mapper rules and `defaultValue?: unknown` on typed bindings. Preserve property presence when serializing, including an explicit `null` or `""`. Reject a non-string or explicit null default on a legacy rule rather than silently treating it as an omitted default.

Existing collection mapper filter/data rule:

```json
{
  "targetField": "status",
  "sourceType": "query",
  "sourceKey": "status",
  "defaultValue": "active"
}
```

Collection response Update data rule:

```json
{
  "targetPath": "enabled",
  "value": {
    "source": "body",
    "key": "enabled",
    "defaultValue": true
  }
}
```

Collection response output override:

```json
{
  "targetPath": "planLabel",
  "value": {
    "source": "mapper",
    "key": "plan.label",
    "defaultValue": "Free"
  }
}
```

Defaults are literal values, not another source/key expression. Hide the default control for literal-source mappings, where the literal itself supplies the value. Reject a supplied default on a literal binding to avoid redundant configuration. Do not add a separate default-enabled flag to stored JSON; the property's presence expresses that state.

## Backend changes

1. Extend the models and API validation. Validate defaults during save/import, including empty strings and explicit JSON null. All existing source/key requirements and operation-specific rules remain in force.
2. In `internal/collection/resolver.go`, extract a presence-aware lookup helper. The current string-only return value loses the distinction between absence and an empty value. Resolve nonliteral mappings with value plus presence, then apply the optional string default. Keep existing output string conversion and the no-default compatibility path, including the current nil-request behavior; do not convert all collection mappers to typed JSON as part of this feature.
3. In `internal/collection/typed_resolver.go`, separate source lookup from default application. When lookup returns `found=false` without error, decode a configured default and return it with `found=true`. Keep `ResolveFilterMap` and `ResolveRequiredMap` using that central resolver so every typed binding site gets the same behavior.
4. Make header lookup presence-aware and case-insensitive. `Header.Get` currently conflates absent headers with present empty values. A present empty header must remain an empty mapped value, not trigger a default. Likewise use map membership for path/query/session/store sources and GJSON existence for body paths. A query/header entry with no values counts as absent; one containing an empty string is present.
5. Ensure default objects/arrays are independently decoded for each resolution; avoid sharing mutable values across requests.
6. Verify persistence, cloning, import/export, and preview payloads retain defaults without schema migration. No default property means legacy behavior, except the narrowly required correction to distinguish present empty headers from missing headers in typed lookup.

Invalid source types, malformed paths where validation rejects them, and unsupported primary bindings on array roots stay invalid even when a default is configured. Defaults are not a way to bypass structural validation.

## Interaction with response selection and rendering

Defaults resolve at the binding site before the filter, mutation data, or rendered override is used:

- A defaulted primary filter participates in response selection exactly like a supplied request value. An empty query result still falls through unless `matchOnEmpty` is enabled; a filter default does not invent a matching document.
- A defaulted write binding satisfies the required-value check. Without a default, missing write bindings retain their current runtime errors. Update/Insert/Upsert/Delete cardinality and execution order do not change.
- An output override first uses its source value, then its own default if that value is absent. Successful default resolution is not reported as an unresolved override.
- Without a default, an unresolved explicit override keeps current behavior: null plus a warning in template mode, or leave the original value plus a warning in identity mode.
- `fallbackToExample` continues to apply to convention-filled template leaves without an explicit override. It does not override a binding default and is not a final fallback for an unresolved explicit override.
- A whole-object/array default in an override directly replaces that target, just like a mapped override value; it is not projected again through the child template.
- Preview uses the same default resolution while remaining read-only. Defaults do not cause skipped mutation mappers to execute.

## Editor changes

Add an optional **Use default when missing** control to nonliteral mapping rows:

- Collection mapper rows: a text input for the string default.
- Collection response rows: a JSON value editor with validation for strings, numbers, booleans, null, objects, and arrays. A string must be entered as valid quoted JSON.
- Keep an explicit local enabled state so users can configure `""` or null. Turning the option off removes the property from the saved payload.
- Show concise helper text: “Used only when the source or key is missing. Existing empty or null values are kept.”
- Preserve defaults when changing keys, reordering rows, or saving/reopening. Clear them when switching to literal source or removing the option.

Update both standalone collection mapper rules in `CollectionMappingsPanel.tsx` and the rule editor in `PipelinePanel.tsx`. Update the shared collection-response `BindingRow` and its conversion helpers in `CollectionResponseEditor.tsx`, covering filters, data rules, and overrides. Check for other `FieldMappingRule`/`ValueBinding` serializers before implementation so no editor silently drops the field.

## Verification

- Resolver tables: missing source, missing key, missing nested path, and out-of-range array index use defaults; existing values always win.
- Typed values: preserve false, zero, empty string, empty object/array, and explicit null; support each as a default where allowed.
- Header cases: present-empty versus absent, case-insensitive names, and zero-value slices. Session/store entries that exist with nil values do not trigger a legacy default; preserve their existing string conversion behavior.
- Required writes: missing field with default succeeds, without default still fails; defaults do not cause writes during candidate matching or preview.
- Selection: defaulted filters select real matching data and still fall through when no document matches.
- Rendering: missing `mapper.outputPath` and document paths use defaults; existing values override defaults; verify precedence against `fallbackToExample` and both template/identity modes.
- Validation: malformed typed defaults, invalid legacy default types, literal-source defaults, undeclared mapper references, and invalid source contexts are rejected.
- Compatibility and round trips: old configurations behave unchanged; create/update, clone, save/reload, and archive import/export preserve configured defaults, including explicit null and empty strings.
- UI: enabling/disabling defaults, invalid JSON feedback, source switching, row reordering, and all applicable editors in light/dark themes.

Run focused collection resolver, collection response, model, and API tests, then the repository's normal Go and UI checks. Update `docs/collection-response-operations.md` and mapper documentation with examples and the missing-versus-empty rule. No new trace model, transaction behavior, or dependency is needed for this feature.
