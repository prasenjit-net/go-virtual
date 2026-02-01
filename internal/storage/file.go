package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/prasenjit/go-virtual/internal/models"
)

// FileStorage implements Storage interface with file-based persistence
type FileStorage struct {
	mu       sync.RWMutex
	basePath string
	memory   *MemoryStorage
}

// NewFileStorage creates a new file-based storage
func NewFileStorage(basePath string) (*FileStorage, error) {
	// Create directories if they don't exist
	dirs := []string{
		basePath,
		filepath.Join(basePath, "specs"),
		filepath.Join(basePath, "operations"),
		filepath.Join(basePath, "responses"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	fs := &FileStorage{
		basePath: basePath,
		memory:   NewMemoryStorage(),
	}

	// Load existing data
	if err := fs.loadAll(); err != nil {
		return nil, err
	}

	return fs, nil
}

// loadAll loads all data from disk
func (f *FileStorage) loadAll() error {
	// Load specs
	specsDir := filepath.Join(f.basePath, "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(specsDir, entry.Name()))
		if err != nil {
			continue
		}

		var spec models.Spec
		if err := json.Unmarshal(data, &spec); err != nil {
			continue
		}

		f.memory.specs[spec.ID] = &spec
	}

	// Load operations
	opsDir := filepath.Join(f.basePath, "operations")
	entries, err = os.ReadDir(opsDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(opsDir, entry.Name()))
		if err != nil {
			continue
		}

		var op models.Operation
		if err := json.Unmarshal(data, &op); err != nil {
			continue
		}

		f.memory.operations[op.ID] = &op
	}

	// Load response configs
	respDir := filepath.Join(f.basePath, "responses")
	entries, err = os.ReadDir(respDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(respDir, entry.Name()))
		if err != nil {
			continue
		}

		var cfg models.ResponseConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}

		f.memory.responseConfigs[cfg.ID] = &cfg
	}

	return nil
}

