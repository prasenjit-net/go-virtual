package archive

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/parser"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
)

// RestoreOptions controls how the archive is applied to the live storage.
type RestoreOptions struct {
	// WipeFirst deletes all existing data before applying the archive.
	// false = upsert (merge); true = full replacement.
	WipeFirst bool `json:"wipeFirst"`
}

// RestoreResult describes the outcome of a restore operation.
type RestoreResult struct {
	Created    map[string]int `json:"created"`
	Updated    map[string]int `json:"updated"`
	WipedFirst bool           `json:"wipedFirst"`
	Errors     []RestoreError `json:"errors,omitempty"`
}

// RestoreError captures a non-fatal error for a single file in the archive.
type RestoreError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// ApplyZIP applies an archive ZIP to the given storage, global store, and
// collection backend. It verifies SHA-256 checksums before touching any data.
// If any checksum fails the operation is aborted immediately and no writes are
// made. cb may be nil (collections are skipped).
func ApplyZIP(data []byte, opts RestoreOptions, stor storage.Storage, gs store.GlobalStoreBackend, cb store.CollectionBackend) (*RestoreResult, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("archive: open zip: %w", err)
	}

	// Build a name→content map and locate the manifest.
	files := make(map[string][]byte, len(r.File))
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("archive: open entry %s: %w", f.Name, err)
		}
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(rc); err != nil {
			rc.Close()
			return nil, fmt.Errorf("archive: read entry %s: %w", f.Name, err)
		}
		rc.Close()
		files[f.Name] = buf.Bytes()
	}

	manifestBytes, ok := files["manifest.json"]
	if !ok {
		return nil, fmt.Errorf("archive: manifest.json not found")
	}
	manifest, err := ParseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}
	if manifest.Version != ManifestVersion {
		return nil, fmt.Errorf("archive: unsupported version %q (expected %q)", manifest.Version, ManifestVersion)
	}

	// Verify all checksums before touching any data.
	var checksumErrors []RestoreError
	for path, expected := range manifest.Checksums {
		content, exists := files[path]
		if !exists {
			checksumErrors = append(checksumErrors, RestoreError{
				Path:    path,
				Message: "file listed in checksums not found in archive",
			})
			continue
		}
		if got := sha256Hex(content); got != expected {
			checksumErrors = append(checksumErrors, RestoreError{
				Path:    path,
				Message: fmt.Sprintf("checksum mismatch: got %s, want %s", got, expected),
			})
		}
	}
	if len(checksumErrors) > 0 {
		return &RestoreResult{Errors: checksumErrors}, fmt.Errorf("archive: checksum verification failed (%d error(s))", len(checksumErrors))
	}

	// ── Optional wipe ────────────────────────────────────────────────────────
	if opts.WipeFirst {
		if err := wipeAll(stor, gs, cb); err != nil {
			return nil, fmt.Errorf("archive: wipe failed: %w", err)
		}
	}

	result := &RestoreResult{
		Created:    map[string]int{"specs": 0, "scripts": 0, "responses": 0, "tags": 0, "storeEntries": 0, "bindings": 0, "collections": 0},
		Updated:    map[string]int{"specs": 0, "scripts": 0, "responses": 0, "tags": 0, "storeEntries": 0, "bindings": 0},
		WipedFirst: opts.WipeFirst,
	}
	var errs []RestoreError

	// ── Tags ────────────────────────────────────────────────────────────────
	if tagsBytes, ok := files["tags.json"]; ok {
		var tags []*models.Tag
		if err := json.Unmarshal(tagsBytes, &tags); err != nil {
			errs = append(errs, RestoreError{Path: "tags.json", Message: err.Error()})
		} else {
			for _, tag := range tags {
				if tag == nil || tag.Name == "" {
					continue
				}
				if _, err := stor.GetTag(tag.Name); err != nil {
					if e := stor.CreateTag(tag); e != nil {
						errs = append(errs, RestoreError{Path: "tags.json", Message: fmt.Sprintf("create tag %s: %v", tag.Name, e)})
					} else {
						result.Created["tags"]++
					}
				} else {
					if e := stor.UpdateTag(tag.Name, tag); e != nil {
						errs = append(errs, RestoreError{Path: "tags.json", Message: fmt.Sprintf("update tag %s: %v", tag.Name, e)})
					} else {
						result.Updated["tags"]++
					}
				}
			}
		}
	}

	// ── Global Store ────────────────────────────────────────────────────────
	if storeBytes, ok := files["store.json"]; ok {
		var sf storeFileFormat
		if err := json.Unmarshal(storeBytes, &sf); err != nil {
			errs = append(errs, RestoreError{Path: "store.json", Message: err.Error()})
		} else {
			for k, v := range sf.Entries {
				if e := gs.Set(k, v); e != nil {
					errs = append(errs, RestoreError{Path: "store.json", Message: fmt.Sprintf("set key %s: %v", k, e)})
				} else {
					result.Created["storeEntries"]++
				}
			}
		}
	}

	// ── Collections ─────────────────────────────────────────────────────────
	if cb != nil {
		for name, content := range files {
			if !strings.HasPrefix(name, "collections/") || !strings.HasSuffix(name, ".json") {
				continue
			}
			colName := strings.TrimSuffix(strings.TrimPrefix(name, "collections/"), ".json")
			if colName == "" {
				continue
			}
			var docs []map[string]any
			if err := json.Unmarshal(content, &docs); err != nil {
				errs = append(errs, RestoreError{Path: name, Message: err.Error()})
				continue
			}
			if err := cb.SeedClear(colName); err != nil {
				errs = append(errs, RestoreError{Path: name, Message: fmt.Sprintf("clear collection %s: %v", colName, err)})
				continue
			}
			for _, doc := range docs {
				if _, err := cb.SeedInsert(colName, doc); err != nil {
					errs = append(errs, RestoreError{Path: name, Message: fmt.Sprintf("seed insert into %s: %v", colName, err)})
				} else {
					result.Created["collections"]++
				}
			}
		}
	}

	// ── Scripts ─────────────────────────────────────────────────────────────
	for name, content := range files {
		if !strings.HasPrefix(name, "scripts/") || !strings.HasSuffix(name, ".json") {
			continue
		}
		var script models.Script
		if err := json.Unmarshal(content, &script); err != nil {
			errs = append(errs, RestoreError{Path: name, Message: err.Error()})
			continue
		}
		// Load source from companion .star file.
		starName := strings.TrimSuffix(name, ".json") + ".star"
		if starBytes, ok := files[starName]; ok {
			script.Source = string(starBytes)
		}
		if _, err := stor.GetScript(script.ID); err != nil {
			if e := stor.CreateScript(&script); e != nil {
				errs = append(errs, RestoreError{Path: name, Message: e.Error()})
			} else {
				result.Created["scripts"]++
			}
		} else {
			if e := stor.UpdateScript(&script); e != nil {
				errs = append(errs, RestoreError{Path: name, Message: e.Error()})
			} else {
				result.Updated["scripts"]++
			}
		}
	}

	// ── Specs (and regenerate operations) ───────────────────────────────────
	p := parser.NewParser()
	for name, content := range files {
		if !strings.HasPrefix(name, "specs/") || !strings.HasSuffix(name, ".json") {
			continue
		}
		var spec models.Spec
		if err := json.Unmarshal(content, &spec); err != nil {
			errs = append(errs, RestoreError{Path: name, Message: err.Error()})
			continue
		}
		// Load spec content from companion content file.
		for _, ext := range []string{".yaml", ".yml", ".spec.json"} {
			contentName := strings.TrimSuffix(name, ".json") + ext
			if cb, ok := files[contentName]; ok {
				spec.Content = string(cb)
				break
			}
		}

		isNew := false
		if _, err := stor.GetSpec(spec.ID); err != nil {
			if e := stor.CreateSpec(&spec); e != nil {
				errs = append(errs, RestoreError{Path: name, Message: e.Error()})
				continue
			}
			result.Created["specs"]++
			isNew = true
		} else {
			if e := stor.UpdateSpec(&spec); e != nil {
				errs = append(errs, RestoreError{Path: name, Message: e.Error()})
				continue
			}
			result.Updated["specs"]++
		}

		// Regenerate operations from spec content so they are immediately available.
		if spec.Content == "" {
			continue
		}
		ops, err := p.ParseOperations(spec.Content, spec.ID, spec.BasePath)
		if err != nil {
			errs = append(errs, RestoreError{Path: name, Message: fmt.Sprintf("parse ops: %v", err)})
			continue
		}
		for _, op := range ops {
			if isNew {
				_ = stor.CreateOperation(op)
			} else {
				if e := stor.UpdateOperation(op); e != nil {
					_ = stor.CreateOperation(op)
				}
			}
		}
	}

	// ── Response configs ─────────────────────────────────────────────────────
	for name, content := range files {
		if !strings.HasPrefix(name, "responses/") || !strings.HasSuffix(name, ".json") {
			continue
		}
		var cfg models.ResponseConfig
		if err := json.Unmarshal(content, &cfg); err != nil {
			errs = append(errs, RestoreError{Path: name, Message: err.Error()})
			continue
		}
		bodyName := strings.TrimSuffix(name, ".json") + ".body"
		if bodyBytes, ok := files[bodyName]; ok {
			cfg.Body = string(bodyBytes)
		}
		if _, err := stor.GetResponseConfig(cfg.ID); err != nil {
			if e := stor.CreateResponseConfig(&cfg); e != nil {
				errs = append(errs, RestoreError{Path: name, Message: e.Error()})
			} else {
				result.Created["responses"]++
			}
		} else {
			if e := stor.UpdateResponseConfig(&cfg); e != nil {
				errs = append(errs, RestoreError{Path: name, Message: e.Error()})
			} else {
				result.Updated["responses"]++
			}
		}
	}

	// ── Script bindings ──────────────────────────────────────────────────────
	for name, content := range files {
		if !strings.HasPrefix(name, "operations/") || !strings.HasSuffix(name, ".scripts.json") {
			continue
		}
		var bindings []*models.ScriptBinding
		if err := json.Unmarshal(content, &bindings); err != nil {
			errs = append(errs, RestoreError{Path: name, Message: err.Error()})
			continue
		}
		for _, b := range bindings {
			if b == nil {
				continue
			}
			existing, _ := stor.GetScriptBindings(b.OperationID)
			found := false
			for _, e := range existing {
				if e.ID == b.ID {
					found = true
					break
				}
			}
			if !found {
				if e := stor.CreateScriptBinding(b); e != nil {
					errs = append(errs, RestoreError{Path: name, Message: fmt.Sprintf("create binding %s: %v", b.ID, e)})
				} else {
					result.Created["bindings"]++
				}
			} else {
				if e := stor.UpdateScriptBinding(b); e != nil {
					errs = append(errs, RestoreError{Path: name, Message: fmt.Sprintf("update binding %s: %v", b.ID, e)})
				} else {
					result.Updated["bindings"]++
				}
			}
		}
	}

	result.Errors = errs
	return result, nil
}

