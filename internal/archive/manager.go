package archive

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/prasenjit/go-virtual/internal/logging"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
)

const (
	indexFile   = "index.json"
	maxUploadMB = 50
)

// ArchiveMeta holds the lightweight metadata for one stored archive.
type ArchiveMeta struct {
	ID         string    `json:"id"`
	Filename   string    `json:"filename"`
	Label      string    `json:"label"`
	CreatedAt  time.Time `json:"createdAt"`
	SizeBytes  int64     `json:"sizeBytes"`
	AppVersion string    `json:"appVersion"`
	Counts     Counts    `json:"counts"`
}

// RestoreInput is the request body accepted by the restore API endpoint.
type RestoreInput struct {
	CreateBackupFirst bool   `json:"createBackupFirst"`
	BackupLabel       string `json:"backupLabel"`
	WipeFirst         bool   `json:"wipeFirst"`
}

// RestoreResponse is what the restore endpoint returns.
type RestoreResponse struct {
	BackupCreated *ArchiveMeta   `json:"backupCreated,omitempty"`
	Result        *RestoreResult `json:"result"`
}

// ArchiveManager owns the archives directory and provides CRUD + restore.
// It implements ArchiveService in full mode.
type ArchiveManager struct {
	dir   string
	stor  storage.Storage
	gs    store.GlobalStoreBackend
	cb    store.CollectionBackend
	mu    sync.Mutex
	index []*ArchiveMeta
}

// NewArchiveManager creates (or opens) the archives directory and loads the index.
// cb may be nil (collections are skipped in exports/imports).
func NewArchiveManager(dir string, stor storage.Storage, gs store.GlobalStoreBackend, cb store.CollectionBackend) (*ArchiveManager, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("archive: create dir %s: %w", dir, err)
	}

	m := &ArchiveManager{dir: dir, stor: stor, gs: gs, cb: cb}
	if err := m.loadIndex(); err != nil {
		return nil, err
	}
	return m, nil
}

// ── Index management ────────────────────────────────────────────────────────

func (m *ArchiveManager) indexPath() string {
	return filepath.Join(m.dir, indexFile)
}

func (m *ArchiveManager) loadIndex() error {
	data, err := os.ReadFile(m.indexPath())
	if os.IsNotExist(err) {
		// Rebuild index by scanning existing ZIPs.
		return m.rebuildIndex()
	}
	if err != nil {
		return fmt.Errorf("archive: read index: %w", err)
	}
	if err := json.Unmarshal(data, &m.index); err != nil {
		// Corrupt index — rebuild from ZIPs.
		return m.rebuildIndex()
	}
	return nil
}

func (m *ArchiveManager) rebuildIndex() error {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return fmt.Errorf("archive: read dir: %w", err)
	}
	m.index = nil
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".zip" {
			continue
		}
		path := filepath.Join(m.dir, e.Name())
		meta, err := metaFromZIPFile(path)
		if err != nil {
			continue // skip corrupt archives
		}
		m.index = append(m.index, meta)
	}
	sortIndex(m.index)
	return m.saveIndex()
}

func (m *ArchiveManager) saveIndex() error {
	data, err := json.MarshalIndent(m.index, "", "  ")
	if err != nil {
		return fmt.Errorf("archive: marshal index: %w", err)
	}
	return os.WriteFile(m.indexPath(), data, 0o644)
}

func sortIndex(idx []*ArchiveMeta) {
	sort.Slice(idx, func(i, j int) bool {
		return idx[i].CreatedAt.After(idx[j].CreatedAt)
	})
}

// ── Public API ──────────────────────────────────────────────────────────────

// Create builds a new archive from the current live state and saves it to disk.
func (m *ArchiveManager) Create(label string) (*ArchiveMeta, error) {
	zipBytes, manifest, err := BuildZIP(label, m.stor, m.gs, m.cb)
	if err != nil {
		return nil, err
	}
	return m.save(manifest, zipBytes)
}

// List returns all archive metadata sorted newest-first.
func (m *ArchiveManager) List() []*ArchiveMeta {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*ArchiveMeta, len(m.index))
	copy(out, m.index)
	return out
}

