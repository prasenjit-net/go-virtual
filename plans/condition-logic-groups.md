# Plan: Condition Logic Groups (AND / OR / NOT)

**Priority:** P2  
**Effort:** M (2–3 days)  
**Target release:** v1.5.0  
**Related:** [improvement-roadmap.md § 2.2](improvement-roadmap.md)

---

## Problem

All response config conditions are evaluated with strict **AND** semantics — "all conditions must match". The evaluator comment reads:

```go
// EvaluateAll evaluates all conditions against request data
// All conditions must match (AND logic)
```

This cannot express:
- "Match if **header X = A** OR **header X = B**"
- "Match if (body contains 'premium') OR (query param `tier` = 'pro')"
- "Match if **method = POST** AND (**body.amount > 100** OR **header X-VIP = true**)"

Users work around this by creating duplicate response configs — one per OR branch — but that means maintaining duplicate bodies and priorities.

---

## Goal

1. Introduce a `ConditionGroup` model that nests conditions with explicit `and`/`or` logic.
2. Support `not` at the group or leaf level.
3. Keep the existing flat `[]Condition` slice API **fully backwards-compatible** — no migration required for existing response configs.
4. Update the condition evaluator to handle both forms.
5. Update the admin UI to render and edit nested groups visually.

---

## Data Model

### New `ConditionGroup` type

```go
// ConditionGroup is a recursive structure that combines conditions with
// logical operators. It replaces (and wraps) the flat []Condition slice.
type ConditionGroup struct {
    // Logic controls how children are combined. "and" (default) | "or"
    Logic string `json:"logic"`

    // Negate inverts the result of the entire group (NOT).
    Negate bool `json:"negate,omitempty"`

    // Conditions are leaf-level condition checks within this group.
    Conditions []Condition `json:"conditions,omitempty"`

    // Groups are nested ConditionGroups evaluated with this group's Logic.
    Groups []ConditionGroup `json:"groups,omitempty"`
}
```

### `ResponseConfig` change

Add a new field alongside the existing one:

```go
type ResponseConfig struct {
    // ... existing fields ...

    // Conditions is the legacy flat list (AND logic). Kept for backwards compat.
    // Deprecated: prefer ConditionGroup.
    Conditions []Condition `json:"conditions,omitempty"`

    // ConditionGroup is the new structured condition tree.
    // When set, it takes precedence over Conditions.
    ConditionGroup *ConditionGroup `json:"conditionGroup,omitempty"`
}
```

Backward-compatibility rule: if `ConditionGroup` is `nil`, fall back to evaluating the flat `Conditions` slice with AND logic (existing behaviour — zero code change required for existing data).

---

## Evaluator Changes

### New method

```go
// EvaluateGroup evaluates a ConditionGroup tree against request data.
func (e *Evaluator) EvaluateGroup(group *ConditionGroup, data *RequestData) bool {
    if group == nil {
        return true
    }

    var results []bool

    // Evaluate all leaf conditions
    for _, cond := range group.Conditions {
        results = append(results, e.Evaluate(cond, data))
    }

    // Evaluate nested groups
    for _, sub := range group.Groups {
        results = append(results, e.EvaluateGroup(&sub, data))
    }

    if len(results) == 0 {
        return true // empty group = match-all
    }

    var combined bool
    switch strings.ToLower(group.Logic) {
    case "or":
        combined = false
        for _, r := range results {
            if r {
                combined = true
                break
            }
        }
    default: // "and"
        combined = true
        for _, r := range results {
            if !r {
                combined = false
                break
            }
        }
    }

    if group.Negate {
        return !combined
    }
    return combined
}
```

### Proxy engine change

In `proxy/engine.go`, the matching call changes from:

```go
if e.condEvaluator.EvaluateAll(cfg.Conditions, reqData) {
```

to:

