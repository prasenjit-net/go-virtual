# Plan: Persistent Trace Storage

**Priority:** P3  
**Effort:** L (3–5 days)  
**Target release:** v2.0.0  
**Related:** [improvement-roadmap.md § 4.1](improvement-roadmap.md)

---

## Problem

All traces are stored in an **in-memory ring buffer** inside `tracing.Service`:

```go
type Service struct {
    mu          sync.RWMutex
    traces      []*models.Trace   // ring buffer
    maxTraces   int
    subscribers map[string]*subscriber
}
```

Issues:
1. **Restart = total trace loss.** Any debugging session is wiped on every deploy.
2. **Ring buffer evicts oldest traces silently.** At 100 req/s and `maxTraces=1000`, traces are 10 seconds old before eviction starts. At 1000 req/s, the buffer fills in 1 second.
3. **`TracingConfig.Retention` (default 24h) is defined in config but wired nowhere.** It is parsed but never consulted.
4. **No full-text search or filtering on the stored traces** (filtering is done in-memory by iterating all records).

---

## Goal

1. Introduce a `TraceStore` interface that the `Service` delegates persistence to.
2. Provide two implementations: `MemoryTraceStore` (current behaviour, default) and `SQLiteTraceStore` (opt-in via config).
3. Wire the `TracingConfig.Retention` field so old traces are evicted by age.
4. Preserve the existing WebSocket live-stream behaviour regardless of storage backend.
5. No breaking changes to the `tracing.Service` public API.

---

## Interface

### New file: `internal/tracing/store.go`

```go
package tracing

import "github.com/prasenjit/go-virtual/internal/models"

// TraceStore persists and queries traces.
type TraceStore interface {
    // Append stores a new trace record.
    Append(trace *models.Trace) error

    // Query returns traces matching the filter.
    Query(filter *models.TraceFilter) ([]*models.Trace, error)

    // Count returns the total number of traces matching the filter (ignoring Limit/Offset).
    Count(filter *models.TraceFilter) (int, error)

    // Delete removes traces matching the filter.
    // Pass an empty/nil filter to clear all.
    Delete(filter *models.TraceFilter) error

    // Close releases any resources (e.g. database connections).
    Close() error
}
```

---

## Memory Implementation

### New file: `internal/tracing/store_memory.go`

Extracts the current ring-buffer logic out of `Service` into `MemoryTraceStore`:

```go
type MemoryTraceStore struct {
    mu        sync.RWMutex
    traces    []*models.Trace
    maxTraces int
    retention time.Duration
}

func NewMemoryTraceStore(maxTraces int, retention time.Duration) *MemoryTraceStore
func (s *MemoryTraceStore) Append(trace *models.Trace) error
func (s *MemoryTraceStore) Query(filter *models.TraceFilter) ([]*models.Trace, error)
func (s *MemoryTraceStore) Count(filter *models.TraceFilter) (int, error)
func (s *MemoryTraceStore) Delete(filter *models.TraceFilter) error
func (s *MemoryTraceStore) Close() error
```

`Append` evicts by retention (sweep from front) **and** by count (trim tail), whichever applies.

---

## SQLite Implementation

### New file: `internal/tracing/store_sqlite.go`

```go
type SQLiteTraceStore struct {
    db *sql.DB
}

func NewSQLiteTraceStore(path string, maxTraces int, retention time.Duration) (*SQLiteTraceStore, error)
```

#### Schema

```sql
CREATE TABLE IF NOT EXISTS traces (
    id          TEXT PRIMARY KEY,
    spec_id     TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    timestamp   DATETIME NOT NULL,
    duration_ns  INTEGER NOT NULL,
    status_code  INTEGER NOT NULL,
    proxy_mode   BOOLEAN NOT NULL DEFAULT 0,
    data         TEXT NOT NULL    -- full Trace JSON blob
);

CREATE INDEX IF NOT EXISTS idx_traces_spec     ON traces(spec_id);
CREATE INDEX IF NOT EXISTS idx_traces_ts       ON traces(timestamp);
CREATE INDEX IF NOT EXISTS idx_traces_op       ON traces(operation_id);
```

The `data` column stores the full serialised `models.Trace` JSON so the query layer can deserialise it without additional joins.

#### Retention cleanup

A background goroutine runs every 5 minutes:
```go
DELETE FROM traces WHERE timestamp < datetime('now', '-' || ? || ' hours');
```

