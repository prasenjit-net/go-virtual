# Starlark Scripting Builtins — Roadmap

Proposals for additional built-in functions to enrich the Starlark scripting
environment in go-virtual.  Each entry follows the same pattern as the existing
`log()` and `store` builtins — injected at runtime as predeclared names, always
available without imports, subject to the script execution timeout.

---

## Currently Available

| Builtin | Signature | Description |
|---------|-----------|-------------|
| `log` | `log(msg, ...)` | Append formatted message to the script trace log |
| `store` | object | Session-scoped key-value store (`get`, `set`, `has`, `delete`, `keys`) |

---

## Proposed Builtins

### 1. `counter` — Persistent atomic counter

```python
counter("page_views")           # → current value (int), starts at 0
counter("page_views", 1)        # → new value after incrementing by 1
counter("page_views", -1)       # → new value after decrementing
counter("retries", 0)           # → read without mutation (same as no-delta form)
```

**Why useful**: Simulate nth-call behaviour (e.g. fail on the 3rd call), round-robin
between responses, track how many times an endpoint has been hit per session, implement
simple rate-limit mock logic.

**Implementation**: Backed by the session store using a reserved key prefix (`__counter__:<name>`).
Atomic within a session; no cross-session sharing (use GlobalStore for that).

---

### 2. `uuid` — Generate a UUID v4

```python
uuid()   # → "f47ac10b-58cc-4372-a567-0e02b2c3d479"
```

**Why useful**: Assign unique IDs to created resources in mock responses without
relying on template variables.  Especially handy when the ID must also be stored in
`store` for later lookups.

**Implementation**: `github.com/google/uuid` — already a dependency.

---

### 3. `now` — Current timestamp

```python
now()             # → Unix timestamp (int, seconds)
now("unix_ms")    # → Unix timestamp in milliseconds
now("iso")        # → "2026-02-23T14:05:00Z"
now("date")       # → "2026-02-23"
```

**Why useful**: Inject dynamic timestamps into response bodies or store values with
time-based logic (e.g. set an expiry field, compute "created 5 minutes ago").

**Implementation**: `time.Now()` formatted per the format argument.

---

### 4. `rand_int` — Random integer in range

```python
rand_int(1, 100)    # → random int in [1, 100] inclusive
rand_int(100)       # → random int in [0, 100]
```

**Why useful**: Simulate variable quantities (stock levels, latency values, scores),
randomise responses for chaos / load testing scenarios.

**Implementation**: `math/rand` — already used by the template engine.

---

### 5. `rand_choice` — Pick a random element from a list

```python
rand_choice(["pending", "active", "suspended"])   # → one of the three strings
rand_choice([200, 200, 200, 429])                 # → weighted: 429 ~25% of the time
```

**Why useful**: Randomly select a status, error code, or any value from a fixed set.
Weighted distribution is achieved naturally by repeating values.

**Implementation**: `rand.Intn(len(list))` index pick.

---

### 6. `base64_encode` / `base64_decode`

```python
base64_encode("hello world")           # → "aGVsbG8gd29ybGQ="
base64_decode("aGVsbG8gd29ybGQ=")     # → "hello world"
```

**Why useful**: Build mock JWT tokens or other base64-encoded payloads, decode
incoming base64 request body fields for inspection.

**Implementation**: `encoding/base64` standard library.

---

### 7. `hash` — Cryptographic / non-cryptographic hashing

```python
hash("sha256", "secret" + req.body["userId"])   # → hex string
hash("md5",    req.query["email"])
```

Supported algorithms: `md5`, `sha1`, `sha256`, `sha512`.

**Why useful**: Generate deterministic IDs from request data (e.g. consistent fake
user IDs keyed by email), verify mock HMAC signatures, anonymise values in logs.

**Implementation**: `crypto/md5`, `crypto/sha256`, etc. — standard library.

---

### 8. `json_parse` / `json_stringify`

```python
obj = json_parse('{"id": 1, "name": "Alice"}')   # → Starlark dict
s   = json_stringify({"status": "ok"})            # → '{"status":"ok"}'
```

**Why useful**: The request body arrives as a raw string; `json_parse` lets scripts
work with nested fields without relying on gjson path strings.  `json_stringify` lets
scripts build structured response parts programmatically.

**Implementation**: `encoding/json` + existing `StarToGo` / `GoToStar` converters.

---

### 9. `sleep` — Artificial latency

```python
sleep(50)    # pause for 50 ms (capped at the script timeout)
```

**Why useful**: Simulate slow backends, test client timeout handling, introduce
variable latency in chaos scenarios.

**Implementation**: `time.Sleep` inside the Starlark call, interrupted by the
existing timeout goroutine.  Cap at `timeoutMs - 10` to leave headroom.

---

### 10. `regex_match` / `regex_find`

```python
regex_match(r"^\d{4}-\d{2}-\d{2}$", req.query["date"])  # → True / False
regex_find(r"\d+", req.body["description"])              # → first match or None
regex_find_all(r"\d+", req.body["text"])                 # → list of matches
```

**Why useful**: Scripts can apply richer validation or extraction logic than the
condition engine (which already supports `regex` operator, but only for conditions).
Useful for parsing structured strings from request bodies.

**Implementation**: `regexp` standard library; compile and cache patterns.

---

## Priority Ranking

| Priority | Builtin | Effort | Impact |
|----------|---------|--------|--------|
| ⭐⭐⭐ | `uuid` | Low | High — needed for create-resource mocks |
| ⭐⭐⭐ | `counter` | Low | High — enables stateful call-count logic |
| ⭐⭐⭐ | `now` | Low | High — timestamps in responses / store |
| ⭐⭐ | `rand_int` / `rand_choice` | Low | Medium — chaos / fuzzing |
| ⭐⭐ | `json_parse` / `json_stringify` | Medium | Medium — richer body handling |
| ⭐⭐ | `base64_encode` / `base64_decode` | Low | Medium — auth token mocking |
| ⭐ | `hash` | Low | Low-medium — deterministic IDs |
| ⭐ | `sleep` | Low | Low-medium — latency simulation |
| ⭐ | `regex_match` / `regex_find` | Medium | Low-medium — body parsing |

---

## Implementation Notes

- All builtins are injected in `starlarkScript.Execute` alongside the existing
  `store` and `log` entries in the `predeclared` dict.
- Compile-time: add each new name to the `isPredeclared` predicate in `Compile` so
  the compiler does not flag them as undefined.
- Each builtin should log any notable action to `logBuf` (where appropriate) to keep
  the trace informative.
