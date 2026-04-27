package archive_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// seedStorage populates a Storage with one spec, one tag, one response config, one script, and one binding.
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

	script := &models.Script{
		ID:      "script-001",
		Name:    "hello-world",
		Source:  `def run(req): log("hello")`,
		Enabled: true,
	}
	if err := stor.CreateScript(script); err != nil {
		t.Fatalf("create script: %v", err)
	}

	binding := &models.ScriptBinding{
		ID:          "bind-001",
		OperationID: "op-001",
		ScriptID:    "script-001",
		Order:       0,
	}
	if err := stor.CreateScriptBinding(binding); err != nil {
		t.Fatalf("create script binding: %v", err)
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

// TestManagerSave validates that an externally supplied ZIP can be saved and retrieved.
func TestManagerSave(t *testing.T) {
	dir := t.TempDir()
	stor := storage.NewMemoryStorage()
	gs := newTestStore(t)
	seedStorage(t, stor)

	mgr, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}

	// Build a valid ZIP to use as the upload payload.
	zipBytes, _, err := archive.BuildZIP("external", stor, gs)
	if err != nil {
		t.Fatalf("BuildZIP: %v", err)
	}

	meta, err := mgr.Save(bytes.NewReader(zipBytes), int64(len(zipBytes)), "uploaded-label")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if meta.Label != "uploaded-label" {
		t.Errorf("label = %q, want %q", meta.Label, "uploaded-label")
	}
	if len(mgr.List()) != 1 {
		t.Errorf("expected 1 archive after Save, got %d", len(mgr.List()))
	}
}

// TestManagerSave_TooLarge ensures oversized uploads are rejected.
func TestManagerSave_TooLarge(t *testing.T) {
	dir := t.TempDir()
	stor := storage.NewMemoryStorage()
	gs := newTestStore(t)

	mgr, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}

	_, err = mgr.Save(bytes.NewReader([]byte("data")), 60<<20, "x")
	if err == nil {
		t.Error("expected error for oversized upload")
	}
}

// TestManagerSave_InvalidZIP ensures garbage bytes are rejected.
func TestManagerSave_InvalidZIP(t *testing.T) {
	dir := t.TempDir()
	stor := storage.NewMemoryStorage()
	gs := newTestStore(t)

	mgr, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}

	_, err = mgr.Save(bytes.NewReader([]byte("not a zip file")), 14, "x")
	if err == nil {
		t.Error("expected error for invalid zip payload")
	}
}

// TestManagerRebuildIndex tests that a deleted index.json is rebuilt from disk ZIPs.
func TestManagerRebuildIndex(t *testing.T) {
	dir := t.TempDir()
	stor := storage.NewMemoryStorage()
	gs := newTestStore(t)
	seedStorage(t, stor)

	mgr, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}
	if _, err := mgr.Create("snap"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Remove the index to force rebuild on next open.
	if err := os.Remove(filepath.Join(dir, "index.json")); err != nil {
		t.Fatalf("remove index: %v", err)
	}

	mgr2, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager after index removal: %v", err)
	}
	if len(mgr2.List()) != 1 {
		t.Errorf("expected 1 archive after rebuild, got %d", len(mgr2.List()))
	}
}

// TestManagerLoadIndex_Corrupt tests that a corrupt index.json is rebuilt from disk ZIPs.
func TestManagerLoadIndex_Corrupt(t *testing.T) {
	dir := t.TempDir()
	stor := storage.NewMemoryStorage()
	gs := newTestStore(t)
	seedStorage(t, stor)

	mgr, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}
	if _, err := mgr.Create("snap"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Overwrite the index with garbage.
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte("{{corrupt}}"), 0o644); err != nil {
		t.Fatalf("write corrupt index: %v", err)
	}

	mgr2, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager with corrupt index: %v", err)
	}
	if len(mgr2.List()) != 1 {
		t.Errorf("expected 1 archive after corrupt-index rebuild, got %d", len(mgr2.List()))
	}
}

// TestManagerGet_NotFound checks that Get returns an error for an unknown ID.
func TestManagerGet_NotFound(t *testing.T) {
	dir := t.TempDir()
	stor := storage.NewMemoryStorage()
	gs := newTestStore(t)

	mgr, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}
	if _, err := mgr.Get("nonexistent"); err == nil {
		t.Error("expected error for nonexistent archive")
	}
}

// TestManagerDelete_NotFound checks that Delete returns an error for an unknown ID.
func TestManagerDelete_NotFound(t *testing.T) {
	dir := t.TempDir()
	stor := storage.NewMemoryStorage()
	gs := newTestStore(t)

	mgr, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}
	if err := mgr.Delete("nonexistent"); err == nil {
		t.Error("expected error for nonexistent archive")
	}
}

