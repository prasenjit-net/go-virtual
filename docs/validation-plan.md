# Request Validation — Implementation Plan

## 1. Overview

Request validation lets operators declaratively describe what a valid (or invalid) request looks like for a given spec or operation. Each **ValidationRule** evaluates a condition tree against the incoming request and injects a named result block into the request context. That result block is then available for:

- **Response config condition matching** — a new `validation` condition source
- **Template variable resolution** — `{{.Validation.<name>.status}}`, `{{.Validation.<name>.<key>}}`

Validations run in order, after operation-level scripts but before response config matching, so their output can influence which response config is selected.

---

## 2. Core Concepts

### 2.1 ValidationRule

A ValidationRule has:

| Field | Type | Description |
|---|---|---|
| `id` | string | UUID |
| `specId` | string? | Set for spec-level rules |
| `operationId` | string? | Set for operation-level rules |
| `name` | string | Unique key used in template/condition access; alphanumeric + underscore |
| `description` | string? | Human-readable description |
| `order` | int | Execution order (lower = first) |
| `enabled` | bool | Whether rule runs |
| `conditionTree` | ConditionNode | Tree of conditions evaluated against the request |
| `onSuccess` | map[string]string | Properties added to context when tree evaluates to `true` |
| `onFailure` | map[string]string | Properties added to context when tree evaluates to `false` |

Exactly one of `specId` / `operationId` is set, defining the scope.

### 2.2 ConditionNode (tree structure)

The current response-config conditions are a flat `[]Condition` (implicitly ANDed). Validations introduce a proper tree to express AND/OR/NOT:

```go
// ConditionNode is either a leaf (single Condition) or a composite group.
type ConditionNode struct {
    // Leaf — set when this node is a single condition
    Condition *models.Condition `json:"condition,omitempty"`

    // Group — set when this node combines children
    Operator string           `json:"operator,omitempty"` // "AND" | "OR" | "NOT"
    Children []*ConditionNode `json:"children,omitempty"`
}
```

Rules:
- A **leaf** node has `Condition` set and `Operator`/`Children` empty.
- A **group** node has `Operator` and at least one child; `Condition` is empty.
- `NOT` groups take exactly one child.
- An empty tree (`nil` root) is always `true` (validation always passes — useful as a catch-all property injector).

### 2.3 Context Output Shape

After all ValidationRules run, the following is available:

```
.Validation.<name>.status        → "pass" | "fail"
.Validation.<name>.<key>         → string value from onSuccess or onFailure map
```

Example: a rule named `auth_check` with `onFailure: {error_code: "AUTH_MISSING", message: "Token required"}`:

```
.Validation.auth_check.status       → "fail"
.Validation.auth_check.error_code   → "AUTH_MISSING"
.Validation.auth_check.message      → "Token required"
```

### 2.4 Execution Order

```
Request arrives
  │
  ├─ 1. Spec-level scripts (existing)
  ├─ 2. Operation-level scripts (existing)
  │       └─ ScriptOutput → RequestData
  │
  ├─ 3. Spec-level ValidationRules (ordered by .order ASC)
  ├─ 4. Operation-level ValidationRules (ordered by .order ASC)
  │       └─ ValidationOutput → RequestData + template.Context
  │
  ├─ 5. Response config matching (conditions can use source="validation")
  └─ 6. Response rendering (templates can use .Validation.*)
```

---

## 3. Data Model Changes

### 3.1 New file: `internal/models/validation.go`

```go
package models

import "time"

// ValidationRule defines a named request validation attached to a spec or operation.
type ValidationRule struct {
    ID          string     `json:"id"`
    SpecID      string     `json:"specId,omitempty"`
    OperationID string     `json:"operationId,omitempty"`
    Name        string     `json:"name"`        // alphanumeric + underscore, used as template key
    Description string     `json:"description,omitempty"`
    Order       int        `json:"order"`
    Enabled     bool       `json:"enabled"`
    ConditionTree *ConditionNode `json:"conditionTree"`
    OnSuccess   map[string]string `json:"onSuccess,omitempty"`
    OnFailure   map[string]string `json:"onFailure,omitempty"`
    CreatedAt   time.Time  `json:"createdAt"`
    UpdatedAt   time.Time  `json:"updatedAt"`
}

// ValidationInput is the create/update payload.
type ValidationInput struct {
    Name        string     `json:"name"`
    Description string     `json:"description,omitempty"`
    Order       int        `json:"order"`
    Enabled     bool       `json:"enabled"`
    ConditionTree *ConditionNode `json:"conditionTree"`
    OnSuccess   map[string]string `json:"onSuccess,omitempty"`
    OnFailure   map[string]string `json:"onFailure,omitempty"`
}

// ConditionNode is a node in the AND/OR/NOT condition tree used by ValidationRules.
type ConditionNode struct {
    // Leaf node (single condition)
    Condition *Condition `json:"condition,omitempty"`

    // Composite group node
    Operator string           `json:"operator,omitempty"` // "AND" | "OR" | "NOT"
    Children []*ConditionNode `json:"children,omitempty"`
}

// ValidationResult holds the evaluated output for one ValidationRule.
type ValidationResult struct {
    Status     string            // "pass" | "fail"
    Properties map[string]string // merged from OnSuccess or OnFailure
}
```