And a count-based cap:
```go
DELETE FROM traces WHERE id NOT IN (
    SELECT id FROM traces ORDER BY timestamp DESC LIMIT ?
);
```

#### Dependency

Add `modernc.org/sqlite` (pure-Go SQLite, no CGO required):
```
go get modernc.org/sqlite
```

---

## `Service` Changes

`Service` delegates to `TraceStore`:

```go
type Service struct {
    store       TraceStore         // injected
    subscribers map[string]*subscriber
    mu          sync.RWMutex
}

func NewService(store TraceStore) *Service
```

`RecordTrace` calls `s.store.Append(trace)` then notifies subscribers (the live-stream path is unchanged).

`GetTraces` delegates to `s.store.Query(filter)`.

`ClearTraces`/`ClearTracesBySpec` delegate to `s.store.Delete(filter)`.

---

## Config Changes

```yaml
# config.yaml
tracing:
  storage: "memory"        # "memory" (default) | "sqlite"
  sqlitePath: "./data/traces.db"
  maxTraces: 10000          # max records before eviction
  retention: 72h            # evict records older than this
```

`serve.go` selects the implementation:

```go
var traceStore tracing.TraceStore
switch cfg.Tracing.Storage {
case "sqlite":
    traceStore, err = tracing.NewSQLiteTraceStore(
        cfg.Tracing.SQLitePath,
        cfg.Tracing.MaxTraces,
        cfg.Tracing.Retention,
    )
    if err != nil { log.Fatal(err) }
default:
    traceStore = tracing.NewMemoryTraceStore(
        cfg.Tracing.MaxTraces,
        cfg.Tracing.Retention,
    )
}
tracingService := tracing.NewService(traceStore)
```

---

## API Changes

### `GET /_api/traces` — add pagination

The trace list endpoint currently has `Limit` and `Offset` in `TraceFilter` but the HTTP handler ignores them. Wire them to query params:

```
GET /_api/traces?specId=abc&limit=50&offset=0
```

Response:
```json
{
  "items": [ { "id": "...", ... } ],
  "total": 412,
  "offset": 0,
  "limit": 50
}
```

The SQLite backend can compute `total` with a `COUNT(*)` query using the same filter (minus limit/offset), making it efficient.

---

## Admin UI Changes

Trace viewer gets a **pagination toolbar**:
- "Older" / "Newer" buttons.
- Page counter: "Showing 1–50 of 412".
- Filter by spec, status code, and time range (already partially implemented).

---

## Test Plan

### Unit tests

#### `store_memory_test.go`
```go
TestMemoryTraceStore_Append
TestMemoryTraceStore_Query_BySpecID
TestMemoryTraceStore_Query_ByStatusCode
TestMemoryTraceStore_Query_ByTimeRange
TestMemoryTraceStore_Count
TestMemoryTraceStore_Delete_BySpecID
TestMemoryTraceStore_Delete_All
TestMemoryTraceStore_EvictByCount
TestMemoryTraceStore_EvictByRetention
```

#### `store_sqlite_test.go` (using temp file / `:memory:`)
```go
TestSQLiteTraceStore_AppendAndQuery
TestSQLiteTraceStore_Count
TestSQLiteTraceStore_Delete
TestSQLiteTraceStore_RetentionCleanup
TestSQLiteTraceStore_CountCap
```

---

## Migration from Existing Service

`tracing.NewService(maxTraces int)` (current signature) is changed to `tracing.NewService(store TraceStore)`. All call sites in `serve.go` and tests must be updated.

---

## Acceptance Criteria

- [ ] `TraceStore` interface defined
- [ ] `MemoryTraceStore` implements `TraceStore`; existing behaviour preserved
- [ ] `SQLiteTraceStore` implements `TraceStore`; `modernc.org/sqlite` added to go.mod
- [ ] `TracingConfig.Retention` is wired and enforced in both implementations
- [ ] `TracingConfig.Storage` selects implementation at startup
- [ ] Live WebSocket stream (`/_api/traces/stream`) works with both backends
- [ ] `GET /_api/traces` supports `limit` + `offset` pagination
- [ ] Admin UI trace list shows pagination controls
- [ ] All existing tests pass
- [ ] New TraceStore tests added (memory + sqlite)
- [ ] `tracing.NewService` signature updated and all call sites updated
