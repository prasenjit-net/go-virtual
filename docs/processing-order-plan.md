# Request Processing Order — Redesign Plan

## 1. Motivation

The current engine conflates response source selection into a single `selectMode` function that returns one winner (Standard, Proxy, or AI). This makes the pipeline hard to reason about and prevents independent configurability of each stage. The redesign separates every stage into an explicit, ordered pipeline step. Each step either **short-circuits** (returns a response) or **falls through** to the next.

---

## 2. New Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│                     Incoming HTTP Request                       │
└────────────────────────────┬────────────────────────────────────┘
                             │
                    ① OPERATION MATCHING
                    Find spec + operation by method + path.
                    → No match: 404 immediately.
                             │
                    ② INITIALISATION
                    Read request body, compute signature,
                    resolve or lazily create session.
                             │
                    ③ SCRIPTS
                    Run spec-level bindings, then
                    operation-level bindings (in order).
                    Populates ScriptOutput → available to
                    all subsequent pipeline steps.
                             │
                    ④ RECORDED RESPONSE CHECK
                    Signature-based lookup against stored
                    ResponseConfigs with origin="proxy".
                    → Match found: return recorded response,
                      skip remaining steps.
                             │
                    ⑤ PROXY
                    Evaluate proxy conditions (can use
                    ScriptOutput). If proxy is enabled AND
                    conditions match AND backendURI is set:
                      – Forward request to backend.
                      – If DisableRecording=false: save
                        response as a recorded ResponseConfig.
                      – Return proxied response.
                    → Not triggered (disabled / no backend /
                      conditions not matched): fall through.
                             │
                    ⑥ VALIDATIONS
                    Run spec-level ValidationRules (by order),
                    then operation-level ValidationRules.
                    Each rule evaluates a ConditionNode tree
                    (can use ScriptOutput via source=script).
                    Populates ValidationOutput → available to
                    steps ⑦ and ⑧.
                             │
                    ⑦ CONFIGURED RESPONSE MATCHING
                    Condition-match against all enabled
                    ResponseConfigs for this operation
                    (origin="manual" or "ai", not "proxy").
                    Conditions can use ScriptOutput AND
                    ValidationOutput (source=validation).
                    → Match found: render and return.
                             │
                    ⑧ AI FALLBACK
                    Evaluate AI conditions (can use
                    ScriptOutput + ValidationOutput).
                    If AI is enabled, configured, AND
                    conditions match:
                      – Generate response via AI provider.
                      – Return AI-generated response.
                    → Not triggered: fall through.
                             │
                    ⑨ SPEC EXAMPLE FALLBACK
                    If spec.UseExampleFallback is true:
                      – Extract first example from OpenAPI
                        spec for this operation + method.
                      – Return example response.
                    → Not found or disabled: fall through.
                             │
                    ⑩ 404
                    No response could be produced.
                    Return HTTP 404.
