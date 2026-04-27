package archive

import "io"

// ArchiveMode describes the capability level of the active ArchiveService.
//   - ModeFull     — multi-archive history on the local filesystem (storage.type = "file")
//   - ModeSnapshot — single on-demand snapshot with no persistent history
//     (storage.type = "memory" or "mongo")
type ArchiveMode string

const (
	ModeFull     ArchiveMode = "full"
	ModeSnapshot ArchiveMode = "snapshot"
)

// ArchiveService is the unified interface used by API handlers and serve.go.
// Both ArchiveManager (full) and SnapshotArchiveManager (snapshot) implement it.
type ArchiveService interface {
	// Mode returns the capability level.
	Mode() ArchiveMode

	// Full-mode operations — available only when Mode() == ModeFull.
	// Calling these in snapshot mode returns ErrSnapshotMode.
	List() []*ArchiveMeta
	Create(label string) (*ArchiveMeta, error)
	Get(id string) (*ArchiveMeta, error)
	Delete(id string) error
	FilePath(id string) (string, error)
	Save(r io.Reader, size int64, label string) (*ArchiveMeta, error)
	Restore(id string, input RestoreInput) (*RestoreResponse, error)

	// Snapshot-mode operations — available only when Mode() == ModeSnapshot.
	// Calling these in full mode returns ErrFullMode.
	DownloadSnapshot() ([]byte, *ArchiveMeta, error)
	RestoreSnapshot(data []byte) (*RestoreResponse, error)
}

// ErrSnapshotMode is returned when a full-mode method is called on a SnapshotArchiveManager.
const ErrSnapshotMode = archiveErr("archive: operation not available in snapshot mode; use GET /archives/snapshot and POST /archives/snapshot/restore")

// ErrFullMode is returned when a snapshot-mode method is called on an ArchiveManager.
const ErrFullMode = archiveErr("archive: operation not available in full mode; use the standard /archives endpoints")

type archiveErr string

func (e archiveErr) Error() string { return string(e) }
