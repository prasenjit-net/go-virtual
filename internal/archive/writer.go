package archive

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
	"github.com/prasenjit/go-virtual/internal/version"
)

// storeFileFormat mirrors the layout written to data/store.json.
type storeFileFormat struct {
	UpdatedAt time.Time      `json:"updatedAt"`
	Entries   map[string]any `json:"entries"`
}

// BuildZIP creates an archive ZIP from the current state of the storage and
// global store. Returns the raw ZIP bytes and the manifest embedded in it.
func BuildZIP(label string, stor storage.Storage, gs store.GlobalStoreBackend) ([]byte, *Manifest, error) {
	id := shortID()
	checksums := make(map[string]string)
	counts := Counts{}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// addFile writes a named entry to the ZIP and records its SHA-256 checksum.
	addFile := func(name string, data []byte) error {
		w, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("archive: create zip entry %s: %w", name, err)
		}
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("archive: write zip entry %s: %w", name, err)
		}
		checksums[name] = sha256Hex(data)
		return nil
	}

	// ── Tags ────────────────────────────────────────────────────────────────
	tags, err := stor.ListTags()
	if err != nil {
		return nil, nil, fmt.Errorf("archive: list tags: %w", err)
	}
	tagsData, err := json.MarshalIndent(tags, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("archive: marshal tags: %w", err)
	}
	if err := addFile("tags.json", tagsData); err != nil {
		return nil, nil, err
	}
	counts.Tags = len(tags)

	// ── Global Store ────────────────────────────────────────────────────────
	snapshot := gs.Snapshot()
	storeFile := storeFileFormat{
		UpdatedAt: time.Now().UTC(),
		Entries:   snapshot,
	}
	storeData, err := json.MarshalIndent(&storeFile, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("archive: marshal store: %w", err)
	}
	if err := addFile("store.json", storeData); err != nil {
		return nil, nil, err
	}
	counts.StoreEntries = len(snapshot)

	// ── Specs ───────────────────────────────────────────────────────────────
	specs, err := stor.GetAllSpecs()
	if err != nil {
		return nil, nil, fmt.Errorf("archive: list specs: %w", err)
	}
	for _, spec := range specs {
		content := spec.Content
		metaCopy := *spec
		metaCopy.Content = "" // content stored separately

		metaData, err := json.MarshalIndent(&metaCopy, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("archive: marshal spec %s: %w", spec.ID, err)
		}
		if err := addFile("specs/"+spec.ID+".json", metaData); err != nil {
			return nil, nil, err
		}
		if content != "" {
			ext := ".yaml"
			if strings.HasPrefix(strings.TrimSpace(content), "{") {
				ext = ".spec.json"
			}
			if err := addFile("specs/"+spec.ID+ext, []byte(content)); err != nil {
				return nil, nil, err
			}
		}
	}
	counts.Specs = len(specs)

	// ── Scripts ─────────────────────────────────────────────────────────────
	scripts, err := stor.GetAllScripts()
	if err != nil {
		return nil, nil, fmt.Errorf("archive: list scripts: %w", err)
	}
	for _, script := range scripts {
		source := script.Source
		// Source has json:"-"; marshalling omits it — that's what we want for the .json file.
		metaData, err := json.MarshalIndent(script, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("archive: marshal script %s: %w", script.ID, err)
		}
		if err := addFile("scripts/"+script.ID+".json", metaData); err != nil {
			return nil, nil, err
		}
		if source != "" {
			if err := addFile("scripts/"+script.ID+".star", []byte(source)); err != nil {
				return nil, nil, err
			}
		}
	}
	counts.Scripts = len(scripts)

	// ── Response configs + script bindings (per operation) ──────────────────
	ops, err := stor.GetAllOperations()
	if err != nil {
		return nil, nil, fmt.Errorf("archive: list operations: %w", err)
	}
	responseCount := 0
	for _, op := range ops {
		cfgs, err := stor.GetResponseConfigsByOperation(op.ID)
		if err != nil {
			continue
		}
		for _, cfg := range cfgs {
			body := cfg.Body
			metaCopy := *cfg
			metaCopy.Body = "" // body stored separately

			cfgData, err := json.MarshalIndent(&metaCopy, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("archive: marshal response cfg %s: %w", cfg.ID, err)
			}
			if err := addFile("responses/"+cfg.ID+".json", cfgData); err != nil {
				return nil, nil, err
			}
			if body != "" {
				if err := addFile("responses/"+cfg.ID+".body", []byte(body)); err != nil {
					return nil, nil, err
				}
			}
			responseCount++
		}

		bindings, err := stor.GetScriptBindings(op.ID)
		if err != nil || len(bindings) == 0 {
			continue
		}
		bindData, err := json.MarshalIndent(bindings, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("archive: marshal bindings for op %s: %w", op.ID, err)
		}
		if err := addFile("operations/"+op.ID+".scripts.json", bindData); err != nil {
			return nil, nil, err
		}
	}
	counts.Responses = responseCount

	// ── Manifest (written last; not included in its own checksums) ───────────
	manifest := &Manifest{
		ID:         id,
		Version:    ManifestVersion,
		AppVersion: version.Get().Version,
		Label:      label,
		ExportedAt: time.Now().UTC(),
		Counts:     counts,
		Checksums:  checksums,
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("archive: marshal manifest: %w", err)
	}
	mw, err := zw.Create("manifest.json")
	if err != nil {
		return nil, nil, fmt.Errorf("archive: create manifest entry: %w", err)
	}
	if _, err := mw.Write(manifestData); err != nil {
		return nil, nil, fmt.Errorf("archive: write manifest: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, nil, fmt.Errorf("archive: close zip: %w", err)
	}

	return buf.Bytes(), manifest, nil
}

// shortID returns a random 8-character hex string.
func shortID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
