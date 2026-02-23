# Scripting Support — Phase 2: Session Store

**Date:** 2026-02-22  
**Target Version:** v0.9.0  
**Status:** Draft / Planning  
**Depends on:** [scripting-phase1-plan.md](scripting-phase1-plan.md) — Core Starlark Engine

---

## 1. Overview

The Session Store is an **application-wide, persistent key–multivalue store** that serves as shared, stateful data for scripts. It is a first-class feature with its own Admin UI, API, and storage — not a bolt-on to scripting.

### Core Design Principles

| Principle | Detail |
|---|---|
| **Global store is admin-owned** | Only the Admin UI / Admin API can read or modify the global store |
| **Scripts get a session snapshot** | A script sees a per-session copy of the global store, never the live global data |
| **Session writes are isolated** | Scripts can mutate their session copy freely; changes never propagate back to global |
| **Sessions are request-scoped by identity** | A session is created or retrieved via the `X-Virtual-Session-Id` header |
| **Sessions expire on inactivity** | After a configurable idle period the session and its store copy are discarded |

### Motivating Use Cases

| Scenario | How the session store enables it |
|---|---|
| Sequential ID generation | Admin seeds `id-counter = 0`; session copy is incremented per call |
| Feature flag per session | Admin manages flags globally; session overrides for a specific test run |
| Call-count-based response variants | Script increments session counter; returns different shape after N calls |
| Stateful wizard / multi-step API flow | Session holds `step = 1..N`; each request advances it |
| Per-user data isolation | Each API consumer carries its own session ID; data is isolated |

---

## 2. Global Store

### 2.1 Data Model

The global store is a flat **key → JSON value** map. Values are unrestricted JSON types — string, number, boolean, array, or object — hence _multivalue_: a key can hold a list, enabling `["item1", "item2"]` as naturally as `"scalar"`.

```go
// StoreEntry represents one key-value pair in the global store.
type StoreEntry struct {
    Key       string    `json:"key"`
    Value     any       `json:"value"`     // Any JSON-serialisable type
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

Example global store contents:

```json
{
  "id-counter":    0,
  "feature-flags": { "darkMode": true, "betaSearch": false },
  "allowed-roles": ["admin", "editor", "viewer"],
  "greeting-prefix": "Hello",
  "rate-limit":    100
}
```

### 2.2 Storage — Single JSON File

The entire global store is persisted in one file:

```
data/
  store.json
```

File format:

```json
{
  "updatedAt": "2026-02-22T10:00:00Z",
  "entries": {
    "id-counter":    0,
    "feature-flags": { "darkMode": true, "betaSearch": false },
    "allowed-roles": ["admin", "editor", "viewer"]
  }
}
```

**Write behaviour:** the file is rewritten atomically (write to a `.tmp` sibling, then `os.Rename`) on every Admin API mutation. There is no partial-write risk.

**Load behaviour:** loaded once at startup into memory (`GlobalStore`). All subsequent reads use the in-memory copy. Writes update both the in-memory map and the file.

---

## 3. Session Lifecycle

### 3.1 Session Identification

Every proxied request is examined for the session header. The header name defaults to `X-Virtual-Session-Id` and is configurable:

```yaml
# config.yaml
session:
  headerName: "X-Virtual-Session-Id"   # Header to read and write
  inactivityTimeout: 30m               # Idle TTL before session is discarded
  echoHeader: true                     # Write session ID back into response headers
  maxSessions: 10000                   # Hard cap on concurrent sessions
```

### 3.2 Session Resolution per Request

```
Incoming proxied request
        │
        ▼
Read header: X-Virtual-Session-Id (or configured name)
        │
   ┌────┴──────────────────────────────────┐
   │ Header present and session found?     │
   │                                       │
   │ YES                    NO             │
   ▼                        ▼             │
Load session          Create new session  │
(update lastActive)   (deep-copy global   │
                       store entries)     │
   └────────────────────────┘
        │
        ▼
Attach session.Store to script execution context
        │
        ▼
Scripts execute (may read/write session store)
        │
        ▼