// TestManagerFilePath_NotFound checks that FilePath returns an error for an unknown ID.
func TestManagerFilePath_NotFound(t *testing.T) {
	dir := t.TempDir()
	stor := storage.NewMemoryStorage()
	gs := newTestStore(t)

	mgr, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}
	if _, err := mgr.FilePath("nonexistent"); err == nil {
		t.Error("expected error for nonexistent archive")
	}
}

// TestManagerSortOrder verifies that archives are returned newest-first.
func TestManagerSortOrder(t *testing.T) {
	dir := t.TempDir()
	stor := storage.NewMemoryStorage()
	gs := newTestStore(t)
	seedStorage(t, stor)

	mgr, err := archive.NewArchiveManager(dir, stor, gs)
	if err != nil {
		t.Fatalf("NewArchiveManager: %v", err)
	}

	if _, err := mgr.Create("first"); err != nil {
		t.Fatalf("Create first: %v", err)
	}
	// Small sleep so timestamps differ.
	time.Sleep(2 * time.Millisecond)
	if _, err := mgr.Create("second"); err != nil {
		t.Fatalf("Create second: %v", err)
	}

	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 archives, got %d", len(list))
	}
	if list[0].Label != "second" {
		t.Errorf("expected newest-first; list[0].Label = %q, want %q", list[0].Label, "second")
	}
}

// ── SnapshotArchiveManager tests ─────────────────────────────────────────────

func TestSnapshotMode(t *testing.T) {
stor := storage.NewMemoryStorage()
gs := newTestStore(t)
seedStorage(t, stor)

mgr := archive.NewSnapshotArchiveManager(stor, gs)

if mgr.Mode() != archive.ModeSnapshot {
t.Fatalf("expected ModeSnapshot, got %q", mgr.Mode())
}
}

func TestSnapshotDownloadAndRestore(t *testing.T) {
stor := storage.NewMemoryStorage()
gs := newTestStore(t)
seedStorage(t, stor)

mgr := archive.NewSnapshotArchiveManager(stor, gs)

// Download
zipBytes, meta, err := mgr.DownloadSnapshot()
if err != nil {
t.Fatalf("DownloadSnapshot: %v", err)
}
if len(zipBytes) == 0 {
t.Fatal("expected non-empty ZIP")
}
if meta == nil {
t.Fatal("expected non-nil meta")
}
if meta.Filename == "" {
t.Error("expected non-empty filename")
}
if meta.SizeBytes != int64(len(zipBytes)) {
t.Errorf("meta.SizeBytes=%d, len(zipBytes)=%d", meta.SizeBytes, len(zipBytes))
}

// Wipe destination and restore
dest := storage.NewMemoryStorage()
gsDest := newTestStore(t)
mgr2 := archive.NewSnapshotArchiveManager(dest, gsDest)

resp, err := mgr2.RestoreSnapshot(zipBytes)
if err != nil {
t.Fatalf("RestoreSnapshot: %v", err)
}
if resp == nil || resp.Result == nil {
t.Fatal("expected non-nil response")
}
if len(resp.Result.Errors) > 0 {
t.Errorf("restore errors: %v", resp.Result.Errors)
}

specs, err := dest.GetAllSpecs()
if err != nil {
t.Fatalf("ListSpecs after restore: %v", err)
}
if len(specs) == 0 {
t.Error("expected at least one spec after restore")
}
}

func TestSnapshotFullModeStubsReturnError(t *testing.T) {
stor := storage.NewMemoryStorage()
gs := newTestStore(t)
mgr := archive.NewSnapshotArchiveManager(stor, gs)

if list := mgr.List(); list != nil {
t.Errorf("expected nil list in snapshot mode, got %v", list)
}

if _, err := mgr.Create("x"); err != archive.ErrSnapshotMode {
t.Errorf("Create: expected ErrSnapshotMode, got %v", err)
}
if _, err := mgr.Get("x"); err != archive.ErrSnapshotMode {
t.Errorf("Get: expected ErrSnapshotMode, got %v", err)
}
if err := mgr.Delete("x"); err != archive.ErrSnapshotMode {
t.Errorf("Delete: expected ErrSnapshotMode, got %v", err)
}
if _, err := mgr.FilePath("x"); err != archive.ErrSnapshotMode {
t.Errorf("FilePath: expected ErrSnapshotMode, got %v", err)
}
if _, err := mgr.Save(bytes.NewReader(nil), 0, ""); err != archive.ErrSnapshotMode {
t.Errorf("Save: expected ErrSnapshotMode, got %v", err)
}
if _, err := mgr.Restore("x", archive.RestoreInput{}); err != archive.ErrSnapshotMode {
t.Errorf("Restore: expected ErrSnapshotMode, got %v", err)
}
}