### 3.2 New condition source: `SourceValidation = "validation"`

Add to `internal/models/condition.go`:

```go
SourceValidation = "validation" // key format: "<ruleName>.status" or "<ruleName>.<property>"
```

Update `ValidSources()` to include it.

---

## 4. Storage Layer

### 4.1 Interface additions (`internal/storage/interface.go`)

```go
// Validation rules
ListValidationRulesBySpec(specID string) ([]*models.ValidationRule, error)
ListValidationRulesByOperation(operationID string) ([]*models.ValidationRule, error)
GetValidationRule(id string) (*models.ValidationRule, error)
CreateValidationRule(rule *models.ValidationRule) (*models.ValidationRule, error)
UpdateValidationRule(rule *models.ValidationRule) (*models.ValidationRule, error)
DeleteValidationRule(id string) error
```

### 4.2 Memory + File backends (`memory.go`, `file.go`)

Standard in-memory map implementation. File backend persists to `validations.json` alongside other entity files.

### 4.3 MongoDB backend (`mongo.go`)

New `validations` collection. Promoted BSON fields for filtering:

| BSON field | Populated from | Used by |
|---|---|---|
| `spec_id` | `rule.SpecID` | spec-scoped queries |
| `operation_id` | `rule.OperationID` | operation-scoped queries |

`EnsureIndexes` additions:
- `{ spec_id: 1 }` on `validations`
- `{ operation_id: 1 }` on `validations`

`ConditionTree` is stored as part of the JSON blob in `genericDoc.Data` — no promotion needed since it is never queried server-side (evaluation happens in Go).

---

## 5. Evaluation Engine

### 5.1 Tree evaluator (`internal/condition/evaluator.go`)

New method on `*Evaluator`:

```go
// EvaluateTree evaluates a ConditionNode tree against request data.
// A nil root always returns true.
func (e *Evaluator) EvaluateTree(node *models.ConditionNode, data *RequestData) bool {
    if node == nil {
        return true
    }
    if node.Condition != nil {
        return e.Evaluate(*node.Condition, data)
    }
    switch strings.ToUpper(node.Operator) {
    case "AND":
        for _, child := range node.Children {
            if !e.EvaluateTree(child, data) { return false }
        }
        return true
    case "OR":
        for _, child := range node.Children {
            if e.EvaluateTree(child, data) { return true }
        }
        return false
    case "NOT":
        if len(node.Children) != 1 { return false }
        return !e.EvaluateTree(node.Children[0], data)
    }
    return false
}
```

### 5.2 Validation source in `extractValue`

```go
case models.SourceValidation:
    // key format: "<ruleName>.status" or "<ruleName>.<property>"
    parts := strings.SplitN(key, ".", 2)
    if len(parts) != 2 || data.ValidationOutput == nil {
        return ""
    }
    result, ok := data.ValidationOutput[parts[0]]
    if !ok { return "" }
    if parts[1] == "status" { return result.Status }
    return result.Properties[parts[1]]
```

### 5.3 `RequestData` extension

Add to `RequestData`:

```go
ValidationOutput map[string]*models.ValidationResult // keyed by rule Name
```

### 5.4 New `internal/validation/runner.go`

```go
package validation

// Runner executes ValidationRules against a request and returns a named result map.
func RunRules(rules []*models.ValidationRule, data *condition.RequestData) map[string]*models.ValidationResult {
    out := make(map[string]*models.ValidationResult)
    eval := condition.NewEvaluator()
    for _, rule := range rules {
        if !rule.Enabled { continue }
        passed := eval.EvaluateTree(rule.ConditionTree, data)
        status := "pass"
        props := rule.OnSuccess
        if !passed {
            status = "fail"
            props = rule.OnFailure
        }
        out[rule.Name] = &models.ValidationResult{
            Status:     status,
            Properties: props,
        }
    }
    return out
}
```

