package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/parser"
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
	// Note: operations are in-memory only (derived from specs), no directory needed
	dirs := []string{
		basePath,
		filepath.Join(basePath, "specs"),
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

	var specsToMigrate []*models.Spec
	p := parser.NewParser()

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

		// Load spec content from separate file if it exists
		specID := strings.TrimSuffix(entry.Name(), ".json")
		content, err := f.loadSpecContent(specID)
		if err == nil && content != "" {
			spec.Content = content
		} else if spec.Content != "" {
			// Content is embedded in JSON (old format) - mark for migration
			specsToMigrate = append(specsToMigrate, &spec)
		}

		// Reset tracing to disabled on load - tracing should not persist across restarts
		spec.Tracing = false

		f.memory.specs[spec.ID] = &spec

		// Regenerate operations from spec content (operations are not persisted)
		if spec.Content != "" {
			operations, err := p.ParseOperations(spec.Content, spec.ID, spec.BasePath)
			if err == nil {
				for _, op := range operations {
					f.memory.operations[op.ID] = op
				}
			}
		}
	}

	// Load response configs
	respDir := filepath.Join(f.basePath, "responses")
	entries, err = os.ReadDir(respDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var configsToMigrate []*models.ResponseConfig

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

		// Load response body from separate file if it exists
		cfgID := strings.TrimSuffix(entry.Name(), ".json")
		body, err := f.loadResponseBody(cfgID)
		if err == nil && body != "" {
			cfg.Body = body
		} else if cfg.Body != "" {
			// Body is embedded in JSON (old format) - mark for migration
			configsToMigrate = append(configsToMigrate, &cfg)
		}

		f.memory.responseConfigs[cfg.ID] = &cfg
	}

	// Migrate specs to new format (separate content files)
	for _, spec := range specsToMigrate {
		if err := f.saveSpec(spec); err != nil {
			// Log but don't fail - data is still in memory
			fmt.Printf("Warning: failed to migrate spec %s to new format: %v\n", spec.ID, err)
		}
	}

	// Migrate response configs to new format (separate body files)
	for _, cfg := range configsToMigrate {
		if err := f.saveResponseConfig(cfg); err != nil {
			// Log but don't fail - data is still in memory
			fmt.Printf("Warning: failed to migrate response config %s to new format: %v\n", cfg.ID, err)
		}
	}

	return nil
}

// loadSpecContent loads the OpenAPI spec content from a separate file
func (f *FileStorage) loadSpecContent(specID string) (string, error) {
	// Try .yaml first, then .yml, then .json
	extensions := []string{".yaml", ".yml", ".spec.json"}
	specsDir := filepath.Join(f.basePath, "specs")

	for _, ext := range extensions {
		path := filepath.Join(specsDir, specID+ext)
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("spec content file not found for %s", specID)
}

// loadResponseBody loads the response body from a separate file
func (f *FileStorage) loadResponseBody(cfgID string) (string, error) {
	respDir := filepath.Join(f.basePath, "responses")
	path := filepath.Join(respDir, cfgID+".body")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// saveSpec saves a spec to disk (metadata in JSON, content in separate file)
func (f *FileStorage) saveSpec(spec *models.Spec) error {
	specsDir := filepath.Join(f.basePath, "specs")

	// Save content to separate file
	content := spec.Content
	if content != "" {
		// Determine file extension based on content
		ext := ".yaml"
		if strings.HasPrefix(strings.TrimSpace(content), "{") {
			ext = ".spec.json"
		}
		contentPath := filepath.Join(specsDir, spec.ID+ext)
		if err := os.WriteFile(contentPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	// Save metadata without content
	specCopy := *spec
	specCopy.Content = "" // Don't embed content in JSON

	data, err := json.MarshalIndent(&specCopy, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(specsDir, spec.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// deleteSpecFile deletes a spec file and its content file from disk
func (f *FileStorage) deleteSpecFile(id string) error {
	specsDir := filepath.Join(f.basePath, "specs")
	
	// Delete metadata JSON
	jsonPath := filepath.Join(specsDir, id+".json")
	os.Remove(jsonPath) // Ignore error if doesn't exist
	
	// Delete content files (try all extensions)
	extensions := []string{".yaml", ".yml", ".spec.json"}
	for _, ext := range extensions {
		os.Remove(filepath.Join(specsDir, id+ext))
	}
	
	return nil
}

// saveResponseConfig saves a response config to disk (metadata in JSON, body in separate file)
func (f *FileStorage) saveResponseConfig(cfg *models.ResponseConfig) error {
	respDir := filepath.Join(f.basePath, "responses")

	// Save body to separate file if not empty
	body := cfg.Body
	if body != "" {
		bodyPath := filepath.Join(respDir, cfg.ID+".body")
		if err := os.WriteFile(bodyPath, []byte(body), 0644); err != nil {
			return err
		}
	}

	// Save metadata without body
	cfgCopy := *cfg
	cfgCopy.Body = "" // Don't embed body in JSON

	data, err := json.MarshalIndent(&cfgCopy, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(respDir, cfg.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// deleteResponseConfigFile deletes a response config file and its body file from disk
func (f *FileStorage) deleteResponseConfigFile(id string) error {
	respDir := filepath.Join(f.basePath, "responses")
	
	// Delete metadata JSON
	jsonPath := filepath.Join(respDir, id+".json")
	os.Remove(jsonPath)
	
	// Delete body file
	bodyPath := filepath.Join(respDir, id+".body")
	os.Remove(bodyPath)
	
	return nil
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

// CreateOperation creates a new operation (in-memory only, not persisted)
func (f *FileStorage) CreateOperation(op *models.Operation) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.memory.CreateOperation(op)
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

// UpdateOperation updates an operation (in-memory only, not persisted)
func (f *FileStorage) UpdateOperation(op *models.Operation) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.memory.UpdateOperation(op)
}

// DeleteOperation deletes an operation (in-memory only)
func (f *FileStorage) DeleteOperation(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.memory.DeleteOperation(id)
}

// DeleteOperationsBySpec deletes all operations for a spec (in-memory only)
func (f *FileStorage) DeleteOperationsBySpec(specID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.memory.DeleteOperationsBySpec(specID)
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
