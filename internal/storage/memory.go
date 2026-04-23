package storage

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
)

// MemoryStorage implements Storage interface with in-memory storage
type MemoryStorage struct {
	mu              sync.RWMutex
	specs           map[string]*models.Spec
	operations      map[string]*models.Operation
	responseConfigs map[string]*models.ResponseConfig
	tags            map[string]*models.Tag
	scripts         map[string]*models.Script
	scriptBindings  map[string]*models.ScriptBinding
}

// NewMemoryStorage creates a new in-memory storage
func NewMemoryStorage() *MemoryStorage {
	storage := &MemoryStorage{
		specs:           make(map[string]*models.Spec),
		operations:      make(map[string]*models.Operation),
		responseConfigs: make(map[string]*models.ResponseConfig),
		tags:            make(map[string]*models.Tag),
		scripts:         make(map[string]*models.Script),
		scriptBindings:  make(map[string]*models.ScriptBinding),
	}

	// Ensure default tag exists
	storage.tags[models.DefaultTagName] = &models.Tag{
		Name:      models.DefaultTagName,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return storage
}

// CreateSpec creates a new spec
func (m *MemoryStorage) CreateSpec(spec *models.Spec) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.specs[spec.ID]; exists {
		return fmt.Errorf("spec with ID %s already exists", spec.ID)
	}

	spec.NormalizeMode()
	m.specs[spec.ID] = spec
	return nil
}

// GetSpec retrieves a spec by ID
func (m *MemoryStorage) GetSpec(id string) (*models.Spec, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	spec, exists := m.specs[id]
	if !exists {
		return nil, fmt.Errorf("spec not found: %s", id)
	}

	return spec, nil
}

// GetAllSpecs retrieves all specs
func (m *MemoryStorage) GetAllSpecs() ([]*models.Spec, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	specs := make([]*models.Spec, 0, len(m.specs))
	for _, spec := range m.specs {
		specs = append(specs, spec)
	}

	// Sort by name
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})

	return specs, nil
}

// GetEnabledSpecs retrieves all enabled specs
func (m *MemoryStorage) GetEnabledSpecs() ([]*models.Spec, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	specs := make([]*models.Spec, 0)
	for _, spec := range m.specs {
		if spec.Enabled {
			specs = append(specs, spec)
		}
	}

	return specs, nil
}

// UpdateSpec updates a spec
func (m *MemoryStorage) UpdateSpec(spec *models.Spec) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.specs[spec.ID]; !exists {
		return fmt.Errorf("spec not found: %s", spec.ID)
	}

	spec.NormalizeMode()
	m.specs[spec.ID] = spec
	return nil
}

// DeleteSpec deletes a spec
func (m *MemoryStorage) DeleteSpec(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.specs[id]; !exists {
		return fmt.Errorf("spec not found: %s", id)
	}

	delete(m.specs, id)
	return nil
}

// ListTags retrieves all tags
func (m *MemoryStorage) ListTags() ([]*models.Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tags := make([]*models.Tag, 0, len(m.tags))
	for _, tag := range m.tags {
		tags = append(tags, tag)
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags, nil
}

// GetTag retrieves a tag by name
func (m *MemoryStorage) GetTag(name string) (*models.Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tag, exists := m.tags[name]
	if !exists {
		return nil, fmt.Errorf("tag not found: %s", name)
	}

	return tag, nil
}

// CreateTag creates a new tag
func (m *MemoryStorage) CreateTag(tag *models.Tag) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tags[tag.Name]; exists {
		return fmt.Errorf("tag already exists: %s", tag.Name)
	}

	m.tags[tag.Name] = tag
	return nil
}

// UpdateTag updates a tag (supports rename)
func (m *MemoryStorage) UpdateTag(oldName string, tag *models.Tag) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tags[oldName]; !exists {
		return fmt.Errorf("tag not found: %s", oldName)
	}

	if oldName != tag.Name {
		if _, exists := m.tags[tag.Name]; exists {
			return fmt.Errorf("tag already exists: %s", tag.Name)
		}
		delete(m.tags, oldName)
	}

	m.tags[tag.Name] = tag
	return nil
}

// DeleteTag deletes a tag
func (m *MemoryStorage) DeleteTag(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tags[name]; !exists {
		return fmt.Errorf("tag not found: %s", name)
	}

	delete(m.tags, name)
	return nil
}

// CreateOperation creates a new operation
func (m *MemoryStorage) CreateOperation(op *models.Operation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.operations[op.ID]; exists {
		return fmt.Errorf("operation with ID %s already exists", op.ID)
	}

	m.operations[op.ID] = op
	return nil
}

// GetOperation retrieves an operation by ID
func (m *MemoryStorage) GetOperation(id string) (*models.Operation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	op, exists := m.operations[id]
	if !exists {
		return nil, fmt.Errorf("operation not found: %s", id)
	}

	return op, nil
}

// GetOperationsBySpec retrieves all operations for a spec
func (m *MemoryStorage) GetOperationsBySpec(specID string) ([]*models.Operation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ops := make([]*models.Operation, 0)
	for _, op := range m.operations {
		if op.SpecID == specID {
			ops = append(ops, op)
		}
	}

	// Sort by path, then method
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return ops[i].Method < ops[j].Method
	})

	return ops, nil
}

// GetAllOperations retrieves all operations
func (m *MemoryStorage) GetAllOperations() ([]*models.Operation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ops := make([]*models.Operation, 0, len(m.operations))
	for _, op := range m.operations {
		ops = append(ops, op)
	}

	return ops, nil
}

// UpdateOperation updates an operation
func (m *MemoryStorage) UpdateOperation(op *models.Operation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.operations[op.ID]; !exists {
		return fmt.Errorf("operation not found: %s", op.ID)
	}

	m.operations[op.ID] = op
	return nil
}