If echoHeader=true → write session ID into response header
```

**New session creation:**
1. Generate a UUID session ID.
2. Deep-copy all current global store entries into the session's private map.
3. Set `lastActive = now`.
4. Store in the in-memory session registry.

**Existing session retrieval:**
1. Look up session ID in the registry.
2. If found and not expired: update `lastActive`, return session.
3. If expired or not found: create a new session (step above), return it.

### 3.3 Session Expiry

A background goroutine runs every minute (configurable) and removes sessions where `now - lastActive > inactivityTimeout`. Expiry is lazy — the session is not checked mid-request, only between requests.

Expired sessions are simply removed from the in-memory registry. Their data is never written back to the global store.

### 3.4 Anonymous Requests

If a request carries no session header, an **ephemeral anonymous session** is created for the duration of that request only:
- Initialised with a snapshot of the global store.
- Writes are discarded after the response is sent.
- No session ID is echoed in the response.

This ensures scripts always have a valid `store` object regardless of whether the caller manages sessions.

---

## 4. Script Access to the Session Store

Scripts access the session store via a `store` builtin dict-like object injected at execution time. The `store` object is **read-write but session-scoped** — modifications affect only the current session's copy.

### 4.1 Starlark Builtins

```python
# Read a value (returns None if key not found)
counter = store.get("id-counter")               # → 0
flags   = store.get("feature-flags")            # → {"darkMode": True, ...}
roles   = store.get("allowed-roles")            # → ["admin", "editor", ...]

# Read with default
counter = store.get("id-counter", 0)

# Write a value (session-local only, does not affect global store)
store.set("id-counter", counter + 1)
store.set("last-caller", req["path"].get("id"))

# Check existence
if store.has("feature-flags"):
    flags = store.get("feature-flags")

# Delete a key (session-local)
store.delete("temp-key")

# List all keys in the session store
keys = store.keys()                             # → ["id-counter", "feature-flags", ...]
```

### 4.2 Using Store Output in Templates

Like all script output, store-derived values flow through the script's return value into `.script.<outputKey>.*`:

```python
def run(req):
    counter = store.get("id-counter", 0)
    store.set("id-counter", counter + 1)
    return {
        "nextId":    "USR-" + str(counter).zfill(4),
        "callCount": counter + 1,
    }
```

Response template:
```json
{
  "id":        "{{.script.ids.nextId}}",
  "callCount": "{{.script.ids.callCount}}"
}
```

### 4.3 Isolation Guarantees

| Boundary | Guarantee |
|---|---|
| Session A vs Session B | Completely isolated; writes in session A never visible to session B |
| Session vs Global store | Script writes never propagate to global store |
| Concurrent requests, same session | Reads/writes are serialised per session via a per-session mutex |
| After session expiry | All session data is discarded; a new session starts fresh from global |

---

## 5. Admin UI

### 5.1 Store Page — `/store`

New top-level sidebar page (between Scripts and Dashboard or at the bottom).

**Global Store List View**
- Table: Key, Value (JSON preview, truncated at 80 chars), Updated, Actions (Edit / Delete)
- Toolbar: "Add Entry" button, search/filter by key name
- Empty state: "No store entries. Add global data accessible to all scripts."

**Add / Edit Entry Modal**
- Key input (string, validated: no whitespace, no dots)
- Value editor — **Monaco Editor** in `json` language mode (supports scalars, arrays, objects)
- Existing value pre-populated on edit
- Inline validation: value must be valid JSON
- Save / Cancel

**Danger Zone** (collapsible, at page bottom)
- "Clear All Entries" — confirmation dialog required

### 5.2 Session Inspector — `/sessions`

Developer tool page for inspecting and managing live sessions.

**Session List View**
- Table: Session ID (truncated), Created, Last Active, Entry Count, Actions (View / Invalidate)
- Total session count badge
- "Invalidate All" button (confirmation required)
- Auto-refreshes every 30 seconds

**Session Detail View** (side panel or modal)
- Session metadata: ID, created, last active, entry count
- Read-only store snapshot table: Key, Value (JSON)
- "Invalidate This Session" button

> The session inspector is a read-only view from the admin perspective — session store data can only be seeded via the global store. Admins invalidate sessions; they do not edit session data directly.

---

## 6. API Endpoints

All endpoints are under the existing `/_api` group.

### 6.1 Global Store CRUD

| Method | Path | Description |
|---|---|---|
| `GET` | `/_api/store` | List all global store entries |
| `GET` | `/_api/store/:key` | Get a single entry by key |
| `PUT` | `/_api/store/:key` | Create or update an entry (upsert) |
| `DELETE` | `/_api/store/:key` | Delete an entry |
| `DELETE` | `/_api/store` | Clear all entries (requires `?confirm=true`) |

**PUT `/_api/store/:key` request body:**
```json
{ "value": ["admin", "editor"] }
```

**GET `/_api/store` response:**
```json
[
  { "key": "id-counter",    "value": 0,                                    "updatedAt": "..." },
  { "key": "feature-flags", "value": { "darkMode": true },                 "updatedAt": "..." },
  { "key": "allowed-roles", "value": ["admin", "editor"],                  "updatedAt": "..." }
]
```

### 6.2 Session Management

| Method | Path | Description |
|---|---|---|
| `GET` | `/_api/sessions` | List all active sessions (metadata only) |
| `GET` | `/_api/sessions/:id` | Get session metadata + store snapshot |
| `DELETE` | `/_api/sessions/:id` | Invalidate a session |
| `DELETE` | `/_api/sessions` | Invalidate all sessions (requires `?confirm=true`) |

---

## 7. Internal Package Structure

```
internal/
  store/
    global.go    # GlobalStore: in-memory map + load/save store.json
    session.go   # Session struct + SessionManager (create, get, expire)
    manager.go   # Combined StoreManager interface used by proxy engine and handlers
    builtin.go   # Starlark store builtin object (wraps session store map)
