package storage

import (
	"github.com/prasenjit/go-virtual/internal/models"
)

// Storage defines the interface for data persistence
type Storage interface {
	// Spec operations
	CreateSpec(spec *models.Spec) error
	GetSpec(id string) (*models.Spec, error)
	GetAllSpecs() ([]*models.Spec, error)
	GetEnabledSpecs() ([]*models.Spec, error)
	UpdateSpec(spec *models.Spec) error
	DeleteSpec(id string) error

	// Tag operations
	ListTags() ([]*models.Tag, error)
	GetTag(name string) (*models.Tag, error)
	CreateTag(tag *models.Tag) error
	UpdateTag(oldName string, tag *models.Tag) error
	DeleteTag(name string) error

	// Operation operations
	CreateOperation(op *models.Operation) error
	GetOperation(id string) (*models.Operation, error)
	GetOperationsBySpec(specID string) ([]*models.Operation, error)
	GetAllOperations() ([]*models.Operation, error)
	UpdateOperation(op *models.Operation) error
	DeleteOperation(id string) error
	DeleteOperationsBySpec(specID string) error

	// ResponseConfig operations
	CreateResponseConfig(cfg *models.ResponseConfig) error
	GetResponseConfig(id string) (*models.ResponseConfig, error)
	GetResponseConfigsByOperation(opID string) ([]*models.ResponseConfig, error)
	UpdateResponseConfig(cfg *models.ResponseConfig) error
	DeleteResponseConfig(id string) error
	DeleteResponseConfigsByOperation(opID string) error

	// Script operations
	CreateScript(script *models.Script) error
	GetScript(id string) (*models.Script, error)
	GetAllScripts() ([]*models.Script, error)
	UpdateScript(script *models.Script) error
	DeleteScript(id string) error

	// ScriptBinding operations
	GetScriptBindings(operationID string) ([]*models.ScriptBinding, error)
	CreateScriptBinding(binding *models.ScriptBinding) error
	UpdateScriptBinding(binding *models.ScriptBinding) error
	DeleteScriptBinding(id string) error
	DeleteScriptBindingsByScript(scriptID string) error

	// Utility
	Close() error
}