The proxy engine (`internal/proxy/engine.go`) calls `validation.RunRules` after scripts, populates `reqData.ValidationOutput`, then proceeds to response matching.

---

## 6. Template Engine Changes

### 6.1 `template.Context` addition

```go
// ValidationOutput holds results from validation rules, keyed by rule name.
ValidationOutput map[string]*models.ValidationResult
```

### 6.2 Template resolution

In `engine.go`, add a new `{{.Validation.<name>.<key>}}` function (same pattern as `{{.Script.*}}`):

```go
"Validation": func(name, key string) string {
    if ctx.ValidationOutput == nil { return "" }
    result, ok := ctx.ValidationOutput[name]
    if !ok { return "" }
    if key == "status" { return result.Status }
    return result.Properties[key]
},
```

Alternatively, register `.Validation` as a map in the template data map so Go's native dot-access works:
```
{{.Validation.auth_check.status}}
{{.Validation.auth_check.error_code}}
```

The proxy engine copies `reqData.ValidationOutput` into `templateCtx.ValidationOutput` before rendering.

---

## 7. Admin API

### 7.1 New routes in `internal/api/router.go`

```
GET  /_api/specs/:id/validations          list spec-level rules
POST /_api/specs/:id/validations          create spec-level rule

GET  /_api/operations/:id/validations     list operation-level rules
POST /_api/operations/:id/validations     create operation-level rule

GET    /_api/validations/:id              get rule
PUT    /_api/validations/:id              update rule
DELETE /_api/validations/:id             delete rule
```

### 7.2 New handler file: `internal/api/handler_validations.go`

Standard CRUD handlers. Validation on input:
- `name` must match `^[a-zA-Z_][a-zA-Z0-9_]*$`
- `conditionTree` is allowed to be null (empty tree = always pass)
- `operator` on group nodes must be `AND`, `OR`, or `NOT`
- `NOT` nodes must have exactly one child

---

## 8. Tracing

Add a `Validations []ValidationTrace` field to `models.Trace`:

```go
type ValidationTrace struct {
    RuleID     string            `json:"ruleId"`
    RuleName   string            `json:"ruleName"`
    Scope      string            `json:"scope"` // "spec" | "operation"
    Status     string            `json:"status"` // "pass" | "fail"
    Properties map[string]string `json:"properties,omitempty"`
    DurationMs int64             `json:"durationMs"`
}
```

The trace viewer gains a "Validations" section similar to the existing Scripts section.

---

## 9. UI

### 9.1 New components

```
ui/src/components/Validation/
  ValidationRulesPanel.tsx     list + manage rules for a spec or operation
  ValidationRuleEditor.tsx     create/edit a single rule (name, order, on-success, on-failure)
  ConditionTreeEditor.tsx      recursive AND/OR/NOT tree builder
  ConditionNodeEditor.tsx      single leaf condition row (source/key/operator/value)
```

### 9.2 ConditionTreeEditor

The tree editor is the most complex UI piece. Design:

```
┌──────────────────────────────────────────────────┐
│  AND group                              [+ Add]  │
│  ├─ source: header  key: Authorization           │
│  │    op: exists                       [×]       │
│  └─ OR group                           [+ Add]  │
│     ├─ source: header  key: X-Role              │
│     │    op: eq  value: admin          [×]       │
│     └─ source: header  key: X-Role              │
│          op: eq  value: superuser      [×]       │
└──────────────────────────────────────────────────┘
```

Each node has:
- **Leaf**: source / key / operator / value / negate toggle — same fields as existing `ConditionRow` component but not inside a flat list
- **Group (AND/OR)**: header with operator toggle (AND↔OR) + `+ Add Condition` + `+ Add Group` + `+ Add NOT` buttons + children
- **NOT**: single-child group; header shows "NOT"; `+ Add` disabled after one child

Re-use the existing condition source/operator field components where possible.

### 9.3 ValidationRulesPanel

Similar layout to `ScriptBindingsPanel`:
- List of rules with name, scope badge (spec/operation), status toggle, order arrows, delete
- Click to expand inline editor

### 9.4 Rule Editor