// Get returns the metadata for a single archive by ID.
func (m *ArchiveManager) Get(id string) (*ArchiveMeta, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.index {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, fmt.Errorf("archive: not found: %s", id)
}

// Delete removes the ZIP file and updates the index.
func (m *ArchiveManager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var found *ArchiveMeta
	remaining := m.index[:0]
	for _, a := range m.index {
		if a.ID == id {
			found = a
		} else {
			remaining = append(remaining, a)
		}
	}
	if found == nil {
		return fmt.Errorf("archive: not found: %s", id)
	}

	if err := os.Remove(filepath.Join(m.dir, found.Filename)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("archive: delete file: %w", err)
	}
	m.index = remaining
	return m.saveIndex()
}

// FilePath returns the absolute filesystem path to the ZIP for a given ID.
func (m *ArchiveManager) FilePath(id string) (string, error) {
	meta, err := m.Get(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(m.dir, meta.Filename), nil
}

// Save validates and stores an externally supplied ZIP (e.g., from an upload).
func (m *ArchiveManager) Save(r io.Reader, size int64, label string) (*ArchiveMeta, error) {
	const maxBytes = maxUploadMB << 20
	if size > maxBytes {
		return nil, fmt.Errorf("archive: upload too large (max %d MB)", maxUploadMB)
	}

	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("archive: read upload: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("archive: upload too large (max %d MB)", maxUploadMB)
	}

	// Validate: parse manifest (checksum verification is done at restore time).
	manifest, err := readManifestFromZIP(data)
	if err != nil {
		return nil, fmt.Errorf("archive: invalid zip: %w", err)
	}
	if label != "" {
		manifest.Label = label
	}

	return m.save(manifest, data)
}

// Restore applies the archive identified by id to the live storage.
// If CreateBackupFirst is set, a snapshot of the current state is created
// first and its metadata is included in the response.
func (m *ArchiveManager) Restore(id string, input RestoreInput) (*RestoreResponse, error) {
	logger := logging.Logger("archive.restore").With(
		"archive_id", id,
		"create_backup_first", input.CreateBackupFirst,
		"wipe_first", input.WipeFirst,
	)
	logger.Info("Starting archive restore", "event", "archive_restore_started")
	var backupMeta *ArchiveMeta

	if input.CreateBackupFirst {
		backupLabel := input.BackupLabel
		if backupLabel == "" {
			backupLabel = "auto-before-restore-" + time.Now().UTC().Format("20060102-150405")
		}
		bm, err := m.Create(backupLabel)
		if err != nil {
			logger.Error("Failed to create backup before restore",
				"event", "archive_restore_backup_failed",
				"backup_label", backupLabel,
				"error", err,
			)
			return nil, fmt.Errorf("archive: pre-restore backup failed: %w", err)
		}
		backupMeta = bm
		logger.Info("Created backup before restore",
			"event", "archive_restore_backup_created",
			"backup_archive_id", bm.ID,
			"backup_label", bm.Label,
		)
	}

	// Read the ZIP from disk.
	path, err := m.FilePath(id)
	if err != nil {
		logger.Error("Failed to resolve archive path for restore", "event", "archive_restore_path_failed", "error", err)
		return nil, err
	}
	zipBytes, err := os.ReadFile(path)
	if err != nil {
		logger.Error("Failed to read archive zip for restore",
			"event", "archive_restore_read_failed",
			"path", path,
			"error", err,
		)
		return nil, fmt.Errorf("archive: read zip for restore: %w", err)
	}

	result, err := ApplyZIP(zipBytes, RestoreOptions{WipeFirst: input.WipeFirst}, m.stor, m.gs, m.cb)
	if err != nil {
		logger.Error("Archive restore failed",
			"event", "archive_restore_failed",
			"path", path,
			"result_errors", len(result.Errors),
			"error", err,
		)
		return &RestoreResponse{BackupCreated: backupMeta, Result: result}, err
	}
	logger.Info("Archive restore completed",
		"event", "archive_restore_completed",
		"path", path,
		"created_specs", result.Created["specs"],
		"updated_specs", result.Updated["specs"],
		"created_responses", result.Created["responses"],
		"updated_responses", result.Updated["responses"],
		"result_errors", len(result.Errors),
	)

	return &RestoreResponse{BackupCreated: backupMeta, Result: result}, nil
}

// ── ArchiveService implementation ────────────────────────────────────────────

// Mode returns ModeFull — ArchiveManager supports full archive history.
func (m *ArchiveManager) Mode() ArchiveMode { return ModeFull }

// DownloadSnapshot is not supported in full mode.
func (m *ArchiveManager) DownloadSnapshot() ([]byte, *ArchiveMeta, error) {
	return nil, nil, ErrFullMode
}

// RestoreSnapshot is not supported in full mode.
func (m *ArchiveManager) RestoreSnapshot(_ []byte) (*RestoreResponse, error) {
	return nil, ErrFullMode
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// save writes zipBytes to disk and updates the in-memory index.
func (m *ArchiveManager) save(manifest *Manifest, zipBytes []byte) (*ArchiveMeta, error) {
	filename := manifest.ExportedAt.UTC().Format("20060102-150405") + "-" + manifest.ID + ".zip"
	path := filepath.Join(m.dir, filename)

	if err := os.WriteFile(path, zipBytes, 0o644); err != nil {
		return nil, fmt.Errorf("archive: write zip: %w", err)
	}

	meta := &ArchiveMeta{
		ID:         manifest.ID,
		Filename:   filename,
		Label:      manifest.Label,
		CreatedAt:  manifest.ExportedAt,
		SizeBytes:  int64(len(zipBytes)),
		AppVersion: manifest.AppVersion,
		Counts:     manifest.Counts,
	}

	m.mu.Lock()
	m.index = append([]*ArchiveMeta{meta}, m.index...) // prepend (newest first)
	if err := m.saveIndex(); err != nil {
		m.mu.Unlock()
		return nil, err
	}
	m.mu.Unlock()

	return meta, nil
}

// metaFromZIPFile reads the manifest from a ZIP file path and builds ArchiveMeta.
func metaFromZIPFile(path string) (*ArchiveMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	manifest, err := readManifestFromZIP(data)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &ArchiveMeta{
		ID:         manifest.ID,
		Filename:   filepath.Base(path),
		Label:      manifest.Label,
		CreatedAt:  manifest.ExportedAt,
		SizeBytes:  info.Size(),
		AppVersion: manifest.AppVersion,
		Counts:     manifest.Counts,
	}, nil
}

// readManifestFromZIP parses the manifest.json from raw ZIP bytes.
func readManifestFromZIP(data []byte) (*Manifest, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	for _, f := range r.File {
		if f.Name != "manifest.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open manifest.json: %w", err)
		}
		defer rc.Close()
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(rc); err != nil {
			return nil, fmt.Errorf("read manifest.json: %w", err)
		}
		return ParseManifest(buf.Bytes())
	}
	return nil, fmt.Errorf("manifest.json not found in zip")
}