```

---

## 3. Step-by-Step Details

### ③ Scripts

No change to script execution logic. Spec-level bindings run first (shared base), then operation-level bindings (can override by key). The combined `scriptOutput` map is injected into `condition.RequestData.ScriptOutput` and used in all subsequent steps.

**What changes**: Scripts now run before the recorded response check. Currently they run just before response config matching. Moving them earlier ensures their output is available to proxy condition evaluation.

### ④ Recorded Response Check

- Filter `ResponseConfig` by `operationId` + `recorded=true` (i.e. `origin="proxy"`)
- Match by **signature equality** (same as today — `config.Signature == reqData.Signature`)
- Sort by `priority` ASC, return first match
- If matched: return immediately; skip steps ⑤–⑩
- **No condition evaluation** — recorded responses are signature-matched only

This step is currently performed inside the general response config scan after scripts + mode selection. Moving it explicitly to step ④ makes it the highest-priority virtual response, checked before any proxy forwarding, validation, or configured matching.

### ⑤ Proxy

Replaces the current "proxy mode" selection. Now an independent step.

**Trigger conditions** (all must be true to activate):
1. `spec.ModePolicy.Proxy.Enabled == true`
2. `spec.BackendURI != ""`
3. All `spec.ModePolicy.Proxy.Conditions` match `reqData` (ScriptOutput available)

**Behaviour when triggered**:
- Forward request to `spec.BackendURI` with full headers + body
- Receive backend response
- If `spec.ModePolicy.Proxy.DisableRecording == false`: persist the response as a `ResponseConfig` with `origin="proxy"`, `recorded=true`, and the request signature stored for future step ④ matches
- Return the proxied response; skip steps ⑥–⑩

**Behaviour when not triggered** (disabled / no backend / conditions not matched):
- Fall through to step ⑥ silently
- Log skip reason for tracing (`proxy_skipped_reason`)

The `ModePolicy.Proxy` struct (`ConditionalModeConfig`) is unchanged:

```go
type ConditionalModeConfig struct {
    Enabled           bool        `json:"enabled"`
    DisableRecording  bool        `json:"disableRecording,omitempty"`
    Conditions        []Condition `json:"conditions"`
}
```

### ⑥ Validations

See [validation-plan.md](validation-plan.md) for the full specification. Summary:

- Run spec-level `ValidationRule`s (ordered by `.order` ASC)
- Run operation-level `ValidationRule`s (ordered by `.order` ASC)
- Each rule evaluates its `ConditionNode` tree against `reqData` (ScriptOutput available via `source=script`)
- Injects `ValidationOutput` into `reqData.ValidationOutput`
- Does **not** short-circuit — all enabled rules always run; the output feeds steps ⑦ and ⑧

### ⑦ Configured Response Matching

- Filter `ResponseConfig` where `recorded=false` (excludes proxy-recorded responses)
- Sort by `priority` ASC
- For each config: evaluate `config.Conditions` against full `reqData` (ScriptOutput + ValidationOutput)
- New condition source `"validation"` is available here: `source=validation, key=<ruleName>.status` or `source=validation, key=<ruleName>.<property>`
- First match wins; render and return

**What changes**: Recorded responses are excluded from this scan (they were already matched in step ④ or not at all). The condition evaluator gains access to `ValidationOutput`.

### ⑧ AI Fallback

Replaces the current "AI mode" selection. Now an independent step.

**Trigger conditions** (all must be true):
1. `spec.ModePolicy.AI.Enabled == true`
2. `aiGenerator.IsConfigured() == true`
3. All `spec.ModePolicy.AI.Conditions` match `reqData` (ScriptOutput + ValidationOutput available)

**Behaviour when triggered**:
- Resolve AI scenario from `X-Virtual-AI-Scenario` header
- Call `aiGenerator.GenerateRuntimeResponse`
- Return AI-generated response; skip steps ⑨–⑩

**Behaviour when not triggered**: fall through.

The `ModePolicy.AI` struct (`ConditionalModeConfig`) is unchanged. Note that `DisableRecording` on the AI config has no effect (AI responses are never recorded).

### ⑨ Spec Example Fallback

No behavioural change. Only runs if `spec.UseExampleFallback == true` and the spec contains an example response for this operation.

### ⑩ 404

HTTP 404 with standard body.

---

## 4. `selectMode` Removal

The current `selectMode` function selects a single winner from proxy/AI/standard. It is replaced by:

```go
// proxyConditionsMet reports whether the proxy step should activate.
func (e *Engine) proxyConditionsMet(spec *models.Spec, reqData *condition.RequestData) (bool, string)