```
Name *         [auth_check            ]
Description    [Validates auth header ]
Order          [0    ]   Enabled [✓]

Condition Tree
┌─────────────────────────────────────┐
│  AND                    [+ Add]     │
│  └─ header / Authorization / exists │
└─────────────────────────────────────┘

On Success (properties when tree = true)
  [+ Add property]
  error_code  │  [                    ]  [×]

On Failure (properties when tree = false)
  [+ Add property]
  error_code  │  AUTH_MISSING          [×]
  message     │  Token required        [×]
```

Property values support Go template expressions (e.g. `{{.header.X-User-ID}}`).

### 9.5 Where validations appear in the UI

- **Spec detail page** — new "Validations" tab alongside Ops, Scripts, Settings
- **Operation detail panel** — new "Validations" tab in the operation sidebar
- **Response Config IDE** — read-only "Active Validations" info panel showing which rules will run (spec + operation level), no editing (editing is done at spec/operation level)
- **Trace detail** — new "Validations" section in the trace inspector

### 9.6 Response condition source update

In the existing `ConditionRow` / condition editor used in response configs, add `validation` to the source dropdown. When selected:
- Key field shows a composed input: `<ruleName>.<property>` with autocomplete for known rule names in the current spec/operation context

---

## 10. Type additions (`ui/src/types/index.ts`)

```typescript
export interface ValidationRule {
    id: string
    specId?: string
    operationId?: string
    name: string
    description?: string
    order: number
    enabled: boolean
    conditionTree: ConditionNode | null
    onSuccess?: Record<string, string>
    onFailure?: Record<string, string>
    createdAt: string
    updatedAt: string
}

export interface ValidationInput {
    name: string
    description?: string
    order: number
    enabled: boolean
    conditionTree: ConditionNode | null
    onSuccess?: Record<string, string>
    onFailure?: Record<string, string>
}

export interface ConditionNode {
    condition?: Condition         // leaf
    operator?: 'AND' | 'OR' | 'NOT' // group
    children?: ConditionNode[]
}

export interface ValidationTrace {
    ruleId: string
    ruleName: string
    scope: 'spec' | 'operation'
    status: 'pass' | 'fail'
    properties?: Record<string, string>
    durationMs: number
}
```

---

## 11. Implementation Phases

### Phase 1 — Backend foundation
1. Add `ConditionNode` and `ValidationRule` / `ValidationResult` models
2. Add `SourceValidation` constant; update `ValidSources()`
3. Extend `RequestData` with `ValidationOutput`
4. Add `EvaluateTree` to `*Evaluator`; add validation source case in `extractValue`
5. Implement `internal/validation/runner.go`
6. Wire runner into `proxy/engine.go` (after scripts, before response matching)
7. Extend `template.Context` with `ValidationOutput`; wire template resolution
8. Add storage interface methods + implement in all three backends (memory, file, mongo)
9. Add API handlers + routes
10. Add `ValidationTrace` to trace model + populate in engine

### Phase 2 — UI
1. `ConditionNodeEditor` (leaf row — re-use existing condition fields)
2. `ConditionTreeEditor` (recursive group + leaf builder)
3. `ValidationRuleEditor` (name/order/onSuccess/onFailure + tree)
4. `ValidationRulesPanel` (list view for spec and operation scope)
5. Wire panels into spec detail, operation panel, and response config IDE
6. Add `validation` source option to existing response config condition editor with key autocomplete
7. Add Validations section to trace detail viewer

### Phase 3 — Polish
1. Unit tests for `EvaluateTree` (AND/OR/NOT, empty tree, deeply nested)
2. Unit tests for `RunRules` (enabled/disabled, pass/fail properties)
3. Storage integration tests (all three backends)
4. API handler tests
5. Archive export/import support for `ValidationRule`

---

## 12. Open Questions / Decisions

| Question | Recommendation |
|---|---|
| Should `OnSuccess`/`OnFailure` values support Go template expressions? | Yes — run them through the template engine with the current request context at evaluation time. This allows dynamic values like `{{.header.X-User-ID}}`. |
| Should validation rules run for proxy mode requests? | Yes — they enrich the context regardless of mode. |
| Name uniqueness scope | Unique per scope (spec or operation). A spec-level rule named `auth` and an operation-level rule named `auth` on the same operation would produce two separate `Validation.auth` entries — the operation-level one overwrites. **Decision needed.** |
| Should failed validations short-circuit response matching and return a fixed error? | Not in this plan — validation only injects context. Returning a fixed error would be a separate "auto-reject" feature. |
| Archive/snapshot support | Include `ValidationRule` in archive export/import (Phase 3). |
