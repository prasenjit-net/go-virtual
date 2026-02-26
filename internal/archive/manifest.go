package archive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// ManifestVersion is the current archive format version.
const ManifestVersion = "1"

// Counts summarises the number of items of each type in an archive.
type Counts struct {
	Specs        int `json:"specs"`
	Responses    int `json:"responses"`
	Scripts      int `json:"scripts"`
	Tags         int `json:"tags"`
	StoreEntries int `json:"storeEntries"`
}

// Manifest is written as manifest.json inside every archive ZIP.
// It does NOT include a checksum for itself.
type Manifest struct {
	ID         string            `json:"id"`
	Version    string            `json:"version"`
	AppVersion string            `json:"appVersion"`
	Label      string            `json:"label"`
	ExportedAt time.Time         `json:"exportedAt"`
	Counts     Counts            `json:"counts"`
	Checksums  map[string]string `json:"checksums"` // relative path → "sha256:<hex>"
}

// sha256Hex computes the SHA-256 hex digest of data.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ParseManifest decodes the manifest from JSON bytes.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("archive: parse manifest: %w", err)
	}
	return &m, nil
}
