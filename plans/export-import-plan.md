# Archives — Server-Side Versioned Backup & Restore Plan

## Goal

Provide a dedicated **Archives** page in the Admin UI that lets users create
point-in-time snapshots of all application data, browse them, and restore any
snapshot back into the running server.

Archives are stored **server-side** in a managed directory (`data/archives/`).
Users never have to handle files manually — but they can download any archive to
their machine or upload one from outside.

> **Explicitly excluded from all archives**: `config.yaml`, TLS certificates,
> `data/certs/`, live sessions, and traces.

---

## 1. Archive Format

Each archive is a single ZIP file stored on the server:

```
data/archives/
├── 20260226-153000-a1b2c3d4.zip
├── 20260226-170000-e5f6a7b8.zip
└── ...
```

### ZIP layout (unchanged from previous revision)

```
<timestamp>-<shortID>.zip
├── manifest.json            # version, exportedAt, label, counts, checksums (SHA-256)
├── tags.json
├── store.json
├── specs/
│   ├── <specID>.json        # metadata only (Content="")
│   └── <specID>.yaml        # raw OpenAPI content
├── responses/
│   ├── <cfgID>.json         # metadata only (Body="")
│   └── <cfgID>.body
├── scripts/
│   ├── <scriptID>.json      # metadata only (Source="")
│   └── <scriptID>.star
└── operations/
    └── <opID>.scripts.json  # []ScriptBinding
```

### `manifest.json`

```json
{
  "id": "a1b2c3d4",
  "version": "1",
  "appVersion": "1.1.0",
  "label": "before-release-v2",
  "exportedAt": "2026-02-26T15:30:00Z",
  "counts": {
    "specs": 3,
    "responses": 12,
    "scripts": 2,
    "tags": 4,
    "storeEntries": 5
  },
  "checksums": {
    "tags.json":  "sha256:<hex>",
    "store.json": "sha256:<hex>",
    "specs/abc.json":  "sha256:<hex>",
    "specs/abc.yaml":  "sha256:<hex>",
    "..."
  }
}
```

`label` is an optional human-readable name the user can set when creating the
archive (defaults to the timestamp string).

---

## 2. Backend — Go

### 2.1 Package layout

```
internal/archive/
├── manifest.go      # Manifest struct, sha256Hex(), BuildManifest(), ParseManifest()
├── writer.go        # buildZIP() — assembles bytes.Buffer from Storage + GlobalStore
├── reader.go        # applyZIP() — upserts data from ZIP bytes; checksum verification
├── manager.go       # ArchiveManager — owns data/archives/ dir; CRUD + restore
├── manager_test.go
├── writer_test.go
└── reader_test.go
```

### 2.2 `ArchiveMeta` struct

```go
// ArchiveMeta is the lightweight summary stored in memory and returned by the list API.
type ArchiveMeta struct {
    ID         string    `json:"id"`          // short random hex (8 chars)
    Filename   string    `json:"filename"`    // basename of the ZIP on disk
    Label      string    `json:"label"`       // user-supplied or auto-generated
    CreatedAt  time.Time `json:"createdAt"`
    SizeBytes  int64     `json:"sizeBytes"`
    AppVersion string    `json:"appVersion"`
    Counts     Counts    `json:"counts"`
}
```

### 2.3 `ArchiveManager`

```go
type ArchiveManager struct {
    dir   string          // absolute path to data/archives/
    store storage.Storage
    gs    *store.GlobalStore
}

func NewArchiveManager(dir string, store storage.Storage, gs *store.GlobalStore) (*ArchiveManager, error)

// Create builds a new archive from current state and saves it to dir.
func (m *ArchiveManager) Create(label string) (*ArchiveMeta, error)

// List returns metadata for all archives, sorted newest-first.
func (m *ArchiveManager) List() ([]*ArchiveMeta, error)

// Get returns the metadata for a single archive.
func (m *ArchiveManager) Get(id string) (*ArchiveMeta, error)

// Delete removes the archive ZIP from disk.
func (m *ArchiveManager) Delete(id string) error

// FilePath returns the absolute path to the ZIP for streaming download.
func (m *ArchiveManager) FilePath(id string) (string, error)

// Save validates and stores an externally supplied ZIP (upload path).
func (m *ArchiveManager) Save(r io.Reader, size int64, label string) (*ArchiveMeta, error)

// Restore applies the archive identified by id to the live storage.
func (m *ArchiveManager) Restore(id string, opts RestoreOptions, engine *proxy.Engine) (*RestoreResult, error)
```