// aiConditionsMet reports whether the AI fallback step should activate.
func (e *Engine) aiConditionsMet(spec *models.Spec, reqData *condition.RequestData) (bool, string)
```

Each returns `(activate bool, skippedReason string)`. The `modeSelection` struct is removed. Tracing fields `ProxySkippedReason` and `AISkippedReason` are kept on the trace model, populated by these two functions.

---

## 5. `findMatchingResponseConfig` Changes

Current signature:
```go
func (e *Engine) findMatchingResponseConfig(
    configs []*models.ResponseConfig,
    reqData *condition.RequestData,
    enabledTags map[string]struct{},
    recordedOnly bool,
) *models.ResponseConfig
```

The `recordedOnly bool` parameter becomes unnecessary — recorded responses are handled separately in step ④. The function is simplified to match **only non-recorded** configs:

```go
func (e *Engine) findMatchingResponseConfig(
    configs []*models.ResponseConfig,
    reqData *condition.RequestData,
    enabledTags map[string]struct{},
) *models.ResponseConfig
```

A separate function handles recorded response lookup:

```go
func (e *Engine) findRecordedResponse(
    configs []*models.ResponseConfig,
    signature string,
) *models.ResponseConfig
```

---

## 6. `condition.RequestData` Changes

Add `ValidationOutput` field (see validation plan):

```go
type RequestData struct {
    PathParams      map[string]string
    QueryParams     map[string][]string
    Headers         map[string][]string
    Body            string
    Signature       string
    ScriptOutput    map[string]any
    ValidationOutput map[string]*models.ValidationResult // NEW — populated at step ⑥
}
```

---

## 7. Tracing Changes

The `Trace` model gains a step-level breakdown to reflect the new pipeline. The existing fields are re-mapped:

| Trace field | Populated at step |
|---|---|
| `Scripts` | ③ |
| `ResponseSource = "config"` | ④ (recorded) or ⑦ (configured) |
| `ResponseTier = "recorded"` | ④ |
| `ResponseTier = "configured"` | ⑦ |
| `ProxyMode`, `BackendURI` | ⑤ |
| `Validations` (new) | ⑥ |
| `ResponseSource = "ai"` | ⑧ |
| `ResponseSource = "example"` | ⑨ |
| `ProxySkippedReason` | ⑤ (when not triggered) |
| `AISkippedReason` | ⑧ (when not triggered) |

---

## 8. UI — Mode Policy Editor Changes

The current spec settings UI has a single "Mode" selector (Standard / Proxy / AI) with a condition editor for each. Under the new model, both proxy and AI are independent toggles with their own condition editors. No mode is "selected" — any combination can be active.

Suggested UI layout for spec settings, "Processing" section:

```
┌─ Recorded Responses ─────────────────────────────┐
│  Checked first (signature match) — no config     │
└──────────────────────────────────────────────────┘
┌─ Proxy  [enabled toggle] ───────────────────────┐
│  Backend URI  [https://api.example.com         ] │
│  Disable Recording  [ ]                          │
│  Conditions (optional — AND logic)               │
│    [+ Add condition]                             │
└──────────────────────────────────────────────────┘
┌─ Configured Responses ──────────────────────────┐
│  Always attempted after proxy step.              │
│  Configured on each operation's Responses tab.  │
└──────────────────────────────────────────────────┘
┌─ AI Fallback  [enabled toggle] ────────────────┐
│  Provider  [openai ▾]   Model  [gpt-4o ▾]      │
│  Conditions (optional — AND logic)              │
│    [+ Add condition]                            │
└──────────────────────────────────────────────────┘
┌─ Spec Example Fallback  [enabled toggle] ──────┐
│  Returns first matching example from spec YAML  │
└──────────────────────────────────────────────────┘
```

The old "Mode" radio/select is removed. The `Spec.Mode` and `Spec.ProxyMode` fields (deprecated aliases of `ModePolicy`) continue to be accepted on input for backward compatibility but are not surfaced in the UI.

---

## 9. Implementation Phases

### Phase 1 — Engine refactoring (no new features)

1. Introduce `findRecordedResponse` helper; remove `recordedOnly` param from `findMatchingResponseConfig`
2. Move script execution to before the recorded response check
3. Replace `selectMode` with `proxyConditionsMet` and `aiConditionsMet`
4. Restructure `ServeHTTP` handler into the explicit 10-step pipeline
5. Update tracing to record proxy/AI skip reasons at the correct pipeline step
6. All existing tests must pass unchanged

### Phase 2 — Validation integration

Wire in `validation.RunRules` at step ⑥ (per [validation-plan.md](validation-plan.md) Phase 1).

### Phase 3 — UI update

Redesign spec settings "Processing" section per §8 above.

---

## 10. Backward Compatibility Notes

| Existing behaviour | New behaviour |
|---|---|
| Proxy mode is mutually exclusive with AI mode | Both are independent; both can be configured; proxy runs first |
| Recorded responses are matched after configured responses in the same scan | Recorded responses are matched first, before configured responses |
| Script output unavailable to proxy conditions | Script output available to proxy conditions (scripts run earlier) |
| `selectMode` returns a single active mode | No single mode; each stage evaluated independently |
| `source=validation` not a valid condition source | Added in Phase 2 |

The `ModePolicy.Configured` flag (currently used to disable the configured response step) is preserved — when `false`, step ⑦ is skipped.

---

## 11. Update to Validation Plan

The execution order in [validation-plan.md](validation-plan.md) §2.4 should be read as:

```
Request arrives
  │
  ├─ ① Operation matching
  ├─ ② Initialisation (body, signature, session)
  ├─ ③ Scripts (spec → operation level)
  ├─ ④ Recorded response check  ──► return if matched
  ├─ ⑤ Proxy step               ──► return if triggered
  ├─ ⑥ Validations (spec → operation level)
  ├─ ⑦ Configured response matching  ──► return if matched
  ├─ ⑧ AI fallback              ──► return if triggered
  ├─ ⑨ Spec example fallback    ──► return if found
  └─ ⑩ 404
```

The validation plan's §2.4 diagram is superseded by this document.
