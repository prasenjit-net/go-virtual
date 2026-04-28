package archive

import (
	"fmt"
	"io"
	"time"

	"github.com/prasenjit/go-virtual/internal/logging"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
)

// SnapshotArchiveManager implements ArchiveService in snapshot mode.
// It is used when storage.type is "memory" or "mongo" — backends where keeping
// archive ZIP files on a local filesystem is not appropriate.
//
// There is no persistent archive history. Each call to DownloadSnapshot()
// generates a fresh ZIP in memory. RestoreSnapshot() wipes the current state
// and applies the uploaded ZIP.
type SnapshotArchiveManager struct {
	stor storage.Storage
	gs   store.GlobalStoreBackend
}

// NewSnapshotArchiveManager creates a SnapshotArchiveManager.
func NewSnapshotArchiveManager(stor storage.Storage, gs store.GlobalStoreBackend) *SnapshotArchiveManager {
	return &SnapshotArchiveManager{stor: stor, gs: gs}
}

// Mode returns ModeSnapshot.
func (s *SnapshotArchiveManager) Mode() ArchiveMode { return ModeSnapshot }

// DownloadSnapshot generates a ZIP of the current state in memory and returns
// the raw bytes together with summary metadata.
func (s *SnapshotArchiveManager) DownloadSnapshot() ([]byte, *ArchiveMeta, error) {
	label := "snapshot-" + time.Now().UTC().Format("20060102-150405")
	zipBytes, manifest, err := BuildZIP(label, s.stor, s.gs)
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot: build zip: %w", err)
	}
	meta := &ArchiveMeta{
		ID:         manifest.ID,
		Filename:   label + ".zip",
		Label:      manifest.Label,
		CreatedAt:  manifest.ExportedAt,
		SizeBytes:  int64(len(zipBytes)),
		AppVersion: manifest.AppVersion,
		Counts:     manifest.Counts,
	}
	return zipBytes, meta, nil
}

// RestoreSnapshot wipes all current data and applies the provided ZIP bytes.
func (s *SnapshotArchiveManager) RestoreSnapshot(data []byte) (*RestoreResponse, error) {
	logger := logging.Logger("archive.snapshot")
	logger.Info("Starting snapshot restore", "event", "snapshot_restore_started", "bytes", len(data))

	result, err := ApplyZIP(data, RestoreOptions{WipeFirst: true}, s.stor, s.gs)
	if err != nil {
		errCount := 0
		if result != nil {
			errCount = len(result.Errors)
		}
		logger.Error("Snapshot restore failed",
			"event", "snapshot_restore_failed",
			"error", err,
			"result_errors", errCount,
		)
		return &RestoreResponse{Result: result}, err
	}

	logger.Info("Snapshot restore completed",
		"event", "snapshot_restore_completed",
		"created_specs", result.Created["specs"],
		"updated_specs", result.Updated["specs"],
	)
	return &RestoreResponse{Result: result}, nil
}

// ── Stub implementations for full-mode methods ───────────────────────────────

func (s *SnapshotArchiveManager) List() []*ArchiveMeta { return nil }

func (s *SnapshotArchiveManager) Create(_ string) (*ArchiveMeta, error) {
	return nil, ErrSnapshotMode
}

func (s *SnapshotArchiveManager) Get(_ string) (*ArchiveMeta, error) {
	return nil, ErrSnapshotMode
}

func (s *SnapshotArchiveManager) Delete(_ string) error { return ErrSnapshotMode }

func (s *SnapshotArchiveManager) FilePath(_ string) (string, error) {
	return "", ErrSnapshotMode
}

func (s *SnapshotArchiveManager) Save(_ io.Reader, _ int64, _ string) (*ArchiveMeta, error) {
	return nil, ErrSnapshotMode
}

func (s *SnapshotArchiveManager) Restore(_ string, _ RestoreInput) (*RestoreResponse, error) {
	return nil, ErrSnapshotMode
}