// saveSpec saves a spec to disk
func (f *FileStorage) saveSpec(spec *models.Spec) error {
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(f.basePath, "specs", spec.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// deleteSpecFile deletes a spec file from disk
func (f *FileStorage) deleteSpecFile(id string) error {
	path := filepath.Join(f.basePath, "specs", id+".json")
	return os.Remove(path)
}

// saveOperation saves an operation to disk
func (f *FileStorage) saveOperation(op *models.Operation) error {
	data, err := json.MarshalIndent(op, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(f.basePath, "operations", op.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// deleteOperationFile deletes an operation file from disk
func (f *FileStorage) deleteOperationFile(id string) error {
	path := filepath.Join(f.basePath, "operations", id+".json")
	return os.Remove(path)
}

// saveResponseConfig saves a response config to disk
func (f *FileStorage) saveResponseConfig(cfg *models.ResponseConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(f.basePath, "responses", cfg.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// deleteResponseConfigFile deletes a response config file from disk
func (f *FileStorage) deleteResponseConfigFile(id string) error {
	path := filepath.Join(f.basePath, "responses", id+".json")
	return os.Remove(path)
}

// CreateSpec creates a new spec
func (f *FileStorage) CreateSpec(spec *models.Spec) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.CreateSpec(spec); err != nil {
		return err
	}

	return f.saveSpec(spec)
}

// GetSpec retrieves a spec by ID
func (f *FileStorage) GetSpec(id string) (*models.Spec, error) {
	return f.memory.GetSpec(id)
}

// GetAllSpecs retrieves all specs
func (f *FileStorage) GetAllSpecs() ([]*models.Spec, error) {
	return f.memory.GetAllSpecs()
}

// GetEnabledSpecs retrieves all enabled specs
func (f *FileStorage) GetEnabledSpecs() ([]*models.Spec, error) {
	return f.memory.GetEnabledSpecs()
}

// UpdateSpec updates a spec
func (f *FileStorage) UpdateSpec(spec *models.Spec) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.UpdateSpec(spec); err != nil {
		return err
	}

	return f.saveSpec(spec)
}

// DeleteSpec deletes a spec
func (f *FileStorage) DeleteSpec(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.DeleteSpec(id); err != nil {
		return err
	}

	return f.deleteSpecFile(id)
}

// CreateOperation creates a new operation
func (f *FileStorage) CreateOperation(op *models.Operation) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.CreateOperation(op); err != nil {
		return err
	}

	return f.saveOperation(op)
}

// GetOperation retrieves an operation by ID
func (f *FileStorage) GetOperation(id string) (*models.Operation, error) {
	return f.memory.GetOperation(id)
}

// GetOperationsBySpec retrieves all operations for a spec
func (f *FileStorage) GetOperationsBySpec(specID string) ([]*models.Operation, error) {
	return f.memory.GetOperationsBySpec(specID)
}

// GetAllOperations retrieves all operations
func (f *FileStorage) GetAllOperations() ([]*models.Operation, error) {
	return f.memory.GetAllOperations()
}

// UpdateOperation updates an operation
func (f *FileStorage) UpdateOperation(op *models.Operation) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.UpdateOperation(op); err != nil {
		return err
	}

	return f.saveOperation(op)
}

// DeleteOperation deletes an operation
func (f *FileStorage) DeleteOperation(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.DeleteOperation(id); err != nil {
		return err
	}

	return f.deleteOperationFile(id)
}

// DeleteOperationsBySpec deletes all operations for a spec
func (f *FileStorage) DeleteOperationsBySpec(specID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Get operations to delete
	ops, _ := f.memory.GetOperationsBySpec(specID)

	// Delete from memory
	if err := f.memory.DeleteOperationsBySpec(specID); err != nil {
		return err
	}

	// Delete files
	for _, op := range ops {
		f.deleteOperationFile(op.ID)
	}

	return nil
}

// CreateResponseConfig creates a new response config
func (f *FileStorage) CreateResponseConfig(cfg *models.ResponseConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.CreateResponseConfig(cfg); err != nil {
		return err
	}

	return f.saveResponseConfig(cfg)
}

// GetResponseConfig retrieves a response config by ID
func (f *FileStorage) GetResponseConfig(id string) (*models.ResponseConfig, error) {
	return f.memory.GetResponseConfig(id)
}

// GetResponseConfigsByOperation retrieves all response configs for an operation
func (f *FileStorage) GetResponseConfigsByOperation(opID string) ([]*models.ResponseConfig, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	cfgs := make([]*models.ResponseConfig, 0)
	for _, cfg := range f.memory.responseConfigs {
		if cfg.OperationID == opID {
			cfgs = append(cfgs, cfg)
		}
	}

	// Sort by priority
	sort.Slice(cfgs, func(i, j int) bool {
		return cfgs[i].Priority < cfgs[j].Priority
	})

	return cfgs, nil
}

// UpdateResponseConfig updates a response config
func (f *FileStorage) UpdateResponseConfig(cfg *models.ResponseConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.UpdateResponseConfig(cfg); err != nil {
		return err
	}

	return f.saveResponseConfig(cfg)
}

// DeleteResponseConfig deletes a response config
func (f *FileStorage) DeleteResponseConfig(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.DeleteResponseConfig(id); err != nil {
		return err
	}

	return f.deleteResponseConfigFile(id)
}

// DeleteResponseConfigsByOperation deletes all response configs for an operation
func (f *FileStorage) DeleteResponseConfigsByOperation(opID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Get configs to delete
	cfgs, _ := f.memory.GetResponseConfigsByOperation(opID)

	// Delete from memory
	if err := f.memory.DeleteResponseConfigsByOperation(opID); err != nil {
		return err
	}

	// Delete files
	for _, cfg := range cfgs {
		f.deleteResponseConfigFile(cfg.ID)
	}

	return nil
}

// Close closes the storage
func (f *FileStorage) Close() error {
	return nil
}
