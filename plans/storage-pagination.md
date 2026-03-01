# Plan: Storage Pagination

**Priority:** P2  
**Effort:** L (3–4 days)  
**Target release:** v1.4.0  
**Related:** [improvement-roadmap.md § 2.1](improvement-roadmap.md)

---

## Problem

The `Storage` interface has no pagination:

```go
GetAllSpecs() ([]*models.Spec, error)
GetAllOperations() ([]*models.Operation, error)
GetAllScripts() ([]*models.Script, error)
```

Every call loads the entire dataset into memory. With 100+ specs (each containing the full OpenAPI YAML in `content`) and thousands of operations this will:

1. Consume large amounts of RAM on every admin list request.
2. Cause the admin UI to render very large tables without scrolling.
3. Make the `LIST` endpoints slow as the dataset grows.

---

## Goal

1. Add `Page` + `PagedResult` types to the storage package.
2. Add **paginated variants** of the three high-cardinality list methods.
3. Keep the existing `GetAll*` methods for internal use (e.g. proxy engine route reload, archive writer) — they do not need pagination.
4. Update HTTP list endpoints to accept `?offset=0&limit=50` query parameters.
5. Update the admin UI list components to use cursor/offset pagination.

---

## New Types

```go
// package storage

// Page specifies an offset-based pagination window.
// Limit = 0 means "return all" (backwards-compatible default).
type Page struct {
    Offset int
    Limit  int
}

// PagedResult wraps a slice of results with pagination metadata.
type PagedResult[T any] struct {
    Items  []T `json:"items"`
    Total  int `json:"total"`   // total matching records (ignoring pagination)
    Offset int `json:"offset"`
    Limit  int `json:"limit"`
}
```

---

## Storage Interface Changes

Add three new methods (the old `GetAllSpecs` etc. are **not removed**):

```go
type Storage interface {
    // ... existing methods unchanged ...

    // Paginated list methods (new)
    ListSpecs(page Page) (*PagedResult[*models.Spec], error)
    ListScripts(page Page) (*PagedResult[*models.Script], error)
    ListOperationsBySpec(specID string, page Page) (*PagedResult[*models.Operation], error)
}
```

### Why keep `GetAll*`?

- `proxy.Engine.ReloadRoutes()` iterates every enabled spec and all their operations — needs the full set.
- `archive.Writer` serialises all specs, operations, responses, scripts — needs the full set.
- Adding pagination there adds complexity with no benefit.

---

## Implementation — `FileStorage`

```go
func (s *FileStorage) ListSpecs(page Page) (*PagedResult[*models.Spec], error) {
    all, err := s.GetAllSpecs() // existing method
    if err != nil {
        return nil, err
    }
    total := len(all)
    if page.Limit <= 0 {
        return &PagedResult[*models.Spec]{Items: all, Total: total, Offset: 0, Limit: total}, nil
    }
    start := page.Offset
    if start >= total {
        return &PagedResult[*models.Spec]{Items: []*models.Spec{}, Total: total, Offset: page.Offset, Limit: page.Limit}, nil
    }
    end := start + page.Limit
    if end > total {
        end = total
    }
    return &PagedResult[*models.Spec]{
        Items:  all[start:end],
        Total:  total,
        Offset: page.Offset,
        Limit:  page.Limit,
    }, nil
}
```

Identical pattern for `ListScripts` and `ListOperationsBySpec`.

---

## HTTP API Changes

### List Specs: `GET /_api/specs`

**Query params (new, optional):**
| Param | Type | Default | Description |
|---|---|---|---|
| `offset` | int | 0 | Skip this many records |
| `limit` | int | 0 | Max records to return; 0 = all |
| `q` | string | "" | Filter by name (case-insensitive substring) |
| `enabled` | bool | — | Filter enabled/disabled specs |

**Response (new envelope):**
```json
{
  "items": [ { "id": "...", "name": "..." } ],
  "total": 42,
  "offset": 0,
  "limit": 20
}
```

**Backwards compatibility:** When `limit=0` (default), `total` = `len(items)`. The `items` field was previously the top-level array. To avoid breaking existing clients, a **response shape migration** is needed. Options:

1. **Preferred:** Use the new envelope shape unconditionally. Document as a breaking change in v1.4.0 release notes. Existing clients using the array directly will break.
2. **Conservative:** Support both via an `Accept` header or `envelope=true` query param.

Recommendation: go with option 1 and call it out in the changelog. The admin UI is the primary client; external integrations are unlikely to rely on the current shape.

### List Scripts: `GET /_api/scripts`

Same query params and envelope response shape.

### List Operations: `GET /_api/specs/:id/operations`

Same pattern.

---

## Admin UI Changes

### Spec list (`SpecManager`)

Replace the current "load all at once" pattern with:
- A `useInfiniteQuery` (React Query) or a simple page control.
- A search box that hits `?q=...&limit=20`.
- "Load more" button or page number controls.

```tsx
// Current
const { data: specs } = useQuery(['specs'], api.listSpecs)

// New
const [page, setPage] = useState({ offset: 0, limit: 20 })
const { data } = useQuery(
  ['specs', page],
  () => api.listSpecs(page),
)
// data.items, data.total, data.offset, data.limit
```

### Script list (`ScriptManager`)

Same pattern as spec list.

---

## Config

No new config keys needed. The pagination defaults (limit=0 = all) are safe for any deployment size.

---

## Test Plan

### Unit tests — `internal/storage/`

Add to `file_test.go` and `memory_test.go`:
```go
TestListSpecs_FirstPage          // offset=0, limit=2 → 2 items
TestListSpecs_SecondPage         // offset=2, limit=2 → next 2
TestListSpecs_BeyondEnd          // offset > total → empty items slice, correct total
TestListSpecs_ZeroLimit          // limit=0 → all items
TestListSpecs_Empty              // no specs → total=0
TestListScripts_Pagination       // similar cases
TestListOperationsBySpec_Pagination
```

### HTTP handler tests

Add to `handler_specs_test.go` (after split):
```go
TestListSpecs_DefaultPagination  // no params → all returned in envelope
TestListSpecs_WithLimit          // limit=2 → 2 items
TestListSpecs_WithOffset         // offset=1, limit=1 → second item
TestListSpecs_FilterByName       // q=pet → only matching specs
```

---

## Acceptance Criteria

- [ ] `Page` and `PagedResult[T]` types defined in `internal/storage/`
- [ ] `ListSpecs(Page)`, `ListScripts(Page)`, `ListOperationsBySpec(specID, Page)` added to interface and both implementations
- [ ] Existing `GetAllSpecs()`, `GetAllOperations()`, `GetAllScripts()` unchanged and used internally
- [ ] `GET /_api/specs` returns paginated envelope; `offset` and `limit` params work
- [ ] `GET /_api/scripts` returns paginated envelope
- [ ] Admin UI spec list uses paginated endpoint
- [ ] All existing tests pass
- [ ] New pagination tests added (storage + handler levels)