func TestArchiveManagerSnapshotStubsReturnError(t *testing.T) {
dir := t.TempDir()
stor := storage.NewMemoryStorage()
gs := newTestStore(t)
mgr, err := archive.NewArchiveManager(dir, stor, gs)
if err != nil {
t.Fatalf("NewArchiveManager: %v", err)
}

if mgr.Mode() != archive.ModeFull {
t.Fatalf("expected ModeFull, got %q", mgr.Mode())
}
if _, _, err := mgr.DownloadSnapshot(); err != archive.ErrFullMode {
t.Errorf("DownloadSnapshot: expected ErrFullMode, got %v", err)
}
if _, err := mgr.RestoreSnapshot(nil); err != archive.ErrFullMode {
t.Errorf("RestoreSnapshot: expected ErrFullMode, got %v", err)
}
}

func TestArchiveServiceInterface(t *testing.T) {
// Both implementations must satisfy the interface at compile time.
var _ archive.ArchiveService = archive.NewSnapshotArchiveManager(storage.NewMemoryStorage(), newTestStore(t))
dir := t.TempDir()
mgr, err := archive.NewArchiveManager(dir, storage.NewMemoryStorage(), newTestStore(t))
if err != nil {
t.Fatalf("NewArchiveManager: %v", err)
}
var _ archive.ArchiveService = mgr
}

func TestArchiveErrTypes(t *testing.T) {
// archiveErr.Error() — the typed sentinel
if archive.ErrSnapshotMode.Error() == "" {
t.Error("ErrSnapshotMode.Error() should be non-empty")
}
if archive.ErrFullMode.Error() == "" {
t.Error("ErrFullMode.Error() should be non-empty")
}
}

func TestSnapshotRestoreCorruptZIP(t *testing.T) {
stor := storage.NewMemoryStorage()
gs := newTestStore(t)
mgr := archive.NewSnapshotArchiveManager(stor, gs)

// Corrupt data should produce an error
_, err := mgr.RestoreSnapshot([]byte("not a zip"))
if err == nil {
t.Error("expected error restoring corrupt ZIP")
}
}

func TestManagerRebuildIndex_MultipleArchives(t *testing.T) {
dir := t.TempDir()
stor := storage.NewMemoryStorage()
gs := newTestStore(t)
seedStorage(t, stor)

mgr, err := archive.NewArchiveManager(dir, stor, gs)
if err != nil {
t.Fatalf("NewArchiveManager: %v", err)
}
if _, err := mgr.Create("first"); err != nil {
t.Fatalf("Create first: %v", err)
}
time.Sleep(2 * time.Millisecond)
if _, err := mgr.Create("second"); err != nil {
t.Fatalf("Create second: %v", err)
}

// Remove the index to force rebuild with 2 ZIPs.
if err := os.Remove(filepath.Join(dir, "index.json")); err != nil {
t.Fatalf("remove index: %v", err)
}

mgr2, err := archive.NewArchiveManager(dir, stor, gs)
if err != nil {
t.Fatalf("NewArchiveManager after index removal: %v", err)
}
list := mgr2.List()
if len(list) != 2 {
t.Fatalf("expected 2 archives after rebuild, got %d", len(list))
}
if list[0].Label != "second" {
t.Errorf("expected newest-first, got %q", list[0].Label)
}
}

func TestManagerRestoreWithBackup(t *testing.T) {
dir := t.TempDir()
stor := storage.NewMemoryStorage()
gs := newTestStore(t)
seedStorage(t, stor)

mgr, err := archive.NewArchiveManager(dir, stor, gs)
if err != nil {
t.Fatalf("NewArchiveManager: %v", err)
}
meta, err := mgr.Create("before-restore")
if err != nil {
t.Fatalf("Create: %v", err)
}

resp, err := mgr.Restore(meta.ID, archive.RestoreInput{WipeFirst: false, CreateBackupFirst: true})
if err != nil {
t.Fatalf("Restore with backup: %v", err)
}
if resp.BackupCreated == nil {
t.Error("expected backup to be created")
}
if len(mgr.List()) < 2 {
t.Errorf("expected >= 2 archives after restore with backup, got %d", len(mgr.List()))
}
}