### 2.4 `RestoreOptions`

```go
type RestoreOptions struct {
    // CreateBackupFirst, when true, creates an archive of the current state
    // before overwriting anything. The resulting backup meta is included in
    // RestoreResult so the UI can show a link to it.
    CreateBackupFirst bool   `json:"createBackupFirst"`
    BackupLabel       string `json:"backupLabel"` // label for the auto-backup

    // WipeFirst deletes all data before applying the archive.
    // false = upsert (merge); true = full replacement.
    WipeFirst bool `json:"wipeFirst"`
}
```

### 2.5 `RestoreResult`

```go
type RestoreResult struct {
    BackupCreated *ArchiveMeta   `json:"backupCreated,omitempty"`
    Created       map[string]int `json:"created"`
    Updated       map[string]int `json:"updated"`
    WipedFirst    bool           `json:"wipedFirst"`
    Errors        []RestoreError `json:"errors,omitempty"`
}

type RestoreError struct {
    Path    string `json:"path"`
    Message string `json:"message"`
}
```

### 2.6 Checksum flow

**On `Create` / `Save`**: `sha256Hex(rawBytes)` is computed for every file
entry as it is written into the ZIP. All hashes are collected into
`manifest.checksums` and written last.

**On `Restore` / `Save`**: Before any data is modified, every file listed in
`manifest.checksums` is re-hashed and compared. If any mismatch is found,
the operation is aborted immediately with a `422` listing the corrupt paths.
No writes happen.

### 2.7 New API endpoints

All under `/_api/archives` — added to `router.go` inside the existing `/_api`
group.

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/_api/archives` | List archives (meta only) |
| `POST` | `/_api/archives` | Create archive of current state |
| `GET`  | `/_api/archives/:id` | Get single archive detail |
| `DELETE` | `/_api/archives/:id` | Delete archive |
| `GET`  | `/_api/archives/:id/download` | Stream ZIP as browser download |
| `POST` | `/_api/archives/upload` | Upload ZIP from browser → save to dir |
| `POST` | `/_api/archives/:id/restore` | Restore from archive |

**Create** (`POST /_api/archives`):
```json
Request:  { "label": "pre-release-v2" }
Response: ArchiveMeta
```

**Restore** (`POST /_api/archives/:id/restore`):
```json
Request:  { "createBackupFirst": true, "backupLabel": "auto-before-restore", "wipeFirst": true }
Response: RestoreResult
```

**Upload** (`POST /_api/archives/upload`):
```
multipart/form-data: archive=<file>, label=<string>
Response: ArchiveMeta
```

### 2.8 Wiring in `serve.go`

```go
archivesDir := filepath.Join(storePath, "archives")
archiveManager, err := archive.NewArchiveManager(archivesDir, store, globalStore)
// ...
handler.SetArchiveManager(archiveManager)
```

---

## 3. Frontend — React / TypeScript

### 3.1 New types (`types/index.ts`)

```typescript
export interface ArchiveMeta {
  id: string;
  filename: string;
  label: string;
  createdAt: string;
  sizeBytes: number;
  appVersion: string;
  counts: { specs: number; responses: number; scripts: number; tags: number; storeEntries: number };
}

