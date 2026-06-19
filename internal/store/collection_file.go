package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// FileCollectionBackend persists each named collection as a JSON array in
// <basePath>/collections/<name>.json.  Writes are atomic (temp-file + rename).
type FileCollectionBackend struct {
	basePath string
	mu       sync.Map // map[string]*sync.RWMutex — one lock per collection name
}

// NewFileCollectionBackend returns a FileCollectionBackend rooted at basePath.
// The collections/ subdirectory is created if it does not exist.
func NewFileCollectionBackend(basePath string) (*FileCollectionBackend, error) {
	dir := filepath.Join(basePath, "collections")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &FileCollectionBackend{basePath: basePath}, nil
}

func (f *FileCollectionBackend) dir() string {
	return filepath.Join(f.basePath, "collections")
}

func (f *FileCollectionBackend) path(collection string) string {
	return filepath.Join(f.dir(), collection+".json")
}

func (f *FileCollectionBackend) lockFor(collection string) *sync.RWMutex {
	v, _ := f.mu.LoadOrStore(collection, &sync.RWMutex{})
	return v.(*sync.RWMutex)
}

// GetAll returns all documents in the named collection's base file.
func (f *FileCollectionBackend) GetAll(collection string) ([]map[string]any, error) {
	mu := f.lockFor(collection)
	mu.RLock()
	defer mu.RUnlock()
	return f.readLocked(collection)
}

func (f *FileCollectionBackend) readLocked(collection string) ([]map[string]any, error) {
	data, err := os.ReadFile(f.path(collection))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var docs []map[string]any
	if err := json.Unmarshal(data, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

func (f *FileCollectionBackend) writeLocked(collection string, docs []map[string]any) error {
	if docs == nil {
		docs = []map[string]any{}
	}
	data, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		return err
	}
	p := f.path(collection)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// SeedInsert appends one document to the collection's base file.
func (f *FileCollectionBackend) SeedInsert(collection string, doc map[string]any) (map[string]any, error) {
	mu := f.lockFor(collection)
	mu.Lock()
	defer mu.Unlock()

	docs, err := f.readLocked(collection)
	if err != nil {
		return nil, err
	}
	d := copyDoc(doc)
	if _, ok := d["_id"]; !ok {
		d["_id"] = uuid.New().String()
	}
	docs = append(docs, d)
	if err := f.writeLocked(collection, docs); err != nil {
		return nil, err
	}
	return copyDoc(d), nil
}

// SeedClear removes all documents from the collection's base file (writes []).
func (f *FileCollectionBackend) SeedClear(collection string) error {
	mu := f.lockFor(collection)
	mu.Lock()
	defer mu.Unlock()
	return f.writeLocked(collection, nil)
}

// ListCollections returns the names of collections that have a base file.
func (f *FileCollectionBackend) ListCollections() ([]string, error) {
	entries, err := os.ReadDir(f.dir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) == ".json" {
			names = append(names, name[:len(name)-5])
		}
	}
	return names, nil
}

// DropCollection removes the collection's base file entirely.
func (f *FileCollectionBackend) DropCollection(collection string) error {
	mu := f.lockFor(collection)
	mu.Lock()
	defer mu.Unlock()

	err := os.Remove(f.path(collection))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