```

### 7.1 Key Types

```go
// GlobalStore holds the application-wide persistent key-value store.
type GlobalStore struct {
    mu        sync.RWMutex
    entries   map[string]StoreEntry
    filePath  string
}

func (g *GlobalStore) Get(key string) (any, bool)
func (g *GlobalStore) Set(key string, value any) error  // saves to file
func (g *GlobalStore) Delete(key string) error          // saves to file
func (g *GlobalStore) Clear() error                     // saves to file
func (g *GlobalStore) Snapshot() map[string]any         // deep copy for new sessions

// Session holds one active session and its private store copy.
type Session struct {
    ID         string
    CreatedAt  time.Time
    LastActive time.Time
    mu         sync.Mutex
    store      map[string]any  // private copy, seeded from GlobalStore.Snapshot()
}

func (s *Session) Get(key string) (any, bool)
func (s *Session) Set(key string, value any)
func (s *Session) Delete(key string)
func (s *Session) Has(key string) bool
func (s *Session) Keys() []string
func (s *Session) Snapshot() map[string]any  // read-only view for trace/inspector

// SessionManager creates, retrieves, and expires sessions.
type SessionManager struct {
    mu          sync.RWMutex
    sessions    map[string]*Session
    global      *GlobalStore
    cfg         SessionConfig
}

func (m *SessionManager) GetOrCreate(sessionID string) *Session
func (m *SessionManager) Get(sessionID string) (*Session, bool)
func (m *SessionManager) Invalidate(sessionID string)
func (m *SessionManager) InvalidateAll()
func (m *SessionManager) ActiveSessions() []*Session
func (m *SessionManager) startExpiryLoop(ctx context.Context)
```

### 7.2 Starlark Builtin Object

The `store` object exposed in Starlark is a custom `starlark.Value` implementation wrapping the session's `*Session`:

```go
// StoreBuiltin implements starlark.Value and starlark.HasAttrs.
// It proxies get/set/delete/has/keys to the underlying session store.
type StoreBuiltin struct {
    session *store.Session
}

// Called as: store.get("key"), store.set("key", val), etc.
// Each method is a starlark.Builtin registered as an attribute.
```

The `store` builtin is injected into the Starlark thread's predeclared values before `run(req)` is called. This does not require changes to `CompiledScript` — the builtin is thread-local state, not part of the compiled program.

### 7.3 Integration Point in Proxy Engine

```go
// In engine.go ServeHTTP, after route match:

// 1. Resolve or create session
sessionID := r.Header.Get(e.cfg.Session.HeaderName)
session   := e.storeManager.GetOrCreate(sessionID)

// 2. Run scripts with session store access
scriptOutput, scriptTraces := e.scriptEngine.RunBindings(
    r.Context(),
    matchedRoute.operation.ID,
    &scripting.ScriptInput{ ... },
    session,  // NEW parameter (nil in Phase 1)
)

