package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

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
		filepath.Join(basePath, "operations"),
		filepath.Join(basePath, "scripts"),
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
	// Load tags
	if err := f.loadTags(); err != nil {
		return err
	}

	// Load scripts
	if err := f.loadScripts(); err != nil {
		return err
	}

	// Load global AI scenarios
	aiScenariosLoaded, err := f.loadAIScenarios()
	if err != nil {
		return err
	}

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
		spec.NormalizeMode()

		f.memory.specs[spec.ID] = &spec

		// Regenerate operations from spec content (operations are not persisted)
		if spec.Content != "" {
			operations, err := p.ParseOperations(spec.Content, spec.ID, spec.BasePath)
			if err == nil {
				for _, op := range operations {
					// Overlay stored customizations (e.g. SignatureConfig)
					if custom, err := f.loadOperationCustomization(op.ID); err == nil && custom != nil {
						op.SignatureConfig = custom.SignatureConfig
					}
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

		if cfg.Tag == "" {
			cfg.Tag = models.DefaultTagName
		}
		cfg.NormalizeOrigin()

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

	if !aiScenariosLoaded {
		if migrated := f.migrateLegacyAIScenariosFromSpecs(); migrated {
			if err := f.saveAIScenarios(); err != nil {
				return err
			}
		}
	}

	return nil
}

func (f *FileStorage) tagsFilePath() string {
	return filepath.Join(f.basePath, "tags.json")
}

func (f *FileStorage) aiScenariosFilePath() string {
	return filepath.Join(f.basePath, "ai-scenarios.json")
}

func (f *FileStorage) loadTags() error {
	path := f.tagsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Ensure default tag exists
			f.memory.tags[models.DefaultTagName] = &models.Tag{
				Name:      models.DefaultTagName,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			return f.saveTags()
		}
		return err
	}

	var tags []*models.Tag
	if err := json.Unmarshal(data, &tags); err != nil {
		return err
	}

	for _, tag := range tags {
		if tag == nil || tag.Name == "" {
			continue
		}
		f.memory.tags[tag.Name] = tag
	}

	if _, exists := f.memory.tags[models.DefaultTagName]; !exists {
		f.memory.tags[models.DefaultTagName] = &models.Tag{
			Name:      models.DefaultTagName,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	return nil
}

func (f *FileStorage) saveTags() error {
	tags := make([]*models.Tag, 0, len(f.memory.tags))
	for _, tag := range f.memory.tags {
		tags = append(tags, tag)
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	data, err := json.MarshalIndent(tags, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(f.tagsFilePath(), data, 0644)
}

func (f *FileStorage) loadAIScenarios() (bool, error) {
	path := f.aiScenariosFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	var scenarios []models.AIScenario
	if err := json.Unmarshal(data, &scenarios); err != nil {
		return false, err
	}

	f.memory.aiScenarios = make(map[string]*models.AIScenario)
	for _, scenario := range models.NormalizeAIScenarios(scenarios) {
		scenarioCopy := scenario
		f.memory.aiScenarios[scenarioCopy.ID] = &scenarioCopy
	}

	return true, nil
}

func (f *FileStorage) saveAIScenarios() error {
	scenarios, err := f.memory.ListAIScenarios()
	if err != nil {
		return err
	}

	serialized := make([]models.AIScenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		if scenario != nil {
			serialized = append(serialized, *scenario)
		}
	}

	data, err := json.MarshalIndent(serialized, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(f.aiScenariosFilePath(), data, 0644)
}

func (f *FileStorage) migrateLegacyAIScenariosFromSpecs() bool {
	changed := false
	seenNames := make(map[string]struct{})
	for _, scenario := range f.memory.aiScenarios {
		seenNames[strings.ToLower(strings.TrimSpace(scenario.Name))] = struct{}{}
	}

	for _, spec := range f.memory.specs {
		for _, scenario := range models.NormalizeAIScenarios(spec.AIScenarios) {
			key := strings.ToLower(strings.TrimSpace(scenario.Name))
			if key == "" {
				continue
			}
			if _, exists := seenNames[key]; exists {
				continue
			}
			scenarioCopy := scenario
			f.memory.aiScenarios[scenarioCopy.ID] = &scenarioCopy
			seenNames[key] = struct{}{}
			changed = true
		}
	}

	return changed
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

// ListTags retrieves all tags
func (f *FileStorage) ListTags() ([]*models.Tag, error) {
	return f.memory.ListTags()
}

// GetTag retrieves a tag by name
func (f *FileStorage) GetTag(name string) (*models.Tag, error) {
	return f.memory.GetTag(name)
}

// CreateTag creates a new tag
func (f *FileStorage) CreateTag(tag *models.Tag) error {
	if err := f.memory.CreateTag(tag); err != nil {
		return err
	}
	return f.saveTags()
}

// UpdateTag updates a tag (supports rename)
func (f *FileStorage) UpdateTag(oldName string, tag *models.Tag) error {
	if err := f.memory.UpdateTag(oldName, tag); err != nil {
		return err
	}
	return f.saveTags()
}

// DeleteTag deletes a tag
func (f *FileStorage) DeleteTag(name string) error {
	if err := f.memory.DeleteTag(name); err != nil {
		return err
	}
	return f.saveTags()
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

// operationCustomizationPath returns the file path for an operation's customization file
func (f *FileStorage) operationCustomizationPath(opID string) string {
	return filepath.Join(f.basePath, "operations", opID+".json")
}

// operationCustomization holds the user-editable fields for an operation
type operationCustomization struct {
	ID              string                  `json:"id"`
	SignatureConfig *models.SignatureConfig `json:"signatureConfig,omitempty"`
}

// saveOperationCustomization persists the customisable fields of an operation to disk.
func (f *FileStorage) saveOperationCustomization(op *models.Operation) error {
	custom := &operationCustomization{
		ID:              op.ID,
		SignatureConfig: op.SignatureConfig,
	}

	data, err := json.MarshalIndent(custom, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(f.operationCustomizationPath(op.ID), data, 0644)
}

// loadOperationCustomization loads the stored customisation for an operation from disk.
// Returns nil without error when no customization file exists yet.
func (f *FileStorage) loadOperationCustomization(opID string) (*operationCustomization, error) {
	data, err := os.ReadFile(f.operationCustomizationPath(opID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var custom operationCustomization
	if err := json.Unmarshal(data, &custom); err != nil {
		return nil, err
	}
	if custom.SignatureConfig != nil {
		custom.SignatureConfig.Normalize()
	}

	return &custom, nil
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

// UpdateOperation updates an operation and persists its customisable fields (e.g. SignatureConfig)
func (f *FileStorage) UpdateOperation(op *models.Operation) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.UpdateOperation(op); err != nil {
		return err
	}

	return f.saveOperationCustomization(op)
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

	// Cascade-delete response-level script bindings
	f.memory.DeleteScriptBindingsByResponse(id) //nolint:errcheck
	os.Remove(f.responseScriptBindingsPath(id))

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

	// Delete files (including any response-level script binding files)
	for _, cfg := range cfgs {
		f.memory.DeleteScriptBindingsByResponse(cfg.ID) //nolint:errcheck
		os.Remove(f.responseScriptBindingsPath(cfg.ID))
		f.deleteResponseConfigFile(cfg.ID)
	}

	return nil
}

// ---- Script helpers ----

func (f *FileStorage) scriptMetaPath(id string) string {
	return filepath.Join(f.basePath, "scripts", id+".json")
}

func (f *FileStorage) scriptSourcePath(id string) string {
	return filepath.Join(f.basePath, "scripts", id+".star")
}

func (f *FileStorage) scriptBindingsPath(operationID string) string {
	return filepath.Join(f.basePath, "operations", operationID+".scripts.json")
}

func (f *FileStorage) responseScriptBindingsPath(responseConfigID string) string {
	return filepath.Join(f.basePath, "responses", responseConfigID+".scripts.json")
}

// saveScript writes <id>.json (no Source) and <id>.star (source text)
func (f *FileStorage) saveScript(script *models.Script) error {
	// Write source to .star file
	sourcePath := f.scriptSourcePath(script.ID)
	if err := os.WriteFile(sourcePath, []byte(script.Source), 0644); err != nil {
		return fmt.Errorf("write script source: %w", err)
	}

	// Write metadata to .json (Source excluded via json:"-")
	data, err := json.MarshalIndent(script, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f.scriptMetaPath(script.ID), data, 0644)
}

// loadScripts loads all scripts from the scripts/ directory
func (f *FileStorage) loadScripts() error {
	scriptsDir := filepath.Join(f.basePath, "scripts")
	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(scriptsDir, entry.Name()))
		if err != nil {
			continue
		}

		var script models.Script
		if err := json.Unmarshal(data, &script); err != nil {
			continue
		}

		// Load source from companion .star file
		sourceData, err := os.ReadFile(f.scriptSourcePath(script.ID))
		if err == nil {
			script.Source = string(sourceData)
		}

		f.memory.scripts[script.ID] = &script
	}

	// Load script bindings from operations/<id>.scripts.json files
	opsDir := filepath.Join(f.basePath, "operations")
	opEntries, err := os.ReadDir(opsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range opEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".scripts.json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(opsDir, entry.Name()))
		if err != nil {
			continue
		}

		var bindings []*models.ScriptBinding
		if err := json.Unmarshal(data, &bindings); err != nil {
			continue
		}

		for _, b := range bindings {
			if b != nil {
				f.memory.scriptBindings[b.ID] = b
			}
		}
	}

	// Load response-level script bindings from responses/<id>.scripts.json files
	respDir := filepath.Join(f.basePath, "responses")
	respEntries, err := os.ReadDir(respDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range respEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".scripts.json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(respDir, entry.Name()))
		if err != nil {
			continue
		}

		var bindings []*models.ScriptBinding
		if err := json.Unmarshal(data, &bindings); err != nil {
			continue
		}

		for _, b := range bindings {
			if b != nil {
				f.memory.scriptBindings[b.ID] = b
			}
		}
	}

	return nil
}

// deleteScriptFiles removes the .json and .star files for a script
func (f *FileStorage) deleteScriptFiles(id string) {
	os.Remove(f.scriptMetaPath(id))
	os.Remove(f.scriptSourcePath(id))
}

// saveScriptBindings persists the bindings for an operation
func (f *FileStorage) saveScriptBindings(operationID string) error {
	bindings, err := f.memory.GetScriptBindings(operationID)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(bindings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(f.scriptBindingsPath(operationID), data, 0644)
}

// saveResponseScriptBindings persists the bindings for a response config
func (f *FileStorage) saveResponseScriptBindings(responseConfigID string) error {
	bindings, err := f.memory.GetResponseScriptBindings(responseConfigID)
	if err != nil {
		return err
	}

	path := f.responseScriptBindingsPath(responseConfigID)

	if len(bindings) == 0 {
		// Clean up the file when there are no bindings
		os.Remove(path)
		return nil
	}

	data, err := json.MarshalIndent(bindings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}



// CreateScript creates a new script
func (f *FileStorage) CreateScript(script *models.Script) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.CreateScript(script); err != nil {
		return err
	}
	return f.saveScript(script)
}

// GetScript retrieves a script by ID
func (f *FileStorage) GetScript(id string) (*models.Script, error) {
	return f.memory.GetScript(id)
}

// GetAllScripts retrieves all scripts
func (f *FileStorage) GetAllScripts() ([]*models.Script, error) {
	return f.memory.GetAllScripts()
}

// UpdateScript updates a script
func (f *FileStorage) UpdateScript(script *models.Script) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.UpdateScript(script); err != nil {
		return err
	}
	return f.saveScript(script)
}

// DeleteScript deletes a script and its files
func (f *FileStorage) DeleteScript(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.DeleteScript(id); err != nil {
		return err
	}
	f.deleteScriptFiles(id)
	return nil
}

// ListAIScenarios retrieves all global AI scenarios.
func (f *FileStorage) ListAIScenarios() ([]*models.AIScenario, error) {
	return f.memory.ListAIScenarios()
}

// GetAIScenario retrieves a global AI scenario by ID.
func (f *FileStorage) GetAIScenario(id string) (*models.AIScenario, error) {
	return f.memory.GetAIScenario(id)
}

// CreateAIScenario creates and persists a global AI scenario.
func (f *FileStorage) CreateAIScenario(scenario *models.AIScenario) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.CreateAIScenario(scenario); err != nil {
		return err
	}
	return f.saveAIScenarios()
}

// UpdateAIScenario updates and persists a global AI scenario.
func (f *FileStorage) UpdateAIScenario(scenario *models.AIScenario) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.UpdateAIScenario(scenario); err != nil {
		return err
	}
	return f.saveAIScenarios()
}

// DeleteAIScenario deletes and persists global AI scenarios.
func (f *FileStorage) DeleteAIScenario(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.DeleteAIScenario(id); err != nil {
		return err
	}
	return f.saveAIScenarios()
}

// GetScriptBindings retrieves all bindings for an operation
func (f *FileStorage) GetScriptBindings(operationID string) ([]*models.ScriptBinding, error) {
	return f.memory.GetScriptBindings(operationID)
}

// GetResponseScriptBindings retrieves all bindings for a response config
func (f *FileStorage) GetResponseScriptBindings(responseConfigID string) ([]*models.ScriptBinding, error) {
	return f.memory.GetResponseScriptBindings(responseConfigID)
}

// CreateScriptBinding creates a new binding and persists the appropriate binding list
func (f *FileStorage) CreateScriptBinding(binding *models.ScriptBinding) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.CreateScriptBinding(binding); err != nil {
		return err
	}
	if binding.IsResponseBinding() {
		return f.saveResponseScriptBindings(binding.ResponseConfigID)
	}
	return f.saveScriptBindings(binding.OperationID)
}

// UpdateScriptBinding updates a binding and re-persists the appropriate binding list
func (f *FileStorage) UpdateScriptBinding(binding *models.ScriptBinding) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.UpdateScriptBinding(binding); err != nil {
		return err
	}
	if binding.IsResponseBinding() {
		return f.saveResponseScriptBindings(binding.ResponseConfigID)
	}
	return f.saveScriptBindings(binding.OperationID)
}

// DeleteScriptBinding removes a binding and re-persists the appropriate binding list
func (f *FileStorage) DeleteScriptBinding(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.memory.mu.RLock()
	binding, exists := f.memory.scriptBindings[id]
	f.memory.mu.RUnlock()
	if !exists {
		return fmt.Errorf("script binding not found: %s", id)
	}
	isResponse := binding.IsResponseBinding()
	ownerID := binding.OperationID
	if isResponse {
		ownerID = binding.ResponseConfigID
	}

	if err := f.memory.DeleteScriptBinding(id); err != nil {
		return err
	}
	if isResponse {
		return f.saveResponseScriptBindings(ownerID)
	}
	return f.saveScriptBindings(ownerID)
}

// DeleteScriptBindingsByScript removes all bindings for a script
func (f *FileStorage) DeleteScriptBindingsByScript(scriptID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Collect affected owner IDs before deletion
	f.memory.mu.RLock()
	affectedOps := make(map[string]struct{})
	affectedResps := make(map[string]struct{})
	for _, b := range f.memory.scriptBindings {
		if b.ScriptID == scriptID {
			if b.IsResponseBinding() {
				affectedResps[b.ResponseConfigID] = struct{}{}
			} else {
				affectedOps[b.OperationID] = struct{}{}
			}
		}
	}
	f.memory.mu.RUnlock()

	if err := f.memory.DeleteScriptBindingsByScript(scriptID); err != nil {
		return err
	}

	for opID := range affectedOps {
		if err := f.saveScriptBindings(opID); err != nil {
			fmt.Printf("Warning: failed to save script bindings for operation %s: %v\n", opID, err)
		}
	}
	for respID := range affectedResps {
		if err := f.saveResponseScriptBindings(respID); err != nil {
			fmt.Printf("Warning: failed to save script bindings for response %s: %v\n", respID, err)
		}
	}
	return nil
}

// DeleteScriptBindingsByResponse removes all bindings for a response config and cleans up the file
func (f *FileStorage) DeleteScriptBindingsByResponse(responseConfigID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.memory.DeleteScriptBindingsByResponse(responseConfigID); err != nil {
		return err
	}
	// saveResponseScriptBindings removes the file when there are no bindings
	return f.saveResponseScriptBindings(responseConfigID)
}

// Close closes the storage
func (f *FileStorage) Close() error {
	return nil
}
