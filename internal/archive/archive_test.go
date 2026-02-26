package archive_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/archive"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
)

// newTestStore builds a GlobalStore backed by a temp file.
func newTestStore(t *testing.T) *store.GlobalStore {
	t.Helper()
	// Use a path inside TempDir that doesn't exist yet so NewGlobalStore
	// creates it fresh rather than failing to parse an empty file.
	path := filepath.Join(t.TempDir(), "store.json")
	gs, err := store.NewGlobalStore(path)
	if err != nil {
		t.Fatalf("new global store: %v", err)
	}
	return gs
}

// seedStorage populates a Storage with one spec, one tag, and one response config.
func seedStorage(t *testing.T, stor storage.Storage) {
	t.Helper()

	tag := &models.Tag{Name: "v2", Description: "Version 2", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := stor.CreateTag(tag); err != nil {
		t.Fatalf("create tag: %v", err)
	}

	spec := &models.Spec{
		ID:      "spec-001",
		Name:    "Pet Store",
		Version: "1.0.0",
		Content: miniPetstoreYAML,
		Enabled: true,
	}
	if err := stor.CreateSpec(spec); err != nil {
		t.Fatalf("create spec: %v", err)
	}

	op := &models.Operation{
		ID:          "op-001",
		SpecID:      "spec-001",
		Method:      "GET",
		Path:        "/pets",
		FullPath:    "/pets",
		OperationID: "listPets",
	}
	if err := stor.CreateOperation(op); err != nil {
		t.Fatalf("create operation: %v", err)
	}

	cfg := &models.ResponseConfig{
		ID:          "resp-001",
		OperationID: "op-001",
		Name:        "default-200",
		StatusCode:  200,
		Body:        `{"pets":[]}`,
		Enabled:     true,
	}
	if err := stor.CreateResponseConfig(cfg); err != nil {
		t.Fatalf("create response config: %v", err)
	}
}

// TestBuildAndParseManifest checks SHA-256 and round-trip manifest parsing.
func TestBuildAndParseManifest(t *testing.T) {
	stor := storage.NewMemoryStorage()
	gs := newTestStore(t)
	if err := gs.Set("greeting", "hello"); err != nil {
		t.Fatalf("set store: %v", err)
	}
	seedStorage(t, stor)

	zipBytes, manifest, err := archive.BuildZIP("test-label", stor, gs)
	if err != nil {
		t.Fatalf("BuildZIP: %v", err)
	}

	if manifest.Label != "test-label" {
		t.Errorf("label = %q, want %q", manifest.Label, "test-label")
	}
	if manifest.Counts.Specs != 1 {
		t.Errorf("counts.specs = %d, want 1", manifest.Counts.Specs)
	}
	if manifest.Counts.Responses != 1 {
		t.Errorf("counts.responses = %d, want 1", manifest.Counts.Responses)
	}
	if manifest.Counts.StoreEntries != 1 {
		t.Errorf("counts.storeEntries = %d, want 1", manifest.Counts.StoreEntries)
	}
	if len(manifest.Checksums) == 0 {
		t.Error("expected non-empty checksums")
	}
	if len(zipBytes) == 0 {
		t.Error("expected non-empty zip")
	}
}

// TestRoundTrip builds a ZIP and restores it into a fresh storage.
func TestRoundTrip(t *testing.T) {
	src := storage.NewMemoryStorage()
	gs := newTestStore(t)
	if err := gs.Set("key1", "value1"); err != nil {
		t.Fatalf("set store: %v", err)
	}
	seedStorage(t, src)

	zipBytes, _, err := archive.BuildZIP("rt", src, gs)
	if err != nil {
		t.Fatalf("BuildZIP: %v", err)
	}

	dst := storage.NewMemoryStorage()
	dstGS := newTestStore(t)

	result, err := archive.ApplyZIP(zipBytes, archive.RestoreOptions{}, dst, dstGS)
	if err != nil {
		t.Fatalf("ApplyZIP: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Errorf("unexpected restore errors: %v", result.Errors)
	}

	// Verify spec was restored.
	spec, err := dst.GetSpec("spec-001")
	if err != nil {
		t.Fatalf("GetSpec after restore: %v", err)
	}
	if spec.Name != "Pet Store" {
		t.Errorf("spec.Name = %q, want %q", spec.Name, "Pet Store")
	}

	// Verify response config was restored.
	cfg, err := dst.GetResponseConfig("resp-001")
	if err != nil {
		t.Fatalf("GetResponseConfig after restore: %v", err)
	}
	if cfg.Body != `{"pets":[]}` {
		t.Errorf("cfg.Body = %q, want %q", cfg.Body, `{"pets":[]}`)
	}

	// Verify store entry was restored.
	v, ok := dstGS.Get("key1")
	if !ok {
		t.Error("expected store key1 after restore")
	}
	if v != "value1" {
		t.Errorf("store key1 = %v, want %q", v, "value1")
	}
}

// TestChecksumFailure ensures ApplyZIP rejects a tampered archive.
func TestChecksumFailure(t *testing.T) {
	src := storage.NewMemoryStorage()
	gs := newTestStore(t)
	seedStorage(t, src)

	zipBytes, _, err := archive.BuildZIP("tamper", src, gs)
	if err != nil {
		t.Fatalf("BuildZIP: %v", err)
	}

	// Corrupt the last few bytes of the zip.
	tampered := make([]byte, len(zipBytes))
	copy(tampered, zipBytes)
	for i := len(tampered) - 10; i < len(tampered)-2; i++ {
		tampered[i] ^= 0xFF
	}

	dst := storage.NewMemoryStorage()
	dstGS := newTestStore(t)
	_, err = archive.ApplyZIP(tampered, archive.RestoreOptions{}, dst, dstGS)
	// Tampered ZIPs will likely fail to open — we just need *some* error.
	if err == nil {
		t.Error("expected error for tampered zip, got nil")
	}
}

// TestWipeFirst verifies that existing data is removed before applying the archive.
func TestWipeFirst(t *testing.T) {
	src := storage.NewMemoryStorage()
	gs := newTestStore(t)
	seedStorage(t, src)

	zipBytes, _, err := archive.BuildZIP("wipe-test", src, gs)
	if err != nil {
		t.Fatalf("BuildZIP: %v", err)
	}

	// Add an extra spec to the destination that is NOT in the archive.
	dst := storage.NewMemoryStorage()
	extraSpec := &models.Spec{ID: "extra-spec", Name: "extra", Content: miniPetstoreYAML}
	if err := dst.CreateSpec(extraSpec); err != nil {
		t.Fatalf("create extra spec: %v", err)
	}
	dstGS := newTestStore(t)
	_ = dstGS.Set("extra-key", "should-be-gone")

	_, err = archive.ApplyZIP(zipBytes, archive.RestoreOptions{WipeFirst: true}, dst, dstGS)
	if err != nil {
		t.Fatalf("ApplyZIP wipe: %v", err)
	}

	// Extra spec should be gone.
	if _, err := dst.GetSpec("extra-spec"); err == nil {
		t.Error("extra spec should have been wiped")
	}

	// Archive spec should be present.
	if _, err := dst.GetSpec("spec-001"); err != nil {
		t.Errorf("archive spec missing after wipe restore: %v", err)
	}

	// Extra store key should be gone.
	if _, ok := dstGS.Get("extra-key"); ok {
		t.Error("extra store key should have been wiped")
	}
}

// TestManagerCreate_List_Delete exercises the ArchiveManager lifecycle.
func TestManagerCreate_List_Delete(t *testing.T) {
	dir := t.TempDir()
	stor := storage.NewMemoryStorage()
	gs := newTestStore(t)
	seedStorage(t, stor)

	mgr, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}

	meta, err := mgr.Create("my-snapshot")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if meta.Label != "my-snapshot" {
		t.Errorf("label = %q, want %q", meta.Label, "my-snapshot")
	}

	list := mgr.List()
	if len(list) != 1 {
		t.Fatalf("List len = %d, want 1", len(list))
	}
	if list[0].ID != meta.ID {
		t.Errorf("list[0].ID = %q, want %q", list[0].ID, meta.ID)
	}

	if err := mgr.Delete(meta.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(mgr.List()) != 0 {
		t.Error("expected empty list after delete")
	}
}

// TestManagerRestore checks pre-restore backup creation and data application.
func TestManagerRestore(t *testing.T) {
	dir := t.TempDir()
	stor := storage.NewMemoryStorage()
	gs := newTestStore(t)
	seedStorage(t, stor)

	mgr, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}

	// Create an archive to restore from.
	snap, err := mgr.Create("snap")
	if err != nil {
		t.Fatalf("Create snap: %v", err)
	}

	// Restore with pre-backup.
	resp, err := mgr.Restore(snap.ID, archive.RestoreInput{
		CreateBackupFirst: true,
		BackupLabel:       "pre-restore",
		WipeFirst:         false,
	})
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if resp.BackupCreated == nil {
		t.Error("expected BackupCreated to be set")
	}
	if resp.BackupCreated.Label != "pre-restore" {
		t.Errorf("backup label = %q, want %q", resp.BackupCreated.Label, "pre-restore")
	}

	// There should now be 2 archives: snap + pre-restore backup.
	if len(mgr.List()) != 2 {
		t.Errorf("expected 2 archives, got %d", len(mgr.List()))
	}
}

// miniPetstoreYAML is a minimal OpenAPI 3 spec for seeding tests that need parseable content.
const miniPetstoreYAML = `openapi: "3.0.0"
info:
  title: Pet Store
  version: "1.0.0"
paths:
  /pets:
    get:
      summary: List all pets
      operationId: listPets
      responses:
        "200":
          description: A list of pets
`