export interface RestoreResult {
  backupCreated?: ArchiveMeta;
  created: Record<string, number>;
  updated: Record<string, number>;
  wipedFirst: boolean;
  errors?: Array<{ path: string; message: string }>;
}
```

### 3.2 API service (`services/api.ts`)

```typescript
export const archivesApi = {
  list: () => apiFetch<ArchiveMeta[]>('/_api/archives'),
  get:  (id: string) => apiFetch<ArchiveMeta>(`/_api/archives/${id}`),

  create: (label: string) =>
    apiFetch<ArchiveMeta>('/_api/archives', { method: 'POST', body: { label } }),

  delete: (id: string) =>
    apiFetch<void>(`/_api/archives/${id}`, { method: 'DELETE' }),

  download: (id: string, filename: string): void => {
    const a = document.createElement('a');
    a.href = `/_api/archives/${id}/download`;
    a.download = filename;
    a.click();
  },

  upload: (file: File, label: string) => {
    const form = new FormData();
    form.append('archive', file);
    form.append('label', label);
    return apiFetch<ArchiveMeta>('/_api/archives/upload', { method: 'POST', body: form });
  },

  restore: (id: string, opts: { createBackupFirst: boolean; backupLabel: string; wipeFirst: boolean }) =>
    apiFetch<RestoreResult>(`/_api/archives/${id}/restore`, { method: 'POST', body: opts }),
};
```

### 3.3 New page: `components/ArchiveManager/`

```
ArchiveManager/
├── ArchiveManager.tsx      # page root — header, create button, upload button, list
├── ArchiveList.tsx          # table of ArchiveMeta rows
├── ArchiveRow.tsx           # single row: label, date, size, counts, action buttons
├── CreateArchiveModal.tsx   # label field + create button
├── UploadArchiveModal.tsx   # file picker + label field + upload button
└── RestoreModal.tsx         # multi-step restore dialog (see below)
```

#### `ArchiveList` table columns

| Column | Content |
|--------|---------|
| Label | user label (editable inline or via modal) |
| Created | relative time (e.g. "2 hours ago") |
| Size | human-readable bytes |
| Counts | "3 specs · 12 responses · 2 scripts" |
| App version | version string from manifest |
| Actions | Download · Restore · Delete |

#### `RestoreModal` — two-step flow

**Step 1 — Safety backup**
- "Create a backup of current state before restoring?" (default **Yes**)
- Label field pre-filled with `"auto-before-restore-<timestamp>"`
- User can change to No

**Step 2 — Restore mode**
- Radio: **Merge** (upsert, keep existing records not in archive) vs
  **Wipe & Restore** (delete all data first)
- `Wipe & Restore` shows red destructive banner

**Confirm** → calls `archivesApi.restore()` → shows `RestoreResult` summary:
- "Backup created: `<label>`" (link to new backup if created)
- Created / Updated counts table
- Error list if any

### 3.4 Route & navigation

- Route: `/archives` → `<ArchiveManager />` in `App.tsx`
- Nav item: `{ to: '/archives', icon: Archive, label: 'Archives' }` in
  `Layout.tsx` (between Sessions and Tags)

---

## 4. Data Safety

| Risk | Mitigation |
|------|-----------|
| Corrupt / tampered archive | SHA-256 per file in `manifest.checksums`; mismatch → `422`, zero writes. |
| Accidental data loss on restore | `createBackupFirst` (default **true**) snapshots current state first; user explicitly opts out. |
| Accidental full wipe | `wipeFirst` defaults to **false** (Merge mode); Wipe requires explicit selection + red banner. |
| Unknown archive version | `manifest.version` check; reject with `409` if unsupported. |
| config.yaml / certs leaking | Never written to archive; import/restore never touches those paths. |
| Large archives | Max 50 MB on upload/restore; `413` if exceeded. |
| Disk space on server | Archive list shows sizes; user can delete old archives. |
| Sessions & traces | Runtime-only; never archived, never wiped by restore. |

---

## 5. Implementation Order

| Step | Task | Files |
|------|------|-------|
| 1 | `archive/manifest.go` — Manifest, Counts, sha256Hex | `internal/archive/manifest.go` |
| 2 | `archive/writer.go` — buildZIP() | `internal/archive/writer.go` |
| 3 | `archive/writer_test.go` | `internal/archive/writer_test.go` |
| 4 | `archive/reader.go` — applyZIP(), checksum verification, wipe logic | `internal/archive/reader.go` |
| 5 | `archive/reader_test.go` — round-trip, wipe, checksum-fail | `internal/archive/reader_test.go` |
| 6 | `archive/manager.go` — ArchiveManager (Create, List, Get, Delete, FilePath, Save, Restore) | `internal/archive/manager.go` |
| 7 | `archive/manager_test.go` | `internal/archive/manager_test.go` |
| 8 | Wire handler methods + routes | `internal/api/handler.go`, `internal/api/router.go` |
| 9 | Wire ArchiveManager in serve.go | `cmd/server/serve.go` |
| 10 | TypeScript types | `ui/src/types/index.ts` |
| 11 | API service | `ui/src/services/api.ts` |
| 12 | `ArchiveManager` UI components | `ui/src/components/ArchiveManager/` |
| 13 | Route + nav | `App.tsx`, `Layout.tsx` |
| 14 | Build + full test suite | `make build` |

---

## 6. Out of Scope

- Scheduled / automatic archiving — can be added later.
- Remote archive storage (S3, GCS) — future enhancement.
- Selective restore (individual specs) — can be added later.
- Live traces / statistics — ephemeral.
- `config.yaml` and TLS certificates — server operator concerns.
- Encryption of archives.