// wipeAll removes all user data from storage, clears the global store, and
// drops all collections. The default tag is preserved. cb may be nil.
func wipeAll(stor storage.Storage, gs store.GlobalStoreBackend, cb store.CollectionBackend) error {
	// Wipe specs (cascade to operations and responses).
	specs, err := stor.GetAllSpecs()
	if err != nil {
		return err
	}
	for _, spec := range specs {
		ops, _ := stor.GetOperationsBySpec(spec.ID)
		for _, op := range ops {
			_ = stor.DeleteResponseConfigsByOperation(op.ID)
			_ = stor.DeleteScriptBindingsByScript("") // no-op for per-op cleanup
		}
		_ = stor.DeleteOperationsBySpec(spec.ID)
		_ = stor.DeleteSpec(spec.ID)
	}

	// Wipe scripts (cascade bindings per script).
	scripts, _ := stor.GetAllScripts()
	for _, s := range scripts {
		_ = stor.DeleteScriptBindingsByScript(s.ID)
		_ = stor.DeleteScript(s.ID)
	}

	// Wipe tags (except default).
	tags, _ := stor.ListTags()
	for _, t := range tags {
		if t.Name == models.DefaultTagName {
			continue
		}
		_ = stor.DeleteTag(t.Name)
	}

	// Clear global store.
	if err := gs.Clear(); err != nil {
		return err
	}

	// Drop all collections.
	if cb != nil {
		names, err := cb.ListCollections()
		if err != nil {
			return fmt.Errorf("archive: list collections for wipe: %w", err)
		}
		for _, name := range names {
			if err := cb.DropCollection(name); err != nil {
				return fmt.Errorf("archive: drop collection %s: %w", name, err)
			}
		}
	}
	return nil
}