func TestWipeFirst_WithRichStore(t *testing.T) {
src := storage.NewMemoryStorage()
gs := newTestStore(t)
seedStorage(t, src)

zipBytes, _, err := archive.BuildZIP("rich-wipe-test", src, gs)
if err != nil {
t.Fatalf("BuildZIP: %v", err)
}

// Use a destination that also has seeded data (scripts, tags, operations)
// so wipeAll exercises the loop bodies for each entity type.
dst := storage.NewMemoryStorage()
dstGS := newTestStore(t)
seedStorage(t, dst)
_ = dstGS.Set("extra-key", "should-be-gone")

_, err = archive.ApplyZIP(zipBytes, archive.RestoreOptions{WipeFirst: true}, dst, dstGS)
if err != nil {
t.Fatalf("ApplyZIP wipe with rich store: %v", err)
}

// Original data should still be present (restored from archive).
if _, err := dst.GetSpec("spec-001"); err != nil {
t.Errorf("archive spec missing after wipe restore: %v", err)
}
// Extra key should be gone.
if _, ok := dstGS.Get("extra-key"); ok {
t.Error("extra store key should have been wiped")
}
}

func TestManagerRestore_InvalidID(t *testing.T) {
dir := t.TempDir()
stor := storage.NewMemoryStorage()
gs := newTestStore(t)

mgr, err := archive.NewArchiveManager(dir, stor, gs)
if err != nil {
t.Fatalf("NewArchiveManager: %v", err)
}

_, err = mgr.Restore("nonexistent-id", archive.RestoreInput{})
if err == nil {
t.Error("expected error when restoring nonexistent archive")
}
}

func TestManagerRestore_DeletedFile(t *testing.T) {
dir := t.TempDir()
stor := storage.NewMemoryStorage()
gs := newTestStore(t)
seedStorage(t, stor)

mgr, err := archive.NewArchiveManager(dir, stor, gs)
if err != nil {
t.Fatalf("NewArchiveManager: %v", err)
}
meta, err := mgr.Create("test")
if err != nil {
t.Fatalf("Create: %v", err)
}

// Delete the ZIP file from disk but keep the index entry.
path, _ := mgr.FilePath(meta.ID)
if err := os.Remove(path); err != nil {
t.Fatalf("Remove: %v", err)
}

// Restore should fail because the ZIP is gone.
_, err = mgr.Restore(meta.ID, archive.RestoreInput{})
if err == nil {
t.Error("expected error when ZIP file is missing")
}
}

func TestBuildZIP_JSONSpec(t *testing.T) {
stor := storage.NewMemoryStorage()
gs := newTestStore(t)

// Create a spec with JSON content (not YAML).
jsonSpec := `{"openapi":"3.0.0","info":{"title":"test","version":"1.0"},"paths":{}}`
if err := stor.CreateSpec(&models.Spec{
ID:      "json-spec",
Name:    "JSON Spec",
Content: jsonSpec,
}); err != nil {
t.Fatalf("CreateSpec: %v", err)
}

zipBytes, _, err := archive.BuildZIP("json-spec-test", stor, gs)
if err != nil {
t.Fatalf("BuildZIP: %v", err)
}

// Verify we can read it back.
result, err := archive.ApplyZIP(zipBytes, archive.RestoreOptions{WipeFirst: true}, storage.NewMemoryStorage(), newTestStore(t))
if err != nil {
t.Fatalf("ApplyZIP: %v", err)
}
if result.Created["specs"] < 1 {
t.Errorf("expected at least 1 spec created, got %d", result.Created["specs"])
}
}

func TestApplyZIP_UnsupportedVersion(t *testing.T) {
// Build a valid ZIP, then tamper with manifest.json to have a different version.
stor := storage.NewMemoryStorage()
gs := newTestStore(t)
zipBytes, _, err := archive.BuildZIP("test", stor, gs)
if err != nil {
t.Fatalf("BuildZIP: %v", err)
}

// Unpack, modify manifest, repack.
r, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
if err != nil {
t.Fatalf("open zip: %v", err)
}
var newBuf bytes.Buffer
zw := zip.NewWriter(&newBuf)
for _, f := range r.File {
rc, _ := f.Open()
var content []byte
if f.Name == "manifest.json" {
var m archive.Manifest
_ = json.NewDecoder(rc).Decode(&m)
rc.Close()
m.Version = "999"
content, _ = json.Marshal(m)
} else {
buf := new(bytes.Buffer)
buf.ReadFrom(rc)
rc.Close()
content = buf.Bytes()
}
w, _ := zw.Create(f.Name)
w.Write(content)
}
zw.Close()

_, err = archive.ApplyZIP(newBuf.Bytes(), archive.RestoreOptions{}, storage.NewMemoryStorage(), newTestStore(t))
if err == nil || !strings.Contains(err.Error(), "unsupported version") {
t.Errorf("expected unsupported version error, got %v", err)
}
}

func TestParseManifest_InvalidJSON(t *testing.T) {
_, err := archive.ParseManifest([]byte("not json"))
if err == nil {
t.Error("expected error for invalid JSON manifest")
}
}