// DeleteOperation deletes an operation
func (m *MemoryStorage) DeleteOperation(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.operations[id]; !exists {
		return fmt.Errorf("operation not found: %s", id)
	}

	delete(m.operations, id)
	return nil
}

// DeleteOperationsBySpec deletes all operations for a spec
func (m *MemoryStorage) DeleteOperationsBySpec(specID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, op := range m.operations {
		if op.SpecID == specID {
			delete(m.operations, id)
		}
	}

	return nil
}

// CreateResponseConfig creates a new response config
func (m *MemoryStorage) CreateResponseConfig(cfg *models.ResponseConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.responseConfigs[cfg.ID]; exists {
		return fmt.Errorf("response config with ID %s already exists", cfg.ID)
	}

	cfg.NormalizeOrigin()
	m.responseConfigs[cfg.ID] = cfg
	return nil
}

// GetResponseConfig retrieves a response config by ID
func (m *MemoryStorage) GetResponseConfig(id string) (*models.ResponseConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, exists := m.responseConfigs[id]
	if !exists {
		return nil, fmt.Errorf("response config not found: %s", id)
	}

	return cfg, nil
}

// GetResponseConfigsByOperation retrieves all response configs for an operation
func (m *MemoryStorage) GetResponseConfigsByOperation(opID string) ([]*models.ResponseConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfgs := make([]*models.ResponseConfig, 0)
	for _, cfg := range m.responseConfigs {
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
func (m *MemoryStorage) UpdateResponseConfig(cfg *models.ResponseConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.responseConfigs[cfg.ID]; !exists {
		return fmt.Errorf("response config not found: %s", cfg.ID)
	}

	cfg.NormalizeOrigin()
	m.responseConfigs[cfg.ID] = cfg
	return nil
}

// DeleteResponseConfig deletes a response config
func (m *MemoryStorage) DeleteResponseConfig(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.responseConfigs[id]; !exists {
		return fmt.Errorf("response config not found: %s", id)
	}

	delete(m.responseConfigs, id)
	return nil
}

// DeleteResponseConfigsByOperation deletes all response configs for an operation
func (m *MemoryStorage) DeleteResponseConfigsByOperation(opID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, cfg := range m.responseConfigs {
		if cfg.OperationID == opID {
			delete(m.responseConfigs, id)
		}
	}

	return nil
}

// ---- Script operations ----

// CreateScript creates a new script
func (m *MemoryStorage) CreateScript(script *models.Script) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.scripts[script.ID]; exists {
		return fmt.Errorf("script with ID %s already exists", script.ID)
	}

	m.scripts[script.ID] = script
	return nil
}

// GetScript retrieves a script by ID
func (m *MemoryStorage) GetScript(id string) (*models.Script, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, exists := m.scripts[id]
	if !exists {
		return nil, fmt.Errorf("script not found: %s", id)
	}

	return s, nil
}

// GetAllScripts retrieves all scripts sorted by name
func (m *MemoryStorage) GetAllScripts() ([]*models.Script, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scripts := make([]*models.Script, 0, len(m.scripts))
	for _, s := range m.scripts {
		scripts = append(scripts, s)
	}

	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].Name < scripts[j].Name
	})

	return scripts, nil
}

// UpdateScript updates a script
func (m *MemoryStorage) UpdateScript(script *models.Script) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.scripts[script.ID]; !exists {
		return fmt.Errorf("script not found: %s", script.ID)
	}

	m.scripts[script.ID] = script
	return nil
}

// DeleteScript deletes a script
func (m *MemoryStorage) DeleteScript(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.scripts[id]; !exists {
		return fmt.Errorf("script not found: %s", id)
	}

	delete(m.scripts, id)
	return nil
}

// ---- ScriptBinding operations ----

// GetScriptBindings retrieves all bindings for an operation, sorted by Order
func (m *MemoryStorage) GetScriptBindings(operationID string) ([]*models.ScriptBinding, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bindings := make([]*models.ScriptBinding, 0)
	for _, b := range m.scriptBindings {
		if b.OperationID == operationID {
			bindings = append(bindings, b)
		}
	}

	sort.Slice(bindings, func(i, j int) bool {
		return bindings[i].Order < bindings[j].Order
	})

	return bindings, nil
}

// CreateScriptBinding creates a new binding
func (m *MemoryStorage) CreateScriptBinding(binding *models.ScriptBinding) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.scriptBindings[binding.ID]; exists {
		return fmt.Errorf("script binding with ID %s already exists", binding.ID)
	}

	m.scriptBindings[binding.ID] = binding
	return nil
}

// UpdateScriptBinding updates an existing binding
func (m *MemoryStorage) UpdateScriptBinding(binding *models.ScriptBinding) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.scriptBindings[binding.ID]; !exists {
		return fmt.Errorf("script binding not found: %s", binding.ID)
	}

	m.scriptBindings[binding.ID] = binding
	return nil
}

// DeleteScriptBinding deletes a binding by ID
func (m *MemoryStorage) DeleteScriptBinding(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.scriptBindings[id]; !exists {
		return fmt.Errorf("script binding not found: %s", id)
	}

	delete(m.scriptBindings, id)
	return nil
}

// DeleteScriptBindingsByScript deletes all bindings referencing a script
func (m *MemoryStorage) DeleteScriptBindingsByScript(scriptID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, b := range m.scriptBindings {
		if b.ScriptID == scriptID {
			delete(m.scriptBindings, id)
		}
	}

	return nil
}

// Close closes the storage (no-op for memory storage)
func (m *MemoryStorage) Close() error {
	return nil
}