// 3. Echo session ID in response if configured
if e.cfg.Session.EchoHeader {
    w.Header().Set(e.cfg.Session.HeaderName, session.ID)
}
```

The Phase 1 `RunBindings` signature adds `session *store.Session` as a new parameter (nil-safe — Phase 1 scripts that don't use `store` are unaffected).

---

## 8. Configuration

```yaml
# config.yaml additions for Phase 2
session:
  headerName:        "X-Virtual-Session-Id"  # Request/response header name
  inactivityTimeout: 30m                     # Idle TTL; supports Go duration strings
  echoHeader:        true                    # Write session ID into response header
  maxSessions:       10000                   # Hard cap; oldest session evicted when exceeded
```

---

## 9. Security Considerations

| Concern | Mitigation |
|---|---|
| Session fixation / hijacking | Session IDs are server-generated UUIDs (v4); caller-provided IDs are treated as lookup keys only — if not found, a new UUID is generated and returned |
| Memory exhaustion (sessions) | `maxSessions` cap; oldest-last-active session evicted when limit is reached |
| Memory exhaustion (store values) | Cap individual entry value size (1 MB); cap total global store size (10 MB) |
| Script poisoning global store | Scripts can only write to session copy; global store is read-only from scripts |
| Concurrent session writes | Per-session mutex serialises all `store.get`/`store.set` calls within a session |
| Session data leaking across requests | Sessions are keyed by ID in a private registry; no enumeration without admin access |
| Admin store exposure | Global store API is under `/_api` which should be access-controlled (same as all admin endpoints) |

---

## 10. Trace Integration

Phase 2 extends the existing script trace record with session information:

```json
"session": {
  "id":          "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "isNew":       false,
  "storeAccess": [
    { "op": "get", "key": "id-counter",    "value": 41 },
    { "op": "set", "key": "id-counter",    "value": 42 },
    { "op": "get", "key": "feature-flags", "value": { "darkMode": true } }
  ]
},
"scripts": [
  {
    "bindingId":  "abc123",
    "scriptId":   "def456",
    "scriptName": "ID Generator",
    "outputKey":  "ids",
    "durationMs": 0.41,
    "output":     { "nextId": "USR-0042", "callCount": 42 },
    "error":      null
  }
]
```

The `storeAccess` log is captured at script execution time and shows exactly which keys were read and written during that request, making session-state debugging straightforward in the Trace Viewer.

---

## 11. Implementation Checklist (v0.9.0)

1. `models.StoreEntry` struct
2. `config.yaml` + `Config` struct: `SessionConfig` section
3. `internal/store` package: `GlobalStore`, `Session`, `SessionManager`, `StoreBuiltin`
4. `data/store.json` initial file (empty entries object)
5. `storage.Storage` interface: no changes needed (store has its own persistence)
6. `internal/api/handler.go` — global store CRUD handlers, session list/detail/invalidate handlers
7. `internal/api/router.go` — register `/_api/store` and `/_api/sessions` routes
8. `internal/scripting/runner.go` — inject `store` builtin into Starlark thread when `session != nil`
9. `internal/scripting/engine.go` — accept `*store.Session` in `RunBindings`; pass to `Execute`
10. `internal/proxy/engine.go` — session resolution before script execution; echo header
11. `internal/models/trace.go` — add `Session *SessionTrace` to `Trace`
12. Admin UI — Store list page + Monaco JSON value editor
13. Admin UI — Sessions inspector page (list + detail panel)
14. Admin UI — Session info visible in Trace Viewer detail
15. Tests — unit: GlobalStore load/save, SessionManager create/expire, StoreBuiltin get/set/delete; integration: session header resolution, store access in script, isolation between sessions

---

## 12. Decisions

| # | Decision |
|---|---|
| Store scope | **Application-wide** — one global store shared across all specs and operations |
| Script write access | **Session-local only** — scripts cannot modify global store |
| Global store modification | **Admin UI / Admin API only** |
| Session identification | **`X-Virtual-Session-Id` header** — name configurable in `config.yaml` |
| Sessionless requests | **Ephemeral anonymous session** — seeded from global, discarded after response |
| Session expiry | **Inactivity-based** — configurable TTL, default 30 minutes |
| Persistence | **Global store only** — persisted to `data/store.json`; session copies are in-memory only |
| Store value types | **Any JSON type** — string, number, boolean, array, object |
| Phase 1 compatibility | **Nil-safe** — `RunBindings` session parameter is nil in Phase 1; scripts without `store` usage are unaffected |
