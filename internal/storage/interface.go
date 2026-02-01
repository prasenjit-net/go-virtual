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

	// Utility
	Close() error
}