```go
matched := false
if cfg.ConditionGroup != nil {
    matched = e.condEvaluator.EvaluateGroup(cfg.ConditionGroup, reqData)
} else {
    matched = e.condEvaluator.EvaluateAll(cfg.Conditions, reqData)
}
if matched {
```

---

## API Changes

### Request body for `POST /_api/operations/:id/responses`

```json
{
  "name": "Premium user response",
  "statusCode": 200,
  "body": "{ \"tier\": \"premium\" }",
  "conditionGroup": {
    "logic": "or",
    "conditions": [
      { "source": "header", "key": "X-Tier", "operator": "eq", "value": "premium" },
      { "source": "query", "key": "tier", "operator": "eq", "value": "pro" }
    ]
  }
}
```

The `conditions` flat array field continues to work as before.

---

## Admin UI Changes

### Response designer — condition editor

Replace the current flat condition list with a **group-aware condition builder**:

```
┌─ Condition Group: [AND ▾] [NOT □]  [+ Add Condition] [+ Add Group] ─────┐
│  source: [header ▾]  key: [X-Auth]  operator: [eq ▾]  value: [Bearer x] │
│                                                                [✕]       │
│  ┌─ Nested Group: [OR ▾] [NOT □]  [+ Add Condition] ──────────────────┐ │
│  │  source: [query ▾]  key: [role]  operator: [eq ▾]  value: [admin]  │ │
│  │  source: [body ▾]   key: [role]  operator: [eq ▾]  value: [admin]  │ │
│  └──────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────┘
```

Key UI interactions:
- **Logic toggle** (AND / OR) — toggles the `logic` field of the group.
- **NOT checkbox** — toggles `negate`.
- **+ Add Condition** — appends a leaf condition to the current group.
- **+ Add Group** — appends a nested `ConditionGroup`.
- **Drag handles** — reorder conditions within a group (using existing `@dnd-kit`).

### Flat → group migration helper

When a response config with a flat `conditions` array is loaded in the UI but has no `conditionGroup`, the frontend synthesises a `{ logic: "and", conditions: [...] }` group for the editor. On save it writes back `conditionGroup` (the flat `conditions` field is preserved for backend compat).

---

## Storage Impact

No storage schema changes are required. `ConditionGroup` is stored as part of the `ResponseConfig` JSON blob (already a JSON column / file). Adding a new optional field is non-breaking for both `FileStorage` and `MemoryStorage`.

---

## Test Plan

### Unit tests — `internal/condition/evaluator_test.go`

New test cases:
```go
TestEvaluateGroup_AND_all_match        // both leaf conditions match
TestEvaluateGroup_AND_one_fails        // one leaf fails → false
TestEvaluateGroup_OR_one_matches       // one leaf matches → true
TestEvaluateGroup_OR_none_match        // no leaves match → false
TestEvaluateGroup_NOT_AND              // negate=true inverts result
TestEvaluateGroup_nested_AND_OR        // (A AND B) OR (C AND D)
TestEvaluateGroup_empty_group          // no conditions → true
TestEvaluateGroup_nil                  // nil group → true
TestEvaluateAll_fallback_compat        // flat slice still works
```

### Integration tests

- Upload a spec, create a response with `conditionGroup` (OR logic), send two requests each matching one branch — both should return the response.
- Legacy flat `conditions` array — existing behaviour unchanged.

---

## Acceptance Criteria

- [ ] `ConditionGroup` model exists in `internal/models/condition.go`
- [ ] `EvaluateGroup` method on `Evaluator` works for AND, OR, NOT, nested groups
- [ ] Proxy engine uses `ConditionGroup` when set, falls back to flat `Conditions`
- [ ] API accepts and stores `conditionGroup` in request body
- [ ] Existing response configs with flat `conditions` continue to work unchanged
- [ ] Admin UI renders flat conditions as an AND group
- [ ] Admin UI allows creating/editing nested groups
- [ ] All existing tests pass
- [ ] New evaluator tests added
